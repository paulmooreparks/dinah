package bench

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"dinah/internal/contract"
	"dinah/internal/durable"
)

// The storage migration carries a workbench from the old layout, where every
// comment and checklist item is a directory holding its own anchor, to the
// card-unit layout, where each is lines of the nearest journal. It runs in
// phases, each of which a rerun of the same command resumes, and it proves
// before it deletes anything that the journals state every member the old
// layout held, by comparing a manifest of both.

const (
	// StorageMigrationFile is the file a storage migration records its
	// progress in, beside workbench.md, from its first write to its last.
	StorageMigrationFile = "storage-migration.json"
	// StorageBackupMarker is the file a storage migration writes into the
	// copy of the store it takes before its first change, saying which
	// workbench the copy is of and whether it is complete.
	StorageBackupMarker = ".dinah-storage-backup.json"
)

// The phases a storage migration names when it stops, in the order it runs
// them.
const (
	StoragePhaseLockOut  = "0"
	StoragePhaseBaseline = "1"
	StoragePhaseProof    = "2"
	StoragePhaseRemoval  = "3"
	StoragePhaseStrays   = "4"
)

// The values dinah.storage-precondition carries in its rule member, one per
// precondition of the migration that is about the store's own files.
const (
	PreconditionUnparseable      = "unparseable"
	PreconditionUnknownKey       = "unknown-key"
	PreconditionUnknownCitation  = "unknown-citation-member"
	PreconditionOrdinalMissing   = "ordinal-missing"
	PreconditionOrdinalDuplicate = "ordinal-duplicate"
	PreconditionIdentifierShared = "identifier-shared"
	PreconditionDestination      = "destination-differs"
)

// commentKeys and itemKeys are the frontmatter keys the build writes on a
// comment's and an item's anchor, which are the keys a baseline can carry.
var (
	commentKeys = setOfKeys("ts", "author", CommentAuthorUnrecoverableField, OrdinalField, CommentDigestField)
	itemKeys    = setOfKeys(ItemKindField, ItemStateField, ItemColumnField, ItemOwnerField, ItemResolutionField,
		CitationsField, ItemStandingField, ItemEvidenceField, "ts", OrdinalField)
	citationKeys    = setOfKeys(ItemCitationScheme, ItemCitationTarget, ItemCitationObserved)
	observationKeys = setOfKeys("before", "after")
)

// setOfKeys builds a set of frontmatter keys.
func setOfKeys(keys ...string) map[string]bool {
	set := make(map[string]bool, len(keys))
	for _, key := range keys {
		set[key] = true
	}
	return set
}

// StorageMigrationRun is what a storage migration is asked to do.
type StorageMigrationRun struct {
	// Template carries the timestamp and the actor of every line the run
	// writes, composed by the caller, which is where an actor is composed.
	Template Event
	// Backup is the directory the run copies the store into before its
	// first change. A resumed run may leave it empty.
	Backup string
	// Rehearse decides everything and writes nothing.
	Rehearse bool
	// Accept are the manifest keys whose after line the operator takes as
	// the record, where the proof found them differing after removal began.
	Accept []string
	// Claims are the references of the cards whose claim a forced run
	// passed.
	Claims []string
}

// StorageMigration is a storage migration's own account of what it did, or
// on a rehearsal of what it would do.
type StorageMigration struct {
	Outcome          string                 `json:"outcome"`
	Rehearsal        bool                   `json:"rehearsal"`
	Resumed          bool                   `json:"resumed"`
	Backup           *StorageBackup         `json:"backup,omitempty"`
	From             int                    `json:"from"`
	To               int                    `json:"to"`
	Cards            StorageCardCounts      `json:"cards"`
	Items            int                    `json:"items"`
	ItemsLive        int                    `json:"-"`
	ItemsArchived    int                    `json:"-"`
	Comments         StorageCommentCounts   `json:"comments"`
	Attachments      StorageAttachmentCount `json:"attachments"`
	Divergences      []string               `json:"divergences"`
	Strays           []string               `json:"strays"`
	WrittenDuringRun []string               `json:"written_during_run"`
	Accepted         []string               `json:"accepted"`
	Files            StorageFileCounts      `json:"files"`
	Manifest         StorageManifestSummary `json:"manifest"`
	ClaimsPassed     []string               `json:"claims_passed"`
	// Phase names the phase a stopped run stopped in, empty on a run that
	// finished.
	Phase string `json:"phase,omitempty"`
	// Differences are the manifest lines a stopped proof found differing.
	Differences []ManifestDifference `json:"differences,omitempty"`
	// Refused is the path and the error of a rename or removal the
	// filesystem refused.
	Refused *StorageRefusal `json:"refused,omitempty"`
	// Held names a card whose lock another process holds, where that is
	// what stopped the run.
	Held *StorageHeldCard `json:"held,omitempty"`
	// RemovalStarted says a stopped run had begun removing the old layout,
	// so a difference the proof found cannot be recomputed from it: the
	// choices are to restore the backup or to accept the lines named.
	RemovalStarted bool `json:"removal_started,omitempty"`
}

