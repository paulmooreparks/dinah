package bench

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rejectDefinition is a two-column flow whose second column declares reject_to
// naming the first. It is the interchange round trip's fixture, written in the
// shape waitingDefinition already uses so the two read alike.
const rejectDefinition = `{
  "profile": "dinah-core/0.7",
  "title": "Rejecting",
  "columns": [
    { "id": "e00000000001", "title": "Editing", "kind": "work",
      "instructions": "Editing instructions.\n" },
    { "id": "e00000000002", "title": "Accepting", "kind": "work",
      "reject_to": "e00000000001", "instructions": "Accepting instructions.\n" }
  ]
}`

// TestReadColumnCarriesTheDeclaredRejectTo is dinah-207 AC-1. readColumn takes
// the declaration verbatim and refuses nothing, because a stale cross-reference
// is a thing dinah check reports rather than a reason to take a whole board
// away from the person who has to repair it.
func TestReadColumnCarriesTheDeclaredRejectTo(t *testing.T) {
	t.Run("a column declaring reject_to carries the ref it declared", func(t *testing.T) {
		opened, err := Open(kindFixture(t, []map[string]any{
			{"id": "a00000000001", "title": "Editing", "slug": "editing", "kind": "work"},
			{"id": "a00000000002", "title": "Accepting", "slug": "accepting", "kind": "work", "reject_to": "editing"},
		}))
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		if got := opened.ColumnByRef("accepting").RejectTo; got != "editing" {
			t.Errorf("wanted the declared ref editing, got %q", got)
		}
	})

	t.Run("a column declaring none reads empty", func(t *testing.T) {
		opened, err := Open(kindFixture(t, []map[string]any{
			{"id": "a00000000001", "title": "Editing", "slug": "editing", "kind": "work"},
			{"id": "a00000000002", "title": "Accepting", "slug": "accepting", "kind": "work"},
		}))
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		if got := opened.ColumnByRef("accepting").RejectTo; got != "" {
			t.Errorf("wanted the empty string on a column declaring nothing, got %q", got)
		}
	})

	t.Run("a ref naming no column opens the workbench anyway", func(t *testing.T) {
		opened, err := Open(kindFixture(t, []map[string]any{
			{"id": "a00000000001", "title": "Only", "slug": "only", "kind": "work", "reject_to": "nowhere at all"},
		}))
		if err != nil {
			t.Fatalf("a declaration naming no column should open, got %v", err)
		}
		if got := opened.ColumnByRef("only").RejectTo; got != "nowhere at all" {
			t.Errorf("wanted the ref stored verbatim, got %q", got)
		}
	})
}

// TestRejectTargetResolvesUnknownAndSelf is dinah-207 AC-2. The resolver is
// deliberately coarse: one nil covers the nil column, the empty declaration, the
// unknown ref and the self-naming ref, because its two callers only need to
// know whether to act.
func TestRejectTargetResolvesUnknownAndSelf(t *testing.T) {
	editing := &Column{ID: "e00000000001", Title: "Editing", Slug: "editing", Kind: "work", Position: 0}
	accepting := &Column{ID: "e00000000002", Title: "Accepting", Slug: "accepting", Kind: "work", Position: 1}
	b := &Bench{Columns: []*Column{editing, accepting}}

	cases := []struct {
		name    string
		column  *Column
		declare string
		want    *Column
	}{
		{name: "a nil column", column: nil, want: nil},
		{name: "an empty declaration", column: accepting, declare: "", want: nil},
		{name: "a ref naming no column", column: accepting, declare: "nowhere", want: nil},
		{name: "a ref naming the declaring column", column: accepting, declare: "accepting", want: nil},
		{name: "a ref naming a sibling", column: accepting, declare: "editing", want: editing},
		{name: "a ref naming a sibling by identifier", column: accepting, declare: "e00000000001", want: editing},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			if one.column != nil {
				one.column.RejectTo = one.declare
				defer func() { one.column.RejectTo = "" }()
			}
			if got := b.RejectTarget(one.column); got != one.want {
				t.Errorf("wanted %v, got %v", one.want, got)
			}
		})
	}
}

// TestCheckReportsAnUnknownRejectTarget is dinah-207 AC-3.
func TestCheckReportsAnUnknownRejectTarget(t *testing.T) {
	opened, err := Open(kindFixture(t, []map[string]any{
		{"id": "a00000000001", "title": "Editing", "slug": "editing", "kind": "work"},
		{"id": "a00000000002", "title": "Accepting", "slug": "accepting", "kind": "work", "reject_to": "nowhere"},
	}))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	assertFindingNames(t, opened, FindingRejectTargetUnknown, "accepting")
}

// TestCheckReportsASelfRejectTarget is dinah-207 AC-4.
func TestCheckReportsASelfRejectTarget(t *testing.T) {
	opened, err := Open(kindFixture(t, []map[string]any{
		{"id": "a00000000001", "title": "Editing", "slug": "editing", "kind": "work"},
		{"id": "a00000000002", "title": "Accepting", "slug": "accepting", "kind": "work", "reject_to": "accepting"},
	}))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	assertFindingNames(t, opened, FindingRejectTargetIsSelf, "accepting")
}

