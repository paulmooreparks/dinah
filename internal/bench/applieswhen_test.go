package bench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The construction blocks of the dinah-590 specification, as a person types
// them into workbench.md: a level axis and a declared field gated on the same
// field in the same syntax.
const constructionLevels = `levels:
  severity:
    values: [cosmetic, functional, safety]
    applies_when:
      field: task.type
      is: [snag]
`

const constructionFields = `fields:
  task.type:
    type: string
    meaning: whether the task is our own crew's work, subcontracted, or a snag found on inspection
    on: [card]
  task.trade:
    type: string
    meaning: the trade the subcontractor brings
    on: [card]
    applies_when:
      field: task.type
      is: [subcontracted]
`

// The fixture card newFixture plants, which stores nothing and is where every
// case below starts.
const fixtureCardID = "c00000000001"

// conditionedFixture plants the registry fixture with the given blocks above
// its columns list, stamps it at the format given, and returns its root.
func conditionedFixture(t *testing.T, format int, blocks string) string {
	t.Helper()
	root := newFixture(t)
	path := filepath.Join(root, WorkbenchAnchor)
	text, err := ReadText(path)
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	stamped := strings.Replace(text, "format: "+strconv.Itoa(StorageFormat), "format: "+strconv.Itoa(format), 1)
	write(t, path, strings.Replace(stamped, "columns:\n", blocks+"columns:\n", 1))
	return root
}

// openConditioned opens a fixture root and fails the test where it will not
// open. It is not openFixture, which is the compatibility opener over a staged
// old store.
func openConditioned(t *testing.T, root string) *Bench {
	t.Helper()
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return opened
}

// fixtureCard loads the one card the fixture plants.
func fixtureCard(t *testing.T, b *Bench) *Card {
	t.Helper()
	card, err := b.LoadCardIn(b.CardsRoot(), fixtureCardID)
	if err != nil {
		t.Fatalf("load the fixture card: %v", err)
	}
	return card
}

// storeOnCard writes one declared field value or one level onto the fixture
// card by hand, which is how a case reaches a stored value without the write
// path's own guard deciding whether it may be there.
func storeOnCard(t *testing.T, root, slot, value string) {
	t.Helper()
	path := filepath.Join(root, CardsDir, fixtureCardID, CardAnchor)
	text, err := ReadText(path)
	if err != nil {
		t.Fatalf("read the card anchor: %v", err)
	}
	fm, body := ParseAnchor(text)
	if KnownLevelAxis(slot) {
		fm.Set(slot, value)
	} else {
		SetFieldValue(fm, slot, value)
	}
	write(t, path, fm.Render(body))
}

// findingsOf runs check and fails the test where check itself failed.
func findingsOf(t *testing.T, b *Bench) []Finding {
	t.Helper()
	findings, err := b.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	return findings
}

// findingWith reports whether a finding carrying the key and the detail is
// among those given.
func findingWith(findings []Finding, key, detail string) bool {
	for _, finding := range findings {
		if finding.Key == key && finding.Detail == detail {
			return true
		}
	}
	return false
}

// countOf is how many findings carry one key.
func countOf(findings []Finding, key string) int {
	n := 0
	for _, finding := range findings {
		if finding.Key == key {
			n++
		}
	}
	return n
}

