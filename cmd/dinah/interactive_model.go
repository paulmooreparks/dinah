//go:build tui

package main

import (
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/screen/keyboard"
	"dinah/internal/verb"
)

// interactiveMode is which of the six modes the model is in.
type interactiveMode int

const (
	modeBrowse interactiveMode = iota
	modeCard
	modeMenu
	modePrompt
	// modeItems lists the target card's items that carry an offered act.
	modeItems
	// modeOutput shows the transcript of a line too long for the message
	// area.
	modeOutput
)

// promptKind is which prompt is open in prompt mode.
type promptKind int

const (
	// promptJump is the command line on :, which is also the jump.
	promptJump promptKind = iota
	promptFilter
	// promptText is the multi-line prompt a stepText gathers its value in,
	// the comment's among them.
	promptText
	// promptStep is the single-line prompt a stepLine gathers its value in.
	promptStep
)

// interactiveMenuState is one open menu: the move menu, the actions menu, or
// the menu a step gathers its value from. choose runs when a row is chosen,
// and refresh rebuilds the rows after a read, answering false where the menu
// has nothing left to offer and closes.
type interactiveMenuState struct {
	title     string
	rows      []stepRow
	highlight int
	back      interactiveMode
	choose    func(stepRow) tea.Cmd
	refresh   func() ([]stepRow, bool)
}

// The messages the head sends its own model.
type (
	// keyMsg is one decoded key and the flush generation it was decoded
	// under.
	keyMsg struct {
		key tea.KeyPressMsg
		gen uint64
	}
	// pasteMsg is one whole paste and its generation.
	pasteMsg struct {
		content string
		gen     uint64
	}
	// flushedMsg confirms a flush and names the generation after it.
	flushedMsg struct {
		gen uint64
	}
	// resizeMsg says the console reported a change of its buffer size.
	resizeMsg struct{}
	// changeMsg is what one wait for a change answered, tagged with the watch
	// epoch the wait was launched under, so an answer launched before the
	// head's own act can be told apart from one launched after it.
	changeMsg struct {
		set   *verb.ChangeSet
		err   error
		epoch uint64
	}
	// viewReadMsg is what one read of the view answered, tagged with the read
	// generation it was asked under, so an answer that arrives after a later
	// read was asked for is dropped.
	viewReadMsg struct {
		seq     uint64
		answer  *verb.ViewAnswer
		err     error
		matches map[string]bool
	}
	// crashMsg says a panic was recovered and the program should quit.
	crashMsg struct{}
	// retryMsg says the pause after a failed wait has passed.
	retryMsg struct{}
	// repaintMsg asks for the whole screen to be drawn again on the next
	// frame, which the console writer sends after a short write.
	repaintMsg struct{}
)

// queuedInput is one key or one paste held in Update's keyMsg or pasteMsg
// case while a read is in flight, replayed by drainQueue in the order it
// arrived, through the ordinary dispatch, once the read lands.
type queuedInput struct {
	key     tea.KeyPressMsg
	paste   string
	isPaste bool
}

// interactiveCrash is a recovered panic: its value and the stack captured
// where it was recovered.
type interactiveCrash struct {
	value any
	stack []byte
}

// changeGate lets the head close the change wait once Run has returned: a
// wait takes the mutex for as long as it runs, and one starting after the
// gate is closed returns at once, so no wait is still reading the workbench
// when the head leaves.
type changeGate struct {
	mu     sync.Mutex
	closed bool
}

// close waits for any wait in progress and stops every later one.
func (g *changeGate) close() {
	g.mu.Lock()
	g.closed = true
	g.mu.Unlock()
}

// laneEntry is one lane of the model: the cards of one column of one section
// in the columns layout, or of one section in the list layout.
type laneEntry struct {
	section  int
	column   string
	title    string
	operator bool
	cards    []verb.CardView
	// refused is the section this workbench could not ask, in the list
	// layout, nil on every other lane.
	refused *verb.ViewSectionAnswer
}

// key is the lane's identity across a re-read.
func (lane laneEntry) key() string {
	return strconv.Itoa(lane.section) + "/" + lane.column
}

// interactiveOffer is the acts the footer and the menus offer for one card,
// with the a and b rows picked out, and the whole answer the library gave.
type interactiveOffer struct {
	ref                     string
	claim, release, comment bool
	moves                   []verb.LegalMove
	forward, back           *verb.LegalMove
	forwardTerminal         bool
	acts                    *verb.OfferedActs
	// edit is OfferActs' Edit asked of the editor ladder as well, once per
	// recompute, so drawing a frame reads no configuration file.
	edit bool
}

// interactiveModel is the terminal head's Bubble Tea model.
type interactiveModel struct {
	s       *session
	l       *verb.Library
	waiter  *verb.Library
	req     *verb.Request
	program *tea.Program
	reader  keyboard.Reader
	glyphs  boardGlyphs
	colour  bool

	width, height int
	answer        *verb.ViewAnswer
	filter        string
	matches       map[string]bool
	lanes         []laneEntry
	focus         int
	selected      int

	mode         interactiveMode
	promptBack   interactiveMode
	cardOpen     bool
	cardRef      string
	cardLines    []string
	cardBasis    string
	cardInView   bool
	cardArchived bool
	viewport     viewport.Model
	menu         *interactiveMenuState
	pending      *pendingAct
	prompt       promptKind
	input        textinput.Model
	area         textarea.Model
	offer        interactiveOffer
	detail       []string
	message      []string
	status       statusParts
	fullHelp     bool
	help         help.Model

	cursor     string
	gate       *changeGate
	flushAsked bool
	minGen     uint64
	quitting   bool
	expiry     time.Time

	// itemRows are the rows item mode lists, and itemHighlight the one
	// highlighted.
	itemRows      []verb.OfferedItem
	itemHighlight int
	// output is the transcript output mode shows, under outputTitle,
	// scrolled to outputOffset, and outputBack the mode it closes into.
	output       []string
	outputTitle  string
	outputOffset int
	outputBack   interactiveMode
	// bindings are the user's key bindings the head reads, and unused the
	// notice of the stored ones it will not run, shown once at start.
	bindings []bench.KeyBinding
	// lend is the command a cycle ended to hand the terminal to, and lent
	// says the cycle now starting follows a lend, whose transcript it shows.
	lend *lendRequest
	lent *lineResult
	// tabbed says the last key at the command line was Tab, so a second Tab
	// lists the candidates.
	tabbed bool

	// readSeq is bumped on every read of the whole view, every one of which
	// now runs off the event loop, whether it is an act's own, a line's own,
	// or the watcher's; an answer whose generation readSeq no longer carries
	// is superseded by a later read and dropped rather than applied.
	// readAnswered is the generation of the last current answer actually
	// applied, so readSeq != readAnswered is true exactly while a read is in
	// flight. A key or a paste that arrives while that holds is queued in
	// queued rather than processed or dropped, and replayed in order,
	// through the ordinary dispatch, once the read lands; ctrl+c is the one
	// key that bypasses the queue; see the keyMsg and pasteMsg cases in
	// Update and drainQueue.
	// watchEpoch is bumped after the head's own successful act, so the wait
	// already in flight when the act was made, which goes on to answer a
	// change the head has already redrawn for itself, is recognised and its
	// own refresh is skipped, keeping one act to one refresh; batchIsOwnAct
	// is where that recognition stops short of swallowing another actor's
	// change riding in the same batch.
	readSeq      uint64
	readAnswered uint64
	queued       []queuedInput
	watchEpoch   uint64
	// ownActRef and ownActRevision are the card and the revision the head's
	// own last successful act left it at, from that act's own response,
	// which is what batchIsOwnAct compares a stale batch's events against
	// rather than identity: several sessions on this workbench share one
	// actor, harness, provider and model, so identity alone cannot tell the
	// head's own act apart from a different session's, sharing that same
	// identity, landing in the same batch.
	ownActRef      string
	ownActRevision string
	// pendingLine is the line whose lineDone notice handleViewRead has not
	// yet fired, because doing so waits on the board-wide read afterLine's
	// own reread dispatched, which now runs off the event loop like every
	// other; nil once handleViewRead has fired it. The line's own message and
	// mode are composed synchronously in afterLine itself, since neither
	// depends on this read.
	pendingLine *lineResult
	// pullCache holds, for the read now on screen, whether pulling the next
	// ready card into a column named by its reference would succeed; the
	// answer depends only on that column, never on which card recomputeOffer
	// was asked about, so it is asked of the library once per column per
	// read rather than once per keystroke of navigation. It is cleared on
	// every fresh read, in applyAnswer.
	pullCache map[string]bool

	crashMu sync.Mutex
	crash   *interactiveCrash
}

