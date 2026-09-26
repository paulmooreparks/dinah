//go:build tui

package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// lineWidth and lineHeight are the window the command line's tests run in,
// and lineDraw the width a line session lays its output out at.
const (
	lineWidth  = 200
	lineHeight = 40
	lineDraw   = lineWidth - 1
)

// runLines types lines at the command line one at a time, each after the
// previous one has ended, and answers what each left. keys, when set, are
// typed after the last line and before the head quits.
func runLines(t *testing.T, root string, lines []string, keys string) ([]*lineResult, tuiRun) {
	t.Helper()
	done := make(chan *lineResult, len(lines)+4)
	s, seam := newScript(t, lineWidth, lineHeight, true)
	seam.lineDone = func(result *lineResult) { done <- result }
	cycles := make(chan *tea.Program, 8)
	seam.program = func(program *tea.Program) { cycles <- program }
	var results []*lineResult
	run := s.run(root, seam, func() {
		waitForProgram(t, cycles)
		for _, line := range lines {
			s.write(":" + line + keyEnter)
			select {
			case result := <-done:
				results = append(results, result)
			case <-time.After(tuiWait):
				t.Errorf("the line %q never ended", line)
				return
			}
		}
		s.write(keys + keyCtrlC)
	})
	if run.model == nil {
		t.Fatalf("the run never finished: %q", run.errw)
	}
	return results, run
}

// cliLines is what run() writes to both streams for words, as lines cleaned
// as a transcript cleans them.
func cliLines(t *testing.T, root string, words []string) []string {
	t.Helper()
	got := runCLI(t, root, words...)
	transcript := &lineTranscript{}
	transcript.Write([]byte(got.out))
	transcript.Write([]byte(got.errw))
	transcript.finish()
	return transcript.lines
}

// TestALineShowsWhatTheCLIPrints is dinah-623/criteria/7. Each of the lines
// below, typed at the command line, shows in the message area or output mode
// exactly the lines run() writes for the same words at the head's draw
// width, in written order and cleaned. They are run twice over the same
// workbench at the same path, once through run() and once at the command
// line, so a line that writes meets the same workbench both times: the
// refused lines leave it unchanged, and the accepted move writes the event
// the CLI writes.
func TestALineShowsWhatTheCLIPrints(t *testing.T) {
	root := tuiBench(t)
	home := os.Getenv("DINAH_HOME")
	cfg := bench.LoadConfig(home)
	if err := cfg.SetAlias("alias.mv", "move $1 acceptance", true); err != nil {
		t.Fatal(err)
	}
	if err := cfg.SetAlias("alias.two", "move $1 $2", true); err != nil {
		t.Fatal(err)
	}
	writeRawSetting(t, home, "alias.bad", "!ls")
	t.Setenv("COLUMNS", strconv.Itoa(lineDraw))
	lines := []string{
		"--format json nosuch",
		"show fx-1 extra",
		"show fx-1 --kind x",
		`raise fx-1 frontier because --kind x`,
		"bad",
		"two fx-1",
		"move fx-1 nowhere",
		"status",
		"list",
		"list columns",
		"list routes",
		"list fx-1/checklist",
		"next",
		"query state:ready",
		"search parser",
		"tree",
		"show fx-1 --fields card",
		"show fx-3 --fields card,body",
		"instructions fx-3",
		"get fx-1 title",
		"path fx-1",
		"whoami",
		"version",
		"help move",
		"view board",
		"view agenda",
		"--json status",
		"--format compact list",
		"prime --brief",
		"config",
		"guide views",
		// The lines that write come last, so every read above meets the
		// fixture the two runs share whatever second each run writes in.
		`comment fx-2 "a quoted remark"`,
		`file fx-1 open_question "Is it ready?" --owner holder`,
		"mv fx-2",
		"move fx-1 acceptance",
	}
	if len(lines) < 30 {
		t.Fatalf("the test covers %d lines, and the criterion asks for at least 30", len(lines))
	}
	scratch := t.TempDir()
	snapshot := filepath.Join(scratch, "snapshot")
	restoreTree(t, root, snapshot)
	var want [][]string
	for _, line := range lines {
		words, err := verb.SplitLine(line)
		if err != nil {
			t.Fatal(err)
		}
		want = append(want, cliLines(t, root, words))
	}
	afterCLI := benchBytes(t, root)
	moved := journaled(t, root, "fx-1", contract.EventMoved)
	restoreTree(t, snapshot, root)
	results, _ := runLines(t, root, lines, "")
	for i, line := range lines {
		if i >= len(results) {
			break
		}
		if got := results[i].transcript.lines; strings.Join(got, "\n") != strings.Join(want[i], "\n") {
			t.Errorf("%s shows\n%s\nand run() writes\n%s", line, strings.Join(got, "\n"), strings.Join(want[i], "\n"))
		}
	}
	if !moved || !journaled(t, root, "fx-1", contract.EventMoved) {
		t.Error("the accepted move wrote no moved event")
	}
	if len(afterCLI) != len(benchBytes(t, root)) {
		t.Error("the command line and run() left workbenches holding different files")
	}
	t.Logf("%d lines compared", len(results))
}

