package bench

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// migrationFixtureDir is the committed store the storage migration's tests
// carry, written by the build before dinah-637 through its verbs.
var migrationFixtureDir = filepath.Join("testdata", "storagemigrate", "before")

// copyMigrationFixture copies the migration fixture into a .dinah container
// under a temporary directory, named for the identifier the store was created
// under, and answers the store's own directory.
func copyMigrationFixture(t *testing.T) string {
	t.Helper()
	id, err := os.ReadFile(filepath.Join("testdata", "storagemigrate", "workbench-id.txt"))
	if err != nil {
		t.Fatalf("read the fixture's identifier: %v", err)
	}
	store := filepath.Join(t.TempDir(), UserBaseName, strings.TrimSpace(string(id)))
	copyTree(t, migrationFixtureDir, store)
	return store
}

// migrationRun is a run of the storage migration as the fixture's operator.
func migrationRun(backup string) StorageMigrationRun {
	return StorageMigrationRun{
		Template: Event{TS: "2026-09-28T12:00:00Z", Actor: NamedActor("sam")},
		Backup:   backup,
	}
}

// openForMigration opens a store the way dinah check does, which is the
// opener the migration runs under.
func openForMigration(t *testing.T, store string) *Bench {
	t.Helper()
	opened, err := OpenAwaitingResolution(store)
	if err != nil {
		t.Fatalf("open %s: %v", store, err)
	}
	return opened
}

// payloadDigests answers the SHA-256 of every attachment payload below a
// store, keyed by the payload's path relative to the store.
func payloadDigests(t *testing.T, store string) map[string]string {
	t.Helper()
	digests := map[string]string{}
	err := filepath.WalkDir(store, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Base(filepath.Dir(path)) != PayloadDir {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		rel, _ := filepath.Rel(store, path)
		digests[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", store, err)
	}
	return digests
}

// TestTheMigrationCarriesTheFixture drives dinah-637/criteria/1 and the first
// half of criteria/2. With the switch on, a run over the fixture leaves no
// member anchor and no checklist directory, declares format 12 with no
// migration in progress, removes its progress file, keeps every attachment
// payload's bytes, keeps the path of every payload that did not hang on an
// item comment, and puts each item-comment payload below the holding card's
// comments collection; its answer carries equal manifests.
//
// Arming: dropping the move of phase 3 leaves the item comment's payload in
// the checklist directory phase 3 deletes, which reddens the payload and
// no-checklist assertions, and the proof reddens first where the proof is
// computed after the deletion.
func TestTheMigrationCarriesTheFixture(t *testing.T) {
	EnableCardUnitForTest(t)
	store := copyMigrationFixture(t)
	before := payloadDigests(t, store)
	if len(before) < 3 {
		t.Fatalf("the fixture carries %d payloads, wanted at least the card's, the comment's and the item comment's", len(before))
	}
	opened := openForMigration(t, store)
	report, err := opened.MigrateStorage(migrationRun(filepath.Join(t.TempDir(), "backup")))
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if report.Outcome != contract.ReadOK {
		t.Fatalf("the migration answered %s at phase %s: %+v", report.Outcome, report.Phase, report)
	}
	if report.Manifest.Before != report.Manifest.After || report.Manifest.Lines == 0 {
		t.Errorf("the manifests are %s and %s over %d lines, wanted two equal hashes over the fixture's lines",
			report.Manifest.Before, report.Manifest.After, report.Manifest.Lines)
	}
	err = filepath.WalkDir(store, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		switch {
		case entry.IsDir() && entry.Name() == ChecklistDir:
			t.Errorf("a checklist directory survived at %s", path)
		case !entry.IsDir() && (entry.Name() == legacyAnchorOf(KindComment) || entry.Name() == legacyAnchorOf(KindItem)):
			t.Errorf("a member anchor survived at %s", path)
		case !entry.IsDir() && entry.Name() == StorageMigrationFile:
			t.Errorf("the progress file survived at %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	reopened, err := Open(store)
	if err != nil {
		t.Fatalf("open the migrated store: %v", err)
	}
	if reopened.Format != CardUnitFormat || reopened.Migrating != "" {
		t.Errorf("the migrated store declares format %d migrating %q", reopened.Format, reopened.Migrating)
	}
	fm, _ := loadAnchor(filepath.Join(store, WorkbenchAnchor))
	if fm.Has(MigratingKey) || fm.Has(MigratingFromKey) {
		t.Errorf("workbench.md still carries the migration's keys")
	}
	after := payloadDigests(t, store)
	moved := 0
	for path, digest := range before {
		if !strings.Contains(path, "/"+ChecklistDir+"/") {
			if after[path] != digest {
				t.Errorf("the payload %s stands at the same path with digest %q, wanted %q", path, after[path], digest)
			}
			continue
		}
		parts := strings.Split(path, "/")
		// cards/<card>/checklist/<item>/comments/<comment>/attachments/<id>/payload/<file>
		moved++
		want := strings.Join(append([]string{parts[0], parts[1]}, parts[4:]...), "/")
		if after[want] != digest {
			t.Errorf("the item comment's payload %s is at %s with digest %q, wanted %q", path, want, after[want], digest)
		}
	}
	if moved == 0 {
		t.Error("no payload of the fixture hung on an item comment, so the move was not exercised")
	}
	if len(after) != len(before) {
		t.Errorf("the store holds %d payloads after the migration and %d before", len(after), len(before))
	}
}
