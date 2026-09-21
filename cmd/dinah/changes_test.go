package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// TestTheCheckpointPrintsItsEventsAndItsCursor asserts what a person reads at
// a terminal: a table of the lines that landed since the cursor they held, and
// the cursor to ask with next time.
//
// Three shapes of the card column are drawn on one walk, because each is what
// the column falls back to when the one in front of it cannot be composed: a
// card that still has an anchor draws its reference, a card directory carrying
// a history and no anchor draws its bare identifier, and a workbench-scoped
// line draws the word a person types to name the workbench.
func TestTheCheckpointPrintsItsEventsAndItsCursor(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "A card that runs into an obstacle"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	cursor := mintedCursor(t, root)

	if got := runCLI(t, root, "block", "fx-1", "the vendor has not answered"); got.code != 0 {
		t.Fatalf("block: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "set", "workbench", "title", "A renamed workbench"); got.code != 0 {
		t.Fatalf("workbench set title: %d %s", got.code, got.errw)
	}
	orphan := writeAnchorlessCard(t, root)

	got := runCLI(t, root, "changes", "--since", cursor)
	if got.code != 0 {
		t.Fatalf("changes: %d %s", got.code, got.errw)
	}
	// Each subject is read off its own row's card column rather than searched
	// for in the whole block. Searching the block proves the string was drawn
	// somewhere, which is the existence assertion standing in for an identity
	// assertion that the counterexamples corpus names, and here it would pass
	// on a renderer that drew every subject in the wrong column.
	rows := changeRows(t, got.out)
	if len(rows) != 3 {
		t.Fatalf("wanted the three lines the fixture wrote, got %d:\n%s", len(rows), got.out)
	}
	remaining := rows
	for _, wanted := range []struct{ detail, subject string }{
		{detail: "the vendor has not answered", subject: "fx-1"},
		{detail: "A card with no anchor", subject: orphan},
	} {
		row, rest := rowCarrying(remaining, wanted.detail)
		if row == nil {
			t.Fatalf("no row carries %q:\n%s", wanted.detail, got.out)
		}
		if row.subject != wanted.subject {
			t.Errorf("the card column of the %q row reads %q, wanted %q", wanted.detail, row.subject, wanted.subject)
		}
		remaining = rest
	}
	// The workbench line is what is left: it is the one act of the three whose
	// detail column is empty, so it cannot be found by a marker of its own.
	if len(remaining) != 1 {
		t.Fatalf("wanted the workbench row left over, got %+v", remaining)
	}
	if remaining[0].subject != bench.WorkbenchKey {
		t.Errorf("the card column of the workbench row reads %q, wanted %q", remaining[0].subject, bench.WorkbenchKey)
	}
	if !strings.Contains(got.out, "cursor: ") {
		t.Errorf("the checkpoint prints no cursor, so the reader has nothing to ask with next:\n%s", got.out)
	}
}

// changeRow is one drawn line of a checkpoint, split far enough to read the
// card column and the detail column off it.
type changeRow struct {
	subject string
	detail  string
	line    string
}

// changeRows reads the drawn table back, one entry per row rather than per
// printed line, because a narrow terminal wraps one row over several lines.
//
// A timestamp in the first column opens a row and every line under it belongs
// to that row until the next one does, which is what separates the rows from
// the heading above them and the cursor below them. Joined back up, the second
// whitespace-separated field is the card column, since the columns in front of
// it never hold a space.
func changeRows(t *testing.T, out string) []changeRow {
	t.Helper()
	// The reading below rests on the four columns in front of the detail
	// holding no space, and three of those four are identifiers the format
	// constrains. The fourth is the actor's name, which is free text that
	// nothing in the format stops carrying a space, and a spaced one would
	// shift every detail silently by one word rather than failing. So the
	// name this run acts under is checked rather than assumed, in the way
	// the column title's own precondition is checked by its caller.
	//
	// The check reaches the name the tool acts under and no further: a line
	// planted by hand carries whatever actor its planter wrote, and a
	// planter wanting a spaced name has to read the cell off the machine
	// surface instead.
	if actor := os.Getenv("DINAH_ACTOR"); strings.ContainsAny(actor, " \t") {
		t.Fatalf("this run acts as %q, whose name carries a space, so the detail column cannot be read as the fields after the fourth", actor)
	}
	var rows []changeRow
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(strings.TrimSpace(line), "cursor:") {
			continue
		}
		if !bench.ParseStamp(fields[0]).IsZero() {
			rows = append(rows, changeRow{line: line})
			continue
		}
		if len(rows) == 0 {
			continue
		}
		rows[len(rows)-1].line += " " + strings.TrimSpace(line)
	}
	for i := range rows {
		fields := strings.Fields(rows[i].line)
		if len(fields) < 2 {
			t.Fatalf("a checkpoint row carries no card column: %q", rows[i].line)
		}
		rows[i].subject = fields[1]
		// The detail is the last of the five columns, and the four in front
		// of it hold no space, which the fatal at the top of this helper
		// checks for the one of the four that is free text. What is left
		// after them is therefore the whole of it. Reading it as a field
		// rather than searching the row for a
		// string is what lets a caller assert the exact bytes a renderer
		// drew: a row containing a value and a row whose detail is that
		// value are different claims, and only the second is what a
		// criterion about the detail column means.
		//
		// A detail carrying a run of two spaces does not survive this,
		// because the drawn row's padding is indistinguishable from it. No
		// caller has one, and a caller that did would have to read the cell
		// off the machine surface instead.
		if len(fields) > 4 {
			rows[i].detail = strings.Join(fields[4:], " ")
		}
	}
	return rows
}

