package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
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
// dinah check reports it until dinah accept-divergence clears it. The
// migration's report names each kept divergence by the reference dinah check
// names it by, which is dinah-637/questions/29's second part: besides the
// fixture's own hand-edited card comment, the test edits the second comment of
// the fixture's second question by hand before migrating, so one of the two
// hangs on an item that is not the checklist's first.
//
// Arming: composing the report's reference from the item's place in the whole
// checklist, as the report did, names the question's comment
// fx-1/checklist/8/comments/2 where check names it fx-1/questions/2/comments/2,
// which the comparison reports; and naming a comment by its ordinal, as check
// did, names that comment fx-1/questions/2/comments/4, which reaches nothing,
// so accept-divergence over it is refused.
// TestARerunOverAMigratedStoreAnswersAlreadyMigrated drives the rerun clause
// of dinah-637/criteria/13. A second dinah check --migrate-storage on a store
// the migration finished answers that the store is already migrated, in the
// human answer by the catalogue sentence and in the --json answer by
// already_migrated, names no claim passed, and writes nothing. It is asked
// with a card claimed, and it answers without the workbench-in-use refusal,
// since a run with nothing to migrate has no claims to be refused over; the
// forced form answers the same and names no claim either.
//
// Arming: answering the rerun with the zero-count account, as the command did,
// prints "The store now declares format 12" and no already sentence; and
// asking about claims before the format refuses the unforced rerun
// dinah.workbench-in-use.
func TestARerunOverAMigratedStoreAnswersAlreadyMigrated(t *testing.T) {
	store, root := migratedFixture(t)
	at := func(args ...string) invocation {
		return runCLI(t, root, append([]string{"--workbench", store}, args...)...)
	}
	// A claim a session left standing, planted as the card file records one,
	// as TestTheMigrationIsTheOperatorsAndRefusedOverClaims plants it.
	anchor := filepath.Join(store, bench.CardsDir, "8fadf319a45a", bench.CardAnchor)
	text, err := os.ReadFile(anchor)
	if err != nil {
		t.Fatalf("read %s: %v", anchor, err)
	}
	fm, body := bench.ParseAnchor(string(text))
	fm.Set("state", "claimed")
	fm.Set("claim_holder", "alex")
	fm.Set("claim_since", "2026-09-28T09:00:00Z")
	if err := os.WriteFile(anchor, []byte(fm.Render(body)), 0o644); err != nil {
		t.Fatalf("plant the claim: %v", err)
	}
	before := redactedTreeBytes(t, store)

	human := at("check", "--migrate-storage")
	if human.code != 0 || !strings.Contains(human.out, "The store already declares format 12, so there is nothing to migrate.") {
		t.Errorf("the rerun answered %d:\n%s%s", human.code, human.out, human.errw)
	}
	for _, stale := range []string{"now declares format", "alex", "Cards"} {
		if strings.Contains(human.out, stale) {
			t.Errorf("the rerun's answer carries %q, which belongs to a run that migrated something:\n%s", stale, human.out)
		}
	}

	forced := at("--json", "check", "--migrate-storage", "--force-claims")
	var answer struct {
		Outcome         string   `json:"outcome"`
		AlreadyMigrated *bool    `json:"already_migrated"`
		ClaimsPassed    []string `json:"claims_passed"`
	}
	if err := json.Unmarshal([]byte(forced.out), &answer); err != nil {
		t.Fatalf("decode the forced rerun's answer %q (%s): %v", forced.out, forced.errw, err)
	}
	if forced.code != 0 || answer.Outcome != "ok" || answer.AlreadyMigrated == nil || !*answer.AlreadyMigrated || len(answer.ClaimsPassed) != 0 {
		t.Errorf("the forced rerun answered %d %s", forced.code, forced.out)
	}
	if redactedTreeBytes(t, store) != before {
		t.Error("a rerun over the migrated store changed it")
	}

	// The one thing a rerun over a migrated store still does is carry a
	// comment.md an older build wrote since. While another process holds the
	// card's lock the rerun stops and says so, still as a run over a store
	// already migrated; once the lock is given back it carries the stray and
	// names it.
	cardDir := filepath.Join(store, bench.CardsDir, "7035845dc37b")
	stray := filepath.Join(cardDir, bench.CommentsDir, "0badc0ffee01", "comment.md")
	if err := os.MkdirAll(filepath.Dir(stray), 0o755); err != nil {
		t.Fatalf("make the stray's directory: %v", err)
	}
	if err := os.WriteFile(stray, []byte("---\nts: 2026-09-29T09:00:00Z\nauthor: an older build\nordinal: 99\n---\nwritten by an older build\n"), 0o644); err != nil {
		t.Fatalf("plant the stray: %v", err)
	}
	held, err := bench.Acquire(cardDir, "another process", "2026-09-29T09:00:01Z")
	if err != nil {
		t.Fatalf("hold the card's lock: %v", err)
	}
	stopped := at("check", "--migrate-storage")
	held.Release()
	if stopped.code != 5 || !strings.Contains(stopped.out, "The store already declares format 12") || !strings.Contains(stopped.out, "another process") {
		t.Errorf("the rerun over a held card answered %d:\n%s%s", stopped.code, stopped.out, stopped.errw)
	}
	carried := at("check", "--migrate-storage")
	if carried.code != 0 || !strings.Contains(carried.out, "The store already declares format 12") || !strings.Contains(carried.out, "0badc0ffee01") {
		t.Errorf("the rerun carrying a stray answered %d:\n%s%s", carried.code, carried.out, carried.errw)
	}
	if _, err := os.Stat(stray); !os.IsNotExist(err) {
		t.Errorf("the stray survived the rerun: %v", err)
	}
}

