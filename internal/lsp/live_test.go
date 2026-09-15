package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/verb"
)

// annotationsOf asks the server for one document's structured annotations.
func annotationsOf(t *testing.T, h *harness, uri string) []wireAnnotation {
	t.Helper()
	return decode[annotationsResult](t, h.send(methodAnnotations, map[string]any{"textDocument": map[string]any{"uri": uri}})).Annotations
}

// hintsOf asks the server for one document's inlay hints.
func hintsOf(t *testing.T, h *harness, uri string) []inlayHint {
	t.Helper()
	return decode[[]inlayHint](t, h.send(methodInlayHint, map[string]any{"textDocument": map[string]any{"uri": uri}}))
}

// clientPulling builds the initialize params of a client that declares both
// the inlay-hint refresh and the configuration pull, which is what a client
// asked for its own settings has to declare.
func clientPulling() map[string]any {
	return map[string]any{
		"capabilities": map[string]any{
			"workspace": map[string]any{
				"inlayHint":     map[string]any{"refreshSupport": true},
				"configuration": true,
			},
		},
	}
}

// proseSettingOn is the notification a client sends when somebody turns the
// prose annotation on while the editor is running.
var proseSettingOn = map[string]any{"settings": map[string]any{"dinah.lsp": map[string]any{"annotateProse": true}}}

// TestTheProseSettingReachesAServerAlreadyRunning is the experiment the cycle
// 1 reviewer ran on dinah-515, kept as a test. The same setting arrives by
// the two routes contract section 5.4 names, once on the command line before
// anything is open and once through workspace/didChangeConfiguration after a
// document is open and its model is already built, and the two servers have
// to draw the same thing.
//
// Every other test of this setting sends it before the first didOpen, which
// is why the defect survived a green suite. A setting read only while a model
// is being built for the first time is indistinguishable from one read
// whenever the model is built.
func TestTheProseSettingReachesAServerAlreadyRunning(t *testing.T) {
	f := build(t)
	card, err := f.bench.LoadCardIn(f.bench.CardsRoot(), f.card)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	text := "The work is on " + card.Ref(f.bench.Slug) + " today.\n"
	doc := filepath.Join(f.bench.Root, "notes.md")

	// The control, told on the command line, so the setting is in force
	// before any model exists. Without it the probe below is an empty read
	// and an empty read proves nothing.
	control, _ := f.serveWith(t, func(o *Options) { o.AnnotateProse = true })
	control.initialize(nil)
	wanted := hintsOf(t, control, control.openText(doc, text))
	if len(wanted) != 1 {
		t.Fatalf("a server told to annotate prose from the start drew %d hints, wanted one", len(wanted))
	}

	// The same setting a moment later, on a server that has already built
	// this document's model without it.
	running, _ := f.serve(t)
	running.initialize(nil)
	uri := running.openText(doc, text)
	if before := hintsOf(t, running, uri); len(before) != 0 {
		t.Fatalf("a server not told to annotate prose drew %d hints, wanted none", len(before))
	}
	running.notify(methodDidChangeConfiguration, proseSettingOn)
	running.settle()

	got := hintsOf(t, running, uri)
	if len(got) != len(wanted) {
		t.Fatalf("after the setting arrived on a running server the document drew %d hints, and the server told from the start drew %d", len(got), len(wanted))
	}
	for i := range got {
		if got[i].Label != wanted[i].Label {
			t.Errorf("hint %d reads %q on the running server and %q on the one told from the start", i, got[i].Label, wanted[i].Label)
		}
		if got[i].Position != wanted[i].Position {
			t.Errorf("hint %d stands at %+v on the running server and at %+v on the one told from the start", i, got[i].Position, wanted[i].Position)
		}
	}

	// An editor asks for hints again when it is told to, so the setting
	// taking effect and the redraw are one thing rather than two.
	running.await(methodInlayHintRefresh, 1)
	running.await(methodAnnotationsChanged, 1)
}

