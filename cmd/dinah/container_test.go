package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
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