// newInteractiveModel builds the model over the first read of the view.
func newInteractiveModel(s *session, l, waiter *verb.Library, req *verb.Request, first *verb.ViewAnswer, cursor string, width, height int) *interactiveModel {
	m := &interactiveModel{
		s:      s,
		l:      l,
		waiter: waiter,
		req:    req,
		glyphs: s.boardGlyphSet(req.ViewPlain),
		colour: colourAllowed(),
		width:  width,
		height: height,
		cursor: cursor,
		gate:   &changeGate{},
	}
	m.status = statusParts{time: s.r.T("view.watch.since", "time", now().Format(watchClock))}
	m.help = interactiveHelp(s, m.glyphs)
	m.input = interactiveInput()
	m.area = interactiveArea()
	m.viewport = viewport.New()
	m.viewport.KeyMap = viewport.KeyMap{}
	m.applyAnswer(first, true)
	m.loadBindings(true)
	return m
}

// Init starts the change loop. A cycle that follows a lend first asks the
// reader to discard the keys typed while the terminal was lent, reads the
// view again and shows what the lent command wrote.
func (m *interactiveModel) Init() tea.Cmd {
	if interactiveSeam != nil && interactiveSeam.init != nil {
		interactiveSeam.init()
	}
	cmds := []tea.Cmd{m.waitForChange()}
	if result := m.lent; result != nil {
		m.lent = nil
		if m.reader != nil {
			m.flushAsked = true
			m.reader.RequestFlush()
		}
		cmds = append(cmds, m.afterLine(result))
	}
	return tea.Batch(cmds...)
}

// recordCrash keeps a recovered panic and the stack where it was recovered,
// the first one only.
func (m *interactiveModel) recordCrash(value any) {
	m.crashMu.Lock()
	defer m.crashMu.Unlock()
	if m.crash == nil {
		m.crash = &interactiveCrash{value: value, stack: debug.Stack()}
	}
}

// crashed answers the recovered panic, nil where none was.
func (m *interactiveModel) crashed() *interactiveCrash {
	m.crashMu.Lock()
	defer m.crashMu.Unlock()
	return m.crash
}

// guarded runs a command inside a deferred recover, so a panic in it is kept
// and answered with crashMsg, on which Update quits.
func (m *interactiveModel) guarded(cmd func() tea.Msg) tea.Cmd {
	return func() (answer tea.Msg) {
		defer func() {
			if r := recover(); r != nil {
				m.recordCrash(r)
				answer = crashMsg{}
			}
		}()
		if interactiveSeam != nil && interactiveSeam.command != nil {
			interactiveSeam.command()
		}
		return cmd()
	}
}

// waitForChange is the command that waits for the next change on the waiter
// library, for at most watchWait. The watch epoch it is launched under travels
// with the answer, so changed can tell a wait launched before the head's own
// act apart from one launched after it.
func (m *interactiveModel) waitForChange() tea.Cmd {
	cursor, gate, waiter, actor, epoch := m.cursor, m.gate, m.waiter, m.req.Actor, m.watchEpoch
	return m.guarded(func() tea.Msg {
		gate.mu.Lock()
		defer gate.mu.Unlock()
		if gate.closed {
			return nil
		}
		set, err := waiter.Changes(&verb.Request{Verb: "changes", Actor: actor, Since: cursor, Wait: true, Timeout: watchWait})
		return changeMsg{set: set, err: err, epoch: epoch}
	})
}

// Update handles one message.
func (m *interactiveModel) Update(msg tea.Msg) (model tea.Model, cmd tea.Cmd) {
	defer func() {
		if r := recover(); r != nil {
			m.recordCrash(r)
			m.quitting = true
			model, cmd = m, tea.Quit
		}
	}()
	if interactiveSeam != nil && interactiveSeam.update != nil {
		interactiveSeam.update(msg)
	}
	if interactiveSeam != nil && interactiveSeam.observe != nil {
		interactiveSeam.observe(m, msg)
	}
	if m.quitting {
		return m, nil
	}
	switch msg := msg.(type) {
	case crashMsg:
		m.quitting = true
		return m, tea.Quit
	case tea.WindowSizeMsg:
		return m, m.resized(msg)
	case resizeMsg:
		return m, m.measure()
	case changeMsg:
		return m, m.changed(msg)
	case retryMsg:
		return m, m.waitForChange()
	case repaintMsg:
		return m, tea.ClearScreen
	case flushedMsg:
		if m.flushAsked {
			m.flushAsked = false
			m.minGen = msg.gen
		}
		return m, nil
	case viewReadMsg:
		return m, m.handleViewRead(msg)
	case keyMsg:
		if m.flushAsked || msg.gen < m.minGen {
			return m, nil
		}
		// A read in flight (the head's own act, or the watcher's) is
		// answered by a later message, not by this one, so a key that
		// arrives now is queued rather than processed or dropped, unless it
		// is the one key nothing may ever wait behind.
		if msg.key.String() != "ctrl+c" && m.readSeq != m.readAnswered {
			m.queued = append(m.queued, queuedInput{key: msg.key})
			return m, nil
		}
		return m, m.key(msg.key)
	case pasteMsg:
		if m.flushAsked || msg.gen < m.minGen {
			return m, nil
		}
		// A paste is exactly as capable of racing an in-flight read as a
		// key is, so it is gated and queued the same way rather than
		// dispatched straight to m.paste.
		if m.readSeq != m.readAnswered {
			m.queued = append(m.queued, queuedInput{paste: msg.content, isPaste: true})
			return m, nil
		}
		return m, m.paste(msg.content)
	}
	return m, nil
}

// tooSmall reports whether the window is below the minimum the screen needs.
func (m *interactiveModel) tooSmall() bool {
	return m.width < interactiveMinimumWidth || m.height < interactiveMinimumHeight
}

// sendSize is the command that delivers a size to Bubble Tea's renderer and
// to the model together.
func sendSize(width, height int) tea.Cmd {
	return func() tea.Msg {
		return tea.WindowSizeMsg{Width: width, Height: height}
	}
}

// resized takes a size Bubble Tea delivered. The head's own measure decides
// the size, so a size it disagrees with is answered with the head's.
func (m *interactiveModel) resized(msg tea.WindowSizeMsg) tea.Cmd {
	width, height := m.s.interactiveSize()
	if width > 0 && height > 0 && (width != msg.Width || height != msg.Height) {
		return sendSize(width, height)
	}
	m.width, m.height = msg.Width, msg.Height
	m.fit()
	return nil
}

// measure reads the window's size and, where it differs from the model's,
// answers the command that delivers it.
func (m *interactiveModel) measure() tea.Cmd {
	width, height := m.s.interactiveSize()
	if width <= 0 || height <= 0 || (width == m.width && height == m.height) {
		return nil
	}
	return sendSize(width, height)
}

