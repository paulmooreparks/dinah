package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
)

// expectViewList is the listing a bare dinah view draws over the healthy
// tree, whose user base declares no view: the one view the workbench declares
// and the one view Dinah ships, both used, sorted by name, in the language
// the sweep draws in.
func expectViewList(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	catalog := msg.For(tag)
	yes := catalog.T("word.yes")
	return sweptExpectation{
		rows: [][]sweptCell{
			sweptTexts(sweptViewName, sweptViewTitle, bench.ViewLayoutList, bench.ViewSourceWorkbench, yes),
			sweptTexts("mine", catalog.T("view.mine.title"), bench.ViewLayoutList, bench.ViewSourceBuiltIn, yes),
		},
		source: "the view the fixture declares and the view Dinah ships",
	}
}

// expectMineClaimed is the first section of the built-in mine drawn as the
// fixture's own owner: every card that owner still holds, with its column's
// title, its two levels and its title. The Item and Holder columns draw
// nothing here, since holder:@me names no item field and every card it
// selects is held by the caller, so the table drops them.
func expectMineClaimed(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	var rows [][]sweptCell
	for _, card := range r.cards {
		if card.holder != r.actor {
			continue
		}
		rows = append(rows, sweptTexts(card.ref, r.columns[card.column].title, card.priority, card.severity, card.title))
	}
	return sweptExpectation{rows: rows, source: "the record's cards still held by the acting owner"}
}