// TestTheStartupConfigurationPullIsRead asserts that the answer to the
// workspace/configuration request sent at initialize reaches the settings.
// That request is the other of the two routes section 5.4 names, and a reply
// nothing correlates costs a round trip and reads nothing.
func TestTheStartupConfigurationPullIsRead(t *testing.T) {
	f := build(t)
	card, err := f.bench.LoadCardIn(f.bench.CardsRoot(), f.card)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	text := "The work is on " + card.Ref(f.bench.Slug) + " today.\n"
	doc := filepath.Join(f.bench.Root, "notes.md")

	h, _ := f.serve(t)
	h.initialize(clientPulling())
	asked := h.await(methodConfiguration, 1)
	var params configurationParams
	if err := json.Unmarshal(asked[0].Params, &params); err != nil {
		t.Fatalf("the configuration request carried %s: %v", asked[0].Params, err)
	}
	if len(params.Items) != 1 || params.Items[0].Section != "dinah.lsp" {
		t.Fatalf("the server asked for %+v, wanted the one section dinah.lsp", params.Items)
	}

	// The protocol answers one member per item asked for.
	h.answer(asked[0], []map[string]any{{"annotateProse": true}})
	h.settle()

	uri := h.openText(doc, text)
	if hints := hintsOf(t, h, uri); len(hints) != 1 {
		t.Fatalf("after the client answered the configuration pull the document drew %d hints, wanted one", len(hints))
	}

	// The accepting case beside a refusing one: a client that declares no
	// configuration capability is asked nothing, so the two routes are not
	// one route read twice.
	quiet, _ := f.serve(t)
	quiet.initialize(clientDeclaring(true))
	quiet.settle()
	if sent := quiet.quiet(methodConfiguration); sent != 0 {
		t.Errorf("a client declaring no configuration capability was sent %d configuration requests, wanted none", sent)
	}
}

// TestAColumnMoveOnDiskReachesAnOpenDocument asserts dinah-515 criterion 5: a
// card moved between two ticks reaches the editor with no process restart and
// no edit, through the refresh request and the namespaced notification, and
// the next annotations answer carries the new column.
func TestAColumnMoveOnDiskReachesAnOpenDocument(t *testing.T) {
	f := build(t)
	h, tick := f.serve(t)
	h.initialize(nil)

	card, err := f.bench.LoadCardIn(f.bench.CardsRoot(), f.card)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	// The document annotates the card through a links[].to position, which is
	// a declared front-matter position and so draws an inline annotation
	// whatever the prose setting says. Nothing in the document's own bytes
	// changes for the rest of this test: what moves is the card underneath.
	text := "---\ntitle: A second card\nlinks:\n  - kind: relates_to\n    to: " + f.card + "\n---\n"
	uri := h.openText(f.cardAnchor(f.other), text)

	before := annotationsOf(t, h, uri)
	if len(before) != 1 {
		t.Fatalf("the document drew %d annotations, wanted one", len(before))
	}
	first := f.bench.Column(card.Column)
	if before[0].Fields[fieldColumn] != first.ID {
		t.Fatalf("the annotation reports the column %q, wanted %q", before[0].Fields[fieldColumn], first.ID)
	}

	destination := f.bench.Columns[1]
	tick.baseline(t)
	refreshes := h.quiet(methodInlayHintRefresh)
	changed := h.quiet(methodAnnotationsChanged)
	f.run(t, "move", &verb.Request{Card: f.card, Column: destination.ID})
	tick.step(t)

	h.await(methodInlayHintRefresh, refreshes+1)
	h.await(methodAnnotationsChanged, changed+1)

	after := annotationsOf(t, h, uri)
	if len(after) != 1 {
		t.Fatalf("after the move the document drew %d annotations, wanted one", len(after))
	}
	if after[0].Fields[fieldColumn] != destination.ID {
		t.Errorf("the annotation reports the column %s, wanted %s", after[0].Fields[fieldColumn], destination.ID)
	}
	if !strings.Contains(after[0].Label, destination.Title) {
		t.Errorf("the annotation reads %q, which does not name %q", after[0].Label, destination.Title)
	}
}