// TestTheMappingFormReadsTheSameLevelsAsTheListForms asserts dinah-590
// specification section 3.3: an axis written in the mapping form declares the
// same members, hints and ranks as the flow and dashed forms of the same
// declaration, whether its values member takes the flow or the dashed
// spelling.
func TestTheMappingFormReadsTheSameLevelsAsTheListForms(t *testing.T) {
	const hint = "Something on site does not work as it should; put it right before handover."
	forms := map[string]string{
		"flow":           "levels:\n  severity: [cosmetic, functional, safety]\n",
		"dashed":         "levels:\n  severity:\n    - cosmetic\n    - functional: " + hint + "\n    - safety\n",
		"mapping flow":   "levels:\n  severity:\n    values: [cosmetic, functional, safety]\n",
		"mapping dashed": "levels:\n  severity:\n    values:\n      - cosmetic\n      - functional: " + hint + "\n      - safety\n",
	}
	want := []string{"cosmetic", "functional", "safety"}
	for name, block := range forms {
		levels := benchDeclaring(t, block).Levels("severity")
		if got := namesOf(levels); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("the %s form declares %v, wanted %v", name, got, want)
			continue
		}
		for i, level := range levels {
			if level.Rank != i {
				t.Errorf("the %s form ranks %s at %d, wanted %d", name, level.Name, level.Rank, i)
			}
		}
		hinted := strings.Contains(name, "dashed")
		if got := levels[1].Hint; (got == hint) != hinted {
			t.Errorf("the %s form carries the hint %q on functional", name, got)
		}
	}
	// A mapping-form axis with no values member declares no set, on the
	// terms a block with no parseable child already declares none.
	if levels := benchDeclaring(t, "levels:\n  severity:\n    applies_when:\n      field: task.type\n      is: [snag]\n").Levels("severity"); levels != nil {
		t.Errorf("a mapping-form axis without values declares %v", namesOf(levels))
	}
}

// malformedCase is one of the six reasons a condition cannot be used: the
// blocks that carry it, the slot it sits on, and the reason token check
// reports. In every case the slot is then admitted on a card storing nothing
// for the gate, which is the card the condition would have excluded.
type malformedCase struct {
	blocks string
	slot   string
	reason string
}

// assertMalformed runs one malformed case: the finding carries the slot and
// the reason, and the slot applies to the fixture card as though no condition
// were declared.
func assertMalformed(t *testing.T, c malformedCase) {
	t.Helper()
	root := conditionedFixture(t, StorageFormat, c.blocks)
	b := openConditioned(t, root)
	detail := c.slot + " (" + c.reason + ")"
	findings := findingsOf(t, b)
	if !findingWith(findings, FindingAppliesWhenMalformed, detail) {
		t.Errorf("check reports no %s with detail %q; findings: %+v", FindingAppliesWhenMalformed, detail, findings)
	}
	if n := countOf(findings, FindingAppliesWhenMalformed); n != 1 {
		t.Errorf("check reports %d malformed conditions, wanted exactly one", n)
	}
	if b.ConditionOn(c.slot) != nil {
		t.Errorf("the refused condition on %s is still held as usable", c.slot)
	}
	card := fixtureCard(t, b)
	if answer := b.Applicability(card, c.slot); !answer.Applies || answer.Gate != "" {
		t.Errorf("%s answers %+v on a card storing nothing for the gate, wanted it to apply as though no condition were declared", c.slot, answer)
	}
	for _, slot := range b.InapplicableSlots(card) {
		if slot == c.slot {
			t.Errorf("the card still lists %s as inapplicable after its condition was refused", slot)
		}
	}
}

// TestAConditionMissingItsIsMemberIsUnreadable asserts the unreadable reason
// of specification section 7.1 for a condition naming a gate and no values.
func TestAConditionMissingItsIsMemberIsUnreadable(t *testing.T) {
	assertMalformed(t, malformedCase{
		blocks: "levels:\n  severity:\n    values: [cosmetic, functional, safety]\n    applies_when:\n      field: task.type\n" + constructionFields,
		slot:   "severity", reason: ConditionUnreadable,
	})
}

// TestAConditionOnTierIsRefused asserts the tier reason: the claim gate's axis
// takes the mapping form and never a condition.
func TestAConditionOnTierIsRefused(t *testing.T) {
	assertMalformed(t, malformedCase{
		blocks: "levels:\n  tier:\n    values: [workhorse, frontier]\n    applies_when:\n      field: task.type\n      is: [snag]\n" + constructionFields,
		slot:   "tier", reason: ConditionOnTier,
	})
}