// writeRawSetting writes a setting into the user's configuration as a person
// editing the file would, past the grammar config set enforces.
func writeRawSetting(t *testing.T, home, key, value string) {
	t.Helper()
	path := bench.LoadConfig(home).Path
	text, err := bench.ReadText(path)
	if err != nil {
		text = "---\n---\n"
	}
	fm, body := bench.ParseAnchor(text)
	fm.Set(key, value)
	if err := bench.WriteText(path, fm.Render(body)); err != nil {
		t.Fatal(err)
	}
}

// TestALineIsAJumpOnlyWhereNoCommandIsNamed is dinah-623/criteria/9: a line
// is a jump only when its first word is a positional naming neither a command
// nor an alias, or when it begins with @. A line beginning with a flag runs,
// a column's title jumps, @ reaches a view whose name is also a command word,
// and an alias expanding to an absent command is refused as
// dinah.not-in-tui.
func TestALineIsAJumpOnlyWhereNoCommandIsNamed(t *testing.T) {
	root := tuiBench(t)
	home := os.Getenv("DINAH_HOME")
	writeUserConfig(t, "---\n"+bench.ViewsKey+":\n  status:\n    title: A view called status\n    sections:\n      - query: \"state:ready\"\n---\n")
	if err := bench.LoadConfig(home).SetAlias("alias.srv", "serve", true); err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"--format json list", "--json status"} {
		results, run := runLines(t, root, []string{line}, "")
		if len(results) != 1 || len(results[0].transcript.lines) == 0 {
			t.Errorf("%s did not run as a command: %v", line, shownLines(run.model))
		}
	}
	run := runTUIThrough(t, root, tuiSeam(t, strings.NewReader(":Acceptance"+keyEnter+keyCtrlC), lineWidth, lineHeight))
	wantModel(t, "the lane after :Acceptance", focusedColumn(run.model), "Acceptance")
	run = runTUIThrough(t, root, tuiSeam(t, strings.NewReader(":@status"+keyEnter+keyCtrlC), lineWidth, lineHeight))
	wantModel(t, "the view after :@status", run.model.req.View, "status")
	results, run := runLines(t, root, []string{"srv"}, "")
	want := refusalShown(run.model, "serve", contract.RefuseWith(contract.NotInTUI, "serve", map[string]string{"reason": "serve"}))
	if len(results) != 1 || strings.Join(results[0].transcript.lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("the alias srv showed %v, wanted %v", shownLines(run.model), want)
	}
	run = runTUIThrough(t, root, tuiSeam(t, strings.NewReader(":zzz"+keyEnter+keyCtrlC), lineWidth, lineHeight))
	wantModel(t, "the message after :zzz", run.model.message, []string{run.model.s.r.T("interactive.line.nothing", "text", "zzz")})
}

