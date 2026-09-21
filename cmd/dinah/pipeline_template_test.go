package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newPipelineBench builds a workbench from the embedded pipeline template,
// the way newBench builds the default one, and returns the directory a
// subsequent runCLI call should be pointed at.
func newPipelineBench(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "workbench")
	t.Setenv("DINAH_HOME", filepath.Join(base, "home"))
	t.Setenv("DINAH_ACTOR", "ops")
	t.Setenv("DINAH_LANG", "")
	t.Setenv("DINAH_FORMAT", "")
	t.Setenv("DINAH_WORKBENCH", "")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	got := runCLI(t, root, "init", "--from", "pipeline", "--slug", "px", "--operator", "ops")
	if got.code != 0 {
		t.Fatalf("init --from pipeline: %d %s", got.code, got.errw)
	}
	return root
}

// attachFile writes a throwaway file and attaches it to a reference, the way
// a work column of the pipeline produces its own attachment.
func attachFile(t *testing.T, root, ref, name, body string) {
	t.Helper()
	source := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(source, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", source, err)
	}
	got := runCLI(t, root, "attach", ref, source)
	if got.code != 0 {
		t.Fatalf("attach %s to %s: %d %s", source, ref, got.code, got.errw)
	}
}

// TestInitFromPipelineProducesACleanWorkbench is dinah-547 AC-1: `dinah init
// --from pipeline --slug <s> --operator <a>` in an empty directory
// instantiates a workbench, and `dinah check` on it exits 0 with zero
// findings.
func TestInitFromPipelineProducesACleanWorkbench(t *testing.T) {
	root := newPipelineBench(t)

	checked := runCLI(t, root, "check")
	if checked.code != 0 {
		t.Fatalf("check: %d\nstdout:\n%s\nstderr:\n%s", checked.code, checked.out, checked.errw)
	}
}

// TestThePipelineWorkbenchDeclaresItsFiveColumns is dinah-547 AC-2, at the
// column listing rather than at the JSON interchange form: five columns in
// order, Intake (intake), Draft (work), Review (work), Done (done), Returned
// (done).
func TestThePipelineWorkbenchDeclaresItsFiveColumns(t *testing.T) {
	root := newPipelineBench(t)

	got := runCLI(t, root, "list", "columns")
	if got.code != 0 {
		t.Fatalf("list columns: %d %s", got.code, got.errw)
	}
	wantOrder := []string{"Intake", "Draft", "Review", "Done", "Returned"}
	at := 0
	for _, line := range strings.Split(got.out, "\n") {
		if at >= len(wantOrder) {
			break
		}
		if strings.Contains(line, wantOrder[at]) {
			at++
		}
	}
	if at != len(wantOrder) {
		t.Fatalf("wanted the five columns in order %v, only matched %d in:\n%s", wantOrder, at, got.out)
	}
}