// fit sizes the components to the window.
func (m *interactiveModel) fit() {
	m.help.SetWidth(m.draw())
	m.input.SetWidth(m.draw() - 3)
	m.area.SetWidth(m.draw())
	m.area.SetHeight(m.areaHeight())
	m.viewport.SetWidth(m.draw())
	m.viewport.SetHeight(m.bodyHeight())
	m.refreshDetail()
	if m.cardOpen {
		m.showCard(m.cardRef, m.cardInView)
	}
}

// draw is how many display columns every row is drawn in: the window's width
// less one, which is the watch's rule.
func (m *interactiveModel) draw() int {
	return m.width - 1
}

// changed handles what one wait for a change answered, and starts the next,
// or, after a wait that failed, the pause before it. A wait launched before
// the head's own act can answer reporting the very change that act already
// made and redrew, and that report is skipped rather than drawn a second
// time, but only where batchIsOwnAct says the report holds nothing else: the
// same batch can carry another actor's change riding in on the same window,
// since Changes reports everything since the old cursor in one shot, and that
// one must still be drawn.
//
// A stale wait's own report never clears the message area, whether or not
// batchIsOwnAct explains the whole of it: the head's own act, launched after
// this wait, already set whatever the message area shows now, out of a
// response this belated report cannot be fresher than, so wiping it here
// would let a report of somebody else's change, riding in on the same
// window as an act on a different card, erase that act's own refusal or
// confirmation before anyone read it. The board itself, the lanes and the
// offer this report may still hand to reread, is a different matter and is
// still redrawn, since those do need the fresher state.
func (m *interactiveModel) changed(msg changeMsg) tea.Cmd {
	cmds := []tea.Cmd{m.measure()}
	stale := msg.epoch != m.watchEpoch
	switch {
	case msg.err != nil:
		// A wait that failed is tried again after watchWait rather than at
		// once, so a workbench that stays unreadable is not read in a loop.
		m.message = m.errorLines(msg.err)
		retry := tea.Tick(watchWait, func(time.Time) tea.Msg { return retryMsg{} })
		return tea.Batch(append(cmds, retry)...)
	case msg.set.Changed:
		m.cursor = msg.set.Cursor
		if len(msg.set.Columns) > 0 {
			if reopened, err := m.s.open(); err == nil {
				m.l = reopened
			}
		}
		if !stale || !m.batchIsOwnAct(msg.set) {
			if !stale {
				m.message = nil
			}
			m.status = statusParts{
				time:   m.s.r.T("view.watch.updated", "time", now().Format(watchClock)),
				change: m.s.changeText(msg.set),
			}
			cmds = append(cmds, m.reread())
		}
	default:
		m.cursor = msg.set.Cursor
		if !stale && !m.expiry.IsZero() && !now().Before(m.expiry) {
			m.status.time = m.s.r.T("view.watch.updated", "time", now().Format(watchClock))
			cmds = append(cmds, m.reread())
		}
	}
	return tea.Batch(append(cmds, m.waitForChange())...)
}

// batchIsOwnAct reports whether every event a change batch carries is
// explained by the head's own last act, which is the only case a stale
// report is nothing but an echo of the act just made. It is decided by the
// specific card and revision that act's own response named, in ownActRef and
// ownActRevision, rather than by identity: several sessions on this
// workbench routinely share one actor, harness, provider and model, so a
// genuine change from a different session carrying that same identity would
// be misattributed as an echo if identity alone decided it, reopening the
// very bug this check exists to close. A batch reporting no events, or one
// following an act whose response named no card to compare against, is not
// attributable to anything named here, so it answers false rather than
// vacuously true.
func (m *interactiveModel) batchIsOwnAct(set *verb.ChangeSet) bool {
	if len(set.Events) == 0 || m.ownActRef == "" {
		return false
	}
	for _, event := range set.Events {
		if event.Ref != m.ownActRef {
			return false
		}
	}
	for _, card := range set.Cards {
		if card.Ref == m.ownActRef && card.Revision != m.ownActRevision {
			return false
		}
	}
	return true
}

// recordOwnAct keeps what a successful act just wrote: the card its response
// named and the revision it left there. batchIsOwnAct compares a later stale
// batch against exactly this, which is what lets it tell the head's own act
// apart from a different session's act sharing its identity. Cleared to
// nothing identifiable when the response names no card, which is the
// signal batchIsOwnAct's own default of "not mine" answers to.
func (m *interactiveModel) recordOwnAct(response *verb.Response) {
	m.ownActRef, m.ownActRevision = "", ""
	if response.Card != nil {
		m.ownActRef, m.ownActRevision = response.Card.Ref, response.Card.Revision
	}
}

// errorLines composes what an error from a read reads as in the message
// area, the way reportError composes it for standard error.
func (m *interactiveModel) errorLines(err error) []string {
	return m.errorLinesFor(err, m.s.command)
}

// errorLinesFor is errorLines composed by a copy of the session naming the
// command that performs the same read on the command line, because a
// refusal's sentence may name the command: a filter's query is refused as
// dinah query would refuse it, not as dinah tui.
func (m *interactiveModel) errorLinesFor(err error, command string) []string {
	refusal, ok := err.(*contract.Refusal)
	if !ok {
		return []string{contract.OutcomeUnreachable + " " + err.Error()}
	}
	composer := *m.s
	composer.command = command
	return composer.composeRefusal(composer.nameTheWorkbench(refusal))
}

// reread asks the library to read the view again, off the event loop, tagged
// with a fresh read generation: an act's own redraw and the watcher's redraw
// of a change somebody else made both go through here, and both are the
// board-wide read the profile measured at 400 ms, which must not hold the
// keyboard or the drawing for that long either way. A key that arrives before
// this lands is queued in Update's keyMsg case rather than dropped or
// processed against data this read is about to replace, and handleViewRead
// replays the queue once a current answer lands, which is what keeps the
// ordering an act's own reread exists for: the next key, if it is another
// act, sees this read's fresh state before it runs, just later than the same
// call used to make it happen, and without blocking. An answer superseded by
// a later read is dropped rather than applied.
func (m *interactiveModel) reread() tea.Cmd {
	m.readSeq++
	seq := m.readSeq
	drawn := *m.req
	l, actor, filter := m.l, m.req.Actor, m.filter
	return m.guarded(func() tea.Msg {
		if interactiveSeam != nil && interactiveSeam.readGate != nil {
			interactiveSeam.readGate()
		}
		answer, err := l.DrawView(&drawn)
		if err != nil {
			return viewReadMsg{seq: seq, err: err}
		}
		var matches map[string]bool
		if filter != "" {
			if found, err := l.Query(&verb.Request{Verb: "query", Actor: actor, Query: filter}); err == nil {
				matches = map[string]bool{}
				for _, card := range found.Cards {
					matches[card.Ref] = true
				}
			}
		}
		return viewReadMsg{seq: seq, answer: answer, matches: matches}
	})
}

// handleViewRead applies a read of the view answered off the event loop, once
// it is still current: reapplies the filter's matches where the read asked
// for them, rebuilds the lanes and keeps the selection by section 8.3's
// rules, then recomputes the offer and the detail pane. Where this read is
// the one afterLine dispatched for a line just run, pendingLine still names
// it, and its lineDone notice, if the head is under test, fires once the
// board is fresh and the queue is about to be drained, whether the read
// landed clean or with an error: a caller waiting on that notice to know the
// line is fully settled must be told either way, or it waits forever. Either
// way, current or superseded, it marks this generation answered before
// draining the queue, since a superseded answer still means whatever asked
// for it is no longer the latest thing waiting to happen; the newer read
// that superseded it is what the queue is actually waiting on, and readSeq
// already reflects that.
func (m *interactiveModel) handleViewRead(msg viewReadMsg) tea.Cmd {
	if msg.seq != m.readSeq {
		return nil
	}
	m.readAnswered = msg.seq
	if msg.err != nil {
		m.message = m.errorLines(msg.err)
	} else {
		if m.filter != "" && msg.matches != nil {
			m.matches = msg.matches
		}
		m.applyAnswer(msg.answer, false)
	}
	if pending := m.pendingLine; pending != nil {
		m.pendingLine = nil
		if interactiveSeam != nil && interactiveSeam.lineDone != nil {
			interactiveSeam.lineDone(pending)
		}
	}
	return m.drainQueue()
}