// TestAHandEditedColumnAnchorReachesAnOpenDocument asserts dinah-515
// criterion 6, which is the case the widened checkpoint exists for: a column
// anchor written by hand, with no Dinah verb involved and no other file
// touched, moves the annotation on a card whose column names it.
//
// Armed by deleting the column half from bench.WatchedEntities, which leaves
// the change term unmoved so the server never re-opens and the label keeps
// its old title.
func TestAHandEditedColumnAnchorReachesAnOpenDocument(t *testing.T) {
	f := build(t)
	h, tick := f.serve(t)
	h.initialize(nil)

	column := f.bench.Columns[0]
	text := "---\ntitle: A card\ncolumn: " + column.ID + "\n---\n"
	uri := h.openText(f.cardAnchor(f.card), text)
	before := annotationsOf(t, h, uri)
	if len(before) != 1 || before[0].Label != column.Title {
		t.Fatalf("the annotation reads %+v, wanted the column's title %q", before, column.Title)
	}
	if before[0].Fields[fieldHold] != "" {
		t.Fatalf("the fixture's first column already holds %q, so the second case below proves nothing", before[0].Fields[fieldHold])
	}

	anchor := f.columnAnchor(column.ID)
	raw, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read the column anchor: %v", err)
	}
	tick.baseline(t)
	renamed := strings.Replace(string(raw), "title: "+column.Title, "title: "+column.Title+"RENAMED", 1)
	if renamed == string(raw) {
		t.Fatalf("the column anchor carries no title line to edit:\n%s", raw)
	}
	if err := os.WriteFile(anchor, []byte(renamed), 0o644); err != nil {
		t.Fatalf("write the column anchor: %v", err)
	}
	tick.step(t)

	after := annotationsOf(t, h, uri)
	if len(after) != 1 {
		t.Fatalf("after the hand edit the document drew %d annotations, wanted one", len(after))
	}
	if after[0].Label != column.Title+"RENAMED" {
		t.Errorf("after a hand edit the annotation reads %q, wanted %q", after[0].Label, column.Title+"RENAMED")
	}

	// The second case: gate_items edited by hand moves the hold member of the
	// fields map, which is the fact the notice exists to report.
	raw, err = os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("reread the column anchor: %v", err)
	}
	gated := strings.Replace(string(raw), "---\ntitle:", "---\ngate_items: out\ntitle:", 1)
	if gated == string(raw) {
		t.Fatalf("the column anchor could not be given a gate_items line:\n%s", raw)
	}
	if err := os.WriteFile(anchor, []byte(gated), 0o644); err != nil {
		t.Fatalf("write the gate: %v", err)
	}
	tick.step(t)

	held := annotationsOf(t, h, uri)
	if len(held) != 1 {
		t.Fatalf("after the gate edit the document drew %d annotations, wanted one", len(held))
	}
	if held[0].Fields[fieldHold] != holdOut {
		t.Errorf("after a hand-edited gate the annotation reports hold %q, wanted %q", held[0].Fields[fieldHold], holdOut)
	}
}

// TestAFailedReadNeverChangesWhatIsShown asserts dinah-515 criterion 7: a
// card directory whose anchor is gone leaves the annotation already on screen
// exactly as it stands, rather than substituting the unresolved label, and
// only a departure the checkpoint reports drops it.
func TestAFailedReadNeverChangesWhatIsShown(t *testing.T) {
	f := build(t)
	h, tick := f.serve(t)
	h.initialize(nil)

	card, err := f.bench.LoadCardIn(f.bench.CardsRoot(), f.card)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ref := card.Ref(f.bench.Slug)
	h.notify(methodDidChangeConfiguration, map[string]any{"settings": map[string]any{"dinah.lsp": map[string]any{"annotateProse": true}}})
	h.settle()
	uri := h.openText(filepath.Join(f.bench.Root, "notes.md"), "The work is on "+ref+" today.\n")

	before := annotationsOf(t, h, uri)
	if len(before) != 1 {
		t.Fatalf("the document drew %d annotations, wanted one", len(before))
	}
	shown := before[0].Label

	// A card directory carrying no anchor is the filing sequence caught
	// midway, which the format calls a detectably incomplete thing. The
	// server has to leave what is on screen alone.
	anchor := f.cardAnchor(f.card)
	saved, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	tick.baseline(t)
	if err := os.Remove(anchor); err != nil {
		t.Fatalf("remove the anchor: %v", err)
	}
	tick.step(t)

	during := annotationsOf(t, h, uri)
	if len(during) != 1 {
		t.Fatalf("while the anchor was absent the document drew %d annotations, wanted the one already shown", len(during))
	}
	if during[0].Label != shown {
		t.Errorf("while the anchor was absent the annotation read %q, wanted the retained %q", during[0].Label, shown)
	}
	if during[0].Label == h.server.messages.T(keyLabelUnresolved) {
		t.Error("a read that could not complete substituted the unresolved label, which establishes nothing")
	}
	if err := os.WriteFile(anchor, saved, 0o644); err != nil {
		t.Fatalf("restore: %v", err)
	}
	tick.step(t)

	// The departure the checkpoint reports is the positive evidence that
	// drops it. Deleting the card through the library is what writes the
	// event the answer's gone entry is derived from.
	f.reopen(t)
	answer := f.library.Delete(&verb.Request{Verb: "delete", Actor: "alka", Ref: f.card, Card: f.card, Confirm: true})
	if answer.Outcome != "ok" {
		t.Fatalf("delete: %s %s", answer.Outcome, answer.Refusal)
	}
	tick.step(t)

	gone := annotationsOf(t, h, uri)
	if len(gone) != 0 {
		t.Errorf("after the departure the document drew %d annotations, wanted none: %+v", len(gone), gone)
	}
}

