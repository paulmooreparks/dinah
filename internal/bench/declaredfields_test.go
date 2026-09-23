package bench

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// declaringFixture is newFixture with a fields block and a declared value on
// the workbench, which is the shape every case below starts from.
//
// The block is written out rather than composed, because the shape of the
// block on the page is half of what this file is about and a composed fixture
// would test the composer rather than the reader.
func declaringFixture(t *testing.T, format int) string {
	t.Helper()
	root := containedPath(t.TempDir())
	anchor := strings.Replace(benchDefinition, "format: 6", "format: "+strconv.Itoa(format), 1)
	anchor = strings.Replace(anchor, "columns:\n", `fields:
  git.branch:
    type: string
    meaning: the branch the card's code lives on
    on: [card]
  venue.deposit-paid:
    type: date
    meaning: the day the deposit fell due
  git.trunk:
    type: string
    meaning: the branch cards merge into
    on: [workbench, column]
  trip.destination:
    type: string
    meaning: the city the trip is to
    on: [workstream]
  ghost.key:
    type: string
    meaning: a key whose on names a kind no declaration reaches
    on: [comment]
field_values:
  git.trunk: main
columns:
`, 1)
	write(t, filepath.Join(root, WorkbenchAnchor), anchor)
	write(t, filepath.Join(root, CardNumbersName), "1 c00000000001\n")
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), columnDefinition)
	write(t, filepath.Join(root, CardsDir, "c00000000001", CardAnchor), cleanCard)
	write(t, filepath.Join(root, CardsDir, "c00000000001", JournalName), cleanJournal)
	return root
}

// TestTheDeclaredKeyGrammarAdmitsAndRefuses drives CORE-FIELD-1, CORE-FIELD-2
// and CORE-FIELD-3. A fields block declaring three well-formed keys declares
// all three; six keys of the shapes the grammar refuses are each undeclared
// and each reported, naming the key.
//
// The compiled expression is pinned against the published one character for
// character, because a grammar an implementer derived rather than copied is
// the defect this card's own review found in the first draft: a segment that
// may end in a hyphen admits `git.branch-`, which the case list below refuses.
func TestTheDeclaredKeyGrammarAdmitsAndRefuses(t *testing.T) {
	const published = `^[a-z][a-z0-9]*(-[a-z0-9]+)*(\.[a-z][a-z0-9]*(-[a-z0-9]+)*)+$`
	if DeclaredFieldKeyExpression != published {
		t.Errorf("the code compiles %q, and the specification publishes %q", DeclaredFieldKeyExpression, published)
	}
	if declaredFieldKey.String() != published {
		t.Errorf("the compiled expression is %q, wanted the published one", declaredFieldKey.String())
	}
	admitted := []string{"git.branch", "gk-software.customer", "com-example.customer.id"}
	refused := []string{"Git.Branch", "git..branch", "1git.branch", "git-branch", "git.branch-", ".git.branch"}
	for _, key := range admitted {
		if !DeclaredFieldKey(key) {
			t.Errorf("%s is refused by the grammar and the specification admits it", key)
		}
	}
	for _, key := range refused {
		if DeclaredFieldKey(key) {
			t.Errorf("%s is admitted by the grammar and the specification refuses it", key)
		}
	}
	// A key at the two limits and a key past each of them, so the bounds are
	// pinned from both sides rather than asserted from one.
	long := "a." + strings.Repeat("b", DeclaredFieldSegmentLimit)
	if !DeclaredFieldKey(long) {
		t.Errorf("a segment of exactly %d characters is refused", DeclaredFieldSegmentLimit)
	}
	if DeclaredFieldKey(long + "b") {
		t.Errorf("a segment of %d characters is admitted", DeclaredFieldSegmentLimit+1)
	}

	root := containedPath(t.TempDir())
	block := "fields:\n"
	for _, key := range append(append([]string{}, admitted...), refused...) {
		block += "  " + key + ":\n    type: string\n    meaning: a fact\n"
	}
	write(t, filepath.Join(root, WorkbenchAnchor), strings.Replace(
		strings.Replace(benchDefinition, "format: 6", "format: "+strconv.Itoa(RegistryFormat), 1),
		"columns:\n", block+"columns:\n", 1))
	write(t, filepath.Join(root, CardNumbersName), "1 c00000000001\n")
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), columnDefinition)
	write(t, filepath.Join(root, CardsDir, "c00000000001", CardAnchor), cleanCard)
	write(t, filepath.Join(root, CardsDir, "c00000000001", JournalName), cleanJournal)
	opened, err := openFixtureAtAnyFormat(t, root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if got := len(opened.DeclaredFields()); got != len(admitted) {
		t.Errorf("the workbench declares %d fields, wanted %d", got, len(admitted))
	}
	named := map[string]bool{}
	for _, finding := range declaredFindings(t, opened, FindingFieldDeclarationMalformed) {
		named[finding.Detail] = true
	}
	if len(named) != len(refused) {
		t.Errorf("check reports %d malformed declarations, wanted %d: %v", len(named), len(refused), named)
	}
	for _, key := range refused {
		if !named[key] {
			t.Errorf("no finding names the refused key %s", key)
		}
	}
}

