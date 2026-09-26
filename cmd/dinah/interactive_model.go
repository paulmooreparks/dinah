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

// interactiveMode is which of the four modes the model is in.
type interactiveMode int

const (
	modeBrowse interactiveMode = iota
	modeCard
	modeMenu
	modePrompt
)

// promptKind is which prompt is open in prompt mode.
type promptKind int

const (
	promptJump promptKind = iota
	promptFilter
	promptComment
)

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
	// changeMsg is what one wait for a change answered.
	changeMsg struct {
		set *verb.ChangeSet
		err error
	}
	// crashMsg says a panic was recovered and the program should quit.
	crashMsg struct{}
	// retryMsg says the pause after a failed wait has passed.
	retryMsg struct{}
	// repaintMsg asks for the whole screen to be drawn again on the next
	// frame, which the console writer sends after a short write.
	repaintMsg struct{}
)

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

// interactiveOffer is the acts the footer and the menu offer for one card,
// with the a and b rows picked out.
type interactiveOffer struct {
	ref                     string
	claim, release, comment bool
	moves                   []verb.LegalMove
	forward, back           *verb.LegalMove
	forwardTerminal         bool
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

	mode       interactiveMode
	cardOpen   bool
	cardRef    string
	cardLines  []string
	cardBasis  string
	cardInView bool
	viewport   viewport.Model
	menuRows   []verb.LegalMove
	highlight  int
	prompt     promptKind
	input      textinput.Model
	area       textarea.Model
	offer      interactiveOffer
	detail     []string
	message    []string
	status     statusParts
	fullHelp   bool
	help       help.Model

	cursor     string
	gate       *changeGate
	flushAsked bool
	minGen     uint64
	quitting   bool
	expiry     time.Time

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
	return m
}