// TestAPipelineCardWalksIntakeToDoneAndASecondIsRejected is dinah-547 AC-4: a
// card walked Intake to Draft to Review to Done with a fresh attachment at
// each work column completes with no refusal, and a second card moved from
// Review to Returned records reject:true on that moved event (read here as
// the (reject) marker the journal listing prints, on the same posture
// TestLogMarksARejectMove already reads it).
func TestAPipelineCardWalksIntakeToDoneAndASecondIsRejected(t *testing.T) {
	root := newPipelineBench(t)

	if got := runCLI(t, root, "add", "Translate the announcement"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "pull", "draft"); got.code != 0 {
		t.Fatalf("pull draft: %d %s", got.code, got.errw)
	}
	attachFile(t, root, "px-1", "draft.md", "the draft translation")
	if got := runCLI(t, root, "move", "px-1", "review"); got.code != 0 {
		t.Fatalf("move to review: %d %s", got.code, got.errw)
	}
	attachFile(t, root, "px-1", "findings.md", "the draft holds up")
	if got := runCLI(t, root, "release", "px-1"); got.code != 0 {
		t.Fatalf("release before landing at a terminal: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "px-1", "done"); got.code != 0 {
		t.Fatalf("move to done: %d %s", got.code, got.errw)
	}

	if got := runCLI(t, root, "add", "Translate the second announcement"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "pull", "draft"); got.code != 0 {
		t.Fatalf("pull draft: %d %s", got.code, got.errw)
	}
	attachFile(t, root, "px-2", "draft.md", "a draft that has drifted")
	if got := runCLI(t, root, "move", "px-2", "review"); got.code != 0 {
		t.Fatalf("move to review: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "release", "px-2"); got.code != 0 {
		t.Fatalf("release before landing at a terminal: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "move", "px-2", "returned"); got.code != 0 {
		t.Fatalf("the rejecting move: %d %s", got.code, got.errw)
	}

	logged := runCLI(t, root, "list", "px-2/journal")
	if logged.code != 0 {
		t.Fatalf("journal: %d %s", logged.code, logged.errw)
	}
	marker := "(reject)"
	marked := 0
	for _, line := range strings.Split(logged.out, "\n") {
		if strings.Contains(line, marker) {
			marked++
		}
	}
	if marked != 1 {
		t.Fatalf("wanted the %s mark on exactly one line of px-2's journal, got %d:\n%s", marker, marked, logged.out)
	}

	// px-1's journal carries no reject mark, since every one of its moves
	// went with the reject_to declaration rather than against it.
	unrejected := runCLI(t, root, "list", "px-1/journal")
	if unrejected.code != 0 {
		t.Fatalf("journal: %d %s", unrejected.code, unrejected.errw)
	}
	for _, line := range strings.Split(unrejected.out, "\n") {
		if strings.Contains(line, marker) {
			t.Errorf("px-1 completed Intake to Done and its journal still carries %s:\n%s", marker, unrejected.out)
		}
	}
}

// pipelineCardToReview files a card, pulls it into Draft, attaches a draft,
// and carries it to Review, which is where a test proving the loop limit
// needs to start.
func pipelineCardToReview(t *testing.T, root string) {
	t.Helper()
	if got := runCLI(t, root, "add", "Translate the release notes"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	if got := runCLI(t, root, "pull", "draft"); got.code != 0 {
		t.Fatalf("pull draft: %d %s", got.code, got.errw)
	}
	attachFile(t, root, "px-1", "draft.md", "the draft translation")
	if got := runCLI(t, root, "move", "px-1", "review"); got.code != 0 {
		t.Fatalf("move to review: %d %s", got.code, got.errw)
	}
}

// TestReviewsLoopLimitBindsTheFourthReturnToDraft is dinah-547 AC-5: a card
// moved from Review back to Draft three times, then a fourth time without
// --override, is refused dinah.at-loop-limit; the same fourth move with
// --override succeeds and its moved event records override:true.
func TestReviewsLoopLimitBindsTheFourthReturnToDraft(t *testing.T) {
	root := newPipelineBench(t)
	pipelineCardToReview(t, root)

	for pass := 1; pass <= 3; pass++ {
		if got := runCLI(t, root, "move", "px-1", "draft"); got.code != 0 {
			t.Fatalf("regressive pass %d: %d %s", pass, got.code, got.errw)
		}
		if got := runCLI(t, root, "move", "px-1", "review"); got.code != 0 {
			t.Fatalf("return to review after pass %d: %d %s", pass, got.code, got.errw)
		}
	}

	refused := runCLI(t, root, "move", "px-1", "draft")
	if refused.code == 0 {
		t.Fatalf("the fourth regressive departure was not refused:\n%s", refused.out)
	}
	if !strings.Contains(refused.errw, "dinah.at-loop-limit") {
		t.Errorf("the refusal does not name dinah.at-loop-limit:\n%s", refused.errw)
	}

	overridden := runCLI(t, root, "move", "px-1", "draft", "--override")
	if overridden.code != 0 {
		t.Fatalf("the override was refused: %d %s", overridden.code, overridden.errw)
	}

	logged := runCLI(t, root, "list", "px-1/journal")
	if logged.code != 0 {
		t.Fatalf("journal: %d %s", logged.code, logged.errw)
	}
	marker := "(override)"
	marked := 0
	for _, line := range strings.Split(logged.out, "\n") {
		if strings.Contains(line, marker) {
			marked++
		}
	}
	if marked != 1 {
		t.Fatalf("wanted the %s mark on exactly one line, got %d:\n%s", marker, marked, logged.out)
	}
}

// TestInitFromPipelineShadowsALocalDirectoryOfTheSameName is dinah-547 AC-7: a
// directory literally named pipeline sitting in the target does not stop
// `dinah init --from pipeline` from resolving the embedded template, because
// the embedded-template lookup in readSource runs before the directory and
// file branches. Without that ordering, today's code would find the
// directory, see it carries no workbench.md, fall through to the file-read
// branch, and refuse UnknownPath.
func TestInitFromPipelineShadowsALocalDirectoryOfTheSameName(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "workbench")
	t.Setenv("DINAH_HOME", filepath.Join(base, "home"))
	t.Setenv("DINAH_ACTOR", "ops")
	t.Setenv("DINAH_LANG", "")
	t.Setenv("DINAH_FORMAT", "")
	t.Setenv("DINAH_WORKBENCH", "")
	if err := os.MkdirAll(filepath.Join(root, "pipeline"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	got := runCLI(t, root, "init", "--from", "pipeline", "--slug", "px", "--operator", "ops", "--here")
	if got.code != 0 {
		t.Fatalf("init --from pipeline, with a same-named local directory present: %d %s", got.code, got.errw)
	}

	checked := runCLI(t, root, "check")
	if checked.code != 0 {
		t.Fatalf("check: %d %s", checked.code, checked.errw)
	}
}