// TestADeclarationEntryIsRefusedWithoutATypeOrAMeaning drives CORE-FIELD-3.
// An entry whose type is absent or outside the five, and one whose meaning is
// absent or blank, each declare nothing and are each reported. The entry
// beside them, well formed, is what keeps this from passing against a reader
// that refuses everything.
func TestADeclarationEntryIsRefusedWithoutATypeOrAMeaning(t *testing.T) {
	root := containedPath(t.TempDir())
	block := `fields:
  good.key:
    type: string
    meaning: a fact worth keeping
  no.type:
    meaning: a fact with no type
  wrong.type:
    type: colour
    meaning: a fact with a type outside the five
  no.meaning:
    type: string
  blank.meaning:
    type: string
    meaning: ""
`
	write(t, filepath.Join(root, WorkbenchAnchor), strings.Replace(
		strings.Replace(benchDefinition, "format: 6", "format: "+strconv.Itoa(RegistryFormat), 1),
		"columns:\n", block+"columns:\n", 1))
	write(t, filepath.Join(root, CardNumbersName), "1 c00000000001\n")
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), columnDefinition)
	write(t, filepath.Join(root, CardsDir, "c00000000001", CardAnchor), cleanCard)
	write(t, filepath.Join(root, CardsDir, "c00000000001", JournalName), cleanJournal)
	opened, err := openFixtureAtAnyFormat(t, root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	declared := opened.DeclaredFields()
	if len(declared) != 1 || declared[0].Key != "good.key" {
		t.Fatalf("the workbench declares %+v, wanted good.key alone", declared)
	}
	findings := declaredFindings(t, opened, FindingFieldDeclarationMalformed)
	if len(findings) != 4 {
		t.Errorf("check reports %d malformed declarations, wanted four", len(findings))
	}
}