// TestOutputModeHoldsALongTranscript is dinah-623/criteria/10: a transcript
// of more than three lines, or with a line wider than the draw width, opens
// output mode, which scrolls with card mode's keys and closes on Enter,
// Backspace or q back to the mode the line was typed from; and a transcript
// of more than 10000 lines keeps 10000 and ends with interactive.output.cut
// naming how many were dropped.
func TestOutputModeHoldsALongTranscript(t *testing.T) {
	root := tuiBench(t)
	for _, closing := range []string{keyEnter, keyBackspace, "q"} {
		run := runTUIThrough(t, root, tuiSeam(t, strings.NewReader(":status"+keyEnter+xtermDown+"j"+closing+keyCtrlC), 120, 30))
		wantModel(t, "the mode after closing output", run.model.mode, modeBrowse)
	}
	run := runTUIThrough(t, root, tuiSeam(t, strings.NewReader(":status"+keyEnter+"jj"+xtermPageDown+xtermEnd+xtermHome+"j"+keyCtrlC), 120, 12))
	wantModel(t, "the mode", run.model.mode, modeOutput)
	wantModel(t, "the offset after home and j", run.model.outputOffset, 1)
	run = runTUIThrough(t, root, tuiSeam(t, strings.NewReader(keyEnter+":status"+keyEnter+"q"+keyCtrlC), 120, 30))
	wantModel(t, "the mode output returns to", run.model.mode, modeCard)
	wide := runTUIThrough(t, root, tuiSeam(t, strings.NewReader(":serve"+keyEnter+keyCtrlC), 60, 30))
	wantModel(t, "the mode after one wide line", wide.model.mode, modeOutput)

	transcript := &lineTranscript{}
	for i := 0; i < interactiveOutputLimit+37; i++ {
		transcript.Write([]byte("line " + strconv.Itoa(i) + "\n"))
	}
	transcript.finish()
	m := run.model
	m.afterLine(&lineResult{transcript: transcript, title: "a long command"})
	wantModel(t, "the mode after a long transcript", m.mode, modeOutput)
	wantModel(t, "the lines kept", len(m.output), interactiveOutputLimit+1)
	wantModel(t, "the last line", m.output[len(m.output)-1], m.s.r.T("interactive.output.cut", "count", "37"))
}

// TestAPasteIntoTheCommandLineRunsNothing is dinah-623/criteria/13: a marked
// paste into the command line holding a line break other than one trailing
// break runs nothing, leaves the prompt's text as it was and shows
// interactive.line.paste; a paste of claim fx-1 and one line break inserts the
// text and claims nothing until Enter; and the filter still joins pasted
// lines with spaces.
func TestAPasteIntoTheCommandLineRunsNothing(t *testing.T) {
	root := tuiBench(t)
	before := benchBytes(t, root)
	run := runTUIThrough(t, root, tuiSeam(t, strings.NewReader(":keep"+pasteOpen+"claim fx-1\r\nclaim fx-2"+pasteClose+keyCtrlC), 120, 30))
	wantModel(t, "the prompt's text", run.model.input.Value(), "keep")
	wantModel(t, "the message", run.model.message, []string{run.model.s.r.T("interactive.line.paste")})
	if !sameBytes(before, benchBytes(t, root)) {
		t.Error("a paste of two lines changed the workbench")
	}
	run = runTUIThrough(t, root, tuiSeam(t, strings.NewReader(":"+pasteOpen+"claim fx-1\r\n"+pasteClose+keyCtrlC), 120, 30))
	wantModel(t, "the prompt's text", run.model.input.Value(), "claim fx-1")
	if journaled(t, root, "fx-1", contract.EventClaimed) {
		t.Error("the paste's own line break claimed fx-1")
	}
	runTUIThrough(t, root, tuiSeam(t, strings.NewReader(":"+pasteOpen+"claim fx-1\n"+pasteClose+keyEnter+keyCtrlC), 120, 30))
	if !journaled(t, root, "fx-1", contract.EventClaimed) {
		t.Error("Enter after the paste did not claim fx-1")
	}
	run = runTUIThrough(t, root, tuiSeam(t, strings.NewReader("/"+pasteOpen+"state:\r\nready"+pasteClose+keyCtrlC), 120, 30))
	wantModel(t, "the filter prompt's text", run.model.input.Value(), "state: ready")
}