// drainQueue replays the keys and the pastes queued while a read was in
// flight, in the order they arrived, through the ordinary dispatch. Replayed
// input can itself start another read, which leaves readSeq and readAnswered
// apart again; the moment that happens, draining stops and whatever is left
// queued waits for that read to land in its turn, rather than running ahead
// of state it would see stale.
func (m *interactiveModel) drainQueue() tea.Cmd {
	var cmds []tea.Cmd
	for len(m.queued) > 0 {
		next := m.queued[0]
		m.queued = m.queued[1:]
		if next.isPaste {
			cmds = append(cmds, m.paste(next.paste))
		} else {
			cmds = append(cmds, m.key(next.key))
		}
		if m.readSeq != m.readAnswered {
			break
		}
	}
	return tea.Batch(cmds...)
}

// query runs a filter's query and answers the references it matched.
func (m *interactiveModel) query(text string) (map[string]bool, error) {
	found, err := m.l.Query(&verb.Request{Verb: "query", Actor: m.req.Actor, Query: text})
	if err != nil {
		return nil, err
	}
	matches := map[string]bool{}
	for _, card := range found.Cards {
		matches[card.Ref] = true
	}
	return matches, nil
}

// applyAnswer takes a read of the view, rebuilds the lanes from it, and puts
// the focus and the selection where section 8.3 says. first starts them as
// section 4.1 says instead.
func (m *interactiveModel) applyAnswer(answer *verb.ViewAnswer, first bool) {
	previousKey, previousRef, previousIndex, previousFocus := "", "", m.selected, m.focus
	if lane, ok := m.focusedLane(); ok {
		previousKey = lane.key()
		if card, ok := m.selectedCard(); ok {
			previousRef = card.Ref
		}
	}
	m.answer = answer
	m.expiry = earliestExpiry(answer)
	m.lanes = m.buildLanes(answer)
	m.pullCache = map[string]bool{}
	switch {
	case len(m.lanes) == 0:
		m.focus, m.selected = 0, 0
	case first:
		m.focus, m.selected = m.firstLane(answer), 0
	default:
		m.keepSelection(previousKey, previousRef, previousIndex, previousFocus)
	}
	m.afterRead()
}

// firstLane is the lane focus starts on: the first whose column the view does
// not collapse, or the first lane where none qualifies or the layout is list.
func (m *interactiveModel) firstLane(answer *verb.ViewAnswer) int {
	if answer.View.Layout != bench.ViewLayoutColumns {
		return 0
	}
	for i, lane := range m.lanes {
		if !slices.Contains(answer.View.Collapsed, lane.column) {
			return i
		}
	}
	return 0
}

// keepSelection applies section 8.3: the same lane keeps focus where it
// still exists, with the same card where it is still there and the same
// index where it is not, and otherwise the lane now at the old position has
// focus with its first card selected.
func (m *interactiveModel) keepSelection(previousKey, previousRef string, previousIndex, previousFocus int) {
	for i, lane := range m.lanes {
		if lane.key() != previousKey {
			continue
		}
		m.focus = i
		for j, card := range lane.cards {
			if card.Ref == previousRef {
				m.selected = j
				return
			}
		}
		m.selected = min(previousIndex, max(len(lane.cards)-1, 0))
		return
	}
	m.focus = min(previousFocus, len(m.lanes)-1)
	m.selected = 0
}

// afterRead brings everything that depends on the read up to date: the card
// shown in card mode, the offer, an open menu's rows, item mode's rows and
// the detail pane.
func (m *interactiveModel) afterRead() {
	if m.cardOpen && !m.showCard(m.cardRef, m.cardInView) {
		m.leaveCard()
	}
	m.recomputeOffer()
	if m.mode == modeMenu {
		m.refreshMenu()
	}
	m.refreshItems()
	m.refreshDetail()
}

// buildLanes turns a read of the view into lanes, keeping only the cards the
// filter matched where one is set.
func (m *interactiveModel) buildLanes(answer *verb.ViewAnswer) []laneEntry {
	body := answer.View
	many := len(body.Sections) > 1
	var lanes []laneEntry
	for i := range body.Sections {
		section := body.Sections[i]
		cards := m.kept(section.Cards)
		if body.Layout != bench.ViewLayoutColumns {
			lane := laneEntry{section: i, title: withoutControls(section.Title), cards: cards}
			if section.Refused != "" {
				lane.refused = &body.Sections[i]
			}
			lanes = append(lanes, lane)
			continue
		}
		for _, lane := range m.columnLanes(i, section, cards) {
			if many {
				lane.title = m.s.r.T("interactive.lane.section", "section", withoutControls(section.Title), "column", lane.title)
			}
			lanes = append(lanes, lane)
		}
	}
	return lanes
}

// kept is the cards the filter keeps, all of them where no filter is set.
func (m *interactiveModel) kept(cards []verb.CardView) []verb.CardView {
	if m.filter == "" || m.matches == nil {
		return cards
	}
	var kept []verb.CardView
	for _, card := range cards {
		if m.matches[card.Ref] {
			kept = append(kept, card)
		}
	}
	return kept
}

// columnLanes is one section's lanes in the columns layout: one per column
// holding at least one of its cards, in the flow's order, then the columns
// the flow no longer lists in byte order of their identifier, which is
// sectionColumns' own order. A collapsed column is a lane like any other.
func (m *interactiveModel) columnLanes(index int, section verb.ViewSectionAnswer, cards []verb.CardView) []laneEntry {
	byColumn := map[string][]verb.CardView{}
	for _, card := range cards {
		byColumn[card.Column] = append(byColumn[card.Column], card)
	}
	var lanes []laneEntry
	for _, column := range m.l.Bench.Columns {
		held := byColumn[column.ID]
		if len(held) == 0 {
			continue
		}
		lanes = append(lanes, laneEntry{section: index, column: column.ID, title: withoutControls(column.Title), operator: column.OperatorOwned, cards: held})
	}
	var unlisted []string
	for id := range byColumn {
		if m.l.Bench.Column(id) == nil {
			unlisted = append(unlisted, id)
		}
	}
	slices.Sort(unlisted)
	for _, id := range unlisted {
		held := byColumn[id]
		title := held[0].ColumnTitle
		if title == "" {
			title = id
		}
		lanes = append(lanes, laneEntry{section: index, column: id, title: withoutControls(title), cards: held})
	}
	return lanes
}

// focusedLane answers the lane with the focus.
func (m *interactiveModel) focusedLane() (laneEntry, bool) {
	if m.focus < 0 || m.focus >= len(m.lanes) {
		return laneEntry{}, false
	}
	return m.lanes[m.focus], true
}

// selectedCard answers the selected card of the focused lane.
func (m *interactiveModel) selectedCard() (verb.CardView, bool) {
	lane, ok := m.focusedLane()
	if !ok || m.selected < 0 || m.selected >= len(lane.cards) {
		return verb.CardView{}, false
	}
	return lane.cards[m.selected], true
}

