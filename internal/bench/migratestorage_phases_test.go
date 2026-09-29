package bench

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/contract"
	"dinah/internal/durable"
)

// mainCard is the fixture's card carrying every kind of member.
const mainCard = "7035845dc37b"

// migrateFixture runs the migration once over a store, opened the way dinah
// check opens it, and answers the run's account and its error.
func migrateFixture(t *testing.T, store string, run StorageMigrationRun) (*StorageMigration, error) {
	t.Helper()
	return openForMigration(t, store).MigrateStorage(run)
}

// plant sets one of the migration's test hooks for the length of a test.
func plant[T any](t *testing.T, hook *T, value T) {
	t.Helper()
	previous := *hook
	*hook = value
	t.Cleanup(func() { *hook = previous })
}

// memberAnchors counts every comment.md and item.md below a store.
func memberAnchors(t *testing.T, store string) int {
	t.Helper()
	count := 0
	err := filepath.WalkDir(store, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && (entry.Name() == legacyAnchorOf(KindComment) || entry.Name() == legacyAnchorOf(KindItem)) {
			count++
		}
		return err
	})
	if err != nil {
		t.Fatalf("walk %s: %v", store, err)
	}
	return count
}

// uninterrupted is the account a run over a fresh copy of the fixture gives
// when nothing stops it.
func uninterrupted(t *testing.T) *StorageMigration {
	t.Helper()
	store := copyMigrationFixture(t)
	report, err := migrateFixture(t, store, migrationRun(filepath.Join(t.TempDir(), "backup")))
	if err != nil || report.Outcome != contract.ReadOK {
		t.Fatalf("the uninterrupted run: %v %+v", err, report)
	}
	return report
}

// TestAProofDifferenceStopsTheRunBeforeAnythingIsDeleted drives the second
// half of dinah-637/criteria/2. A baseline writer that drops a comment's
// baseline, drops an item's resolution, or drops one of an item's citations
// is caught by the proof at phase 2, which names the manifest line that
// differs and stops the run with every member file still standing; and a
// manifest line altered between an interrupted phase 3 and its rerun stops
// the rerun at phase 2 too.
//
// Arming: making the proof compare the manifest's line count rather than its
// lines lets the resolution and citation plants through, since each changes a
// line without adding or removing one.
func TestAProofDifferenceStopsTheRunBeforeAnythingIsDeleted(t *testing.T) {
	EnableCardUnitForTest(t)
	plants := []struct {
		name  string
		plant func(ev *Event) bool
		kind  string
	}{
		{"a dropped comment baseline", func(ev *Event) bool {
			return !(ev.Event == contract.EventCommentBaseline && strings.HasPrefix(ev.Text, "a first card comment"))
		}, KindComment},
		{"an item baseline whose resolution was dropped", func(ev *Event) bool {
			if ev.Event == contract.EventItemBaseline && ev.Resolution != "" {
				ev.Resolution = ""
			}
			return true
		}, KindItem},
		{"an item baseline missing one citation", func(ev *Event) bool {
			if ev.Event == contract.EventItemBaseline && len(ev.Citations) > 1 {
				ev.Citations = ev.Citations[:1]
			}
			return true
		}, KindItem},
	}
	for _, tc := range plants {
		t.Run(tc.name, func(t *testing.T) {
			store := copyMigrationFixture(t)
			anchors := memberAnchors(t, store)
			plant(t, &storageBaselinePlant, tc.plant)
			report, err := migrateFixture(t, store, migrationRun(filepath.Join(t.TempDir(), "backup")))
			if err != nil {
				t.Fatalf("migrate: %v", err)
			}
			if report.Outcome != contract.ReadFindings || report.Phase != StoragePhaseProof {
				t.Fatalf("the run answered %s at phase %q, wanted findings at phase 2", report.Outcome, report.Phase)
			}
			named := false
			for _, difference := range report.Differences {
				named = named || strings.HasPrefix(difference.Key, tc.kind+"/")
			}
			if !named {
				t.Errorf("the run names the differences %+v, wanted a %s line among them", report.Differences, tc.kind)
			}
			if got := memberAnchors(t, store); got != anchors {
				t.Errorf("the stopped run left %d member files of the %d the store held", got, anchors)
			}
		})
	}

	t.Run("a manifest line altered between an interrupted removal and its rerun", func(t *testing.T) {
		store := copyMigrationFixture(t)
		refused := 0
		plant(t, &storageMigrationStep, func(op, path string) error {
			if refused == 0 && op == "remove" {
				refused++
				return errors.New("planted: the removal is interrupted")
			}
			return nil
		})
		report, err := migrateFixture(t, store, migrationRun(filepath.Join(t.TempDir(), "backup")))
		if err != nil || report.Outcome != contract.ReadFindings || report.Phase != StoragePhaseRemoval {
			t.Fatalf("the interrupted removal answered %v %+v", err, report)
		}
		state := readStateFile(t, store)
		altered := ""
		for i, line := range state.Manifest.Lines {
			if strings.HasPrefix(line, "comment\t") {
				altered = manifestKey(line)
				state.Manifest.Lines[i] = line + "x"
				break
			}
		}
		writeStateFile(t, store, state)
		storageMigrationStep = nil
		report, err = migrateFixture(t, store, StorageMigrationRun{Template: migrationRun("").Template})
		if err != nil {
			t.Fatalf("the rerun: %v", err)
		}
		if report.Outcome != contract.ReadFindings || report.Phase != StoragePhaseProof || !report.RemovalStarted {
			t.Fatalf("the rerun answered %s at phase %q removal started %v, wanted findings at phase 2 after removal began", report.Outcome, report.Phase, report.RemovalStarted)
		}
		if len(report.Differences) != 1 || report.Differences[0].Key != altered {
			t.Errorf("the rerun names %+v, wanted the altered line %s alone", report.Differences, altered)
		}
		if report.Backup == nil || report.Backup.Path == "" {
			t.Errorf("the rerun names no backup to restore")
		}
	})
}

