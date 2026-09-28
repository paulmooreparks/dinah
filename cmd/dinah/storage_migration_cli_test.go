package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// fixtureCopy copies the migration fixture into a .dinah container and
// answers the store and the directory commands run from, with the operator
// as the actor and the layout switched off, as main ships it.
func fixtureCopy(t *testing.T) (store, root string) {
	t.Helper()
	id, err := os.ReadFile(filepath.Join(storageFixture, "workbench-id.txt"))
	if err != nil {
		t.Fatalf("read the fixture's identifier: %v", err)
	}
	base := t.TempDir()
	root = filepath.Join(base, "wb")
	store = filepath.Join(root, bench.UserBaseName, strings.TrimSpace(string(id)))
	copyFixtureTree(t, filepath.Join(storageFixture, "before"), store)
	t.Setenv("DINAH_HOME", filepath.Join(base, "home"))
	t.Setenv("DINAH_ACTOR", "sam")
	t.Setenv("DINAH_LANG", "")
	t.Setenv("DINAH_FORMAT", "")
	t.Setenv("DINAH_WORKBENCH", "")
	return store, root
}

// storageRefused reports whether an invocation was refused under a name.
func storageRefused(got invocation, name string) bool {
	return got.code == contract.ExitCode(contract.OutcomeRefused) && strings.HasPrefix(strings.TrimSpace(got.errw), name)
}

// TestTheSwitchOffBuildRehearsesAndRefusesToWrite drives the switch-off half
// of dinah-637/criteria/11: with the layout switched off, every verb opens the
// format-11 fixture and writes the old layout, a writing migration is refused
// dinah.migration-awaits-capabilities, and a rehearsal runs.
//
// Arming: dropping the switch test in Library.MigrateStorage lets the writing
// run through, which reddens the refusal assertion and leaves the fixture
// migrated.
func TestTheSwitchOffBuildRehearsesAndRefusesToWrite(t *testing.T) {
	store, root := fixtureCopy(t)
	at := func(args ...string) invocation {
		return runCLI(t, root, append([]string{"--workbench", store}, args...)...)
	}
	if got := at("comment", "fx-1", "a comment the old layout keeps"); got.code != 0 {
		t.Fatalf("comment: %d %s", got.code, got.errw)
	}
	if got := at("file", "fx-1", "decision", "a decision the old layout keeps"); got.code != 0 {
		t.Fatalf("file: %d %s", got.code, got.errw)
	}
	anchors := 0
	filepath.WalkDir(store, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && (entry.Name() == "comment.md" || entry.Name() == "item.md") {
			anchors++
		}
		return err
	})
	if anchors == 0 {
		t.Fatal("the switch-off build wrote no member file")
	}
	if got := at("check", "--migrate-storage", "--backup", filepath.Join(t.TempDir(), "backup")); !storageRefused(got, contract.MigrationAwaitsCapabilities) {
		t.Errorf("a writing migration with the layout switched off answered %d %q, wanted %s", got.code, got.errw, contract.MigrationAwaitsCapabilities)
	}
	got := at("check", "--migrate-storage", "--rehearse", "--json")
	if got.code != 0 {
		t.Fatalf("the rehearsal: %d %s", got.code, got.errw)
	}
	var report bench.StorageMigration
	if err := json.Unmarshal([]byte(got.out), &report); err != nil || !report.Rehearsal || report.Manifest.Before != report.Manifest.After {
		t.Errorf("the rehearsal answered %v %+v", err, report)
	}
	opened, err := bench.Open(store)
	if err != nil || opened.Format != bench.CardUnitFormat-1 {
		t.Errorf("after the rehearsal the store opens at format %v (%v), wanted it untouched at 11", opened, err)
	}
}