// rowCarrying picks the one row whose detail column carries a marker, and
// returns the rest, so each marker is matched once and the leftover row is the
// one no marker names.
func rowCarrying(rows []changeRow, marker string) (*changeRow, []changeRow) {
	for i, row := range rows {
		if !strings.Contains(row.line, marker) {
			continue
		}
		rest := append(append([]changeRow{}, rows[:i]...), rows[i+1:]...)
		return &rows[i], rest
	}
	return nil, rows
}

// TestTheCheckpointPrintsACursorEvenWhenNothingMoved asserts the answer a
// person gets on the call that reports nothing, which is the common one: the
// table is empty and the cursor is still there to carry forward.
func TestTheCheckpointPrintsACursorEvenWhenNothingMoved(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "A card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	cursor := mintedCursor(t, root)

	got := runCLI(t, root, "changes", "--since", cursor)
	if got.code != 0 {
		t.Fatalf("changes: %d %s", got.code, got.errw)
	}
	if !strings.Contains(got.out, "cursor: "+cursor) {
		t.Errorf("an unchanged workbench did not print the cursor back unchanged:\n%s", got.out)
	}
}

// TestTheCheckpointRefusesACursorItDidNotIssue asserts the refusal reaches the
// terminal under its own name, which is what a script reads with cut.
func TestTheCheckpointRefusesACursorItDidNotIssue(t *testing.T) {
	root := newBench(t)
	got := runCLI(t, root, "changes", "--since", "not-a-cursor")
	if got.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("wanted the refused exit code, got %d\n%s", got.code, got.out)
	}
	if !strings.HasPrefix(got.errw, contract.Malformed+" ") {
		t.Errorf("wanted %s to lead stderr, got %q", contract.Malformed, got.errw)
	}
}

// mintedCursor takes a cursor off the machine form, which is where a caller
// reads one when a script is holding it.
func mintedCursor(t *testing.T, root string) string {
	t.Helper()
	got := runCLI(t, root, "--json", "changes")
	if got.code != 0 {
		t.Fatalf("mint a cursor: %d %s", got.code, got.errw)
	}
	var minted struct {
		Cursor string `json:"cursor"`
	}
	if err := json.Unmarshal([]byte(got.out), &minted); err != nil {
		t.Fatalf("read the minted cursor: %v\n%s", err, got.out)
	}
	if minted.Cursor == "" {
		t.Fatal("the machine form carried no cursor")
	}
	return minted.Cursor
}

