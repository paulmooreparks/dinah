package bench

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// declaringColumn is the wedding column of the dinah-593 contract, planted as
// the fixture's one column: three well-formed entries in declaration order,
// one carrying an evidence scheme and one an operator owner.
const declaringColumn = `---
title: Only
slug: only
kind: work
gate_items: out
standing_items:
  deposit-paid:
    kind: acceptance_criterion
    text: The vendor's deposit has been paid and the receipt is on the card.
    evidence: receipt
  contract-countersigned:
    kind: acceptance_criterion
    text: Both parties have signed the vendor contract.
    owner: operator
  date-confirmed:
    kind: open_question
    text: Has the vendor confirmed the wedding date in writing?
---
Column text.
`

// wellFormedEntries is what declaringColumn declares, as the reader answers it.
var wellFormedEntries = []StandingItem{
	{Key: "deposit-paid", Kind: "acceptance_criterion", Text: "The vendor's deposit has been paid and the receipt is on the card.", Evidence: "receipt"},
	{Key: "contract-countersigned", Kind: "acceptance_criterion", Text: "Both parties have signed the vendor contract.", Owner: "operator"},
	{Key: "date-confirmed", Kind: "open_question", Text: "Has the vendor confirmed the wedding date in writing?"},
}

// plantStandingInstance writes one checklist item carrying the column and the
// standing key a minting would write, in the state given, so a test can put a
// card in every re-entry position without going through an arrival.
func plantStandingInstance(t *testing.T, root, card, id, column, key, state string) string {
	t.Helper()
	fm := NewFrontmatter()
	fm.Set(ItemKindField, "acceptance_criterion")
	fm.Set(ItemStateField, state)
	fm.Set(ItemColumnField, column)
	fm.Set(ItemStandingField, key)
	path := filepath.Join(root, CardsDir, card, ChecklistDir, id, ItemAnchor)
	write(t, path, fm.Render("An instance.\n"))
	return path
}

// plantItemState writes one checklist item carrying a column value and a
// state, which is plantItemColumn with the state chosen, for the cases that
// turn on whether a withdrawn item is passed over.
func plantItemState(t *testing.T, root, card, id, column, state string) string {
	t.Helper()
	fm := NewFrontmatter()
	fm.Set(ItemKindField, "decision")
	fm.Set(ItemStateField, state)
	if column != "" {
		fm.Set(ItemColumnField, column)
	}
	path := filepath.Join(root, CardsDir, card, ChecklistDir, id, ItemAnchor)
	write(t, path, fm.Render("An item.\n"))
	return path
}

// TestReadStandingItemsKeepsDeclarationOrderAndSkipsWhatItCannotRead drives
// the reader posture the contract fixes: the three well-formed entries come
// back in the order the file declares them with every member, and each of the
// four defects is skipped and named rather than raised over or swallowed.
// The count of refused entries is asserted as well as their names, because a
// reader that refused everything would name the four and pass a check that
// read only the names.
func TestReadStandingItemsKeepsDeclarationOrderAndSkipsWhatItCannotRead(t *testing.T) {
	root := newFixture(t)
	damaged := strings.Replace(declaringColumn, "  date-confirmed:\n", `  Bad Key:
    kind: decision
    text: A key the grammar refuses.
  no-kind:
    text: An entry declaring no kind.
  odd-kind:
    kind: reminder
    text: An entry declaring a kind outside the three.
  no-text:
    kind: decision
  - a dashed line where a member was expected
  date-confirmed:
`, 1)
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), damaged)
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	column := opened.Column("b00000000001")
	if !reflect.DeepEqual(column.StandingItems, wellFormedEntries) {
		t.Errorf("the reader answered %+v, wanted %+v", column.StandingItems, wellFormedEntries)
	}
	wantMalformed := []string{"- a dashed line where a member was expected", "Bad Key", "no-kind", "odd-kind", "no-text"}
	if !reflect.DeepEqual(column.MalformedStandingItems, wantMalformed) {
		t.Errorf("the reader refused %v, wanted %v", column.MalformedStandingItems, wantMalformed)
	}
	findings := findingsOfKey(declaredFindings(t, opened, FindingStandingItemMalformed), FindingStandingItemMalformed)
	if len(findings) != len(wantMalformed) {
		t.Fatalf("check reported %d malformed entries, wanted %d: %+v", len(findings), len(wantMalformed), findings)
	}
	for at, finding := range findings {
		if want := "only " + wantMalformed[at]; finding.Detail != want {
			t.Errorf("finding %d reads %q, wanted %q", at, finding.Detail, want)
		}
		if SeverityOf(finding) != SeverityDefect {
			t.Errorf("finding %d carries severity %q, wanted the default", at, finding.Severity)
		}
	}

	// A column whose entries are all well formed reports nothing under the
	// key, which is what tells this from a sweep reporting every entry.
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), declaringColumn)
	opened, err = Open(root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if found := declaredFindings(t, opened, FindingStandingItemMalformed); len(found) != 0 {
		t.Errorf("a well-formed declaration is reported: %+v", found)
	}
	if len(opened.Column("b00000000001").MalformedStandingItems) != 0 {
		t.Errorf("a well-formed declaration carries refused entries: %v", opened.Column("b00000000001").MalformedStandingItems)
	}
}

