//go:build tui

package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// TestALineRefreshesTheViewAndTheOffer is dinah-623/criteria/12 and /16.
// After move fx-2 acceptance typed at the command line, the view is read
// again and the footer's offer is recomputed from the fresh read, so the act
// key pressed next carries the fresh revision and is not answered stale.
// After a line that renames a column, the next frame draws the new title
// without waiting for the change loop, because the head opens its pinned
// workbench again. With item mode open on an item a line then archives, the
// row leaves and item mode closes with interactive.items.none. The line
// archives the item rather than resolving it, because every item a card
// holds offers cite to any owner, so a resolved item keeps its row.
func TestALineRefreshesTheViewAndTheOffer(t *testing.T) {
	root := tuiBench(t)
	results, run := runLines(t, root, []string{"move fx-2 acceptance"}, keyBackspace+"t")
	if len(results) != 1 {
		t.Fatal("the move never ended")
	}
	if !journaled(t, root, "fx-1", contract.EventClaimed) {
		t.Errorf("t after the line did not claim fx-1: %v", run.model.message)
	}
	for _, line := range run.model.message {
		if strings.HasPrefix(line, contract.OutcomeStale) {
			t.Errorf("the act after the line was answered stale: %v", run.model.message)
		}
	}

	root = tuiBench(t)
	var frames []string
	s, seam := newScript(t, lineWidth, lineHeight, true)
	done := make(chan *lineResult, 2)
	seam.lineDone = func(result *lineResult) { done <- result }
	seam.frame = func(content string) { frames = append(frames, content) }
	cycles := make(chan *tea.Program, 4)
	seam.program = func(program *tea.Program) { cycles <- program }
	renamed := -1
	s.run(root, seam, func() {
		waitForProgram(t, cycles)
		s.write(":set implement title Building" + keyEnter)
		<-done
		s.write(keyCtrlL)
		s.waitFor("ctrl+l", isCtrlL)
		renamed = len(frames)
		s.write(keyCtrlC)
	})
	if renamed < 1 || !strings.Contains(frames[renamed-1], "Building") {
		t.Errorf("the frame after the line does not draw the column's new title")
	}

	root = tuiBench(t)
	step(t, root, "file", "fx-1", "open_question", "Which vendor?")
	s, seam = newScript(t, lineWidth, lineHeight, true)
	done = make(chan *lineResult, 2)
	seam.lineDone = func(result *lineResult) { done <- result }
	run = s.run(root, seam, func() {
		s.write("i")
		s.waitFor("the item key", isKey("i"))
		s.write(":archive fx-1/questions/1" + keyEnter)
		<-done
		s.write(keyCtrlC)
	})
	if run.model.mode == modeItems {
		t.Error("item mode stayed open with no item left")
	}
	if !strings.Contains(strings.Join(run.model.message, "\n"), run.model.s.r.T("interactive.items.none", "ref", "fx-1")) {
		t.Errorf("item mode closed without interactive.items.none: %v", run.model.message)
	}
}

// keyCtrlL is Ctrl+L, which draws the whole screen again.
const keyCtrlL = "\x0c"