// cardInLanes answers the card with a reference as the most recent read drew
// it, wherever in the view's lanes it stands.
func (m *interactiveModel) cardInLanes(ref string) (verb.CardView, bool) {
	for _, lane := range m.lanes {
		for _, card := range lane.cards {
			if card.Ref == ref {
				return card, true
			}
		}
	}
	return verb.CardView{}, false
}

// target answers the card the act keys act on, and the basis of that act:
// the revision of the card from the read that produced the offer.
func (m *interactiveModel) target() (ref, basis string, ok bool) {
	if m.cardOpen {
		if card, found := m.cardInLanes(m.cardRef); found {
			return card.Ref, card.Revision, true
		}
		return m.cardRef, m.cardBasis, m.cardRef != ""
	}
	card, found := m.selectedCard()
	if !found {
		return "", "", false
	}
	return card.Ref, card.Revision, true
}

// identity is a request carrying the identity the command line's own request
// carries, and nothing else.
func (m *interactiveModel) identity(name string) *verb.Request {
	return &verb.Request{
		Verb:     name,
		Actor:    m.req.Actor,
		Harness:  m.req.Harness,
		Provider: m.req.Provider,
		Model:    m.req.Model,
		Server:   m.req.Server,
	}
}

// recomputeOffer asks the library what may be offered for the target card,
// and, where no card is targeted, what may be offered on the workbench and in
// the focused lane's column.
//
// This runs synchronously, deliberately: OfferActs answers one card and is
// cheap next to a board-wide read, and Bubble Tea gives no guarantee that an
// asynchronous command's answer lands before the very next key, or before a
// quit already in flight, so making this asynchronous raced the offer itself
// stale against the input that is supposed to have produced it. reread, which
// asks for the whole board, is where the read that actually took 400 ms in
// the profile lives, and that one runs off the loop; see reread.
//
// It is asked fresh on every call rather than cached against the card
// reference for the read now on screen, on purpose: whether the offer runs is
// not only a function of the card's own revision, which a read does capture,
// but also of the clock, since a claim's lapse depends on it, and a card
// whose revision has not changed can still lapse between two selections of
// the same card within one read. A cache keyed by reference answered the
// claim a lapsed hold's owner no longer held with the offer as it stood
// before the lapse; dinah-636's own suite caught it.
//
// The pull check is a different matter: whether pulling the next ready card
// into a column would succeed depends only on that column, which is the
// focused lane's own, and never on the card being asked about here, so it is
// left out of this call's own Column and answered instead through
// pulledInto, which keeps the one answer for every card asked about in the
// same lane in the same read.
func (m *interactiveModel) recomputeOffer() {
	m.offer = interactiveOffer{}
	ref, _, ok := m.target()
	if !ok {
		m.offerWithoutCard()
		return
	}
	asking := m.identity(verb.Move)
	asking.Card = ref
	asking.Archived = m.cardOpen && m.cardArchived
	offered, err := m.l.OfferActs(asking)
	if err != nil || offered == nil {
		return
	}
	if column := m.pullColumn(); column != nil && !asking.Archived {
		offered.Pull = m.pulledInto(column.Ref())
	}
	m.offer = interactiveOffer{
		ref:     ref,
		claim:   offered.Claim,
		release: offered.Release,
		comment: offered.Comment,
		moves:   offered.Moves,
		acts:    offered,
		edit:    offered.Edit && m.editorResolves(),
	}
	for i := range offered.Moves {
		row := &offered.Moves[i]
		if row.OnRoute && m.offer.forward == nil {
			m.offer.forward = row
			if column := m.l.Bench.Column(row.Column); column != nil {
				m.offer.forwardTerminal = column.Terminal()
			}
		}
		if row.Reject && m.offer.back == nil {
			m.offer.back = row
		}
	}
}

// pulledInto answers, from pullCache where the column was already asked
// about in the read now on screen, whether pulling the next ready card into
// it would succeed. It is asked fresh once per column per read: a mutation
// that could change the answer, someone moving a card into or out of the
// column, always produces a fresh read of its own before the head could ask
// again, so the cached answer never outlives the state it was computed from.
func (m *interactiveModel) pulledInto(columnRef string) bool {
	if cached, found := m.pullCache[columnRef]; found {
		return cached
	}
	asking := m.identity(verb.Move)
	asking.Column = columnRef
	pulled, err := m.l.OfferPull(asking)
	if err != nil {
		pulled = false
	}
	if m.pullCache != nil {
		m.pullCache[columnRef] = pulled
	}
	return pulled
}

// offerWithoutCard asks the library what may be offered where no card is
// targeted, which is a filing and a pull into the focused lane's column.
func (m *interactiveModel) offerWithoutCard() {
	asking := m.identity(verb.Move)
	offered, err := m.l.OfferActs(asking)
	if err != nil || offered == nil {
		return
	}
	pull := false
	if column := m.pullColumn(); column != nil {
		pull = m.pulledInto(column.Ref())
	}
	m.offer = interactiveOffer{acts: &verb.OfferedActs{Add: offered.Add, Pull: pull}}
}

// refreshDetail renders the detail pane for the selected card, at a window
// wide enough to hold it. As recomputeOffer, this runs synchronously: Show on
// one card is cheap next to reread's board-wide read, and the same race
// against the next key or a quit applies here too.
func (m *interactiveModel) refreshDetail() {
	m.detail = nil
	if m.width < interactiveWideWidth || m.tooSmall() {
		return
	}
	m.detail = []string{}
	card, ok := m.selectedCard()
	if !ok {
		return
	}
	width := m.draw() - interactiveListWidth(m.draw()) - 1
	m.detail = m.showLines(card.Ref, "card,body,checklist", width)
}

// showLines is what dinah show prints for a card in the human form, with the
// fields given, laid out at width and cleaned line by line. A card the
// library will not show answers no lines.
func (m *interactiveModel) showLines(ref, fields string, width int) []string {
	lines, _ := m.shown(ref, fields, width)
	return lines
}

// shown is showLines with the revision the answer carried, and false where
// the library would not show the card. A read naming the checklist asks for
// the unresolved items alone, which is what the detail pane lists.
func (m *interactiveModel) shown(ref, fields string, width int) ([]string, string) {
	asking := m.identity("show")
	asking.Card = ref
	asking.Fields = fields
	asking.Unresolved = strings.Contains(fields, "checklist")
	asking.Archived = m.cardArchived && ref == m.cardRef
	detail, _, _, _, err := m.l.Show(asking)
	if err != nil || detail == nil {
		return nil, ""
	}
	var cleaned []string
	for _, line := range m.s.detailText(detail, width) {
		cleaned = append(cleaned, withoutControls(line))
	}
	return cleaned, detail.Card.Revision
}

// showCard opens card mode's content on a card, answering false where the
// card no longer resolves.
func (m *interactiveModel) showCard(ref string, inView bool) bool {
	lines, revision := m.shown(ref, "", m.draw())
	if lines == nil {
		return false
	}
	offset := m.viewport.YOffset()
	m.cardRef, m.cardLines, m.cardBasis, m.cardInView = ref, lines, revision, inView
	m.viewport.SetContentLines(lines)
	m.viewport.SetYOffset(offset)
	return true
}

// refreshMenu recomputes the open menu's rows from the fresh offer, keeping
// the highlight on the same value where that row is still offered, and
// closes the menu, ending any act it was asking for, where no row is left.
func (m *interactiveModel) refreshMenu() {
	menu := m.menu
	if menu == nil || menu.refresh == nil {
		return
	}
	rows, ok := menu.refresh()
	if !ok {
		if m.pending != nil {
			m.endPending()
			return
		}
		m.menu = nil
		m.mode = menu.back
		if m.mode == modeCard && !m.cardOpen {
			m.mode = modeBrowse
		}
		return
	}
	highlighted := ""
	if menu.highlight < len(menu.rows) {
		highlighted = menu.rows[menu.highlight].value
	}
	menu.rows = rows
	menu.highlight = 0
	for i, row := range rows {
		if row.value == highlighted {
			menu.highlight = i
		}
	}
}

