package bench

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	"dinah/internal/contract"
	"dinah/internal/durable"
)

// newDurableFixture is newFixture for a durability test, whose workbench
// flushes every write as a production binary does whatever the build.
func newDurableFixture(t *testing.T) string {
	t.Helper()
	root := newFixture(t)
	t.Cleanup(durable.KeepFlushingUnder(root))
	return root
}

// traced records the steps durable reports while a test runs, and makes a
// sync step fail when failSync is set.
type traced struct {
	mu       sync.Mutex
	steps    []durable.Step
	failSync error
}

// trace installs a recorder on durable.Observe for the rest of the test.
func trace(t *testing.T) *traced {
	t.Helper()
	recorder := &traced{}
	durable.Observe = func(step durable.Step) error {
		recorder.mu.Lock()
		defer recorder.mu.Unlock()
		recorder.steps = append(recorder.steps, step)
		if step.Op == "sync" {
			return recorder.failSync
		}
		return nil
	}
	t.Cleanup(func() { durable.Observe = nil })
	return recorder
}

// reset forgets the steps recorded so far.
func (r *traced) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.steps = nil
}

// ops answers the recorded steps as op words, each with the base name of its
// path, and with the MoveFileEx flags on a replace.
func (r *traced) ops() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var ops []string
	for _, step := range r.steps {
		word := step.Op + " " + filepath.Base(step.Path)
		if step.Op == "replace" && step.Flags != 0 {
			word += " flags=" + flagsText(step.Flags)
		}
		ops = append(ops, word)
	}
	return ops
}

// flagsText renders a flag word in hexadecimal.
func flagsText(flags uint32) string {
	return "0x" + strconv.FormatUint(uint64(flags), 16)
}

// replaceWord is how a replace over dest reads on this platform: with
// MOVEFILE_REPLACE_EXISTING|MOVEFILE_WRITE_THROUGH on Windows, and with no
// flags elsewhere.
func replaceWord(dest string) string {
	if runtime.GOOS == "windows" {
		return "replace " + dest + " flags=0x9"
	}
	return "replace " + dest
}

// temporaryWrite is the order every durable write follows for one
// destination: the temporary is written, flushed and closed, the rename lands,
// and on Linux alone the directory is flushed.
func temporaryWrite(dest, dir string) []string {
	steps := []string{"write .dinah-", "sync .dinah-", "close .dinah-", replaceWord(dest)}
	if runtime.GOOS == "linux" {
		steps = append(steps, "syncdir "+dir)
	}
	return steps
}

// matchSteps reports whether got follows want in order, where a want entry
// ending in a dash matches any step beginning with it, which is how the random
// suffix of a temporary is matched.
func matchSteps(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if strings.HasSuffix(want[i], "-") {
			if !strings.HasPrefix(got[i], want[i]) {
				return false
			}
			continue
		}
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// temporariesIn answers the durable temporaries standing in a directory.
func temporariesIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("list %s: %v", dir, err)
	}
	var found []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".dinah-") {
			found = append(found, entry.Name())
		}
	}
	return found
}

