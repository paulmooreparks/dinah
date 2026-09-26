//go:build tui

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"

	"dinah/internal/bench"
	"dinah/internal/msg"
)

// The two sequences a planted title carries: an OSC that would set the
// window's title, and an SGR that would blink and reverse the text in red.
const (
	plantedOSC   = "\x1b]0;pwned\x07"
	plantedSGR   = "\x1b[5;7;31m"
	plantedTitle = plantedOSC + plantedSGR + "Planted title"
)

// TestNoPlantedEscapeReachesTheTerminal is dinah-603/criteria/13. A card
// title, the workbench's title, a column's title, the actor's name and the
// declared model all carry an OSC and an SGR ahead of the text Planted
// title, and the card is shown in a lane, the detail pane, card mode, the
// move menu and a change text. Neither sequence appears anywhere in what the
// program wrote, the text Planted title appears in the lane row and the
// detail pane, and under NO_COLOR the model's drawn text holds no ESC at all.
func TestNoPlantedEscapeReachesTheTerminal(t *testing.T) {
	for _, noColour := range []bool{false, true} {
		t.Run(fmt.Sprintf("NO_COLOR %v", noColour), func(t *testing.T) {
			root := tuiBench(t)
			step(t, root, "set", "fx-1", "title", plantedTitle)
			anchor := filepath.Join(benchDir(t, root), bench.WorkbenchAnchor)
			editWorkbenchAnchor(t, anchor, "title: Terminal\n", "title: \""+strings.ReplaceAll(plantedTitle, "\x1b", "\\e")+" workbench\"\n")
			renameColumn(t, root, "Implement", "\"Implement "+strings.ReplaceAll(plantedTitle, "\x1b", "\\e")+"\"")
			t.Setenv("DINAH_ACTOR", "alka"+plantedSGR)
			t.Setenv("DINAH_PROVIDER", "acme")
			t.Setenv("DINAH_MODEL", "model"+plantedOSC)
			if noColour {
				t.Setenv("NO_COLOR", "1")
			}
			s, seam := newScript(t, 110, 30, true)
			s.when("Z", func() { step(t, root, "comment", "fx-1", "changed") })
			var browse string
			run := s.run(root, seam, func() {
				s.write(keyEnter + keyEnter + "m" + keyCtrlG + "Z")
				s.waitFor("the change", isChange)
				s.write(keyCtrlC)
			})
			m := run.model
			if m == nil {
				t.Fatalf("the run never finished: %q", run.errw)
			}
			browse = m.content()
			// Bubble Tea's renderer re-encodes styled text, so the output
			// alone could hide a planted sequence the head let through. The
			// head's own drawing is held to carrying neither as well.
			for _, planted := range []string{plantedOSC, plantedSGR} {
				if strings.Contains(run.output, planted) {
					t.Errorf("the output carries the planted sequence %q", planted)
				}
				if strings.Contains(browse, planted) {
					t.Errorf("the head's drawing carries the planted sequence %q", planted)
				}
			}
			rows := strings.Split(browse, "\n")
			listed := false
			for _, row := range rows {
				if strings.Contains(row, "Planted title") && strings.Contains(row, "›") {
					listed = true
				}
			}
			if !listed {
				t.Errorf("no lane row carries the text Planted title:\n%s", browse)
			}
			if !strings.Contains(strings.Join(m.detail, "\n"), "Planted title") {
				t.Errorf("the detail pane does not carry the text Planted title: %q", m.detail)
			}
			if noColour && strings.Contains(browse, "\x1b") {
				t.Errorf("with the Ascii profile the drawn text carries an ESC byte:\n%q", browse)
			}
		})
	}
}

// renameColumn sets the title of the column with a title, written as YAML.
func renameColumn(t *testing.T, root, title, yaml string) {
	t.Helper()
	columns := filepath.Join(soleBenchDir(t, root), bench.ColumnsDir)
	ids, err := bench.ListIDs(columns)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		path := filepath.Join(columns, id, bench.ColumnAnchor)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "title: "+title+"\n") {
			text := strings.Replace(string(data), "title: "+title+"\n", "title: "+yaml+"\n", 1)
			if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no column is titled %s", title)
}

