package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
	"dinah/internal/screen"
	"dinah/internal/verb"
)

// expectViewList is the listing a bare dinah view draws over the healthy
// tree, whose user base declares no view: the one view the workbench declares
// and the two views Dinah ships, all used, sorted by name, in the language
// the sweep draws in.
func expectViewList(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	catalog := msg.For(tag)
	yes := catalog.T("word.yes")
	return sweptExpectation{
		rows: [][]sweptCell{
			sweptTexts("agenda", catalog.T("view.agenda.title"), bench.ViewLayoutList, bench.ViewSourceBuiltIn, yes),
			sweptTexts(sweptViewName, sweptViewTitle, bench.ViewLayoutList, bench.ViewSourceWorkbench, yes),
			sweptTexts("board", catalog.T("view.board.title"), bench.ViewLayoutColumns, bench.ViewSourceBuiltIn, yes),
			sweptTexts("mine", catalog.T("view.mine.title"), bench.ViewLayoutList, bench.ViewSourceBuiltIn, yes),
		},
		source: "the view the fixture declares and the two views Dinah ships",
	}
}

// sweptDefaultWeights are the shipped priority and severity weights in
// tenths, lowest level first, which is what the healthy tree's backlog view
// ranks its intake cards by: the tree declares four levels on each axis and
// no dinah.urgency block.
var sweptDefaultWeights = map[string]map[string]int{
	"priority": {"later": 0, "soon": 20, "next": 40, "now": 60},
	"severity": {"trivial": 0, "minor": 10, "major": 20, "critical": 40},
}

