//go:build tui

package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/testenv"
	"dinah/internal/verb"
)

// pinDefinition is the flow the pinning test stands in: an intake, a work
// column and a done column.
const pinDefinition = `{
  "profile": "dinah-core/0.12",
  "title": "Pinned",
  "columns": [
    { "id": "p10000000001", "title": "Intake", "slug": "intake", "kind": "intake" },
    { "id": "p10000000002", "title": "Work", "slug": "work", "kind": "work" },
    { "id": "p10000000003", "title": "Done", "slug": "done", "kind": "done" }
  ]
}`

// pinClock is the clock every library a line opens reads, so the journal
// lines two runs of the same lines write carry the same stamps.
var pinClock = time.Date(2031, 3, 4, 5, 6, 7, 0, time.UTC)

// pinLine is one line of pinFixtures: the words typed, whether the command
// opens a workbench, and, where it does not, why.
type pinLine struct {
	line   string
	opens  bool
	reason string
}

// pinPaths are the paths the fixture lines name.
type pinPaths struct {
	definition, file, initDir, extractDir string
}

// pinFixtures holds, for each of the commands classed direct or line, the
// lines TestNoCommandFromTheUIResolvesAPinnedSettingAfresh runs. Every line
// gets past its command's argument checks to the point where it resolves its
// workbench or its settings, so a line refused before it reached that point
// could not show a setting resolved afresh. A command marked as opening a
// workbench must reach onOpen, and a command that opens none names why.
func pinFixtures(p pinPaths) map[string][]pinLine {
	opens := func(lines ...string) []pinLine {
		var out []pinLine
		for _, line := range lines {
			out = append(out, pinLine{line: line, opens: true})
		}
		return out
	}
	none := func(line, reason string) []pinLine {
		return []pinLine{{line: line, reason: reason}}
	}
	return map[string][]pinLine{
		"add":               opens(`add "Pinned card" --column work`),
		"claim":             opens("claim fx-1"),
		"move":              opens("move fx-1 intake"),
		"pull":              opens("pull work"),
		"release":           opens("release fx-1"),
		"block":             opens("block fx-3 waiting"),
		"unblock":           opens("unblock fx-3"),
		"raise":             opens("raise fx-2 frontier harder"),
		"comment":           opens("comment fx-1 noted"),
		"attach":            opens("attach fx-1 " + p.file),
		"file":              opens("file fx-1 decision decided"),
		"cite":              opens("cite fx-1/decisions/1 test cmd/dinah/pin_test.go"),
		"resolve":           opens("resolve fx-1/decisions/1 --text done"),
		"verify":            opens("verify fx-1/criteria/1 --text shown"),
		"fail":              opens("fail fx-1/criteria/2 --text broken"),
		"waive":             opens("waive fx-1/criteria/2 --text fine"),
		"withdraw":          opens("withdraw fx-1/questions/1 --text moot"),
		"reopen":            opens("reopen fx-1/decisions/1 again"),
		"grant":             opens("grant fx-1 criterion-retirement"),
		"revoke":            opens("revoke fx-1 criterion-retirement"),
		"link":              opens("link fx-1 relates_to fx-3"),
		"unlink":            opens("unlink fx-1 relates_to fx-3"),
		"join":              opens("join fx-1 parser"),
		"leave":             opens("leave fx-1 parser"),
		"archive":           opens("archive fx-4"),
		"restore":           opens("restore fx-4"),
		"delete":            opens("delete fx-5 --yes"),
		"accept-divergence": opens("accept-divergence fx-1/comments/1"),
		"rename":            opens("rename fx-1/attachments/1 renamed.txt"),
		"status":            opens("status"),
		"list":              opens("list"),
		"next":              opens("next"),
		"query":             opens("query state:ready"),
		"search":            opens("search parser"),
		"tree":              opens("tree"),
		"view":              opens("view board"),
		"show":              opens("show fx-1"),
		"changes":           opens("changes"),
		"instructions":      opens("instructions fx-1"),
		"prime":             opens("prime"),
		"guide":             none("guide views", "prints a guide the binary carries, which reads no workbench and no setting"),
		"init":              none("init "+p.initDir+" --from "+p.definition+" --slug cc --operator alka", "creates a workbench in the directory it names and opens none"),
		"export":            opens("export"),
		"extract":           opens("extract " + p.extractDir),
		"reshape":           opens("reshape --from " + p.definition),
		"path":              opens("path fx-1"),
		"edit":              opens("edit fx-1"),
		"get":               opens("get fx-1 title"),
		"set":               opens("set fx-1 title Renamed"),
		"config":            none("config", "lists the settings, resolving the workbench's rung without opening it, and reports the start values the line session pins"),
		"check": {
			{line: "check --migrate-container --yes", reason: repairOpensNone},
			{line: "check --migrate-vocabulary --yes", reason: repairOpensNone},
		},
		"whoami":     opens("whoami"),
		"workbench":  opens("workbench"),
		"workstream": opens("workstream new Pinned --slug pinned"),
		"column":     opens("column new Extra"),
		"version":    none("version", "reports the binary, which reads no workbench"),
		"setup":      none("setup claude-code --dry-run", "plans a harness's files from the recipe and the settings, and the plan it prints names the start actor"),
		"help":       none("help status", "prints a page the binary carries"),
	}
}