// endsRestored fails unless the output ends with the alternate screen left
// and the cursor shown after the last frame, followed by the bracketed-paste
// disable, which is the last thing written, and unless paste is never
// switched off while a frame is on the screen.
func endsRestored(t *testing.T, output string) {
	t.Helper()
	left := strings.LastIndex(output, "\x1b[?1049l")
	shown := strings.LastIndex(output, "\x1b[?25h")
	disabled := strings.LastIndex(output, "\x1b[?2004l")
	if left < 0 || shown < left || disabled < shown {
		t.Errorf("the output ends %q, wanted ESC [?1049l, then ESC [?25h, then ESC [?2004l", output[max(0, len(output)-80):])
	}
	if !strings.HasSuffix(output, "\x1b[?2004l") {
		t.Errorf("the output does not end with ESC [?2004l: %q", output[max(0, len(output)-80):])
	}
	if why := pasteOffOnTheAlternateScreen(output); why != "" {
		t.Error(why)
	}
}

// pasteOffOnTheAlternateScreen answers where an output switches bracketed
// paste off while a program's frames are on the screen, and nothing where it
// never does. Paste may be switched off only once the alternate screen has
// been left, so at every ESC [?2004l the later of the last ESC [?1049h and
// the last ESC [?1049l before it must be the leave. A disable written while
// the screen is up leaves every paste until the next program unmarked, which
// is what dinah-623/criteria/52 refuses.
func pasteOffOnTheAlternateScreen(output string) string {
	for at := 0; ; {
		found := strings.Index(output[at:], "\x1b[?2004l")
		if found < 0 {
			return ""
		}
		found += at
		entered, left := strings.LastIndex(output[:found], "\x1b[?1049h"), strings.LastIndex(output[:found], "\x1b[?1049l")
		if entered > left {
			return fmt.Sprintf("ESC [?2004l is written at byte %d while the alternate screen entered at byte %d is up, so pastes are no longer marked: %q", found, entered, output[entered:min(len(output), found+40)])
		}
		at = found + 1
	}
}

// TestCtrlCInTheMoveMenuQuitsAndRestores is dinah-603/criteria/14: ctrl+c
// with the move menu open exits 0 with no move made, and the output ends with
// the terminal restored and bracketed paste switched off once; q in browse
// ends the same way.
func TestCtrlCInTheMoveMenuQuitsAndRestores(t *testing.T) {
	root := tuiBench(t)
	anchor, journal := anchorText(t, root, "fx-1"), journalText(t, root, "fx-1")
	menu := runTUIThrough(t, root, tuiSeam(t, strings.NewReader("mj"+keyCtrlC), 100, 30))
	if menu.code != 0 {
		t.Errorf("ctrl+c in the menu exited %d: %s", menu.code, menu.errw)
	}
	if anchorText(t, root, "fx-1") != anchor || journalText(t, root, "fx-1") != journal {
		t.Error("ctrl+c in the menu moved the card")
	}
	endsRestored(t, menu.output)
	quit := runTUIThrough(t, root, tuiSeam(t, strings.NewReader("q"), 100, 30))
	if quit.code != 0 {
		t.Errorf("q exited %d", quit.code)
	}
	endsRestored(t, quit.output)
}

// TestALoneEscChangesNothingInAnyMode is the seam half of
// dinah-603/criteria/15: a lone ESC written in browse, card mode, the move
// menu and each prompt leaves the mode, the selection, the highlight and the
// prompt's text as they were, performs no act and does not quit. Each script
// is run twice, with the ESC and without it, and the two final models and the
// workbench must agree; the ESC is followed by the ctrl+c that ends the run,
// which the decoder delivers with the ESC dropped.
func TestALoneEscChangesNothingInAnyMode(t *testing.T) {
	scripts := map[string]string{
		"browse":             "lj",
		"card mode":          "j" + keyEnter,
		"the move menu":      "mj",
		"the jump prompt":    ":fx",
		"the filter prompt":  "/state",
		"the comment prompt": "cnote",
	}
	type outcome struct {
		mode       interactiveMode
		lane, card string
		highlight  int
		input      string
		area       string
		journal    string
	}
	read := func(t *testing.T, keys string) outcome {
		root := tuiBench(t)
		run := runTUIThrough(t, root, tuiSeam(t, strings.NewReader(keys), 100, 30))
		m := run.model
		if m == nil {
			t.Fatalf("the run of %q never finished: %q", keys, run.errw)
		}
		var events []string
		for _, ref := range []string{"fx-1", "fx-2", "fx-3"} {
			for _, event := range cardEvents(t, root, ref) {
				events = append(events, ref+" "+event.Event)
			}
		}
		return outcome{m.mode, focusedColumn(m), selectedRef(m), m.menuHighlight(), m.input.Value(), m.area.Value(), strings.Join(events, ", ")}
	}
	for name, keys := range scripts {
		t.Run(name, func(t *testing.T) {
			without := read(t, keys+keyCtrlC)
			with := read(t, keys+"\x1b"+keyCtrlC)
			if with != without {
				t.Errorf("with a lone ESC the run ended %+v, and without it %+v", with, without)
			}
		})
	}
}