// Init starts the change loop.
func (m *interactiveModel) Init() tea.Cmd {
	return m.waitForChange()
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
// library, for at most watchWait.
func (m *interactiveModel) waitForChange() tea.Cmd {
	cursor, gate, waiter, actor := m.cursor, m.gate, m.waiter, m.req.Actor
	return m.guarded(func() tea.Msg {
		gate.mu.Lock()
		defer gate.mu.Unlock()
		if gate.closed {
			return nil
		}
		set, err := waiter.Changes(&verb.Request{Verb: "changes", Actor: actor, Since: cursor, Wait: true, Timeout: watchWait})
		return changeMsg{set: set, err: err}
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
	case keyMsg:
		if m.flushAsked || msg.gen < m.minGen {
			return m, nil
		}
		return m, m.key(msg.key)
	case pasteMsg:
		if m.flushAsked || msg.gen < m.minGen {
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
// or, after a wait that failed, the pause before it.
func (m *interactiveModel) changed(msg changeMsg) tea.Cmd {
	cmds := []tea.Cmd{m.measure()}
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
		m.message = nil
		m.status = statusParts{
			time:   m.s.r.T("view.watch.updated", "time", now().Format(watchClock)),
			change: m.s.changeText(msg.set),
		}
		m.reread()
	default:
		m.cursor = msg.set.Cursor
		if !m.expiry.IsZero() && !now().Before(m.expiry) {
			m.status.time = m.s.r.T("view.watch.updated", "time", now().Format(watchClock))
			m.reread()
		}
	}
	return tea.Batch(append(cmds, m.waitForChange())...)
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

// reread reads the view again, reapplies the filter, rebuilds the lanes and
// keeps the selection by section 8.3's rules, then recomputes the offer.
func (m *interactiveModel) reread() {
	drawn := *m.req
	answer, err := m.l.DrawView(&drawn)
	if err != nil {
		m.message = m.errorLines(err)
		return
	}
	if m.filter != "" {
		if matches, err := m.query(m.filter); err == nil {
			m.matches = matches
		}
	}
	m.applyAnswer(answer, false)
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
// shown in card mode, the move menu's rows, the offer and the detail pane.
func (m *interactiveModel) afterRead() {
	if m.cardOpen && !m.showCard(m.cardRef, m.cardInView) {
		m.leaveCard()
	}
	m.recomputeOffer()
	if m.mode == modeMenu {
		m.refreshMenu()
	}
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

// recomputeOffer asks the library what may be offered for the target card.
func (m *interactiveModel) recomputeOffer() {
	m.offer = interactiveOffer{}
	ref, _, ok := m.target()
	if !ok {
		return
	}
	asking := m.identity(verb.Move)
	asking.Card = ref
	offered, err := m.l.OfferActs(asking)
	if err != nil || offered == nil {
		return
	}
	m.offer = interactiveOffer{
		ref:     ref,
		claim:   offered.Claim,
		release: offered.Release,
		comment: offered.Comment,
		moves:   offered.Moves,
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

// refreshDetail renders the detail pane for the selected card, at a window
// wide enough to hold it.
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
	m.detail = m.showLines(card.Ref, "card,body", width)
}

// showLines is what dinah show prints for a card in the human form, with the
// fields given, laid out at width and cleaned line by line. A card the
// library will not show answers no lines.
func (m *interactiveModel) showLines(ref, fields string, width int) []string {
	lines, _ := m.shown(ref, fields, width)
	return lines
}

// shown is showLines with the revision the answer carried, and false where
// the library would not show the card.
func (m *interactiveModel) shown(ref, fields string, width int) ([]string, string) {
	asking := m.identity("show")
	asking.Card = ref
	asking.Fields = fields
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

// refreshMenu recomputes the move menu's rows from the fresh offer, keeping
// the highlight on the same column where that row is still offered, and
// closes the menu where the card left the focused lane or no row is left.
func (m *interactiveModel) refreshMenu() {
	ref, _, ok := m.target()
	if !ok || ref != m.offer.ref || len(m.offer.moves) == 0 {
		m.mode = m.returnMode()
		return
	}
	highlighted := ""
	if m.highlight < len(m.menuRows) {
		highlighted = m.menuRows[m.highlight].Column
	}
	m.menuRows = m.offer.moves
	m.highlight = 0
	for i, row := range m.menuRows {
		if row.Column == highlighted {
			m.highlight = i
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
		if m.prompt == promptComment {
			return m.areaHeight() + 1
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
	m.message = nil
	keys := m.keys()
	switch m.mode {
	case modePrompt:
		return m.promptKey(keys, pressed)
	case modeMenu:
		return m.menuKey(keys, pressed)
	case modeCard:
		return m.cardKey(keys, pressed)
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

// actKey handles an act key in browse or card mode. An act the offer does
// not allow does nothing and writes nothing.
func (m *interactiveModel) actKey(keys *interactiveKeys, pressed tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(pressed, keys.claim) && m.offer.claim:
		m.act(verb.Claim, nil)
	case key.Matches(pressed, keys.release) && m.offer.release:
		m.act(verb.Release, nil)
	case key.Matches(pressed, keys.advance) && m.offer.forward != nil:
		m.act(verb.Move, m.offer.forward)
	case key.Matches(pressed, keys.sendBack) && m.offer.back != nil:
		m.act(verb.Move, m.offer.back)
	case key.Matches(pressed, keys.move) && len(m.offer.moves) > 0:
		m.menuRows = m.offer.moves
		m.highlight = 0
		m.mode = modeMenu
	case key.Matches(pressed, keys.comment) && m.offer.comment:
		return m.openPrompt(promptComment, "")
	}
	return nil
}

// menuKey handles a key while the move menu is open.
func (m *interactiveModel) menuKey(keys *interactiveKeys, pressed tea.KeyPressMsg) tea.Cmd {
	switch {
	case key.Matches(pressed, keys.menuUp):
		m.highlight = max(m.highlight-1, 0)
	case key.Matches(pressed, keys.menuDown):
		m.highlight = min(m.highlight+1, len(m.menuRows)-1)
	case key.Matches(pressed, keys.menuNumber):
		chosen := int(pressed.Code - '1')
		if chosen < len(m.menuRows) {
			m.chooseMove(chosen)
		}
	case key.Matches(pressed, keys.menuChoose):
		m.chooseMove(m.highlight)
	case key.Matches(pressed, keys.menuCancel), key.Matches(pressed, keys.menuQuit):
		m.mode = m.returnMode()
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
	m.cardRef, m.cardLines, m.cardBasis = "", nil, ""
	if m.mode == modeCard {
		m.mode = modeBrowse
	}
}

// chooseMove moves the card to the menu's row at index.
func (m *interactiveModel) chooseMove(index int) {
	if index < 0 || index >= len(m.menuRows) {
		return
	}
	row := m.menuRows[index]
	m.mode = m.returnMode()
	m.act(verb.Move, &row)
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
// answer, and reads the view again at once.
func (m *interactiveModel) act(name string, row *verb.LegalMove) {
	ref, basis, ok := m.target()
	if !ok {
		return
	}
	asking := m.identity(name)
	asking.Card = ref
	asking.Basis = basis
	if row != nil {
		asking.Column = row.Column
	}
	response := m.l.Do(asking)
	m.answered(response, name, ref, row)
	if response.Outcome == contract.OutcomeOK && m.cardOpen {
		m.leaveCard()
		m.mode = modeBrowse
	}
	m.reread()
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
	m.prompt = kind
	m.mode = modePrompt
	if kind == promptComment {
		m.area.Reset()
		m.area.SetHeight(m.areaHeight())
		return m.area.Focus()
	}
	m.input.Prompt = ": "
	if kind == promptFilter {
		m.input.Prompt = "/ "
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
}

// promptKey handles a key while a prompt is open.
func (m *interactiveModel) promptKey(keys *interactiveKeys, pressed tea.KeyPressMsg) tea.Cmd {
	if key.Matches(pressed, keys.promptCancel) {
		m.closePrompt()
		return nil
	}
	if m.prompt == promptComment {
		if key.Matches(pressed, keys.promptPost) {
			m.postComment()
			return nil
		}
		var cmd tea.Cmd
		m.area, cmd = m.area.Update(pressed)
		return cmd
	}
	if key.Matches(pressed, keys.promptGo) {
		return m.submit()
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(pressed)
	return cmd
}

// submit handles enter in the jump and filter prompts. On a terminal that has
// not shown it marks pastes, it first asks the reader to discard the input
// already waiting, and drops every event until the reader confirms.
func (m *interactiveModel) submit() tea.Cmd {
	if m.reader != nil && !m.reader.SawPaste() {
		m.flushAsked = true
		m.reader.RequestFlush()
	}
	text := strings.TrimSpace(m.input.Value())
	if m.prompt == promptFilter {
		m.applyFilter(text)
		return nil
	}
	if text == "" {
		m.closePrompt()
		return nil
	}
	m.closePrompt()
	m.jumpTo(text)
	return nil
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
	m.message = []string{withoutControls(m.s.r.T("interactive.jump.nothing", "text", text))}
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

// postComment posts the comment prompt's text as typed, line breaks
// included, and closes the prompt. An empty or all-whitespace text closes it
// without calling the library.
func (m *interactiveModel) postComment() {
	text := m.area.Value()
	ref, _, ok := m.target()
	m.closePrompt()
	if strings.TrimSpace(text) == "" || !ok {
		return
	}
	asking := m.identity("comment")
	asking.Card = ref
	asking.Text = text
	response := m.l.Comment(asking)
	m.answered(response, "comment", ref, nil)
	m.reread()
}

// paste handles one whole paste. Outside a prompt it is discarded whole.
func (m *interactiveModel) paste(content string) tea.Cmd {
	if m.tooSmall() || m.mode != modePrompt {
		return nil
	}
	if m.prompt == promptComment {
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