// TestCheckReportsAForwardRejectTargetUnlessItIsDone is dinah-207 AC-5, and the
// done-column subtest is where the operator's ruling on that card lands: a
// rejected card ends in the same done queue a finished one ends in, so naming
// the terminal is legal and nothing is reported.
func TestCheckReportsAForwardRejectTargetUnlessItIsDone(t *testing.T) {
	t.Run("a forward target that is an ordinary station is reported", func(t *testing.T) {
		opened, err := Open(kindFixture(t, []map[string]any{
			{"id": "a00000000001", "title": "Editing", "slug": "editing", "kind": "work", "reject_to": "accepting"},
			{"id": "a00000000002", "title": "Accepting", "slug": "accepting", "kind": "work"},
			{"id": "a00000000003", "title": "Finished", "slug": "finished", "kind": "done"},
		}))
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		assertFindingNames(t, opened, FindingRejectTargetForward, "editing")
	})

	t.Run("a forward target that is a done column is not reported", func(t *testing.T) {
		opened, err := Open(kindFixture(t, []map[string]any{
			{"id": "a00000000001", "title": "Editing", "slug": "editing", "kind": "work", "reject_to": "finished"},
			{"id": "a00000000002", "title": "Accepting", "slug": "accepting", "kind": "work"},
			{"id": "a00000000003", "title": "Finished", "slug": "finished", "kind": "done"},
		}))
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		assertNoRejectFinding(t, opened, "a rejection naming the terminal")
	})

	t.Run("a backward target is not reported", func(t *testing.T) {
		opened, err := Open(kindFixture(t, []map[string]any{
			{"id": "a00000000001", "title": "Editing", "slug": "editing", "kind": "work"},
			{"id": "a00000000002", "title": "Accepting", "slug": "accepting", "kind": "work", "reject_to": "editing"},
			{"id": "a00000000003", "title": "Finished", "slug": "finished", "kind": "done"},
		}))
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		assertNoRejectFinding(t, opened, "the motivating case")
	})
}

// assertNoRejectFinding fails when check reports anything under one of the
// three reject_to keys, which is how a negative case is asserted without
// naming a key it might be reported under and missing the other two.
func assertNoRejectFinding(t *testing.T, b *Bench, what string) {
	t.Helper()
	findings, err := b.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	for _, finding := range findings {
		switch finding.Key {
		case FindingRejectTargetForward, FindingRejectTargetUnknown, FindingRejectTargetIsSelf:
			t.Errorf("%s was reported: %+v", what, finding)
		}
	}
}

// TestRejectToRidesTheInterchange is dinah-207 AC-10. The round trip is what
// catches a field added to the parser and forgotten in exportColumn or in
// knownColumnKeys, and it passes here with interchange.go untouched because
// reject_to travels as an unrecognized member under CORE-JSON-7.
func TestRejectToRidesTheInterchange(t *testing.T) {
	first := filepath.Join(t.TempDir(), "first")
	definition, err := ReadDefinition([]byte(rejectDefinition))
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	if err := Instantiate(first, "rj", "alka", definition); err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	opened, err := Open(first)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	exported, err := opened.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	columns := exportedColumns(t, exported)
	if len(columns) != 2 {
		t.Fatalf("wanted two columns in the export, got %d", len(columns))
	}
	if _, carried := columns[0]["reject_to"]; carried {
		t.Error("the export carried the member on a column that does not declare it")
	}
	raw, carried := columns[1]["reject_to"]
	if !carried {
		t.Fatal("the export left the member off the column that declares it")
	}
	var ref string
	if err := json.Unmarshal(raw, &ref); err != nil || ref != "e00000000001" {
		t.Errorf("the member exported as %s, wanted e00000000001", raw)
	}

	// init --from reads the export back, which is the import half, and an
	// export of the result matching the first byte for byte is what proves
	// nothing was dropped or invented in between.
	second := filepath.Join(t.TempDir(), "second")
	reread, err := ReadDefinition(exported)
	if err != nil {
		t.Fatalf("read the export back: %v", err)
	}
	if err := Instantiate(second, "rj", "alka", reread); err != nil {
		t.Fatalf("instantiate the import: %v", err)
	}
	imported, err := Open(second)
	if err != nil {
		t.Fatalf("open the import: %v", err)
	}
	if got := imported.Column("e00000000002").RejectTo; got != "e00000000001" {
		t.Errorf("the import reproduced the declaration as %q", got)
	}
	if got := imported.Column("e00000000001").RejectTo; got != "" {
		t.Errorf("the import invented a declaration on a column that carries none: %q", got)
	}
	if target := imported.RejectTarget(imported.Column("e00000000002")); target == nil || target.ID != "e00000000001" {
		t.Errorf("the imported declaration does not resolve, got %v", target)
	}
	again, err := imported.Export()
	if err != nil {
		t.Fatalf("export the import: %v", err)
	}
	if string(again) != string(exported) {
		t.Errorf("the round trip is not byte for byte:\nfirst:\n%s\nsecond:\n%s", exported, again)
	}

	// The anchor on disk carries the key as the frontmatter it was declared
	// in, rather than as a preserved unknown member with a JSON value.
	text, err := os.ReadFile(filepath.Join(second, ColumnsDir, "e00000000002", ColumnAnchor))
	if err != nil {
		t.Fatalf("read the imported anchor: %v", err)
	}
	if !strings.Contains(string(text), "reject_to: e00000000001") {
		t.Errorf("the imported anchor reads:\n%s", text)
	}
}
