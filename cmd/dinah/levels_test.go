package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
	"dinah/internal/verb"
)

// bothAxesDefinition declares the two level sets docs/design/format.md uses as
// its own example, which is what the Dinah board itself declares.
const bothAxesDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Levelled",
  "levels": { "severity": ["trivial", "minor", "major", "critical"], "priority": ["later", "soon", "next", "now"] },
  "columns": [
    { "id": "b00000000001", "title": "Intake", "kind": "intake" },
    { "id": "b00000000002", "title": "Done", "kind": "done" }
  ]
}`

// severityOnlyDefinition is the workbench dinah-193 AC-26 is written against:
// one axis declared and the other not, which is an ordinary workbench rather
// than a degenerate one, and the case a single workbench-wide "does this
// declare any levels at all" gate would pass while breaking the format.
const severityOnlyDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Severity only",
  "levels": { "severity": ["trivial", "minor", "major", "critical"] },
  "columns": [
    { "id": "b00000000001", "title": "Intake", "kind": "intake" },
    { "id": "b00000000002", "title": "Done", "kind": "done" }
  ]
}`

// newBenchFromDefinition builds a workbench from an interchange definition and
// returns the container every runCLI below is run in.
func newBenchFromDefinition(t *testing.T, definition string) string {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "workbench")
	source := filepath.Join(base, "definition.json")
	t.Setenv("DINAH_HOME", filepath.Join(base, "home"))
	t.Setenv("DINAH_ACTOR", "alka")
	t.Setenv("DINAH_LANG", "")
	t.Setenv("DINAH_FORMAT", "")
	t.Setenv("DINAH_WORKBENCH", "")
	if err := os.WriteFile(source, []byte(definition), 0o644); err != nil {
		t.Fatalf("definition: %v", err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := runCLI(t, root, "init", "--from", source, "--slug", "fx", "--operator", "alka"); got.code != 0 {
		t.Fatalf("init: %d %s", got.code, got.errw)
	}
	return root
}

// anchorText reads a card's anchor back through the path the tool itself
// reports, so no test has to know where a card directory sits.
func anchorText(t *testing.T, root, ref string) string {
	t.Helper()
	got := runCLI(t, root, "path", ref)
	if got.code != 0 {
		t.Fatalf("path %s: %d %s", ref, got.code, got.errw)
	}
	data, err := os.ReadFile(strings.TrimSpace(got.out))
	if err != nil {
		t.Fatalf("read the anchor of %s: %v", ref, err)
	}
	return string(data)
}

// cardEvents reads a card's own journal back as decoded events.
func cardEvents(t *testing.T, root, ref string) []bench.Event {
	t.Helper()
	got := runCLI(t, root, "path", ref)
	if got.code != 0 {
		t.Fatalf("path %s: %d %s", ref, got.code, got.errw)
	}
	journal := filepath.Join(filepath.Dir(strings.TrimSpace(got.out)), bench.JournalName)
	data, err := os.ReadFile(journal)
	if err != nil {
		t.Fatalf("read the journal of %s: %v", ref, err)
	}
	var events []bench.Event
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event bench.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decode %q: %v", line, err)
		}
		events = append(events, event)
	}
	return events
}

// updatesOf is the card_updated lines of a journal, which is what most of the
// assertions below count.
func updatesOf(events []bench.Event) []bench.Event {
	var updates []bench.Event
	for _, event := range events {
		if event.Event == contract.EventCardUpdated {
			updates = append(updates, event)
		}
	}
	return updates
}

// refusalNameOf reads the refusal name off stderr, which is the first
// whitespace-delimited token of the composed refusal.
func refusalNameOf(errw string) string {
	return strings.Fields(errw)[0]
}