// repairOpensNone is why the two repairs of check open no library: each
// resolves the workbench through sweepRoot and repairs its files directly.
// What holds them to the start workbench is the directory their output names
// and the files under bb, which the test holds byte-identical.
const repairOpensNone = "resolves its workbench through sweepRoot and repairs the files, opening no library; its output names the workbench it reached, and bb's files are held unchanged"

// pinBench builds workbench aa, where the pinned head starts, with the cards
// and members every fixture line reaches.
func pinBench(t *testing.T, p pinPaths) string {
	t.Helper()
	root := newBenchFromDefinition(t, pinDefinition)
	steps := [][]string{
		{"add", "Build the parser"}, {"move", "fx-1", "work"},
		{"add", "Waiting in intake"},
		{"add", "To be blocked"}, {"move", "fx-3", "work"},
		{"add", "To be archived"}, {"move", "fx-4", "work"},
		{"add", "To be deleted"}, {"move", "fx-5", "work"},
		{"add", "Standing in work"}, {"move", "fx-6", "work"},
		{"file", "fx-1", "acceptance_criterion", "It reads every line"},
		{"file", "fx-1", "acceptance_criterion", "It rejects a torn line"},
		{"file", "fx-1", "open_question", "Which vendor?"},
		{"workstream", "new", "Parser", "--slug", "parser"},
		{"attach", "fx-1", p.file},
	}
	for _, words := range steps {
		step(t, root, words...)
	}
	editedFirstComment(t, root)
	return root
}

// editedFirstComment leaves fx-1 with a first comment whose body was edited
// outside the tool.
func editedFirstComment(t *testing.T, root string) {
	t.Helper()
	step(t, root, "comment", "fx-1", "about to be edited")
	path := strings.TrimSpace(runCLI(t, root, "path", "fx-1/comments/1").out)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(data), "about to be edited", "edited by hand", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
}

// pinRun is one run of the fixture lines: what each line left, which
// libraries each line opened, and what the Tab checks listed.
type pinRun struct {
	results []*lineResult
	opened  [][]*verb.Library
	tabs    [][]string
}

// pinTabs are the lines a run completes with two Tabs, whose candidates and
// descriptions it records: a move's destinations and the commands beginning
// with s, whose summaries are in the head's language.
var pinTabs = []string{"move fx-6 ", "s"}