// panicSite is where a crash test plants its panic.
type panicSite struct {
	name  string
	value string
	plant func(seam *interactiveSeams, once *sync.Once)
}

// TestAPanicRestoresTheTerminalAndReportsIt is dinah-603/criteria/17: a
// panic planted in Update, in a command and in View each ends the program
// with the terminal restored and bracketed paste switched off once after it,
// writes interactive.crashed, the panic's value and the captured stack to
// stderr, and reaches the seam's repanic with the original value.
func TestAPanicRestoresTheTerminalAndReportsIt(t *testing.T) {
	sites := []panicSite{
		{name: "Update", value: "planted in Update", plant: func(seam *interactiveSeams, once *sync.Once) {
			seam.update = func(msg tea.Msg) {
				if _, ok := msg.(keyMsg); ok {
					once.Do(func() { panic("planted in Update") })
				}
			}
		}},
		{name: "a command", value: "planted in a command", plant: func(seam *interactiveSeams, once *sync.Once) {
			seam.command = func() { once.Do(func() { panic("planted in a command") }) }
		}},
		{name: "View", value: "planted in View", plant: func(seam *interactiveSeams, once *sync.Once) {
			seam.view = func() { once.Do(func() { panic("planted in View") }) }
		}},
	}
	for _, site := range sites {
		t.Run(site.name, func(t *testing.T) {
			root := tuiBench(t)
			seam := tuiSeam(t, strings.NewReader("j"), 100, 30)
			var once sync.Once
			site.plant(seam, &once)
			var repanicked any
			seam.repanic = func(value any) { repanicked = value }
			run := runTUIThrough(t, root, seam)
			want := site.value
			if repanicked != want {
				t.Errorf("repanic received %v, wanted %q", repanicked, want)
			}
			endsRestored(t, run.output)
			lines := strings.Split(run.errw, "\n")
			crashed := msg.For(msg.Base).T("interactive.crashed")
			if len(lines) < 4 || lines[0] != crashed || lines[1] != want {
				t.Fatalf("stderr is %q, wanted the crash sentence, then the value, then the stack", run.errw)
			}
			if !strings.Contains(run.errw, "runtime/debug.Stack") {
				t.Errorf("stderr carries no captured stack: %q", run.errw)
			}
		})
	}
}