// TestMissingStandingItemsReadsTheLiveChecklistWhateverTheState drives the
// identity rule: an instance in any of the six states counts as present, an
// instance naming another column or carrying no standing key does not, and
// the entries come back in declaration order. Every state is planted, because
// a reading keyed on pending alone would re-mint over a settled instance.
func TestMissingStandingItemsReadsTheLiveChecklistWhateverTheState(t *testing.T) {
	for _, state := range ItemStates {
		t.Run(state, func(t *testing.T) {
			root := newFixture(t)
			write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), declaringColumn)
			plantStandingInstance(t, root, "c00000000001", "d00000000001", "b00000000001", "deposit-paid", state)
			// An instance of the same key minted for another column, and a
			// hand-filed item naming this column, are neither of them this
			// column's instance.
			plantStandingInstance(t, root, "c00000000001", "d00000000002", "b00000000009", "date-confirmed", ItemPending)
			plantItemColumn(t, root, "c00000000001", "d00000000003", "b00000000001")
			opened, err := Open(root)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			card, err := opened.LoadCardIn(opened.CardsRoot(), "c00000000001")
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			missing, err := opened.MissingStandingItems(card, opened.Column("b00000000001"))
			if err != nil {
				t.Fatalf("missing: %v", err)
			}
			if !reflect.DeepEqual(missing, wellFormedEntries[1:]) {
				t.Errorf("wanted the two entries the card carries no instance of, got %+v", missing)
			}
		})
	}
}

// TestCheckReportsAMissingStandingItemOnlyWhereTheCardStands drives the
// finding at cleanup severity: one line per (card, column, key) for a card
// standing in the declaring column, none for a card standing elsewhere, and
// none for a key the card carries an instance of.
func TestCheckReportsAMissingStandingItemOnlyWhereTheCardStands(t *testing.T) {
	root := newFixture(t)
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), declaringColumn)
	write(t, filepath.Join(root, ColumnsDir, "b00000000002", ColumnAnchor), strings.Replace(columnDefinition, "Only", "Other", 1))
	write(t, filepath.Join(root, WorkbenchAnchor), strings.Replace(registryBenchDefinition, "  - b00000000001\n", "  - b00000000001\n  - b00000000002\n", 1))
	plantCard(t, root, "c00000000002", 2)
	write(t, filepath.Join(root, CardsDir, "c00000000002", CardAnchor), strings.Replace(cleanCard, "b00000000001", "b00000000002", 1))
	plantStandingInstance(t, root, "c00000000001", "d00000000001", "b00000000001", "contract-countersigned", ItemFailed)
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	findings := declaredFindings(t, opened, FindingStandingItemMissing)
	if len(findings) != 2 {
		t.Fatalf("check reported %d missing instances, wanted 2: %+v", len(findings), findings)
	}
	want := []string{"fx-1 only deposit-paid", "fx-1 only date-confirmed"}
	for at, finding := range findings {
		if finding.Detail != want[at] {
			t.Errorf("finding %d reads %q, wanted %q", at, finding.Detail, want[at])
		}
		if SeverityOf(finding) != SeverityCleanup {
			t.Errorf("finding %d carries severity %q, wanted cleanup", at, finding.Severity)
		}
		if finding.Path != filepath.Join(root, CardsDir, "c00000000001", CardAnchor) {
			t.Errorf("finding %d names %s, wanted the card's anchor", at, finding.Path)
		}
	}
}