// TestALineOpeningWithAHyphenIsReportedRatherThanSwallowed drives CORE-FIELD-2
// at the one shape the reader cannot see as a key.
//
// The dashed-entry pattern is tried ahead of the member pattern, so a key
// spelled with a leading hyphen reads as an entry of an `on` list wherever it
// stands, and before this guard it was neither declared nor reported: the
// grammar refuses it, and nothing said so. A dashed line met outside an `on`
// list is now reported, naming the line as written.
//
// The entry beside it declares a legitimate `on` list in the dashed spelling,
// which is what keeps this from passing against a reader that reports every
// dashed line and stops reading the one it is for.
func TestALineOpeningWithAHyphenIsReportedRatherThanSwallowed(t *testing.T) {
	root := containedPath(t.TempDir())
	block := `fields:
  good.key:
    type: string
    meaning: a fact worth keeping
    on:
      - card
      - column
  -hyphen.led:
    type: string
    meaning: a key the grammar refuses and the dashed pattern swallows
`
	write(t, filepath.Join(root, WorkbenchAnchor), strings.Replace(
		strings.Replace(benchDefinition, "format: 6", "format: "+strconv.Itoa(RegistryFormat), 1),
		"columns:\n", block+"columns:\n", 1))
	write(t, filepath.Join(root, CardNumbersName), "1 c00000000001\n")
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), columnDefinition)
	write(t, filepath.Join(root, CardsDir, "c00000000001", CardAnchor), cleanCard)
	write(t, filepath.Join(root, CardsDir, "c00000000001", JournalName), cleanJournal)
	opened, err := openFixtureAtAnyFormat(t, root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	declared := opened.DeclaredFields()
	if len(declared) != 1 || declared[0].Key != "good.key" {
		t.Fatalf("the workbench declares %+v, wanted good.key alone", declared)
	}
	if got := declared[0].Kinds(); !reflect.DeepEqual(got, []string{KindCard, KindColumn}) {
		t.Errorf("the dashed `on` list read as %v, wanted [card column]", got)
	}
	named := []string{}
	for _, finding := range declaredFindings(t, opened, FindingFieldDeclarationMalformed) {
		named = append(named, finding.Detail)
	}
	if !reflect.DeepEqual(named, []string{"-hyphen.led:"}) {
		t.Errorf("check reports %v, wanted the line opening with a hyphen and nothing else", named)
	}
}

// TestADeclarationNamingNoKindsReachesAllThree drives CORE-FIELD-4. A key
// declaring `on` reaches exactly the kinds it names, and a key declaring none
// reaches a workbench, a column and a card alike.
func TestADeclarationNamingNoKindsReachesAllThree(t *testing.T) {
	opened, err := openFixtureAtAnyFormat(t, declaringFixture(t, RegistryFormat))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	branch := opened.DeclaredFieldOf("git.branch")
	trunk := opened.DeclaredFieldOf("git.trunk")
	deposit := opened.DeclaredFieldOf("venue.deposit-paid")
	if branch == nil || trunk == nil || deposit == nil {
		t.Fatalf("the workbench declares %+v, wanted all three", opened.DeclaredFields())
	}
	for kind, want := range map[string]bool{KindCard: true, KindColumn: false, KindWorkbench: false} {
		if branch.Declares(kind) != want {
			t.Errorf("git.branch on %s reports %v, wanted %v", kind, branch.Declares(kind), want)
		}
	}
	for kind, want := range map[string]bool{KindCard: false, KindColumn: true, KindWorkbench: true} {
		if trunk.Declares(kind) != want {
			t.Errorf("git.trunk on %s reports %v, wanted %v", kind, trunk.Declares(kind), want)
		}
	}
	for _, kind := range []string{KindCard, KindColumn, KindWorkbench} {
		if !deposit.Declares(kind) {
			t.Errorf("venue.deposit-paid declares no `on` and does not reach %s", kind)
		}
	}
	// Declaration order is the order a reader typed, and it is what `show`
	// prints, so it is asserted here rather than left to a listing.
	var order []string
	for _, field := range opened.DeclaredFields() {
		order = append(order, field.Key)
	}
	want := []string{"git.branch", "venue.deposit-paid", "git.trunk", "trip.destination", "ghost.key"}
	if !reflect.DeepEqual(order, want) {
		t.Errorf("the declaration order is %v, wanted %v", order, want)
	}
	if got := opened.DeclaredFieldKeysOn(KindColumn); !reflect.DeepEqual(got, []string{"venue.deposit-paid", "git.trunk"}) {
		t.Errorf("the keys declared on a column are %v", got)
	}
}

