package main

import (
	"bytes"
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
	edited := strings.Replace(string(data), "state: ready\n", "state: ready\n"+line+"\n", 1)
	if edited == string(data) {
		t.Fatalf("the anchor carries no state line to write under:\n%s", data)
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