// TestCheckReportsADeclaringColumnHoldingOnEntryAlone drives the entry-hold
// finding across all five spellings of gate_items, and across a column at
// true that declares nothing, which is the case that tells a sweep reading the
// declaration from one reading the hold alone.
func TestCheckReportsADeclaringColumnHoldingOnEntryAlone(t *testing.T) {
	for _, c := range []struct {
		hold     string
		declares bool
		reported bool
	}{
		{"true", true, true},
		{"out", true, false},
		{"both", true, false},
		{"false", true, false},
		{"", true, false},
		{"true", false, false},
	} {
		name := c.hold + "/declares=" + map[bool]string{true: "yes", false: "no"}[c.declares]
		t.Run(name, func(t *testing.T) {
			root := newFixture(t)
			anchor := columnDefinition
			if c.declares {
				anchor = strings.Replace(declaringColumn, "gate_items: out\n", "", 1)
			}
			if c.hold != "" {
				anchor = strings.Replace(anchor, "kind: work\n", "kind: work\ngate_items: "+c.hold+"\n", 1)
			}
			write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), anchor)
			opened, err := Open(root)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			findings := declaredFindings(t, opened, FindingStandingItemsEntryHold)
			if c.reported != (len(findings) == 1) {
				t.Fatalf("wanted reported=%v, got %+v", c.reported, findings)
			}
			if c.reported {
				if findings[0].Detail != "only" || SeverityOf(findings[0]) != SeverityCleanup {
					t.Errorf("the finding reads %+v, wanted the column reference at cleanup severity", findings[0])
				}
			}
		})
	}
}

// TestCheckReportsAnEvidenceSchemeTheBlockDoesNotDeclare drives the two
// shapes of the finding, told apart by their detail, and the two silences: a
// scheme the block declares, and a workbench declaring no block at all.
func TestCheckReportsAnEvidenceSchemeTheBlockDoesNotDeclare(t *testing.T) {
	plant := func(t *testing.T, block string) (*Bench, string) {
		t.Helper()
		root := newFixture(t)
		write(t, filepath.Join(root, WorkbenchAnchor), strings.Replace(registryBenchDefinition, "columns:\n", block+"columns:\n", 1))
		write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor),
			strings.Replace(declaringColumn, "    text: Has the vendor confirmed the wedding date in writing?\n",
				"    text: Has the vendor confirmed the wedding date in writing?\n    evidence: invoice\n", 1))
		plantStandingInstance(t, root, "c00000000001", "d00000000001", "b00000000001", "deposit-paid", ItemPending)
		demanding := plantStandingInstance(t, root, "c00000000001", "d00000000002", "b00000000001", "date-confirmed", ItemPending)
		text := strings.Replace(mustRead(t, demanding), "standing: date-confirmed\n", "standing: date-confirmed\nevidence: invoice\n", 1)
		write(t, demanding, text)
		opened, err := Open(root)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		return opened, demanding
	}

	opened, demanding := plant(t, "evidence:\n  receipt: The receipt's number.\n")
	findings := declaredFindings(t, opened, FindingEvidenceSchemeUndeclared)
	if len(findings) != 2 {
		t.Fatalf("check reported %d undeclared schemes, wanted 2: %+v", len(findings), findings)
	}
	byDetail := map[string]Finding{}
	for _, finding := range findings {
		byDetail[finding.Detail] = finding
		if SeverityOf(finding) != SeverityCleanup {
			t.Errorf("%q carries severity %q, wanted cleanup", finding.Detail, finding.Severity)
		}
	}
	if _, found := byDetail["only date-confirmed invoice"]; !found {
		t.Errorf("the column entry demanding invoice was not reported: %+v", findings)
	}
	if finding, found := byDetail["fx-1 d00000000002 invoice"]; !found {
		t.Errorf("the item demanding invoice was not reported: %+v", findings)
	} else if finding.Path != demanding {
		t.Errorf("the item finding names %s, wanted %s", finding.Path, demanding)
	}

	opened, _ = plant(t, "")
	if found := declaredFindings(t, opened, FindingEvidenceSchemeUndeclared); len(found) != 0 {
		t.Errorf("a workbench declaring no evidence block reports %+v", found)
	}
}

