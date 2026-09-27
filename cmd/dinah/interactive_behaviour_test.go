//go:build tui

package main

import (
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// tuiWait is how long a scripted run waits for the program to reach a point
// before failing the test.
const tuiWait = 20 * time.Second

// script drives a run through a pipe, a step at a time. The seam's update
// hook reports every message the model receives, so a step can wait for the
// program to have reached a point before writing the next keys.
type script struct {
	t        *testing.T
	writer   *io.PipeWriter
	seen     chan tea.Msg
	mu       sync.Mutex
	onKey    map[string]func()
	program  *tea.Program
	programs chan *tea.Program
}

// newScript builds a seam of width by height fed from a pipe, whose keys a
// script writes.
func newScript(t *testing.T, width, height int, marked bool) (*script, *interactiveSeams) {
	t.Helper()
	reader, writer := io.Pipe()
	var seam *interactiveSeams
	if marked {
		seam = tuiSeam(t, reader, width, height)
	} else {
		seam = unmarkedSeam(t, reader, width, height)
	}
	s := &script{t: t, writer: writer, seen: make(chan tea.Msg, 4096), onKey: map[string]func(){}, programs: make(chan *tea.Program, 1)}
	seam.update = func(msg tea.Msg) {
		if k, ok := msg.(keyMsg); ok {
			s.mu.Lock()
			do := s.onKey[k.key.Text]
			delete(s.onKey, k.key.Text)
			s.mu.Unlock()
			if do != nil {
				do()
			}
		}
		select {
		case s.seen <- msg:
		default:
		}
	}
	seam.program = func(program *tea.Program) { s.programs <- program }
	return s, seam
}

// when runs do on the event loop the first time Update meets the key typing
// text, before the model handles it.
func (s *script) when(text string, do func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onKey[text] = do
}

// write sends keys through the pipe.
func (s *script) write(keys string) {
	if _, err := s.writer.Write([]byte(keys)); err != nil {
		s.t.Errorf("write %q: %v", keys, err)
	}
}

// waitFor blocks until the model has received a message the predicate
// accepts, and answers it.
func (s *script) waitFor(what string, accept func(tea.Msg) bool) tea.Msg {
	deadline := time.After(tuiWait)
	for {
		select {
		case msg := <-s.seen:
			if accept(msg) {
				return msg
			}
		case <-deadline:
			s.t.Errorf("the program never received %s", what)
			return nil
		}
	}
}

// send hands a message to the running program, from outside it.
func (s *script) send(msg tea.Msg) {
	if s.program == nil {
		select {
		case s.program = <-s.programs:
		case <-time.After(tuiWait):
			s.t.Error("the program never started")
			return
		}
	}
	s.program.Send(msg)
}

// run starts the program on a goroutine of its own driving the steps, and
// answers the run once the steps have ended it.
func (s *script) run(root string, seam *interactiveSeams, steps func(), argv ...string) tuiRun {
	s.t.Helper()
	go func() {
		defer s.writer.Close()
		steps()
	}()
	return runTUIThrough(s.t, root, seam, argv...)
}

// isKey accepts the key message typing text.
func isKey(text string) func(tea.Msg) bool {
	return func(msg tea.Msg) bool {
		k, ok := msg.(keyMsg)
		return ok && k.key.Text == text
	}
}

// countKeys accepts the key message that is the nth the model has met.
func countKeys(n int) func(tea.Msg) bool {
	seen := 0
	return func(msg tea.Msg) bool {
		if _, ok := msg.(keyMsg); ok {
			seen++
		}
		return seen == n
	}
}

// isChange accepts a wait that saw a change.
func isChange(msg tea.Msg) bool {
	change, ok := msg.(changeMsg)
	return ok && change.set != nil && change.set.Changed
}

// isSize accepts a size message of width by height.
func isSize(width, height int) func(tea.Msg) bool {
	return func(msg tea.Msg) bool {
		size, ok := msg.(tea.WindowSizeMsg)
		return ok && size.Width == width && size.Height == height
	}
}

// TestAnOtherSessionsMoveKeepsTheLaneAndSelectsTheCardAtTheOldIndex is
// dinah-603/criteria/11: when another session moves the selected card out of
// the focused lane, the lane keeps focus, the card now at the old index is
// selected, the card is not followed, the offer is the new card's, and the
// live status carries the change text s.changeText gives for that move.
func TestAnOtherSessionsMoveKeepsTheLaneAndSelectsTheCardAtTheOldIndex(t *testing.T) {
	root := tuiBench(t)
	s, seam := newScript(t, 100, 30, true)
	var change *verb.ChangeSet
	s.when("Z", func() { step(t, root, "move", "fx-1", "acceptance") })
	run := s.run(root, seam, func() {
		s.write("Z")
		if msg := s.waitFor("the change", isChange); msg != nil {
			change = msg.(changeMsg).set
		}
		s.write(keyCtrlC)
	})
	m := run.model
	if m == nil || change == nil {
		t.Fatalf("the run ended without a change: %q", run.errw)
	}
	wantModel(t, "the focused lane", focusedColumn(m), "Implement")
	wantModel(t, "the selection", selectedRef(m), "fx-2")
	wantModel(t, "the card the offer is for", m.offer.ref, "fx-2")
	want := m.s.changeText(change)
	wantModel(t, "the live status's change", m.status.change, want)
	if !strings.Contains(m.messageLines()[0], want) {
		t.Errorf("the message area reads %q, which does not carry %q", m.messageLines()[0], want)
	}
}

// TestAnArchivedCardLeavesCardMode is dinah-603/criteria/12: when the card
// shown in card mode is archived by another session, the head returns to
// browse, and the message area carries the change text naming it.
func TestAnArchivedCardLeavesCardMode(t *testing.T) {
	root := tuiBench(t)
	s, seam := newScript(t, 100, 30, true)
	s.when("Z", func() { step(t, root, "archive", "fx-1") })
	run := s.run(root, seam, func() {
		s.write(keyEnter + "Z")
		s.waitFor("the change", isChange)
		s.write(keyCtrlC)
	})
	m := run.model
	if m == nil {
		t.Fatalf("the run never finished: %q", run.errw)
	}
	wantModel(t, "the mode", m.mode, modeBrowse)
	if !strings.Contains(m.messageLines()[0], "fx-1") {
		t.Errorf("the message area reads %q, which does not name fx-1", m.messageLines()[0])
	}
}

// TestAStaleActIsAnsweredStaleAndWritesNothing is dinah-603/criteria/9: an
// act on a card whose revision changed after the frame was drawn, by a CLI
// act between the frame and the key with no change delivered, is answered
// stale, writes nothing to the card, and the message area shows the stale
// line reportOutcome composes.
func TestAStaleActIsAnsweredStaleAndWritesNothing(t *testing.T) {
	root := tuiBench(t)
	s, seam := newScript(t, 100, 30, true)
	hold := make(chan struct{})
	seam.command = func() { <-hold }
	var anchor, journal string
	s.when("Z", func() {
		step(t, root, "set", "fx-1", "title", "Renamed between the frame and the key")
		anchor, journal = anchorText(t, root, "fx-1"), journalText(t, root, "fx-1")
	})
	run := s.run(root, seam, func() {
		s.write("Z")
		s.waitFor("Z", isKey("Z"))
		s.write("t" + keyCtrlC)
	})
	close(hold)
	m := run.model
	if m == nil {
		t.Fatalf("the run never finished: %q", run.errw)
	}
	if anchorText(t, root, "fx-1") != anchor || journalText(t, root, "fx-1") != journal {
		t.Error("the stale claim wrote to fx-1")
	}
	revision := cardRevision(t, root, "fx-1")
	want := contract.OutcomeStale + " " + m.s.r.T("outcome.stale", "revision", revision)
	if len(m.message) != 1 || m.message[0] != want {
		t.Errorf("the message area shows %q, wanted %q", m.message, want)
	}
}

// cardRevision reads a card's revision through dinah show --json.
func cardRevision(t *testing.T, root, ref string) string {
	t.Helper()
	got := runCLI(t, root, "show", ref, "--json", "--fields", "card")
	match := regexp.MustCompile(`"revision": "([^"]*)"`).FindStringSubmatch(got.out)
	if match == nil {
		t.Fatalf("no revision in %s", got.out)
	}
	return match[1]
}

// capacityDefinition gives Review room for one card, so a card another
// session carries there fills it without changing the card the head offers
// the move for.
const capacityDefinition = `{
  "profile": "dinah-core/0.12",
  "title": "Capacity",
  "columns": [
    { "id": "k10000000001", "title": "Intake", "kind": "intake" },
    { "id": "k10000000002", "title": "Implement", "kind": "work" },
    { "id": "k10000000003", "title": "Review", "kind": "work", "capacity": 1 },
    { "id": "k10000000004", "title": "Done", "kind": "done" }
  ]
}`

// TestARefusedOfferedMoveShowsTheMovesOwnRefusal is dinah-603/criteria/10:
// when a move the footer offered is refused because the destination filled
// without the card's revision changing, the message area shows exactly the
// lines, cleaned, that dinah move writes to stderr for the same refusal on a
// copy of the workbench, the card is unchanged, and the next frame no longer
// offers that destination.
func TestARefusedOfferedMoveShowsTheMovesOwnRefusal(t *testing.T) {
	root := newBenchFromDefinition(t, capacityDefinition)
	for _, argv := range [][]string{{"add", "Offered"}, {"move", "fx-1", "implement"}, {"add", "Filler"}, {"move", "fx-2", "implement"}} {
		step(t, root, argv...)
	}
	s, seam := newScript(t, 100, 30, true)
	hold := make(chan struct{})
	seam.command = func() { <-hold }
	var anchor string
	s.when("Z", func() {
		step(t, root, "move", "fx-2", "review")
		anchor = anchorText(t, root, "fx-1")
	})
	run := s.run(root, seam, func() {
		s.write("Z")
		s.waitFor("Z", isKey("Z"))
		s.write("a" + keyCtrlC)
	})
	close(hold)
	m := run.model
	if m == nil {
		t.Fatalf("the run never finished: %q", run.errw)
	}
	if anchorText(t, root, "fx-1") != anchor {
		t.Error("the refused move changed fx-1")
	}
	cli := runCLI(t, copyWorkbench(t, root), "move", "fx-1", "review")
	want := strings.Split(strings.TrimRight(cli.errw, "\n"), "\n")
	for i := range want {
		want[i] = withoutControls(want[i])
	}
	if refusalNameOf(cli.errw) != contract.AtCapacity {
		t.Fatalf("the oracle move was refused %q, wanted %s", refusalNameOf(cli.errw), contract.AtCapacity)
	}
	if strings.Join(m.message, "\n") != strings.Join(want, "\n") {
		t.Errorf("the message area shows\n%s\nand dinah move wrote\n%s", strings.Join(m.message, "\n"), strings.Join(want, "\n"))
	}
	if m.offer.forward != nil {
		t.Errorf("the next frame still offers %s", m.offer.forward.Title)
	}
}

// TestTheAcceptanceWalk is dinah-603/criteria/19: the operator pressing a
// three times in Acceptance moves its three cards to Done in lane order, the
// selection advancing each time; and b sends the selected card to
// Acceptance's reject target.
func TestTheAcceptanceWalk(t *testing.T) {
	root := tuiBench(t)
	run := runTUIThrough(t, root, tuiSeam(t, strings.NewReader("laaaq"), 100, 30))
	if run.code != 0 {
		t.Fatalf("exit %d: %s", run.code, run.errw)
	}
	var stamps []string
	for _, ref := range []string{"fx-3", "fx-4", "fx-5"} {
		wantModel(t, ref+"'s column", columnOf(t, root, ref), "Done")
		events := cardEvents(t, root, ref)
		stamps = append(stamps, events[len(events)-1].TS)
	}
	if !slices.IsSorted(stamps) {
		t.Errorf("the three moves were journaled at %v, which is not lane order", stamps)
	}
	back := tuiBench(t)
	runTUIThrough(t, back, tuiSeam(t, strings.NewReader("ljbq"), 100, 30))
	wantModel(t, "fx-4's column after b", columnOf(t, back, "fx-4"), "Implement")
}

// TestAnAgentIsOfferedNoMoveOutOfTheOperatorsColumn is the agent half of
// dinah-603/criteria/19: an owner who is not the operator is offered no a, b
// or m on the cards standing in Acceptance.
func TestAnAgentIsOfferedNoMoveOutOfTheOperatorsColumn(t *testing.T) {
	root := tuiBench(t)
	t.Setenv("DINAH_ACTOR", "brin")
	run := runTUIThrough(t, root, tuiSeam(t, strings.NewReader("lq"), 100, 30))
	if run.model == nil {
		t.Fatalf("the run never finished: %q", run.errw)
	}
	keys := run.model.keys()
	for _, binding := range keys.shortHelp(run.model, false) {
		if name := binding.Help().Key; name == "a" || name == "b" || name == "m" {
			t.Errorf("the agent's footer offers %s", name)
		}
	}
}

// tieredTUIDefinition declares a tier table and a work column, where a card
// requiring frontier stands.
const tieredTUIDefinition = `{
  "profile": "dinah-core/0.12",
  "title": "Tiered",
  "levels": { "tier": ["workhorse", "frontier"] },
  "tiers": {
    "workhorse": { "meaning": "scoped work", "models": [{ "provider": "acme", "model": "workhorse" }] },
    "frontier": { "meaning": "judgement work", "models": [{ "provider": "acme", "model": "frontier" }] }
  },
  "columns": [
    { "id": "r10000000001", "title": "Intake", "kind": "intake" },
    { "id": "r10000000002", "title": "Work", "kind": "work" },
    { "id": "r10000000003", "title": "Done", "kind": "done" }
  ]
}`

// TestAClaimBelowTheCardsTierIsNeverOffered is dinah-603/criteria/7: a
// caller whose declared model resolves below the card's tier is offered no
// claim, and t leaves the card's anchor and journal byte-identical; a caller
// resolving at the tier is offered the claim, and t takes it.
func TestAClaimBelowTheCardsTierIsNeverOffered(t *testing.T) {
	root := newBenchFromDefinition(t, tieredTUIDefinition)
	for _, argv := range [][]string{{"add", "Needs judgement"}, {"set", "fx-1", "tier", "frontier"}, {"move", "fx-1", "work"}} {
		step(t, root, argv...)
	}
	t.Setenv("DINAH_ACTOR", "brin")
	t.Setenv("DINAH_PROVIDER", "acme")
	t.Setenv("DINAH_MODEL", "workhorse")
	anchor, journal := anchorText(t, root, "fx-1"), journalText(t, root, "fx-1")
	below := runTUIThrough(t, root, tuiSeam(t, strings.NewReader("tq"), 100, 30))
	if below.model == nil {
		t.Fatalf("the run never finished: %q", below.errw)
	}
	if below.model.offer.claim {
		t.Error("a caller below the card's tier was offered the claim")
	}
	if anchorText(t, root, "fx-1") != anchor || journalText(t, root, "fx-1") != journal {
		t.Error("t below the card's tier wrote to fx-1")
	}
	t.Setenv("DINAH_MODEL", "frontier")
	at := runTUIThrough(t, root, tuiSeam(t, strings.NewReader("tq"), 100, 30))
	if at.model == nil {
		t.Fatalf("the run never finished: %q", at.errw)
	}
	wantModel(t, "fx-1", stateOf(t, root, "fx-1"), "state: active, claim_holder: brin")
}

// TestEveryActIsJournaledUnderTheCallersIdentity is dinah-603/criteria/8:
// the journal line each act writes, claim, release, move and comment,
// carries the same actor, harness, provider and model as the line the CLI
// writes for the same act by the same caller on a fresh copy.
func TestEveryActIsJournaledUnderTheCallersIdentity(t *testing.T) {
	root := tuiBench(t)
	t.Setenv("DINAH_HARNESS", "claude-code")
	t.Setenv("DINAH_PROVIDER", "acme")
	t.Setenv("DINAH_MODEL", "frontier")
	fresh := copyWorkbench(t, root)
	runTUIThrough(t, root, tuiSeam(t, strings.NewReader("trcsaid so"+keyCtrlD+"aq"), 100, 30))
	for _, argv := range [][]string{{"claim", "fx-1"}, {"release", "fx-1"}, {"comment", "fx-1", "said so"}, {"move", "fx-1", "acceptance"}} {
		step(t, fresh, argv...)
	}
	head, cli := cardEvents(t, root, "fx-1"), cardEvents(t, fresh, "fx-1")
	head, cli = head[len(head)-4:], cli[len(cli)-4:]
	for i := range head {
		if head[i].Event != cli[i].Event {
			t.Errorf("act %d journaled %s from the head and %s from the command line", i, head[i].Event, cli[i].Event)
		}
		if head[i].Actor != cli[i].Actor {
			t.Errorf("the %s line carries %+v from the head and %+v from the command line", head[i].Event, head[i].Actor, cli[i].Actor)
		}
	}
	if head[0].Actor.Harness != "claude-code" || head[0].Actor.Model != "frontier" {
		t.Errorf("the head's claim carries %+v, which lost the declared identity", head[0].Actor)
	}
}

// TestALapsedClaimIsAnsweredStaleAndOfferedAgain is dinah-603/criteria/28:
// when a claim held by another owner expires after the view was read and
// before OfferActs runs, t is offered; pressing it is answered stale, the
// card is not claimed, exactly one expired line is journaled, and the re-read
// that follows offers t again against the card's new revision.
func TestALapsedClaimIsAnsweredStaleAndOfferedAgain(t *testing.T) {
	root := tuiBench(t)
	step(t, root, "claim", "fx-1", "--actor", "cato", "--expires", "3s")
	s, seam := newScript(t, 99, 30, true)
	hold := make(chan struct{})
	seam.command = func() { <-hold }
	run := s.run(root, seam, func() {
		time.Sleep(4 * time.Second)
		s.write("jk")
		s.waitFor("k", isKey("k"))
		s.write("t" + keyCtrlC)
	})
	close(hold)
	m := run.model
	if m == nil {
		t.Fatalf("the run never finished: %q", run.errw)
	}
	if !strings.HasPrefix(strings.Join(m.message, " "), contract.OutcomeStale+" ") {
		t.Errorf("t on the lapsed claim was answered %q, wanted stale", m.message)
	}
	expired := 0
	for _, event := range cardEvents(t, root, "fx-1") {
		if event.Event == contract.EventExpired {
			expired++
		}
		if event.Event == contract.EventClaimed && event.Actor.Name == "alka" {
			t.Error("the lapsed card was claimed by the head")
		}
	}
	wantModel(t, "the expired lines", expired, 1)
	if !m.offer.claim {
		t.Error("the re-read after the stale answer does not offer t again")
	}
	if card, ok := m.selectedCard(); !ok || card.Revision != cardRevision(t, root, "fx-1") {
		t.Errorf("the offer stands against revision %q, and the card's is %q", card.Revision, cardRevision(t, root, "fx-1"))
	}
}

// TestAResizeWithAPromptOpenKeepsItsText is dinah-603/criteria/16: with the
// comment prompt holding half a thought, a resize to 50 by 10 draws only the
// too-small notice and drops a typed x, a resize back to 100 by 30 shows the
// prompt holding its text with the cursor at its end, and ctrl+d posts
// exactly that comment.
func TestAResizeWithAPromptOpenKeepsItsText(t *testing.T) {
	root := tuiBench(t)
	s, seam := newScript(t, 100, 30, true)
	var small string
	run := s.run(root, seam, func() {
		typed := "chalf a thought"
		s.write(typed)
		s.waitFor("every letter of half a thought", countKeys(len(typed)))
		seam.resize(50, 10)
		s.send(resizeMsg{})
		s.waitFor("the small size", isSize(50, 10))
		s.write("x")
		s.waitFor("x", isKey("x"))
		// Bubble Tea writes a frame on its own ticker, so the small frame is
		// given time to reach the output before the window grows again.
		time.Sleep(200 * time.Millisecond)
		seam.resize(100, 30)
		s.send(resizeMsg{})
		s.waitFor("the large size", isSize(100, 30))
		s.write("!" + keyCtrlD + keyCtrlC)
	})
	m := run.model
	if m == nil {
		t.Fatalf("the run never finished: %q", run.errw)
	}
	small = m.s.r.T("interactive.too-small", "size", "60x12")
	if !strings.Contains(visible(run.output), small) {
		t.Error("the frames never drew the too-small notice")
	}
	comments := commentBodies(t, root, "fx-1")
	if len(comments) != 1 || comments[0] != "half a thought!" {
		t.Errorf("the comments posted are %q, wanted the one half a thought!, which keeps the text, drops the x and leaves the cursor at the end", comments)
	}
}

// commentBodies reads the text of every comment on a card.
func commentBodies(t *testing.T, root, ref string) []string {
	t.Helper()
	dir := filepath.Join(filepath.Dir(anchorPath(t, root, ref)), bench.CommentsDir)
	entries, _ := os.ReadDir(dir)
	var bodies []string
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(dir, entry.Name(), bench.CommentAnchor))
		if err != nil {
			continue
		}
		_, body := bench.ParseAnchor(string(data))
		bodies = append(bodies, strings.TrimRight(body, "\n"))
	}
	return bodies
}

