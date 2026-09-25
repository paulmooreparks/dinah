package bench

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// holdsFlow is Intake, Triage, Build Queue, Implement, Acceptance, Done and
// Closed, in flow order, each carrying its lower-case title as its slug.
func holdsFlow() []*Column {
	specs := []struct{ id, title, kind string }{
		{"e00000000001", "intake", contract.KindIntake},
		{"e00000000002", "triage", contract.KindWork},
		{"e00000000003", "build-queue", contract.KindBuffer},
		{"e00000000004", "implement", contract.KindWork},
		{"e00000000005", "acceptance", contract.KindWork},
		{"e00000000006", "done", contract.KindDone},
		{"e00000000007", "closed", contract.KindDone},
	}
	var columns []*Column
	for i, spec := range specs {
		columns = append(columns, &Column{ID: spec.id, Title: spec.title, Slug: spec.title, Kind: spec.kind, Position: i})
	}
	return columns
}

// resolverOf resolves a reference against columns by slug or identifier, as
// Bench.ColumnByRef does.
func resolverOf(columns []*Column) func(string) *Column {
	return func(ref string) *Column {
		for _, column := range columns {
			if column.ID == ref || column.Slug == ref {
				return column
			}
		}
		return nil
	}
}

// readHoldsOf reads a dinah.holds block, given whole, over holdsFlow.
func readHoldsOf(t *testing.T, block string) (HoldSettings, []HoldDefect) {
	t.Helper()
	fm, _ := ParseAnchor("---\ntitle: x\n" + block + "---\n")
	columns := holdsFlow()
	return ReadHolds(fm, columns, resolverOf(columns))
}