// TestTheFilterAndTheJump is dinah-603/criteria/20: the filter narrows the
// lanes to its matches and is reapplied on every refresh; a query the library
// refuses shows its refusal and keeps the prompt open holding the text as
// typed; an empty text clears the filter; and a jump resolves a view name,
// then a card, then a column, showing interactive.jump.nothing when none
// matches.
func TestTheFilterAndTheJump(t *testing.T) {
	t.Run("narrows and is reapplied on a refresh", func(t *testing.T) {
		root := tuiBench(t)
		s, seam := newScript(t, 100, 30, true)
		s.when("Z", func() { step(t, root, "move", "fx-1", "acceptance") })
		run := s.run(root, seam, func() {
			s.write("/column:acceptance" + keyEnter + "Z")
			s.waitFor("the change", isChange)
			s.write(keyCtrlC)
		})
		m := run.model
		if m == nil {
			t.Fatalf("the run never finished: %q", run.errw)
		}
		if len(m.lanes) != 1 || len(m.lanes[0].cards) != 4 {
			t.Errorf("after the refresh the filter left %d lanes, the first holding %d cards; wanted Acceptance alone holding four", len(m.lanes), len(m.lanes[0].cards))
		}
	})
	t.Run("a refused query keeps the prompt open", func(t *testing.T) {
		root := tuiBench(t)
		run := runTUIThrough(t, root, tuiSeam(t, strings.NewReader("/bogus:x"+keyEnter+keyCtrlC), 100, 30))
		m := run.model
		wantModel(t, "the mode", m.mode, modePrompt)
		wantModel(t, "the prompt's text", m.input.Value(), "bogus:x")
		wantModel(t, "the filter", m.filter, "")
		if len(m.message) == 0 || !strings.HasPrefix(m.message[0], "dinah.") && !strings.Contains(m.message[0], "bogus") {
			t.Errorf("the message area shows %q, wanted the query's refusal", m.message)
		}
	})
	t.Run("an empty text clears the filter", func(t *testing.T) {
		root := tuiBench(t)
		run := runTUIThrough(t, root, tuiSeam(t, strings.NewReader("/column:acceptance"+keyEnter+"/\x15"+keyEnter+"q"), 100, 30))
		wantModel(t, "the filter", run.model.filter, "")
		wantModel(t, "the lanes", len(run.model.lanes), 2)
	})
	jumps := []struct {
		text  string
		check func(t *testing.T, m *interactiveModel)
	}{
		{"agenda", func(t *testing.T, m *interactiveModel) { wantModel(t, "the view", m.req.View, "agenda") }},
		{"fx-4", func(t *testing.T, m *interactiveModel) {
			wantModel(t, "the lane", focusedColumn(m), "Acceptance")
			wantModel(t, "the selection", selectedRef(m), "fx-4")
		}},
		{"acceptance", func(t *testing.T, m *interactiveModel) { wantModel(t, "the lane", focusedColumn(m), "Acceptance") }},
		{"done", func(t *testing.T, m *interactiveModel) {
			wantModel(t, "the message", m.message, []string{m.s.r.T("interactive.jump.no-lane", "column", "Done")})
		}},
		{"zzz", func(t *testing.T, m *interactiveModel) {
			wantModel(t, "the message", m.message, []string{m.s.r.T("interactive.line.nothing", "text", "zzz")})
		}},
	}
	for _, jump := range jumps {
		t.Run("jump "+jump.text, func(t *testing.T) {
			root := tuiBench(t)
			run := runTUIThrough(t, root, tuiSeam(t, strings.NewReader(":"+jump.text+keyEnter+keyCtrlC), 100, 30))
			if run.model == nil {
				t.Fatalf("the run never finished: %q", run.errw)
			}
			jump.check(t, run.model)
		})
	}
	t.Run("a view name wins over a column of the same name", func(t *testing.T) {
		root := tuiBench(t)
		writeUserConfig(t, "---\n"+bench.ViewsKey+":\n  acceptance:\n    title: Acceptance view\n    sections:\n      - query: \"state:ready\"\n---\n")
		run := runTUIThrough(t, root, tuiSeam(t, strings.NewReader(":acceptance"+keyEnter+keyCtrlC), 100, 30))
		wantModel(t, "the view", run.model.req.View, "acceptance")
	})
	t.Run("a card wins over a column", func(t *testing.T) {
		root := tuiBench(t)
		run := runTUIThrough(t, root, tuiSeam(t, strings.NewReader(":fx-2"+keyEnter+keyCtrlC), 100, 30))
		wantModel(t, "the selection", selectedRef(run.model), "fx-2")
	})
}

// wideScriptTitle is a title in a script whose characters take two columns.
const wideScriptTitle = "一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十"