// TestAConditionedEntryReachingBeyondCardsIsRefused asserts the
// reaches-beyond-cards reason for an entry that names no on member and so
// reaches a column and the workbench as well.
func TestAConditionedEntryReachingBeyondCardsIsRefused(t *testing.T) {
	assertMalformed(t, malformedCase{
		blocks: "fields:\n  task.type:\n    type: string\n    meaning: what kind of task this is\n    on: [card]\n  task.trade:\n    type: string\n    meaning: the trade the subcontractor brings\n    applies_when:\n      field: task.type\n      is: [subcontracted]\n",
		slot:   "task.trade", reason: ConditionReachesBeyondCards,
	})
}

// TestAConditionNamingAnUndeclaredGateIsRefused asserts the gate-undeclared
// reason for a field member naming no key the workbench declares.
func TestAConditionNamingAnUndeclaredGateIsRefused(t *testing.T) {
	assertMalformed(t, malformedCase{
		blocks: "fields:\n  task.trade:\n    type: string\n    meaning: the trade the subcontractor brings\n    on: [card]\n    applies_when:\n      field: nobody.home\n      is: [subcontracted]\n",
		slot:   "task.trade", reason: ConditionGateUndeclared,
	})
}

// TestAConditionNamingAGateOffCardsIsRefused asserts the gate-off-cards reason
// for a gate declared on a column alone.
func TestAConditionNamingAGateOffCardsIsRefused(t *testing.T) {
	assertMalformed(t, malformedCase{
		blocks: "fields:\n  task.type:\n    type: string\n    meaning: what kind of task this is\n    on: [column]\n  task.trade:\n    type: string\n    meaning: the trade the subcontractor brings\n    on: [card]\n    applies_when:\n      field: task.type\n      is: [subcontracted]\n",
		slot:   "task.trade", reason: ConditionGateOffCards,
	})
}

// TestAConditionNamingAConditionedGateIsRefused asserts the gate-conditioned
// reason, in the case the specification calls out: a slot naming itself.
func TestAConditionNamingAConditionedGateIsRefused(t *testing.T) {
	assertMalformed(t, malformedCase{
		blocks: "fields:\n  task.trade:\n    type: string\n    meaning: the trade the subcontractor brings\n    on: [card]\n    applies_when:\n      field: task.trade\n      is: [electrical]\n",
		slot:   "task.trade", reason: ConditionGateConditioned,
	})
}

// TestAnUnmatchableIsValueIsReportedAndAWellTypedOneIsNot asserts the second
// finding of specification section 7.1: a value the gate's declared type can
// never hold is reported, and the condition otherwise stands.
func TestAnUnmatchableIsValueIsReportedAndAWellTypedOneIsNot(t *testing.T) {
	const gate = "fields:\n  vendor.confirmed:\n    type: boolean\n    meaning: whether the vendor has confirmed\n    on: [card]\n  vendor.deposit-required:\n    type: boolean\n    meaning: whether the vendor asks for a deposit\n    on: [card]\n    applies_when:\n      field: vendor.confirmed\n      is: [%s]\n"
	unmatchable := openConditioned(t, conditionedFixture(t, StorageFormat, strings.Replace(gate, "%s", "maybe", 1)))
	findings := findingsOf(t, unmatchable)
	if !findingWith(findings, FindingAppliesWhenValueUnmatchable, "vendor.deposit-required (maybe)") {
		t.Errorf("check reports no %s for a boolean gate asked for maybe; findings: %+v", FindingAppliesWhenValueUnmatchable, findings)
	}
	if unmatchable.ConditionOn("vendor.deposit-required") == nil {
		t.Error("an unmatchable value undeclared the whole condition, and the rest of it should stand")
	}
	typed := openConditioned(t, conditionedFixture(t, StorageFormat, strings.Replace(gate, "%s", "true", 1)))
	if n := countOf(findingsOf(t, typed), FindingAppliesWhenValueUnmatchable); n != 0 {
		t.Errorf("check reports %d unmatchable values for is: [true] on a boolean gate", n)
	}
}