// TestAFailedWaitIsTriedAgainAfterAPause holds the change loop's handling of
// a wait that fails: the message area shows the error as reportError would
// compose it, and the wait is tried again once a watchWait has passed rather
// than at once, so a workbench that stays unreadable is not read in a loop.
func TestAFailedWaitIsTriedAgainAfterAPause(t *testing.T) {
	root := tuiBench(t)
	workbench := soleBenchDir(t, root)
	s, seam := newScript(t, 100, 30, true)
	failed := 0
	var mu sync.Mutex
	user := seam.update
	seam.update = func(msg tea.Msg) {
		if change, ok := msg.(changeMsg); ok && change.err != nil {
			mu.Lock()
			failed++
			mu.Unlock()
		}
		user(msg)
	}
	s.when("Z", func() {
		if err := os.Rename(workbench, workbench+".away"); err != nil {
			t.Errorf("take the workbench away: %v", err)
		}
	})
	run := s.run(root, seam, func() {
		s.write("Z")
		s.waitFor("the first failed wait", func(msg tea.Msg) bool { change, ok := msg.(changeMsg); return ok && change.err != nil })
		time.Sleep(3 * time.Second)
		if err := os.Rename(workbench+".away", workbench); err != nil {
			t.Errorf("put the workbench back: %v", err)
		}
		s.write(keyCtrlC)
	})
	if run.model == nil {
		t.Fatalf("the run never finished: %q", run.errw)
	}
	mu.Lock()
	defer mu.Unlock()
	if failed < 2 || failed > 5 {
		t.Errorf("the wait failed %d times in about three seconds, wanted it tried again about once a second", failed)
	}
}