// TestEveryRowFitsTheWindow is dinah-603/criteria/23: at widths 60, 99, 100
// and 160, with titles in a wide script, every row of the heading, the lane
// bar, the list pane, the detail pane, the message area and the footer, with
// the short help and with the full help, is at most one column narrower than
// the window by Dinah's own measure.
func TestEveryRowFitsTheWindow(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	for _, width := range []int{60, 99, 100, 160} {
		for _, keys := range []string{"q", "?" + keyCtrlC} {
			root := tuiBench(t)
			step(t, root, "set", "fx-1", "title", wideScriptTitle)
			step(t, root, "set", "fx-2", "title", "Latin "+wideScriptTitle)
			run := runTUIThrough(t, root, tuiSeam(t, strings.NewReader(keys), width, 30))
			if run.model == nil {
				t.Fatalf("the run at %d never finished: %q", width, run.errw)
			}
			rows := strings.Split(run.model.content(), "\n")
			if len(rows) != 30 {
				t.Errorf("at %d the screen has %d rows, wanted 30", width, len(rows))
			}
			for i, row := range rows {
				if got := displayWidth(row); got > width-1 {
					t.Errorf("at width %d, row %d draws %d columns: %q", width, i+1, got, row)
				}
			}
		}
	}
}

// controlSequence matches one control sequence, OSC string or escape the
// program wrote, for the set test below.
var controlSequence = regexp.MustCompile("\x1b\\[[0-9;?<>=]*[ -/]*[@-~]|\x1b\\][^\x07\x1b]*(\x07|\x1b\\\\)|\x1b[@-Z\\\\-_]")

// allowedSequence matches the sequences the program may write: the
// alternate screen, the cursor's visibility and bracketed paste switched on
// and off, SGR, and the cursor, erase, text-modification, scrolling and
// tab sequences Bubble Tea's renderer chooses among to update a frame:
// cursor up, down, forward, back, next line, previous line, absolute column
// and absolute row, cursor position, erase in display and in line, insert
// and delete character, erase character, insert and delete line, scroll up
// and down, backward and forward tab, the scrolling margins, and reverse
// index. Each is listed on Microsoft's "Console Virtual Terminal Sequences"
// page. Which of them a frame uses depends on TERM, which the seam sets to
// xterm, and on the platform, because Bubble Tea turns its scroll
// optimization off on Windows. dinah-603/decisions/10 records the reading.
var allowedSequence = regexp.MustCompile(`^(\x1b\[(\?1049[hl]|\?25[hl]|\?2004[hl]|[0-9;]*[ABCDEFGHJKLMPSTXZdfmr@I])|\x1bM)$`)

// undocumentedSequence matches the sequences the renderer knows and
// Microsoft's page does not list: repeat the previous character, horizontal
// position absolute, insert mode and autowrap. The renderer enables the first
// two only for terminals other than xterm and an unset TERM, and writes the
// last two only when it inserts characters without ICH or fills the
// lower-right cell, which no frame of the head reaches.
var undocumentedSequence = regexp.MustCompile(`^\x1b\[([0-9]*b|[0-9]*` + "`" + `|4[hl]|\?7[hl])$`)

// everyModeKeys open card mode and leave it, open the move menu and cancel
// it, and open the jump prompt, with a ctrl+v typed in it, the filter prompt
// and the comment prompt, cancelling each.
const everyModeKeys = keyEnter + keyEnter + "m" + keyCtrlG + ":" + "\x16" + keyCtrlG + "/x" + keyCtrlG + "cy" + keyCtrlG

// everyModeRun is the headless run the sequence tests sweep: at width by
// height it types everyModeKeys, resizes the window to one ten columns
// narrower and five rows shorter and back, opens the full help, and quits
// with q. output, where it is not nil, is where the program writes, and
// frame, where it is not nil, is called with the content of every frame.
func everyModeRun(t *testing.T, root string, width, height int, output io.Writer, frame func(string)) tuiRun {
	t.Helper()
	s, seam := newScript(t, width, height, true)
	seam.output = output
	seam.frame = frame
	return s.run(root, seam, func() {
		s.write(everyModeKeys)
		s.waitFor("every key of the modes", countKeys(len(everyModeKeys)))
		seam.resize(width-10, height-5)
		s.send(resizeMsg{})
		s.waitFor("the smaller window", isSize(width-10, height-5))
		seam.resize(width, height)
		s.send(resizeMsg{})
		s.waitFor("the window restored", isSize(width, height))
		s.write("?")
		s.waitFor("?", isKey("?"))
		s.write("q")
	})
}