// TestTabCompletesAsTheHead is dinah-623/criteria/15: Tab at the command line
// completes through the shell completion engine over the head's pinned
// workbench and as the head's identity. A head started with --actor naming
// the operator while DINAH_ACTOR names an agent is offered, after move and a
// card, the destinations MoveDestinations answers for the operator, which
// differ from the agent's. One candidate is inserted with a trailing space,
// a second Tab lists several, and a word completed as a file gets nothing.
func TestTabCompletesAsTheHead(t *testing.T) {
	root := tuiBench(t)
	t.Setenv("DINAH_ACTOR", "brin")
	opened, err := bench.Open(soleBenchDir(t, root))
	if err != nil {
		t.Fatal(err)
	}
	l := verb.New(opened, os.Getenv("DINAH_HOME"))
	operator, _ := l.MoveDestinations(&verb.Request{Verb: verb.Move, Card: "fx-3", Actor: "alka"})
	agent, _ := l.MoveDestinations(&verb.Request{Verb: verb.Move, Card: "fx-3", Actor: "brin"})
	if len(operator) < 2 || len(operator) == len(agent) {
		t.Fatalf("the fixture's destinations do not differ: operator %v, agent %v", operator, agent)
	}
	want := operator[0].Ref + "  " + operator[0].Title
	var listed [][]string
	var values []string
	keys := ":move fx-3 \t\t" + keyCtrlG + ":stat\t" + keyCtrlG + ":attach fx-1 \t\t" + keyCtrlC
	seam := tuiSeam(t, strings.NewReader(keys), lineWidth, lineHeight)
	seam.observe = func(m *interactiveModel, msg tea.Msg) {
		pressed, ok := msg.(keyMsg)
		if ok && (pressed.key.String() == "ctrl+g" || pressed.key.String() == "ctrl+c") {
			listed = append(listed, append([]string{}, m.message...))
			values = append(values, m.input.Value())
		}
	}
	run := runTUIThrough(t, root, seam, "--actor", "alka")
	if run.model == nil || len(listed) != 3 {
		t.Fatalf("the run left %d listings: %q", len(listed), run.errw)
	}
	if len(listed[0]) == 0 || listed[0][0] != want {
		t.Errorf("the second Tab listed %q, and the operator's first destination is %q", listed[0], want)
	}
	if values[1] != "status " {
		t.Errorf("one candidate left the prompt at %q, wanted %q", values[1], "status ")
	}
	if values[2] != "attach fx-1 " || len(listed[2]) != 0 {
		t.Errorf("a file word was completed: %q %v", values[2], listed[2])
	}
}

// TestTheDetailPaneIsTheCLIsUnresolvedShow is dinah-623/criteria/23: the
// detail pane shows exactly the lines dinah show <ref> --fields
// card,body,checklist --unresolved prints at the pane's width.
func TestTheDetailPaneIsTheCLIsUnresolvedShow(t *testing.T) {
	root := tuiBench(t)
	step(t, root, "file", "fx-1", "open_question", "Which vendor?")
	step(t, root, "file", "fx-1", "decision", "Decided already")
	step(t, root, "resolve", "fx-1/decisions/1", "--text", "settled")
	run := runTUIThrough(t, root, tuiSeam(t, strings.NewReader(keyCtrlC), lineWidth, lineHeight))
	width := lineDraw - interactiveListWidth(lineDraw) - 1
	t.Setenv("COLUMNS", strconv.Itoa(width))
	want := cliLines(t, root, []string{"show", "fx-1", "--fields", "card,body,checklist", "--unresolved"})
	shown := strings.Join(run.model.detail, "\n")
	if shown != strings.Join(want, "\n") {
		t.Errorf("the detail pane shows\n%s\nand dinah show prints\n%s", shown, strings.Join(want, "\n"))
	}
	if !strings.Contains(shown, "Which vendor?") || strings.Contains(shown, "Decided already") {
		t.Errorf("the detail pane does not list the unresolved item alone:\n%s", shown)
	}
}