// TestNothingIsRecalledAndEveryWordedCommandPassesThroughRunLine is
// dinah-623/criteria/46: this card builds no history, so Up and Down at the
// command line change nothing, . in browse mode does nothing and cannot be
// bound, every command the head runs by words (typed, bound, a read key and
// the actions menu's edit) passes through runLine, and typing lines writes
// nothing under DINAH_HOME or the workbench.
func TestNothingIsRecalledAndEveryWordedCommandPassesThroughRunLine(t *testing.T) {
	root := tuiBench(t)
	home := os.Getenv("DINAH_HOME")
	var ran []string
	record := func(seam *interactiveSeams) *interactiveSeams {
		seam.lineDone = func(result *lineResult) { ran = append(ran, strings.Join(result.words, " ")) }
		return seam
	}
	homeBefore, benchBefore := treeBytes(t, home), benchBytes(t, root)
	run := runTUIThrough(t, root, record(tuiSeam(t, strings.NewReader(":status"+keyEnter+":typed"+xtermUp+xtermDown+xtermUp+keyCtrlC), 120, 30)))
	wantModel(t, "the prompt after Up and Down", run.model.input.Value(), "typed")
	if !sameBytes(homeBefore, treeBytes(t, home)) || !sameBytes(benchBefore, benchBytes(t, root)) {
		t.Error("typing a line wrote under DINAH_HOME or the workbench")
	}
	before := benchBytes(t, root)
	run = runTUIThrough(t, root, record(tuiSeam(t, strings.NewReader("."+keyCtrlC), 120, 30)))
	wantModel(t, "the mode after .", run.model.mode, modeBrowse)
	if !sameBytes(before, benchBytes(t, root)) {
		t.Error(". changed the workbench")
	}
	if got := runCLI(t, root, "config", "set", "tui.key..", "status"); got.code == 0 || !strings.HasPrefix(got.errw, contract.InvalidKeyBinding+" ") {
		t.Errorf("a binding on . was accepted: %d %s", got.code, got.errw)
	}
	step(t, root, "config", "set", "tui.key.o", "whoami")
	t.Setenv("DINAH_EDITOR", os.Args[0])
	t.Setenv(editorRecordVar, filepath.Join(t.TempDir(), "editor.log"))
	ran = nil
	runTUIThrough(t, root, record(tuiSeam(t, strings.NewReader("o>"+keyCtrlC), 120, 30)))
	lendRun(t, root, "alka", "x"+strings.Repeat("j", menuIndexOf(t, root, "edit"))+keyEnter)
	if len(ran) < 2 || ran[0] != "whoami" || ran[1] != "next" {
		t.Errorf("the binding and the read key ran %v through runLine", ran)
	}
}

// treeBytes reads every file under a directory, which is how a test holds a
// user base unchanged.
func treeBytes(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr == nil {
			files[path] = string(data)
		}
		return nil
	})
	return files
}

// menuIndexOf is where the actions menu lists a verb over fx-1 for the
// operator.
func menuIndexOf(t *testing.T, root, verb string) int {
	t.Helper()
	seen := fixtureRun(t, root, "alka", "x")
	index := menuIndex(seen.model, verb)
	if index < 0 {
		t.Fatalf("the actions menu does not list %s", verb)
	}
	return index
}