// TestTheItemColumnSweepPassesOverAWithdrawnItemAlone drives dinah-593's
// narrowing of check.item-column-unresolved: a withdrawn item is absent from
// the report whatever its column names, the five other states are reported
// exactly as before, and a withdrawn item returned to pending is reported on
// the next check. Each state is planted against the same unresolvable value,
// so the only thing that varies between the reported and the passed-over is
// the state.
func TestTheItemColumnSweepPassesOverAWithdrawnItemAlone(t *testing.T) {
	root := newFixture(t)
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), declaringColumn)
	reported := map[string]bool{}
	for at, state := range []string{ItemPending, ItemResolved, ItemVerified, ItemFailed, ItemWaived} {
		reported[plantItemState(t, root, "c00000000001", "d0000000000"+string(rune('1'+at)), "no-such-column", state)] = true
	}
	// The three withdrawn shapes: naming no column, naming the column by
	// its slug, and a standing instance naming a retired identifier.
	plantItemState(t, root, "c00000000001", "d00000000006", "", ItemWithdrawn)
	bySlug := plantItemState(t, root, "c00000000001", "d00000000007", "only", ItemWithdrawn)
	retired := plantStandingInstance(t, root, "c00000000001", "d00000000008", "b00000000009", "deposit-paid", ItemWithdrawn)
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	findings := declaredFindings(t, opened, FindingItemColumnUnresolved)
	if len(findings) != len(reported) {
		t.Fatalf("the sweep reported %d items, wanted %d: %+v", len(findings), len(reported), findings)
	}
	for _, finding := range findings {
		if !reported[finding.Path] {
			t.Errorf("the sweep reported %s, which is one of the withdrawn items", finding.Path)
		}
	}

	// Reopening returns the item to the report, since reopening re-imposes
	// the obligation the withdrawal ended.
	for _, path := range []string{bySlug, retired} {
		write(t, path, strings.Replace(mustRead(t, path), "state: withdrawn\n", "state: pending\n", 1))
	}
	opened, err = Open(root)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if found := declaredFindings(t, opened, FindingItemColumnUnresolved); len(found) != len(reported)+2 {
		t.Errorf("after the reopen the sweep reported %d items, wanted %d", len(found), len(reported)+2)
	}
}

