package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
)

// legacyContainerBench builds a workbench through the tool and then renames its
// directory to the 12-hex width a workbench carried before the containment
// rule, which is the shape the migration meets in the field. It answers the
// directory the workbench now sits in.
func legacyContainerBench(t *testing.T, container string) string {
	t.Helper()
	root := filepath.Dir(container)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := runCLI(t, root, "init", root, "--slug", "lg", "--operator", "alka"); got.code != 0 {
		t.Fatalf("init at %s: %d %s", root, got.code, got.errw)
	}
	written := benchDir(t, root)
	if got := runCLI(t, root, "--workbench", written, "add", "a card the migration must keep"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	legacy := filepath.Join(container, "d00000000001")
	if err := os.Rename(written, legacy); err != nil {
		t.Fatalf("rename to the legacy width: %v", err)
	}
	return legacy
}

// TestTheContainerMigrationPreviewsBeforeItMoves asserts what `dinah check
// --migrate-container` does with and without the confirmation: a bare run
// names what it would move and writes nothing, and a confirmed run moves each
// workbench and says where it went.
func TestTheContainerMigrationPreviewsBeforeItMoves(t *testing.T) {
	// The tree is resolved the way the head resolves its own working
	// directory, because these cases compare a path the head printed against a
	// path composed here, and a temporary directory reached through a symlink
	// has two spellings on macOS.
	tree := resolvedDir(t, emptyTree(t))
	container := filepath.Join(tree, "project", bench.UserBaseName)
	legacy := legacyContainerBench(t, container)

	preview := runCLI(t, tree, "check", "--migrate-container")
	if preview.code != 5 {
		t.Fatalf("the preview exited %d, wanted the findings code 5: %s%s", preview.code, preview.out, preview.errw)
	}
	if !strings.Contains(preview.out, legacy) {
		t.Errorf("the preview does not name the workbench it would move:\n%s", preview.out)
	}
	if !strings.Contains(preview.out, "--yes") {
		t.Errorf("the preview does not say how to authorize the move:\n%s", preview.out)
	}
	if !bench.Exists(legacy) {
		t.Fatal("the preview moved the workbench")
	}

	applied := runCLI(t, tree, "check", "--migrate-container", "--yes")
	if applied.code != 0 {
		t.Fatalf("the migration exited %d: %s%s", applied.code, applied.out, applied.errw)
	}
	if bench.Exists(legacy) {
		t.Error("the migration left the workbench under its legacy name")
	}
	ids := bench.ListWorkbenchIDs(container)
	if len(ids) != 1 || !bench.IsWorkbenchID(ids[0]) {
		t.Fatalf("the container holds %v, wanted one minted identifier", ids)
	}
	if !strings.Contains(applied.out, filepath.Join(container, ids[0])) {
		t.Errorf("the report does not name where the workbench went:\n%s", applied.out)
	}
	// The workbench answers as itself afterwards, which is the point of the
	// move: the card it carried is still there and still reachable by name.
	listed := runCLI(t, filepath.Join(tree, "project"), "ls")
	if listed.code != 0 {
		t.Fatalf("ls after the migration: %d %s", listed.code, listed.errw)
	}
	if !strings.Contains(listed.out, "a card the migration must keep") {
		t.Errorf("the migrated workbench lost its card:\n%s", listed.out)
	}

	// A second confirmed run has nothing to do and says so, which is what an
	// unattended sweep over an already-migrated tree looks like.
	again := runCLI(t, tree, "check", "--migrate-container", "--yes")
	if again.code != 0 {
		t.Fatalf("the second run exited %d: %s%s", again.code, again.out, again.errw)
	}
	if !strings.Contains(again.out, filepath.Join(container, ids[0])) {
		t.Errorf("the second run does not report the workbench as already contained:\n%s", again.out)
	}
}

// TestTheContainerMigrationReportsADuplicateAndRemintRepairsIt asserts the
// command half of dinah-285 AC-11: a shared identifier is reported by the
// sweep and repaired by nothing, and `dinah check --remint <path>` renames the
// one directory it is given.
func TestTheContainerMigrationReportsADuplicateAndRemintRepairsIt(t *testing.T) {
	tree := resolvedDir(t, emptyTree(t))
	first := filepath.Join(tree, "one", bench.UserBaseName, "0199a1b2c3d47abc8000000000000001")
	second := filepath.Join(tree, "two", bench.UserBaseName, "0199a1b2c3d47abc8000000000000001")
	for _, where := range []string{first, second} {
		if err := os.MkdirAll(where, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		definition, err := bench.ReadDefinition([]byte(fmtBaseDefinition("Copy")))
		if err != nil {
			t.Fatalf("definition: %v", err)
		}
		if err := bench.Instantiate(where, "cp", "alka", definition); err != nil {
			t.Fatalf("instantiate: %v", err)
		}
		// Two clones of a repository whose workbench was interrupted between
		// its last move and its stamp declare the format that predates the
		// rule, which is the one state the stamp still has work to do on. It
		// is left undone here, because the report says these directories were
		// untouched.
		editAnchorAt(t, filepath.Join(where, bench.WorkbenchAnchor), "format: "+strconv.Itoa(bench.StorageFormat), "format: 1")
	}

	swept := runCLI(t, tree, "check", "--migrate-container", "--yes")
	if swept.code != 5 {
		t.Fatalf("a tree carrying a duplicate exited %d, wanted the findings code 5: %s%s", swept.code, swept.out, swept.errw)
	}
	for _, where := range []string{first, second} {
		if !strings.Contains(swept.out, where) {
			t.Errorf("the report does not name %s:\n%s", where, swept.out)
		}
		if !bench.Exists(where) {
			t.Errorf("the sweep moved %s, and choosing between two copies is not its to make", where)
		}
		if !anchorDeclares(t, filepath.Join(where, bench.WorkbenchAnchor), "format: 1") {
			t.Errorf("the sweep wrote into %s, which its own report says it left alone", where)
		}
	}

	reminted := runCLI(t, tree, "check", "--remint", first)
	if reminted.code != 0 {
		t.Fatalf("remint exited %d: %s%s", reminted.code, reminted.out, reminted.errw)
	}
	if bench.Exists(first) {
		t.Error("the remint left the workbench under the identifier it was asked to replace")
	}
	if !bench.Exists(second) {
		t.Error("the remint touched the other copy")
	}
	ids := bench.ListWorkbenchIDs(filepath.Dir(first))
	if len(ids) != 1 || ids[0] == "0199a1b2c3d47abc8000000000000001" {
		t.Errorf("the container holds %v, wanted one freshly minted identifier", ids)
	}
	if !strings.Contains(reminted.out, ids[0]) {
		t.Errorf("the remint does not say what the workbench is called now:\n%s", reminted.out)
	}
}

// TestTheContainerMigrationSpeaksTheMachineForm asserts that the repair answers
// a machine caller with the same facts it prints, since an unattended sweep is
// the caller this repair exists for.
func TestTheContainerMigrationSpeaksTheMachineForm(t *testing.T) {
	tree := resolvedDir(t, emptyTree(t))
	container := filepath.Join(tree, "project", bench.UserBaseName)
	legacy := legacyContainerBench(t, container)

	got := runCLI(t, tree, "--json", "check", "--migrate-container")
	if got.code != 5 {
		t.Fatalf("the preview exited %d, wanted 5: %s%s", got.code, got.out, got.errw)
	}
	var preview struct {
		Outcome  string `json:"outcome"`
		Preview  bool   `json:"preview"`
		Migrated []struct {
			Path  string `json:"path"`
			To    string `json:"to"`
			Shape string `json:"shape"`
		} `json:"migrated"`
	}
	if err := json.Unmarshal([]byte(got.out), &preview); err != nil {
		t.Fatalf("the machine form did not decode: %v\n%s", err, got.out)
	}
	if !preview.Preview {
		t.Error("a run carrying no confirmation did not mark itself a preview")
	}
	if len(preview.Migrated) != 1 || preview.Migrated[0].Path != legacy {
		t.Fatalf("the machine form reports %+v, wanted the one workbench at %s", preview.Migrated, legacy)
	}
	if preview.Migrated[0].Shape != string(bench.ShapeLegacy) {
		t.Errorf("the shape reads %q, wanted %q", preview.Migrated[0].Shape, bench.ShapeLegacy)
	}
	if preview.Migrated[0].To != "" {
		t.Errorf("a preview named a destination %q, and the identifier is minted at the moment of the move", preview.Migrated[0].To)
	}
}

// TestEveryWorkbenchListingCarriesTheIdentifier asserts dinah-285 AC-15
// against the rendered output of each surface rather than against the Go
// struct: the terminal's own machine form, and the compact form, both carry the
// identifier for a migrated workbench and omit it for one the migration has not
// reached.
func TestEveryWorkbenchListingCarriesTheIdentifier(t *testing.T) {
	tree := resolvedDir(t, emptyTree(t))
	container := filepath.Join(tree, "project", bench.UserBaseName)
	legacy := legacyContainerBench(t, container)

	before := workbenchRows(t, tree, tree)
	if len(before) != 1 {
		t.Fatalf("the listing answered %v, wanted the one workbench", before)
	}
	if _, carried := before[0]["id"]; carried {
		t.Errorf("an unmigrated workbench carries an id key: %v", before[0])
	}
	if before[0]["path"] != legacy {
		t.Errorf("the row names %v, wanted %s", before[0]["path"], legacy)
	}

	if got := runCLI(t, tree, "check", "--migrate-container", "--yes"); got.code != 0 {
		t.Fatalf("migrate: %d %s", got.code, got.errw)
	}
	ids := bench.ListWorkbenchIDs(container)
	if len(ids) != 1 {
		t.Fatalf("the container holds %v, wanted one workbench", ids)
	}
	after := workbenchRows(t, tree, tree)
	if len(after) != 1 {
		t.Fatalf("the listing answered %v, wanted the one workbench", after)
	}
	if after[0]["id"] != ids[0] {
		t.Errorf("the row carries the id %v, wanted the directory name %s", after[0]["id"], ids[0])
	}

	// The compact form carries the same identifier in the same record, which
	// is what makes the two machine forms one answer rather than two.
	compact := runCLI(t, tree, "--format", "compact", "workbenches", tree)
	if compact.code != 0 {
		t.Fatalf("workbenches --format compact: %d %s", compact.code, compact.errw)
	}
	if !strings.Contains(compact.out, ids[0]) {
		t.Errorf("the compact listing carries no identifier:\n%s", compact.out)
	}
}

// workbenchRows answers the rows `dinah workbenches --json` prints for a path,
// decoded as maps rather than into a struct, because the criterion is about
// which keys the rendered JSON carries and a struct would supply a zero value
// for a key that was never there.
func workbenchRows(t *testing.T, from, path string) []map[string]any {
	t.Helper()
	got := runCLI(t, from, "--json", "workbenches", path)
	if got.code != 0 {
		t.Fatalf("workbenches --json: %d %s", got.code, got.errw)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(got.out), &rows); err != nil {
		t.Fatalf("the listing did not decode: %v\n%s", err, got.out)
	}
	return rows
}

// TestTwoBranchesAppendingToOneJournalMergeWithoutAConflict asserts the git
// half of dinah-285 AC-12. The .gitattributes a fresh workbench carries names
// git's own union driver for every journal in the tree, so two branches that
// each append a line to one card's journal merge with no conflict and both
// lines survive.
//
// The claim the format document makes is about git's behaviour rather than
// about ours, so it is asserted by running git rather than by reading the
// attributes file, which the bench package already checks.
func TestTwoBranchesAppendingToOneJournalMergeWithoutAConflict(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on the path, so the merge behaviour cannot be exercised")
	}
	tree := resolvedDir(t, emptyTree(t))
	if got := runCLI(t, tree, "init", tree, "--slug", "mg", "--operator", "alka"); got.code != 0 {
		t.Fatalf("init: %d %s", got.code, got.errw)
	}
	root := benchDir(t, tree)
	if got := runCLI(t, tree, "--workbench", root, "add", "a card with a journal"); got.code != 0 {
		t.Fatalf("add: %d %s", got.code, got.errw)
	}
	cards := bench.ListIDs(filepath.Join(root, bench.CardsDir))
	if len(cards) != 1 {
		t.Fatalf("the workbench holds %v cards, wanted one", cards)
	}
	journal := filepath.Join(root, bench.CardsDir, cards[0], bench.JournalName)

	git := func(args ...string) string {
		t.Helper()
		full := append([]string{"-C", root, "-c", "user.name=dinah test", "-c", "user.email=dinah@example.invalid"}, args...)
		out, err := exec.Command("git", full...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return string(out)
	}
	appendLine := func(text string) {
		t.Helper()
		handle, err := os.OpenFile(journal, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			t.Fatalf("open the journal: %v", err)
		}
		if _, err := handle.WriteString(text); err != nil {
			t.Fatalf("append to the journal: %v", err)
		}
		handle.Close()
	}

	git("init", "--quiet", "--initial-branch=trunk")
	git("add", ".")
	git("commit", "--quiet", "-m", "the workbench as it was written")

	git("checkout", "--quiet", "-b", "left")
	appendLine("{\"ts\":\"2026-01-01T00:00:01Z\",\"event\":\"left\"}\n")
	git("commit", "--quiet", "-am", "the left branch appends")

	git("checkout", "--quiet", "trunk")
	git("checkout", "--quiet", "-b", "right")
	appendLine("{\"ts\":\"2026-01-01T00:00:02Z\",\"event\":\"right\"}\n")
	git("commit", "--quiet", "-am", "the right branch appends")

	git("merge", "--no-edit", "-q", "left")

	merged, err := os.ReadFile(journal)
	if err != nil {
		t.Fatalf("read the merged journal: %v", err)
	}
	text := string(merged)
	if strings.Contains(text, "<<<<<<<") {
		t.Fatalf("the merge left a conflict in the journal:\n%s", text)
	}
	for _, want := range []string{"\"left\"", "\"right\""} {
		if !strings.Contains(text, want) {
			t.Errorf("the merged journal lost the %s append:\n%s", want, text)
		}
	}
}

// TestTheProfileDocumentIsUntouchedByTheContainerRule asserts the half of
// dinah-285 AC-14 that is a property of the diff rather than of the build:
// docs/spec/core-profile.md declares no storage format and says nothing about
// where a workbench sits, so the containment rule and the format bump changed
// nothing in it.
//
// The check reads the document rather than the diff, because a test that ran
// git diff would pass on a checkout with no history and would be asserting the
// state of somebody's working tree rather than the state of the contract.
func TestTheProfileDocumentIsUntouchedByTheContainerRule(t *testing.T) {
	source, err := os.ReadFile(filepath.Join(repositoryRoot, "docs", "spec", "core-profile.md"))
	if err != nil {
		t.Fatalf("read the profile: %v", err)
	}
	text := string(source)
	for _, forbidden := range []string{".dinah", "storage format", "container"} {
		if strings.Contains(strings.ToLower(text), strings.ToLower(forbidden)) {
			t.Errorf("the profile mentions %q, and where a workbench sits is a storage fact rather than an interchange one", forbidden)
		}
	}
}

// fmtBaseDefinition composes the small flow these cases instantiate from,
// which is baseDefinition with a title spliced into it.
func fmtBaseDefinition(title string) string {
	return strings.Replace(baseDefinition, "%q", "\""+title+"\"", 1)
}

// TestTheContainerMigrationReportsAWorkbenchItCouldNotMove asserts what the
// sweep does with a workbench somebody is holding: it names the workbench and
// the reason, moves nothing, and carries the finding out in its exit code, so
// an unattended run cannot read as a clean pass.
func TestTheContainerMigrationReportsAWorkbenchItCouldNotMove(t *testing.T) {
	tree := resolvedDir(t, emptyTree(t))
	container := filepath.Join(tree, "project", bench.UserBaseName)
	legacy := legacyContainerBench(t, container)
	cards := bench.ListIDs(filepath.Join(legacy, bench.CardsDir))
	if len(cards) != 1 {
		t.Fatalf("the workbench holds %v cards, wanted one", cards)
	}
	held := filepath.Join(legacy, bench.CardsDir, cards[0], bench.LockName)
	if err := os.WriteFile(held, []byte("{\"holder\":\"alka\"}\n"), 0o644); err != nil {
		t.Fatalf("write the lock: %v", err)
	}

	got := runCLI(t, tree, "check", "--migrate-container", "--yes")
	if got.code != 5 {
		t.Fatalf("a held workbench exited %d, wanted the findings code 5: %s%s", got.code, got.out, got.errw)
	}
	if !strings.Contains(got.out, legacy) {
		t.Errorf("the report does not name the workbench it could not move:\n%s", got.out)
	}
	if !bench.Exists(legacy) {
		t.Error("the sweep moved a workbench somebody was holding")
	}
}

// bareWorkbench writes a workbench directly into a project directory, beside a
// source file, a readme and a git directory, which is the arrangement the
// format allows and the one the containment rule stopped finding. It answers
// the project directory.
func bareWorkbench(t *testing.T, project string) string {
	t.Helper()
	for path, text := range map[string]string{
		filepath.Join(project, "README.md"):      "A project that happens to hold a workbench.\n",
		filepath.Join(project, "src", "main.go"): "package main\n\nfunc main() {}\n",
		filepath.Join(project, ".git", "HEAD"):   "ref: refs/heads/main\n",
	} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	definition, err := bench.ReadDefinition([]byte(fmtBaseDefinition("Bare")))
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	if err := bench.Instantiate(project, "br", "alka", definition); err != nil {
		t.Fatalf("instantiate: %v", err)
	}
	return project
}

// TestTheRefusalSaysWhyABareWorkbenchStoppedBeingFound asserts the half of
// dinah-285 AC-5 a person meets. A workbench sitting outside a container is no
// longer found, and the reader who runs a command inside it has to be told
// that rather than being handed the sentence an empty machine prints. The
// assertion is on the rendered sentence, because the refusal carried the fact
// in its data long before any catalog carried a sentence naming it.
func TestTheRefusalSaysWhyABareWorkbenchStoppedBeingFound(t *testing.T) {
	tree := resolvedDir(t, emptyTree(t))
	project := bareWorkbench(t, filepath.Join(tree, "myproject"))

	got := runCLI(t, project, "status")
	if got.code == 0 {
		t.Fatalf("a bare workbench should not be found, got exit 0: %s", got.out)
	}
	if !strings.Contains(got.errw, project) {
		t.Errorf("the refusal does not name the bare workbench it walked past:\n%s", got.errw)
	}
	if !strings.Contains(got.errw, "--migrate-container") {
		t.Errorf("the refusal does not say how to carry the workbench into a container:\n%s", got.errw)
	}

	// The sentence printed where nothing is there at all is the one this
	// refusal used to print here, and a reader cannot act on a report that
	// says the same thing about two different situations.
	empty := filepath.Join(tree, "nothing")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	nothing := runCLI(t, empty, "status")
	if nothing.code == 0 {
		t.Fatalf("an empty directory should not resolve a workbench, got exit 0: %s", nothing.out)
	}
	if strings.Contains(nothing.errw, "--migrate-container") {
		t.Errorf("a directory with nothing in it was told to migrate a container:\n%s", nothing.errw)
	}
}

// TestThePreviewSaysWhatMovesAndWhatStays asserts that a reader authorizing a
// directory move is told the blast radius rather than a shape word. The lift
// creates a container inside the project directory and moves the workbench's
// own files into it, and the preview says so, because a row reading only
// "(bare)" leaves a reader unable to predict which directory is created or
// what else is involved.
func TestThePreviewSaysWhatMovesAndWhatStays(t *testing.T) {
	tree := resolvedDir(t, emptyTree(t))
	project := bareWorkbench(t, filepath.Join(tree, "myproject"))

	preview := runCLI(t, tree, "check", "--migrate-container")
	if preview.code != 5 {
		t.Fatalf("the preview exited %d, wanted the findings code 5: %s%s", preview.code, preview.out, preview.errw)
	}
	if !strings.Contains(preview.out, project) {
		t.Errorf("the preview does not name the project directory:\n%s", preview.out)
	}
	if !strings.Contains(preview.out, bench.UserBaseName) {
		t.Errorf("the preview does not name the container it would create:\n%s", preview.out)
	}
	if strings.Contains(preview.out, "(bare)") {
		t.Errorf("the preview names the shape and not what happens to the directory:\n%s", preview.out)
	}

	applied := runCLI(t, tree, "check", "--migrate-container", "--yes")
	if applied.code != 0 {
		t.Fatalf("the migration exited %d: %s%s", applied.code, applied.out, applied.errw)
	}
	// The ground the workbench stood on is still where it stood.
	for _, kept := range []string{
		filepath.Join(project, "README.md"),
		filepath.Join(project, "src", "main.go"),
		filepath.Join(project, ".git", "HEAD"),
	} {
		if !bench.Exists(kept) {
			t.Errorf("the lift carried away %s, which is not the workbench's to move", kept)
		}
	}
	if bench.Exists(filepath.Join(project, bench.WorkbenchAnchor)) {
		t.Error("the anchor never moved into the container")
	}
	ids := bench.ListWorkbenchIDs(filepath.Join(project, bench.UserBaseName))
	if len(ids) != 1 {
		t.Fatalf("the container inside the project holds %v, wanted the one workbench that moved", ids)
	}
	entries, err := os.ReadDir(tree)
	if err != nil {
		t.Fatalf("read %s: %v", tree, err)
	}
	var beside []string
	for _, entry := range entries {
		if entry.Name() != filepath.Base(project) {
			beside = append(beside, entry.Name())
		}
	}
	if len(beside) != 0 {
		t.Errorf("the lift created %v beside the project directory, and the container belongs inside it", beside)
	}
}

// TestTheReportSaysAWorkbenchMovedBeforeTheMigrationStopped asserts what an
// operator reads when the move succeeds and the format stamp does not. The two
// are separate writes, so the workbench really is somewhere else, and a report
// naming only the directory it used to occupy would send him looking for it
// there.
func TestTheReportSaysAWorkbenchMovedBeforeTheMigrationStopped(t *testing.T) {
	tree := resolvedDir(t, emptyTree(t))
	project := bareWorkbench(t, filepath.Join(tree, "myproject"))
	anchor := filepath.Join(project, bench.WorkbenchAnchor)
	text, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read the anchor: %v", err)
	}
	stamped := regexp.MustCompile(`(?m)^format: .*$`).ReplaceAllString(string(text), "format: 99")
	if stamped == string(text) {
		t.Fatal("the anchor carries no format line, so this case cannot be arranged")
	}
	if err := os.WriteFile(anchor, []byte(stamped), 0o644); err != nil {
		t.Fatalf("write the anchor: %v", err)
	}

	got := runCLI(t, tree, "check", "--migrate-container", "--yes")
	if got.code != 5 {
		t.Fatalf("a migration that stopped exited %d, wanted the findings code 5: %s%s", got.code, got.out, got.errw)
	}
	ids := bench.ListWorkbenchIDs(filepath.Join(project, bench.UserBaseName))
	if len(ids) != 1 {
		t.Fatalf("the container holds %v, wanted the one workbench that moved before the stamp failed", ids)
	}
	landed := filepath.Join(project, bench.UserBaseName, ids[0])
	if !strings.Contains(got.out, landed) {
		t.Errorf("the report does not say where the workbench went:\n%s", got.out)
	}
	if !strings.Contains(got.out, project) {
		t.Errorf("the report does not name the directory the workbench came from:\n%s", got.out)
	}
}

// TestTheSweepFinishesAWorkbenchThatMovedBeforeItWasStamped asserts the last
// interruption the migration's recoverability story leaves, and the only one
// that hides. Every member of a bare workbench moves before the format stamp
// is written, so a run that stops between the two leaves a workbench sitting
// exactly where the containment rule puts one and still declaring the format
// that predates the rule. It opens, it is found, and it works, which is why a
// sweep that classified it as already contained and returned would walk past
// it on that run and on every run after it.
//
// The interruption here is a real one rather than a state the test wrote. The
// workbench declares a profile revision above this build's ceiling, so the
// members move and Open refuses inside the stamp, which leaves the format line
// untouched at 1. Removing that refusal is then the whole of the fixture, and
// the second sweep meets a workbench in the state a crash leaves.
func TestTheSweepFinishesAWorkbenchThatMovedBeforeItWasStamped(t *testing.T) {
	tree := resolvedDir(t, emptyTree(t))
	project := bareWorkbench(t, filepath.Join(tree, "myproject"))
	anchorWas := filepath.Join(project, bench.WorkbenchAnchor)
	// A bare workbench in the field predates the containment rule, so it
	// declares the format the rule replaced, and that is the workbench the
	// stamp has real work to do on.
	editAnchorAt(t, anchorWas, "format: "+strconv.Itoa(bench.StorageFormat), "format: 1")
	beyond := bench.ProfileName + "/" + strconv.Itoa(bench.ProfileMajor+1) + ".0"
	editAnchorAt(t, anchorWas, "profile: "+bench.ProfileVersion, "profile: "+beyond)

	stopped := runCLI(t, tree, "check", "--migrate-container", "--yes")
	if stopped.code != 5 {
		t.Fatalf("a migration that stopped at the stamp exited %d, wanted the findings code 5: %s%s", stopped.code, stopped.out, stopped.errw)
	}
	container := filepath.Join(project, bench.UserBaseName)
	ids := bench.ListWorkbenchIDs(container)
	if len(ids) != 1 {
		t.Fatalf("the container holds %v, wanted the one workbench whose members moved before the stamp", ids)
	}
	landed := filepath.Join(container, ids[0])
	anchor := filepath.Join(landed, bench.WorkbenchAnchor)
	if !anchorDeclares(t, anchor, "format: 1") {
		t.Fatalf("the stopped run left a format this fixture cannot use:\n%s", readAnchorText(t, anchor))
	}

	editAnchorAt(t, anchor, "profile: "+beyond, "profile: "+bench.ProfileVersion)
	again := runCLI(t, tree, "check", "--migrate-container", "--yes")
	if again.code != 0 {
		t.Fatalf("the second sweep exited %d: %s%s", again.code, again.out, again.errw)
	}
	if !anchorDeclares(t, anchor, "format: "+strconv.Itoa(bench.ContainerFormat)) {
		t.Errorf("the workbench still declares the format it was left with, so the sweep walked past the one interruption it had to finish:\n%s", readAnchorText(t, anchor))
	}
	if got := bench.ListWorkbenchIDs(container); len(got) != 1 || got[0] != ids[0] {
		t.Errorf("the container holds %v, wanted the one directory the interrupted run had already filled at %s", got, landed)
	}
	third := runCLI(t, tree, "check", "--migrate-container", "--yes")
	if third.code != 0 || third.out != again.out {
		t.Errorf("a third sweep read differently from the second, so finishing the interruption is not idempotent:\n%s\n%s", again.out, third.out)
	}
}

// readAnchorText answers an anchor's bytes, so a failure can print what the
// file actually says rather than only that it disagreed.
func readAnchorText(t *testing.T, path string) string {
	t.Helper()
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(text)
}

// anchorDeclares reports whether an anchor carries a frontmatter line, which
// is the reading a test wanting the declared value rather than the value a
// reader would default to has to make.
func anchorDeclares(t *testing.T, path, line string) bool {
	t.Helper()
	for _, each := range strings.Split(readAnchorText(t, path), "\n") {
		if strings.TrimRight(each, "\r") == line {
			return true
		}
	}
	return false
}

// TestTheSweepLeavesAContainedWorkbenchItCannotOpenAlone asserts that a healthy
// workbench a later release wrote is reported as already contained rather than
// as a migration failure, and that the preview and the applying run say the
// same thing about it.
//
// A machine can hold one board written by a newer build and another this one
// still has to carry forward, and the sweep meets both in the same walk.
// Nothing about the newer board needs carrying: it already sits where the rule
// puts one, under a minted name, declaring the format the rule arrived at. A
// sweep that reported it as a failure would exit on findings every time it ran
// unattended, and it would do so over a workbench that is not this build's
// business to read at all.
//
// The two runs are compared against each other rather than only against an
// expectation, because MigrateContainerTree's own promise is that a preview
// classifies every workbench exactly as a migration does. That promise is
// falsifiable only by running both over one unchanged tree.
func TestTheSweepLeavesAContainedWorkbenchItCannotOpenAlone(t *testing.T) {
	tree := resolvedDir(t, emptyTree(t))
	project := filepath.Join(tree, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := runCLI(t, project, "init", project, "--slug", "nw", "--operator", "alka"); got.code != 0 {
		t.Fatalf("init at %s: %d %s", project, got.code, got.errw)
	}
	written := benchDir(t, project)
	anchor := filepath.Join(written, bench.WorkbenchAnchor)
	beyond := bench.ProfileName + "/" + strconv.Itoa(bench.ProfileMajor+99) + ".0"
	editAnchorAt(t, anchor, "profile: "+bench.ProfileVersion, "profile: "+beyond)
	if !anchorDeclares(t, anchor, "format: "+strconv.Itoa(bench.ContainerFormat)) {
		t.Fatalf("the fixture does not declare the format the containment rule arrived at:\n%s", readAnchorText(t, anchor))
	}
	before := readAnchorText(t, anchor)

	preview := runCLI(t, tree, "--json", "check", "--migrate-container")
	if preview.code != 0 {
		t.Fatalf("the preview exited %d over a tree with nothing to carry forward: %s%s", preview.code, preview.out, preview.errw)
	}
	applied := runCLI(t, tree, "--json", "check", "--migrate-container", "--yes")
	if applied.code != 0 {
		t.Fatalf("the applying run exited %d over a workbench it had no work to do on: %s%s", applied.code, applied.out, applied.errw)
	}
	// The two runs are compared through the machine form with the preview flag
	// itself dropped, because that flag is the one thing the two are entitled
	// to disagree about. What is left is the classification, which is what
	// MigrateContainerTree promises the two share.
	previewClass, appliedClass := classification(t, preview.out), classification(t, applied.out)
	if previewClass != appliedClass {
		t.Errorf("the preview and the applying run classify the same unchanged tree differently:\npreview: %s\napplied: %s", previewClass, appliedClass)
	}
	if !strings.Contains(appliedClass, jsonPath(written)) {
		t.Errorf("the run does not report the workbench as already contained:\n%s", applied.out)
	}
	if strings.Contains(appliedClass, "failed") {
		t.Errorf("a workbench with nothing to carry forward was reported as a migration failure:\n%s", applied.out)
	}
	if got := readAnchorText(t, anchor); got != before {
		t.Errorf("the sweep wrote to a workbench it had no work to do on:\nbefore:\n%s\nafter:\n%s", before, got)
	}
}

// classification answers a container run's machine form with the preview flag
// removed, which is the report's verdict on a tree stripped of the one field
// that says which of the two runs produced it.
func classification(t *testing.T, out string) string {
	t.Helper()
	var report map[string]any
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("the machine form did not decode: %v\n%s", err, out)
	}
	delete(report, "preview")
	verdict, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	return string(verdict)
}

// jsonPath spells a path the way the machine form carries it, since a Windows
// separator is escaped inside a JSON string and a test comparing against a
// composed path has to ask the same encoder.
func jsonPath(path string) string {
	encoded, err := json.Marshal(path)
	if err != nil {
		return path
	}
	return strings.Trim(string(encoded), "\"")
}

// TestOneSweepRefusesEveryHeldWorkbenchItMeets asserts that the two repairs and
// the stamp answer a held workbench the same way, in one run over one tree.
//
// The sweep is meant to run over boards somebody is using, so the workbench it
// meets under a lock is the ordinary case rather than the exotic one. A bare
// workbench under a lock is refused because the lift would move the directory
// the lock sits in. A contained workbench still owed its format stamp is
// refused for a different reason, since nothing moves there: the stamp is a
// read-modify-write of the whole anchor, so a writer holding the lock and
// saving the anchor either loses its edit or swallows the stamp.
//
// Both halves are asserted in one run because the defect this guards against
// was invisible in any run holding only one of them. A sweep that refused the
// bare workbench and wrote to the contained one read as a sweep that refuses
// held workbenches.
func TestOneSweepRefusesEveryHeldWorkbenchItMeets(t *testing.T) {
	tree := resolvedDir(t, emptyTree(t))
	bare := bareWorkbench(t, filepath.Join(tree, "myproject"))
	bareAnchor := filepath.Join(bare, bench.WorkbenchAnchor)
	editAnchorAt(t, bareAnchor, "format: "+strconv.Itoa(bench.StorageFormat), "format: 1")

	contained := filepath.Join(tree, "other")
	if err := os.MkdirAll(contained, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if got := runCLI(t, contained, "init", contained, "--slug", "ct", "--operator", "alka"); got.code != 0 {
		t.Fatalf("init at %s: %d %s", contained, got.code, got.errw)
	}
	held := benchDir(t, contained)
	heldAnchor := filepath.Join(held, bench.WorkbenchAnchor)
	// The stamp has real work to do only on a workbench interrupted between
	// its last move and its stamp, which is a contained workbench under a
	// minted name still declaring the format the rule replaced.
	editAnchorAt(t, heldAnchor, "format: "+strconv.Itoa(bench.StorageFormat), "format: 1")

	for _, root := range []string{bare, held} {
		if err := os.WriteFile(filepath.Join(root, bench.LockName), []byte("{\"holder\":\"alka\"}\n"), 0o644); err != nil {
			t.Fatalf("plant a lock in %s: %v", root, err)
		}
	}
	bareWas, heldWas := readAnchorText(t, bareAnchor), readAnchorText(t, heldAnchor)

	swept := runCLI(t, tree, "check", "--migrate-container", "--yes")
	if swept.code != 5 {
		t.Fatalf("a sweep over two held workbenches exited %d, wanted the findings code 5: %s%s", swept.code, swept.out, swept.errw)
	}
	for _, root := range []string{bare, held} {
		if !strings.Contains(swept.out, root) {
			t.Errorf("the report does not name the held workbench at %s:\n%s", root, swept.out)
		}
	}
	if strings.Count(swept.out, "dinah.locked") != 2 {
		t.Errorf("the sweep reports %d lock refusals, wanted one for each of the two held workbenches:\n%s", strings.Count(swept.out, "dinah.locked"), swept.out)
	}
	if got := readAnchorText(t, bareAnchor); got != bareWas {
		t.Errorf("the sweep wrote to the held bare workbench:\n%s", got)
	}
	if got := readAnchorText(t, heldAnchor); got != heldWas {
		t.Errorf("the sweep stamped the held contained workbench, so the stamp is the one write here that walks past a lock:\n%s", got)
	}
	if bench.Exists(filepath.Join(bare, bench.UserBaseName)) {
		t.Error("the refused lift created the container anyway")
	}
}
