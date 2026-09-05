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

// withColumnBeforeDone answers the columns with one more work station spliced
// in ahead of the first done column, which is where a station may legally
// stand: nothing that is not done stands after a column that is.
func withColumnBeforeDone(columns []map[string]any, title string) []map[string]any {
	// The identifier is a label rather than a real one, which is what an
	// element a definition adds carries: the run derives the identifier the
	// column is written at from the source and the element's position. The
	// member itself is required, so the label cannot simply be left out.
	station := map[string]any{"id": strings.ToLower(title), "title": title, "kind": "work"}
	built := make([]map[string]any, 0, len(columns)+1)
	spliced := false
	for _, column := range columns {
		if !spliced && column["kind"] == "done" {
			built = append(built, station)
			spliced = true
		}
		built = append(built, column)
	}
	if !spliced {
		built = append(built, station)
	}
	return built
}

// soleCardDir answers the directory of the one card a workbench holds, which
// is where its lock file sits.
func soleCardDir(t *testing.T, root string) string {
	t.Helper()
	cards := filepath.Join(soleBenchDir(t, root), "cards")
	entries, err := os.ReadDir(cards)
	if err != nil {
		t.Fatalf("read %s: %v", cards, err)
	}
	if len(entries) != 1 {
		t.Fatalf("wanted one card under %s, got %d", cards, len(entries))
	}
	return filepath.Join(cards, entries[0].Name())
}