// TestReadHoldsFallsBackMemberByMember is dinah-608/criteria/12 at the
// reader: every defect is reported by kind and member, and each falls back as
// the specification's section 3.3 says.
func TestReadHoldsFallsBackMemberByMember(t *testing.T) {
	const implement, triage, acceptance = "e00000000004", "e00000000002", "e00000000005"
	cases := []struct {
		name    string
		block   string
		startAt string
		rules   string // kind:held:waits:lag:finish for each rule, joined by spaces
		defects string // defect:kind:member:read for each defect, joined by " | "
	}{
		{name: "no block", block: "", startAt: triage},
		{name: "a block with nothing beneath it", block: "dinah.holds:\n", startAt: triage},
		{name: "a scalar block", block: "dinah.holds: blocks\n", startAt: triage, defects: HoldsNotAMapping + ":::"},
		{name: "a declared commitment column", block: "dinah.holds:\n  start_at: implement\n", startAt: implement},
		{name: "start_at naming no column", block: "dinah.holds:\n  start_at: nowhere\n", startAt: triage, defects: HoldsMalformedMember + "::start_at:nowhere"},
		{name: "start_at naming a done column", block: "dinah.holds:\n  start_at: done\n", startAt: triage, defects: HoldsMalformedMember + "::start_at:" + holdReadDone},
		{name: "start_at naming the first column", block: "dinah.holds:\n  start_at: intake\n", startAt: triage, defects: HoldsMalformedMember + "::start_at:" + holdReadFirst},
		{name: "kinds not a mapping", block: "dinah.holds:\n  kinds: blocks\n", startAt: triage, defects: HoldsMalformedMember + "::kinds:blocks"},
		{name: "a kind not a mapping", block: "dinah.holds:\n  kinds:\n    blocks: named\n", startAt: triage, defects: HoldsKindNotAMapping + ":blocks::named"},
		{name: "held missing", block: "dinah.holds:\n  kinds:\n    blocks:\n      lag_days: 2\n", startAt: triage, defects: HoldsMalformedMember + ":blocks:held:\"\""},
		{name: "held unreadable", block: "dinah.holds:\n  kinds:\n    blocks:\n      held: both\n", startAt: triage, defects: HoldsMalformedMember + ":blocks:held:both"},
		{name: "a full rule", block: "dinah.holds:\n  start_at: implement\n  kinds:\n    blocks:\n      held: named\n      waits_for: finish\n      lag_days: 7\n      finish_at: acceptance\n", startAt: implement, rules: "blocks:named:finish:7:" + acceptance},
		{name: "defaults", block: "dinah.holds:\n  kinds:\n    parked_behind:\n      held: carrier\n", startAt: triage, rules: "parked_behind:carrier:finish:0:"},
		{name: "waits_for unreadable", block: "dinah.holds:\n  kinds:\n    blocks:\n      held: named\n      waits_for: soon\n", startAt: triage, rules: "blocks:named:finish:0:", defects: HoldsMalformedMember + ":blocks:waits_for:soon"},
		{name: "lag quoted", block: "dinah.holds:\n  kinds:\n    blocks:\n      held: named\n      lag_days: \"7\"\n", startAt: triage, rules: "blocks:named:finish:0:", defects: HoldsMalformedMember + ":blocks:lag_days:\"7\""},
		{name: "lag 366", block: "dinah.holds:\n  kinds:\n    blocks:\n      held: named\n      lag_days: 366\n", startAt: triage, rules: "blocks:named:finish:0:", defects: HoldsMalformedMember + ":blocks:lag_days:366"},
		{name: "lag 365", block: "dinah.holds:\n  kinds:\n    blocks:\n      held: named\n      lag_days: 365\n", startAt: triage, rules: "blocks:named:finish:365:"},
		{name: "finish_at naming no column", block: "dinah.holds:\n  kinds:\n    blocks:\n      held: named\n      finish_at: nowhere\n", startAt: triage, rules: "blocks:named:finish:0:", defects: HoldsMalformedMember + ":blocks:finish_at:nowhere"},
		{name: "finish_at beside a start", block: "dinah.holds:\n  kinds:\n    blocks:\n      held: named\n      finish_at: acceptance\n      waits_for: start\n", startAt: triage, rules: "blocks:named:start:0:", defects: HoldsMalformedMember + ":blocks:finish_at:" + holdReadStartOnly},
		{name: "finish_at before the commitment column", block: "dinah.holds:\n  start_at: implement\n  kinds:\n    blocks:\n      held: named\n      finish_at: triage\n", startAt: implement, rules: "blocks:named:finish:0:", defects: HoldsMalformedMember + ":blocks:finish_at:" + holdReadBeforeStart},
		{name: "an unknown member of the block", block: "dinah.holds:\n  start-at: implement\n", startAt: triage, defects: HoldsUnknownMember + "::start-at:implement"},
		{name: "an unknown member of a kind", block: "dinah.holds:\n  kinds:\n    blocks:\n      held: named\n      lag: 2\n", startAt: triage, rules: "blocks:named:finish:0:", defects: HoldsUnknownMember + ":blocks:lag:2"},
		{name: "a kind named start_at", block: "dinah.holds:\n  kinds:\n    start_at:\n      held: named\n", startAt: triage, rules: "start_at:named:finish:0:"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			settings, defects := readHoldsOf(t, c.block)
			if settings.StartAt != c.startAt {
				t.Errorf("the commitment column read as %s, want %s", settings.StartAt, c.startAt)
			}
			var rules []string
			for _, rule := range settings.Rules {
				rules = append(rules, strings.Join([]string{rule.Kind, rule.Held, rule.WaitsFor, strconv.Itoa(rule.LagDays), rule.FinishAt}, ":"))
			}
			if got := strings.Join(rules, " "); got != c.rules {
				t.Errorf("the rules read as %q, want %q", got, c.rules)
			}
			var read []string
			for _, defect := range defects {
				read = append(read, strings.Join([]string{defect.Defect, defect.Kind, defect.Member, defect.Read}, ":"))
			}
			if got := strings.Join(read, " | "); got != c.defects {
				t.Errorf("the defects read as %q, want %q", got, c.defects)
			}
		})
	}
}

// TestCheckReportsEveryHoldsDefect is dinah-608/criteria/12 at dinah check:
// each defect becomes the finding its kind names, at the workbench anchor.
func TestCheckReportsEveryHoldsDefect(t *testing.T) {
	block := "dinah.holds:\n  start_at: nowhere\n  colour: red\n  kinds:\n    blocks: named\n    parked_behind:\n      held: both\n    needs:\n      held: carrier\n      lag: 2\n"
	root := conditionedFixture(t, StorageFormat, block)
	findings := findingsOf(t, openConditioned(t, root))
	anchor := filepath.Join(root, WorkbenchAnchor)
	want := map[string]string{
		"dinah.holds start_at nowhere":        FindingHoldsMalformed,
		"colour":                              FindingHoldsMemberUnknown,
		"dinah.holds blocks not a mapping":    FindingHoldsMalformed,
		"dinah.holds parked_behind held both": FindingHoldsMalformed,
		"needs lag":                           FindingHoldsMemberUnknown,
	}
	seen := 0
	for _, finding := range findings {
		if finding.Key != FindingHoldsMalformed && finding.Key != FindingHoldsMemberUnknown {
			continue
		}
		seen++
		if want[finding.Detail] != finding.Key || finding.Path != anchor {
			t.Errorf("an unexpected finding %+v", finding)
		}
		if finding.Key == FindingHoldsMemberUnknown && SeverityOf(finding) != SeverityCleanup {
			t.Errorf("an unknown member is reported at %s", SeverityOf(finding))
		}
	}
	if seen != len(want) {
		t.Errorf("check reported %d holds findings, want %d: %+v", seen, len(want), findings)
	}
	scalar := conditionedFixture(t, StorageFormat, "dinah.holds: blocks\n")
	if !findingWith(findingsOf(t, openConditioned(t, scalar)), FindingHoldsMalformed, "dinah.holds not a mapping") {
		t.Error("a scalar block raised no finding")
	}
}