// runPinLines starts a head over root and types every fixture line in the
// command table's order, one at a time, waiting for each to end. change, when
// set, runs once the head has drawn and before the first line, which is where
// the pinned run changes the configuration and the environment.
func runPinLines(t *testing.T, root string, lines []pinLine, change func()) pinRun {
	t.Helper()
	var mu sync.Mutex
	var run pinRun
	var opened []*verb.Library
	done := make(chan *lineResult, len(lines)+8)
	s, seam := newScript(t, 200, 40, true)
	seam.lineDone = func(result *lineResult) { done <- result }
	seam.lineLibrary = func(l *verb.Library) {
		l.Now = func() time.Time { return pinClock }
		mu.Lock()
		opened = append(opened, l)
		mu.Unlock()
	}
	cycles := make(chan *tea.Program, 8)
	seam.program = func(program *tea.Program) { cycles <- program }
	var final *interactiveModel
	seam.finish = func(model *interactiveModel, _ error) { final = model }
	listed := make(chan []string, len(pinTabs))
	seam.observe = func(m *interactiveModel, msg tea.Msg) {
		pressed, ok := msg.(keyMsg)
		if ok && pressed.key.String() == "ctrl+g" && m.mode == modePrompt && m.prompt == promptJump {
			listed <- append([]string{}, m.message...)
		}
	}
	tui := s.run(root, seam, func() {
		waitForProgram(t, cycles)
		if change != nil {
			change()
		}
		for _, line := range lines {
			mu.Lock()
			opened = nil
			mu.Unlock()
			s.write(":" + line.line + keyEnter)
			if strings.HasPrefix(line.line, "edit ") {
				waitForProgram(t, cycles)
				s.waitFor("the flush after the lend", isFlushed)
			}
			select {
			case result := <-done:
				run.results = append(run.results, result)
			case <-time.After(tuiWait):
				t.Errorf("the line %q never ended", line.line)
				return
			}
			mu.Lock()
			run.opened = append(run.opened, opened)
			mu.Unlock()
		}
		for _, typed := range pinTabs {
			s.write(":" + typed + "\t\t" + keyCtrlG)
			select {
			case candidates := <-listed:
				run.tabs = append(run.tabs, candidates)
			case <-time.After(tuiWait):
				t.Errorf("the Tab after %q never answered", typed)
				return
			}
		}
		s.write(keyCtrlC)
	})
	if tui.model == nil && final == nil {
		t.Fatalf("the run never finished: %q", tui.errw)
	}
	return run
}

// minted matches an identifier the store mints, which two runs of the same
// lines mint differently because it is random: a card's or a column's twelve
// hexadecimal digits, or the thirty-two of the workbench directory init
// creates.
var minted = regexp.MustCompile(`\b[0-9a-f]{12}\b|\b[0-9a-f]{32}\b`)

// changeCursor matches the cursor dinah changes prints, an encoding of the
// card identifiers the workbench holds and a digest over them, which two runs
// that minted different identifiers print differently for the same reason.
var changeCursor = regexp.MustCompile(`cursor: \S+`)

// transcriptOf is a line's transcript with every identifier minted during
// the run replaced, and the change cursor that carries them, since nothing
// else about two runs of the same lines on the same workbench may differ.
// The store mints identifiers at random and offers no seam to fix them, so
// these two are the whole of what the comparison lets differ.
func transcriptOf(result *lineResult, known map[string]bool) string {
	text := strings.Join(result.transcript.lines, "\n")
	text = changeCursor.ReplaceAllString(text, "cursor: <cursor>")
	return minted.ReplaceAllStringFunc(text, func(id string) string {
		if known[id] {
			return id
		}
		return "<minted>"
	})
}

// knownIDs are the identifiers a workbench holds before any line runs.
func knownIDs(t *testing.T, root string) map[string]bool {
	t.Helper()
	known := map[string]bool{}
	filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && bench.IsID(entry.Name()) {
			known[entry.Name()] = true
		}
		return nil
	})
	return known
}