// TestEachDeclaredTypeAdmitsOneValueAndRefusesAnother drives CORE-FIELD-3 and
// CORE-FIELD-5. Each of the five types accepts one value and refuses another,
// so a guard that accepted everything and a guard that refused everything both
// fail here.
func TestEachDeclaredTypeAdmitsOneValueAndRefusesAnother(t *testing.T) {
	cases := []struct {
		declared string
		value    string
		admit    bool
	}{
		{FieldTypeString, "one line", true},
		{FieldTypeString, "two\nlines", false},
		{FieldTypeNumber, "3.5", true},
		{FieldTypeNumber, "3.5.1", false},
		{FieldTypeBoolean, "true", true},
		{FieldTypeBoolean, "yes", false},
		{FieldTypeURL, "https://example.com/x", true},
		{FieldTypeURL, "12", false},
		{FieldTypeDate, "2026-09-14", true},
		{FieldTypeDate, "2026-02-30", false},
	}
	admitted := 0
	for _, c := range cases {
		got := AdmitsFieldValue(c.declared, c.value)
		if got != c.admit {
			t.Errorf("%s admitting %q reports %v, wanted %v", c.declared, c.value, got, c.admit)
		}
		if got {
			admitted++
		}
	}
	if len(cases) != 10 {
		t.Errorf("the table carries %d outcomes, wanted ten", len(cases))
	}
	if admitted != 5 {
		t.Errorf("%d of the ten were admitted, wanted five", admitted)
	}
}

// TestAColumnRequiringAnUndeclaredFieldIsReported drives CORE-FIELD-10. A
// column whose require_fields names a key the workbench does not declare is
// reported, naming the column and the key, and the workbench still opens. A
// column naming a key the workbench does declare produces no such finding,
// which is what keeps this from passing against a check that reports every
// requirement.
func TestAColumnRequiringAnUndeclaredFieldIsReported(t *testing.T) {
	root := declaringFixture(t, FieldsFormat)
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor),
		strings.Replace(columnDefinition, "kind: work\n", "kind: work\nrequire_fields: [git.nothing]\n", 1))
	opened, err := openFixtureAtAnyFormat(t, root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	findings := declaredFindings(t, opened, FindingRequiredFieldUndeclared)
	if len(findings) != 1 {
		t.Fatalf("check reports %d undeclared requirements, wanted one", len(findings))
	}
	if !strings.Contains(findings[0].Detail, "only") || !strings.Contains(findings[0].Detail, "git.nothing") {
		t.Errorf("the finding says %q, which does not name the column and the key", findings[0].Detail)
	}

	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor),
		strings.Replace(columnDefinition, "kind: work\n", "kind: work\nrequire_fields: [git.trunk]\n", 1))
	opened, err = openFixtureAtAnyFormat(t, root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := opened.Column("b00000000001").RequireFields; !reflect.DeepEqual(got, []string{"git.trunk"}) {
		t.Errorf("the column requires %v, wanted [git.trunk]", got)
	}
	if found := declaredFindings(t, opened, FindingRequiredFieldUndeclared); len(found) != 0 {
		t.Errorf("a declared key is reported as undeclared: %+v", found)
	}
}

// TestASurvivingHeadingIsReportedOnlyOnAMigratedWorkbench drives the format
// number's one job. A workbench declaring the format the retirement arrived at
// reports a card whose body still carries the heading; the same workbench
// declaring the format before it reports none; and a migrated workbench whose
// cards carry no heading reports none either.
//
// The three counts are asserted rather than the presence of one finding,
// because a check that reported every card would pass the first case alone.
func TestASurvivingHeadingIsReportedOnlyOnAMigratedWorkbench(t *testing.T) {
	withHeading := strings.Replace(cleanCard, "Framing.\n", "Framing.\n\n## Branch\n\ndinah-498-declared-fields\n", 1)
	cases := []struct {
		name   string
		format int
		card   string
		want   int
	}{
		{"a migrated workbench carrying a heading", FieldsFormat, withHeading, 1},
		{"the same workbench before the migration", FieldsFormat - 1, withHeading, 0},
		{"a migrated workbench carrying none", FieldsFormat, cleanCard, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := declaringFixture(t, c.format)
			write(t, filepath.Join(root, CardsDir, "c00000000001", CardAnchor), c.card)
			opened, err := openFixtureAtAnyFormat(t, root)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			if got := len(declaredFindings(t, opened, FindingBranchHeadingInBody)); got != c.want {
				t.Errorf("check reports %d surviving headings, wanted %d", got, c.want)
			}
		})
	}
}