// TestStandingItemsTravelThroughTheInterchange drives CORE-JSON-14 as Dinah
// reads it: the export carries the member as an ordered object of the entry
// keys, a workbench instantiated from that export declares the same entries,
// its own export is byte-identical to the first, and a column declaring
// nothing exports no member.
func TestStandingItemsTravelThroughTheInterchange(t *testing.T) {
	root := newFixture(t)
	write(t, filepath.Join(root, ColumnsDir, "b00000000001", ColumnAnchor), declaringColumn)
	opened, err := Open(root)
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
	var columns []map[string]json.RawMessage
	if err := json.Unmarshal(object["columns"], &columns); err != nil {
		t.Fatalf("read the columns: %v", err)
	}
	raw, carried := columns[0][StandingItemsKey]
	if !carried {
		t.Fatalf("the column object carries no %s member: %s", StandingItemsKey, columns[0])
	}
	if got := string(raw); strings.Index(got, "deposit-paid") > strings.Index(got, "contract-countersigned") ||
		strings.Index(got, "contract-countersigned") > strings.Index(got, "date-confirmed") {
		t.Errorf("the exported declaration reorders its entries: %s", got)
	}
	var entries map[string]map[string]string
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("the member is not an object of objects: %s", raw)
	}
	if len(entries) != 3 || entries["deposit-paid"]["evidence"] != "receipt" || entries["contract-countersigned"]["owner"] != "operator" || entries["date-confirmed"]["kind"] != "open_question" {
		t.Errorf("the member carries %v", entries)
	}
	if _, present := entries["date-confirmed"]["owner"]; present {
		t.Errorf("an entry declaring no owner exports one: %v", entries["date-confirmed"])
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
	if !reflect.DeepEqual(cloned.Columns[0].StandingItems, wellFormedEntries) {
		t.Errorf("the clone declares %+v, wanted %+v", cloned.Columns[0].StandingItems, wellFormedEntries)
	}
	if len(cloned.Columns[0].MalformedStandingItems) != 0 {
		t.Errorf("the clone refuses %v", cloned.Columns[0].MalformedStandingItems)
	}
	second, err := cloned.Export()
	if err != nil {
		t.Fatalf("export the clone: %v", err)
	}
	// The workbench object of a clone stamps the build's own profile where
	// the fixture declares an older one, so the columns are what the round
	// trip is held to, byte for byte.
	secondObject := map[string]json.RawMessage{}
	if err := json.Unmarshal(second, &secondObject); err != nil {
		t.Fatalf("read the clone's export: %v", err)
	}
	if !bytes.Equal(object["columns"], secondObject["columns"]) {
		t.Errorf("the clone's columns differ from the first export:\n%s\n---\n%s", object["columns"], secondObject["columns"])
	}

	bare, err := Open(newFixture(t))
	if err != nil {
		t.Fatalf("open the bare workbench: %v", err)
	}
	bareExport, err := bare.Export()
	if err != nil {
		t.Fatalf("export the bare workbench: %v", err)
	}
	if bytes.Contains(bareExport, []byte(StandingItemsKey)) {
		t.Errorf("a column declaring nothing exports a %s member", StandingItemsKey)
	}

	// The shapes a schema-free reading of the line would misread. A text
	// opening and closing with a bracket read as a flow sequence on the
	// round the reviewer drove on dinah-593, so the export wrote an array,
	// the import wrote a dashed list and the clone lost the entry; a text of
	// bare digits read as a number; and a quoted text was read away to its
	// bare spelling before either rule ran. Each is held to the whole trip:
	// the export carries the text as a JSON string, the clone declares the
	// entry with its text intact, and the clone's export is the first one.
	hostile := strings.Replace(declaringColumn, "  date-confirmed:\n", `  draft-date:
    kind: open_question
    text: [Draft] confirm the date [urgent]
  count-only:
    kind: decision
    text: 12
  quoted-flow:
    kind: decision
    text: "[a, b]"
  date-confirmed:
`, 1)
	hostileRoot := newFixture(t)
	write(t, filepath.Join(hostileRoot, ColumnsDir, "b00000000001", ColumnAnchor), hostile)
	source, err := Open(hostileRoot)
	if err != nil {
		t.Fatalf("open the hostile workbench: %v", err)
	}
	wantHostile := []StandingItem{
		wellFormedEntries[0], wellFormedEntries[1],
		{Key: "draft-date", Kind: "open_question", Text: "[Draft] confirm the date [urgent]"},
		{Key: "count-only", Kind: "decision", Text: "12"},
		{Key: "quoted-flow", Kind: "decision", Text: "[a, b]"},
		wellFormedEntries[2],
	}
	if !reflect.DeepEqual(source.Columns[0].StandingItems, wantHostile) {
		t.Fatalf("the source declares %+v, wanted %+v", source.Columns[0].StandingItems, wantHostile)
	}
	hostileExport, err := source.Export()
	if err != nil {
		t.Fatalf("export the hostile workbench: %v", err)
	}
	hostileObject := map[string]json.RawMessage{}
	if err := json.Unmarshal(hostileExport, &hostileObject); err != nil {
		t.Fatalf("read the hostile export: %v", err)
	}
	var hostileColumns []map[string]json.RawMessage
	if err := json.Unmarshal(hostileObject["columns"], &hostileColumns); err != nil {
		t.Fatalf("read the hostile columns: %v", err)
	}
	var hostileEntries map[string]map[string]string
	if err := json.Unmarshal(hostileColumns[0][StandingItemsKey], &hostileEntries); err != nil {
		t.Fatalf("a member of the hostile export is not a string: %s", hostileColumns[0][StandingItemsKey])
	}
	for _, entry := range wantHostile[2:5] {
		if got := hostileEntries[entry.Key]["text"]; got != entry.Text {
			t.Errorf("the export carries %s's text as %q, wanted %q", entry.Key, got, entry.Text)
		}
	}
	hostileDefinition, err := ReadDefinition(hostileExport)
	if err != nil {
		t.Fatalf("read the hostile definition: %v", err)
	}
	hostileClone := containedPath(t.TempDir())
	if err := Instantiate(hostileClone, "fx", "alka", hostileDefinition); err != nil {
		t.Fatalf("instantiate the hostile clone: %v", err)
	}
	clonedHostile, err := Open(hostileClone)
	if err != nil {
		t.Fatalf("open the hostile clone: %v", err)
	}
	if !reflect.DeepEqual(clonedHostile.Columns[0].StandingItems, wantHostile) {
		t.Errorf("the hostile clone declares %+v, wanted %+v", clonedHostile.Columns[0].StandingItems, wantHostile)
	}
	if len(clonedHostile.Columns[0].MalformedStandingItems) != 0 {
		t.Errorf("the hostile clone refuses %v", clonedHostile.Columns[0].MalformedStandingItems)
	}
	secondHostile, err := clonedHostile.Export()
	if err != nil {
		t.Fatalf("export the hostile clone: %v", err)
	}
	secondHostileObject := map[string]json.RawMessage{}
	if err := json.Unmarshal(secondHostile, &secondHostileObject); err != nil {
		t.Fatalf("read the hostile clone's export: %v", err)
	}
	if !bytes.Equal(hostileObject["columns"], secondHostileObject["columns"]) {
		t.Errorf("the hostile clone's columns differ from the first export:\n%s\n---\n%s", hostileObject["columns"], secondHostileObject["columns"])
	}
}