// TestTheProgramWritesOnlyTheSequencesItMay is dinah-603/criteria/24. A
// headless run of the program built with interactiveProgramOptions for the
// running GOOS, through every mode, the help, a resize and each prompt,
// writes no control sequence outside the allowed set and no DECRQM, mouse,
// focus, modifyOtherKeys or kitty keyboard sequence; ESC [?2004h is the first
// thing written and ESC [?2004l the last, both Dinah's own, and paste is
// never switched off while a frame is up (Bubble Tea may also write the
// enable, since every frame asks for the mode on, as dinah-623/criteria/52
// explains); and off Windows
// it writes no REP, HPA, insert-mode or autowrap sequence, the Windows
// configuration being held by TestAnInsertedRunNeverUsesInsertMode and
// TestTheWindowsOutputIsOnlyWhatMicrosoftLists. ctrl+v in a prompt inserts
// nothing, and neither prompt's key map reaches its component's Paste, and
// under NO_COLOR the output carries no SGR colour parameter while the
// selected row still carries its marker.
func TestTheProgramWritesOnlyTheSequencesItMay(t *testing.T) {
	root := tuiBench(t)
	run := everyModeRun(t, root, 100, 30, nil, nil)
	if run.model == nil {
		t.Fatalf("the run never finished: %q", run.errw)
	}
	counts := map[string]int{}
	for _, sequence := range controlSequence.FindAllString(run.output, -1) {
		counts[sequence]++
		if !allowedSequence.MatchString(sequence) {
			t.Errorf("the program wrote %q, which is outside the allowed set", sequence)
		}
		if undocumentedSequence.MatchString(sequence) {
			t.Errorf("the program wrote %q, which Microsoft's console page does not list", sequence)
		}
	}
	t.Logf("%d distinct sequences written", len(counts))
	if !strings.HasPrefix(run.output, "\x1b[?2004h") {
		t.Errorf("the output does not begin with ESC [?2004h: %q", run.output[:min(len(run.output), 40)])
	}
	endsRestored(t, run.output)
	for _, forbidden := range []string{"$p", "\x1b[?1000", "\x1b[?1002", "\x1b[?1003", "\x1b[?1006", "\x1b[?1004", "\x1b[>4", "\x1b[>1u", "\x1b[<u", "\x1b[?u"} {
		if strings.Contains(run.output, forbidden) {
			t.Errorf("the output carries %q", forbidden)
		}
	}

	pasted := runTUIThrough(t, tuiBench(t), tuiSeam(t, strings.NewReader(":\x16"+keyCtrlC), 100, 30))
	wantModel(t, "the jump prompt after ctrl+v", pasted.model.input.Value(), "")
	area := runTUIThrough(t, tuiBench(t), tuiSeam(t, strings.NewReader("c\x16"+keyCtrlC), 100, 30))
	wantModel(t, "the comment prompt after ctrl+v", area.model.area.Value(), "")
	if interactiveInputKeys().Paste.Enabled() || interactiveAreaKeys().Paste.Enabled() {
		t.Error("a prompt's key map reaches its component's Paste")
	}

	t.Setenv("NO_COLOR", "1")
	plain := runTUIThrough(t, tuiBench(t), tuiSeam(t, strings.NewReader("q"), 100, 30))
	colour := regexp.MustCompile(`\x1b\[([0-9;]*;)?(3[0-9]|4[0-9]|9[0-7]|10[0-7])(;[0-9;]*)?m`)
	if found := colour.FindString(plain.output); found != "" {
		t.Errorf("under NO_COLOR the output carries the colour %q", found)
	}
	if !strings.Contains(plain.model.content(), "› ") {
		t.Error("under NO_COLOR the selected row lost its marker")
	}
}