// TestTheSwitchOnCheckReportsTheStoreAwaitingItsMigration drives the check
// clause of dinah-637/criteria/11: with the layout switched on, dinah check on
// a format-11 store reports that the store awaits the storage migration and
// every file a precondition of that migration would refuse, and nothing else,
// while an ordinary verb is refused naming the migration.
//
// Arming: dropping the short circuit in Library.Check runs the ordinary
// checks over the old layout, which reddens the nothing-else assertion.
func TestTheSwitchOnCheckReportsTheStoreAwaitingItsMigration(t *testing.T) {
	bench.EnableCardUnitForTest(t)
	store, root := fixtureCopy(t)
	at := func(args ...string) invocation {
		return runCLI(t, root, append([]string{"--workbench", store}, args...)...)
	}
	if got := at("show", "fx-1"); !storageRefused(got, contract.StoreAwaitingMigration) || !strings.Contains(got.errw, "--migrate-storage") {
		t.Errorf("show on a format-11 store with the layout on answered %d %q, wanted %s naming the migration", got.code, got.errw, contract.StoreAwaitingMigration)
	}
	// One file the migration would refuse: a comment with an unknown key.
	comments, _ := bench.ListIDs(filepath.Join(store, bench.CardsDir, "7035845dc37b", bench.CommentsDir))
	anchor := filepath.Join(store, bench.CardsDir, "7035845dc37b", bench.CommentsDir, comments[0], "comment.md")
	text, _ := os.ReadFile(anchor)
	os.WriteFile(anchor, []byte(strings.Replace(string(text), "---\n", "---\nmood: grumpy\n", 1)), 0o644)
	got := at("check", "--json")
	var report struct {
		Findings []bench.Finding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(got.out), &report); err != nil {
		t.Fatalf("decode check: %v\n%s%s", err, got.out, got.errw)
	}
	keys := map[string]int{}
	for _, finding := range report.Findings {
		keys[finding.Key]++
	}
	if keys[bench.FindingStoreAwaitingMigration] != 1 || keys[bench.FindingStoragePrecondition] != 1 || len(report.Findings) != 2 {
		t.Errorf("check reports %v, wanted the store awaiting migration and the one precondition, and nothing else", keys)
	}
}

// TestTheMigrationIsTheOperatorsAndRefusedOverClaims drives the refusals of
// dinah-637/criteria/12 that the library raises: a non-operator is refused,
// a claimed card refuses the run naming it, and a forced run names the card
// in its answer and on its own line of the workbench journal.
func TestTheMigrationIsTheOperatorsAndRefusedOverClaims(t *testing.T) {
	bench.EnableCardUnitForTest(t)
	store, root := fixtureCopy(t)
	backup := filepath.Join(t.TempDir(), "backup")
	at := func(args ...string) invocation {
		return runCLI(t, root, append([]string{"--workbench", store}, args...)...)
	}
	if got := at("--actor", "someone", "check", "--migrate-storage", "--backup", backup); !storageRefused(got, contract.NotOperator) {
		t.Errorf("a non-operator's migration answered %d %q, wanted %s", got.code, got.errw, contract.NotOperator)
	}
	// A claim a session left standing, planted as the card file records one.
	anchor := filepath.Join(store, bench.CardsDir, "8fadf319a45a", bench.CardAnchor)
	text, _ := os.ReadFile(anchor)
	fm, body := bench.ParseAnchor(string(text))
	fm.Set("state", "claimed")
	fm.Set("claim_holder", "alex")
	fm.Set("claim_since", "2026-09-28T09:00:00Z")
	os.WriteFile(anchor, []byte(fm.Render(body)), 0o644)
	got := at("check", "--migrate-storage", "--backup", backup)
	if !storageRefused(got, contract.WorkbenchInUse) || !strings.Contains(got.errw, "alex") {
		t.Errorf("a migration over a claimed card answered %d %q, wanted %s naming the card and its holder", got.code, got.errw, contract.WorkbenchInUse)
	}
	got = at("check", "--migrate-storage", "--backup", backup, "--force-claims", "--json")
	if got.code != 0 {
		t.Fatalf("the forced migration: %d %s", got.code, got.errw)
	}
	var report bench.StorageMigration
	if err := json.Unmarshal([]byte(got.out), &report); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(report.ClaimsPassed) != 1 {
		t.Errorf("the forced run names the claims %v, wanted the one card", report.ClaimsPassed)
	}
	journal, _ := os.ReadFile(filepath.Join(store, bench.JournalName))
	if !strings.Contains(string(journal), `"event":"storage_migrated"`) || !strings.Contains(string(journal), `"cards":["`+report.ClaimsPassed[0]+`"]`) {
		t.Errorf("the workbench journal's storage_migrated line does not name the passed card")
	}
}