// TestFilingACardWithBothLevelsWritesThePairUnderState asserts dinah-193
// AC-5 and AC-6: the two flags land in the anchor in order under state, and
// a filing that names neither leaves absence as absence rather than writing
// two empty values.
func TestFilingACardWithBothLevelsWritesThePairUnderState(t *testing.T) {
	root := newBenchFromDefinition(t, bothAxesDefinition)
	if got := runCLI(t, root, "add", "--severity", "major", "--priority", "now", "a classified card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if written := anchorText(t, root, "fx-1"); !strings.Contains(written, "state: ready\nseverity: major\npriority: now\n") {
		t.Errorf("the pair did not land under state in order:\n%s", written)
	}
	if got := runCLI(t, root, "add", "a card nobody has classified yet"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	written := anchorText(t, root, "fx-2")
	for _, key := range []string{"severity", "priority"} {
		if strings.Contains(written, key+":") {
			t.Errorf("a filing that named no level wrote %s anyway:\n%s", key, written)
		}
	}
}

// TestWritingALevelOnAFiledCardJournalsOneLine asserts dinah-193 AC-7, AC-8
// and AC-9 together, because the three are one story: a write, the clear that
// undoes it, and the write that changes nothing.
func TestWritingALevelOnAFiledCardJournalsOneLine(t *testing.T) {
	root := newBenchFromDefinition(t, bothAxesDefinition)
	if got := runCLI(t, root, "add", "--severity", "minor", "a card to reclassify"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "set", "fx-1", "severity", "major"); got.code != 0 {
		t.Fatalf("card set: %d %s", got.code, got.errw)
	}
	if written := anchorText(t, root, "fx-1"); !strings.Contains(written, "severity: major\n") {
		t.Errorf("the write did not land:\n%s", written)
	}
	updates := updatesOf(cardEvents(t, root, "fx-1"))
	if len(updates) != 1 {
		t.Fatalf("the write appended %d card_updated lines, wanted one", len(updates))
	}
	if updates[0].Field != "severity" || updates[0].From != "minor" || updates[0].To != "major" {
		t.Errorf("the line reads field %q, from %q, to %q", updates[0].Field, updates[0].From, updates[0].To)
	}

	// AC-9: writing the level the card already carries succeeds, writes
	// nothing and journals nothing.
	before := anchorText(t, root, "fx-1")
	if got := runCLI(t, root, "set", "fx-1", "severity", "major"); got.code != 0 {
		t.Fatalf("rewriting the same level: %d %s", got.code, got.errw)
	}
	if after := anchorText(t, root, "fx-1"); after != before {
		t.Errorf("rewriting the same level rewrote the anchor:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if again := updatesOf(cardEvents(t, root, "fx-1")); len(again) != 1 {
		t.Errorf("rewriting the same level appended a journal line, leaving %d", len(again))
	}

	// AC-8: the clear removes the key and journals a line whose to member is
	// absent, which is what omitempty makes of an empty value.
	if got := runCLI(t, root, "set", "fx-1", "severity"); got.code != 0 {
		t.Fatalf("clearing: %d %s", got.code, got.errw)
	}
	if written := anchorText(t, root, "fx-1"); strings.Contains(written, "severity") {
		t.Errorf("the clear left the key on the anchor:\n%s", written)
	}
	cleared := updatesOf(cardEvents(t, root, "fx-1"))
	if len(cleared) != 2 {
		t.Fatalf("the clear left %d card_updated lines, wanted two", len(cleared))
	}
	if cleared[1].From != "major" || cleared[1].To != "" {
		t.Errorf("the clear's line reads from %q, to %q, and a cleared field goes from something to nothing", cleared[1].From, cleared[1].To)
	}
	if raw := rawJournalLines(t, root, "fx-1"); strings.Contains(raw[len(raw)-1], `"to"`) {
		t.Errorf("the clear's line carries a to member: %s", raw[len(raw)-1])
	}
}

// rawJournalLines reads a card's journal back as the undecoded lines it holds,
// which is what an assertion about an absent member has to read.
func rawJournalLines(t *testing.T, root, ref string) []string {
	t.Helper()
	got := runCLI(t, root, "path", ref)
	if got.code != 0 {
		t.Fatalf("path %s: %d %s", ref, got.code, got.errw)
	}
	journal := filepath.Join(filepath.Dir(strings.TrimSpace(got.out)), bench.JournalName)
	data, err := os.ReadFile(journal)
	if err != nil {
		t.Fatalf("read the journal of %s: %v", ref, err)
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

// TestReadingALevelBackPrintsOneLine asserts dinah-193 AC-10: get prints the
// stored level on a line of its own, and prints an empty line rather than
// refusing for a card carrying none.
func TestReadingALevelBackPrintsOneLine(t *testing.T) {
	root := newBenchFromDefinition(t, bothAxesDefinition)
	if got := runCLI(t, root, "add", "--severity", "minor", "a half-classified card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	stored := runCLI(t, root, "get", "fx-1", "severity")
	if stored.code != 0 {
		t.Fatalf("card get: %d %s", stored.code, stored.errw)
	}
	if stored.out != "minor\n" {
		t.Errorf("the stored level printed as %q, wanted one line reading minor", stored.out)
	}
	absent := runCLI(t, root, "get", "fx-1", "priority")
	if absent.code != 0 {
		t.Errorf("reading a level the card does not carry exited %d: %s", absent.code, absent.errw)
	}
	if absent.out != "\n" {
		t.Errorf("a card carrying no level printed %q, wanted one empty line", absent.out)
	}
}

// TestNamingALevelTheWorkbenchDoesNotDeclareRefuses asserts dinah-193 AC-11:
// the refusal names the level the reader typed and lists the ones that axis
// declares, in declaration order, so the reader picks from the answer.
func TestNamingALevelTheWorkbenchDoesNotDeclareRefuses(t *testing.T) {
	root := newBenchFromDefinition(t, bothAxesDefinition)
	if got := runCLI(t, root, "add", "a card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	refused := runCLI(t, root, "set", "fx-1", "severity", "urgent")
	if refused.code != 2 {
		t.Fatalf("naming an undeclared level exited %d, wanted 2: %s", refused.code, refused.errw)
	}
	if name := refusalNameOf(refused.errw); name != contract.UnknownLevel {
		t.Errorf("the refusal name is %s, wanted %s", name, contract.UnknownLevel)
	}
	for _, phrase := range []string{"urgent", "trivial, minor, major, critical", "severity"} {
		if !strings.Contains(refused.errw, phrase) {
			t.Errorf("the refusal does not carry %q:\n%s", phrase, refused.errw)
		}
	}
	if strings.Contains(refused.errw, "critical, major") {
		t.Errorf("the declared levels are not listed in declaration order:\n%s", refused.errw)
	}
}

// TestOneWorkbenchDeclaringOneAxisAnswersBothPaths asserts dinah-193 AC-26,
// which is the criterion a single workbench-wide "does this declare any levels
// at all" gate fails. It also carries AC-12, since the refusal it demands is
// decided from the named axis's own declaration.
func TestOneWorkbenchDeclaringOneAxisAnswersBothPaths(t *testing.T) {
	root := newBenchFromDefinition(t, severityOnlyDefinition)
	if got := runCLI(t, root, "add", "a card on a half-declared workbench"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "set", "fx-1", "severity", "major"); got.code != 0 {
		t.Fatalf("a severity write on a workbench that declares severity: %d %s", got.code, got.errw)
	}
	if written := anchorText(t, root, "fx-1"); !strings.Contains(written, "severity: major\n") {
		t.Errorf("the severity write did not land:\n%s", written)
	}
	refused := runCLI(t, root, "set", "fx-1", "priority", "now")
	if refused.code != 2 {
		t.Fatalf("a priority write on a workbench that declares no priority exited %d, wanted 2: %s", refused.code, refused.errw)
	}
	if name := refusalNameOf(refused.errw); name != contract.NoLevels {
		t.Errorf("the refusal name is %s, wanted %s", name, contract.NoLevels)
	}
	if !strings.Contains(refused.errw, "priority") {
		t.Errorf("the refusal does not name the axis it was raised over:\n%s", refused.errw)
	}
	if strings.Contains(refused.errw, "no levels,") || strings.Contains(refused.errw, "declares no levels") {
		t.Errorf("the sentence reports the workbench rather than the axis:\n%s", refused.errw)
	}
	if !strings.Contains(refused.errw, bench.WorkbenchAnchor) {
		t.Errorf("the refusal does not name the file the declaration belongs in:\n%s", refused.errw)
	}
	if strings.Contains(refused.errw, "unknown") {
		t.Errorf("the sentence calls the level unknown, and on this axis every name would be:\n%s", refused.errw)
	}

	// The same pair holds for the two flags of add, since each is evaluated
	// against the axis its own flag names.
	if got := runCLI(t, root, "add", "--severity", "major", "a card filed with a severity"); got.code != 0 {
		t.Fatalf("add --severity on a workbench that declares severity: %d %s", got.code, got.errw)
	}
	refusedAdd := runCLI(t, root, "add", "--priority", "now", "a card filed with a priority")
	if refusedAdd.code != 2 {
		t.Fatalf("add --priority on a workbench that declares no priority exited %d, wanted 2", refusedAdd.code)
	}
	if name := refusalNameOf(refusedAdd.errw); name != contract.NoLevels {
		t.Errorf("the add refusal name is %s, wanted %s", name, contract.NoLevels)
	}
	if !strings.Contains(refusedAdd.errw, "priority") {
		t.Errorf("the add refusal does not name the axis:\n%s", refusedAdd.errw)
	}
	// An invocation carrying both flags refuses over priority alone.
	both := runCLI(t, root, "add", "--severity", "major", "--priority", "now", "a card naming both")
	if both.code != 2 || refusalNameOf(both.errw) != contract.NoLevels || !strings.Contains(both.errw, "priority") {
		t.Errorf("naming both flags refused as %d %q, wanted dinah.no-levels over priority", both.code, both.errw)
	}
}

// TestNamingAFieldACardDoesNotRecordRefuses asserts dinah-193 AC-13: the
// refusal names the fields a card records inside its own sentence, prints no
// listing table beneath it, and prints none of the query's ordered-operator
// clause, which is written about the query language and says nothing a card
// reader can use.
func TestNamingAFieldACardDoesNotRecordRefuses(t *testing.T) {
	root := newBenchFromDefinition(t, bothAxesDefinition)
	if got := runCLI(t, root, "add", "a card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	refused := runCLI(t, root, "set", "fx-1", "urgency", "major")
	if refused.code != 2 {
		t.Fatalf("naming a field a card does not record exited %d, wanted 2: %s", refused.code, refused.errw)
	}
	if name := refusalNameOf(refused.errw); name != contract.UnknownField {
		t.Errorf("the refusal name is %s, wanted %s", name, contract.UnknownField)
	}
	if !strings.Contains(refused.errw, "The fields of card are: "+strings.Join(bench.FieldsOf(bench.KindCard), ", ")+".") {
		t.Errorf("the sentence does not carry the fields a card records:\n%s", refused.errw)
	}
	// The sentence lists the resolved kind's own set and no other kind's, so
	// a reader who typed an item's field at a card is told what a card
	// records rather than what any entity records.
	for _, foreign := range bench.FieldsOf(bench.KindItem) {
		if _, shared := bench.FieldOf(bench.KindCard, foreign); shared {
			continue
		}
		if strings.Contains(refused.errw, foreign) {
			t.Errorf("the sentence names %s, which belongs to an item rather than to a card:\n%s", foreign, refused.errw)
		}
	}
	if strings.Contains(refused.errw, "a query may name") {
		t.Errorf("the card reader got the sentence written for the query:\n%s", refused.errw)
	}
	if strings.Contains(refused.errw, ">=") {
		t.Errorf("the ordered-operator clause reached the card reader:\n%s", refused.errw)
	}
	if !strings.Contains(refused.errw, "dinah help set") {
		t.Errorf("the next step does not point at this command's own help page:\n%s", refused.errw)
	}
	// No listing table: every line after the first is a table row, and this
	// refusal draws none.
	if lines := strings.Split(strings.TrimSuffix(refused.errw, "\n"), "\n"); len(lines) != 1 {
		t.Errorf("the refusal drew %d lines, and it carries no listing:\n%s", len(lines), refused.errw)
	}
}

// TestTheQueryKeepsItsOwnUnknownFieldRendering asserts dinah-193 AC-22 in
// every catalog the binary ships. Gating the ordered-operator clause on the
// query command alters no existing rendering, so the query still gets the
// clause, the shared sentence and the next step pointing at its own guide.
func TestTheQueryKeepsItsOwnUnknownFieldRendering(t *testing.T) {
	root := newBenchFromDefinition(t, bothAxesDefinition)
	for _, tag := range msg.Tags() {
		catalog := msg.For(tag)
		refused := runCLI(t, root, "query", "urgency:major", "--lang", tag)
		if refused.code != 2 {
			t.Fatalf("%s: a query naming no field exited %d: %s", tag, refused.code, refused.errw)
		}
		if name := refusalNameOf(refused.errw); name != contract.UnknownField {
			t.Errorf("%s: the refusal name is %s, wanted %s", tag, name, contract.UnknownField)
		}
		for _, key := range []string{"refusal.dinah.unknown-field.ordered", "refusal.dinah.unknown-field.next"} {
			rendered := strings.TrimSpace(catalog.T(key, "instantField", verb.FieldAt))
			if !strings.Contains(refused.errw, rendered) {
				t.Errorf("%s: the query rendering lost %s:\n%s", tag, key, refused.errw)
			}
		}
		if strings.Contains(refused.errw, strings.TrimSpace(catalog.T("refusal.dinah.unknown-field.card.next"))) {
			t.Errorf("%s: the query reader got the card variant's next step:\n%s", tag, refused.errw)
		}
	}
}

// TestARefusedFilingCreatesNoCardDirectory asserts dinah-193 AC-14: the two
// add flags refuse in the words the equivalent card set refuses in, and the
// refused filing leaves nothing on disk.
func TestARefusedFilingCreatesNoCardDirectory(t *testing.T) {
	root := newBenchFromDefinition(t, bothAxesDefinition)
	if got := runCLI(t, root, "add", "a card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	anchor := soleBenchDir(t, root)
	before := bench.ListIDs(filepath.Join(anchor, bench.CardsDir))
	for _, axis := range []string{"severity", "priority"} {
		filing := runCLI(t, root, "add", "--"+axis, "urgent", "a card filed with a level nobody declared")
		if filing.code != 2 {
			t.Fatalf("add --%s urgent exited %d, wanted 2: %s", axis, filing.code, filing.errw)
		}
		writing := runCLI(t, root, "set", "fx-1", axis, "urgent")
		if writing.code != 2 {
			t.Fatalf("card set %s urgent exited %d, wanted 2: %s", axis, writing.code, writing.errw)
		}
		if filing.errw != writing.errw {
			t.Errorf("the two paths refuse in different words:\nadd:\n%s\ncard set:\n%s", filing.errw, writing.errw)
		}
	}
	if after := bench.ListIDs(filepath.Join(anchor, bench.CardsDir)); len(after) != len(before) {
		t.Errorf("a refused filing left %d card directories where there were %d", len(after), len(before))
	}
}

// TestTheRefusalOrderIsObservable asserts dinah-193 AC-15: a card that does
// not exist is reported as such even when the field and the level are both
// wrong, and an unknown field is reported as such even on a workbench
// declaring neither axis.
func TestTheRefusalOrderIsObservable(t *testing.T) {
	root := newBenchFromDefinition(t, bothAxesDefinition)
	missing := runCLI(t, root, "set", "fx-99", "urgency", "urgent")
	if name := refusalNameOf(missing.errw); name != contract.UnknownCard {
		t.Errorf("a card that does not exist reported %s, and row 1 runs before rows 2 and 4", name)
	}
	bare := newBench(t)
	if got := runCLI(t, bare, "add", "a card on a workbench declaring neither axis"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	field := runCLI(t, bare, "set", "fx-1", "urgency", "urgent")
	if name := refusalNameOf(field.errw); name != contract.UnknownField {
		t.Errorf("an unknown field reported %s on a workbench declaring neither axis, and row 2 runs before row 3", name)
	}
}

// TestAStoredLevelNobodyDeclaresIsToleratedAndReported asserts dinah-193
// AC-16. Every read tolerates it, because the write path is the only place a
// level is validated, and check is where the format already puts a stored
// value that resolves to nothing.
func TestAStoredLevelNobodyDeclaresIsToleratedAndReported(t *testing.T) {
	root := newBenchFromDefinition(t, bothAxesDefinition)
	if got := runCLI(t, root, "add", "a card somebody hand-edited"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	handWrite(t, root, "fx-1", "severity: urgent")
	for _, argv := range [][]string{{"ls"}, {"show", "fx-1"}, {"get", "fx-1", "severity"}} {
		if got := runCLI(t, root, argv...); got.code != 0 {
			t.Errorf("%v refused a card carrying an undeclared level: %d %s", argv, got.code, got.errw)
		}
	}
	if got := runCLI(t, root, "get", "fx-1", "severity"); got.out != "urgent\n" {
		t.Errorf("the stored level read back as %q", got.out)
	}
	checked := runCLI(t, root, "check")
	if checked.code != contract.ExitCodeForRead(contract.ReadFindings) {
		t.Fatalf("check exited %d over a defect, wanted %d: %s", checked.code, contract.ExitCodeForRead(contract.ReadFindings), checked.out)
	}
	sentence := msg.For(msg.Base).T(bench.FindingUnknownLevel, "detail", "severity urgent")
	if strings.Count(checked.out, sentence) != 1 {
		t.Errorf("check reported %d findings reading %q:\n%s", strings.Count(checked.out, sentence), sentence, checked.out)
	}
	if !strings.Contains(checked.out, "card.md") {
		t.Errorf("the finding does not name the card's own file:\n%s", checked.out)
	}
}

// TestAStaleLevelStaysClearableWhereTheDeclarationWent asserts dinah-193
// AC-23. Rows 3 and 4 run only where a value is present, so the one workbench
// where somebody wants to clear a stale level is not the one workbench where
// clearing is refused.
func TestAStaleLevelStaysClearableWhereTheDeclarationWent(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "a card carrying a level from an earlier declaration"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	handWrite(t, root, "fx-1", "severity: urgent")
	cleared := runCLI(t, root, "set", "fx-1", "severity")
	if cleared.code != 0 {
		t.Fatalf("clearing on a workbench declaring no severity set exited %d: %s", cleared.code, cleared.errw)
	}
	if written := anchorText(t, root, "fx-1"); strings.Contains(written, "severity") {
		t.Errorf("the clear did not remove the key:\n%s", written)
	}
	if updates := updatesOf(cardEvents(t, root, "fx-1")); len(updates) != 1 {
		t.Errorf("the clear journalled %d card_updated lines, wanted one", len(updates))
	}
	refused := runCLI(t, root, "set", "fx-1", "severity", "major")
	if refused.code != 2 || refusalNameOf(refused.errw) != contract.NoLevels {
		t.Errorf("a write carrying a value on the same workbench answered %d %q, wanted dinah.no-levels", refused.code, refused.errw)
	}
}

// handWrite adds one frontmatter line to a card's anchor, which is how a level
// an earlier declaration admitted gets onto a card no command would put it on.
func handWrite(t *testing.T, root, ref, line string) {
	t.Helper()
	got := runCLI(t, root, "path", ref)
	if got.code != 0 {
		t.Fatalf("path %s: %d %s", ref, got.code, got.errw)
	}
	path := strings.TrimSpace(got.out)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	edited := strings.Replace(string(data), "state: ready\n", "state: ready\n"+line+"\n", 1)
	if edited == string(data) {
		t.Fatalf("the anchor carries no state line to write under:\n%s", data)
	}
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatalf("write the anchor: %v", err)
	}
}

// ratifiedSetHelp is the block dinah help set prints, which is the page the
// retired card command's own page became. It is quoted here rather than
// derived, because a page compared against the declaration it is rendered from
// agrees with itself while both drift from the catalog, and a fixture nothing
// derives is what notices a row arriving in the runtime list.
//
// The block is drawn against bothAxesDefinition at eighty columns, so the --at
// row names that workbench's own two columns. The field row names no
// workbench's vocabulary, because the argument publishes the union over every
// kind rather than anything one workbench declares, so that row and every
// other line read the same wherever the command is run.
const ratifiedSetHelp = `set <ref> <field> [value|-] [--at <column>] [--note <text>] [--yes]

Write one field of any entity of this workbench

What you may write:
  As you write it  What it is
  ---------------  -------------------------------------------------------------
  <ref>            the entity you are writing, written as its reference: this
                   workbench as ` + "`" + `workbench` + "`" + ` or ` + "`" + `.` + "`" + `, a column, a card, or
                   something below a card such as wb-1/comments/1
  <field>          which field you are writing; which names are legal depends on
                   the kind the reference resolves to (one of: body, capacity,
                   column, description, filename, instructions, kind, note,
                   notes, operator, owner, priority, severity, slug, state,
                   status, text, tier, title)
  [value|-]        what to store in it; write a single dash to read it from
                   standard input, and leave it out to clear a field that may be
                   cleared
  [--at <column>]  the column a tier write applies to, instead of the card's own
                   baseline; a card's tier is the one field that takes it (one
                   of: intake, done)
  [--note <text>]  the note the verb behind a ` + "`" + `state` + "`" + ` write records; ` + "`" + `state` + "`" + ` is
                   the one field that takes it
  [--yes]          confirm the act, which Dinah does not carry out without it

What can go wrong, in the order each is checked:
  Order  What can go wrong                                Refusal
  -----  -----------------------------------------------  -------------------
  1      this workbench designates an operator            no-operator
  2      the reference resolves to one entity             dinah.unknown-path
  3      the field is one that kind records               dinah.unknown-field
  4      the value is present, and one line unless prose  malformed
  5      the field's own guard admits the value           dinah.unknown-level
  6      the request names an owner                       no-owner
  7      that owner is the operator, where the kind asks  not-operator
  8      a slug change carries the confirmation flag      dinah.unconfirmed

For more, run ` + "`" + `dinah guide references` + "`" + `.

Exit codes: 0 ok, 2 refused, 3 stale, 4 unreachable.
`

// TestTheSetHelpPageIsTheBlockTheOperatorApproved asserts the page dinah help
// set prints: the syntax line, one row per argument, and the checks in the
// order the runtime evaluates them. It is the page the retired card command's
// own page became, and it is held the same way that one was.
//
// The refusal half compares the printed table against verb.Checks("card")
// index for index rather than searching the page for a hand-written list of
// rows, and dinah-413 rewrote it for a reason worth keeping in view. The
// earlier form held five {check text, refusal} pairs and asked only that each
// appeared somewhere on the page at an index no earlier than the last. It
// never counted the printed rows and never asked that the table held nothing
// else, so a row added to the list passed unremarked. That is how the three
// refusals dinah-408's --at flag can raise stood missing from an
// operator-approved block for a whole card.
//
// Keying it to verb.Checks rather than to a second hand-written list is the
// other half of the rewrite. The runtime list is what the page is rendered
// from, so a copy of it here would be a copy of the thing under test, and the
// two would go on agreeing with each other while both drifted from the
// catalog the page actually prints.
func TestTheSetHelpPageIsTheBlockTheOperatorApproved(t *testing.T) {
	root := newBenchFromDefinition(t, bothAxesDefinition)
	t.Setenv("COLUMNS", "80")
	got := runCLI(t, root, "help", "set")
	if got.code != 0 {
		t.Fatalf("help set: %d %s", got.code, got.errw)
	}
	// The comparison runs over the page's words rather than its lines. An
	// argument's meaning wraps under its own column at this window, which the
	// <ref> row and the <field> row both do, and a wrap is allowed to change
	// nothing about the words themselves.
	flat := flattenWords(got.out)
	for _, phrase := range []string{
		"set <ref> <field> [value|-] [--at <column>] [--note <text>] [--yes]",
		"<ref>",
		"<field>",
		"[value|-]",
		"[--at <column>]",
		"[--note <text>]",
		"[--yes]",
		"which field you are writing; which names are legal depends on the kind",
		"write a single dash to read it from standard input",
		"the column a tier write applies to, instead of the card's own baseline",
	} {
		if !strings.Contains(flat, phrase) {
			t.Errorf("the page does not carry %q:\n%s", phrase, got.out)
		}
	}
	// The block itself, byte for byte, which is the assertion that records
	// the approval. It is what notices a row added to the runtime list: the
	// index-exact comparison below reads the printed table and the list it is
	// rendered from, so an addition arrives on both sides of it at once and
	// passes, which is the same blindness in a new place. A fixture ratified
	// on its own is what catches an addition, because nothing in the tree
	// derives it. TestWorkbenchHelpIsTheBlockTheOperatorApproved holds the
	// workbench page the same way.
	if got.out != ratifiedSetHelp {
		t.Errorf("the emitted block differs from the one the operator approved:\n%s", diffLines(ratifiedSetHelp, got.out))
	}

	checks := verb.Checks("set")
	catalog := msg.For(msg.Base)
	rows := parseRefusalTable(t, got.out)
	if len(rows) != len(checks) {
		t.Fatalf("the page draws %d refusal rows, wanted the %d of verb.Checks(\"set\"):\n%s", len(rows), len(checks), got.out)
	}
	for i, check := range checks {
		// A key the catalog does not carry renders as the key itself, on the
		// page and in the comparison alike, so the two would agree about a
		// row that says nothing to a reader. The entry is checked for rather
		// than inferred from the render.
		if _, carried := msg.BaseEntry(check.Key); !carried {
			t.Errorf("row %d names the catalog key %s, which no entry carries", i+1, check.Key)
		}
		want := catalog.T(check.Key)
		if rows[i].order != i+1 || rows[i].check != want || rows[i].refusal != check.Refusal {
			t.Errorf("row %d: wanted order %d, check %q, refusal %s; got order %d, check %q, refusal %s",
				i+1, i+1, want, check.Refusal, rows[i].order, rows[i].check, rows[i].refusal)
		}
	}
}

// refusalRow is one parsed line of a help page's refusal table.
type refusalRow struct {
	order          int
	check, refusal string
}

// parseRefusalTable reads the order, check and refusal columns back out of a
// rendered help page, one struct per printed row.
//
// It splits on the table's own column boundaries, which the rule line under
// the headings draws as runs of dashes, rather than on runs of whitespace. A
// check cell is a sentence carrying spaces of its own, so a whitespace split
// would report a different field count for every row and would join a check
// to the refusal beside it.
//
// The refusal table is the last table on a help page, so its rule is the last
// rule line, and the table ends at the first blank line under that rule. A
// page drawing no rule at all fails the test rather than reporting no rows,
// because no rows and no table are the same answer to a caller comparing a
// count, and only one of the two is a defect this helper should hide.
func parseRefusalTable(t *testing.T, page string) []refusalRow {
	t.Helper()
	lines := strings.Split(page, "\n")
	rule := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "---") {
			rule = i
		}
	}
	if rule < 0 {
		t.Fatalf("the page draws no table rule, so it carries no refusal table:\n%s", page)
	}
	spans := columnSpans(lines[rule])
	if len(spans) != 3 {
		t.Fatalf("the refusal table's rule draws %d columns, wanted 3:\n%s", len(spans), page)
	}
	var rows []refusalRow
	for _, line := range lines[rule+1:] {
		if strings.TrimSpace(line) == "" {
			break
		}
		fields := make([]string, 0, len(spans))
		for _, span := range spans {
			fields = append(fields, strings.TrimSpace(runeSlice(line, span[0], span[1])))
		}
		order, err := strconv.Atoi(fields[0])
		if err != nil {
			t.Fatalf("the refusal table draws %q where a row number belongs:\n%s", fields[0], page)
		}
		rows = append(rows, refusalRow{order: order, check: fields[1], refusal: fields[2]})
	}
	return rows
}

// columnSpans reads a table's rule line into one half-open span per column.
// A run of dashes is a column and the gutter between two runs is not, so the
// spans sit exactly where the renderer put the columns.
//
// The last span runs to the end of the line rather than to the end of its own
// dashes, so a cell drawn wider than its rule is read whole rather than
// truncated into a value that still looks like a real one.
func columnSpans(rule string) [][2]int {
	var spans [][2]int
	start := -1
	for i, r := range []rune(rule) {
		if r == '-' {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			spans = append(spans, [2]int{start, i})
			start = -1
		}
	}
	if start >= 0 {
		spans = append(spans, [2]int{start, len([]rune(rule))})
	}
	if len(spans) > 0 {
		spans[len(spans)-1][1] = -1
	}
	return spans
}

// runeSlice takes the runes of line between two column positions, tolerating
// a line shorter than the span and reading a negative end as the line's end.
func runeSlice(line string, from, to int) string {
	runes := []rune(line)
	if from > len(runes) {
		return ""
	}
	if to < 0 || to > len(runes) {
		to = len(runes)
	}
	return string(runes[from:to])
}

// TestShowPrintsIndependentlyConditionalSeverityAndPriorityLines asserts
// dinah-194 AC-3: renderCard prints a severity line and a priority line
// directly under the card's summary line, each independently conditional on
// the field being non-empty, and ahead of the holder and blocked lines.
func TestShowPrintsIndependentlyConditionalSeverityAndPriorityLines(t *testing.T) {
	root := newBenchFromDefinition(t, bothAxesDefinition)
	if got := runCLI(t, root, "add", "a card carrying neither level"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "add", "--severity", "major", "a card carrying only a severity"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "add", "--priority", "now", "a card carrying only a priority"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "add", "--severity", "critical", "--priority", "soon", "a card carrying both levels"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}

	neither := runCLI(t, root, "show", "fx-1")
	if strings.Contains(neither.out, "severity:") || strings.Contains(neither.out, "priority:") {
		t.Errorf("a card carrying neither level printed a severity or priority line:\n%s", neither.out)
	}

	severityOnly := runCLI(t, root, "show", "fx-2")
	if !strings.Contains(severityOnly.out, "\n  severity: major\n") {
		t.Errorf("a card carrying only a severity did not print its severity line:\n%s", severityOnly.out)
	}
	if strings.Contains(severityOnly.out, "priority:") {
		t.Errorf("a card carrying no priority printed a priority line:\n%s", severityOnly.out)
	}

	priorityOnly := runCLI(t, root, "show", "fx-3")
	if !strings.Contains(priorityOnly.out, "\n  priority: now\n") {
		t.Errorf("a card carrying only a priority did not print its priority line:\n%s", priorityOnly.out)
	}
	if strings.Contains(priorityOnly.out, "severity:") {
		t.Errorf("a card carrying no severity printed a severity line:\n%s", priorityOnly.out)
	}

	both := runCLI(t, root, "show", "fx-4")
	wantBoth := "a card carrying both levels  [Intake / ready]\n  severity: critical\n  priority: soon\n"
	if !strings.HasSuffix(strings.TrimPrefix(both.out, "fx-4  "), wantBoth) {
		t.Errorf("a card carrying both levels did not print severity then priority directly under the summary line:\n%s", both.out)
	}

	// Both plus holder plus blocked: the summary line, then severity, then
	// priority, then holder, then blocked, in that order. block() itself
	// clears a card's holder (mutate.go), so this combination is not
	// reachable through the CLI's own verbs; renderCard is exercised
	// directly against a CardView carrying all four fields instead, which is
	// exactly what the UX sketch's own worked example draws.
	buf := &bytes.Buffer{}
	s := &session{out: buf, r: msg.For(msg.Base)}
	s.renderCard(&verb.CardView{
		Ref:         "fx-4",
		Title:       "a card carrying both levels",
		ColumnTitle: "Intake",
		State:       "active",
		Severity:    "critical",
		Priority:    "soon",
		Holder:      "paul",
		BlockReason: "waiting on a decision",
	})
	fullOut := buf.String()
	severityAt := strings.Index(fullOut, "severity:")
	priorityAt := strings.Index(fullOut, "priority:")
	holderAt := strings.Index(fullOut, "held by")
	blockedAt := strings.Index(fullOut, "blocked:")
	if severityAt < 0 || priorityAt < 0 || holderAt < 0 || blockedAt < 0 {
		t.Fatalf("a held, blocked card carrying both levels did not print all four lines:\n%s", fullOut)
	}
	if !(severityAt < priorityAt && priorityAt < holderAt && holderAt < blockedAt) {
		t.Errorf("the four lines are not in severity, priority, holder, blocked order:\n%s", fullOut)
	}
}

// TestLsGainsSeverityAndPriorityColumnsBetweenStandingAndTitle asserts
// dinah-194 AC-4: dinah ls draws a Severity column and a Priority column, in
// that order, between Standing and Title, against a listing carrying both,
// one, and neither axis on its cards.
func TestLsGainsSeverityAndPriorityColumnsBetweenStandingAndTitle(t *testing.T) {
	root := newBenchFromDefinition(t, bothAxesDefinition)
	if got := runCLI(t, root, "add", "--severity", "major", "--priority", "now", "a workbench states its default lane"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "add", "a card carrying neither level"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "add", "--severity", "minor", "a card carrying only a severity"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	got := runCLI(t, root, "ls", "intake")
	if got.code != 0 {
		t.Fatalf("ls: %d %s", got.code, got.errw)
	}
	heading := strings.SplitN(got.out, "\n", 2)[0]
	standingAt := strings.Index(heading, "Standing")
	severityAt := strings.Index(heading, "Severity")
	priorityAt := strings.Index(heading, "Priority")
	titleAt := strings.Index(heading, "Title")
	if standingAt < 0 || severityAt < 0 || priorityAt < 0 || titleAt < 0 {
		t.Fatalf("the heading does not carry all four columns:\n%s", heading)
	}
	if !(standingAt < severityAt && severityAt < priorityAt && priorityAt < titleAt) {
		t.Errorf("the columns are not in Standing, Severity, Priority, Title order:\n%s", heading)
	}

	// The heading proves the columns exist and are ordered; it does not prove
	// a value landed in its own column rather than merely somewhere in the
	// row. Slice each data row at the heading's own offsets and check the
	// Severity and Priority cells by identity, not by scanning the whole
	// listing for a substring that could equally have printed in Title.
	lines := strings.Split(strings.TrimRight(got.out, "\n"), "\n")
	rows := lines[2:] // heading, rule, then one line per card
	if len(rows) != 3 {
		t.Fatalf("the listing carries %d data rows, wanted 3:\n%s", len(rows), got.out)
	}
	cellAt := func(row string, from, to int) string {
		if from >= len(row) {
			return ""
		}
		if to > len(row) {
			to = len(row)
		}
		return strings.TrimSpace(row[from:to])
	}
	cases := []struct {
		row                string
		severity, priority string
	}{
		{rows[0], "major", "now"},
		{rows[1], "", ""},
		{rows[2], "minor", ""},
	}
	for _, c := range cases {
		if got := cellAt(c.row, severityAt, priorityAt); got != c.severity {
			t.Errorf("row %q: Severity cell is %q, wanted %q", c.row, got, c.severity)
		}
		if got := cellAt(c.row, priorityAt, titleAt); got != c.priority {
			t.Errorf("row %q: Priority cell is %q, wanted %q", c.row, got, c.priority)
		}
	}
}

// TestLsDropsAnAxisColumnNobodyPopulates asserts dinah-194 AC-5: on a listing
// where no visible card carries a value for one axis, that axis's column,
// heading included, is dropped by the table layer's existing
// withoutEmptyColumns pass, with no new special-case code needed to produce
// it.
func TestLsDropsAnAxisColumnNobodyPopulates(t *testing.T) {
	root := newBenchFromDefinition(t, bothAxesDefinition)
	if got := runCLI(t, root, "add", "--severity", "minor", "a card carrying only a severity"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "add", "--severity", "major", "another card carrying only a severity"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	got := runCLI(t, root, "ls", "intake")
	if got.code != 0 {
		t.Fatalf("ls: %d %s", got.code, got.errw)
	}
	heading := strings.SplitN(got.out, "\n", 2)[0]
	if strings.Contains(heading, "Priority") {
		t.Errorf("a listing where no card carries a priority still drew the Priority column:\n%s", heading)
	}
	if !strings.Contains(heading, "Severity") {
		t.Errorf("a listing where a card carries a severity dropped the Severity column:\n%s", heading)
	}
}

// TestUndeclaredLevelDisplaysUnmarkedOnAllThreeSurfaces asserts dinah-194
// AC-6: a card whose stored severity names a level the workbench's current
// declaration does not carry is shown exactly as stored, unmarked, on the ls
// column, the show line and the machine form, carrying forward dinah-193
// D-2's reader posture with no new validation on any read path.
func TestUndeclaredLevelDisplaysUnmarkedOnAllThreeSurfaces(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "a card whose severity nobody declares any more"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	handWrite(t, root, "fx-1", "severity: urgent")

	listing := runCLI(t, root, "ls", "intake")
	if listing.code != 0 {
		t.Fatalf("ls: %d %s", listing.code, listing.errw)
	}
	if !strings.Contains(listing.out, "urgent") {
		t.Errorf("the listing does not show the undeclared severity as stored:\n%s", listing.out)
	}

	shown := runCLI(t, root, "show", "fx-1")
	if shown.code != 0 {
		t.Fatalf("show: %d %s", shown.code, shown.errw)
	}
	if !strings.Contains(shown.out, "\n  severity: urgent\n") {
		t.Errorf("show does not print the undeclared severity as stored:\n%s", shown.out)
	}

	asJSON := runCLI(t, root, "show", "fx-1", "--json")
	if asJSON.code != 0 {
		t.Fatalf("show --json: %d %s", asJSON.code, asJSON.errw)
	}
	var decoded struct {
		Card struct {
			Severity string `json:"severity"`
		} `json:"card"`
	}
	if err := json.Unmarshal([]byte(asJSON.out), &decoded); err != nil {
		t.Fatalf("decode: %v\n%s", err, asJSON.out)
	}
	if decoded.Card.Severity != "urgent" {
		t.Errorf("the machine form carries severity %q, wanted the undeclared value as stored", decoded.Card.Severity)
	}
}

// TestLevelNamesNeverPassThroughTheTokenCatalog asserts dinah-194 AC-7:
// severity and priority values render as the workbench declared them,
// bypassing s.token() entirely, the same treatment workstreamsCell already
// gives workbench-declared names. A level literally named "active" is used
// because the German catalog translates the fixed token "active" to "aktiv";
// if the display path ran the value through s.token(), the German rendering
// would read "aktiv" instead of "active".
func TestLevelNamesNeverPassThroughTheTokenCatalog(t *testing.T) {
	const collidingDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Colliding level",
  "levels": { "severity": ["active"] },
  "columns": [
    { "id": "b00000000001", "title": "Intake", "kind": "intake" },
    { "id": "b00000000002", "title": "Done", "kind": "done" }
  ]
}`
	root := newBenchFromDefinition(t, collidingDefinition)
	if got := runCLI(t, root, "add", "--severity", "active", "a card whose severity collides with a fixed token"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	for _, tag := range []string{"en", "de"} {
		shown := runCLI(t, root, "show", "fx-1", "--lang", tag)
		if shown.code != 0 {
			t.Fatalf("%s: show: %d %s", tag, shown.code, shown.errw)
		}
		if !strings.Contains(shown.out, "active") {
			t.Errorf("%s: show did not print the level name unchanged:\n%s", tag, shown.out)
		}
		if strings.Contains(shown.out, "aktiv") {
			t.Errorf("%s: the level name reached the German token catalog:\n%s", tag, shown.out)
		}
		listing := runCLI(t, root, "ls", "intake", "--lang", tag)
		if listing.code != 0 {
			t.Fatalf("%s: ls: %d %s", tag, listing.code, listing.errw)
		}
		if !strings.Contains(listing.out, "active") {
			t.Errorf("%s: ls did not print the level name unchanged:\n%s", tag, listing.out)
		}
		if strings.Contains(listing.out, "aktiv") {
			t.Errorf("%s: the level name reached the German token catalog on ls:\n%s", tag, listing.out)
		}
	}
}

// TestAQueryFiltersBySeverityAndPriority asserts dinah-195 AC-1 through AC-6:
// a query admits severity and priority as equality-only fields, tolerates a
// drifted value the same way it tolerates a drifted workstream, refuses an
// unknown value naming the declared members in declaration order rather than
// alphabetically, answers correctly on an axis the workbench does not declare
// at all, and treats the explicit empty value as a request for absence.
func TestAQueryFiltersBySeverityAndPriority(t *testing.T) {
	root := newBenchFromDefinition(t, bothAxesDefinition)
	if got := runCLI(t, root, "add", "--severity", "major", "a major card"); got.code != 0 {
		t.Fatalf("add --severity major: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "add", "--severity", "minor", "a minor card"); got.code != 0 {
		t.Fatalf("add --severity minor: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "add", "--priority", "now", "an urgent card"); got.code != 0 {
		t.Fatalf("add --priority now: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "add", "a plain card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}

	// AC-1: a severity query returns every live card whose stored severity
	// matches, and refuses nothing.
	major := queryRefs(t, root, "severity:major")
	if len(major) != 1 || major[0] != "fx-1" {
		t.Errorf("severity:major selected %v, wanted [fx-1]", major)
	}

	// AC-2: the same holds for priority, on the same check path.
	urgent := queryRefs(t, root, "priority:now")
	if len(urgent) != 1 || urgent[0] != "fx-3" {
		t.Errorf("priority:now selected %v, wanted [fx-3]", urgent)
	}

	// AC-3: a value no declaration carries but a live card does is still
	// findable, the same drift tolerance workstreamRoster gives workstream.
	handWrite(t, root, "fx-4", "severity: urgent")
	drifted := queryRefs(t, root, "severity:urgent")
	if len(drifted) != 1 || drifted[0] != "fx-4" {
		t.Errorf("severity:urgent (drifted) selected %v, wanted [fx-4]", drifted)
	}

	// AC-4: a value nothing carries refuses, and the declared members are
	// listed in declaration order, not sorted.
	refused := runCLI(t, root, "query", "severity:critical-plus")
	if refused.code != 2 {
		t.Fatalf("query severity:critical-plus exited %d, wanted 2: %s", refused.code, refused.errw)
	}
	if name := refusalNameOf(refused.errw); name != contract.UnknownValue {
		t.Errorf("the refusal name is %s, wanted %s", name, contract.UnknownValue)
	}
	if !strings.Contains(refused.errw, "trivial, minor, major, critical") {
		t.Errorf("the refusal does not list the declared members in declaration order:\n%s", refused.errw)
	}
	if strings.Contains(refused.errw, "critical, major") {
		t.Errorf("the refusal lists the declared members alphabetically rather than in declaration order:\n%s", refused.errw)
	}
	// The drifted value from AC-3 trails the declared members rather than
	// being interleaved among them.
	if !strings.Contains(refused.errw, "trivial, minor, major, critical, urgent") {
		t.Errorf("the drifted value does not trail the declared members:\n%s", refused.errw)
	}

	// AC-5: an axis the workbench does not declare at all still answers
	// through the same check rather than a different one. With no live card
	// carrying a value on that axis either, the roster is empty and any named
	// value refuses; the empty value still asks for absence and finds every
	// card, since none of them carries a priority at all.
	bare := newBenchFromDefinition(t, severityOnlyDefinition)
	if got := runCLI(t, bare, "add", "a card on a severity-only workbench"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	refusedBare := runCLI(t, bare, "query", "priority:now")
	if refusedBare.code == 0 {
		t.Fatalf("query priority:now on a workbench declaring no priority succeeded")
	}
	if name := refusalNameOf(refusedBare.errw); name != contract.UnknownValue {
		t.Errorf("the refusal name on an undeclared axis is %s, wanted %s", name, contract.UnknownValue)
	}
	unsetBare := queryRefs(t, bare, `priority:""`)
	if len(unsetBare) != 1 || unsetBare[0] != "fx-1" {
		t.Errorf(`priority:"" on a workbench declaring no priority selected %v, wanted [fx-1]`, unsetBare)
	}

	// AC-6: the explicit empty value asks for absence, on both a declared
	// axis and one no card has ever set.
	unset := queryRefs(t, root, `severity:""`)
	want := map[string]bool{"fx-3": true}
	if len(unset) != len(want) {
		t.Errorf(`severity:"" selected %v, wanted the cards carrying no severity`, unset)
	}
	for _, ref := range unset {
		if !want[ref] {
			t.Errorf(`severity:"" selected %s, which carries a severity`, ref)
		}
	}
	unsetPriority := queryRefs(t, root, `priority:""`)
	wantPriority := map[string]bool{"fx-1": true, "fx-2": true, "fx-4": true}
	if len(unsetPriority) != len(wantPriority) {
		t.Errorf(`priority:"" selected %v, wanted the cards carrying no priority`, unsetPriority)
	}
	for _, ref := range unsetPriority {
		if !wantPriority[ref] {
			t.Errorf(`priority:"" selected %s, which carries a priority`, ref)
		}
	}
}

// queryRefs runs a query through the CLI and decodes the refs it selected,
// failing the test unless the query succeeded.
func queryRefs(t *testing.T, root, text string) []string {
	t.Helper()
	got := runCLI(t, root, "query", text, "--json")
	if got.code != 0 {
		t.Fatalf("query %q: %d %s", text, got.code, got.errw)
	}
	var document struct {
		Cards []struct {
			Ref string `json:"ref"`
		} `json:"cards"`
	}
	if err := json.Unmarshal([]byte(got.out), &document); err != nil {
		t.Fatalf("query %q: decode: %v\n%s", text, err, got.out)
	}
	refs := make([]string, 0, len(document.Cards))
	for _, card := range document.Cards {
		refs = append(refs, card.Ref)
	}
	return refs
}