// TestApplicabilityAnswersTheFourStatesAndOrdersTheSlots asserts specification
// section 4 and the order section 6.1 fixes: one card per state, and the
// inapplicable slots listed as severity, then priority, then declared keys.
func TestApplicabilityAnswersTheFourStatesAndOrdersTheSlots(t *testing.T) {
	blocks := "levels:\n  severity:\n    values: [cosmetic, functional, safety]\n    applies_when:\n      field: task.type\n      is: [snag]\n  priority:\n    values: [later, now]\n    applies_when:\n      field: task.type\n      is: [snag]\n" + constructionFields
	cases := []struct {
		name      string
		taskType  string
		severity  string
		applies   bool
		gateValue string
		orphaned  bool
	}{
		{name: "set", taskType: "snag", severity: "safety", applies: true, gateValue: "snag"},
		{name: "unset", taskType: "snag", applies: true, gateValue: "snag"},
		{name: "inapplicable", taskType: "own", gateValue: "own"},
		{name: "orphaned", taskType: "own", severity: "safety", gateValue: "own", orphaned: true},
	}
	for _, c := range cases {
		root := conditionedFixture(t, StorageFormat, blocks)
		if c.taskType != "" {
			storeOnCard(t, root, "task.type", c.taskType)
		}
		if c.severity != "" {
			storeOnCard(t, root, "severity", c.severity)
		}
		b := openConditioned(t, root)
		card := fixtureCard(t, b)
		answer := b.Applicability(card, "severity")
		if answer.Applies != c.applies || answer.Gate != "task.type" || answer.GateValue != c.gateValue {
			t.Errorf("%s: severity answers %+v", c.name, answer)
		}
		orphaned := b.OrphanedValues(card)
		if (len(orphaned) == 1 && orphaned[0].Slot == "severity" && orphaned[0].Value == "safety") != c.orphaned {
			t.Errorf("%s: orphaned values are %+v", c.name, orphaned)
		}
	}
	// A card storing nothing for the gate has every conditioned slot
	// inapplicable, in the one order every report lists them.
	b := openConditioned(t, conditionedFixture(t, StorageFormat, blocks))
	if got := strings.Join(b.InapplicableSlots(fixtureCard(t, b)), ","); got != "severity,priority,task.trade" {
		t.Errorf("the inapplicable slots are %q, wanted severity, then priority, then the declared key", got)
	}
}

// trunkExport is the interchange form the build at 08fa893a wrote for the
// fixture below, captured from that build's own binary. A workbench carrying
// no condition has to export byte for byte as it did before conditions
// existed, and this is what byte for byte is measured against.
const trunkExport = `{
  "columns": [
    {
      "id": "cbbb67c1bd05",
      "kind": "intake",
      "slug": "intake",
      "title": "Intake"
    },
    {
      "id": "93a1d8fde955",
      "kind": "work",
      "slug": "doing",
      "title": "Doing"
    },
    {
      "id": "3522f97901bf",
      "kind": "done",
      "slug": "done",
      "title": "Done"
    }
  ],
  "fields": {
    "task.type": {
      "type": "string",
      "meaning": "whether the task is our own crew's work, subcontracted, or a snag found on inspection",
      "on": [
        "card"
      ]
    },
    "venue.deposit-paid": {
      "type": "date",
      "meaning": "the day the deposit fell due"
    }
  },
  "levels": {
    "severity": [
      "trivial",
      "minor",
      {
        "major": "A person's work is wrong or blocked; fix before new work."
      }
    ],
    "priority": [
      "later",
      "soon",
      "next",
      "now"
    ]
  },
  "profile": "dinah-core/0.18",
  "title": "wb"
}`