// StorageBackup is where a storage migration copied the store, and the
// digest it verified the copy by.
type StorageBackup struct {
	Path   string `json:"path"`
	Digest string `json:"digest"`
}

// StorageCardCounts are the cards a storage migration carried.
type StorageCardCounts struct {
	Live     int `json:"live"`
	Archived int `json:"archived"`
}

// StorageCommentCounts are the comments a storage migration carried, by
// what they hang on.
type StorageCommentCounts struct {
	Card   int `json:"card"`
	Item   int `json:"item"`
	Column int `json:"column"`
}

// StorageAttachmentCount is how many attachments the manifest covers and how
// many of them the migration moved.
type StorageAttachmentCount struct {
	Checked int `json:"checked"`
	Moved   int `json:"moved"`
}

// StorageFileCounts are the files the store held before the run and after
// it, or on a rehearsal after the run it would make.
type StorageFileCounts struct {
	Before int `json:"before"`
	After  int `json:"after"`
}

// StorageManifestSummary is the manifest's hash before and after and the
// number of lines it holds.
type StorageManifestSummary struct {
	Before string `json:"before"`
	After  string `json:"after"`
	Lines  int    `json:"lines"`
}

// ManifestDifference is one manifest line the proof found differing, by its
// key, with the line on each side, either of which may be absent.
type ManifestDifference struct {
	Key    string `json:"key"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
}

// StorageRefusal is a rename or a removal the filesystem refused.
type StorageRefusal struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// StorageHeldCard is a card whose lock another process held when the run
// reached it.
type StorageHeldCard struct {
	Card   string `json:"card"`
	Holder string `json:"holder"`
}

// storageState is storage-migration.json: the run's progress, which a rerun
// reads to resume.
type storageState struct {
	From              int             `json:"from"`
	OriginalWorkbench string          `json:"original_workbench"`
	Backup            string          `json:"backup,omitempty"`
	BackupDigest      string          `json:"backup_digest,omitempty"`
	Manifest          *storedManifest `json:"manifest,omitempty"`
	RemovalStarted    bool            `json:"removal_started"`
	Accepted          []string        `json:"accepted,omitempty"`
	WrittenDuringRun  []string        `json:"written_during_run,omitempty"`
	FilesBefore       int             `json:"files_before,omitempty"`
	Counts            *storageCounts  `json:"counts,omitempty"`
	Moved             int             `json:"moved,omitempty"`
	Strays            []string        `json:"strays,omitempty"`
}

// storageCounts are the counts a run's report carries, recorded with the
// manifest, so a resumed run whose old layout is partly gone reports what the
// store held rather than what is left of it.
type storageCounts struct {
	Cards         StorageCardCounts    `json:"cards"`
	Items         int                  `json:"items"`
	ItemsLive     int                  `json:"items_live"`
	ItemsArchived int                  `json:"items_archived"`
	Comments      StorageCommentCounts `json:"comments"`
	Checked       int                  `json:"attachments_checked"`
	Divergences   []string             `json:"divergences,omitempty"`
}

// countsOf takes the counts out of a report.
func countsOf(report *StorageMigration) *storageCounts {
	return &storageCounts{
		Cards: report.Cards, Items: report.Items, ItemsLive: report.ItemsLive, ItemsArchived: report.ItemsArchived,
		Comments: report.Comments, Checked: report.Attachments.Checked, Divergences: report.Divergences,
	}
}

// apply puts recorded counts back into a report.
func (c *storageCounts) apply(report *StorageMigration) {
	report.Cards, report.Items, report.ItemsLive, report.ItemsArchived = c.Cards, c.Items, c.ItemsLive, c.ItemsArchived
	report.Comments, report.Attachments.Checked = c.Comments, c.Checked
	report.Divergences = append([]string{}, c.Divergences...)
}

// storedManifest is a manifest as storage-migration.json records it.
type storedManifest struct {
	Lines []string `json:"lines"`
	Count int      `json:"count"`
	Hash  string   `json:"hash"`
}

// backupMarker is the marker file a backup carries.
type backupMarker struct {
	Workbench string `json:"workbench"`
	Started   string `json:"started"`
	Complete  bool   `json:"complete"`
}

// storageMigrationPoint, when set, is called at the end of each phase and
// between one card's member baselines and its card baseline, and an error it
// answers stops the run there as a crash would. Only tests set it.
var storageMigrationPoint func(point string) error

// storageMigrationStep, when set, is called before each rename and removal
// phase 3 makes, and an error it answers is taken as the filesystem refusing
// that step. Only tests set it.
var storageMigrationStep func(op, path string) error

// storageBaselinePlant, when set, is handed every member baseline phase 1 is
// about to write, and may change it or, by answering false, drop it. It
// stands in for a faulty writer, which is what the proof exists to catch.
// Only tests set it.
var storageBaselinePlant func(ev *Event) bool

// plantedBaseline answers whether a member baseline is to be written, after
// the test plant has had its way with it.
func plantedBaseline(ev *Event) bool {
	if storageBaselinePlant == nil {
		return true
	}
	return storageBaselinePlant(ev)
}

// planted answers the member baselines phase 1 writes, after the test plant
// has had its way with each.
func planted(baselines []Event) []Event {
	kept := baselines[:0]
	for i := range baselines {
		if plantedBaseline(&baselines[i]) {
			kept = append(kept, baselines[i])
		}
	}
	return kept
}

// passPoint reports one point of the run to the test hook.
func passPoint(point string) error {
	if storageMigrationPoint == nil {
		return nil
	}
	return storageMigrationPoint(point)
}

// MigrateStorage runs the storage migration on this workbench, or rehearses
// it. The caller has already refused a non-operator, a claimed card and a
// writing run while the switch is off, and holds nothing: the run takes the
// workbench lock from its first write to its last and each card's lock
// around what it writes to that card.
//
// A precondition that fails refuses the run with nothing written. A run that
// stops part way, because a card's lock is held, the proof found a
// difference, or the filesystem refused a step, answers its account with the
// outcome findings and the phase it stopped in, and a rerun of the same
// command resumes it.
func (b *Bench) MigrateStorage(run StorageMigrationRun) (*StorageMigration, error) {
	report := &StorageMigration{
		Rehearsal:        run.Rehearse,
		To:               CardUnitFormat,
		Divergences:      []string{},
		Strays:           []string{},
		WrittenDuringRun: []string{},
		Accepted:         []string{},
		ClaimsPassed:     append([]string{}, run.Claims...),
	}
	state, resumed, err := b.readStorageState()
	if err != nil {
		return nil, err
	}
	if !resumed {
		if b.Format >= CardUnitFormat {
			return b.migratedAlready(run, report)
		}
		if b.Format < DesignationFormat {
			return nil, contract.RefuseWith(contract.StoreAwaitingMigration, b.Root, nil)
		}
	}
	report.Resumed = resumed
	if err := b.checkBackupArgument(run, state, resumed); err != nil {
		return nil, err
	}
	if err := b.checkLocksForStorage(); err != nil {
		return nil, err
	}
	source, err := b.readOldLayout(state)
	if err != nil {
		return nil, err
	}
	if err := source.preconditions(); err != nil {
		return nil, err
	}
	source.count(report)
	if resumed && state.Counts != nil {
		state.Counts.apply(report)
	}
	if run.Rehearse {
		return b.rehearseStorage(run, source, report)
	}

	now := run.Template.TS
	held, err := Acquire(b.Root, run.Template.Actor.Name, now)
	if err != nil {
		return nil, err
	}
	defer held.Release()

	if !resumed {
		original, err := durable.ReadFile(filepath.Join(b.Root, WorkbenchAnchor))
		if err != nil {
			return nil, err
		}
		state = &storageState{From: b.Format, OriginalWorkbench: string(original)}
		if err := b.writeStorageState(state); err != nil {
			return nil, err
		}
		fm, body := ParseAnchor(NormalizeNewlines(strings.TrimPrefix(string(original), byteOrderMark)))
		fm.Set("format", strconv.Itoa(CardUnitFormat))
		fm.Set(MigratingKey, MigratingStorage)
		fm.Set(MigratingFromKey, strconv.Itoa(state.From))
		if err := WriteText(filepath.Join(b.Root, WorkbenchAnchor), fm.Render(body)); err != nil {
			return nil, err
		}
	}
	report.From = state.From
	if err := b.takeBackup(run, state); err != nil {
		return nil, err
	}
	report.Backup = &StorageBackup{Path: state.Backup, Digest: state.BackupDigest}
	if state.Manifest == nil {
		files, err := countFiles(b.Root)
		if err != nil {
			return nil, err
		}
		state.FilesBefore = files
		state.Manifest = source.manifest.stored()
		state.Counts = countsOf(report)
		if err := b.writeStorageState(state); err != nil {
			return nil, err
		}
	}
	if err := passPoint("phase-0"); err != nil {
		return nil, err
	}
	report.Files.Before = state.FilesBefore

	// Phase 1, and phase 2, which may send the run back to phase 1 once.
	stopped, err := b.baselineAndProve(run, held, state, source, report)
	if err != nil || stopped {
		return b.stoppedStorage(report, state, err)
	}

	// Phase 3, removal.
	if !state.RemovalStarted {
		state.RemovalStarted = true
		if err := b.writeStorageState(state); err != nil {
			return nil, err
		}
	}
	report.Phase = StoragePhaseRemoval
	if stopped, err := b.removeOldLayout(run, state, source, report); err != nil || stopped {
		return b.stoppedStorage(report, state, err)
	}
	if err := passPoint("phase-3"); err != nil {
		return nil, err
	}

	// Phase 4, strays and finish.
	report.Phase = StoragePhaseStrays
	if stopped, err := b.carryStrays(run, held, state, report); err != nil || stopped {
		return b.stoppedStorage(report, state, err)
	}
	// A run that stopped after writing its own line and before unstamping
	// the store does not write the line a second time.
	recorded := false
	for _, ev := range mustEvents(b.JournalPath()) {
		if ev.Event == contract.EventStorageMigrated {
			recorded = true
		}
	}
	if !recorded {
		finished := run.Template
		finished.Event = contract.EventStorageMigrated
		finished.From = strconv.Itoa(state.From)
		finished.To = strconv.Itoa(CardUnitFormat)
		finished.Cards = run.Claims
		finished.Accepted = state.Accepted
		finished.WrittenDuringRun = state.WrittenDuringRun
		if err := AppendEvent(held, b.JournalPath(), finished); err != nil {
			return nil, err
		}
	}
	if err := passPoint("phase-4"); err != nil {
		return nil, err
	}
	text, err := ReadText(filepath.Join(b.Root, WorkbenchAnchor))
	if err != nil {
		return nil, err
	}
	fm, body := ParseAnchor(text)
	fm.Delete(MigratingKey)
	fm.Delete(MigratingFromKey)
	if err := WriteText(filepath.Join(b.Root, WorkbenchAnchor), fm.Render(body)); err != nil {
		return nil, err
	}
	if err := removeIfPresent(filepath.Join(b.Root, StorageMigrationFile)); err != nil {
		return nil, err
	}
	after, err := countFiles(b.Root)
	if err != nil {
		return nil, err
	}
	report.Phase = ""
	report.Files.After = after
	report.Strays = append([]string{}, state.Strays...)
	report.WrittenDuringRun = append([]string{}, state.WrittenDuringRun...)
	report.Accepted = append([]string{}, state.Accepted...)
	report.Attachments.Moved = state.Moved
	report.Outcome = contract.ReadOK
	return report, nil
}

// migratedAlready answers a store that already declares the card-unit format
// and carries no migration in progress. A leftover progress file from a run
// that finished its last step but one is removed, and a comment.md or
// item.md an older build wrote since is carried as phase 4 carries a stray.
func (b *Bench) migratedAlready(run StorageMigrationRun, report *StorageMigration) (*StorageMigration, error) {
	report.From = b.Format
	report.Outcome = contract.ReadOK
	strays, err := b.StrayMemberFiles()
	if err != nil {
		return nil, err
	}
	if run.Rehearse {
		for _, stray := range strays {
			report.Strays = append(report.Strays, stray.Path)
		}
		return report, nil
	}
	if err := removeIfPresent(filepath.Join(b.Root, StorageMigrationFile)); err != nil {
		return nil, err
	}
	if len(strays) == 0 {
		return report, nil
	}
	held, err := Acquire(b.Root, run.Template.Actor.Name, run.Template.TS)
	if err != nil {
		return nil, err
	}
	defer held.Release()
	state := &storageState{From: b.Format}
	stopped, err := b.carryStrays(run, held, state, report)
	if err := errors.Join(err, removeIfPresent(filepath.Join(b.Root, StorageMigrationFile))); err != nil {
		return nil, err
	}
	report.Strays = append([]string{}, state.Strays...)
	if stopped {
		report.Outcome = contract.ReadFindings
		report.Phase = StoragePhaseStrays
	}
	return report, nil
}

// stoppedStorage answers a run that stopped part way: an error is answered
// as it is, and a stop the run reported is the account with the outcome
// findings.
func (b *Bench) stoppedStorage(report *StorageMigration, state *storageState, err error) (*StorageMigration, error) {
	if err != nil {
		return nil, err
	}
	report.Outcome = contract.ReadFindings
	report.WrittenDuringRun = append([]string{}, state.WrittenDuringRun...)
	report.Accepted = append([]string{}, state.Accepted...)
	report.RemovalStarted = state.RemovalStarted
	return report, nil
}

// readStorageState reads storage-migration.json where the store carries
// migrating: storage, and answers whether it did. A store carrying the key
// with no progress file is damage the run cannot resume from.
func (b *Bench) readStorageState() (*storageState, bool, error) {
	if b.Migrating != MigratingStorage {
		return nil, false, nil
	}
	path := filepath.Join(b.Root, StorageMigrationFile)
	data, err := durable.ReadFile(path)
	if err != nil {
		return nil, false, contract.Refuse(contract.Malformed, path)
	}
	var state storageState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, false, contract.Refuse(contract.Malformed, path)
	}
	return &state, true, nil
}

// writeStorageState writes storage-migration.json through a temporary file
// and a rename.
func (b *Bench) writeStorageState(state *storageState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return durable.WriteFile(filepath.Join(b.Root, StorageMigrationFile), append(data, '\n'), 0o644)
}

// removeIfPresent removes a file, answering nothing where it is already gone.
func removeIfPresent(path string) error {
	if _, err := os.Lstat(path); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return durable.Remove(path)
}

// checkBackupArgument applies the first precondition: a writing run names a
// backup directory outside the store, holding nothing or a backup this
// migration started of this workbench, and a resumed run names the one it
// recorded or none.
func (b *Bench) checkBackupArgument(run StorageMigrationRun, state *storageState, resumed bool) error {
	if run.Rehearse {
		return nil
	}
	given := strings.TrimSpace(run.Backup)
	if resumed && state.Backup != "" {
		if given == "" {
			return nil
		}
		if !durable.SamePath(absolute(given), state.Backup) {
			return contract.RefuseWith(contract.BackupMismatch, given, map[string]string{"recorded": state.Backup})
		}
		return nil
	}
	if given == "" {
		return contract.Refuse(contract.BackupRequired, b.Root)
	}
	directory := absolute(given)
	if within(b.Root, directory) {
		return contract.Refuse(contract.BackupInsideStore, directory)
	}
	entries, err := os.ReadDir(directory)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return contract.Refuse(contract.BackupNotEmpty, directory)
	}
	if len(entries) == 0 {
		return nil
	}
	if len(entries) == 1 && entries[0].IsDir() && entries[0].Name() == b.ID {
		marker, err := readBackupMarker(filepath.Join(directory, b.ID))
		if err == nil && marker.Workbench == b.ID {
			return nil
		}
	}
	return contract.Refuse(contract.BackupNotEmpty, directory)
}

// absolute answers a path made absolute and clean, or the path as given where
// the working directory cannot be read.
func absolute(path string) string {
	if resolved, err := filepath.Abs(path); err == nil {
		return resolved
	}
	return filepath.Clean(path)
}

// within reports whether path is root or stands below it, compared segment
// by segment on the case rule the filesystem's paths follow here.
func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || !(rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// readBackupMarker reads the marker a backup carries.
func readBackupMarker(copy string) (*backupMarker, error) {
	data, err := durable.ReadFile(filepath.Join(copy, StorageBackupMarker))
	if err != nil {
		return nil, err
	}
	var marker backupMarker
	if err := json.Unmarshal(data, &marker); err != nil {
		return nil, err
	}
	return &marker, nil
}

// checkLocksForStorage applies the third precondition as far as it concerns
// the store's files: no structural act's sibling lock stands anywhere. The
// workbench lock is the run's own to take, and taking it refuses dinah.locked
// where another process holds it, before the run writes anything; a
// rehearsal takes no lock and writes nothing, so it is not refused over one.
func (b *Bench) checkLocksForStorage() error {
	var sibling string
	err := filepath.WalkDir(b.Root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || sibling != "" {
			return nil
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".lock") && IsID(strings.TrimSuffix(name, ".lock")) {
			sibling = path
		}
		return nil
	})
	if err != nil {
		return err
	}
	if sibling != "" {
		return contract.Refuse(contract.Interrupted, sibling)
	}
	return nil
}

// takeBackup copies the store into the backup directory, verifies the copy
// by a digest over every path and byte, and records both, unless the
// progress file already records a complete copy whose digest still holds.
func (b *Bench) takeBackup(run StorageMigrationRun, state *storageState) error {
	if state.Backup == "" {
		state.Backup = filepath.Join(absolute(run.Backup), b.ID)
	}
	if err := passPoint("before-backup"); err != nil {
		return err
	}
	copy := state.Backup
	if marker, err := readBackupMarker(copy); err == nil && marker.Complete && state.BackupDigest != "" {
		if digest, err := storeDigest(copy, ""); err == nil && digest == state.BackupDigest {
			return nil
		}
	}
	if err := durable.RemoveAll(copy); err != nil {
		return err
	}
	if err := os.MkdirAll(copy, 0o755); err != nil {
		return err
	}
	marker := backupMarker{Workbench: b.ID, Started: run.Template.TS}
	if err := writeBackupMarker(copy, marker); err != nil {
		return err
	}
	copied := 0
	err := filepath.WalkDir(b.Root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(b.Root, path)
		if entry.IsDir() {
			return os.MkdirAll(filepath.Join(copy, rel), 0o755)
		}
		if excludedFromBackup(rel) {
			return nil
		}
		data, err := durable.ReadFile(path)
		if err != nil {
			return err
		}
		if rel == WorkbenchAnchor {
			data = []byte(state.OriginalWorkbench)
		}
		if err := durable.WriteFile(filepath.Join(copy, rel), data, 0o644); err != nil {
			return err
		}
		copied++
		if copied == 1 {
			return passPoint("backup-copying")
		}
		return nil
	})
	if err != nil {
		return err
	}
	want, err := storeDigest(b.Root, state.OriginalWorkbench)
	if err != nil {
		return err
	}
	got, err := storeDigest(copy, "")
	if err != nil {
		return err
	}
	if got != want {
		return contract.Refuse(contract.BackupUnverified, copy)
	}
	marker.Complete = true
	if err := writeBackupMarker(copy, marker); err != nil {
		return err
	}
	state.BackupDigest = got
	return b.writeStorageState(state)
}

// writeBackupMarker writes a backup's marker file.
func writeBackupMarker(copy string, marker backupMarker) error {
	data, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return err
	}
	return durable.WriteFile(filepath.Join(copy, StorageBackupMarker), append(data, '\n'), 0o644)
}

// excludedFromBackup reports the files a backup leaves out, and its digest
// with them: every entity's lock, every structural act's sibling lock, the
// progress file, and the marker a backup carries.
func excludedFromBackup(rel string) bool {
	name := filepath.Base(rel)
	switch {
	case name == LockName:
		return true
	case strings.HasSuffix(name, ".lock") && IsID(strings.TrimSuffix(name, ".lock")):
		return true
	case rel == StorageMigrationFile, rel == StorageBackupMarker:
		return true
	}
	return false
}

// storeDigest is a SHA-256 over every relative path and byte of a tree,
// leaving out what a backup leaves out. Where workbench is not empty it
// stands in for workbench.md's own bytes, which is how the source is
// compared with a copy taken before the run stamped it.
func storeDigest(root, workbench string) (string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		if excludedFromBackup(rel) {
			return nil
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Slice(files, func(i, j int) bool { return filepath.ToSlash(files[i]) < filepath.ToSlash(files[j]) })
	hash := sha256.New()
	for _, rel := range files {
		data, err := durable.ReadFile(filepath.Join(root, rel))
		if err != nil {
			return "", err
		}
		if rel == WorkbenchAnchor && workbench != "" {
			data = []byte(workbench)
		}
		hash.Write([]byte(filepath.ToSlash(rel)))
		hash.Write([]byte{0})
		hash.Write([]byte(strconv.Itoa(len(data))))
		hash.Write([]byte{0})
		hash.Write(data)
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

// countFiles counts the regular files below a store that are the store's
// own, leaving out what a backup leaves out: the locks a run holds and the
// progress file it keeps, which stand only while it runs.
func countFiles(root string) (int, error) {
	count := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if !entry.IsDir() && !excludedFromBackup(rel) {
			count++
		}
		return nil
	})
	return count, err
}