// expectRankedBacklog is the healthy tree's backlog view, which is ordered by
// urgency and selects the intake column, drawn as the fixture's own owner.
// Neither intake card is held, blocked, claimed or carries an item, and none
// stands where the operator owns the column or has stood there a whole day,
// so each score is its two level weights and each Why cell names the levels
// that weigh anything. Ties break on arrival, which for a card never moved is
// the order it was filed in.
func expectRankedBacklog(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	type scored struct {
		card  sweptCardRecord
		score int
		why   []string
	}
	var cards []scored
	for _, card := range r.cards {
		if card.column != 0 {
			continue
		}
		entry := scored{card: card}
		for _, axis := range []struct{ name, level string }{{"priority", card.priority}, {"severity", card.severity}} {
			weight := sweptDefaultWeights[axis.name][axis.level]
			entry.score += weight
			if weight != 0 {
				entry.why = append(entry.why, axis.level)
			}
		}
		cards = append(cards, entry)
	}
	sort.SliceStable(cards, func(i, j int) bool { return cards[i].score > cards[j].score })
	var rows [][]sweptCell
	for i, entry := range cards {
		rows = append(rows, sweptTexts(strconv.Itoa(i+1), entry.card.ref, r.columns[0].title,
			bench.FormatTenths(int64(entry.score)), strings.Join(entry.why, ", "), entry.card.title))
	}
	return sweptExpectation{rows: rows, source: "the record's intake cards, scored on the shipped level weights"}
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

// sweptExplainedCard is the intake card the sweep explains, which carries both
// level axes, so two of its terms score and the rest read as nothing.
const sweptExplainedCard = "fx-3"

// expectExplainedTerms is one card's terms under dinah view backlog --explain,
// drawn as the fixture's own owner, who is the operator: a line per term in
// term order, then the total. The rule between them is cut out of the harvest
// by the entry's own render. The card stands at intake, which
// nobody owns; it carries no item, no block and no claim, and it was filed
// during the run, so it has stood in its column no whole day. Its two levels
// are the only terms that score.
func expectExplainedTerms(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	catalog := msg.For(tag)
	card := r.cards[sweptCardAt(r, sweptExplainedCard)]
	column := r.columns[card.column].title
	rank := func(axis, level string) string {
		for i, name := range map[string][]string{
			"priority": {"later", "soon", "next", "now"},
			"severity": {"trivial", "minor", "major", "critical"},
		}[axis] {
			if name == level {
				return strconv.Itoa(i + 1)
			}
		}
		return ""
	}
	priority := sweptDefaultWeights["priority"][card.priority]
	severity := sweptDefaultWeights["severity"][card.severity]
	term := func(name, reading string, points int) []sweptCell {
		return sweptTexts(catalog.T("view.urgency.term."+name), reading, "+"+bench.FormatTenths(int64(points)))
	}
	rows := [][]sweptCell{
		term(bench.UrgencyWaitsOnYou, catalog.T("view.urgency.explain.waits-on-you.unowned", "column", column), 0),
		term(bench.UrgencyYourQuestion, catalog.T("view.urgency.explain.your-question.none"), 0),
		term(bench.UrgencyPriority, catalog.T("view.urgency.explain.level", "level", card.priority, "rank", rank("priority", card.priority), "of", "4"), priority),
		term(bench.UrgencySeverity, catalog.T("view.urgency.explain.level", "level", card.severity, "rank", rank("severity", card.severity), "of", "4"), severity),
		term(bench.UrgencyBlocked, catalog.T("view.urgency.explain.blocked.not"), 0),
		term(bench.UrgencyBlocksOthers, catalog.T("view.urgency.explain.blocks-others"), 0),
		term(bench.UrgencyAge, catalog.TN("view.urgency.explain.age", 0, "column", column, "per-day", "0.5"), 0),
		term(bench.UrgencyStaleClaim, catalog.T("view.urgency.explain.stale-claim.none"), 0),
		sweptTexts(catalog.T("view.urgency.total"), catalog.T("view.urgency.explain.total"), bench.FormatTenths(int64(priority+severity))),
	}
	return sweptExpectation{rows: rows, source: "the eight terms of the record's explained intake card and the total"}
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
	titled := false
	for _, line := range strings.Split(mustView(t, root), "\n") {
		fields := strings.Fields(line)
		titled = titled || (len(fields) > 1 && fields[0] == "plain" && fields[1] == "plain")
	}
	if !titled {
		t.Errorf("the listing does not title the view by its name:\n%s", mustView(t, root))
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
// terminal, once per defect token: the listing's human and JSON forms name
// the defect, a draw is refused naming it and the workbench layer, and check
// reports it, while the well-formed sibling in the same block draws. The
// view named for invalid-name is drawn by the name it carries, which the
// grammar refuses and the command still resolves.
func TestAMalformedViewIsListedRefusedAndChecked(t *testing.T) {
	root := newBench(t)
	declareViewsIn(t, root, bench.ViewsKey+":\n"+
		"  Bad_Name:\n    sections:\n      - query: \"state:ready\"\n"+
		"  flat: just some text\n"+
		"  nested-title:\n    title:\n      deep: value\n    sections:\n      - query: \"state:ready\"\n"+
		"  empty-sections:\n    sections: []\n"+
		"  no-query:\n    sections:\n      - title: only a title\n"+
		"  grid-layout:\n    layout: grid\n    sections:\n      - query: \"state:ready\"\n"+
		"  priority-order:\n    order: priority\n    sections:\n      - query: \"state:ready\"\n"+
		"  bogus-scope:\n    sections:\n      - scope: bogus\n"+
		"  sibling:\n    sections:\n      - query: \"state:ready\"\n")
	defects := map[string]string{
		"Bad_Name": bench.ViewInvalidName, "flat": bench.ViewNotAMapping, "nested-title": bench.ViewMalformedMember,
		"empty-sections": bench.ViewNoSections, "no-query": bench.ViewSectionWithoutQuery,
		"grid-layout": bench.ViewUnknownLayout, "priority-order": bench.ViewUnknownOrder,
		"bogus-scope": bench.ViewUnknownScope,
	}
	if len(defects) != len(bench.ViewDefects) {
		t.Fatalf("the fixture builds %d defects and the build declares %d", len(defects), len(bench.ViewDefects))
	}
	listing := mustView(t, root)
	machine := mustView(t, root, "--json")
	checked := runCLI(t, root, "check", "--json").out
	for name, token := range defects {
		if !strings.Contains(listing, "(malformed: "+token+")") {
			t.Errorf("the listing does not mark %s with %s:\n%s", name, token, listing)
		}
		if !strings.Contains(machine, `"malformed": "`+token+`"`) {
			t.Errorf("the JSON listing does not carry %s:\n%s", token, machine)
		}
		got := runCLI(t, root, "view", name, "--json")
		if got.code == 0 || !strings.Contains(got.out, `"refusal": "dinah.malformed-view"`) ||
			!strings.Contains(got.out, `"defect": "`+token+`"`) || !strings.Contains(got.out, `"source": "workbench"`) {
			t.Errorf("drawing %s answered %d:\n%s", name, got.code, got.out)
		}
		if !strings.Contains(checked, `"Detail": "`+name+" "+token+`"`) {
			t.Errorf("check does not report %s %s:\n%s", name, token, checked)
		}
	}
	mustView(t, root, "sibling")
}

// TestACardInTwoSectionsIsDrawnInBoth is the human half of
// dinah-600/criteria/19.
func TestACardInTwoSectionsIsDrawnInBoth(t *testing.T) {
	root := newBench(t)
	mustRunHere(t, root, "add", "Asked twice")
	declareViewsIn(t, root, bench.ViewsKey+":\n  twice:\n    sections:\n      - title: First\n        query: \"state:ready\"\n      - title: Second\n        query: \"column:intake\"\n")
	drawn := mustView(t, root, "twice")
	for _, heading := range []string{"First (1)", "Second (1)"} {
		if !strings.Contains(sectionOf(drawn, heading), "Asked twice") {
			t.Errorf("the section %s does not draw the card:\n%s", heading, drawn)
		}
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
	if help.code != 0 || !strings.HasPrefix(help.out, "view [view] [ref] [--explain] [--all] [--plain] [--watch]\n") || !strings.Contains(help.out, "dinah guide views") {
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

// fourteenColumns is a workbench shaped like this project's own: fourteen
// columns in one flow, the first an intake column and the last a done
// column, with Acceptance owned by the operator and the priorities the
// board draws. Every identifier is spelled so a test can name a column by
// its position.
const fourteenColumns = `{
  "profile": "dinah-core/0.7",
  "title": "Fourteen",
  "levels": { "priority": ["later", "soon", "next", "now"] },
  "columns": [
    { "id": "c00000000001", "title": "Intake", "kind": "intake" },
    { "id": "c00000000002", "title": "Triage", "kind": "work" },
    { "id": "c00000000003", "title": "Design Queue", "kind": "work" },
    { "id": "c00000000004", "title": "Spec", "kind": "work" },
    { "id": "c00000000005", "title": "Agent Design Review", "kind": "work" },
    { "id": "c00000000006", "title": "Operator Design Review", "kind": "work" },
    { "id": "c00000000007", "title": "Build Queue", "kind": "work" },
    { "id": "c00000000008", "title": "Implement", "kind": "work" },
    { "id": "c00000000009", "title": "Agent Code Review", "kind": "work" },
    { "id": "c00000000010", "title": "Operator Code Review", "kind": "work" },
    { "id": "c00000000011", "title": "Test", "kind": "work" },
    { "id": "c00000000012", "title": "Merge", "kind": "work" },
    { "id": "c00000000013", "title": "Acceptance", "kind": "work", "operator_owned": true },
    { "id": "c00000000014", "title": "Done", "kind": "done" }
  ]
}`

// fourteenTitles are the titles of fourteenColumns, in flow order.
var fourteenTitles = []string{
	"Intake", "Triage", "Design Queue", "Spec", "Agent Design Review", "Operator Design Review", "Build Queue",
	"Implement", "Agent Code Review", "Operator Code Review", "Test", "Merge", "Acceptance", "Done",
}

// columnID is the identifier fourteenColumns gives the column at a one-based
// position.
func columnID(position int) string {
	return "c000000000" + fmt.Sprintf("%02d", position)
}

// allColumnsView declares a columns view collapsing nothing, so every column
// holding a card is drawn.
const allColumnsView = bench.ViewsKey + ":\n  every:\n    title: Every\n    layout: columns\n    order: column\n    collapsed: []\n    sections:\n      - query: \"state:ready,active,blocked\"\n"

// addTo files a card straight into a column and answers its reference.
func addTo(t *testing.T, root string, position int, title string, flags ...string) string {
	t.Helper()
	got := mustRunHere(t, root, append([]string{"--json", "add", title, "--column", columnID(position)}, flags...)...)
	var answer struct {
		Card struct {
			Ref string `json:"ref"`
		} `json:"card"`
	}
	if err := json.Unmarshal([]byte(got.out), &answer); err != nil || answer.Card.Ref == "" {
		t.Fatalf("add answered %s", got.out)
	}
	return answer.Card.Ref
}

// drawAt draws a view with COLUMNS set to width, failing unless it exits zero.
func drawAt(t *testing.T, root string, width int, argv ...string) string {
	t.Helper()
	t.Setenv("COLUMNS", strconv.Itoa(width))
	return mustView(t, root, argv...)
}

// boardLinesOf splits a drawing into its lines, without the newline the last
// one ends in.
func boardLinesOf(out string) []string {
	return strings.Split(strings.TrimSuffix(out, "\n"), "\n")
}

// ruleRuns is every run of a rule glyph in a line, as the display column the
// run starts at and how many columns it covers.
func ruleRuns(line string, glyph rune) [][2]int {
	var runs [][2]int
	at := 0
	start := -1
	for _, r := range line {
		if r == glyph {
			if start < 0 {
				start = at
			}
		} else if start >= 0 {
			runs = append(runs, [2]int{start, at - start})
			start = -1
		}
		at += displayWidth(string(r))
	}
	if start >= 0 {
		runs = append(runs, [2]int{start, at - start})
	}
	return runs
}

// bandsOf reads a drawn board back into its bands: for each rule line, the
// runs of the rule glyph it carries and the heading line above it.
type drawnBand struct {
	heading string
	runs    [][2]int
	at      int
}

func bandsOf(lines []string, glyph rune) []drawnBand {
	var bands []drawnBand
	for i, line := range lines {
		runs := ruleRuns(line, glyph)
		if len(runs) == 0 || strings.Trim(line, string(glyph)+" ") != "" || i == 0 {
			continue
		}
		bands = append(bands, drawnBand{heading: lines[i-1], runs: runs, at: i})
	}
	return bands
}

// assertBoardBounds holds every line of a drawing to the promises every
// columns drawing makes: no wider than draw display columns, no trailing
// space, and no escape character.
func assertBoardBounds(t *testing.T, out string, draw int) {
	t.Helper()
	for _, line := range boardLinesOf(out) {
		if width := displayWidth(line); width > draw {
			t.Errorf("a line draws %d columns, wider than %d: %q", width, draw, line)
		}
		if strings.HasSuffix(line, " ") {
			t.Errorf("a line ends in a space: %q", line)
		}
	}
	if strings.ContainsRune(out, 0x1b) {
		t.Errorf("the drawing carries an escape:\n%q", out)
	}
}

// TestFourteenColumnsAtEightyDrawFiveBands is dinah-288/criteria/1 and
// dinah-288/criteria/13: a view collapsing nothing, over a card in each of
// fourteen columns, drawn at COLUMNS=80, draws five bands of 3, 3, 3, 3 and 2
// columns in flow order, every column 25 wide starting at display offsets 0,
// 27 and 54, a blank line between bands, no line wider than 79 and none
// ending in a space. The last band's second column runs out of cards before
// the first, which is where a trailing pad would show.
func TestFourteenColumnsAtEightyDrawFiveBands(t *testing.T) {
	root := newBenchFromDefinition(t, fourteenColumns)
	declareViewsIn(t, root, allColumnsView)
	for position := 1; position <= 14; position++ {
		addTo(t, root, position, "A card title long enough to be cut in column "+strconv.Itoa(position))
	}
	addTo(t, root, 13, "A second card in Acceptance")
	out := drawAt(t, root, 80, "every")
	assertBoardBounds(t, out, 79)
	lines := boardLinesOf(out)
	bands := bandsOf(lines, '─')
	wantSizes := []int{3, 3, 3, 3, 2}
	if len(bands) != len(wantSizes) {
		t.Fatalf("drew %d bands, want %d:\n%s", len(bands), len(wantSizes), out)
	}
	next := 0
	for b, band := range bands {
		if len(band.runs) != wantSizes[b] {
			t.Errorf("band %d holds %d columns, want %d", b+1, len(band.runs), wantSizes[b])
		}
		for k, run := range band.runs {
			if want := k * 27; run[0] != want || run[1] != 25 {
				t.Errorf("band %d column %d starts at %d and is %d wide, want %d and 25", b+1, k+1, run[0], run[1], want)
			}
			title := fourteenTitles[next]
			if !strings.Contains(band.heading, title+" (") && !strings.Contains(band.heading, cutTitle(title)) {
				t.Errorf("band %d heading does not carry %s in flow order: %q", b+1, title, band.heading)
			}
			next++
		}
		if b > 0 && lines[band.at-2] != "" {
			t.Errorf("band %d is not preceded by a blank line: %q", b+1, lines[band.at-2])
		}
	}
	if next != 14 {
		t.Errorf("the bands hold %d columns, want 14", next)
	}
}

// cutTitle is how the board cuts a column title that does not fit its
// heading at a column of 25 with a count of one digit.
func cutTitle(title string) string {
	return cutText(title, 25-len(" (1)"), tailEllipsis)
}

// TestTheBandArithmeticMatchesTheTable is dinah-288/criteria/2: every row
// of specification section 4.2's table through boardBands, and five of them
// drawn end to end.
func TestTheBandArithmeticMatchesTheTable(t *testing.T) {
	cases := []struct {
		window, visible, perBand, width, bands int
	}{
		{80, 14, 3, 25, 5}, {80, 5, 3, 25, 2}, {80, 2, 2, 38, 1}, {118, 5, 5, 21, 1},
		{120, 5, 5, 22, 1}, {31, 1, 1, 30, 1}, {20, 3, 1, 19, 3}, {12, 3, 1, 11, 3},
	}
	for _, c := range cases {
		perBand, width := boardBands(c.visible, c.window-1)
		if perBand != c.perBand || width != c.width {
			t.Errorf("%d visible at %d: %d per band of %d, want %d of %d", c.visible, c.window, perBand, width, c.perBand, c.width)
		}
	}
	for _, c := range []struct {
		window, visible, width, bands int
	}{{118, 5, 21, 1}, {120, 5, 22, 1}, {80, 2, 38, 1}, {31, 1, 30, 1}, {20, 3, 19, 3}} {
		root := newBenchFromDefinition(t, fourteenColumns)
		declareViewsIn(t, root, allColumnsView)
		for position := 2; position < 2+c.visible; position++ {
			addTo(t, root, position, "Card in column "+strconv.Itoa(position))
		}
		out := drawAt(t, root, c.window, "every")
		assertBoardBounds(t, out, c.window-1)
		bands := bandsOf(boardLinesOf(out), '─')
		if len(bands) != c.bands {
			t.Errorf("%d visible at %d drew %d bands, want %d:\n%s", c.visible, c.window, len(bands), c.bands, out)
			continue
		}
		for _, band := range bands {
			for _, run := range band.runs {
				if run[1] != c.width {
					t.Errorf("%d visible at %d drew a column %d wide, want %d", c.visible, c.window, run[1], c.width)
				}
			}
		}
	}
}

// TestAWideTitleIsCutWhole is dinah-288/criteria/3: a title of wide
// characters in a column of 30 is drawn in at most 30 columns ending in an
// ellipsis, a wide character that would need two columns where one remains
// leaves that column blank, and a title carrying an emoji ZWJ sequence, a
// flag and a base with a combining mark is cut only between whole units.
func TestAWideTitleIsCutWhole(t *testing.T) {
	root := newBenchFromDefinition(t, fourteenColumns)
	declareViewsIn(t, root, allColumnsView)
	addTo(t, root, 4, "看板を端末に描く：列の幅に合わせて切る")
	out := drawAt(t, root, 31, "every")
	assertBoardBounds(t, out, 30)
	title := ""
	for _, line := range boardLinesOf(out) {
		if strings.HasPrefix(line, "  看") {
			title = line
		}
	}
	if !strings.HasSuffix(title, tailEllipsis) || displayWidth(title) > 30 {
		t.Errorf("the wide title drew %q", title)
	}
	blank := boardTitleLine("ab看板", 6, unicodeGlyphs)
	if blank.text != "  ab…" {
		t.Errorf("a wide character needing two columns where one remains drew %q, want the column left blank", blank.text)
	}
	family := "\U0001F468\u200D\U0001F469\u200D\U0001F467"
	flag := "\U0001F1EF\U0001F1F5"
	mark := "e\u0301"
	title = "ab" + family + flag + mark + "cdefgh"
	boundaries := map[string]bool{}
	for _, unit := range []string{"a", "b", family, flag, mark, "c", "d", "e", "f", "g", "h"} {
		boundaries[strings.Join(append(keptUnits(boundaries), unit), "")] = true
	}
	boundaries[""] = true
	for width := 3; width <= displayWidth(title)+2; width++ {
		drawn := boardTitleLine(title, width, unicodeGlyphs).text
		kept := strings.TrimSuffix(strings.TrimLeft(drawn, " "), tailEllipsis)
		if !strings.HasPrefix(title, kept) || !boundaries[kept] {
			t.Errorf("at %d the title kept %q, which is not a run of whole units", width, kept)
		}
		if displayWidth(drawn) > width {
			t.Errorf("at %d the title line draws %d columns", width, displayWidth(drawn))
		}
	}
}

// keptUnits is the longest run of units already recorded as a boundary,
// which is the prefix the next unit extends. The boundaries are written out
// unit by unit rather than read from textwidth, so a broken unit walk cannot
// agree with itself here.
func keptUnits(boundaries map[string]bool) []string {
	longest := ""
	for prefix := range boundaries {
		if len(prefix) > len(longest) {
			longest = prefix
		}
	}
	if longest == "" {
		return nil
	}
	return []string{longest}
}

// colourSeam installs a terminal seam whose screen is the terminfo layer over
// the committed xterm-256color entry, writing through the session's own
// writer, and removes it when the test ends.
func colourSeam(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "internal", "screen", "testdata", "terminfo", "78", "xterm-256color"))
	if err != nil {
		t.Fatalf("read the xterm-256color fixture: %v", err)
	}
	entry, err := screen.ParseTerminfo(data)
	if err != nil {
		t.Fatalf("parse the xterm-256color fixture: %v", err)
	}
	terminalSeam = &terminalSeams{screen: func(text io.Writer) screen.Screen { return screen.NewTerminfo(entry, text) }}
	t.Cleanup(func() { terminalSeam = nil })
}

// sequences matches the two strings the xterm-256color entry's setaf and
// sgr0 produce for the board's three colours.
var sequences = regexp.MustCompile(`\x1b\[3[134]m|\x1b\(B\x1b\[m`)

// colourFixture is a workbench whose board carries a card in each state and
// one waiting on the operator, in an operator-owned column.
func colourFixture(t *testing.T) string {
	root := newBenchFromDefinition(t, fourteenColumns)
	declareViewsIn(t, root, allColumnsView)
	addTo(t, root, 8, "A ready card")
	held := addTo(t, root, 8, "A held card")
	mustRunHere(t, root, "claim", held)
	blocked := addTo(t, root, 8, "A blocked card")
	mustRunHere(t, root, "block", blocked, "waiting on a ruling", "--kind", "operator-ruling")
	addTo(t, root, 13, "Waiting at the operator's station")
	return root
}

// TestAPipedBoardIsTheSameDrawingEverywhere is dinah-288/criteria/4: with
// COLUMNS unset and output a pipe, the board is drawn at 80, every line in
// at most 79 columns, with no escape byte, identically across two runs, and
// identically to the drawing a terminal seam makes at COLUMNS=80 with colour
// off.
func TestAPipedBoardIsTheSameDrawingEverywhere(t *testing.T) {
	root := colourFixture(t)
	os.Unsetenv("COLUMNS")
	first := mustView(t, root, "every")
	second := mustView(t, root, "every")
	assertBoardBounds(t, first, 79)
	if first != second {
		t.Errorf("two piped runs differ:\n%s\n%s", first, second)
	}
	bands := bandsOf(boardLinesOf(first), '─')
	if len(bands) == 0 || bands[0].runs[0][1] != 38 {
		t.Errorf("the piped board was not laid out at 80:\n%s", first)
	}
	colourSeam(t)
	t.Setenv("NO_COLOR", "1")
	if terminal := drawAt(t, root, 80, "every"); terminal != first {
		t.Errorf("the terminal drawing with colour off differs from the piped one:\n%s\n%s", terminal, first)
	}
}

// TestNoColorRemovesColourOnly is dinah-288/criteria/5: on a terminal seam
// able to colour, NO_COLOR set to a value draws no colour and keeps the
// glyph set, and NO_COLOR empty or absent colours the glyph segments, with
// the coloured bytes stripped of the entry's own sequences equal to the
// NO_COLOR bytes.
func TestNoColorRemovesColourOnly(t *testing.T) {
	root := colourFixture(t)
	colourSeam(t)
	t.Setenv("NO_COLOR", "1")
	plain := drawAt(t, root, 80, "every")
	if strings.ContainsRune(plain, 0x1b) {
		t.Errorf("NO_COLOR=1 still coloured:\n%q", plain)
	}
	for _, glyph := range []string{"○", "●", "✕", "◆"} {
		if !strings.Contains(plain, glyph) {
			t.Errorf("NO_COLOR changed the glyph set: no %s in\n%s", glyph, plain)
		}
	}
	for _, value := range []string{"", "unset"} {
		if value == "unset" {
			os.Unsetenv("NO_COLOR")
		} else {
			t.Setenv("NO_COLOR", value)
		}
		coloured := drawAt(t, root, 80, "every")
		for _, want := range []string{"\x1b[34m●", "\x1b[31m✕", "\x1b[33m◆"} {
			if !strings.Contains(coloured, want) {
				t.Errorf("NO_COLOR %s did not colour %q:\n%q", value, want, coloured)
			}
		}
		if stripped := sequences.ReplaceAllString(coloured, ""); stripped != plain {
			t.Errorf("NO_COLOR %s: the coloured drawing without its sequences differs from the NO_COLOR one:\n%q\n%q", value, stripped, plain)
		}
	}
}

// TestAPlainBoardIsAscii is dinah-288/criteria/7: with --plain, and with the
// glyphs setting the operator's ruling of 2026-09-25 added, every byte of a
// board whose own text is ASCII is below 0x80, the marks are o, *, x and !,
// the rule is -, and a cut ends in three full stops.
func TestAPlainBoardIsAscii(t *testing.T) {
	root := colourFixture(t)
	addTo(t, root, 8, "A card title long enough to be cut at a column of twenty five")
	plain := drawAt(t, root, 80, "every", "--plain")
	for i := 0; i < len(plain); i++ {
		if plain[i] >= 0x80 {
			t.Fatalf("byte %d of the plain board is %#x:\n%s", i, plain[i], plain)
		}
	}
	for _, want := range []string{"o 1", "* 2", "x 3", "Acceptance ! (1)", "-------------------------", "..."} {
		if !strings.Contains(plain, want) {
			t.Errorf("the plain board carries no %q:\n%s", want, plain)
		}
	}
	writeUserConfig(t, "---\nglyphs: plain\n---\n")
	if set := drawAt(t, root, 80, "every"); set != plain {
		t.Errorf("glyphs: plain did not draw the plain board:\n%s", set)
	}
}

// TestTheCapShowsFiveAndCountsTheRest is dinah-288/criteria/8: a column of
// eight cards shows five and +3 more, --all shows all eight and no count,
// and the machine form carries all eight either way.
func TestTheCapShowsFiveAndCountsTheRest(t *testing.T) {
	root := newBenchFromDefinition(t, fourteenColumns)
	declareViewsIn(t, root, allColumnsView)
	for i := 1; i <= 8; i++ {
		addTo(t, root, 3, "Queued card "+strconv.Itoa(i))
	}
	capped := drawAt(t, root, 80, "every")
	if got := strings.Count(capped, "Queued card"); got != 5 || !strings.Contains(capped, "+3 more") {
		t.Errorf("the capped column drew %d cards:\n%s", got, capped)
	}
	all := drawAt(t, root, 80, "every", "--all")
	if got := strings.Count(all, "Queued card"); got != 8 || strings.Contains(all, "more") {
		t.Errorf("--all drew %d cards:\n%s", got, all)
	}
	for _, argv := range [][]string{{"every", "--json"}, {"every", "--all", "--json"}} {
		var answer verb.ViewAnswer
		if err := json.Unmarshal([]byte(mustView(t, root, argv...)), &answer); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if got := len(answer.View.Sections[0].Cards); got != 8 {
			t.Errorf("%v carries %d cards, want 8", argv, got)
		}
	}
}

// TestCollapsedColumnsAreCounted is dinah-288/criteria/9: the built-in board
// draws no column for the intake and done columns and names them with their
// counts, collapsed: [] draws them, a collapsed column holding none of the
// view's cards is not named, and an entry naming no column is ignored.
func TestCollapsedColumnsAreCounted(t *testing.T) {
	root := newBenchFromDefinition(t, fourteenColumns)
	declareViewsIn(t, root, allColumnsView+
		"  missing:\n    layout: columns\n    collapsed: [\"nowhere\", \"Implement\"]\n    sections:\n      - query: \"state:ready,active,blocked\"\n")
	addTo(t, root, 1, "Waiting in intake")
	addTo(t, root, 1, "Also waiting in intake")
	addTo(t, root, 8, "Being built")
	addTo(t, root, 14, "Finished")
	board := drawAt(t, root, 80, "board")
	if !strings.Contains(board, "Collapsed: Intake 2, Done 1\n") || strings.Contains(board, "Intake (") || strings.Contains(board, "Done (") {
		t.Errorf("the built-in board did not collapse intake and done:\n%s", board)
	}
	if every := drawAt(t, root, 80, "every"); !strings.Contains(every, "Intake (2)") || strings.Contains(every, "Collapsed") {
		t.Errorf("collapsed: [] did not draw the intake column:\n%s", every)
	}
	missing := drawAt(t, root, 80, "missing")
	if !strings.Contains(missing, "Collapsed: Implement 1\n") || !strings.Contains(missing, "Intake (2)") {
		t.Errorf("a view collapsing a missing column and Implement drew:\n%s", missing)
	}
}

// TestAColumnHoldingNoCardTakesNoWidth is dinah-288/criteria/10: cards in
// two of fourteen columns at 80 draw one band of two columns of 38.
func TestAColumnHoldingNoCardTakesNoWidth(t *testing.T) {
	root := newBenchFromDefinition(t, fourteenColumns)
	declareViewsIn(t, root, allColumnsView)
	addTo(t, root, 3, "Queued")
	addTo(t, root, 11, "Under test")
	bands := bandsOf(boardLinesOf(drawAt(t, root, 80, "every")), '─')
	if len(bands) != 1 || len(bands[0].runs) != 2 || bands[0].runs[0][1] != 38 || bands[0].runs[1][0] != 40 {
		t.Errorf("two occupied columns drew %v", bands)
	}
}

// TestACardGivesUpFieldsInOrder is dinah-288/criteria/11: the holder and
// the priority together where both fit, the priority dropped first, the
// holder then cut while at least four columns remain for it, then dropped,
// with the glyph, the number and the title present at every width and
// severity never drawn.
func TestACardGivesUpFieldsInOrder(t *testing.T) {
	card := boardCard{glyph: "●", colour: screen.Blue, number: "598", holder: "claude-reviewer", priority: "now", title: "A starved run"}
	cases := []struct {
		width int
		want  string
	}{
		{30, "● 598  claude-reviewer  now"},
		{26, "● 598  claude-reviewer"},
		{22, "● 598  claude-reviewer"},
		{21, "● 598  claude-review…"},
		{11, "● 598  cla…"},
		{10, "● 598"},
		{5, "● 598"},
		{4, "● 5…"},
	}
	for _, c := range cases {
		if got := boardCardLine(card, c.width, unicodeGlyphs).text; got != c.want {
			t.Errorf("at %d the first line is %q, want %q", c.width, got, c.want)
		}
		if title := boardTitleLine(card.title, c.width, unicodeGlyphs).text; title == "" {
			t.Errorf("at %d the title line is empty", c.width)
		}
	}
	root := colourFixture(t)
	mustRunHere(t, root, "set", "fx-1", "priority", "now")
	if board := drawAt(t, root, 80, "every"); strings.Contains(board, "severity") || !strings.Contains(board, "○ 1  now") {
		t.Errorf("the board drew:\n%s", board)
	}
}

// TestTheOperatorMarkStaysWhole is dinah-288/criteria/12: an operator-owned
// column's heading carries the mark before its count and a card waiting on
// the operator carries it after its number, in both glyph sets, and a title
// too wide for its heading is cut with the mark and the count kept whole.
func TestTheOperatorMarkStaysWhole(t *testing.T) {
	for _, glyphs := range []boardGlyphs{unicodeGlyphs, plainGlyphs} {
		heading := boardHeadingCell(boardColumn{title: "Operator Design Review", operator: true, count: "(12)"}, 21, glyphs)
		want := cutText("Operator Design Review", 21-displayWidth(" "+glyphs.operator+" (12)"), glyphs.ellipsis) + " " + glyphs.operator + " (12)"
		if heading.text != want || displayWidth(heading.text) > 21 {
			t.Errorf("the cut heading is %q, want %q", heading.text, want)
		}
		marked := boardCardLine(boardCard{glyph: glyphs.ready, number: "565", mark: true}, 25, glyphs)
		if marked.text != glyphs.ready+" 565 "+glyphs.operator {
			t.Errorf("a card waiting on the operator drew %q", marked.text)
		}
	}
	root := newBenchFromDefinition(t, fourteenColumns)
	declareViewsIn(t, root, allColumnsView)
	card := addTo(t, root, 13, "Awaiting acceptance")
	mustRunHere(t, root, "file", card, "open_question", "Ship it?", "--owner", "operator", "--column", columnID(13))
	board := drawAt(t, root, 80, "every")
	if !strings.Contains(board, "Acceptance ◆ (1)") || !strings.Contains(board, "○ 1 ◆") {
		t.Errorf("the marks are missing:\n%s", board)
	}
}

// TestAControlCharacterIsDrawnAsASpace is dinah-288/criteria/14: a title
// and a holder carrying an escape and a C1 control are drawn with each
// replaced by a space, and the grid stays aligned.
func TestAControlCharacterIsDrawnAsASpace(t *testing.T) {
	card := boardCardOf(verb.CardView{Ref: "fx-9", State: "active", Holder: "al\x1bka", Title: "bad\x1b[31mtitle\u0085end"}, unicodeGlyphs)
	if card.holder != "al ka" || card.title != "bad [31mtitle end" {
		t.Errorf("the controls survived: %q %q", card.holder, card.title)
	}
	root := newBenchFromDefinition(t, fourteenColumns)
	declareViewsIn(t, root, allColumnsView)
	ref := addTo(t, root, 3, "A plain title")
	addTo(t, root, 4, "Beside it")
	rewriteAnchor(t, root, ref, "title: A plain title", "title: bad\x1btitle")
	out := drawAt(t, root, 80, "every")
	assertBoardBounds(t, out, 79)
	bands := bandsOf(boardLinesOf(out), '─')
	if len(bands) != 1 || len(bands[0].runs) != 2 || bands[0].runs[1][0] != 40 {
		t.Errorf("the grid moved:\n%s", out)
	}
}

// TestTheBoardIsABuiltInView is dinah-288/criteria/15: dinah view lists
// board as a built-in view with layout columns, and a workbench view named
// board replaces it.
func TestTheBoardIsABuiltInView(t *testing.T) {
	root := newBench(t)
	listing := mustView(t, root, "--json")
	var answer verb.ViewListing
	if err := json.Unmarshal([]byte(listing), &answer); err != nil {
		t.Fatalf("decode: %v", err)
	}
	found := false
	for _, row := range answer.Views {
		if row.Name == "board" && row.Source == bench.ViewSourceBuiltIn && row.Layout == bench.ViewLayoutColumns && row.Used {
			found = true
		}
	}
	if !found {
		t.Errorf("the listing carries no built-in board:\n%s", listing)
	}
	declareViewsIn(t, root, bench.ViewsKey+":\n  board:\n    title: Our board\n    sections:\n      - query: \"state:ready\"\n")
	if drawn := mustView(t, root, "board"); !strings.HasPrefix(drawn, "Our board") {
		t.Errorf("the workbench's board did not replace the built-in one:\n%s", drawn)
	}
}

// TestTheMachineFormNamesTheCollapsedColumns is the terminal half of
// dinah-288/criteria/16: the JSON answer of the built-in board carries
// layout columns and the collapsed intake and done columns in flow order, and
// a list view carries collapsed as an empty array.
func TestTheMachineFormNamesTheCollapsedColumns(t *testing.T) {
	root := newBenchFromDefinition(t, fourteenColumns)
	declareViewsIn(t, root, bench.ViewsKey+":\n  plainlist:\n    sections:\n      - query: \"state:ready\"\n")
	addTo(t, root, 2, "Triaged")
	var board, list struct {
		View map[string]json.RawMessage `json:"view"`
	}
	if err := json.Unmarshal([]byte(mustView(t, root, "board", "--json")), &board); err != nil {
		t.Fatal(err)
	}
	var collapsed []string
	if err := json.Unmarshal(board.View["collapsed"], &collapsed); err != nil {
		t.Fatalf("the board's collapsed member is %s", board.View["collapsed"])
	}
	if strings.Join(collapsed, " ") != columnID(1)+" "+columnID(14) || string(board.View["layout"]) != `"columns"` {
		t.Errorf("the board answered collapsed %v and layout %s", collapsed, board.View["layout"])
	}
	if err := json.Unmarshal([]byte(mustView(t, root, "plainlist", "--json")), &list); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(strings.Fields(string(list.View["collapsed"])), ""); got != "[]" {
		t.Errorf("a list view answered collapsed %s", got)
	}
}

// TestABoardUnderTwentyColumnsStaysInside is dinah-288/criteria/26 and the
// narrow cases the second design review asked for: at COLUMNS=12 three
// columns draw three bands of one column 11 wide and no line is wider than
// 11; at 2 and 3 with --plain, where the plain ellipsis is wider than the
// room, no line reaches past the window less one, and nothing ends in a
// space. windowWidth still clamps to 20 for every table.
func TestABoardUnderTwentyColumnsStaysInside(t *testing.T) {
	root := colourFixture(t)
	for i := 1; i <= 7; i++ {
		addTo(t, root, 3, "Queued card with a long title "+strconv.Itoa(i))
	}
	mustRunHere(t, root, "file", "fx-4", "open_question", "Ship it?", "--owner", "operator", "--column", columnID(13))
	narrow := drawAt(t, root, 12, "every")
	assertBoardBounds(t, narrow, 11)
	bands := bandsOf(boardLinesOf(narrow), '─')
	if len(bands) != 3 {
		t.Errorf("drew %d bands at 12, want 3:\n%s", len(bands), narrow)
	}
	for _, band := range bands {
		if len(band.runs) != 1 || band.runs[0][1] != 11 {
			t.Errorf("a band at 12 is %v", band.runs)
		}
	}
	for _, width := range []int{2, 3} {
		out := drawAt(t, root, width, "every", "--plain")
		assertBoardBounds(t, out, width-1)
	}
	t.Setenv("COLUMNS", "12")
	if got := windowWidth(); got != 20 {
		t.Errorf("windowWidth at COLUMNS=12 answered %d, want the tables' 20", got)
	}
}

// TestCardsInUnlistedColumnsDrawAsTrailingColumns is dinah-288/criteria/33:
// cards standing in two columns the flow does not list draw as two columns
// after every flow column, in byte order of their identifiers, each titled
// by the identifier where no stored title names it.
func TestCardsInUnlistedColumnsDrawAsTrailingColumns(t *testing.T) {
	root := newBenchFromDefinition(t, fourteenColumns)
	declareViewsIn(t, root, allColumnsView)
	late := addTo(t, root, 3, "Standing in an unlisted column late in byte order")
	early := addTo(t, root, 3, "Standing in an unlisted column early in byte order")
	addTo(t, root, 12, "In the flow")
	rewriteAnchor(t, root, late, "column: "+columnID(3), "column: ffffffffff02")
	rewriteAnchor(t, root, early, "column: "+columnID(3), "column: ffffffffff01")
	out := drawAt(t, root, 80, "every")
	bands := bandsOf(boardLinesOf(out), '─')
	if len(bands) != 1 {
		t.Fatalf("drew %d bands:\n%s", len(bands), out)
	}
	merge := strings.Index(bands[0].heading, "Merge (1)")
	first := strings.Index(bands[0].heading, "ffffffffff01 (1)")
	second := strings.Index(bands[0].heading, "ffffffffff02 (1)")
	if merge < 0 || first <= merge || second <= first {
		t.Errorf("the trailing columns are out of order: %q", bands[0].heading)
	}
}

// TestTheLayoutsEdgesHold covers the positions of the board's layout that
// no ordinary workbench reaches: a band line where every column has run out
// of text, a section whose every card stands in a collapsed column, a title
// whose first character is wider than a column one character wide, with the
// Unicode ellipsis and with the plain one that is wider than the room, and a
// status line whose change begins with a wide character and must be cut to
// its first unit.
func TestTheLayoutsEdgesHold(t *testing.T) {
	band := boardBand([][]drawnLine{{{text: "a"}, {}, {text: "b"}}, {{text: "c"}}}, 5)
	if len(band) != 3 || band[1].text != "" || band[2].text != "b" {
		t.Errorf("a band whose columns are empty on one line drew %v", band)
	}
	if perBand, width := boardBands(0, 79); perBand != 1 || width != 79 {
		t.Errorf("a section with no visible column banded %d of %d", perBand, width)
	}
	if lines := boardLines(nil, 79, unicodeGlyphs); len(lines) != 0 {
		t.Errorf("a section with no visible column drew %v", lines)
	}
	if wide := boardTitleLine("看板", 1, unicodeGlyphs); wide.text != "…" {
		t.Errorf("a wide title in a column one character wide drew %q, want the ellipsis alone", wide.text)
	}
	if wide := boardTitleLine("看板", 1, plainGlyphs); wide.text != "" {
		t.Errorf("a wide title in a column narrower than the plain ellipsis drew %q, want nothing", wide.text)
	}
	line := statusLine("updated 09:41:07", "看板 moved Spec to Merge by claude", "Ctrl+C stops", " · ", 20, tailEllipsis)
	if line != "看… · Ctrl+C stops" {
		t.Errorf("a change starting with a wide character was cut to %q", line)
	}
}

// TestAColumnsViewHonoursTheAgenda is dinah-288/criteria/30, against what
// dinah-602 shipped. A columns view ordered by urgency draws the board with
// each column's cards in urgency order and no Rank, Urgency or Why text.
// --explain on it draws the agenda's explanation blocks in place of the
// board, and on a columns view ordered by column it refuses with
// dinah.view-not-ranked. A card after the view's name narrows the board to
// that card's column, drawn even where the view collapses it, and every
// other section draws that nothing matches. A watch of the explained view
// draws the same blocks.
func TestAColumnsViewHonoursTheAgenda(t *testing.T) {
	root := newBenchFromDefinition(t, fourteenColumns)
	declareViewsIn(t, root, allColumnsView+
		"  ranked-board:\n    title: Sorted board\n    layout: columns\n    order: urgency\n    collapsed: []\n    sections:\n      - query: \"state:ready,active,blocked\"\n"+
		"  two-sections:\n    title: Two sections\n    layout: columns\n    sections:\n      - title: Queued\n        query: \"column:"+columnID(3)+"\"\n      - title: Building\n        query: \"column:"+columnID(8)+"\"\n")
	low := addTo(t, root, 3, "Low priority work", "--priority", "later")
	high := addTo(t, root, 3, "High priority work", "--priority", "now")
	mid := addTo(t, root, 3, "Middle priority work", "--priority", "soon")
	intake := addTo(t, root, 1, "Waiting in intake")

	ranked := drawAt(t, root, 80, "ranked-board")
	assertBoardBounds(t, ranked, 79)
	h, m, l := strings.Index(ranked, "High priority"), strings.Index(ranked, "Middle priority"), strings.Index(ranked, "Low priority")
	if h < 0 || m < 0 || l < 0 || !(h < m && m < l) {
		t.Errorf("the ranked board does not list its column in urgency order:\n%s", ranked)
	}
	for _, heading := range []string{"Rank", "Urgency", "Why"} {
		if strings.Contains(ranked, heading) {
			t.Errorf("the ranked board draws the ranked table's %s:\n%s", heading, ranked)
		}
	}
	if bands := bandsOf(boardLinesOf(ranked), '─'); len(bands) != 1 || len(bands[0].runs) != 2 {
		t.Errorf("the ranked board is not a board of the two occupied columns:\n%s", ranked)
	}

	explained := drawAt(t, root, 80, "ranked-board", "--explain")
	if !strings.Contains(explained, "the sum of the terms above") || strings.Contains(explained, "─────") {
		t.Errorf("--explain on a columns view did not draw the explanation blocks alone:\n%s", explained)
	}
	for _, ref := range []string{high, mid, low} {
		if !strings.Contains(explained, ref+": ") {
			t.Errorf("--explain names no block for %s:\n%s", ref, explained)
		}
	}
	notRanked := runCLI(t, root, "view", "every", "--explain")
	if notRanked.code == 0 || !strings.HasPrefix(notRanked.errw, contract.ViewNotRanked+" ") {
		t.Errorf("--explain on a columns view ordered by column answered %d %q", notRanked.code, notRanked.errw)
	}

	narrowed := drawAt(t, root, 80, "board", intake)
	if !strings.Contains(narrowed, "Intake (1)") || strings.Contains(narrowed, "Collapsed") || strings.Contains(narrowed, "Design Queue") {
		t.Errorf("the board narrowed to a card in a collapsed column drew:\n%s", narrowed)
	}
	if bands := bandsOf(boardLinesOf(narrowed), '─'); len(bands) != 1 || len(bands[0].runs) != 1 {
		t.Errorf("the narrowed board is not one band of one column:\n%s", narrowed)
	}
	sections := drawAt(t, root, 80, "two-sections", high)
	queued, building := strings.Index(sections, "Queued (1)"), strings.Index(sections, "Building (0)")
	if queued < 0 || building < 0 || !strings.Contains(sections[building:], "Nothing matches.") || strings.Contains(sections, "Low priority") {
		t.Errorf("a two-section board narrowed to one card drew:\n%s", sections)
	}

	rig := newWatchRig(t, 80, 40)
	watching := runAside(t, root, "view", "ranked-board", "--explain", "--watch")
	waitFor(t, 5*time.Second, "the first frame", func() bool { return len(rig.scr.allFrames()) >= 1 })
	rig.interrupt()
	watching.finish(t)
	if frame := rig.scr.allFrames()[0].text(); !strings.Contains(frame, "the sum of the terms above") {
		t.Errorf("a watch of the explained view did not draw the blocks:\n%s", frame)
	}
}