// TestEveryDurableWriteFlushesBeforeItRenames asserts the order durable's
// trace records for each write the specification names: WriteText, an
// attachment's payload, a replaced payload, a lock record and a journal
// append. Each temporary is written, flushed and closed before the rename, the
// rename carries MOVEFILE_REPLACE_EXISTING|MOVEFILE_WRITE_THROUGH on Windows,
// and the directory is flushed after it on Linux alone.
func TestEveryDurableWriteFlushesBeforeItRenames(t *testing.T) {
	root := newDurableFixture(t)
	card := fixtureCardDir(root)
	recorder := trace(t)

	anchor := filepath.Join(card, CardAnchor)
	if err := WriteText(anchor, cleanCard); err != nil {
		t.Fatalf("write text: %v", err)
	}
	if want := temporaryWrite(CardAnchor, "c00000000001"); !matchSteps(recorder.ops(), want) {
		t.Errorf("WriteText recorded %v, wanted %v", recorder.ops(), want)
	}

	recorder.reset()
	attached, err := AddAttachmentBytes(card, "notes.txt", []byte("first"), "", "test")
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	payloadSteps := recorder.ops()[len(temporaryWrite(AttachmentAnchor, "")):]
	if want := temporaryWrite("notes.txt", PayloadDir); !matchSteps(payloadSteps, want) {
		t.Errorf("the payload write recorded %v, wanted %v", payloadSteps, want)
	}

	recorder.reset()
	source := filepath.Join(t.TempDir(), "notes.txt")
	write(t, source, "second")
	if _, err := ReplaceAttachment(attached.Dir, source); err != nil {
		t.Fatalf("replace the attachment: %v", err)
	}
	if want := temporaryWrite("notes.txt", PayloadDir); !matchSteps(recorder.ops(), want) {
		t.Errorf("ReplaceAttachment recorded %v, wanted %v", recorder.ops(), want)
	}

	recorder.reset()
	lock, err := Acquire(card, "alka", "2026-09-28T00:00:00Z")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	lockSteps := recorder.ops()
	if want := []string{"write " + LockName, "sync " + LockName}; !matchSteps(lockSteps, want) {
		t.Errorf("the lock record recorded %v, wanted %v", lockSteps, want)
	}

	recorder.reset()
	ev := Event{TS: "2026-09-28T00:00:00Z", Event: contract.EventCommented, Actor: NamedActor("alka")}
	err = AppendEvent(lock, filepath.Join(card, JournalName), ev)
	appendSteps := recorder.ops()
	lock.Release()
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	want := []string{"append " + JournalName, "sync " + JournalName, "close " + JournalName}
	if !matchSteps(appendSteps, want) {
		t.Errorf("AppendEvent recorded %v, wanted %v", appendSteps, want)
	}
}

// TestAFailedFlushLeavesTheDestinationAsItWas asserts that when durable's
// trace seam fails the flush, each write answers that error, the destination
// keeps the bytes it had, and no temporary is left behind.
func TestAFailedFlushLeavesTheDestinationAsItWas(t *testing.T) {
	root := newDurableFixture(t)
	card := fixtureCardDir(root)
	attached, err := AddAttachmentBytes(card, "notes.txt", []byte("first"), "", "test")
	if err != nil {
		t.Fatalf("attach: %v", err)
	}
	payloadDir := filepath.Join(attached.Dir, PayloadDir)
	journal := filepath.Join(card, JournalName)
	held, err := Acquire(card, "alka", "2026-09-28T00:00:00Z")
	if err != nil {
		t.Fatalf("take the card's lock for the append: %v", err)
	}
	defer held.Release()
	flushFailed := errors.New("the seam failed this flush")
	recorder := trace(t)
	recorder.failSync = flushFailed
	// The card's lock is held open for the append below, and a held lock
	// file cannot be read on Windows, so the snapshot leaves that one file
	// out. It is the lock the append is made under rather than a
	// destination any write here could change.
	heldLock := filepath.Join(card, LockName)
	snapshotOf := func() map[string]string {
		files := everyFileUnderExcept(t, root, heldLock)
		return files
	}
	before := snapshotOf()

	if err := WriteText(filepath.Join(card, CardAnchor), "changed"); !errors.Is(err, flushFailed) {
		t.Errorf("WriteText answered %v, wanted the failed flush", err)
	}
	if _, err := AddAttachmentBytes(card, "more.txt", []byte("more"), "", "test"); !errors.Is(err, flushFailed) {
		t.Errorf("AddAttachmentBytes answered %v, wanted the failed flush", err)
	}
	source := filepath.Join(t.TempDir(), "notes.txt")
	write(t, source, "second")
	if _, err := ReplaceAttachment(attached.Dir, source); !errors.Is(err, flushFailed) {
		t.Errorf("ReplaceAttachment answered %v, wanted the failed flush", err)
	}
	if _, err := Acquire(attached.Dir, "alka", "2026-09-28T00:00:00Z"); !errors.Is(err, flushFailed) {
		t.Errorf("the lock record answered %v, wanted the failed flush", err)
	}
	ev := Event{TS: "2026-09-28T00:00:00Z", Event: contract.EventCommented, Actor: NamedActor("alka")}
	if err := AppendEvent(held, journal, ev); !errors.Is(err, flushFailed) {
		t.Errorf("AppendEvent answered %v, wanted the failed flush", err)
	}
	after := snapshotOf()
	for path, text := range before {
		if after[path] != text {
			t.Errorf("%s changed under a failed flush", path)
		}
	}
	for path := range after {
		if _, was := before[path]; !was && !strings.Contains(filepath.ToSlash(path), "/attachments/") {
			t.Errorf("%s appeared under a failed flush", path)
		}
	}
	for _, dir := range []string{card, payloadDir} {
		if left := temporariesIn(t, dir); len(left) != 0 {
			t.Errorf("temporaries stand in %s: %v", dir, left)
		}
	}
	if Exists(filepath.Join(attached.Dir, LockName)) {
		t.Error("a lock whose record never reached the disk was left standing")
	}
}