func TestADivergenceIsCarriedAndStillReported(t *testing.T) {
	store, root, migrated := migratedFixtureReport(t, func(store string) {
		anchor := filepath.Join(store, bench.CardsDir, "7035845dc37b", "checklist", "87b95a15aacc", bench.CommentsDir, "6ebdf797b337", "comment.md")
		text, err := os.ReadFile(anchor)
		if err != nil {
			t.Fatalf("read %s: %v", anchor, err)
		}
		if err := os.WriteFile(anchor, append(text, []byte("edited by hand after its digest was recorded\n")...), 0o644); err != nil {
			t.Fatalf("edit %s: %v", anchor, err)
		}
	})
	var migration struct {
		Divergences []string `json:"divergences"`
	}
	if err := json.Unmarshal([]byte(migrated), &migration); err != nil {
		t.Fatalf("decode the migration's report %s: %v", migrated, err)
	}
	at := func(args ...string) invocation {
		return runCLI(t, root, append([]string{"--workbench", store}, args...)...)
	}
	got := at("check", "--json")
	var report struct {
		Findings []bench.Finding `json:"findings"`
	}
	json.Unmarshal([]byte(got.out), &report)
	var diverged []string
	for _, finding := range report.Findings {
		if finding.Key == bench.FindingCommentBodyDiverged {
			diverged = append(diverged, finding.Detail)
		}
	}
	if len(diverged) != 2 {
		t.Fatalf("check on the migrated fixture reports the diverged comments %v, wanted the fixture's own and the one edited here: %s", diverged, got.out)
	}
	sort.Strings(diverged)
	kept := append([]string(nil), migration.Divergences...)
	sort.Strings(kept)
	if strings.Join(kept, " ") != strings.Join(diverged, " ") {
		t.Errorf("the migration's report names the kept divergences %v, and check names the comments %v", kept, diverged)
	}
	for _, reference := range diverged {
		if got := at("accept-divergence", reference); got.code != 0 {
			t.Fatalf("accept-divergence %s: %d %s", reference, got.code, got.errw)
		}
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

// TestAMigratedStoreIsRefusedByTheSwitchOffBuild drives the terminal half of
// dinah-637/criteria/39. The migration fixture, migrated with the layout
// switched on, is refused unsupported-version naming format 12 by dinah check
// and by show once the layout is switched off again, which is what every
// build whose highest format is 11 answers; with the layout switched on,
// show of a card, a comment and an item answers.
//
// Arming: comparing the opener's integer test against StorageFormat rather
// than the effective ceiling opens the store with the switch off, and the
// refusal assertions fail.
func TestAMigratedStoreIsRefusedByTheSwitchOffBuild(t *testing.T) {
	store, root := fixtureCopy(t)
	at := func(args ...string) invocation {
		return runCLI(t, root, append([]string{"--workbench", store}, args...)...)
	}
	t.Run("migrate", func(st *testing.T) {
		bench.EnableCardUnitForTest(st)
		if got := at("check", "--migrate-storage", "--backup", filepath.Join(st.TempDir(), "backup")); got.code != 0 {
			st.Fatalf("migrate: %d %s%s", got.code, got.out, got.errw)
		}
	})
	for _, args := range [][]string{{"check"}, {"show", "fx-1"}} {
		got := at(args...)
		if !storageRefused(got, contract.UnsupportedVer) || !strings.Contains(got.errw, "format 12") {
			t.Errorf("dinah %s with the layout switched off answered %d %q, wanted %s naming format 12", strings.Join(args, " "), got.code, got.errw, contract.UnsupportedVer)
		}
	}
	bench.EnableCardUnitForTest(t)
	for _, ref := range []string{"fx-1", "fx-1/comments/1", "fx-1/checklist/1"} {
		if got := at("show", ref); got.code != 0 || strings.TrimSpace(got.out) == "" {
			t.Errorf("show %s with the layout switched on answered %d %q", ref, got.code, got.errw)
		}
	}
}

// TestExportIsTheSameBeforeAndAfterTheMigration drives the export clause of
// dinah-637/criteria/21. The interchange form carries the workbench's
// definition and no card, so dinah export of the migration fixture answers
// the same bytes before the storage migration, with the layout switched off
// as main ships it, and after, with the layout switched on.
//
// Arming: an export that listed a card's comments would differ across the
// migration, since the one before reads directories and the one after the
// journal.
func TestExportIsTheSameBeforeAndAfterTheMigration(t *testing.T) {
	store, root := fixtureCopy(t)
	at := func(args ...string) invocation {
		return runCLI(t, root, append([]string{"--workbench", store}, args...)...)
	}
	before := at("export")
	if before.code != 0 {
		t.Fatalf("export before the migration: %d %s", before.code, before.errw)
	}
	bench.EnableCardUnitForTest(t)
	if got := at("check", "--migrate-storage", "--backup", filepath.Join(t.TempDir(), "backup")); got.code != 0 {
		t.Fatalf("migrate: %d %s%s", got.code, got.out, got.errw)
	}
	after := at("export")
	if after.code != 0 {
		t.Fatalf("export after the migration: %d %s", after.code, after.errw)
	}
	if after.out != before.out {
		t.Errorf("the export changed across the migration:\n%s", diffLines(before.out, after.out))
	}
}