// TestTheHoldsFormatIsAFloorAndItsFindingNeedsARule is dinah-608/criteria/15
// at the store: StorageFormat is 11, the finding is raised on a format-10
// workbench declaring a usable rule and not otherwise, and the stamp writes
// the one format line only with the confirmation.
func TestTheHoldsFormatIsAFloorAndItsFindingNeedsARule(t *testing.T) {
	if StorageFormat != 11 || HoldsFormat != 11 {
		t.Fatalf("the formats are %d and %d, want 11", StorageFormat, HoldsFormat)
	}
	rule := "dinah.holds:\n  kinds:\n    blocks:\n      held: named\n"
	for name, c := range map[string]struct {
		format int
		block  string
		want   int
	}{
		"format 10 with a rule":         {10, rule, 1},
		"format 10 with no usable rule": {10, "dinah.holds:\n  kinds:\n    blocks:\n      held: both\n", 0},
		"format 10 with no block":       {10, "", 0},
		"format 11 with a rule":         {11, rule, 0},
	} {
		if n := countOf(findingsOf(t, openConditioned(t, conditionedFixture(t, c.format, c.block))), FindingHoldsBelowFormat); n != c.want {
			t.Errorf("%s raised %d below-format findings, want %d", name, n, c.want)
		}
	}
	root := conditionedFixture(t, 10, rule)
	anchor := filepath.Join(root, WorkbenchAnchor)
	before, _ := ReadText(anchor)
	preview, err := openConditioned(t, root).MigrateHolds(false)
	if err != nil || preview.Stamped || !preview.Preview || preview.From != 10 {
		t.Fatalf("the preview answered %+v %v", preview, err)
	}
	if after, _ := ReadText(anchor); after != before {
		t.Error("the preview wrote the anchor")
	}
	stamped, err := openConditioned(t, root).MigrateHolds(true)
	if err != nil || !stamped.Stamped {
		t.Fatalf("the stamp answered %+v %v", stamped, err)
	}
	if after, _ := ReadText(anchor); after != strings.Replace(before, "format: 10", "format: 11", 1) {
		t.Errorf("the stamp wrote more than the format line:\n%s", after)
	}
	if !findingWith(findingsOf(t, openConditioned(t, conditionedFixture(t, 10, rule))), FindingHoldsBelowFormat, "10") {
		t.Error("the finding does not name the declared format")
	}
	again, err := openConditioned(t, root).MigrateHolds(true)
	if err != nil || again.Stamped {
		t.Errorf("a second stamp answered %+v %v", again, err)
	}
	old := conditionedFixture(t, 6, "")
	opened, err := openFixtureAtAnyFormat(t, old)
	if err != nil {
		t.Fatalf("open the format-6 store: %v", err)
	}
	_, err = opened.MigrateHolds(true)
	var refusal *contract.Refusal
	if !errors.As(err, &refusal) || refusal.Name != contract.StoreAwaitingMigration {
		t.Errorf("a format-6 store answered %v", err)
	}
}

// TestHoldCycleThroughNamesTheShortestWayBack pins the path the link warning
// names, and that an edge closing nothing names nothing.
func TestHoldCycleThroughNamesTheShortestWayBack(t *testing.T) {
	edges := []HoldEdge{
		{Held: "a", Holder: "b"}, {Held: "b", Holder: "c"}, {Held: "c", Holder: "a"},
		{Held: "b", Holder: "a"}, {Held: "d", Holder: "a"},
	}
	if got := strings.Join(HoldCycleThrough(edges, "a", "b"), ","); got != "a,b" {
		t.Errorf("the cycle through a and b reads %q, want the shorter a,b", got)
	}
	if got := HoldCycleThrough(edges, "d", "a"); got != nil {
		t.Errorf("an edge closing no cycle names %v", got)
	}
	if got := strings.Join(HoldCycleThrough(nil, "e", "e"), ","); got != "e" {
		t.Errorf("a self edge names %q", got)
	}
}