// TestACardJournalIsCalledTornOnlyWhenItIsTornUnderTheLock asserts that a
// journal ending in a partial line while another holder has the card's lock
// is not reported torn, since an append may be in flight, and that the same
// journal with the lock free is.
func TestACardJournalIsCalledTornOnlyWhenItIsTornUnderTheLock(t *testing.T) {
	root := newDurableFixture(t)
	card := fixtureCardDir(root)
	appendText(t, filepath.Join(card, JournalName), `{"ts":"2026-09-28T00:00:00Z","ev`)
	held, err := Acquire(card, "brin", "2026-09-28T00:00:00Z")
	if err != nil {
		t.Fatalf("hold the card's lock: %v", err)
	}
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if tornFindings(t, opened) != 0 {
		t.Error("a journal whose card's lock another holder has was reported torn")
	}
	held.Release()
	if tornFindings(t, opened) != 1 {
		t.Error("a torn journal with its card's lock free was not reported")
	}
}

// tornFindings counts check's torn-journal findings.
func tornFindings(t *testing.T, opened *Bench) int {
	t.Helper()
	findings, err := opened.Check()
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	count := 0
	for _, finding := range findings {
		if finding.Key == FindingTornJournal {
			count++
		}
	}
	return count
}

// TestALockPlantedDuringTheArchiveDoesNotTravel asserts that a lock standing
// in a card's directory when the archive moves it, which a writer creating the
// lock between the act's release of it and the move leaves, is removed from
// the archived directory, and that such a writer's own acquisition refuses
// and completes its refusal.
func TestALockPlantedDuringTheArchiveDoesNotTravel(t *testing.T) {
	root := newDurableFixture(t)
	card := fixtureCardDir(root)
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	var writerErr error
	opened.Hooks = &Hooks{AfterStep: func(step int) error {
		if step != 5 {
			return nil
		}
		_, writerErr = Acquire(card, "cato", "2026-09-28T00:00:00Z")
		plantRecord(t, filepath.Join(card, LockName), LockRecord{Actor: "cato", PID: os.Getpid(), TS: "2026-09-28T00:00:00Z"})
		return nil
	}}
	journal := filepath.Join(card, JournalName)
	act := &StructuralAct{
		Dir:     card,
		LockDir: card,
		Op:      OpArchive,
		Actor:   "alka",
		Now:     "2026-09-28T00:00:00Z",
		Record: func(locks ActLocks) error {
			ev := Event{TS: "2026-09-28T00:00:00Z", Event: contract.EventArchived, Actor: NamedActor("alka"), Note: "c00000000001"}
			return AppendEvent(locks.For(journal), journal, ev)
		},
	}
	if err := opened.Run(act); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if !isLocked(writerErr) {
		t.Errorf("the writer in the window answered %v, wanted dinah.locked", writerErr)
	}
	archived := ArchiveTarget(card)
	if !Exists(filepath.Join(archived, CardAnchor)) {
		t.Fatal("the card did not reach the archive")
	}
	if Exists(filepath.Join(archived, LockName)) {
		t.Error("a lock travelled into the archive with the card")
	}
	if Exists(SiblingPath(card)) {
		t.Error("the sibling still stands after the archive finished")
	}
}
