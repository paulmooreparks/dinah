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
	"dinah/internal/verb"
)

// bothAxesDefinition declares the two level sets docs/design/format.md uses as
// its own example, which is what the Dinah board itself declares.
const bothAxesDefinition = `{
  "profile": "dinah-core/1.0",
  "title": "Levelled",
  "levels": { "severity": ["trivial", "minor", "major", "critical"], "priority": ["later", "soon", "next", "now"] },
  "states": [
    { "id": "b00000000001", "title": "Intake", "kind": "intake" },
    { "id": "b00000000002", "title": "Done", "kind": "done" }
  ]
}`

// severityOnlyDefinition is the workbench dinah-193 AC-26 is written against:
// one axis declared and the other not, which is an ordinary workbench rather
// than a degenerate one, and the case a single workbench-wide "does this
// declare any levels at all" gate would pass while breaking the format.
const severityOnlyDefinition = `{
  "profile": "dinah-core/1.0",
  "title": "Severity only",
  "levels": { "severity": ["trivial", "minor", "major", "critical"] },
  "states": [
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

// TestFilingACardWithBothLevelsWritesThePairUnderSubstate asserts dinah-193
// AC-5 and AC-6: the two flags land in the anchor in order under substate, and
// a filing that names neither leaves absence as absence rather than writing
// two empty values.
func TestFilingACardWithBothLevelsWritesThePairUnderSubstate(t *testing.T) {
	root := newBenchFromDefinition(t, bothAxesDefinition)
	if got := runCLI(t, root, "add", "--severity", "major", "--priority", "now", "a classified card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if written := anchorText(t, root, "fx-1"); !strings.Contains(written, "substate: ready\nseverity: major\npriority: now\n") {
		t.Errorf("the pair did not land under substate in order:\n%s", written)
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
	if got := runCLI(t, root, "card", "set", "fx-1", "severity", "major"); got.code != 0 {
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
	if got := runCLI(t, root, "card", "set", "fx-1", "severity", "major"); got.code != 0 {
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
	if got := runCLI(t, root, "card", "set", "fx-1", "severity"); got.code != 0 {
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
	stored := runCLI(t, root, "card", "get", "fx-1", "severity")
	if stored.code != 0 {
		t.Fatalf("card get: %d %s", stored.code, stored.errw)
	}
	if stored.out != "minor\n" {
		t.Errorf("the stored level printed as %q, wanted one line reading minor", stored.out)
	}
	absent := runCLI(t, root, "card", "get", "fx-1", "priority")
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
	refused := runCLI(t, root, "card", "set", "fx-1", "severity", "urgent")
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
	if got := runCLI(t, root, "card", "set", "fx-1", "severity", "major"); got.code != 0 {
		t.Fatalf("a severity write on a workbench that declares severity: %d %s", got.code, got.errw)
	}
	if written := anchorText(t, root, "fx-1"); !strings.Contains(written, "severity: major\n") {
		t.Errorf("the severity write did not land:\n%s", written)
	}
	refused := runCLI(t, root, "card", "set", "fx-1", "priority", "now")
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
	refused := runCLI(t, root, "card", "set", "fx-1", "urgency", "major")
	if refused.code != 2 {
		t.Fatalf("naming a field a card does not record exited %d, wanted 2: %s", refused.code, refused.errw)
	}
	if name := refusalNameOf(refused.errw); name != contract.UnknownField {
		t.Errorf("the refusal name is %s, wanted %s", name, contract.UnknownField)
	}
	if !strings.Contains(refused.errw, "The fields a card records are: severity, priority.") {
		t.Errorf("the sentence does not carry the fields a card records:\n%s", refused.errw)
	}
	if strings.Contains(refused.errw, "a query may name") {
		t.Errorf("the card reader got the sentence written for the query:\n%s", refused.errw)
	}
	if strings.Contains(refused.errw, ">=") {
		t.Errorf("the ordered-operator clause reached the card reader:\n%s", refused.errw)
	}
	if !strings.Contains(refused.errw, "dinah help card") {
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
		writing := runCLI(t, root, "card", "set", "fx-1", axis, "urgent")
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
	missing := runCLI(t, root, "card", "set", "fx-99", "urgency", "urgent")
	if name := refusalNameOf(missing.errw); name != contract.UnknownCard {
		t.Errorf("a card that does not exist reported %s, and row 1 runs before rows 2 and 4", name)
	}
	bare := newBench(t)
	if got := runCLI(t, bare, "add", "a card on a workbench declaring neither axis"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	field := runCLI(t, bare, "card", "set", "fx-1", "urgency", "urgent")
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
	for _, argv := range [][]string{{"ls"}, {"show", "fx-1"}, {"card", "get", "fx-1", "severity"}} {
		if got := runCLI(t, root, argv...); got.code != 0 {
			t.Errorf("%v refused a card carrying an undeclared level: %d %s", argv, got.code, got.errw)
		}
	}
	if got := runCLI(t, root, "card", "get", "fx-1", "severity"); got.out != "urgent\n" {
		t.Errorf("the stored level read back as %q", got.out)
	}
	checked := runCLI(t, root, "check")
	if checked.code != 2 {
		t.Fatalf("check exited %d over a defect, wanted 2: %s", checked.code, checked.out)
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
	cleared := runCLI(t, root, "card", "set", "fx-1", "severity")
	if cleared.code != 0 {
		t.Fatalf("clearing on a workbench declaring no severity set exited %d: %s", cleared.code, cleared.errw)
	}
	if written := anchorText(t, root, "fx-1"); strings.Contains(written, "severity") {
		t.Errorf("the clear did not remove the key:\n%s", written)
	}
	if updates := updatesOf(cardEvents(t, root, "fx-1")); len(updates) != 1 {
		t.Errorf("the clear journalled %d card_updated lines, wanted one", len(updates))
	}
	refused := runCLI(t, root, "card", "set", "fx-1", "severity", "major")
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
	edited := strings.Replace(string(data), "substate: ready\n", "substate: ready\n"+line+"\n", 1)
	if edited == string(data) {
		t.Fatalf("the anchor carries no substate line to write under:\n%s", data)
	}
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatalf("write the anchor: %v", err)
	}
}

// TestTheCardHelpPageIsTheBlockTheOperatorApproved asserts dinah-193 AC-18
// against the sketch's section 8: the syntax line, one row per argument, and
// the five checks in the order the runtime evaluates them.
func TestTheCardHelpPageIsTheBlockTheOperatorApproved(t *testing.T) {
	root := newBenchFromDefinition(t, bothAxesDefinition)
	t.Setenv("COLUMNS", "80")
	got := runCLI(t, root, "help", "card")
	if got.code != 0 {
		t.Fatalf("help card: %d %s", got.code, got.errw)
	}
	for _, phrase := range []string{
		"card <get|set> <card> <field> [value]",
		"<get|set>",
		"<card>",
		"<field>",
		"[value]",
		"which field you are reading or writing: severity or priority",
		"the level to write; leave it out to clear the field",
	} {
		if !strings.Contains(got.out, phrase) {
			t.Errorf("the page does not carry %q:\n%s", phrase, got.out)
		}
	}
	rows := []struct{ check, refusal string }{
		{"the reference names a card of this workbench", contract.UnknownCard},
		{"the field is one a card records", contract.UnknownField},
		{"the workbench declares levels for that field", contract.NoLevels},
		{"the value is a level that field declares", contract.UnknownLevel},
		{"the request names an owner", contract.NoOwner},
	}
	at := 0
	for i, row := range rows {
		found := strings.Index(got.out, row.check)
		if found < 0 {
			t.Errorf("the page does not carry the check %q:\n%s", row.check, got.out)
			continue
		}
		if found < at {
			t.Errorf("row %d is drawn out of the order the runtime evaluates:\n%s", i+1, got.out)
		}
		at = found
		if !strings.Contains(got.out[found:], row.refusal) {
			t.Errorf("the check %q is not paired with %s:\n%s", row.check, row.refusal, got.out)
		}
	}
}