// TestAWorkbenchWithoutAConditionExportsAsItDidBefore asserts specification
// section 3.4's last rule against the capture above: the fixture is the
// workbench that capture was taken from, planted here file by file.
func TestAWorkbenchWithoutAConditionExportsAsItDidBefore(t *testing.T) {
	root := containedPath(t.TempDir())
	anchor := "---\nformat: 7\nprofile: dinah-core/0.18\ntitle: wb\nslug: fx\noperator: alka\n" +
		"levels:\n  severity:\n    - trivial\n    - minor\n    - major: A person's work is wrong or blocked; fix before new work.\n  priority: [later, soon, next, now]\n" +
		"fields:\n  task.type:\n    type: string\n    meaning: whether the task is our own crew's work, subcontracted, or a snag found on inspection\n    on: [card]\n  venue.deposit-paid:\n    type: date\n    meaning: the day the deposit fell due\n" +
		"columns:\n  - cbbb67c1bd05\n  - 93a1d8fde955\n  - 3522f97901bf\n---\n"
	write(t, filepath.Join(root, WorkbenchAnchor), anchor)
	for _, column := range [][3]string{{"cbbb67c1bd05", "Intake", "intake"}, {"93a1d8fde955", "Doing", "work"}, {"3522f97901bf", "Done", "done"}} {
		write(t, filepath.Join(root, ColumnsDir, column[0], ColumnAnchor), "---\ntitle: "+column[1]+"\nslug: "+strings.ToLower(column[1])+"\nkind: "+column[2]+"\n---\n")
	}
	encoded, err := openConditioned(t, root).Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if string(encoded) != trunkExport {
		t.Errorf("the export differs from the one the build at 08fa893a wrote:\n%s", encoded)
	}
}

// TestTheFormatFindingFiresOnlyWhereAConditionIsCarried asserts specification
// section 7.3 and section 10: a format-7 workbench carrying a condition is
// reported, one carrying none is not, the migration without confirmation
// leaves the anchor byte for byte, and with confirmation stamps the format
// after which the finding is gone.
func TestTheFormatFindingFiresOnlyWhereAConditionIsCarried(t *testing.T) {
	plain := openConditioned(t, conditionedFixture(t, AppliesWhenFormat-1, "levels:\n  severity: [cosmetic, functional, safety]\n"))
	if n := countOf(findingsOf(t, plain), FindingAppliesWhenBelowFormat); n != 0 {
		t.Errorf("a format-7 workbench carrying no condition draws %d format findings", n)
	}
	if plain.UsesAppliesWhen() {
		t.Error("a workbench carrying no condition and no mapping-form axis reports that it uses applies_when")
	}

	root := conditionedFixture(t, AppliesWhenFormat-1, constructionLevels+constructionFields)
	b := openConditioned(t, root)
	if !findingWith(findingsOf(t, b), FindingAppliesWhenBelowFormat, strconv.Itoa(AppliesWhenFormat-1)) {
		t.Fatalf("a format-7 workbench carrying a condition draws no %s", FindingAppliesWhenBelowFormat)
	}
	anchor := filepath.Join(root, WorkbenchAnchor)
	before, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	preview, err := b.MigrateAppliesWhen(false)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Stamped || !preview.Preview || preview.From != AppliesWhenFormat-1 {
		t.Errorf("the preview answers %+v", preview)
	}
	after, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("the preview wrote the anchor:\n%s", after)
	}
	stamped, err := b.MigrateAppliesWhen(true)
	if err != nil {
		t.Fatalf("stamp: %v", err)
	}
	if !stamped.Stamped || stamped.Preview {
		t.Errorf("the confirmed run answers %+v", stamped)
	}
	reopened := openConditioned(t, root)
	if reopened.Format != AppliesWhenFormat {
		t.Errorf("the workbench declares format %d after the stamp, wanted %d", reopened.Format, AppliesWhenFormat)
	}
	if n := countOf(findingsOf(t, reopened), FindingAppliesWhenBelowFormat); n != 0 {
		t.Errorf("the finding survives the stamp: %d", n)
	}
	again, err := reopened.MigrateAppliesWhen(true)
	if err != nil {
		t.Fatalf("stamp again: %v", err)
	}
	if again.Stamped {
		t.Error("a second confirmed run stamped a workbench already at the format")
	}
}