// restoreTree replaces a directory with a copy of another, so a second run
// meets the first run's workbench at the same path.
func restoreTree(t *testing.T, from, to string) {
	t.Helper()
	if err := os.RemoveAll(to); err != nil {
		t.Fatal(err)
	}
	err := filepath.WalkDir(from, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		target := filepath.Join(to, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestNoCommandFromTheUIResolvesAPinnedSettingAfresh is dinah-623/criteria/47,
// with /8 and /33. It runs one line for each of the commands classed direct
// or line, first in a head whose configuration and environment never change,
// then again, on the same workbench at the same path, in a head that started
// on workbench aa as claude on m1 in English with the Unicode glyphs and
// then had its configuration rewritten to name workbench bb, actor other,
// language de and the plain glyphs, its environment set to name bb, other,
// m2 and de, and a third workbench created in its working directory's walk.
//
// It asserts that every library a line opened is aa's, that the two repairs
// change no file under bb, that every journal line written carries claude and
// m1, that view board draws the Unicode glyphs, that the bare config listing
// reports the start workbench, actor and language, that every line's output
// is the unchanged head's, and that Tab lists what the unchanged head's Tab
// lists. A line marked as opening a workbench must reach onOpen; the count
// is reported beside the number of lines.
func TestNoCommandFromTheUIResolvesAPinnedSettingAfresh(t *testing.T) {
	scratch := t.TempDir()
	p := pinPaths{
		definition: filepath.Join(scratch, "definition.json"),
		file:       filepath.Join(scratch, "evidence.txt"),
		initDir:    filepath.Join(scratch, "created"),
		extractDir: filepath.Join(scratch, "extracted"),
	}
	for path, text := range map[string]string{p.definition: pinDefinition, p.file: "evidence\n"} {
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fixtures := pinFixtures(p)
	var names, lineOrder []string
	var lines []pinLine
	for _, c := range commands {
		if c.terminal == terminalAbsent {
			continue
		}
		names = append(names, c.name)
		for _, line := range fixtures[c.name] {
			lines = append(lines, line)
			lineOrder = append(lineOrder, c.name)
		}
	}
	var keyed []string
	for name := range fixtures {
		keyed = append(keyed, name)
	}
	sort.Strings(keyed)
	sorted := append([]string{}, names...)
	sort.Strings(sorted)
	if strings.Join(keyed, " ") != strings.Join(sorted, " ") {
		t.Fatalf("pinFixtures holds\n%v\nand the commands classed direct or line are\n%v", keyed, sorted)
	}

	root := pinBench(t, p)
	home := os.Getenv("DINAH_HOME")
	aa := soleBenchDir(t, root)
	other := newBenchFromDefinition(t, pinDefinition)
	bb := soleBenchDir(t, other)
	t.Setenv("DINAH_HOME", home)
	t.Setenv("DINAH_ACTOR", "claude")
	t.Setenv("DINAH_PROVIDER", "acme")
	t.Setenv("DINAH_MODEL", "m1")
	t.Setenv("DINAH_LANG", "en")
	t.Setenv("DINAH_WORKBENCH", "")
	t.Setenv("DINAH_EDITOR", os.Args[0])
	t.Setenv(editorRecordVar, filepath.Join(scratch, "editor.log"))
	t.Setenv(testenv.EditorAppendVar, "appended by the editor")
	snapshot := filepath.Join(scratch, "snapshot")
	restoreTree(t, root, snapshot)
	known := knownIDs(t, root)

	control := runPinLines(t, root, lines, nil)
	restoreTree(t, snapshot, root)
	os.RemoveAll(p.initDir)
	os.RemoveAll(p.extractDir)
	before := benchBytes(t, other)

	pinned := runPinLines(t, root, lines, func() {
		cfg := bench.LoadConfig(home)
		for key, value := range map[string]string{"workbench": bb, "actor": "other", "lang": "de", "glyphs": "plain"} {
			if err := cfg.Set(key, value); err != nil {
				t.Error(err)
			}
		}
		if created := runCLI(t, root, "init", root, "--here", "--from", p.definition, "--slug", "zz", "--operator", "alka"); created.code != 0 {
			t.Errorf("the third workbench was not created in %s: %s", root, created.errw)
		}
		os.Setenv("DINAH_WORKBENCH", bb)
		os.Setenv("DINAH_ACTOR", "other")
		os.Setenv("DINAH_MODEL", "m2")
		os.Setenv("DINAH_LANG", "de")
	})

	if len(pinned.results) != len(lines) || len(control.results) != len(lines) {
		t.Fatalf("ran %d and %d of %d lines", len(control.results), len(pinned.results), len(lines))
	}
	opening := 0
	for i, line := range lines {
		if line.opens {
			opening++
			if len(pinned.opened[i]) == 0 {
				t.Errorf("%s (%s) is marked as opening a workbench and opened none, so it was refused before it resolved anything: %q", line.line, lineOrder[i], pinned.results[i].transcript.lines)
			}
		} else if line.reason == "" {
			t.Errorf("%s opens no workbench and names no reason", line.line)
		}
		for _, l := range pinned.opened[i] {
			if !sameDir(l.Bench.Root, aa) {
				t.Errorf("%s opened %s, and the head started on %s", line.line, l.Bench.Root, aa)
			}
		}
		got, want := transcriptOf(pinned.results[i], known), transcriptOf(control.results[i], known)
		if got != want {
			t.Errorf("%s printed\n%s\nin the pinned head, and the unchanged head printed\n%s", line.line, got, want)
		}
	}
	if !sameBytes(before, benchBytes(t, other)) {
		t.Error("a line changed a file under the workbench the configuration and the environment named afterwards")
	}
	checkPinnedJournals(t, aa)
	checkPinnedReads(t, lines, pinned)
	for i := range pinTabs {
		if len(pinned.tabs[i]) == 0 {
			t.Errorf("Tab after %q listed nothing, so the comparison proves nothing: %q %q", pinTabs[i], pinned.tabs, control.tabs)
		}
		if strings.Join(pinned.tabs[i], "\n") != strings.Join(control.tabs[i], "\n") {
			t.Errorf("Tab after %q listed\n%s\nin the pinned head and\n%s\nin the unchanged head", pinTabs[i], strings.Join(pinned.tabs[i], "\n"), strings.Join(control.tabs[i], "\n"))
		}
	}
	if !t.Failed() {
		t.Logf("ran %d lines for %d commands; %d lines are marked as opening a workbench, and every one reached onOpen", len(lines), len(names), opening)
	}
}

// sameDir compares two directory paths by what they name.
func sameDir(a, b string) bool {
	return filepath.Clean(strings.ToLower(a)) == filepath.Clean(strings.ToLower(b))
}

// checkPinnedJournals asserts every journal line the pinned run wrote under
// the workbench it started on, which are the lines stamped by pinClock,
// carries claude and m1.
func checkPinnedJournals(t *testing.T, aa string) {
	t.Helper()
	stamp := bench.Stamp(pinClock)
	written := 0
	filepath.WalkDir(aa, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.Name() != bench.JournalName {
			return nil
		}
		events, _, readErr := bench.ReadJournal(path)
		if readErr != nil {
			t.Errorf("read %s: %v", path, readErr)
			return nil
		}
		for _, event := range events {
			if event.TS != stamp {
				continue
			}
			written++
			if event.Actor.Name != "claude" || event.Actor.Model != "m1" {
				t.Errorf("%s carries %s, written by %+v", path, event.Event, event.Actor)
			}
		}
		return nil
	})
	if written == 0 {
		t.Error("the pinned run wrote no journal line, so the identity check read nothing")
	}
	t.Logf("%d journal lines the pinned run wrote carry claude and m1", written)
}

// checkPinnedReads asserts the two reads whose answer names a pinned setting:
// view board draws the Unicode glyphs, and the bare config listing reports
// the start workbench, actor and language with the rung each resolved at.
func checkPinnedReads(t *testing.T, lines []pinLine, pinned pinRun) {
	t.Helper()
	for i, line := range lines {
		text := strings.Join(pinned.results[i].transcript.lines, "\n")
		switch line.line {
		case "view board":
			if !strings.Contains(text, "─") {
				t.Errorf("view board drew no Unicode rule:\n%s", text)
			}
		case "config":
			for _, want := range []string{"claude", "en"} {
				if !strings.Contains(text, want) {
					t.Errorf("the config listing does not report %s:\n%s", want, text)
				}
			}
			for _, unwanted := range []string{"other", " de "} {
				if strings.Contains(text, unwanted) {
					t.Errorf("the config listing reports %q:\n%s", unwanted, text)
				}
			}
		}
	}
}

// TestTheReviewersReproductionsHoldNoLonger is dinah-623/criteria/28. It
// types the design review's reproductions at the command line of a head
// started on workbench aa. config set workbench names bb, and the line says
// the setting is pinned; add then files its card on aa, bb gains nothing,
// and the head still draws aa. The two check repairs run with --yes, name
// aa's directory and change no file under bb, and the bare config listing
// names aa rather than the bb the line just stored. Last, init --here puts a
// second workbench where the working directory's walk reaches it, and the
// line after it runs on aa with no dinah.ambiguous-workbench.
func TestTheReviewersReproductionsHoldNoLonger(t *testing.T) {
	root := tuiBench(t)
	aa := soleBenchDir(t, root)
	other := newBenchFromDefinition(t, tuiDefinition)
	bb := soleBenchDir(t, other)
	t.Setenv("DINAH_WORKBENCH", "")
	definition := filepath.Join(t.TempDir(), "definition.json")
	if err := os.WriteFile(definition, []byte(tuiDefinition), 0o644); err != nil {
		t.Fatal(err)
	}
	before := benchBytes(t, other)
	lines := []string{
		"config set workbench " + bb,
		"add second",
		"check --migrate-container --yes",
		"check --migrate-vocabulary --yes",
		"config",
		"init " + root + " --here --from " + definition + " --slug zz --operator alka",
		"status",
	}
	results, run := runLines(t, root, lines, "")
	if len(results) != len(lines) {
		t.Fatalf("ran %d of %d lines", len(results), len(lines))
	}
	text := func(i int) string { return strings.Join(results[i].transcript.lines, "\n") }
	if results[0].code != 0 || results[0].pinned != "workbench" {
		t.Errorf("config set workbench answered %d and named %q as pinned: %s", results[0].code, results[0].pinned, text(0))
	}
	if results[1].code != 0 {
		t.Errorf("add second was refused: %s", text(1))
	}
	for _, i := range []int{2, 3} {
		if !strings.Contains(text(i), filepath.Base(aa)) || strings.Contains(text(i), filepath.Base(bb)) {
			t.Errorf("%s does not name aa's directory %s alone:\n%s", lines[i], filepath.Base(aa), text(i))
		}
	}
	if strings.Contains(text(4), filepath.Base(bb)) || !strings.Contains(text(4), filepath.Base(aa)) {
		t.Errorf("the config listing does not name the start workbench alone:\n%s", text(4))
	}
	if results[5].code != 0 {
		t.Errorf("init --here was refused: %s", text(5))
	}
	if results[6].code != 0 || strings.Contains(text(6), contract.AmbiguousWorkbench) {
		t.Errorf("status after init answered %d:\n%s", results[6].code, text(6))
	}
	if !sameBytes(before, benchBytes(t, other)) {
		t.Error("a line changed a file under bb")
	}
	if run.model == nil || !sameDir(run.model.l.Bench.Root, aa) {
		t.Errorf("the head no longer draws aa")
	}
	filed := false
	for _, lane := range run.model.lanes {
		for _, card := range lane.cards {
			if card.Title == "second" {
				filed = true
			}
		}
	}
	if !filed {
		t.Error("the head draws no card titled second on aa")
	}
}