// TestAPasteIsTextInAPromptAndNothingElsewhere is dinah-603/criteria/27. A
// marked paste of looks fine, CR LF, but a question: into the comment prompt
// holds both lines and ctrl+d posts them with the line break; into the jump
// and filter prompts holds them on one line and submits nothing; and in
// browse mode, card mode and the move menu it is discarded, performing no act
// and leaving the card's anchor and journal byte-identical.
func TestAPasteIsTextInAPromptAndNothingElsewhere(t *testing.T) {
	pasted := pasteOpen + "looks fine\r\nbut a question" + pasteClose
	root := tuiBench(t)
	run := runTUIThrough(t, root, tuiSeam(t, strings.NewReader("c"+pasted+keyCtrlD+"q"), 100, 30))
	if comments := commentBodies(t, root, "fx-1"); len(comments) != 1 || comments[0] != "looks fine\nbut a question" {
		t.Errorf("the comment posted is %q", comments)
	}
	_ = run
	root = tuiBench(t)
	run = runTUIThrough(t, root, tuiSeam(t, strings.NewReader("/"+pasted+keyCtrlC), 100, 30))
	wantModel(t, "/ prompt's text", run.model.input.Value(), "looks fine but a question")
	wantModel(t, "/ prompt's mode", run.model.mode, modePrompt)
	// The command line takes a paste of one line alone, which
	// TestAPasteIntoTheCommandLineRunsNothing holds; a paste of two lines is
	// discarded there and leaves the prompt open and empty.
	root = tuiBench(t)
	run = runTUIThrough(t, root, tuiSeam(t, strings.NewReader(":"+pasted+keyCtrlC), 100, 30))
	wantModel(t, ": prompt's text", run.model.input.Value(), "")
	wantModel(t, ": prompt's mode", run.model.mode, modePrompt)
	for name, keys := range map[string]string{"browse": "", "card mode": keyEnter, "the move menu": "m"} {
		root := tuiBench(t)
		anchor, journal := anchorText(t, root, "fx-1"), journalText(t, root, "fx-1")
		run := runTUIThrough(t, root, tuiSeam(t, strings.NewReader(keys+pasteOpen+"q\r\namc"+pasteClose+keyCtrlC), 100, 30))
		if anchorText(t, root, "fx-1") != anchor || journalText(t, root, "fx-1") != journal {
			t.Errorf("a paste in %s acted on fx-1", name)
		}
		if run.model == nil || !run.model.quitting {
			t.Errorf("a paste in %s did not leave the head running until ctrl+c", name)
		}
	}
}

// TestTheSizeComesFromTheOneMeasure is dinah-603/criteria/32: with COLUMNS
// and LINES set, the first frame is laid out at the seam's size; and a size
// change seen by the one-second poll with no event delivers one WindowSizeMsg
// and redraws at the new size.
func TestTheSizeComesFromTheOneMeasure(t *testing.T) {
	t.Setenv("COLUMNS", "200")
	t.Setenv("LINES", "80")
	root := tuiBench(t)
	s, seam := newScript(t, 100, 30, true)
	sized := 0
	var mu sync.Mutex
	user := seam.update
	seam.update = func(msg tea.Msg) {
		if size, ok := msg.(tea.WindowSizeMsg); ok && size.Width == 120 {
			mu.Lock()
			sized++
			mu.Unlock()
		}
		user(msg)
	}
	var first string
	run := s.run(root, seam, func() {
		s.write("j")
		s.waitFor("j", isKey("j"))
		seam.resize(120, 40)
		s.waitFor("the size the poll found", isSize(120, 40))
		s.waitFor("the next wait", func(msg tea.Msg) bool { _, ok := msg.(changeMsg); return ok })
		s.write(keyCtrlC)
	})
	m := run.model
	if m == nil {
		t.Fatalf("the run never finished: %q", run.errw)
	}
	first = strings.SplitN(visible(strings.ReplaceAll(run.output, "\x1b[B", "\n")), "\n", 2)[0]
	if displayWidth(first) > 99 {
		t.Errorf("the first frame's first row draws %d columns, which is wider than the seam's 100 less one", displayWidth(first))
	}
	mu.Lock()
	defer mu.Unlock()
	wantModel(t, "the WindowSizeMsg deliveries of 120 by 40", sized, 1)
	wantModel(t, "the model's size", [2]int{m.width, m.height}, [2]int{120, 40})
	if rows := strings.Split(m.content(), "\n"); len(rows) != 40 {
		t.Errorf("the redraw has %d rows, wanted 40", len(rows))
	}
}