// TestANoticeNamesAColumnRequiringAConditionedKey asserts specification
// section 9: Bench.Notices reports a column requiring a key that carries a
// usable condition, not one requiring an unconditioned key, and Bench.Check
// returns no finding for either.
func TestANoticeNamesAColumnRequiringAConditionedKey(t *testing.T) {
	for _, required := range []string{"task.trade", "task.type"} {
		root := conditionedFixture(t, StorageFormat, constructionFields)
		write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), "---\ntitle: Only\nslug: only\nkind: work\nrequire_fields: ["+required+"]\n---\nColumn text.\n")
		b := openConditioned(t, root)
		if findings := findingsOf(t, b); len(findings) != 0 {
			t.Errorf("requiring %s draws findings: %+v", required, findings)
		}
		notices := b.Notices()
		if required == "task.type" {
			if len(notices) != 0 {
				t.Errorf("requiring an unconditioned key draws notices: %+v", notices)
			}
			continue
		}
		if len(notices) != 1 || notices[0].Key != NoticeRequiredFieldConditioned || notices[0].Detail != "only (task.trade)" {
			t.Errorf("requiring a conditioned key draws %+v", notices)
			continue
		}
		if notices[0].Path != filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor) {
			t.Errorf("the notice names %s rather than the column anchor", notices[0].Path)
		}
		if SeverityOf(notices[0]) != SeverityCleanup {
			t.Errorf("the notice carries the severity %q", SeverityOf(notices[0]))
		}
	}
}

// TestEachApplicabilityFindingCarriesItsSeverity asserts the table of
// specification section 7.4 read through SeverityOf, one assertion per key,
// on a workbench drawing all four findings at once.
func TestEachApplicabilityFindingCarriesItsSeverity(t *testing.T) {
	blocks := "levels:\n  severity:\n    values: [cosmetic, functional, safety]\n    applies_when:\n      field: task.type\n      is: [snag]\n  priority:\n    values: [later, now]\n    applies_when:\n      field: task.type\n" +
		"fields:\n  task.type:\n    type: string\n    meaning: what kind of task this is\n    on: [card]\n  vendor.confirmed:\n    type: boolean\n    meaning: whether the vendor has confirmed\n    on: [card]\n  vendor.deposit-required:\n    type: boolean\n    meaning: whether the vendor asks for a deposit\n    on: [card]\n    applies_when:\n      field: vendor.confirmed\n      is: [maybe]\n"
	root := conditionedFixture(t, AppliesWhenFormat-1, blocks)
	storeOnCard(t, root, "task.type", "own")
	storeOnCard(t, root, "severity", "safety")
	findings := findingsOf(t, openConditioned(t, root))
	want := map[string]string{
		FindingAppliesWhenMalformed:        SeverityDefect,
		FindingAppliesWhenValueUnmatchable: SeverityDefect,
		FindingAppliesWhenBelowFormat:      SeverityDefect,
		FindingInapplicableValue:           SeverityCleanup,
	}
	for key, severity := range want {
		found := false
		for _, finding := range findings {
			if finding.Key != key {
				continue
			}
			found = true
			if got := SeverityOf(finding); got != severity {
				t.Errorf("%s carries the severity %q, wanted %q", key, got, severity)
			}
		}
		if !found {
			t.Errorf("the fixture draws no %s; findings: %+v", key, findings)
		}
	}
}

// exportedObject decodes an export into its top-level members.
func exportedObject(t *testing.T, encoded []byte) map[string]json.RawMessage {
	t.Helper()
	object := map[string]json.RawMessage{}
	if err := json.Unmarshal(encoded, &object); err != nil {
		t.Fatalf("unmarshal the export: %v", err)
	}
	return object
}