// TestThePollLoopNeverOverlapsAndReportsBothSlowEdges asserts dinah-515
// criterion 22: the loop sleeps for the larger of the interval and the walk
// just finished, so two walks are never in flight at once, and the slow-walk
// state sends exactly one warning on each edge and none in between.
func TestThePollLoopNeverOverlapsAndReportsBothSlowEdges(t *testing.T) {
	f := build(t)
	tick := newTicker()
	var walks int32
	var overlapped bool
	running := make(chan struct{}, 1)
	h := start(t, Options{
		Workbench:   f.root,
		PollSeconds: 1,
		Sleep:       tick.sleep,
		Now:         tick.now,
		Since:       tick.since,
		Walk: func() bool {
			select {
			case running <- struct{}{}:
			default:
				overlapped = true
			}
			walks++
			time.Sleep(time.Millisecond)
			<-running
			return false
		},
	})
	h.initialize(nil)

	// A walk quicker than the interval sleeps the interval and sends nothing.
	tick.takes(0)
	if slept := tick.step(t); slept != time.Second {
		t.Errorf("a quick walk slept %s, wanted the interval of 1s", slept)
	}
	if sent := h.quiet(methodLogMessage); sent > 1 {
		t.Errorf("a quick walk sent %d log lines beyond the startup line", sent-1)
	}
	startup := h.quiet(methodLogMessage)

	// A walk four times the interval sleeps the walk rather than the
	// interval, and sends exactly one warning as the state is entered.
	tick.takes(4 * time.Second)
	if slept := tick.step(t); slept != 4*time.Second {
		t.Errorf("a slow walk slept %s, wanted the walk's own 4s", slept)
	}
	entered := h.await(methodLogMessage, startup+1)
	if len(entered) != startup+1 {
		t.Fatalf("entering the slow state sent %d log lines, wanted one", len(entered)-startup)
	}
	warning := decode[logMessageParams](t, entered[len(entered)-1].Params)
	if warning.Type != messageTypeWarning {
		t.Errorf("the entering line is type %d, wanted a warning", warning.Type)
	}
	if !strings.Contains(warning.Message, "4s") || !strings.Contains(warning.Message, "1s") {
		t.Errorf("the entering line reads %q, which does not name the walk and the interval", warning.Message)
	}

	// Three further slow walks send nothing, because a warning repeated every
	// interval is a warning nobody reads.
	for i := 0; i < 3; i++ {
		tick.step(t)
	}
	if sent := h.quiet(methodLogMessage); sent != startup+1 {
		t.Errorf("three further slow walks sent %d more log lines, wanted none", sent-startup-1)
	}

	// The reset route: raising the interval above the walk's duration sends
	// nothing by itself, and the next completed walk sends the leaving edge.
	h.notify(methodDidChangeConfiguration, map[string]any{"settings": map[string]any{"dinah.lsp": map[string]any{"pollIntervalSeconds": 10}}})
	h.settle()
	if sent := h.quiet(methodLogMessage); sent != startup+1 {
		t.Errorf("the configuration notification sent %d log lines by itself, wanted none", sent-startup-1)
	}
	if slept := tick.step(t); slept != 10*time.Second {
		t.Errorf("after the interval was raised the loop slept %s, wanted 10s", slept)
	}
	left := h.await(methodLogMessage, startup+2)
	leaving := decode[logMessageParams](t, left[len(left)-1].Params)
	if leaving.Type != messageTypeWarning {
		t.Errorf("the leaving line is type %d, wanted a warning", leaving.Type)
	}
	if !strings.Contains(leaving.Message, "4s") || !strings.Contains(leaving.Message, "10s") {
		t.Errorf("the leaving line reads %q, which does not name the walk and the interval it is again sleeping", leaving.Message)
	}
	if overlapped {
		t.Error("two walks were in flight at once")
	}
	if walks < 6 {
		t.Errorf("the loop ran %d walks, and this test drove six", walks)
	}
}

