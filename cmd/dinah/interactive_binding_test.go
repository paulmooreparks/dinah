//go:build tui

package main

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// settleProbe is a message a test sends the running program to read, on the
// event loop, whether the model has finished with everything it was given.
// Update ignores it.
type settleProbe struct{}

// modelSettled reports whether a model that has met keys key messages has handled
// all of them: none is queued behind a read, no read of the view is in flight
// (readSeq and readAnswered, the generation tags every read carries, agree),
// and no line is waiting on its own read to report itself done. An act and a
// line both run on the event loop and then read the view off it, so a model
// in this state has run every act and every line its keys asked for.
func modelSettled(m *interactiveModel, seen, keys int) bool {
	return seen >= keys && len(m.queued) == 0 && m.readSeq == m.readAnswered && m.pendingLine == nil
}

// awaitSettled blocks until the model has met keys key messages and handled
// them all, probing it from outside with settleProbe and reading its answer
// through the seam's observe hook, and fails the test at tuiWait.
func awaitSettled(t *testing.T, s *script, answers <-chan bool) {
	t.Helper()
	deadline := time.After(tuiWait)
	for {
		s.send(settleProbe{})
		select {
		case done := <-answers:
			if done {
				return
			}
		case <-deadline:
			t.Error("the program never finished handling the keys it was given")
			return
		}
		select {
		case <-deadline:
			t.Error("the program never finished handling the keys it was given")
			return
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// bindKey stores a key binding and its label in the user's configuration,
// past the command line, as a test arranging one before the head starts.
func bindKey(t *testing.T, key, template, label string) {
	t.Helper()
	cfg := bench.LoadConfig(os.Getenv("DINAH_HOME"))
	if err := cfg.SetKeyBinding(bench.KeyBindingPrefix+key, template, true); err != nil {
		t.Fatalf("bind %s: %v", key, err)
	}
	if label != "" {
		if err := cfg.SetKeyBinding(bench.KeyLabelPrefix+key, label, true); err != nil {
			t.Fatalf("label %s: %v", key, err)
		}
	}
}

// boundRun runs the head over keys, recording the words every line ran. Every
// line a bound key runs, and every act a key makes, ends off the event loop,
// so keys is written whole and then boundRun waits for the model to report
// that it has handled every one of them before it writes ctrl+c: a key still
// queued behind an outstanding read when ctrl+c reaches the program is
// answered never, since ctrl+c quits ahead of the queue rather than behind
// it. Each byte of keys is one key, which holds for every caller here, since
// none writes an escape sequence.
func boundRun(t *testing.T, root, actor, keys string) (tuiRun, [][]string) {
	t.Helper()
	return boundRunWith(t, root, actor, keys, nil)
}

// boundRunWith is boundRun with arrange called on the seam before the run
// starts, where a test sets a hook boundRun leaves alone.
func boundRunWith(t *testing.T, root, actor, keys string, arrange func(*interactiveSeams)) (tuiRun, [][]string) {
	t.Helper()
	asActor(t, actor)
	var mu sync.Mutex
	var ran [][]string
	s, seam := newScript(t, actWidth, actHeight, true)
	if arrange != nil {
		arrange(seam)
	}
	seam.lineDone = func(result *lineResult) {
		mu.Lock()
		ran = append(ran, result.words)
		mu.Unlock()
	}
	answers := make(chan bool, 1)
	seen := 0
	seam.observe = func(m *interactiveModel, msg tea.Msg) {
		switch msg.(type) {
		case keyMsg:
			seen++
		case settleProbe:
			answers <- modelSettled(m, seen, len(keys))
		}
	}
	run := s.run(root, seam, func() {
		if keys != "" {
			s.write(keys)
		}
		awaitSettled(t, s, answers)
		s.write(keyCtrlC)
	})
	mu.Lock()
	defer mu.Unlock()
	if run.model == nil {
		t.Fatalf("the run never finished: %q", run.errw)
	}
	return run, ran
}

// footerEntry is how the footer draws a binding: its key and its label.
func footerEntry(key, label string) string {
	return key + " " + label
}

// TestABindingRunsItsLineWithTheValuesSubstituted is dinah-623/criteria/42
// and /50: a binding runs through runLine with its placeholders substituted
// into its tokens. With tui.key.o set to comment $card noted and a card
// selected, the comment is posted on that card as the head's pinned identity
// on the pinned workbench; $card/checklist substitutes inside its token; the
// command is handed exactly as many arguments as the template has tokens
// whatever a substituted value holds, so a value carrying spaces and
// --override stays one argument rather than becoming a flag; $column is the
// column's slug, never
// its title; and a value beginning with - or carrying $ is refused as
// dinah.usage and runs nothing.
func TestABindingRunsItsLineWithTheValuesSubstituted(t *testing.T) {
	root := tuiBench(t)
	bindKey(t, "o", "comment $card noted", "")
	bindKey(t, "p", "list $card/checklist", "")
	bindKey(t, "u", "comment fx-2 $view", "")
	t.Setenv("DINAH_ACTOR", "alka")
	t.Setenv("DINAH_PROVIDER", "acme")
	t.Setenv("DINAH_MODEL", "m1")
	_, ran := boundRun(t, root, "alka", "o"+keyBackspace+"p"+keyBackspace)
	if len(ran) != 2 || strings.Join(ran[0], " ") != "comment fx-1 noted" || strings.Join(ran[1], " ") != "list fx-1/checklist" {
		t.Errorf("the bindings ran %q", ran)
	}
	events := cardEvents(t, root, "fx-1")
	last := events[len(events)-1]
	if last.Event != contract.EventCommented || last.Actor.Name != "alka" || last.Actor.Model != "m1" {
		t.Errorf("the comment was journaled as %+v", last)
	}
	s, seam := newScript(t, actWidth, actHeight, true)
	seam.bindingValue = func(placeholder, value string) string {
		if placeholder == "view" {
			return "sorted --override later"
		}
		return value
	}
	var words []string
	done := make(chan *lineResult, 1)
	seam.lineDone = func(result *lineResult) { words = result.words; done <- result }
	s.run(root, seam, func() {
		s.write("u")
		<-done
		s.write(keyBackspace + keyCtrlC)
	})
	if len(words) != 3 || words[2] != "sorted --override later" {
		t.Errorf("the binding handed %q, wanted three words with the value whole", words)
	}
	if bodies := commentBodies(t, root, "fx-2"); len(bodies) != 1 || bodies[0] != "sorted --override later" {
		t.Errorf("fx-2 carries %q", bodies)
	}

	root = newBenchFromDefinition(t, `{
  "profile": "dinah-core/0.12",
  "title": "Queue",
  "columns": [
    { "id": "q10000000001", "title": "Intake", "slug": "intake", "kind": "intake" },
    { "id": "q10000000002", "title": "Build Queue", "slug": "build-queue", "kind": "work" }
  ]
}`)
	step(t, root, "add", "Waiting")
	step(t, root, "add", "Also waiting")
	step(t, root, "move", "fx-2", "build-queue")
	bindKey(t, "g", "pull $column", "")
	_, ran = boundRun(t, root, "alka", "lg")
	if len(ran) != 1 || strings.Join(ran[0], " ") != "pull build-queue" {
		t.Errorf("pull $column ran %q", ran)
	}
	if !journaled(t, root, "fx-1", contract.EventClaimed) {
		t.Error("pull $column pulled nothing into Build Queue")
	}
	for _, value := range []string{"-x", "a$b"} {
		before := benchBytes(t, root)
		seam := tuiSeam(t, strings.NewReader("lg"+keyCtrlC), actWidth, actHeight)
		seam.bindingValue = func(string, string) string { return value }
		var ran []string
		seam.lineDone = func(result *lineResult) { ran = result.words }
		run := runTUIThrough(t, root, seam)
		if len(ran) != 0 || !sameBytes(before, benchBytes(t, root)) {
			t.Errorf("a substituted %q ran %q", value, ran)
		}
		if len(run.model.message) == 0 || !strings.HasPrefix(run.model.message[0], contract.Usage+" "+value) {
			t.Errorf("a substituted %q showed %q", value, run.model.message)
		}
	}
}

// TestABindingNeedingASelectionWaitsForOne is dinah-623/criteria/43: with
// nothing selected, a binding using $card is absent from the footer and
// pressing it shows interactive.binding.needs and runs nothing, and a binding
// using only $view is listed and runs.
func TestABindingNeedingASelectionWaitsForOne(t *testing.T) {
	root := newBenchFromDefinition(t, tuiDefinition)
	bindKey(t, "o", "comment $card noted", "note it")
	bindKey(t, "p", "view $view", "redraw")
	run, ran := boundRun(t, root, "alka", "")
	footer := run.model.footerText()
	if strings.Contains(footer, footerEntry("o", "note it")) || !strings.Contains(footer, footerEntry("p", "redraw")) {
		t.Errorf("the footer with nothing selected reads:\n%s", footer)
	}
	run, ran = boundRun(t, root, "alka", "o")
	what := run.model.s.r.T("interactive.binding.what.card")
	if len(ran) != 0 || strings.Join(run.model.message, "") != run.model.s.r.T("interactive.binding.needs", "key", "o", "what", what) {
		t.Errorf("o with nothing selected ran %q and showed %q", ran, run.model.message)
	}
	_, ran = boundRun(t, root, "alka", "p")
	if len(ran) != 1 || strings.Join(ran[0], " ") != "view board" {
		t.Errorf("p ran %q", ran)
	}
}

// TestTheFooterListsABindingTheOfferAccepts is dinah-623/criteria/44: a
// binding whose command is a single card verb on the selected card is listed
// only when the offer accepts it, so claim $card is absent on a card the
// actor may not claim, where pressing it writes nothing, and listed on one he
// may; a binding running status is always listed; and a binding running a
// card verb on another card is listed and shows the refusal when pressed.
func TestTheFooterListsABindingTheOfferAccepts(t *testing.T) {
	root := tuiBench(t)
	bindKey(t, "o", "claim $card", "take it")
	bindKey(t, "p", "status", "")
	bindKey(t, "u", "claim fx-2", "take the other")
	step(t, root, "claim", "fx-1", "--actor", "cato")
	step(t, root, "claim", "fx-2", "--actor", "cato")
	run, _ := boundRun(t, root, "brin", "")
	footer := run.model.footerText()
	if strings.Contains(footer, footerEntry("o", "take it")) {
		t.Errorf("claim $card is listed on a card the actor may not claim:\n%s", footer)
	}
	for _, want := range []string{footerEntry("p", "status"), footerEntry("u", "take the other")} {
		if !strings.Contains(footer, want) {
			t.Errorf("the footer does not list %q:\n%s", want, footer)
		}
	}
	before := benchBytes(t, root)
	_, ran := boundRun(t, root, "brin", "o")
	if len(ran) != 0 || !sameBytes(before, benchBytes(t, root)) {
		t.Errorf("o on a card the actor may not claim ran %q", ran)
	}
	run, _ = boundRun(t, root, "brin", "u")
	if len(run.model.message) == 0 || !strings.HasPrefix(run.model.message[0], contract.Held+" ") {
		t.Errorf("u on another held card showed %q", run.model.message)
	}
	root = tuiBench(t)
	bindKey(t, "o", "claim $card", "take it")
	run, _ = boundRun(t, root, "brin", "")
	if !strings.Contains(run.model.footerText(), footerEntry("o", "take it")) {
		t.Errorf("claim $card is not listed on a card the actor may claim:\n%s", run.model.footerText())
	}
}

// TestABindingsLabelIsTheUsersOwn is dinah-623/criteria/45: a binding's
// footer entry shows its tui.label value, or its template where it has none,
// as the user wrote it with control characters replaced and never looked up
// in a catalog; and a stored binding on a reserved key or with an invalid
// template is reported once at start with interactive.binding.unused and
// does not run.
func TestABindingsLabelIsTheUsersOwn(t *testing.T) {
	root := tuiBench(t)
	bindKey(t, "o", "status", "interactive.help.claim \x1b[31mred")
	bindKey(t, "p", "whoami", "")
	writeRawSetting(t, os.Getenv("DINAH_HOME"), "tui.key.t", "status")
	writeRawSetting(t, os.Getenv("DINAH_HOME"), "tui.key.u", "!ls")
	run, _ := boundRun(t, root, "alka", "")
	footer := run.model.footerText()
	if !strings.Contains(footer, footerEntry("o", withoutControls("interactive.help.claim \x1b[31mred"))) {
		t.Errorf("the label is not shown as written:\n%s", footer)
	}
	if strings.Contains(footer, "\x1b[31mred") {
		t.Errorf("the label carries its control character:\n%q", footer)
	}
	if !strings.Contains(footer, footerEntry("p", "whoami")) {
		t.Errorf("a binding with no label does not show its template:\n%s", footer)
	}
	r := run.model.s.r
	want := []string{
		r.T("interactive.binding.unused", "key", "t", "reason", r.T("interactive.binding.defect.reserved-key")),
		r.T("interactive.binding.unused", "key", "u", "reason", r.T("interactive.binding.defect.shell-template")),
	}
	var first []string
	seam := tuiSeam(t, strings.NewReader(keyCtrlC), actWidth, actHeight)
	seam.frame = func(content string) {
		if first == nil {
			first = strings.Split(content, "\n")
		}
	}
	runTUIThrough(t, root, seam)
	joined := strings.Join(first, "\n")
	for _, notice := range want {
		if !strings.Contains(joined, withoutControls(notice)) {
			t.Errorf("the first frame does not report %q:\n%s", notice, joined)
		}
	}
	_, ran := boundRun(t, root, "alka", "u")
	if len(ran) != 0 {
		t.Errorf("the invalid binding ran %q", ran)
	}
}

// TestABindingIsReadInBrowseAndCardModeAlone is dinah-623/criteria/51: with
// v, f and o bound, v, f and o in item mode still verify, fail and reopen,
// and v typed at the command line puts v in the prompt.
//
// Every read of the view is held for slowRead, longer than the 300 ms this
// test once waited for the head to fall quiet, which stands for a loaded
// runner whose flushed writes and rereads are slow: the three acts' keys queue
// behind those reads, and the run must wait for the model to have handled
// them rather than for a quiet period.
func TestABindingIsReadInBrowseAndCardModeAlone(t *testing.T) {
	const slowRead = 400 * time.Millisecond
	root := tuiBench(t)
	for _, key := range []string{"v", "f", "o"} {
		bindKey(t, key, "status", "")
	}
	step(t, root, "file", "fx-1", "acceptance_criterion", "It reads every line")
	step(t, root, "file", "fx-1", "acceptance_criterion", "It rejects a torn line")
	step(t, root, "file", "fx-1", "open_question", "Which vendor?")
	step(t, root, "resolve", "fx-1/questions/1", "--text", "the usual")
	slow := func(seam *interactiveSeams) { seam.readGate = func() { time.Sleep(slowRead) } }
	_, ran := boundRunWith(t, root, "alka", "iv"+"it read them"+keyCtrlD+"jf"+"it tore"+keyCtrlD+"jjo"+"closed wrongly"+keyCtrlD, slow)
	if len(ran) != 0 {
		t.Errorf("a letter in item mode ran a binding: %q", ran)
	}
	for _, event := range []string{contract.EventItemVerified, contract.EventItemFailed, contract.EventItemReopened} {
		if !journaled(t, root, "fx-1", event) {
			t.Errorf("item mode's letters journaled no %s", event)
		}
	}
	run, ran := boundRun(t, root, "alka", ":v")
	if run.model.input.Value() != "v" || len(ran) != 0 {
		t.Errorf("v at the command line left %q and ran %q", run.model.input.Value(), ran)
	}
}