// bodyHeight is how many rows the body has, between the rule and the message
// area.
func (m *interactiveModel) bodyHeight() int {
	return max(m.height-3-m.messageHeight()-m.footerHeight(), 0)
}

// areaHeight is the comment prompt's height: the message area's rows less
// the one its label takes.
func (m *interactiveModel) areaHeight() int {
	room := m.height - 3 - interactiveMinimumBody - 1
	return min(max(room, interactiveAreaMinimum), interactiveAreaMaximum) - 1
}

// messageHeight is how many rows the message area takes.
func (m *interactiveModel) messageHeight() int {
	if m.mode == modePrompt {
		if m.prompt == promptText {
			return m.areaHeight() + 1
		}
		if m.prompt == promptStep {
			return 2 + min(len(m.message), interactiveMessageLimit-2)
		}
		return 1 + min(len(m.message), interactiveMessageLimit-1)
	}
	return min(max(len(m.messageLines()), 1), interactiveMessageLimit)
}

// footerHeight is how many rows the footer takes: one, or the full help's
// rows while it is shown, never leaving the body fewer than
// interactiveMinimumBody rows.
func (m *interactiveModel) footerHeight() int {
	if !m.fullHelp || (m.mode != modeBrowse && m.mode != modeCard) {
		return 1
	}
	rows := len(strings.Split(m.footerText(), "\n"))
	room := m.height - 3 - interactiveMinimumBody - m.messageHeight()
	return max(min(rows, room), 1)
}

// messageLines are the lines the message area shows outside a prompt: an
// act's answer or a refusal where one stands, and the live status otherwise.
func (m *interactiveModel) messageLines() []string {
	if len(m.message) > 0 {
		return m.message
	}
	separator := m.s.r.T("view.watch.separator")
	return []string{statusLine(m.status.time, m.status.change, "", separator, m.draw(), m.glyphs.ellipsis)}
}

// key handles one key press.
func (m *interactiveModel) key(pressed tea.KeyPressMsg) tea.Cmd {
	if pressed.String() == "ctrl+c" {
		m.quitting = true
		return tea.Quit
	}
	// ctrl+l draws the whole screen again in every mode, a too-small
	// window included, and changes nothing else.
	if pressed.String() == "ctrl+l" {
		return tea.ClearScreen
	}
	if m.tooSmall() {
		return nil
	}
	tabbed := m.tabbed
	m.tabbed = false
	if m.mode != modePrompt || m.prompt != promptJump || pressed.String() != "tab" {
		m.message = nil
	}
	keys := m.keys()
	switch m.mode {
	case modePrompt:
		return m.promptKey(keys, pressed, tabbed)
	case modeMenu:
		return m.menuKey(keys, pressed)
	case modeCard:
		return m.cardKey(keys, pressed)
	case modeItems:
		return m.itemsKey(keys, pressed)
	case modeOutput:
		return m.outputKey(keys, pressed)
	}
	return m.browseKey(keys, pressed)
}

// keys builds the bindings in the session's language, with the a and b
// labels naming the offer's destinations.
func (m *interactiveModel) keys() *interactiveKeys {
	forward, back := "", ""
	if m.offer.forward != nil {
		forward = withoutControls(m.offer.forward.Title)
	}
	if m.offer.back != nil {
		back = withoutControls(m.offer.back.Title)
	}
	return newInteractiveKeys(m.s.r, interactiveArrow(m.glyphs), forward, back)
}

// browseKey handles a key in browse mode.
func (m *interactiveModel) browseKey(keys *interactiveKeys, pressed tea.KeyPressMsg) tea.Cmd {
	lane, _ := m.focusedLane()
	switch {
	case key.Matches(pressed, keys.up):
		m.selectCard(m.selected - 1)
	case key.Matches(pressed, keys.down):
		m.selectCard(m.selected + 1)
	case key.Matches(pressed, keys.left):
		m.focusLane(m.focus - 1)
	case key.Matches(pressed, keys.right):
		m.focusLane(m.focus + 1)
	case key.Matches(pressed, keys.page):
		step := max(m.bodyHeight()-1, 1)
		if pressed.String() == "pgup" {
			step = -step
		}
		m.selectCard(min(max(m.selected+step, 0), len(lane.cards)-1))
	case key.Matches(pressed, keys.ends):
		if pressed.String() == "home" {
			m.selectCard(0)
		} else {
			m.selectCard(len(lane.cards) - 1)
		}
	case key.Matches(pressed, keys.show):
		if card, ok := m.selectedCard(); ok {
			m.openCard(card.Ref, true)
		}
	case key.Matches(pressed, keys.jump):
		return m.openPrompt(promptJump, "")
	case key.Matches(pressed, keys.filter):
		return m.openPrompt(promptFilter, m.filter)
	case key.Matches(pressed, keys.more):
		m.fullHelp = !m.fullHelp
	case key.Matches(pressed, keys.quit):
		m.quitting = true
		return tea.Quit
	default:
		return m.actKey(keys, pressed)
	}
	return nil
}

// itemsKey handles a key in item mode: the highlight keys, the letters of
// the item acts offered on the highlighted item, and the keys that close it.
// A letter whose act is not offered on the highlighted item does nothing and
// writes nothing, and the digits choose nothing.
func (m *interactiveModel) itemsKey(keys *interactiveKeys, pressed tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(pressed, keys.menuUp):
		m.itemHighlight = max(m.itemHighlight-1, 0)
		return nil
	case key.Matches(pressed, keys.menuDown):
		m.itemHighlight = min(m.itemHighlight+1, max(len(m.itemRows)-1, 0))
		return nil
	case key.Matches(pressed, keys.page):
		step := max(m.bodyHeight()-2, 1)
		if pressed.String() == "pgup" {
			step = -step
		}
		m.itemHighlight = min(max(m.itemHighlight+step, 0), max(len(m.itemRows)-1, 0))
		return nil
	case key.Matches(pressed, keys.ends):
		if pressed.String() == "home" {
			m.itemHighlight = 0
		} else {
			m.itemHighlight = max(len(m.itemRows)-1, 0)
		}
		return nil
	case key.Matches(pressed, keys.menuCancel), key.Matches(pressed, keys.menuQuit):
		m.closeItems()
		return nil
	case key.Matches(pressed, keys.jump):
		return m.openPrompt(promptJump, "")
	}
	for _, row := range interactiveActs {
		if row.mode != actItems || !key.Matches(pressed, row.binding(keys)) {
			continue
		}
		if !row.offered(m) {
			return nil
		}
		return m.startKeyAct(row)
	}
	return nil
}

// outputKey handles a key in output mode.
func (m *interactiveModel) outputKey(keys *interactiveKeys, pressed tea.KeyPressMsg) tea.Cmd {
	room := max(m.bodyHeight()-1, 1)
	last := max(len(m.output)-room, 0)
	switch {
	case key.Matches(pressed, keys.scrollUp):
		m.outputOffset = max(m.outputOffset-1, 0)
	case key.Matches(pressed, keys.scrollDown):
		m.outputOffset = min(m.outputOffset+1, last)
	case key.Matches(pressed, keys.page):
		if pressed.String() == "pgup" {
			m.outputOffset = max(m.outputOffset-room, 0)
		} else {
			m.outputOffset = min(m.outputOffset+room, last)
		}
	case key.Matches(pressed, keys.ends):
		if pressed.String() == "home" {
			m.outputOffset = 0
		} else {
			m.outputOffset = last
		}
	case key.Matches(pressed, keys.outputClose):
		m.closeOutput()
	case key.Matches(pressed, keys.jump):
		m.closeOutput()
		return m.openPrompt(promptJump, "")
	}
	return nil
}