// TestAReshapeRefusedAfterItHadWrittenSaysSo is the transcript an operator
// meets when a reshape stops part way through, and it is the whole reason the
// report comes back with the error rather than being dropped for it.
//
// The run here is stopped by a lock another process holds on the one card the
// carry has to move, which is the likeliest late refusal there is and needs no
// unusual behaviour from anybody: an ordinary act on that card, in flight while
// the reshape runs, is enough. By the time the carry meets it, step one has
// written a column and put it in the flow.
//
// What the operator must be able to read off the output is that the workbench
// was written to, which part of the new shape landed, and that running the same
// command again finishes the job. A preview says nothing was written and an
// applied run says the workbench carries the new shape, so neither of those
// lines may appear here.
func TestAReshapeRefusedAfterItHadWrittenSaysSo(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "a card the carry cannot take"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	carryToDoing(t, root, "fx-1")

	var doingID, intakeID string
	source := reshapeSourceFrom(t, root, func(columns []map[string]any) []map[string]any {
		doingID = idOf(t, columns, "Doing")
		intakeID = idOf(t, columns, "Intake")
		return withColumnBeforeDone(without(columns, "Doing"), "Triage")
	})

	// Another process is mid-transaction on the card the carry has to move.
	// The lock is written by hand because no verb leaves one standing, which
	// is the same reason the library's own tests plant them.
	lock := filepath.Join(soleCardDir(t, root), "lock")
	if err := os.WriteFile(lock, []byte(`{"actor":"brin","pid":4,"ts":"2026-01-01T00:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatalf("plant the lock: %v", err)
	}

	got := runCLI(t, root, "reshape", "--from", source, "--yes", "--map", doingID+"="+intakeID)
	if got.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("wanted the refusal exit code, got %d\n%s\n%s", got.code, got.out, got.errw)
	}
	// The refusal still leads stderr, which is what every caller reading the
	// leading token depends on, and the report is on stdout beside it.
	if !strings.HasPrefix(got.errw, contract.Locked+" ") {
		t.Errorf("wanted %s to lead stderr, got %q", contract.Locked, got.errw)
	}
	for _, want := range []string{"part way between its old shape and the new one", "now written and in the flow: 1", "run the same command again"} {
		if !strings.Contains(got.out, want) {
			t.Errorf("the report does not carry %q:\n%s", want, got.out)
		}
	}
	for _, unwanted := range []string{"Nothing was written", "now carries the new shape"} {
		if strings.Contains(got.out, unwanted) {
			t.Errorf("the half-applied run printed %q, which is a line for one of the other two states:\n%s", unwanted, got.out)
		}
	}
	// The report is describing something real: the added column is in the
	// flow, and the retirement the run did not reach is still there.
	listed := runCLI(t, root, "columns")
	for _, title := range []string{"Triage", "Doing"} {
		if !strings.Contains(listed.out, title) {
			t.Errorf("wanted %s in the flow after the half-applied run:\n%s", title, listed.out)
		}
	}

	// Clearing what the refusal names and running the same command again
	// finishes the rest, which is what the report told the operator to do.
	if err := os.Remove(lock); err != nil {
		t.Fatalf("release the lock: %v", err)
	}
	finished := runCLI(t, root, "reshape", "--from", source, "--yes", "--map", doingID+"="+intakeID)
	if finished.code != 0 {
		t.Fatalf("the second run: %d %s\n%s", finished.code, finished.errw, finished.out)
	}
	if !strings.Contains(finished.out, "now carries the new shape") {
		t.Errorf("the second run does not report the shape as applied:\n%s", finished.out)
	}
	if again := runCLI(t, root, "columns"); strings.Contains(again.out, "Doing") {
		t.Errorf("the second run did not finish the retirement:\n%s", again.out)
	}
	if checked := runCLI(t, root, "check"); checked.code != 0 {
		t.Errorf("the finished workbench does not check clean: %d %s\n%s", checked.code, checked.errw, checked.out)
	}
}

// TestTheMachineFormOfAHalfAppliedRunCarriesBothHalves is the same state read
// by a script rather than by a person. The machine answer is the report rather
// than the bare refusal, because a refusal document names what stopped the run
// and says nothing at all about what the run had already done to the workbench.
//
// Leaving the shared refusal document is what makes the rest of this test
// necessary. That document carries an outcome, a refusal name, a detail and a
// context, and a report answering in its place has to carry them too: the
// outcome because CORE-OUT-1 requires every answer to report one, and the
// detail because the report's own closing line tells its reader to clear what
// the refusal names, which is a card this document would otherwise never name.
// The applied run at the foot holds the other half of the outcome member,
// which belongs to every answer rather than to this path alone.
func TestTheMachineFormOfAHalfAppliedRunCarriesBothHalves(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "a card the carry cannot take"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	carryToDoing(t, root, "fx-1")

	var doingID, intakeID string
	source := reshapeSourceFrom(t, root, func(columns []map[string]any) []map[string]any {
		doingID = idOf(t, columns, "Doing")
		intakeID = idOf(t, columns, "Intake")
		return withColumnBeforeDone(without(columns, "Doing"), "Triage")
	})
	lock := filepath.Join(soleCardDir(t, root), "lock")
	if err := os.WriteFile(lock, []byte(`{"actor":"brin","pid":4,"ts":"2026-01-01T00:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatalf("plant the lock: %v", err)
	}

	got := runCLI(t, root, "reshape", "--from", source, "--yes", "--json", "--map", doingID+"="+intakeID)
	if got.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("wanted the refusal exit code, got %d\n%s\n%s", got.code, got.out, got.errw)
	}
	var answer struct {
		Outcome string `json:"outcome"`
		Applied bool   `json:"applied"`
		Refusal string `json:"refusal"`
		Detail  string `json:"detail"`
		Wrote   []struct {
			Step  string `json:"step"`
			Count int    `json:"count"`
		} `json:"wrote"`
	}
	if err := json.Unmarshal([]byte(got.out), &answer); err != nil {
		t.Fatalf("read the machine answer: %v\n%s", err, got.out)
	}
	if answer.Outcome != contract.OutcomeRefused {
		t.Errorf("wanted the half-applied answer to report the outcome %s, got %q", contract.OutcomeRefused, answer.Outcome)
	}
	if answer.Applied {
		t.Error("the half-applied run reported itself as applied")
	}
	if answer.Refusal != contract.Locked {
		t.Errorf("wanted the report to name %s, got %q", contract.Locked, answer.Refusal)
	}
	// The report tells its reader to clear what the refusal names, so it has
	// to name it. Whether that is a card, a column or the holder of a lock is
	// the refusal's own answer, and the test is that the machine form carries
	// the same token the person reading stderr is given, rather than sending a
	// script after something only the other stream identified.
	if answer.Detail != "brin" {
		t.Errorf("wanted the report to name what the refusal names, got %q", answer.Detail)
	}
	if !strings.Contains(got.errw, answer.Detail) {
		t.Errorf("the two streams name different things: %q is not in %q", answer.Detail, got.errw)
	}
	if len(answer.Wrote) != 1 || answer.Wrote[0].Step != "added" || answer.Wrote[0].Count != 1 {
		t.Errorf("wanted one added column recorded, got %+v", answer.Wrote)
	}

	// Clearing the lock and running the same command again finishes the
	// shape, and that answer reports an outcome of its own. The member is on
	// every answer rather than on the refusal path alone, so no caller has to
	// read anything into its absence.
	if err := os.Remove(lock); err != nil {
		t.Fatalf("release the lock: %v", err)
	}
	finished := runCLI(t, root, "reshape", "--from", source, "--yes", "--json", "--map", doingID+"="+intakeID)
	if finished.code != 0 {
		t.Fatalf("the second run: %d %s\n%s", finished.code, finished.errw, finished.out)
	}
	answer.Outcome, answer.Applied, answer.Refusal, answer.Detail, answer.Wrote = "", false, "", "", nil
	if err := json.Unmarshal([]byte(finished.out), &answer); err != nil {
		t.Fatalf("read the second machine answer: %v\n%s", err, finished.out)
	}
	if answer.Outcome != contract.OutcomeOK {
		t.Errorf("wanted the applied run to report the outcome %s, got %q", contract.OutcomeOK, answer.Outcome)
	}
	if !answer.Applied {
		t.Errorf("the second run did not report itself as applied:\n%s", finished.out)
	}
	if answer.Refusal != "" || answer.Detail != "" {
		t.Errorf("the applied run named a refusal: %q %q", answer.Refusal, answer.Detail)
	}
}