// TestAWaitingCallExitsZeroAtItsTimeout covers dinah-546/criteria/2: against
// a bench nothing touches during the call, --wait --timeout 200ms returns at
// or shortly after 200ms with Changed false and the cursor handed back
// byte-identical, exit 0. The 200ms timeout, well under waitPollInterval
// (500ms, internal/verb/changes.go), also exercises the deadline cap
// (changes.go's waitForChange): without it the wait would not answer until
// the first 500ms sleep completed, well past 200ms.
//
// Hard test deadline: the call is synchronous and its own worst case is
// bounded below by checking elapsed against a generous overshoot budget,
// rather than by a select/time.After, since nothing here can hang the test
// runner longer than the process itself would hang.
func TestAWaitingCallExitsZeroAtItsTimeout(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "A card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	cursor := mintedCursor(t, root)

	started := time.Now()
	got := runCLI(t, root, "--json", "changes", "--since", cursor, "--wait", "--timeout", "200ms")
	elapsed := time.Since(started)
	if got.code != 0 {
		t.Fatalf("changes --wait --timeout 200ms: %d %s", got.code, got.errw)
	}
	if elapsed < 200*time.Millisecond {
		t.Errorf("returned after %s, before its own 200ms timeout", elapsed)
	}
	const overshootBudget = 400 * time.Millisecond
	if elapsed > 200*time.Millisecond+overshootBudget {
		t.Errorf("returned after %s, more than %s past its own 200ms timeout: the deadline cap did not hold", elapsed, overshootBudget)
	}
	var set struct {
		Cursor  string `json:"cursor"`
		Changed bool   `json:"changed"`
	}
	if err := json.Unmarshal([]byte(got.out), &set); err != nil {
		t.Fatalf("decode the answer: %v\n%s", err, got.out)
	}
	if set.Changed {
		t.Error("the wait reported a change nothing made")
	}
	if set.Cursor != cursor {
		t.Errorf("the cursor came back rewritten:\nwanted %q\ngot    %q", cursor, set.Cursor)
	}
}

// TestAWaitingCallExitsUnreachableWhenTheWorkbenchDisappears covers
// dinah-546/criteria/4's exit-code half: the workbench directory disappears
// while a --wait call is blocked on it, and the answer that reaches a
// terminal is exit 4 (contract.OutcomeUnreachable) with that outcome leading
// stderr, which is main.go's reportError (517-522) mapping a plain,
// non-refusal error the way it already maps one for every other command.
// The library-level half, that Library.Changes itself propagates such an
// error promptly rather than hanging or retrying, is
// TestAWaitingCallReturnsUnreachableWhenTheWorkbenchDisappears in
// internal/verb/changes_wait_test.go.
//
// The wait still reaches the head through runCLI, per
// TestOnlyRunCLIDrivesTheHead, which every test in this package answers to.
// What keeps this test safe to run concurrently with the rename below is
// where its process cwd sits: runCLI chdirs into the directory it is given
// for the call's duration, so the call below is pointed at an unrelated,
// untouched directory and reaches the bench through --workbench instead. The
// directory that disappears mid-call is therefore never the process's own
// working directory, which is the one Windows will not let a rename or a
// delete touch.
//
// Hard test deadline: 6s, via the select's time.After.
func TestAWaitingCallExitsUnreachableWhenTheWorkbenchDisappears(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "add", "A card"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	cursor := mintedCursor(t, root)
	dir := soleBenchDir(t, root)
	elsewhere := t.TempDir()

	done := make(chan invocation, 1)
	go func() {
		done <- runCLI(t, elsewhere, "--workbench", dir, "changes", "--since", cursor, "--wait", "--timeout", "5s")
	}()

	// Give the head's own goroutine time to open the workbench, run its
	// first checkpoint and enter its sleep before the directory goes away.
	time.Sleep(150 * time.Millisecond)
	renamedTo := dir + "-renamed-out-from-under-the-wait"
	if err := os.Rename(dir, renamedTo); err != nil {
		t.Fatalf("rename the workbench directory out from under the wait: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(renamedTo) })

	select {
	case got := <-done:
		if got.code != contract.ExitCode(contract.OutcomeUnreachable) {
			t.Fatalf("wanted exit %d, got %d\n%s", contract.ExitCode(contract.OutcomeUnreachable), got.code, got.errw)
		}
		if !strings.HasPrefix(got.errw, contract.OutcomeUnreachable+" ") {
			t.Errorf("wanted %s to lead stderr, got %q", contract.OutcomeUnreachable, got.errw)
		}
	case <-time.After(6 * time.Second):
		t.Fatal("the wait never returned after its own workbench disappeared; hard test deadline hit")
	}
}

// absentWorkbench names a directory that does not exist, for the grammar
// refusal tests below: a refusal that fires ahead of any bench walk answers
// Malformed against this path exactly as it would against a real one, and a
// grammar check that ran after opening the workbench would instead fail on a
// workbench resolution error, which is how these tests tell the two apart
// without reading any internal counter.
func absentWorkbench(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "does-not-exist")
}

// TestWaitWithSinceEmptyIsRefusedBeforeAnyWalk covers dinah-546/criteria/8:
// --wait given with --since empty is refused, Malformed, exit 2, and reaches
// no Library, proven by naming a workbench that does not exist and still
// getting Malformed rather than a workbench-resolution failure.
func TestWaitWithSinceEmptyIsRefusedBeforeAnyWalk(t *testing.T) {
	got := runCLI(t, t.TempDir(), "--workbench", absentWorkbench(t), "changes", "--wait")
	if got.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("wanted the refused exit code, got %d\n%s%s", got.code, got.out, got.errw)
	}
	if !strings.HasPrefix(got.errw, contract.Malformed+" ") {
		t.Errorf("wanted %s to lead stderr, got %q", contract.Malformed, got.errw)
	}
	if !strings.Contains(got.errw, "--wait") {
		t.Errorf("the refusal does not name --wait: %q", got.errw)
	}

	// Control: the same call with a cursor named reaches a real workbench and
	// answers ok, which is what shows the refusal above came from the empty
	// --since rather than from something else about the invocation.
	root := newBench(t)
	cursor := mintedCursor(t, root)
	control := runCLI(t, root, "changes", "--since", cursor, "--wait", "--timeout", "1ms")
	if control.code != 0 {
		t.Fatalf("the control call with --since named was refused too, so the assertion above proves nothing: %d %s", control.code, control.errw)
	}
}

