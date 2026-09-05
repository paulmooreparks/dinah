package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// reshapeSourceFrom exports the workbench's own definition, hands it to edit,
// and writes the result to a file outside the workbench, which is how a person
// arrives at a new shape: the identifiers of the columns they keep come from
// the workbench they already have.
//
// The identifiers a `dinah init` mints are drawn rather than fixed, so a
// definition written out by hand could not name them. Exporting and editing is
// what a real operator does for the same reason.
func reshapeSourceFrom(t *testing.T, root string, edit func(columns []map[string]any) []map[string]any) string {
	t.Helper()
	exported := runCLI(t, root, "export")
	if exported.code != 0 {
		t.Fatalf("export: %d %s", exported.code, exported.errw)
	}
	var definition map[string]any
	if err := json.Unmarshal([]byte(exported.out), &definition); err != nil {
		t.Fatalf("read the exported definition: %v", err)
	}
	var columns []map[string]any
	raw, err := json.Marshal(definition["columns"])
	if err != nil {
		t.Fatalf("re-encode the columns: %v", err)
	}
	if err := json.Unmarshal(raw, &columns); err != nil {
		t.Fatalf("read the columns: %v", err)
	}
	definition["columns"] = edit(columns)
	body, err := json.MarshalIndent(definition, "", "  ")
	if err != nil {
		t.Fatalf("write the definition: %v", err)
	}
	path := filepath.Join(t.TempDir(), "shape.json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// without answers the columns with the one carrying a title left out.
func without(columns []map[string]any, title string) []map[string]any {
	kept := make([]map[string]any, 0, len(columns))
	for _, column := range columns {
		if column["title"] == title {
			continue
		}
		kept = append(kept, column)
	}
	return kept
}

// idOf answers the identifier of the column carrying a title.
func idOf(t *testing.T, columns []map[string]any, title string) string {
	t.Helper()
	for _, column := range columns {
		if column["title"] == title {
			return column["id"].(string)
		}
	}
	t.Fatalf("the definition carries no column titled %s", title)
	return ""
}

// TestHelpReshapeDocumentsItsRefusalsAndNamesDinahCheck is dinah-316 AC-13's
// help half, and the half of AC-9 a test can hold. The page is generated from
// the verb's own declarations, so a refusal the verb can raise and the page
// does not list is a page that has drifted from the behaviour, and the whole
// point of naming `dinah check` in the --map row is that an operator meets the
// two-command workflow rather than guessing the second command exists.
func TestHelpReshapeDocumentsItsRefusalsAndNamesDinahCheck(t *testing.T) {
	root := newBench(t)
	page := runCLI(t, root, "help", "reshape")
	if page.code != 0 {
		t.Fatalf("help reshape: %d %s", page.code, page.errw)
	}
	for _, refusal := range []string{
		contract.ReshapeNeedsDestination,
		contract.ReshapeHeldCardInQueue,
		contract.ReshapeMapSourceEmpty,
		contract.ReshapeDestinationRetiring,
		contract.ReshapeDestinationAmbiguous,
		contract.UnknownColumn,
		contract.Malformed,
	} {
		if !strings.Contains(page.out, refusal) {
			t.Errorf("the page does not document the refusal %s:\n%s", refusal, page.out)
		}
	}
	if !strings.Contains(page.out, "dinah check") {
		t.Errorf("the page does not name dinah check, so nothing tells a reader where a stranded identifier is found:\n%s", page.out)
	}
	if !strings.Contains(page.out, "--map") || !strings.Contains(page.out, "--from") {
		t.Errorf("the page does not spell the arguments:\n%s", page.out)
	}
}

// TestTheReshapeReportReadsAsAPreviewAndThenAsAnApply is the worked transcript
// AC-9 puts in front of the operator, run as a test so that the page he reads
// in the pull request is the page the tool prints.
//
// The preview says at the top that nothing was written, names every column
// with what the run would do to it, and names the destination and the count
// for the retirement. The apply prints the same page with the opening line
// that says the shape is on disk.
func TestTheReshapeReportReadsAsAPreviewAndThenAsAnApply(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "a carried card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	carryToDoing(t, root, "fx-1")
	if got := runCLI(t, root, "block", "fx-1", "a vendor has not answered"); got.code != 0 {
		t.Fatalf("block: %d %s", got.code, got.errw)
	}

	var doingID, intakeID string
	source := reshapeSourceFrom(t, root, func(columns []map[string]any) []map[string]any {
		doingID = idOf(t, columns, "Doing")
		intakeID = idOf(t, columns, "Intake")
		return without(columns, "Doing")
	})

	preview := runCLI(t, root, "reshape", "--from", source, "--map", doingID+"="+intakeID)
	if preview.code != 0 {
		t.Fatalf("the preview: %d %s", preview.code, preview.errw)
	}
	for _, want := range []string{"Nothing was written", "kept Intake", "retired Doing", "Cards: 1", "Carried while blocked"} {
		if !strings.Contains(preview.out, want) {
			t.Errorf("the preview does not carry %q:\n%s", want, preview.out)
		}
	}
	if !strings.Contains(preview.out, "--yes") {
		t.Errorf("the preview does not name the flag that applies it:\n%s", preview.out)
	}
	// A preview writes nothing, so the card has not moved and the column is
	// still in the flow, which `dinah columns` is the reader's own way of
	// seeing.
	if listed := runCLI(t, root, "columns"); !strings.Contains(listed.out, "Doing") {
		t.Errorf("the preview retired a column:\n%s", listed.out)
	}

	applied := runCLI(t, root, "reshape", "--from", source, "--yes", "--map", doingID+"="+intakeID)
	if applied.code != 0 {
		t.Fatalf("the apply: %d %s", applied.code, applied.errw)
	}
	if !strings.Contains(applied.out, "now carries the new shape") {
		t.Errorf("the apply does not say the shape is on disk:\n%s", applied.out)
	}
	if strings.Contains(applied.out, "Nothing was written") {
		t.Errorf("the apply printed the preview's opening line:\n%s", applied.out)
	}
	if listed := runCLI(t, root, "columns"); strings.Contains(listed.out, "Doing") {
		t.Errorf("the retired column is still in the flow:\n%s", listed.out)
	}
	// The card came through blocked, which is what CORE-MOVE-8 requires of
	// any move and what the report warned the reader about.
	shown := runCLI(t, root, "show", "fx-1")
	if !strings.Contains(shown.out, "blocked") {
		t.Errorf("the carried card lost its block:\n%s", shown.out)
	}
	if checked := runCLI(t, root, "check"); checked.code != 0 {
		t.Errorf("the reshaped workbench does not check clean: %d %s\n%s", checked.code, checked.errw, checked.out)
	}
}

// TestAnEmptyRetirementNeedsNoDestination is the other half of the report: a
// column nothing stands in retires with no destination at all, so the run
// asks the operator for nothing and says so.
func TestAnEmptyRetirementNeedsNoDestination(t *testing.T) {
	root := newBench(t)
	source := reshapeSourceFrom(t, root, func(columns []map[string]any) []map[string]any {
		return without(columns, "Done")
	})

	preview := runCLI(t, root, "reshape", "--from", source)
	if preview.code != 0 {
		t.Fatalf("the preview: %d %s", preview.code, preview.errw)
	}
	if !strings.Contains(preview.out, "no card standing in it") {
		t.Errorf("the preview does not say the retirement is empty:\n%s", preview.out)
	}
	if applied := runCLI(t, root, "reshape", "--from", source, "--yes"); applied.code != 0 {
		t.Fatalf("the apply: %d %s", applied.code, applied.errw)
	}
	if listed := runCLI(t, root, "columns"); strings.Contains(listed.out, "Done") {
		t.Errorf("the empty column was not retired:\n%s", listed.out)
	}
}

// TestReshapeRefusesAnInvocationNamingNoSource is the arity check the command
// runs before it opens anything: --from is what the verb reads its new shape
// out of, and an invocation carrying none has named nothing to reshape to.
func TestReshapeRefusesAnInvocationNamingNoSource(t *testing.T) {
	root := newBench(t)
	got := runCLI(t, root, "reshape")
	if got.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("wanted the refusal exit code, got %d\n%s", got.code, got.out)
	}
	if !strings.HasPrefix(got.errw, contract.Usage+" ") {
		t.Errorf("wanted %s to lead stderr, got %q", contract.Usage, got.errw)
	}
}

// TestARepeatedMapIsReadOccurrenceByOccurrence is what --map needs that no
// other flag in the tool does. One run retires as many columns as the new
// definition drops, and the parser's own last-value-wins rule would carry the
// last entry and silently discard the rest, which is the shape a run would
// then refuse for a retirement whose destination the caller had actually
// named.
func TestARepeatedMapIsReadOccurrenceByOccurrence(t *testing.T) {
	root := newBench(t)
	for _, title := range []string{"first", "second"} {
		if got := runCLI(t, root, "add", title); got.code != 0 {
			t.Fatalf("add %s: %d %s", title, got.code, got.errw)
		}
	}
	carryToDoing(t, root, "fx-1")
	if got := runCLI(t, root, "move", "fx-2", "done"); got.code != 0 {
		t.Fatalf("move fx-2 to done: %d %s", got.code, got.errw)
	}

	var doingID, doneID, intakeID string
	source := reshapeSourceFrom(t, root, func(columns []map[string]any) []map[string]any {
		doingID = idOf(t, columns, "Doing")
		doneID = idOf(t, columns, "Done")
		intakeID = idOf(t, columns, "Intake")
		return without(without(columns, "Doing"), "Done")
	})

	applied := runCLI(t, root, "reshape", "--from", source, "--yes",
		"--map", doingID+"="+intakeID, "--map", doneID+"="+intakeID)
	if applied.code != 0 {
		t.Fatalf("the apply: %d %s\n%s", applied.code, applied.errw, applied.out)
	}
	listed := runCLI(t, root, "ls", "intake")
	for _, card := range []string{"fx-1", "fx-2"} {
		if !strings.Contains(listed.out, card) {
			t.Errorf("%s was not carried into the one surviving column:\n%s", card, listed.out)
		}
	}
}