// TestTheInterchangeFormCarriesTheFourNewMembers drives CORE-JSON-11 and
// CORE-JSON-12. A workbench declaring fields, holding a value of its own and
// carrying a column that requires one exports all four members and no value
// twice, and instantiating that export yields a workbench whose declaration,
// values and requirements are the ones the original carried, read off the
// anchors the instantiation wrote rather than off the objects it was given.
//
// The workbench that declares none of them is what keeps the assertion from
// passing against an exporter that emits the members unconditionally.
func TestTheInterchangeFormCarriesTheFourNewMembers(t *testing.T) {
	root := declaringFixture(t, FieldsFormat)
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor),
		strings.Replace(columnDefinition, "kind: work\n",
			"kind: work\nrequire_fields: [git.trunk]\nfield_values:\n  git.trunk: release\n", 1))
	opened, err := openFixtureAtAnyFormat(t, root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	exported, err := opened.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	object := map[string]json.RawMessage{}
	if err := json.Unmarshal(exported, &object); err != nil {
		t.Fatalf("read the export: %v", err)
	}
	for _, member := range []string{FieldsKey, FieldValuesKey} {
		if _, carried := object[member]; !carried {
			t.Errorf("the workbench object carries no %s member", member)
		}
	}
	var columns []map[string]json.RawMessage
	if err := json.Unmarshal(object["columns"], &columns); err != nil {
		t.Fatalf("read the columns: %v", err)
	}
	for _, member := range []string{FieldValuesKey, RequireFieldsKey} {
		if _, carried := columns[0][member]; !carried {
			t.Errorf("the column object carries no %s member", member)
		}
	}
	// The declaration order survives the trip, which a Go map would not carry
	// and which is what `show` prints an entity's fields in.
	if got := string(object[FieldsKey]); strings.Index(got, "git.branch") > strings.Index(got, "git.trunk") {
		t.Errorf("the exported declaration reorders its entries: %s", got)
	}

	definition, err := ReadDefinition(exported)
	if err != nil {
		t.Fatalf("read the definition: %v", err)
	}
	clone := containedPath(t.TempDir())
	if err := Instantiate(clone, "fx", "alka", definition); err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	cloned, err := Open(clone)
	if err != nil {
		t.Fatalf("open the clone: %v", err)
	}
	if !reflect.DeepEqual(cloned.DeclaredFields(), opened.DeclaredFields()) {
		t.Errorf("the clone declares %+v, wanted %+v", cloned.DeclaredFields(), opened.DeclaredFields())
	}
	if got := FieldValue(cloned.FM, "git.trunk"); got != "main" {
		t.Errorf("the clone's own value for git.trunk is %q, wanted main", got)
	}
	clonedColumn := cloned.Columns[0]
	if !reflect.DeepEqual(clonedColumn.RequireFields, []string{"git.trunk"}) {
		t.Errorf("the cloned column requires %v", clonedColumn.RequireFields)
	}
	if got := FieldValue(clonedColumn.FM, "git.trunk"); got != "release" {
		t.Errorf("the cloned column's value for git.trunk is %q, wanted release", got)
	}

	bare, err := Open(newFixture(t))
	if err != nil {
		t.Fatalf("open the bare workbench: %v", err)
	}
	bareExport, err := bare.Export()
	if err != nil {
		t.Fatalf("export the bare workbench: %v", err)
	}
	bareObject := map[string]json.RawMessage{}
	if err := json.Unmarshal(bareExport, &bareObject); err != nil {
		t.Fatalf("read the bare export: %v", err)
	}
	for _, member := range []string{FieldsKey, FieldValuesKey} {
		if _, carried := bareObject[member]; carried {
			t.Errorf("a workbench declaring nothing exports a %s member", member)
		}
	}
}