// TestTimeoutWithoutWaitIsRefusedIndependentOfItsValue covers
// dinah-546/criteria/9: --timeout given without --wait is refused,
// Malformed, exit 2, whatever value it carries, including one ParseDuration
// itself would reject; the case matters because it proves the refusal fires
// on the missing --wait rather than on an attempt to parse the value.
func TestTimeoutWithoutWaitIsRefusedIndependentOfItsValue(t *testing.T) {
	structurallyValidCursor := mintedCursor(t, newBench(t))
	for _, value := range []string{"1s", "notaduration"} {
		got := runCLI(t, t.TempDir(), "--workbench", absentWorkbench(t), "changes", "--since", structurallyValidCursor, "--timeout", value)
		if got.code != contract.ExitCode(contract.OutcomeRefused) {
			t.Fatalf("--timeout %s: wanted the refused exit code, got %d\n%s%s", value, got.code, got.out, got.errw)
		}
		if !strings.HasPrefix(got.errw, contract.Malformed+" ") {
			t.Errorf("--timeout %s: wanted %s to lead stderr, got %q", value, contract.Malformed, got.errw)
		}
		if !strings.Contains(got.errw, "--timeout") {
			t.Errorf("--timeout %s: the refusal does not name --timeout: %q", value, got.errw)
		}
	}

	// Control: the same flag with --wait added reaches a real workbench and
	// answers ok.
	root := newBench(t)
	cursor := mintedCursor(t, root)
	control := runCLI(t, root, "changes", "--since", cursor, "--wait", "--timeout", "1ms")
	if control.code != 0 {
		t.Fatalf("the control call with --wait added was refused too, so the assertion above proves nothing: %d %s", control.code, control.errw)
	}
}