// TestTheFlushFallbackDropsTheKeyTypedAfterEnter is dinah-603/criteria/35.
// With no paste marker ever written, enter in the jump prompt followed at
// once by a, as one burst, submits the jump and asks the reader to flush; the
// a is dropped, the card is untouched, and the reader records the flush.
// After the seam has delivered one paste start marker, the same burst submits
// the jump and delivers the a as a key, with no flush asked for.
func TestTheFlushFallbackDropsTheKeyTypedAfterEnter(t *testing.T) {
	burst := ":fx-4" + keyEnter + "a"
	root := tuiBench(t)
	s, seam := newScript(t, 100, 30, false)
	run := s.run(root, seam, func() {
		s.write(burst)
		s.waitFor("the flush", func(msg tea.Msg) bool { _, ok := msg.(flushedMsg); return ok })
		s.write(keyCtrlC)
	})
	if run.model == nil {
		t.Fatalf("the run never finished: %q", run.errw)
	}
	wantModel(t, "the flushes the reader made", run.flushes, 1)
	wantModel(t, "the selection", selectedRef(run.model), "fx-4")
	wantModel(t, "fx-4's column", columnOf(t, root, "fx-4"), "Acceptance")

	marked := tuiBench(t)
	again := runTUIThrough(t, marked, tuiSeam(t, strings.NewReader(burst+"q"), 100, 30))
	wantModel(t, "the flushes with markers seen", again.flushes, 0)
	wantModel(t, "fx-4's column with markers seen", columnOf(t, marked, "fx-4"), "Done")
}

// TestCtrlCInsideAnOpenPasteQuits is the head's half of the design review's
// fourth-round note on dinah-603: a paste whose end marker never arrives
// cannot keep the person from quitting. Ctrl+C typed inside it abandons the
// paste and quits with exit 0, in browse mode and in a prompt, and nothing of
// the abandoned paste is taken as text.
func TestCtrlCInsideAnOpenPasteQuits(t *testing.T) {
	for name, keys := range map[string]string{"browse": "", "the comment prompt": "c"} {
		t.Run(name, func(t *testing.T) {
			root := tuiBench(t)
			run := runTUIThrough(t, root, tuiSeam(t, strings.NewReader(keys+pasteOpen+"half a paste"+keyCtrlC), 100, 30))
			if run.code != 0 || run.model == nil || !run.model.quitting {
				t.Fatalf("ctrl+c inside an open paste did not quit: exit %d, %q", run.code, run.errw)
			}
			wantModel(t, "the comment prompt's text", run.model.area.Value(), "")
			endsRestored(t, run.output)
		})
	}
}

// TestBubbleTeaReadsXtermOnWindows holds the environment the head gives
// Bubble Tea: on Windows it is the process's own with every TERM entry
// removed, whatever its case, and exactly TERM=xterm added, and on every
// other GOOS it is the process's own unchanged.
func TestBubbleTeaReadsXtermOnWindows(t *testing.T) {
	environ := []string{"PATH=/bin", "TERM=kitty", "Term=alacritty", "TERMINFO=/x"}
	if got := interactiveEnviron(environ, "windows"); strings.Join(got, " ") != "PATH=/bin TERMINFO=/x TERM=xterm" {
		t.Errorf("on windows the environment is %q, wanted PATH, TERMINFO and TERM=xterm alone", got)
	}
	if got := interactiveEnviron(environ, "linux"); strings.Join(got, " ") != strings.Join(environ, " ") {
		t.Errorf("on linux the environment is %q, wanted it unchanged", got)
	}
}

// TestAWindowsTerminalNamingKittyGetsNothingUndocumented runs the head on
// Windows under TERM=kitty, which would lead Bubble Tea's renderer to write
// REP and HPA, and requires every sequence written to be one Microsoft's
// console page lists.
func TestAWindowsTerminalNamingKittyGetsNothingUndocumented(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("TERM reaches Bubble Tea off Windows, where the terminal it names documents what it answers")
	}
	tuiTerm = "kitty"
	defer func() { tuiTerm = "xterm" }()
	run := runTUIThrough(t, tuiBench(t), tuiSeam(t, strings.NewReader("jjl"+keyEnter+keyEnter+"q"), 100, 30))
	written := 0
	for _, sequence := range controlSequence.FindAllString(run.output, -1) {
		written++
		if !allowedSequence.MatchString(sequence) || undocumentedSequence.MatchString(sequence) {
			t.Errorf("under TERM=kitty the program wrote %q", sequence)
		}
	}
	if written == 0 {
		t.Fatal("the run wrote no sequence, so this proves nothing")
	}
}