// readStateFile reads a store's storage-migration.json.
func readStateFile(t *testing.T, store string) *storageState {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(store, StorageMigrationFile))
	if err != nil {
		t.Fatalf("read the progress file: %v", err)
	}
	var state storageState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("decode the progress file: %v", err)
	}
	return &state
}

// writeStateFile writes a store's storage-migration.json.
func writeStateFile(t *testing.T, store string, state *storageState) {
	t.Helper()
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("encode the progress file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(store, StorageMigrationFile), data, 0o644); err != nil {
		t.Fatalf("write the progress file: %v", err)
	}
}

// TestAnInterruptedRunResumesToTheSameResult drives the interruption half of
// dinah-637/criteria/12. A run stopped at the end of each phase, and between a
// card's member baselines and its card baseline, is resumed by the same
// command with no backup named, and the rerun answers resumed, the recorded
// backup, and the members, manifest and file counts an uninterrupted run
// answers, with no member stated twice.
//
// Arming: dropping the progress file's recorded counts makes a rerun after
// removal count what is left of the old layout, which reddens the phase-3 and
// phase-4 cases on the items and comments carried.
func TestAnInterruptedRunResumesToTheSameResult(t *testing.T) {
	EnableCardUnitForTest(t)
	want := uninterrupted(t)
	for _, point := range []string{"phase-0", "members:" + mainCard, "phase-1", "phase-2", "phase-3", "phase-4"} {
		t.Run(point, func(t *testing.T) {
			store := copyMigrationFixture(t)
			backup := filepath.Join(t.TempDir(), "backup")
			stopped := false
			plant(t, &storageMigrationPoint, func(at string) error {
				if at == point && !stopped {
					stopped = true
					return errors.New("planted: the run is interrupted at " + at)
				}
				return nil
			})
			if _, err := migrateFixture(t, store, migrationRun(backup)); err == nil || !stopped {
				t.Fatalf("the run was not interrupted at %s: %v", point, err)
			}
			got, err := migrateFixture(t, store, StorageMigrationRun{Template: migrationRun("").Template})
			if err != nil {
				t.Fatalf("the rerun: %v", err)
			}
			if got.Outcome != contract.ReadOK || !got.Resumed {
				t.Fatalf("the rerun answered %s resumed %v: %+v", got.Outcome, got.Resumed, got)
			}
			if got.Backup == nil || got.Backup.Path != filepath.Join(backup, filepath.Base(store)) {
				t.Errorf("the rerun names the backup %+v, wanted the one the interrupted run took", got.Backup)
			}
			if got.Manifest != want.Manifest || got.Files != want.Files || got.Items != want.Items || got.Comments != want.Comments {
				t.Errorf("the rerun answered manifest %+v files %+v items %d comments %+v, and an uninterrupted run %+v %+v %d %+v",
					got.Manifest, got.Files, got.Items, got.Comments, want.Manifest, want.Files, want.Items, want.Comments)
			}
			reopened, err := Open(store)
			if err != nil {
				t.Fatalf("open the migrated store: %v", err)
			}
			card, err := reopened.LoadCardIn(reopened.CardsRoot(), mainCard)
			if err != nil {
				t.Fatalf("load the main card: %v", err)
			}
			events, _, _ := ReadJournal(card.JournalPath())
			baselines := map[string]int{}
			for _, ev := range events {
				if ev.Event == contract.EventCardBaseline {
					baselines["card"]++
				}
			}
			if baselines["card"] != 1 {
				t.Errorf("the main card's journal carries %d card baselines, wanted one", baselines["card"])
			}
			bench := mustEvents(Disk{}, reopened.JournalPath())
			migrated := 0
			for _, ev := range bench {
				if ev.Event == contract.EventStorageMigrated {
					migrated++
				}
			}
			if migrated != 1 {
				t.Errorf("the workbench journal carries %d storage_migrated lines, wanted one", migrated)
			}
		})
	}
}