// closeOutput leaves output mode for the mode it was opened from.
func (m *interactiveModel) closeOutput() {
	m.output, m.outputTitle, m.outputOffset = nil, "", 0
	m.mode = m.outputBack
	if m.mode == modeCard && !m.cardOpen {
		m.mode = modeBrowse
	}
	if m.mode == modeItems && len(m.itemRows) == 0 {
		m.mode = m.returnMode()
	}
}

// cardKey handles a key in card mode.
func (m *interactiveModel) cardKey(keys *interactiveKeys, pressed tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(pressed, keys.scrollUp):
		m.viewport.ScrollUp(1)
	case key.Matches(pressed, keys.scrollDown):
		m.viewport.ScrollDown(1)
	case key.Matches(pressed, keys.page):
		if pressed.String() == "pgup" {
			m.viewport.PageUp()
		} else {
			m.viewport.PageDown()
		}
	case key.Matches(pressed, keys.ends):
		if pressed.String() == "home" {
			m.viewport.GotoTop()
		} else {
			m.viewport.GotoBottom()
		}
	case key.Matches(pressed, keys.back):
		m.leaveCard()
		m.recomputeOffer()
		m.refreshDetail()
	case key.Matches(pressed, keys.jump):
		return m.openPrompt(promptJump, "")
	case key.Matches(pressed, keys.more):
		m.fullHelp = !m.fullHelp
	case key.Matches(pressed, keys.quit):
		m.quitting = true
		return tea.Quit
	default:
		return m.actKey(keys, pressed)
	}
	return nil
}

// actKey handles an act key in browse or card mode: a row of the act table
// whose key it is, then the items and actions keys, then a key binding of the
// user's. An act the offer does not allow does nothing and writes nothing.
func (m *interactiveModel) actKey(keys *interactiveKeys, pressed tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(pressed, keys.items):
		if len(m.offeredItems()) > 0 {
			m.openItems()
		}
		return nil
	case key.Matches(pressed, keys.actions):
		return m.openActions()
	}
	for _, row := range interactiveActs {
		if row.mode != actBrowse || row.name == "show" || row.name == "query" || row.name == "view" {
			continue
		}
		if !key.Matches(pressed, row.binding(keys)) {
			continue
		}
		if !row.offered(m) {
			continue
		}
		return m.startKeyAct(row)
	}
	return m.bindingKey(pressed)
}

// menuHighlight is the highlighted row of the open menu, 0 where none is
// open.
func (m *interactiveModel) menuHighlight() int {
	if m.menu == nil {
		return 0
	}
	return m.menu.highlight
}

// openMenu opens a menu, with its first row highlighted.
func (m *interactiveModel) openMenu(menu *interactiveMenuState) {
	m.menu = menu
	m.mode = modeMenu
}

// menuKey handles a key while a menu is open.
func (m *interactiveModel) menuKey(keys *interactiveKeys, pressed tea.KeyPressMsg) tea.Cmd {
	menu := m.menu
	if menu == nil {
		m.mode = m.returnMode()
		return nil
	}
	switch {
	case key.Matches(pressed, keys.menuUp):
		menu.highlight = max(menu.highlight-1, 0)
	case key.Matches(pressed, keys.menuDown):
		menu.highlight = min(menu.highlight+1, len(menu.rows)-1)
	case key.Matches(pressed, keys.page):
		step := max(m.bodyHeight()-2, 1)
		if pressed.String() == "pgup" {
			step = -step
		}
		menu.highlight = min(max(menu.highlight+step, 0), len(menu.rows)-1)
	case key.Matches(pressed, keys.ends):
		if pressed.String() == "home" {
			menu.highlight = 0
		} else {
			menu.highlight = len(menu.rows) - 1
		}
	case key.Matches(pressed, keys.menuNumber):
		chosen := int(pressed.Code - '1')
		if chosen < len(menu.rows) {
			return menu.choose(menu.rows[chosen])
		}
	case key.Matches(pressed, keys.menuChoose):
		if menu.highlight < len(menu.rows) {
			return menu.choose(menu.rows[menu.highlight])
		}
	case key.Matches(pressed, keys.menuCancel), key.Matches(pressed, keys.menuQuit):
		if m.pending != nil {
			m.endPending()
			return nil
		}
		m.menu = nil
		m.mode = menu.back
		if m.mode == modeCard && !m.cardOpen {
			m.mode = modeBrowse
		}
	}
	return nil
}

// returnMode is the mode the menu and the prompts close back into: card mode
// where they were opened over a card, browse mode otherwise.
func (m *interactiveModel) returnMode() interactiveMode {
	if m.cardOpen {
		return modeCard
	}
	return modeBrowse
}

// openCard opens card mode on a card, doing nothing where the library will
// not show it. inView says whether the card stands in a lane of the view.
func (m *interactiveModel) openCard(ref string, inView bool) {
	m.viewport.SetYOffset(0)
	if !m.showCard(ref, inView) {
		return
	}
	m.viewport.GotoTop()
	m.cardOpen = true
	m.mode = modeCard
	m.recomputeOffer()
}

// leaveCard closes card mode and returns to browse.
func (m *interactiveModel) leaveCard() {
	m.cardOpen = false
	m.cardArchived = false
	m.cardRef, m.cardLines, m.cardBasis = "", nil, ""
	if m.mode == modeCard {
		m.mode = modeBrowse
	}
}

// selectCard selects a card of the focused lane by index, doing nothing
// outside the lane.
func (m *interactiveModel) selectCard(index int) {
	lane, ok := m.focusedLane()
	if !ok || index < 0 || index >= len(lane.cards) {
		return
	}
	m.selected = index
	m.recomputeOffer()
	m.refreshDetail()
}

// focusLane focuses a lane by index with its first card selected, doing
// nothing past either end.
func (m *interactiveModel) focusLane(index int) {
	if index < 0 || index >= len(m.lanes) {
		return
	}
	m.focus, m.selected = index, 0
	m.recomputeOffer()
	m.refreshDetail()
}

// act performs one act on the target card through the library, shows its
// answer, and reads the view again off the event loop. Every act bumps the
// watch epoch and records itself, whatever its outcome, because every act
// dispatches this same reread and that reread already reads the board as it
// now stands: a wait already in flight, which goes on to answer a change with
// nothing this act's own read has not already picked up, is recognised as one
// the head has redrawn for regardless, and its own refresh is skipped rather
// than run a second time on top of this one and, for a refusal, wiping the
// message this call just composed out from under it. A refusal still leaves
// response.Card naming the card as it now stands, which is what
// batchIsOwnAct compares a later batch against.
func (m *interactiveModel) act(name string, row *verb.LegalMove) tea.Cmd {
	ref, basis, ok := m.target()
	if !ok {
		return nil
	}
	asking := m.identity(name)
	asking.Card = ref
	asking.Basis = basis
	if row != nil {
		asking.Column = row.Column
	}
	response := m.l.Do(asking)
	m.answered(response, name, ref, row)
	m.watchEpoch++
	m.recordOwnAct(response)
	if response.Outcome == contract.OutcomeOK && m.cardOpen {
		m.leaveCard()
		m.mode = modeBrowse
	}
	return m.reread()
}