// TestAnUndeclaredKeyInTheValueBlockSurvivesAWriteOfItsNeighbour drives
// CORE-FIELD-7 and CORE-FIELD-9. A key nothing declares keeps its bytes across
// a write of the key beside it, a value never reaches the top level of an
// anchor, and a value cleared takes its key with it.
func TestAnUndeclaredKeyInTheValueBlockSurvivesAWriteOfItsNeighbour(t *testing.T) {
	fm, body := ParseAnchor(`---
title: A card
field_values:
  imported.key: kept
  git.branch: first
---
Framing.
`)
	if got := FieldValue(fm, "imported.key"); got != "kept" {
		t.Errorf("the undeclared key reads back as %q, wanted kept", got)
	}
	SetFieldValue(fm, "git.branch", "second")
	rendered := fm.Render(body)
	if !strings.Contains(rendered, "  imported.key: kept\n") {
		t.Errorf("the undeclared line did not survive the write:\n%s", rendered)
	}
	if strings.Contains(rendered, "\ngit.branch:") {
		t.Errorf("a value reached the top level of the anchor:\n%s", rendered)
	}
	if got := FieldValue(fm, "git.branch"); got != "second" {
		t.Errorf("the written value reads back as %q", got)
	}
	SetFieldValue(fm, "git.branch", "")
	if got := FieldValue(fm, "git.branch"); got != "" {
		t.Errorf("a cleared value reads back as %q", got)
	}
	if !strings.Contains(fm.Render(body), "  imported.key: kept\n") {
		t.Errorf("the clear took its neighbour with it:\n%s", fm.Render(body))
	}
	SetFieldValue(fm, "imported.key", "")
	if fm.Has(FieldValuesKey) {
		t.Errorf("a block left with no key is still carried:\n%s", fm.Render(body))
	}
}

// TestADeclarationReachesAWorkstreamOnlyWhereItNamesOne drives dinah-582's
// split of the one kind list into two. DeclarableKinds says which kinds an
// `on` member may name and DefaultKinds says which kinds an entry naming no
// `on` reaches, and the workstream is in the first and not the second.
//
// The three declarations exercise the three outcomes the split produces: an
// `on` naming the workstream reaches it and nothing else, an entry naming no
// `on` reaches the three of DefaultKinds and not the workstream, and an `on`
// naming a kind no declaration reaches declares nothing at all. Both lists
// are held to a length as well as to their members, because a list somebody
// emptied answers every membership question with false and reads exactly like
// a list nothing failed against.
func TestADeclarationReachesAWorkstreamOnlyWhereItNamesOne(t *testing.T) {
	if len(DeclarableKinds) != 4 {
		t.Errorf("DeclarableKinds carries %d kinds, wanted 4: %v", len(DeclarableKinds), DeclarableKinds)
	}
	if got := strings.Join(DeclarableKinds, " "); got != strings.Join([]string{KindCard, KindColumn, KindWorkbench, KindWorkstream}, " ") {
		t.Errorf("DeclarableKinds is %v", DeclarableKinds)
	}
	if len(DefaultKinds) != 3 {
		t.Errorf("DefaultKinds carries %d kinds, wanted 3: %v", len(DefaultKinds), DefaultKinds)
	}
	if got := strings.Join(DefaultKinds, " "); got != strings.Join([]string{KindCard, KindColumn, KindWorkbench}, " ") {
		t.Errorf("DefaultKinds is %v", DefaultKinds)
	}

	opened, err := openFixtureAtAnyFormat(t, declaringFixture(t, RegistryFormat))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	destination := opened.DeclaredFieldOf("trip.destination")
	deposit := opened.DeclaredFieldOf("venue.deposit-paid")
	ghost := opened.DeclaredFieldOf("ghost.key")
	if destination == nil || deposit == nil || ghost == nil {
		t.Fatalf("the workbench declares %+v", opened.DeclaredFields())
	}

	// An `on` naming the workstream reaches it and reaches nothing else.
	for kind, want := range map[string]bool{
		KindWorkstream: true, KindCard: false, KindColumn: false, KindWorkbench: false,
	} {
		if destination.Declares(kind) != want {
			t.Errorf("trip.destination on %s reports %v, wanted %v", kind, destination.Declares(kind), want)
		}
	}
	if got := destination.Kinds(); !reflect.DeepEqual(got, []string{KindWorkstream}) {
		t.Errorf("trip.destination reaches %v, wanted just the workstream", got)
	}

	// An entry naming no `on` reaches the three of DefaultKinds and stops
	// short of the workstream, which is CORE-FIELD-4's fixed default.
	for _, kind := range DefaultKinds {
		if !deposit.Declares(kind) {
			t.Errorf("venue.deposit-paid declares no `on` and does not reach %s", kind)
		}
	}
	if deposit.Declares(KindWorkstream) {
		t.Error("venue.deposit-paid declares no `on` and reaches a workstream")
	}
	if got := deposit.Kinds(); !reflect.DeepEqual(got, []string{KindCard, KindColumn, KindWorkbench}) {
		t.Errorf("venue.deposit-paid reaches %v, wanted the three of DefaultKinds", got)
	}

	// An `on` naming a kind outside DeclarableKinds reaches nothing, which
	// is what proves the widened list admitted one kind rather than opening
	// the gate.
	for _, kind := range EntityKinds() {
		if ghost.Declares(kind) {
			t.Errorf("ghost.key names `on: [comment]` and reaches %s", kind)
		}
	}
}