// TestABackupIsThePreRunStoreTakenAfterTheLockOut drives the backup half of
// dinah-637/criteria/12 and criteria/34. The store declares the migration in
// progress before the copy is taken; the copy hashes to the store as it stood
// before the run; a copy interrupted part way is marked incomplete and taken
// again by the rerun; and a copy leaves out every lock, every sibling lock and
// the progress file, so the store it restores starts unlocked.
//
// Arming: copying the stamped workbench.md rather than the original reddens
// the digest assertion, and dropping the lock exclusion reddens the
// no-lock-in-the-copy assertion on the planted card lock.
func TestABackupIsThePreRunStoreTakenAfterTheLockOut(t *testing.T) {
	EnableCardUnitForTest(t)
	store := copyMigrationFixture(t)
	// A card lock and a sibling lock stand in the store, as a crashed
	// process and a crashed structural act leave them, and a progress file
	// is planted where a run would write one.
	cardLock := filepath.Join(store, CardsDir, mainCard, LockName)
	if err := os.WriteFile(cardLock, []byte(`{"actor":"ghost","pid":1,"ts":"2026-01-01T00:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatalf("plant a card lock: %v", err)
	}
	pristine, err := storeDigest(Disk{}, store, "")
	if err != nil {
		t.Fatalf("digest the pristine store: %v", err)
	}
	backup := filepath.Join(t.TempDir(), "backup")
	stampedFirst := false
	copying := false
	plant(t, &storageMigrationPoint, func(at string) error {
		switch at {
		case "before-backup":
			fm, _ := loadAnchor(Disk{}, filepath.Join(store, WorkbenchAnchor))
			stampedFirst = fm.Value(MigratingKey) == MigratingStorage
		case "backup-copying":
			if !copying {
				copying = true
				return errors.New("planted: the copy is interrupted")
			}
		}
		return nil
	})
	if _, err := migrateFixture(t, store, migrationRun(backup)); err == nil {
		t.Fatal("the interrupted copy answered no error")
	}
	if !stampedFirst {
		t.Error("the store did not declare the migration in progress before its copy was taken")
	}
	copy := filepath.Join(backup, filepath.Base(store))
	marker, err := readBackupMarker(Disk{}, copy)
	if err != nil || marker.Complete {
		t.Fatalf("the interrupted copy's marker reads %+v (%v), wanted an incomplete one", marker, err)
	}
	// The rerun takes the copy again and stops at the card whose lock the
	// crashed process left, which is a card lock like any other until the
	// operator has cleared it; the copy it took is what this test is about.
	report, err := migrateFixture(t, store, migrationRun(backup))
	if err != nil || report.Outcome != contract.ReadFindings || report.Held == nil {
		t.Fatalf("the rerun: %v %+v", err, report)
	}
	marker, err = readBackupMarker(Disk{}, copy)
	if err != nil || !marker.Complete {
		t.Errorf("the rerun's copy marker reads %+v (%v), wanted a complete one", marker, err)
	}
	digest, err := storeDigest(Disk{}, copy, "")
	if err != nil {
		t.Fatalf("digest the copy: %v", err)
	}
	if digest != pristine || report.Backup.Digest != pristine {
		t.Errorf("the copy hashes to %s and the run recorded %s, and the store before the run hashed to %s", digest, report.Backup.Digest, pristine)
	}
	err = filepath.WalkDir(copy, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		if name := entry.Name(); name == LockName || strings.HasSuffix(name, ".lock") || name == StorageMigrationFile {
			t.Errorf("the copy carries %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the copy: %v", err)
	}
	// The copy restores to a store the switch-off layout opens and reads.
	restored := filepath.Join(t.TempDir(), UserBaseName, filepath.Base(store))
	copyTree(t, copy, restored)
	durable.RemoveAll(filepath.Join(restored, StorageBackupMarker))
	cardUnitEnabled = false
	opened, err := Open(restored)
	cardUnitEnabled = true
	if err != nil {
		t.Fatalf("open the restored copy with the layout switched off: %v", err)
	}
	if opened.Format >= CardUnitFormat || opened.Migrating != "" {
		t.Errorf("the restored copy declares format %d migrating %q", opened.Format, opened.Migrating)
	}
	lock, err := Acquire(filepath.Join(restored, CardsDir, mainCard), "sam", "2026-09-28T12:00:00Z")
	if err != nil {
		t.Errorf("the restored copy's card could not be locked: %v", err)
	} else {
		lock.Release()
	}
}

// TestARehearsalWritesNothingAndPrintsWhatTheRunPrints drives the rehearsal
// clause of dinah-637/criteria/12: a rehearsal takes no backup, leaves every
// path and byte of the store as it was, and answers the counts and the
// manifest hash the real run then answers.
//
// Arming: taking the rehearsal's after figure as its before figure, which is
// what a rehearsal that forgot the files the run deletes would answer,
// reddens the comparison with the run's own counts.
func TestARehearsalWritesNothingAndPrintsWhatTheRunPrints(t *testing.T) {
	EnableCardUnitForTest(t)
	store := copyMigrationFixture(t)
	before, err := storeDigest(Disk{}, store, "")
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	rehearsal, err := migrateFixture(t, store, StorageMigrationRun{Template: migrationRun("").Template, Rehearse: true})
	if err != nil || rehearsal.Outcome != contract.ReadOK || !rehearsal.Rehearsal {
		t.Fatalf("the rehearsal: %v %+v", err, rehearsal)
	}
	after, err := storeDigest(Disk{}, store, "")
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	if after != before {
		t.Error("the rehearsal changed the store")
	}
	if rehearsal.Backup != nil {
		t.Errorf("the rehearsal names a backup %+v", rehearsal.Backup)
	}
	real, err := migrateFixture(t, store, migrationRun(filepath.Join(t.TempDir(), "backup")))
	if err != nil || real.Outcome != contract.ReadOK {
		t.Fatalf("the real run: %v %+v", err, real)
	}
	if rehearsal.Manifest != real.Manifest || rehearsal.Items != real.Items || rehearsal.Comments != real.Comments ||
		rehearsal.Cards != real.Cards || rehearsal.Files != real.Files {
		t.Errorf("the rehearsal answered %+v %d %+v %+v %+v, and the run %+v %d %+v %+v %+v",
			rehearsal.Manifest, rehearsal.Items, rehearsal.Comments, rehearsal.Cards, rehearsal.Files,
			real.Manifest, real.Items, real.Comments, real.Cards, real.Files)
	}
}

// TestTheBackupArgumentIsRefusedWhereItCannotServe drives the backup refusals
// of dinah-637/criteria/12, each of which writes nothing.
func TestTheBackupArgumentIsRefusedWhereItCannotServe(t *testing.T) {
	EnableCardUnitForTest(t)
	cases := []struct {
		name    string
		backup  func(store string) string
		refusal string
	}{
		{"none given", func(string) string { return "" }, contract.BackupRequired},
		{"inside the store", func(store string) string { return filepath.Join(store, "backup") }, contract.BackupInsideStore},
		{"holding something else", func(string) string {
			dir := t.TempDir()
			os.WriteFile(filepath.Join(dir, "somebody's file"), []byte("x"), 0o644)
			return dir
		}, contract.BackupNotEmpty},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := copyMigrationFixture(t)
			before, _ := storeDigest(Disk{}, store, "")
			_, err := migrateFixture(t, store, migrationRun(tc.backup(store)))
			var refusal *contract.Refusal
			if !errors.As(err, &refusal) || refusal.Name != tc.refusal {
				t.Fatalf("wanted %s, got %v", tc.refusal, err)
			}
			if after, _ := storeDigest(Disk{}, store, ""); after != before {
				t.Error("the refused run changed the store")
			}
		})
	}
	t.Run("a resumed run naming another directory", func(t *testing.T) {
		store := copyMigrationFixture(t)
		plant(t, &storageMigrationPoint, func(at string) error {
			if at == "phase-0" {
				return errors.New("planted: interrupted")
			}
			return nil
		})
		migrateFixture(t, store, migrationRun(filepath.Join(t.TempDir(), "backup")))
		storageMigrationPoint = nil
		_, err := migrateFixture(t, store, migrationRun(filepath.Join(t.TempDir(), "elsewhere")))
		var refusal *contract.Refusal
		if !errors.As(err, &refusal) || refusal.Name != contract.BackupMismatch {
			t.Fatalf("wanted %s, got %v", contract.BackupMismatch, err)
		}
	})
}

// TestEachPreconditionRefusesNamingTheFile drives dinah-637/criteria/13. Each
// precondition about the store's own files refuses the run naming the file,
// and writes nothing.
//
// Arming: skipping the key check in checkAnchor lets the unknown-key plant
// through to a run that then carries no baseline of that key, which reddens
// its case.
func TestEachPreconditionRefusesNamingTheFile(t *testing.T) {
	EnableCardUnitForTest(t)
	card := filepath.Join(CardsDir, mainCard)
	anyItem := func(t *testing.T, store string) string {
		ids, _ := ListIDs(filepath.Join(store, card, ChecklistDir))
		return filepath.Join(store, card, ChecklistDir, ids[0], legacyAnchorOf(KindItem))
	}
	anyComment := func(t *testing.T, store string) string {
		ids, _ := ListIDs(filepath.Join(store, card, CommentsDir))
		return filepath.Join(store, card, CommentsDir, ids[0], legacyAnchorOf(KindComment))
	}
	edit := func(t *testing.T, path string, change func(string) string) {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(change(string(data))), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	cases := []struct {
		name  string
		rule  string
		plant func(t *testing.T, store string) string
	}{
		{"an unparseable anchor", PreconditionUnparseable, func(t *testing.T, store string) string {
			path := anyComment(t, store)
			edit(t, path, func(string) string { return "no frontmatter here\n" })
			return path
		}},
		{"an unknown key", PreconditionUnknownKey, func(t *testing.T, store string) string {
			path := anyItem(t, store)
			edit(t, path, func(text string) string { return strings.Replace(text, "---\n", "---\nmood: grumpy\n", 1) })
			return path
		}},
		{"an unknown citation member", PreconditionUnknownCitation, func(t *testing.T, store string) string {
			path := anyItem(t, store)
			edit(t, path, func(text string) string {
				return strings.Replace(text, "---\n", "---\ncitations:\n  - scheme: test\n    target: x\n    weight: 3\n", 1)
			})
			return path
		}},
		{"a missing ordinal", PreconditionOrdinalMissing, func(t *testing.T, store string) string {
			path := anyComment(t, store)
			edit(t, path, func(text string) string {
				lines := strings.Split(text, "\n")
				var kept []string
				for _, line := range lines {
					if !strings.HasPrefix(line, OrdinalField+":") {
						kept = append(kept, line)
					}
				}
				return strings.Join(kept, "\n")
			})
			return path
		}},
		{"a duplicate ordinal", PreconditionOrdinalDuplicate, func(t *testing.T, store string) string {
			ids, _ := ListIDs(filepath.Join(store, card, CommentsDir))
			first := filepath.Join(store, card, CommentsDir, ids[0], legacyAnchorOf(KindComment))
			second := filepath.Join(store, card, CommentsDir, ids[1], legacyAnchorOf(KindComment))
			fm, _ := loadAnchor(Disk{}, first)
			edit(t, second, func(text string) string {
				other, body := ParseAnchor(text)
				other.Set(OrdinalField, fm.Value(OrdinalField))
				return other.Render(body)
			})
			return second
		}},
		{"a comment and an item sharing an identifier", PreconditionIdentifierShared, func(t *testing.T, store string) string {
			comments, _ := ListIDs(filepath.Join(store, card, CommentsDir))
			items, _ := ListIDs(filepath.Join(store, card, ChecklistDir))
			from := filepath.Join(store, card, CommentsDir, comments[0])
			to := filepath.Join(store, card, CommentsDir, items[0])
			if err := os.Rename(from, to); err != nil {
				t.Fatalf("rename: %v", err)
			}
			return filepath.Join(to, legacyAnchorOf(KindComment))
		}},
		{"an attachment destination holding a different tree", PreconditionDestination, func(t *testing.T, store string) string {
			moves := legacyMoves(Disk{}, filepath.Join(store, card), true)
			if len(moves) == 0 {
				t.Fatal("the fixture carries no attachment to move")
			}
			os.MkdirAll(moves[0].to, 0o755)
			os.WriteFile(filepath.Join(moves[0].to, "attachment.md"), []byte("---\nfilename: other\n---\n"), 0o644)
			return moves[0].to
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := copyMigrationFixture(t)
			named := tc.plant(t, store)
			before, _ := storeDigest(Disk{}, store, "")
			_, err := migrateFixture(t, store, migrationRun(filepath.Join(t.TempDir(), "backup")))
			var refusal *contract.Refusal
			if !errors.As(err, &refusal) || refusal.Name != contract.StoragePrecondition {
				t.Fatalf("wanted %s, got %v", contract.StoragePrecondition, err)
			}
			if !strings.Contains(refusal.Extra["files"], tc.rule+" "+named) {
				t.Errorf("the refusal lists %q, wanted %s %s among them", refusal.Extra["files"], tc.rule, named)
			}
			if after, _ := storeDigest(Disk{}, store, ""); after != before {
				t.Error("the refused run changed the store")
			}
		})
	}
	t.Run("a store below the designation format", func(t *testing.T) {
		store := copyMigrationFixture(t)
		opened := openForMigration(t, store)
		opened.Format = DesignationFormat - 1
		_, err := opened.MigrateStorage(migrationRun(filepath.Join(t.TempDir(), "backup")))
		var refusal *contract.Refusal
		if !errors.As(err, &refusal) || refusal.Name != contract.StoreAwaitingMigration {
			t.Fatalf("wanted %s, got %v", contract.StoreAwaitingMigration, err)
		}
	})
	t.Run("a standing sibling lock", func(t *testing.T) {
		store := copyMigrationFixture(t)
		sibling := filepath.Join(store, CardsDir, mainCard+".lock")
		os.WriteFile(sibling, []byte("{}\n"), 0o644)
		_, err := migrateFixture(t, store, migrationRun(filepath.Join(t.TempDir(), "backup")))
		var refusal *contract.Refusal
		if !errors.As(err, &refusal) || refusal.Name != contract.Interrupted || refusal.Detail != sibling {
			t.Fatalf("wanted %s naming %s, got %v", contract.Interrupted, sibling, err)
		}
	})
	t.Run("a held workbench lock", func(t *testing.T) {
		store := copyMigrationFixture(t)
		held, err := Acquire(store, "someone else", "2026-09-28T12:00:00Z")
		if err != nil {
			t.Fatalf("hold the workbench lock: %v", err)
		}
		defer held.Release()
		_, err = migrateFixture(t, store, migrationRun(filepath.Join(t.TempDir(), "backup")))
		var refusal *contract.Refusal
		if !errors.As(err, &refusal) || refusal.Name != contract.Locked {
			t.Fatalf("wanted %s, got %v", contract.Locked, err)
		}
	})
	t.Run("a store already migrated", func(t *testing.T) {
		store := copyMigrationFixture(t)
		if report, err := migrateFixture(t, store, migrationRun(filepath.Join(t.TempDir(), "backup"))); err != nil || report.Outcome != contract.ReadOK {
			t.Fatalf("the migration: %v %+v", err, report)
		}
		report, err := migrateFixture(t, store, migrationRun(""))
		if err != nil || report.Outcome != contract.ReadOK || report.From != CardUnitFormat {
			t.Fatalf("a run over a migrated store answered %v %+v, wanted ok", err, report)
		}
	})
}

// TestAStrayIsCarriedByPhaseFour drives dinah-637/criteria/27. A comment.md an
// older process writes on a card after phase 1 is left by the removal, carried
// by phase 4 as a baseline, named among the strays and deleted; on a store the
// migration finished, dinah check reports a stray and a rerun carries it.
//
// Arming: letting phase 3 delete every member file rather than the ones the
// proof covered deletes the stray before phase 4 can carry it, which
// reddens the strays assertion.
func TestAStrayIsCarriedByPhaseFour(t *testing.T) {
	EnableCardUnitForTest(t)
	store := copyMigrationFixture(t)
	cardDir := filepath.Join(store, CardsDir, mainCard)
	var stray *Comment
	plant(t, &storageMigrationPoint, func(at string) error {
		if at == "phase-1" && stray == nil {
			written, err := legacyAddComment(Disk{}, cardDir, "an older process", "2026-09-28T12:01:00Z", "written after the baselines")
			if err != nil {
				return err
			}
			stray = written
		}
		return nil
	})
	report, err := migrateFixture(t, store, migrationRun(filepath.Join(t.TempDir(), "backup")))
	if err != nil || report.Outcome != contract.ReadOK {
		t.Fatalf("migrate: %v %+v", err, report)
	}
	if stray == nil {
		t.Fatal("the plant wrote no stray")
	}
	if len(report.Strays) != 1 || report.Strays[0] != "comment/"+mainCard+"/"+stray.ID {
		t.Errorf("the run carried the strays %v, wanted the planted comment", report.Strays)
	}
	if Exists(stray.Home) {
		t.Errorf("the stray's directory survived at %s", stray.Home)
	}
	opened, err := Open(store)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	card, _ := opened.LoadCardIn(opened.CardsRoot(), mainCard)
	record, err := opened.LoadCardRecord(card)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if carried, ok := record.Comment(stray.ID); !ok || carried.Body != "written after the baselines" {
		t.Errorf("the carried stray reads %+v", carried)
	}

	// A stray on a store the migration finished is a finding, and a rerun
	// carries it.
	late, err := legacyAddComment(Disk{}, cardDir, "a later older process", "2026-09-28T12:02:00Z", "written after the migration")
	if err != nil {
		t.Fatalf("plant a late stray: %v", err)
	}
	findings, err := opened.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	reported := false
	for _, finding := range findings {
		if finding.Key == FindingStrayMemberFile && finding.Detail == late.ID {
			reported = true
		}
	}
	if !reported {
		t.Errorf("check does not report the late stray: %+v", findings)
	}
	rerun, err := migrateFixture(t, store, migrationRun(""))
	if err != nil || len(rerun.Strays) != 1 {
		t.Fatalf("the rerun over the migrated store: %v %+v", err, rerun)
	}
	if Exists(late.Home) {
		t.Errorf("the late stray survived the rerun")
	}
}

// TestAnEditMadeDuringTheRemovalIsCarriedByPhaseFour answers
// dinah-637/questions/25. An older process that already had the store open
// rewrites an existing comment.md of a card phase 3 has not reached yet, at
// the moment phase 3 makes its first removal on another card. The proof has
// passed by then, so the comment's manifest key is in the stored manifest,
// but its file no longer states the stored line. Phase 3 leaves it, phase 4
// carries it as a baseline and names it among the strays, and the comment
// then reads with the edited text; nothing written after the proof is lost.
//
// Arming: letting phase 3 remove an anchor on its manifest key alone, as it
// did, deletes the edited file before phase 4 sees it, so the run names no
// stray and the comment reads with the text the proof covered.
func TestAnEditMadeDuringTheRemovalIsCarriedByPhaseFour(t *testing.T) {
	EnableCardUnitForTest(t)
	store := copyMigrationFixture(t)
	const laterCard, edited = "b745ea46d9e3", "fa57b145f78d"
	const text = "rewritten by an older process after the proof passed"
	anchor := filepath.Join(store, CardsDir, laterCard, CommentsDir, edited, legacyAnchorOf(KindComment))
	if !strings.Contains(mustRead(t, anchor), "---") {
		t.Fatalf("the fixture holds no comment at %s", anchor)
	}
	rewritten := false
	plant(t, &storageMigrationStep, func(op, path string) error {
		if rewritten || op != "remove" || !strings.Contains(filepath.ToSlash(path), "/"+mainCard+"/") {
			return nil
		}
		rewritten = true
		fm, _ := ParseAnchor(mustRead(t, anchor))
		return WriteText(anchor, fm.Render(text))
	})
	report, err := migrateFixture(t, store, migrationRun(filepath.Join(t.TempDir(), "backup")))
	if err != nil || report.Outcome != contract.ReadOK {
		t.Fatalf("migrate: %v %+v", err, report)
	}
	if !rewritten {
		t.Fatal("the plant never ran, so phase 3 made no removal on the main card")
	}
	key := "comment/" + laterCard + "/" + edited
	if len(report.Strays) != 1 || report.Strays[0] != key {
		t.Errorf("the run carried the strays %v, wanted %s", report.Strays, key)
	}
	if Exists(anchor) {
		t.Errorf("the edited file survived at %s", anchor)
	}
	opened, err := Open(store)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	card, err := opened.LoadCardIn(opened.CardsRoot(), laterCard)
	if err != nil {
		t.Fatalf("load %s: %v", laterCard, err)
	}
	record, err := opened.LoadCardRecord(card)
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if carried, ok := record.Comment(edited); !ok || carried.Body != text {
		t.Errorf("the edited comment reads %+v, wanted the text written after the proof", carried)
	}
}

// TestARefusedRemovalStopsTheRunAndARerunCompletes drives dinah-637/criteria/28
// through the test failure hook: a removal the filesystem refuses stops the
// run with outcome findings naming the path and the error, deletes nothing
// further, and a rerun once the refusal lifts completes with the same
// manifest hash.
func TestARefusedRemovalStopsTheRunAndARerunCompletes(t *testing.T) {
	EnableCardUnitForTest(t)
	want := uninterrupted(t)
	store := copyMigrationFixture(t)
	var refusedPath string
	plant(t, &storageMigrationStep, func(op, path string) error {
		if op == "rename" && refusedPath == "" {
			refusedPath = path
			return errors.New("the process cannot access the file because it is being used by another process")
		}
		return nil
	})
	before := memberAnchors(t, store)
	report, err := migrateFixture(t, store, migrationRun(filepath.Join(t.TempDir(), "backup")))
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if report.Outcome != contract.ReadFindings || report.Refused == nil || report.Refused.Path != refusedPath ||
		!strings.Contains(report.Refused.Error, "being used by another process") {
		t.Fatalf("the refused removal answered %+v", report)
	}
	if got := memberAnchors(t, store); got != before {
		t.Errorf("the stopped run deleted %d member files after the refusal", before-got)
	}
	storageMigrationStep = nil
	rerun, err := migrateFixture(t, store, migrationRun(""))
	if err != nil || rerun.Outcome != contract.ReadOK {
		t.Fatalf("the rerun: %v %+v", err, rerun)
	}
	if rerun.Manifest != want.Manifest {
		t.Errorf("the rerun's manifest is %+v, and an uninterrupted run's %+v", rerun.Manifest, want.Manifest)
	}
}

// TestAWriteDuringTheRunIsRecomputedOnce drives dinah-637/criteria/32. A
// comment an older process writes in the old layout between phase 1 and phase
// 2 makes the run recompute its manifest once, name the comment as written
// during the run, baseline it and finish. Separately, a baseline altered once
// removal has begun stops the rerun naming the line and the backup, a rerun
// accepting that key finishes and records it, and a key naming no differing
// line is refused.
//
// Arming: skipping the recompute when removal has not begun stops the first
// case at phase 2 instead of finishing, which reddens its outcome assertion.
func TestAWriteDuringTheRunIsRecomputedOnce(t *testing.T) {
	EnableCardUnitForTest(t)
	t.Run("a comment written during the run", func(t *testing.T) {
		store := copyMigrationFixture(t)
		cardDir := filepath.Join(store, CardsDir, mainCard)
		var written *Comment
		plant(t, &storageMigrationPoint, func(at string) error {
			if at != "phase-1" || written != nil {
				return nil
			}
			comment, err := legacyAddComment(Disk{}, cardDir, "sam", "2026-09-28T12:03:00Z", "written while the run was in progress")
			if err != nil {
				return err
			}
			written = comment
			lock, err := Acquire(cardDir, "sam", "2026-09-28T12:03:00Z")
			if err != nil {
				return err
			}
			defer lock.Release()
			return AppendEvent(lock, filepath.Join(cardDir, JournalName), Event{
				TS: "2026-09-28T12:03:00Z", Event: contract.EventCommented, Actor: NamedActor("sam"), Comment: comment.ID,
			})
		})
		report, err := migrateFixture(t, store, migrationRun(filepath.Join(t.TempDir(), "backup")))
		if err != nil || report.Outcome != contract.ReadOK {
			t.Fatalf("migrate: %v %+v", err, report)
		}
		if len(report.WrittenDuringRun) != 1 || report.WrittenDuringRun[0] != "comment/"+mainCard+"/"+written.ID {
			t.Errorf("the run names %v as written during it, wanted the planted comment", report.WrittenDuringRun)
		}
		opened, _ := Open(store)
		card, _ := opened.LoadCardIn(opened.CardsRoot(), mainCard)
		record, err := opened.LoadCardRecord(card)
		if err != nil {
			t.Fatalf("record: %v", err)
		}
		if comment, ok := record.Comment(written.ID); !ok || comment.Body != "written while the run was in progress" {
			t.Errorf("the comment written during the run reads %+v", comment)
		}
	})
	t.Run("a baseline altered after removal began", func(t *testing.T) {
		store := copyMigrationFixture(t)
		backup := filepath.Join(t.TempDir(), "backup")
		stopped := false
		plant(t, &storageMigrationPoint, func(at string) error {
			if at == "phase-3" && !stopped {
				stopped = true
				return errors.New("planted: interrupted after removal")
			}
			return nil
		})
		migrateFixture(t, store, migrationRun(backup))
		storageMigrationPoint = nil
		// A comment baseline on the main card's journal is altered, as a
		// hand edit or a bad merge might, after the old layout is gone.
		journal := filepath.Join(store, CardsDir, mainCard, JournalName)
		data, _ := os.ReadFile(journal)
		lines := strings.Split(string(data), "\n")
		key := ""
		for i, line := range lines {
			if strings.Contains(line, `"event":"comment_baseline"`) {
				var ev Event
				json.Unmarshal([]byte(line), &ev)
				ev.Text += " (altered)"
				altered, _ := EncodeEvent(ev)
				lines = append(lines[:i+1], lines[i:]...)
				lines[i+1] = string(altered)
				key = "comment/" + mainCard + "/" + ev.Comment
				break
			}
		}
		os.WriteFile(journal, []byte(strings.Join(lines, "\n")), 0o644)
		report, err := migrateFixture(t, store, migrationRun(""))
		if err != nil || report.Outcome != contract.ReadFindings || !report.RemovalStarted {
			t.Fatalf("the rerun: %v %+v", err, report)
		}
		if len(report.Differences) != 1 || report.Differences[0].Key != key || report.Backup == nil {
			t.Fatalf("the rerun names %+v and backup %+v, wanted %s and the backup", report.Differences, report.Backup, key)
		}
		refusedRun := migrationRun("")
		refusedRun.Accept = []string{"comment/" + mainCard + "/nosuch"}
		if _, err := migrateFixture(t, store, refusedRun); err == nil || !strings.Contains(err.Error(), contract.NotADifference) {
			t.Fatalf("an accepted key naming no difference answered %v, wanted %s", err, contract.NotADifference)
		}
		accepting := migrationRun("")
		accepting.Accept = []string{key}
		report, err = migrateFixture(t, store, accepting)
		if err != nil || report.Outcome != contract.ReadOK {
			t.Fatalf("the accepting rerun: %v %+v", err, report)
		}
		if len(report.Accepted) != 1 || report.Accepted[0] != key {
			t.Errorf("the run records %v as accepted, wanted %s", report.Accepted, key)
		}
		opened, _ := Open(store)
		recorded := false
		for _, ev := range mustEvents(Disk{}, opened.JournalPath()) {
			if ev.Event == contract.EventStorageMigrated && len(ev.Accepted) == 1 && ev.Accepted[0] == key {
				recorded = true
			}
		}
		if !recorded {
			t.Error("storage_migrated does not record the accepted key")
		}
	})
}