// TestTheRefreshDegradationIsReportedRatherThanSilent asserts dinah-515
// criterion 24: a client declaring no inlay-hint refresh support is told so
// once and receives no refresh on any tick, and a client declaring it is told
// nothing and does receive one.
func TestTheRefreshDegradationIsReportedRatherThanSilent(t *testing.T) {
	for _, declares := range []bool{false, true} {
		name := "a client declaring no refresh support"
		if declares {
			name = "a client declaring refresh support"
		}
		t.Run(name, func(t *testing.T) {
			f := build(t)
			h, tick := f.serve(t)
			h.initialize(clientDeclaring(declares))

			want := 1
			if declares {
				want = 0
			}
			h.await(methodLogMessage, want+1)
			warned := 0
			for _, sent := range h.sent(methodLogMessage) {
				if decode[logMessageParams](t, sent.Params).Message == h.server.messages.T(keyLogNoRefreshSupport) {
					warned++
				}
			}
			if warned != want {
				t.Errorf("%s received %d no-refresh lines, wanted %d", name, warned, want)
			}

			card, err := f.bench.LoadCardIn(f.bench.CardsRoot(), f.card)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			_ = card
			h.openText(f.cardAnchor(f.other), "---\ntitle: A second card\nlinks:\n  - kind: relates_to\n    to: "+f.card+"\n---\n")
			tick.baseline(t)
			f.run(t, "move", &verb.Request{Card: f.card, Column: f.bench.Columns[1].ID})
			tick.step(t)
			h.await(methodAnnotationsChanged, 1)

			refreshes := h.quiet(methodInlayHintRefresh)
			if declares && refreshes == 0 {
				t.Error("a client declaring refresh support received no refresh on a tick whose model changed")
			}
			if !declares && refreshes != 0 {
				t.Errorf("a client declaring no refresh support received %d refreshes", refreshes)
			}
		})
	}
}

// TestNoWorkbenchServesAnywayAndPicksOneUpLater asserts dinah-515 criterion
// 16: a directory under which nothing resolves is served rather than exited,
// every capability declared and every answer empty, and a workbench created
// under it afterwards is picked up on a tick without a restart.
func TestNoWorkbenchServesAnywayAndPicksOneUpLater(t *testing.T) {
	empty := t.TempDir()
	t.Setenv("DINAH_HOME", filepath.Join(empty, "home"))
	tick := newTicker()
	h := start(t, Options{
		Wd:    empty,
		Sleep: tick.sleep,
		Now:   tick.now,
		Since: tick.since,
		Discover: func(start string) (string, error) {
			root, _, err := bench.Discover(start, "", filepath.Join(empty, "home"), "")
			return root, err
		},
	})
	h.initialize(nil)

	if sent := h.await(methodShowMessage, 1); len(sent) != 1 {
		t.Fatalf("the server sent %d show-message lines, wanted one", len(sent))
	}
	shown := decode[logMessageParams](t, h.sent(methodShowMessage)[0].Params)
	if shown.Message != h.server.messages.T(keyNoWorkbench) {
		t.Errorf("the server showed %q, wanted the no-workbench line", shown.Message)
	}

	doc := filepath.Join(empty, "notes.md")
	uri := h.openText(doc, "The work is on wb-1 today.\n")
	at := map[string]any{"textDocument": map[string]any{"uri": uri}}
	where := map[string]any{"textDocument": map[string]any{"uri": uri}, "position": map[string]any{"line": 0, "character": 16}}
	if got := string(h.send(methodHover, where)); got != "null" {
		t.Errorf("hover answered %s, wanted null", got)
	}
	if got := decode[[]documentLink](t, h.send(methodDocumentLink, at)); len(got) != 0 {
		t.Errorf("documentLink answered %d links, wanted none", len(got))
	}
	if got := string(h.send(methodDefinition, where)); got != "null" {
		t.Errorf("definition answered %s, wanted null", got)
	}
	if got := decode[completionList](t, h.send(methodCompletion, where)); len(got.Items) != 0 {
		t.Errorf("completion answered %d items, wanted none", len(got.Items))
	}
	if got := decode[[]inlayHint](t, h.send(methodInlayHint, at)); len(got) != 0 {
		t.Errorf("inlayHint answered %d hints, wanted none", len(got))
	}

	// A workbench created under that directory afterwards is picked up on a
	// tick, with no restart.
	written, err := verb.Init(empty, "wb", "alka", "", "", "")
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	opened, err := bench.Open(written)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if answer := verb.New(opened, "").Add(&verb.Request{Verb: "add", Actor: "alka", Title: "The first card"}); answer.Outcome != "ok" {
		t.Fatalf("add: %s %s", answer.Outcome, answer.Refusal)
	}
	tick.step(t)
	tick.step(t)

	h.notify(methodDidChange, map[string]any{
		"textDocument":   map[string]any{"uri": uri},
		"contentChanges": []map[string]any{{"text": "The work is on wb-1 today.\n"}},
	})
	h.settle()
	hover := decode[hoverResult](t, h.send(methodHover, where))
	if hover.Contents.Value == "" {
		t.Error("after a workbench appeared the server still answered no hover, so it never picked one up")
	}
}