// TestADivergenceIsCarriedAndStillReported drives the divergence clause of
// dinah-637/criteria/13: a comment whose body was edited by hand after its
// digest was recorded is carried with that digest, and after the migration
// dinah check reports it until dinah accept-divergence clears it.
func TestADivergenceIsCarriedAndStillReported(t *testing.T) {
	store, root := migratedFixture(t)
	at := func(args ...string) invocation {
		return runCLI(t, root, append([]string{"--workbench", store}, args...)...)
	}
	got := at("check", "--json")
	var report struct {
		Findings []bench.Finding `json:"findings"`
	}
	json.Unmarshal([]byte(got.out), &report)
	diverged := ""
	for _, finding := range report.Findings {
		if finding.Key == bench.FindingCommentBodyDiverged {
			diverged = finding.Detail
		}
	}
	if diverged == "" {
		t.Fatalf("check on the migrated fixture reports no diverged comment: %s", got.out)
	}
	if got := at("accept-divergence", diverged); got.code != 0 {
		t.Fatalf("accept-divergence %s: %d %s", diverged, got.code, got.errw)
	}
	got = at("check", "--json")
	json.Unmarshal([]byte(got.out), &report)
	for _, finding := range report.Findings {
		if finding.Key == bench.FindingCommentBodyDiverged {
			t.Errorf("check still reports %s after accept-divergence", finding.Detail)
		}
	}
}

// TestAColumnCommentFollowsItsColumnAcrossTheMigration drives
// dinah-637/criteria/24. The fixture's archived column carries a comment,
// which after the migration resolves in the archived half alone; and a
// column deleted after the migration takes its comments with it while every
// other column keeps its own.
func TestAColumnCommentFollowsItsColumnAcrossTheMigration(t *testing.T) {
	store, root := migratedFixture(t)
	at := func(args ...string) invocation {
		return runCLI(t, root, append([]string{"--workbench", store}, args...)...)
	}
	// The comment reads in the archived half of its column and in no other.
	// A reference through the archived column is refused as the merge-base
	// build refused it, dinah.not-archived advising the column's restore,
	// which the goldens pin byte for byte; the half is asked of the
	// workbench journal's replay directly.
	opened, err := bench.Open(store)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	parked, err := opened.ArchivedColumnByRef("parked")
	if err != nil || parked == nil {
		t.Fatalf("the fixture's archived column: %v", err)
	}
	archived, _ := opened.ColumnComments(parked.ID, bench.ArchivedHalf)
	live, _ := opened.ColumnComments(parked.ID, bench.LiveHalf)
	if len(archived) != 1 || len(live) != 0 {
		t.Errorf("the archived column's comment reads %d times in the archived half and %d in the live, wanted once and never", len(archived), len(live))
	}
	if got := at("column", "new", "Scratch"); got.code != 0 {
		t.Fatalf("add a column: %d %s", got.code, got.errw)
	}
	if got := at("comment", "scratch", "a comment on a column about to go"); got.code != 0 {
		t.Fatalf("comment on the column: %d %s", got.code, got.errw)
	}
	before := at("show", "review/comments/1").out
	if got := at("delete", "scratch", "--yes"); got.code != 0 {
		t.Fatalf("delete the column: %d %s", got.code, got.errw)
	}
	for _, args := range [][]string{{"show", "scratch/comments/1"}, {"show", "scratch/comments/1", "--archived"}} {
		if got := at(args...); got.code == 0 {
			t.Errorf("dinah %s still answers after the column was deleted: %s", strings.Join(args, " "), got.out)
		}
	}
	if after := at("show", "review/comments/1").out; after != before {
		t.Errorf("another column's comment changed when a column was deleted:\n%s", diffLines(before, after))
	}
}