// TestTimeoutValueFailingParseDurationIsRefused covers dinah-546/criteria/10:
// --timeout notaduration, given together with --wait, is refused Malformed
// exit 2, reusing ParseDuration's own error the same way --expires
// notaduration does on claim.
func TestTimeoutValueFailingParseDurationIsRefused(t *testing.T) {
	structurallyValidCursor := mintedCursor(t, newBench(t))
	got := runCLI(t, t.TempDir(), "--workbench", absentWorkbench(t), "changes", "--since", structurallyValidCursor, "--wait", "--timeout", "notaduration")
	if got.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("wanted the refused exit code, got %d\n%s%s", got.code, got.out, got.errw)
	}
	if !strings.HasPrefix(got.errw, contract.Malformed+" ") {
		t.Errorf("wanted %s to lead stderr, got %q", contract.Malformed, got.errw)
	}
	if !strings.Contains(got.errw, "notaduration") {
		t.Errorf("the refusal does not carry the bad value, unlike --expires notaduration on claim: %q", got.errw)
	}

	// Control: a value ParseDuration accepts reaches a real workbench and
	// answers ok.
	root := newBench(t)
	cursor := mintedCursor(t, root)
	control := runCLI(t, root, "changes", "--since", cursor, "--wait", "--timeout", "1ms")
	if control.code != 0 {
		t.Fatalf("the control call with a parseable value was refused too, so the assertion above proves nothing: %d %s", control.code, control.errw)
	}
}

// TestWaitWithRootIsRefusedAndReachesNoLibrary covers dinah-546/criteria/11:
// --wait given with --root is refused, Malformed, exit 2, and reaches no
// Library, proven the same way TestWaitWithSinceEmptyIsRefusedBeforeAnyWalk
// proves it: a root that does not exist still answers Malformed rather than
// an unknown-root failure, because the refusal fires before rootWalkFor runs.
func TestWaitWithRootIsRefusedAndReachesNoLibrary(t *testing.T) {
	structurallyValidCursor := mintedCursor(t, newBench(t))
	got := runCLI(t, t.TempDir(), "changes", "--since", structurallyValidCursor, "--wait", "--root", absentWorkbench(t))
	if got.code != contract.ExitCode(contract.OutcomeRefused) {
		t.Fatalf("wanted the refused exit code, got %d\n%s%s", got.code, got.out, got.errw)
	}
	if !strings.HasPrefix(got.errw, contract.Malformed+" ") {
		t.Errorf("wanted %s to lead stderr, got %q", contract.Malformed, got.errw)
	}
	if !strings.Contains(got.errw, "--root") || !strings.Contains(got.errw, "--wait") {
		t.Errorf("the refusal does not name both --wait and --root: %q", got.errw)
	}

	// Control: --root without --wait against the same absent path is refused
	// too, but as dinah.unknown-root rather than as malformed: --root names a
	// path the filesystem does not carry a directory at, independent of
	// --wait. That distinct name is what shows the case above is refused for
	// naming --wait together with --root, and not merely for naming a root
	// that does not resolve.
	control := runCLI(t, t.TempDir(), "changes", "--root", absentWorkbench(t))
	if strings.HasPrefix(control.errw, contract.Malformed+" ") {
		t.Fatalf("--root alone against an absent path also answered malformed, so the assertion above proves nothing: %d %s", control.code, control.errw)
	}
	if !strings.Contains(control.errw, "unknown-root") {
		t.Errorf("wanted the control refused as unknown-root, got: %d %s", control.code, control.errw)
	}
}

// writeAnchorlessCard builds a card directory carrying a history and no
// anchor, which is what a crash between the directory and the anchor leaves
// and what dinah check reports. The walk keys on the directory, so the line
// is delivered and no reference can be composed for it.
func writeAnchorlessCard(t *testing.T, container string) string {
	t.Helper()
	id := "0f8d904a8b4c"
	dir := filepath.Join(soleBenchDir(t, container), bench.CardsDir, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	event := bench.Event{TS: bench.Stamp(bench.ParseStamp("2099-01-01T00:00:00Z")), Event: contract.EventCreated, Actor: bench.NamedActor("alka"), Title: "A card with no anchor"}
	if err := bench.AppendEvent(filepath.Join(dir, bench.JournalName), event); err != nil {
		t.Fatalf("write the history: %v", err)
	}
	return id
}