// TestAWorkstreamAnchorCarriesItsDeclaredValues reads CORE-FIELD-7 at the kind
// dinah-582 adds. A workstream anchor's field_values block round-trips through
// a render and a reparse, and a key the workbench does not declare keeps its
// bytes across a write of the key beside it.
func TestAWorkstreamAnchorCarriesItsDeclaredValues(t *testing.T) {
	fm, body := ParseAnchor(`---
title: Berlin trip
slug: berlin
status: active
field_values:
  imported.key: kept
---
What the trip is for.
`)
	SetFieldValue(fm, "trip.destination", "Berlin")
	rendered := fm.Render(body)

	reparsed, reparsedBody := ParseAnchor(rendered)
	if got := FieldValue(reparsed, "trip.destination"); got != "Berlin" {
		t.Errorf("the written value reads back as %q after a render and a reparse", got)
	}
	if got := FieldValue(reparsed, "imported.key"); got != "kept" {
		t.Errorf("the undeclared key reads back as %q, wanted kept", got)
	}
	if got := FieldValues(reparsed); !reflect.DeepEqual(got, []StoredField{{Key: "imported.key", Value: "kept"}, {Key: "trip.destination", Value: "Berlin"}}) {
		t.Errorf("the block carries %v, wanted the undeclared key and the written one", got)
	}
	if strings.Contains(rendered, "\ntrip.destination:") {
		t.Errorf("a value reached the top level of the workstream anchor:\n%s", rendered)
	}
	if got := reparsed.Value(TitleField); got != "Berlin trip" {
		t.Errorf("the workstream's own title reads back as %q", got)
	}
	if strings.TrimSpace(reparsedBody) != "What the trip is for." {
		t.Errorf("the workstream's notes read back as %q", reparsedBody)
	}
}

// declaredFindings runs the checker over a whole workbench and answers the
// findings carrying one key, failing the test if the run itself could not
// finish. findingsOfKey in check_readfailure_test.go filters a slice somebody
// already has; this one is what produces the slice.
func declaredFindings(t *testing.T, b *Bench, key string) []Finding {
	t.Helper()
	findings, err := b.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	var matched []Finding
	for _, finding := range findings {
		if finding.Key == key {
			matched = append(matched, finding)
		}
	}
	return matched
}