// answered shows an act's answer in the message area. A refusal is composed
// by a copy of the session naming the command that performs the same act on
// the command line, because a refusal's sentence may name the command, so the
// lines are the ones that command writes to standard error.
func (m *interactiveModel) answered(response *verb.Response, name, ref string, row *verb.LegalMove) {
	if response.Outcome != contract.OutcomeOK {
		composer := *m.s
		composer.command = name
		m.message = m.cleaned(composer.outcomeLines(response))
		return
	}
	switch {
	case name == verb.Claim:
		m.message = []string{withoutControls(m.s.r.T("interactive.acted.claim", "ref", ref))}
	case name == verb.Release:
		m.message = []string{withoutControls(m.s.r.T("interactive.acted.release", "ref", ref))}
	case name == verb.Move && row != nil:
		m.message = []string{withoutControls(m.s.r.T("interactive.acted.move", "ref", ref, "column", row.Title))}
	default:
		m.message = []string{withoutControls(m.s.r.T("interactive.acted.comment", "ref", ref))}
	}
}

// cleaned is lines with every control character replaced.
func (m *interactiveModel) cleaned(lines []string) []string {
	var out []string
	for _, line := range lines {
		out = append(out, withoutControls(line))
	}
	return out
}

// openPrompt opens a prompt, holding text.
func (m *interactiveModel) openPrompt(kind promptKind, text string) tea.Cmd {
	m.promptBack = m.returnMode()
	if m.mode == modeItems {
		m.promptBack = modeItems
	}
	m.prompt = kind
	m.mode = modePrompt
	if kind == promptText {
		m.area.Reset()
		m.area.SetHeight(m.areaHeight())
		return m.area.Focus()
	}
	m.input.Prompt = ": "
	switch kind {
	case promptFilter:
		m.input.Prompt = "/ "
	case promptStep:
		m.input.Prompt = "> "
	}
	m.input.SetValue(text)
	m.input.CursorEnd()
	return m.input.Focus()
}

// closePrompt closes the prompt, keeping no text.
func (m *interactiveModel) closePrompt() {
	m.input.Blur()
	m.input.Reset()
	m.area.Blur()
	m.area.Reset()
	m.mode = m.returnMode()
	if m.promptBack == modeItems && len(m.itemRows) > 0 {
		m.mode = modeItems
	}
}

// promptKey handles a key while a prompt is open. Up and Down at the command
// line are left unbound, for dinah-629's history.
func (m *interactiveModel) promptKey(keys *interactiveKeys, pressed tea.KeyPressMsg, tabbed bool) tea.Cmd {
	if key.Matches(pressed, keys.promptCancel) {
		if m.pending != nil {
			m.endPending()
			return nil
		}
		m.closePrompt()
		return nil
	}
	if m.prompt == promptText {
		if key.Matches(pressed, keys.promptPost) {
			return m.answerStep(m.area.Value())
		}
		var cmd tea.Cmd
		m.area, cmd = m.area.Update(pressed)
		return cmd
	}
	if key.Matches(pressed, keys.promptGo) {
		return m.submit()
	}
	if m.prompt == promptJump {
		switch pressed.String() {
		case "tab":
			m.complete(tabbed)
			return nil
		case "up", "down":
			return nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(pressed)
	return cmd
}

// submit handles enter in the command line, the filter and a step's line
// prompt. On a terminal that has not shown it marks pastes, it first asks the
// reader to discard the input already waiting, and drops every event until
// the reader confirms.
func (m *interactiveModel) submit() tea.Cmd {
	if m.reader != nil && !m.reader.SawPaste() {
		m.flushAsked = true
		m.reader.RequestFlush()
	}
	if m.prompt == promptStep {
		return m.answerStep(m.input.Value())
	}
	text := strings.TrimSpace(m.input.Value())
	if m.prompt == promptFilter {
		m.applyFilter(text)
		return nil
	}
	return m.submitLine(text)
}

// applyFilter sets the filter to text, or clears it for an empty text. A
// query the library refuses leaves the filter as it was, shows the refusal,
// and keeps the prompt open with the text as typed.
func (m *interactiveModel) applyFilter(text string) {
	if text == "" {
		m.filter, m.matches = "", nil
		m.closePrompt()
		m.rebuild()
		return
	}
	matches, err := m.query(text)
	if err != nil {
		m.message = m.cleaned(m.errorLinesFor(err, "query"))
		return
	}
	m.filter, m.matches = text, matches
	m.closePrompt()
	m.rebuild()
}

// rebuild rebuilds the lanes from the last read, keeping the selection.
func (m *interactiveModel) rebuild() {
	m.applyAnswer(m.answer, false)
}

// jumpTo resolves the jump prompt's text: a view name first, then a card,
// then a column.
func (m *interactiveModel) jumpTo(text string) {
	if listing, err := m.l.ListViews(m.req); err == nil {
		for _, row := range listing.Views {
			if row.Used && row.Name == text {
				m.jumpToView(text)
				return
			}
		}
	}
	if found, err := m.l.Bench.ResolveCard(text); err == nil && found != nil {
		m.jumpToCard(found.Card.Ref(m.l.Bench.Slug))
		return
	}
	if column := m.l.Bench.ColumnByRef(text); column != nil {
		for i, lane := range m.lanes {
			if lane.column == column.ID {
				m.leaveCard()
				m.mode = modeBrowse
				m.focusLane(i)
				return
			}
		}
		m.message = []string{withoutControls(m.s.r.T("interactive.jump.no-lane", "column", column.Title))}
		return
	}
	if found, err := m.l.Bench.ResolveEntityIn(bench.ArchivedHalf, text); err == nil && found.Kind == bench.KindCard && found.Card != nil {
		m.openArchivedCard(found.Card.Ref(m.l.Bench.Slug))
		return
	}
	m.message = []string{withoutControls(m.s.r.T("interactive.line.nothing", "text", text))}
}

// openArchivedCard opens card mode over a card in the archive, read with
// Archived set, so the actions menu can offer restore on it.
func (m *interactiveModel) openArchivedCard(ref string) {
	m.leaveCard()
	m.cardArchived = true
	m.cardRef = ref
	m.viewport.SetYOffset(0)
	if !m.showCard(ref, false) {
		m.cardArchived = false
		return
	}
	m.viewport.GotoTop()
	m.cardOpen = true
	m.mode = modeCard
	m.recomputeOffer()
}

// jumpToView reads another view afresh, clearing the filter.
func (m *interactiveModel) jumpToView(name string) {
	drawn := *m.req
	drawn.View = name
	answer, err := m.l.DrawView(&drawn)
	if err != nil {
		m.message = m.cleaned(m.errorLines(err))
		return
	}
	m.req.View = name
	m.filter, m.matches = "", nil
	m.lanes = nil
	m.leaveCard()
	m.mode = modeBrowse
	m.applyAnswer(answer, true)
}

// jumpToCard selects a card where it stands in a lane of the view, and opens
// card mode on it where it stands in none.
func (m *interactiveModel) jumpToCard(ref string) {
	for i, lane := range m.lanes {
		for j, card := range lane.cards {
			if card.Ref == ref {
				m.leaveCard()
				m.mode = modeBrowse
				m.focus, m.selected = i, j
				m.recomputeOffer()
				m.refreshDetail()
				return
			}
		}
	}
	m.openCard(ref, false)
}

// paste handles one whole paste. Outside a prompt it is discarded whole. The
// command line takes a paste of one line alone, with one trailing line break
// removed, and discards a paste holding more; the filter and a step's line
// prompt join the pasted lines with spaces; the multi-line prompt keeps the
// line breaks.
func (m *interactiveModel) paste(content string) tea.Cmd {
	if m.tooSmall() || m.mode != modePrompt {
		return nil
	}
	if m.prompt == promptJump {
		return m.pasteLine(content)
	}
	if m.prompt == promptText {
		content = strings.ReplaceAll(content, "\r\n", "\n")
		content = strings.ReplaceAll(content, "\r", "\n")
		var cmd tea.Cmd
		m.area, cmd = m.area.Update(tea.PasteMsg{Content: content})
		return cmd
	}
	content = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ", "\t", " ").Replace(content)
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(tea.PasteMsg{Content: content})
	return cmd
}