// declareViewsIn writes a dinah.views block into a workbench's definition.
func declareViewsIn(t *testing.T, root, block string) {
	t.Helper()
	path := filepath.Join(soleBenchDir(t, root), bench.WorkbenchAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		t.Fatalf("read the workbench anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	fm.SetRaw(bench.ViewsKey, bench.SplitLines(strings.TrimSuffix(block, "\n")))
	if err := bench.WriteText(path, fm.Render(body)); err != nil {
		t.Fatalf("write the workbench anchor: %v", err)
	}
}

// writeUserConfig writes the config.md of the user base the test harness
// points DINAH_HOME at.
func writeUserConfig(t *testing.T, text string) {
	t.Helper()
	if err := bench.WriteText(bench.UserViewsPath(os.Getenv("DINAH_HOME")), text); err != nil {
		t.Fatalf("write the user's config: %v", err)
	}
}

// mustView runs a view command and fails unless it exits zero.
func mustView(t *testing.T, root string, argv ...string) string {
	t.Helper()
	got := runCLI(t, root, append([]string{"view"}, argv...)...)
	if got.code != 0 {
		t.Fatalf("view %v: exit %d\n%s", argv, got.code, got.errw)
	}
	return got.out
}

// mustRunHere runs one command and fails unless it exits zero.
func mustRunHere(t *testing.T, root string, argv ...string) invocation {
	t.Helper()
	got := runCLI(t, root, argv...)
	if got.code != 0 {
		t.Fatalf("%v: exit %d\n%s", argv, got.code, got.errw)
	}
	return got
}

// TestAWorkbenchViewIsListedDrawnAndCarriedThroughExport is
// dinah-600/criteria/1 at the terminal: the workbench's view is listed from
// the workbench layer and draws both sections, and after export and init
// --from the new definition carries the block as a block, the listing is
// byte-identical, and a second export matches the first.
func TestAWorkbenchViewIsListedDrawnAndCarriedThroughExport(t *testing.T) {
	root := newBench(t)
	mustRunHere(t, root, "add", "Ready one")
	mustRunHere(t, root, "add", "Ready two")
	carryToDoing(t, root, "fx-2")
	declareViewsIn(t, root, bench.ViewsKey+":\n  daily:\n    title: Daily\n    order: column\n    sections:\n"+
		"      - title: Waiting\n        query: \"column:intake\"\n      - title: Under way\n        query: \"column:doing\"\n")
	listing := mustView(t, root)
	if !strings.Contains(listing, "daily") || !strings.Contains(listing, bench.ViewSourceWorkbench) {
		t.Errorf("the listing does not show the workbench's view:\n%s", listing)
	}
	drawn := mustView(t, root, "daily")
	if !strings.Contains(sectionOf(drawn, "Waiting (1)"), "fx-1") || !strings.Contains(sectionOf(drawn, "Under way (1)"), "fx-2") {
		t.Errorf("the view did not draw both sections:\n%s", drawn)
	}
	exported := mustRunHere(t, root, "export")
	file := filepath.Join(t.TempDir(), "definition.json")
	if err := os.WriteFile(file, []byte(exported.out), 0o644); err != nil {
		t.Fatalf("write the export: %v", err)
	}
	clone := filepath.Join(t.TempDir(), "clone")
	if err := os.MkdirAll(clone, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mustRunHere(t, clone, "init", "--from", file, "--slug", "fx")
	anchor, err := bench.ReadText(filepath.Join(soleBenchDir(t, clone), bench.WorkbenchAnchor))
	if err != nil {
		t.Fatalf("read the clone's anchor: %v", err)
	}
	if !strings.Contains(anchor, bench.ViewsKey+":\n  daily:\n") || strings.Contains(anchor, bench.ViewsKey+": {") {
		t.Errorf("the clone does not carry the views as a nested block:\n%s", anchor)
	}
	if first, second := mustView(t, root, "--json"), mustView(t, clone, "--json"); first != second {
		t.Errorf("the --json listing differs after the round trip:\n%s\n%s", first, second)
	}
	again := mustRunHere(t, clone, "export")
	if stampedExport(exported.out) != stampedExport(again.out) {
		t.Errorf("a second export differs from the first:\n%s\n%s", exported.out, again.out)
	}
}

// stampedExport sets aside the one line of an export a clone is meant to
// change, the profile revision Instantiate stamps, so the rest of a byte
// comparison says something.
func stampedExport(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.Contains(line, `"profile":`) {
			lines[i] = "profile"
		}
	}
	return strings.Join(lines, "\n")
}

// TestADefaultedViewDrawsUnderItsNameAndItsQueries is dinah-600/criteria/2 on
// both forms.
func TestADefaultedViewDrawsUnderItsNameAndItsQueries(t *testing.T) {
	root := newBench(t)
	declareViewsIn(t, root, bench.ViewsKey+":\n  plain:\n    sections:\n      - query: \"state:ready\"\n")
	if listing := mustView(t, root); !strings.Contains(listing, "plain  plain") {
		t.Errorf("the listing does not title the view by its name:\n%s", listing)
	}
	drawn := mustView(t, root, "plain")
	if !strings.HasPrefix(drawn, "plain") || !strings.Contains(drawn, "\nstate:ready (0)\n") {
		t.Errorf("the view is not headed by its name and its section by its query:\n%s", drawn)
	}
	var answer struct {
		View struct {
			Title    string `json:"title"`
			Layout   string `json:"layout"`
			Order    string `json:"order"`
			Sections []struct {
				Title string `json:"title"`
			} `json:"sections"`
		} `json:"view"`
	}
	if err := json.Unmarshal([]byte(mustView(t, root, "plain", "--json")), &answer); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if answer.View.Title != "plain" || answer.View.Layout != "list" || answer.View.Order != "arrival" || answer.View.Sections[0].Title != "state:ready" {
		t.Errorf("the JSON form reads %+v", answer.View)
	}
}

// TestAMalformedViewIsListedRefusedAndChecked is dinah-600/criteria/3 at the
// terminal: the listing's human and JSON forms name the defect, a draw is
// refused naming it and its layer, and check reports it, while the sibling in
// the same block draws.
func TestAMalformedViewIsListedRefusedAndChecked(t *testing.T) {
	root := newBench(t)
	declareViewsIn(t, root, bench.ViewsKey+":\n  broken:\n    sections: []\n  sibling:\n    sections:\n      - query: \"state:ready\"\n")
	if listing := mustView(t, root); !strings.Contains(listing, "(malformed: no-sections)") {
		t.Errorf("the listing does not mark the view:\n%s", listing)
	}
	if listing := mustView(t, root, "--json"); !strings.Contains(listing, `"malformed": "no-sections"`) {
		t.Errorf("the JSON listing does not carry the defect:\n%s", listing)
	}
	got := runCLI(t, root, "view", "broken", "--json")
	if got.code == 0 || !strings.Contains(got.out, `"refusal": "dinah.malformed-view"`) ||
		!strings.Contains(got.out, `"defect": "no-sections"`) || !strings.Contains(got.out, `"source": "workbench"`) {
		t.Errorf("drawing the malformed view answered %d:\n%s", got.code, got.out)
	}
	mustView(t, root, "sibling")
	if checked := runCLI(t, root, "check", "--json"); !strings.Contains(checked.out, `"Detail": "broken no-sections"`) {
		t.Errorf("check does not report the view:\n%s", checked.out)
	}
}

// TestAnUnknownMemberLeavesTheFileAndTheDrawAlone is dinah-600/criteria/5 at
// the terminal.
func TestAnUnknownMemberLeavesTheFileAndTheDrawAlone(t *testing.T) {
	root := newBench(t)
	declareViewsIn(t, root, bench.ViewsKey+":\n  held:\n    titel: Oops\n    sections:\n      - query: \"state:ready\"\n        colour: red\n")
	path := filepath.Join(soleBenchDir(t, root), bench.WorkbenchAnchor)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	mustView(t, root, "held")
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(before) != string(after) {
		t.Error("drawing the view rewrote the workbench definition")
	}
	checked := runCLI(t, root, "check", "--json")
	for _, detail := range []string{`"Detail": "held titel"`, `"Detail": "held sections/1/colour"`} {
		if !strings.Contains(checked.out, detail) {
			t.Errorf("check does not report %s:\n%s", detail, checked.out)
		}
	}
}

// TestAViewNeedingTheCallerIsRefusedAtTheTerminal is dinah-600/criteria/10 at
// the terminal: nothing on standard output, the read-refusal exit code, the
// harness sentence where a harness is declared, and a query that compares
// @me as text.
func TestAViewNeedingTheCallerIsRefusedAtTheTerminal(t *testing.T) {
	root := newBench(t)
	t.Setenv("DINAH_ACTOR", "")
	t.Setenv("DINAH_HARNESS", "")
	got := runCLI(t, root, "view", "mine")
	if got.out != "" {
		t.Errorf("a refused view wrote to standard output:\n%s", got.out)
	}
	if got.code != contract.ExitCode(contract.OutcomeRefused) || !strings.Contains(got.errw, contract.NoOwner+" ") {
		t.Errorf("view mine with no actor answered %d:\n%s", got.code, got.errw)
	}
	t.Setenv("DINAH_HARNESS", "claude-code")
	harnessed := runCLI(t, root, "view", "mine")
	if !strings.Contains(harnessed.errw, "claude-code") || harnessed.errw == got.errw {
		t.Errorf("the harnessed refusal does not carry the harness sentence:\n%s", harnessed.errw)
	}
	t.Setenv("DINAH_HARNESS", "")
	if query := runCLI(t, root, "query", "holder:@me"); query.code != 0 {
		t.Errorf("query holder:@me with no actor was refused:\n%s", query.errw)
	}
}

// sectionOf is the lines of a drawn view from one section heading to the
// blank line that ends the section, or the empty string where the view
// carries no such heading.
func sectionOf(drawn, heading string) string {
	at := strings.Index(drawn, heading+"\n")
	if at < 0 {
		return ""
	}
	rest := drawn[at:]
	if end := strings.Index(rest, "\n\n"); end >= 0 {
		return rest[:end+1]
	}
	return rest
}

// TestTheSectionTableShowsItsColumnsOnlyWhereTheyCarrySomething is
// dinah-600/criteria/20's human half, with criteria/27's heading line.
func TestTheSectionTableShowsItsColumnsOnlyWhereTheyCarrySomething(t *testing.T) {
	root := newBench(t)
	mustRunHere(t, root, "add", "One question")
	mustRunHere(t, root, "add", "Two questions")
	mustRunHere(t, root, "add", "Held by somebody else")
	mustRunHere(t, root, "file", "fx-1", "open_question", "a question", "--owner", "operator")
	mustRunHere(t, root, "file", "fx-2", "open_question", "a question", "--owner", "operator")
	mustRunHere(t, root, "file", "fx-2", "open_question", "another question", "--owner", "operator")
	carryToDoing(t, root, "fx-3")
	mustRunHere(t, root, "claim", "fx-3", "--actor", "bo")
	declareViewsIn(t, root, bench.ViewsKey+":\n  checks:\n    title: Checks\n    sections:\n"+
		"      - title: Asked\n        query: \"item_owner:operator item_state:pending\"\n"+
		"      - title: Held\n        query: \"state:active\"\n"+
		"      - title: Nobody\n        query: \"holder:nobody-at-all\"\n")
	catalog := msg.For(msg.Base)
	drawn := mustView(t, root, "checks")
	if heading := strings.Split(drawn, "\n")[0]; heading != "Checks  "+catalog.T("view.acting.operator", "actor", "alka") {
		t.Errorf("the operator's heading reads %q", heading)
	}
	asked := sectionOf(drawn, "Asked (2)")
	if !strings.Contains(asked, "Item") || !strings.Contains(asked, "questions/1 +1") || strings.Contains(asked, "Holder") {
		t.Errorf("the item section reads:\n%s", asked)
	}
	for _, line := range strings.Split(asked, "\n") {
		if strings.HasPrefix(line, "  fx-1 ") && !strings.Contains(line, "questions/1 ") || strings.HasPrefix(line, "  fx-1 ") && strings.Contains(line, "+") {
			t.Errorf("the card one item witnessed reads %q, want questions/1 alone", line)
		}
	}
	if held := sectionOf(drawn, "Held (1)"); !strings.Contains(held, "Holder") || !strings.Contains(held, "bo") || strings.Contains(held, "Item") {
		t.Errorf("the held section reads:\n%s", held)
	}
	if nobody := sectionOf(drawn, "Nobody (0)"); !strings.Contains(nobody, "Nothing matches.") {
		t.Errorf("the empty section reads:\n%s", nobody)
	}
	if strings.Contains(drawn, "Pri") || strings.Contains(drawn, "Sev") {
		t.Errorf("a view whose cards carry no level draws a level column:\n%s", drawn)
	}

	t.Setenv("DINAH_ACTOR", "bo")
	drawn = mustView(t, root, "checks")
	if heading := strings.Split(drawn, "\n")[0]; heading != "Checks  "+catalog.T("view.acting", "actor", "bo") {
		t.Errorf("the heading for a caller who is not the operator reads %q", heading)
	}
	if held := sectionOf(drawn, "Held (1)"); strings.Contains(held, "Holder") {
		t.Errorf("the held section shows a Holder column when the caller holds its one card:\n%s", held)
	}
	t.Setenv("DINAH_ACTOR", "")
	if heading := strings.Split(mustView(t, root, "checks"), "\n")[0]; heading != "Checks" {
		t.Errorf("the heading with no actor reads %q", heading)
	}
}

// TestTheLevelColumnsAppearWhereACardCarriesALevel is the Pri and Sev half of
// dinah-600/criteria/20.
func TestTheLevelColumnsAppearWhereACardCarriesALevel(t *testing.T) {
	root := newBench(t)
	writeLevels(t, root, "levels:\n  severity: [minor, major]\n  priority: [later, now]\n")
	mustRunHere(t, root, "add", "Leveled", "--severity", "major", "--priority", "now")
	mustRunHere(t, root, "add", "Plain")
	declareViewsIn(t, root, bench.ViewsKey+":\n  all:\n    sections:\n      - query: \"state:ready\"\n")
	drawn := mustView(t, root, "all")
	if !strings.Contains(drawn, "Pri") || !strings.Contains(drawn, "Sev") || !strings.Contains(drawn, "major") || !strings.Contains(drawn, "now") {
		t.Errorf("a section whose card carries both levels does not draw them:\n%s", drawn)
	}
}

// writeLevels puts a levels block into a workbench's definition.
func writeLevels(t *testing.T, root, block string) {
	t.Helper()
	path := filepath.Join(soleBenchDir(t, root), bench.WorkbenchAnchor)
	text, err := bench.ReadText(path)
	if err != nil {
		t.Fatalf("read the workbench anchor: %v", err)
	}
	fm, body := bench.ParseAnchor(text)
	fm.SetRaw(bench.LevelsKey, bench.SplitLines(strings.TrimSuffix(block, "\n")))
	if err := bench.WriteText(path, fm.Render(body)); err != nil {
		t.Fatalf("write the workbench anchor: %v", err)
	}
}

// TestALongTitleIsCutOnlyWhereTheWidthIsKnown is dinah-600/criteria/21.
func TestALongTitleIsCutOnlyWhereTheWidthIsKnown(t *testing.T) {
	root := newBench(t)
	long := "A title long enough that no sixty column window could ever hold the whole of it"
	wide := "\u6f22\u5b57\u306e\u984c\u540d\u306f\u4e00\u6587\u5b57\u304c\u4e8c\u5217\u3092\u5360\u3081\u308b\u306e\u3067\u8868\u793a\u5e45\u3067\u5207\u3089\u306a\u3051\u308c\u3070\u306a\u3089\u306a\u3044"
	mustRunHere(t, root, "add", long)
	mustRunHere(t, root, "add", wide)
	declareViewsIn(t, root, bench.ViewsKey+":\n  all:\n    sections:\n      - query: \"state:ready\"\n")
	t.Setenv("COLUMNS", "60")
	drawn := mustView(t, root, "all")
	cut := 0
	for _, line := range strings.Split(strings.TrimSuffix(drawn, "\n"), "\n") {
		if displayWidth(line) > 60 {
			t.Errorf("a line draws %d columns at a window of 60: %q", displayWidth(line), line)
		}
		if strings.HasSuffix(line, tailEllipsis) {
			cut++
		}
	}
	if cut != 2 {
		t.Errorf("%d titles were cut, want the two long ones:\n%s", cut, drawn)
	}
	t.Setenv("COLUMNS", "")
	piped := mustView(t, root, "all")
	if !strings.Contains(piped, long) || !strings.Contains(piped, wide) || strings.Contains(piped, tailEllipsis) {
		t.Errorf("with no width known a title was cut:\n%s", piped)
	}
}

// TestAWholeViewRefusalNamesItsSectionOnStandardError is the terminal half of
// dinah-600/criteria/30.
func TestAWholeViewRefusalNamesItsSectionOnStandardError(t *testing.T) {
	root := newBench(t)
	declareViewsIn(t, root, bench.ViewsKey+":\n  typo:\n    sections:\n      - query: \"state:ready\"\n      - title: The typo\n        query: \"state:reday\"\n")
	got := runCLI(t, root, "view", "typo")
	if got.out != "" || got.code == 0 {
		t.Errorf("the refused view answered %d with output:\n%s", got.code, got.out)
	}
	locating := msg.For(msg.Base).T("view.section-refused", "view", "typo", "section", "2", "title", "The typo")
	if !strings.HasPrefix(got.errw, contract.UnknownValue+" ") || !strings.HasSuffix(got.errw, "\n"+locating+"\n") {
		t.Errorf("standard error does not lead with the refusal and close on %q:\n%s", locating, got.errw)
	}
	machine := runCLI(t, root, "view", "typo", "--json")
	if !strings.Contains(machine.out, `"view": "typo"`) || !strings.Contains(machine.out, `"section": "2"`) {
		t.Errorf("the JSON refusal does not carry the view and the section:\n%s", machine.out)
	}
}

// TestTheViewCommandIsDocumented is dinah-600/criteria/25's terminal half:
// help renders the command's syntax and points at the views guide, and the
// guide is served.
func TestTheViewCommandIsDocumented(t *testing.T) {
	root := newBench(t)
	help := runCLI(t, root, "help", "view")
	if help.code != 0 || !strings.HasPrefix(help.out, "view [view]\n") || !strings.Contains(help.out, "dinah guide views") {
		t.Errorf("help view reads:\n%s", help.out)
	}
	guide := runCLI(t, root, "guide", "views")
	if guide.code != 0 || !strings.Contains(guide.out, "Asking the same questions every day") {
		t.Errorf("guide views answered %d:\n%s", guide.code, guide.errw)
	}
}

// TestConfigReportsTheViewsItReads is dinah-600/criteria/26.
func TestConfigReportsTheViewsItReads(t *testing.T) {
	root := newBench(t)
	writeUserConfig(t, "---\n"+bench.ViewsKey+":\n  zulu:\n    sections:\n      - query: \"state:ready\"\n  alpha:\n    sections:\n      - query: \"state:ready\"\n---\n")
	listed := runCLI(t, root, "config", "--json")
	if !strings.Contains(listed.out, `"key": "dinah.views"`) || !strings.Contains(listed.out, `"value": "zulu, alpha"`) || !strings.Contains(listed.out, `"source": "config"`) {
		t.Errorf("config does not report the views in declaration order:\n%s", listed.out)
	}
	got := runCLI(t, root, "config", "get", bench.ViewsKey)
	if got.code == 0 || !strings.Contains(got.errw, contract.UnknownKey) {
		t.Errorf("config get dinah.views answered %d:\n%s", got.code, got.errw)
	}
}

// TestASectionThisWorkbenchCannotAskIsDrawnAsNotAsked is the terminal half of
// dinah-600/criteria/15: whichever layer declares the view, its section
// naming a column the workbench lacks draws with no cards and the refusal
// composed without its next step, the valid section beside it draws its card,
// and the command exits zero.
func TestASectionThisWorkbenchCannotAskIsDrawnAsNotAsked(t *testing.T) {
	block := bench.ViewsKey + ":\n  stale:\n    sections:\n      - query: \"column:nosuch\"\n      - query: \"state:ready\"\n"
	for _, layer := range []string{bench.ViewSourceWorkbench, bench.ViewSourceUser} {
		t.Run(layer, func(t *testing.T) {
			root := newBench(t)
			mustRunHere(t, root, "add", "A ready card")
			if layer == bench.ViewSourceWorkbench {
				declareViewsIn(t, root, block)
			} else {
				writeUserConfig(t, "---\n"+block+"---\n")
			}
			drawn := mustView(t, root, "stale")
			refused := sectionOf(drawn, "column:nosuch (0)")
			if !strings.Contains(refused, "  "+msg.For(msg.Base).T("view.section.refused", "refusal", contract.UnknownColumn+": ")) {
				t.Errorf("the refused section reads:\n%s", refused)
			}
			if strings.Contains(refused, msg.For(msg.Base).T("refusal.unknown-column.next", "command", "view")) {
				t.Errorf("the refused section carries the refusal's next step:\n%s", refused)
			}
			if !strings.Contains(sectionOf(drawn, "state:ready (1)"), "fx-1") {
				t.Errorf("the valid section did not draw its card:\n%s", drawn)
			}
		})
	}
}