// TestALineRunsInTheHeadsProcessAsTheHead is dinah-623/criteria/8 and /33.
// The head starts as claude under a declared harness, provider and model;
// the test then names another actor and model in the environment and empties
// PATH before typing the writing lines of the parity set and two more. Every
// one of them must open its library in this process, where lineLibrary sees
// it and replaces the library's clock, and every event it journals must carry
// that clock's stamp and the identity the head started with. A line that ran
// a second dinah would open no library here, stamp its events with the real
// time, and find no binary on PATH.
//
// The clock stands where the criterion names the Interpose hook. Interpose
// fires only in the item verbs and in reshape, and a comment fires no hook
// at all, while every event a library journals is stamped by its Now.
func TestALineRunsInTheHeadsProcessAsTheHead(t *testing.T) {
	root := tuiBench(t)
	t.Setenv("DINAH_ACTOR", "claude")
	t.Setenv("DINAH_HARNESS", "claude-code")
	t.Setenv("DINAH_PROVIDER", "acme")
	t.Setenv("DINAH_MODEL", "frontier")
	t.Setenv("PATH", os.Getenv("PATH"))
	lines := []struct {
		line, ref, event string
	}{
		{`comment fx-2 "a quoted remark"`, "fx-2", contract.EventCommented},
		{`file fx-1 open_question "Is it ready?" --owner holder`, "fx-1", contract.EventItemFiled},
		{"claim fx-2", "fx-2", contract.EventClaimed},
		{"release fx-2", "fx-2", contract.EventReleased},
		{"move fx-1 acceptance", "fx-1", contract.EventMoved},
	}
	var mu sync.Mutex
	current := -1
	opened := map[int]int{}
	done := make(chan *lineResult, len(lines)+4)
	s, seam := newScript(t, lineWidth, lineHeight, true)
	seam.lineDone = func(result *lineResult) { done <- result }
	seam.lineLibrary = func(l *verb.Library) {
		mu.Lock()
		defer mu.Unlock()
		opened[current]++
		l.Now = func() time.Time { return inProcessClock }
	}
	cycles := make(chan *tea.Program, 8)
	seam.program = func(program *tea.Program) { cycles <- program }
	var results []*lineResult
	run := s.run(root, seam, func() {
		waitForProgram(t, cycles)
		os.Setenv("DINAH_ACTOR", "other")
		os.Setenv("DINAH_MODEL", "m2")
		os.Setenv("PATH", "")
		for i, line := range lines {
			mu.Lock()
			current = i
			mu.Unlock()
			s.write(":" + line.line + keyEnter)
			select {
			case result := <-done:
				results = append(results, result)
			case <-time.After(tuiWait):
				t.Errorf("the line %q never ended", line.line)
				return
			}
		}
		s.write(keyCtrlC)
	})
	if run.model == nil {
		t.Fatalf("the run never finished: %q", run.errw)
	}
	os.Setenv("DINAH_ACTOR", "claude")
	os.Setenv("DINAH_MODEL", "frontier")
	for i, line := range lines {
		if i >= len(results) {
			t.Fatalf("ran %d of %d lines", len(results), len(lines))
		}
		if results[i].code != 0 {
			t.Errorf("%s was refused: %q", line.line, results[i].transcript.lines)
		}
		mu.Lock()
		gotOpened := opened[i]
		mu.Unlock()
		if gotOpened == 0 {
			t.Errorf("%s opened no library in this process", line.line)
		}
		events := cardEvents(t, root, line.ref)
		var last *bench.Event
		for j := range events {
			if events[j].Event == line.event {
				last = &events[j]
			}
		}
		if last == nil {
			t.Errorf("%s journaled no %s event on %s", line.line, line.event, line.ref)
			continue
		}
		if last.Actor.Name != "claude" || last.Actor.Harness != "claude-code" || last.Actor.Provider != "acme" || last.Actor.Model != "frontier" {
			t.Errorf("%s journaled %s as %+v, not as the identity the head started with", line.line, line.event, last.Actor)
		}
		if last.TS != bench.Stamp(inProcessClock) {
			t.Errorf("%s journaled %s at %s, which no library this process opened stamped", line.line, line.event, last.TS)
		}
	}
	if !t.Failed() {
		t.Logf("%d writing lines opened their library here and journaled the start identity at its clock", len(lines))
	}
}

// inProcessClock is the time a library opened by a line session reports in
// TestALineRunsInTheHeadsProcessAsTheHead, which no other writer stamps.
var inProcessClock = time.Date(2031, 3, 4, 5, 6, 7, 0, time.UTC)