// TestNoControlCharacterReachesAnyMode is dinah-623/criteria/24: a fixture
// whose item text, card title, column title, scheme name and command output
// carry ESC, BEL and CR draws no control character in any frame of browse,
// card, item, prompt and output mode.
func TestNoControlCharacterReachesAnyMode(t *testing.T) {
	root := newBenchFromDefinition(t, evidenceStatesDefinition)
	planted := "a\x1b[31mred\x07bell\rreturn"
	step(t, root, "add", "Title "+planted)
	step(t, root, "move", "fx-1", "doing")
	step(t, root, "file", "fx-1", "acceptance_criterion", "Item "+planted)
	renameColumn(t, root, "Doing", `"Doing a\e[31mred\abell\rreturn"`)
	anchor := filepath.Join(soleBenchDir(t, root), bench.WorkbenchAnchor)
	editWorkbenchAnchor(t, anchor, "evidence:", "evidence:\n  \"s\\e[31m\\a\":\n    hint: planted\n")
	var frames []string
	keys := "i" + "c" + keyCtrlG + keyCtrlG + keyEnter + ":show fx-1" + keyEnter + "q" + ":comment fx-1" + keyCtrlG + keyCtrlC
	seam := tuiSeam(t, strings.NewReader(keys), lineWidth, lineHeight)
	seam.frame = func(content string) { frames = append(frames, content) }
	run := runTUIThrough(t, root, seam)
	if run.model == nil || len(frames) == 0 {
		t.Fatalf("the run drew no frame: %q", run.errw)
	}
	modes := map[string]bool{}
	for _, frame := range frames {
		for _, planted := range []string{"\x1b[31mred", "\x07", "\r"} {
			if strings.Contains(frame, planted) {
				t.Fatalf("a frame carries %q:\n%q", planted, frame)
			}
		}
		switch {
		case strings.Contains(frame, "items of fx-1"):
			modes["items"] = true
		case strings.Contains(frame, "evidence scheme for"):
			modes["menu"] = true
		case strings.Contains(frame, "output of show fx-1"):
			modes["output"] = true
		case strings.Contains(frame, ": comment fx-1"):
			modes["prompt"] = true
		}
	}
	t.Logf("%d frames read; modes seen %v", len(frames), modes)
	for _, mode := range []string{"items", "menu", "output", "prompt"} {
		if !modes[mode] {
			t.Errorf("the frames never reached %s", mode)
		}
	}
}

// TestTheReadKeysRunThroughTheLine is dinah-623/criteria/31: >, S, F, C and
// W are listed in the browse and card footers after :, and each runs its
// line through runLine.
func TestTheReadKeysRunThroughTheLine(t *testing.T) {
	root := tuiBench(t)
	for _, mode := range []string{"", keyEnter} {
		run := fixtureRun(t, root, "alka", mode)
		footer := run.model.footerText()
		colon := strings.Index(footer, actKeyOf(run.model, "view"))
		if colon < 0 {
			t.Errorf("the footer does not list the command line:\n%s", footer)
		}
		for _, name := range []string{"next", "status", "search", "changes", "whoami"} {
			at := strings.Index(footer, actKeyOf(run.model, name))
			if at < 0 || at < colon {
				t.Errorf("the footer does not list %s after the command line:\n%s", name, footer)
			}
		}
	}
	var words []string
	keys := ">" + keyBackspace + "S" + keyBackspace + "Fparser" + keyEnter + keyBackspace + "C" + keyBackspace + "W" + keyCtrlC
	seam := tuiSeam(t, strings.NewReader(keys), lineWidth, lineHeight)
	seam.lineDone = func(result *lineResult) { words = append(words, strings.Join(result.words, " ")) }
	runTUIThrough(t, root, seam)
	want := []string{"next", "status", "search parser", "changes --card fx-1", "whoami"}
	if strings.Join(words, "|") != strings.Join(want, "|") {
		t.Errorf("the read keys ran %q, wanted %q", words, want)
	}
}

// TestAnArchivedCardIsReachedAndRestored is dinah-623/criteria/40: typing :
// and an archived card's reference opens card mode over the archived card,
// whose actions menu lists restore it and nothing else, and choosing it puts
// the card back in the live set.
func TestAnArchivedCardIsReachedAndRestored(t *testing.T) {
	root := tuiBench(t)
	step(t, root, "archive", "fx-1")
	seen := fixtureRun(t, root, "alka", ":fx-1"+keyEnter+"x")
	wantModel(t, "the mode", seen.model.mode, modeMenu)
	var rows []string
	for _, row := range seen.model.menu.rows {
		rows = append(rows, row.label)
	}
	if strings.Join(rows, "|") != menuLabel(seen.model, "restore") {
		t.Errorf("the archived card's actions menu lists %v", rows)
	}
	fixtureRun(t, root, "alka", ":fx-1"+keyEnter+"x1")
	if got := runCLI(t, root, "show", "fx-1", "--fields", "card"); got.code != 0 {
		t.Errorf("restore it left fx-1 archived: %s", got.errw)
	}
}