// TestAddStandingItemWritesWhatTheEntryDeclaresAndNoMore holds the anchor a
// minting writes to the contract's own example: the declaring column, the
// standing key and the entry's members, with the owner and the evidence keys
// present exactly where the entry declares them.
func TestAddStandingItemWritesWhatTheEntryDeclaresAndNoMore(t *testing.T) {
	root := newFixture(t)
	cardDir := filepath.Join(root, CardsDir, "c00000000001")
	for at, entry := range wellFormedEntries {
		item, err := AddStandingItem(cardDir, "b00000000001", entry, "2026-09-24T09:14:02Z")
		if err != nil {
			t.Fatalf("mint %s: %v", entry.Key, err)
		}
		loaded, err := LoadItem(item.Dir)
		if err != nil {
			t.Fatalf("load %s: %v", entry.Key, err)
		}
		want := &Item{ID: item.ID, Dir: item.Dir, Kind: entry.Kind, State: ItemPending, Ordinal: at + 1, Column: "b00000000001", Owner: entry.Owner, Standing: entry.Key, Evidence: entry.Evidence, Text: entry.Text}
		if !reflect.DeepEqual(loaded, want) {
			t.Errorf("the minted %s reads back as %+v, wanted %+v", entry.Key, loaded, want)
		}
		text := mustRead(t, filepath.Join(item.Dir, ItemAnchor))
		if (entry.Owner == "") == strings.Contains(text, "owner:") || (entry.Evidence == "") == strings.Contains(text, "evidence:") {
			t.Errorf("the anchor of %s carries a key its entry does not declare, or lacks one it does:\n%s", entry.Key, text)
		}
	}
	items, err := Items(cardDir)
	if err != nil {
		t.Fatalf("items: %v", err)
	}
	if len(items) != len(wellFormedEntries) {
		t.Fatalf("the card carries %d items, wanted %d", len(items), len(wellFormedEntries))
	}
	// The claim refusal reads the minted question exactly as a hand-filed
	// one: it names a declared column, so the column's hold owns it.
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if opened.ItemBlocksClaim(items[2]) {
		t.Errorf("a minted question naming a declared column refuses the claim")
	}
}
