package main

import (
	"sort"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/guide"
)

// parseReferencesGuideFieldTable reads the "Which fields a kind has" table out
// of the shipped references guide and returns, per kind, the field names its
// row draws.
//
// It parses the table the way parseReferencesGuideTable at
// cmd/dinah/references_command_resolution_test.go:161 parses the command
// table: find the heading row, skip the rule under it, and read rows until the
// first line that is not a table row. A parser whose heading has moved reads
// nothing, which is why the caller is fatal on an empty answer rather than
// passing over a table it never found.
func parseReferencesGuideFieldTable(t *testing.T) map[string][]string {
	t.Helper()
	text, err := guide.Text("references")
	if err != nil {
		t.Fatalf("guide references: %v", err)
	}
	lines := strings.Split(text, "\n")
	headerAt := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "| Kind") {
			headerAt = i
			break
		}
	}
	if headerAt < 0 {
		t.Fatal("the references guide carries no \"| Kind\" table header; this parser and the guide have drifted apart")
	}
	drawn := map[string][]string{}
	for _, line := range lines[headerAt+2:] {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			break
		}
		row := cells(line)
		if len(row) != 2 {
			t.Fatalf("the references guide's field row %q carries %d cells, wanted two", trimmed, len(row))
		}
		var names []string
		for _, name := range strings.Split(row[1], ",") {
			name = strings.Trim(strings.TrimSpace(name), "`")
			if name != "" {
				names = append(names, name)
			}
		}
		drawn[row[0]] = names
	}
	return drawn
}

// TestTheReferencesGuideNamesEveryFieldOfEveryKind holds the guide's per-kind
// field table to what the tool records, in four directions: a kind the table
// draws no row for, a row naming a kind the grammar does not carry, a field
// the row omits, and a field the row carries that the kind does not record.
//
// The table is hand-written markdown rather than generated, deliberately.
// Nothing in this tree generates a guide, and a table compared against the
// generator it came from passes on every input including a wrong one, so the
// only input this check reads that a person writes is the table itself. That
// is what lets it fail against correct code only when the table is wrong.
//
// What it does not pin is the content of bench.FieldsOf, since the expectation
// is read from exactly that call. AC-1's two-directional sample table is what
// pins that, and the next reader should not read this check as covering it.
func TestTheReferencesGuideNamesEveryFieldOfEveryKind(t *testing.T) {
	drawn := parseReferencesGuideFieldTable(t)
	if len(drawn) == 0 {
		t.Fatal("the references guide's field table draws no row, so this check read nothing")
	}
	kinds := bench.EntityKinds()
	if len(kinds) == 0 {
		t.Fatal("the grammar names no kind, so this check read nothing")
	}
	known := map[string]bool{}
	for _, kind := range kinds {
		known[kind] = true
		row, drew := drawn[kind]
		if !drew {
			t.Errorf("the grammar names the kind %s and the guide's field table draws no row for it", kind)
			continue
		}
		carried := map[string]bool{}
		for _, name := range row {
			carried[name] = true
		}
		for _, name := range bench.FieldsOf(kind) {
			if !carried[name] {
				t.Errorf("a %s records the field %s and the guide's row for it does not name that field", kind, name)
			}
		}
		records := map[string]bool{}
		for _, name := range bench.FieldsOf(kind) {
			records[name] = true
		}
		for _, name := range row {
			if !records[name] {
				t.Errorf("the guide's %s row names the field %s and a %s does not record it", kind, name, kind)
			}
		}
	}
	for kind := range drawn {
		if !known[kind] {
			t.Errorf("the guide's field table draws a row for %s, which the grammar does not name", kind)
		}
	}
}

// TestTheGuideSaysWhichFieldsTheWorkbenchListingLeavesOut asserts the one
// sentence that stops a reader meeting a difference nobody explained: the
// workbench records four fields and the bare listing prints three, and the
// guide says so rather than leaving that to be noticed.
func TestTheGuideSaysWhichFieldsTheWorkbenchListingLeavesOut(t *testing.T) {
	fields := bench.FieldsOf(bench.KindWorkbench)
	listed := bench.WorkbenchListingFields
	if len(fields) == 0 || len(listed) == 0 {
		t.Fatal("one of the two sets is empty, so this check read nothing")
	}
	var withheld []string
	shown := map[string]bool{}
	for _, name := range listed {
		shown[name] = true
	}
	for _, name := range fields {
		if !shown[name] {
			withheld = append(withheld, name)
		}
	}
	sort.Strings(withheld)
	if len(withheld) == 0 {
		t.Skip("the listing prints every field the workbench records, so there is nothing for the guide to explain")
	}
	text, err := guide.Text("references")
	if err != nil {
		t.Fatalf("guide references: %v", err)
	}
	folded := strings.Join(strings.Fields(text), " ")
	for _, name := range withheld {
		if !strings.Contains(folded, "leaves `"+name+"` out") {
			t.Errorf("the workbench listing withholds %s and the guide does not say so", name)
		}
	}
}
