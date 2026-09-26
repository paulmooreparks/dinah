//go:build windows

package resident

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"dinah/internal/bench"
	"golang.org/x/sys/windows"
)

// realDeadline bounds every wait on a real notification. It bounds the test
// only: the documentation promises delivery and no latency, so a wait that
// reaches it is a finding to report, not a flake to retry.
const realDeadline = 30 * time.Second

// realWatch is a resident over a real directory with the platform's own
// watcher.
type realWatch struct {
	t         *testing.T
	root      string
	w         *Workbench
	published chan Published
	holdMu    sync.Mutex
	hold      chan struct{}
	holdWhich func(Published) bool
	holding   chan Published
}

// writeFile puts a file on disk, creating the directories above it.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// realTree is the tree the real-watcher tests start from.
func realTree(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "workbench")
	for rel, content := range map[string]string{
		"workbench.md":                                      "---\ntitle: Real\n---\n",
		"cards/0123456789ab/card.md":                        "---\ntitle: A card\n---\n",
		"cards/0123456789ab/journal.ndjson":                 "{\"event\":\"created\"}\n",
		"cards/0123456789ab/checklist/0000000000a1/item.md": "---\nkind: decision\n---\nOne.\n",
		"cards/0123456789cd/card.md":                        "---\ntitle: Another\n---\n",
	} {
		writeFile(t, filepath.Join(root, filepath.FromSlash(rel)), content)
	}
	return root
}

// openReal opens a resident with the platform's watcher and waits for its
// first publish.
func openReal(t *testing.T, root string, extra *Hooks) *realWatch {
	t.Helper()
	r := &realWatch{t: t, root: root, published: make(chan Published, 1024), holding: make(chan Published, 16)}
	hooks := &Hooks{
		BeforePublish: func(p Published) {
			r.holdMu.Lock()
			hold, which := r.hold, r.holdWhich
			r.holdMu.Unlock()
			if hold != nil && which(p) {
				r.holding <- p
				<-hold
			}
		},
		AfterPublish: func(p Published) { r.published <- p },
	}
	if extra != nil {
		hooks.BeforeRearm = extra.BeforeRearm
	}
	w, err := Open(root, Options{Hooks: hooks})
	if err != nil {
		t.Fatalf("open the resident with the platform's watcher: %v", err)
	}
	r.w = w
	t.Cleanup(func() { w.Close() })
	select {
	case <-w.Ready():
	case <-time.After(realDeadline):
		t.Fatal("the first build did not finish")
	}
	return r
}

// holdPublishes makes BeforePublish wait, for the publishes which picks,
// until the answered function runs. A late notification of a directory's
// last-write time, which the documentation says arrives "only when the cache
// is sufficiently flushed", publishes a reconcile at any moment, so a hold
// names the publish it is for rather than taking the first. The release also
// runs when the test ends, so a failing test never leaves the applier held.
func (r *realWatch) holdPublishes(which func(Published) bool) func() {
	hold := make(chan struct{})
	r.holdMu.Lock()
	r.hold, r.holdWhich = hold, which
	r.holdMu.Unlock()
	var once sync.Once
	release := func() {
		once.Do(func() {
			r.holdMu.Lock()
			r.hold = nil
			r.holdMu.Unlock()
			close(hold)
		})
	}
	r.t.Cleanup(release)
	return release
}

// until waits for a publish satisfying want, and for the snapshot then
// current to mirror the disk, since one write may arrive as several
// completions. Its failure says that no notification arrived within the
// deadline, which is a finding.
func (r *realWatch) until(what string, want func(Published) bool) {
	r.t.Helper()
	timeout := time.After(realDeadline)
	seen := false
	for {
		select {
		case p := <-r.published:
			if want(p) {
				seen = true
			}
			if seen {
				if snapshot := r.w.Current(time.Now()).Snapshot; snapshot != nil && len(MirrorDiff(snapshot, r.root)) == 0 {
					return
				}
			}
		case <-timeout:
			detail := "no publish " + what + " arrived"
			if seen {
				detail = "a publish " + what + " arrived, but the snapshot never came to mirror the disk"
			}
			if snapshot := r.w.Current(time.Now()).Snapshot; snapshot != nil {
				detail += ": " + strings.Join(MirrorDiff(snapshot, r.root), "; ")
			}
			r.t.Fatalf("%s within %s; the documentation promises delivery and no latency, so this is a finding", detail, realDeadline)
		}
	}
}

// naming answers a Published test naming one slash-separated path.
func naming(rel string) func(Published) bool {
	return func(p Published) bool {
		for _, path := range p.Paths {
			if path == rel {
				return true
			}
		}
		return false
	}
}

// TestTheWatcherReportsAChangeMadeAfterArming is part of
// dinah-619/criteria/6. A file written with bench.WriteText and a line
// appended with bench.AppendEvent each appear in a publish's paths, and the
// snapshot mirrors the disk.
func TestTheWatcherReportsAChangeMadeAfterArming(t *testing.T) {
	root := realTree(t)
	r := openReal(t, root, nil)
	if err := bench.WriteText(filepath.Join(root, "cards", "0123456789ab", "card.md"), "---\ntitle: Written\n---\n"); err != nil {
		t.Fatal(err)
	}
	r.until("naming the written card", naming("cards/0123456789ab/card.md"))
	if err := bench.AppendEvent(filepath.Join(root, "cards", "0123456789ab", "journal.ndjson"), bench.Event{TS: "2026-09-26T00:00:00Z", Event: "moved", Actor: bench.Actor{Name: "alka"}}); err != nil {
		t.Fatal(err)
	}
	r.until("naming the appended journal", naming("cards/0123456789ab/journal.ndjson"))
}

// TestTheWatcherOverflowsWhenItsBufferIsExceeded is part of
// dinah-619/criteria/7. The watcher is held after its first completion,
// before it issues its next call, while 4,000 files with 100-character names
// are created: at least 800,000 bytes of records against a 65,536-byte
// buffer. A reported overflow then rebuilds the snapshot from disk.
//
// When no overflow is reported the test fails and says why: the
// documentation fixes the system buffer at the first call, and a machine
// whose buffer absorbs 800,000 bytes is a finding, not a flake.
func TestTheWatcherOverflowsWhenItsBufferIsExceeded(t *testing.T) {
	if testing.Short() {
		t.Skip("creates 4,000 files")
	}
	const files, nameLength = 4000, 100
	root := realTree(t)
	blocked := make(chan struct{})
	release := make(chan struct{})
	var once, released sync.Once
	r := openReal(t, root, &Hooks{BeforeRearm: func() {
		once.Do(func() {
			close(blocked)
			<-release
		})
	}})
	t.Cleanup(func() { released.Do(func() { close(release) }) })
	writeFile(t, filepath.Join(root, "first.txt"), "the first completion")
	select {
	case <-blocked:
	case <-time.After(realDeadline):
		t.Fatal("the watcher consumed no first completion within the deadline")
	}
	probe := filepath.Join(root, "overflow-probe")
	if err := os.MkdirAll(probe, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < files; i++ {
		name := fmt.Sprintf("%05d-%s", i, strings.Repeat("x", nameLength-6))
		writeFile(t, filepath.Join(probe, name), "")
	}
	released.Do(func() { close(release) })
	timeout := time.After(realDeadline)
	for {
		select {
		case p := <-r.published:
			if p.Rebuilt && p.Generation > 1 {
				r.w.mu.Lock()
				overflows := r.w.overflows
				r.w.mu.Unlock()
				if overflows == 0 {
					t.Fatalf("a rebuild was published with no overflow reported, so something other than the buffer's loss caused it: %+v", p)
				}
				snapshot := r.w.Current(time.Now()).Snapshot
				if snapshot == nil {
					continue
				}
				if diffs := MirrorDiff(snapshot, root); len(diffs) > 0 {
					t.Errorf("the rebuild after the overflow does not mirror the disk: %v", diffs[:min(len(diffs), 5)])
				}
				return
			}
		case <-timeout:
			t.Fatalf("no overflow was reported after %d files with %d-character names (at least %d bytes of records) were created while the watcher held no call; the documentation fixes the system buffer at the first call, and its size on this machine is the finding, not a flake", files, nameLength, files*2*nameLength)
		}
	}
}

// TestADirectoryDeletedAndRecreatedIsReadAgain is part of
// dinah-619/criteria/8. A card's directory removed and recreated with the
// same identifier and different items is read again whole.
func TestADirectoryDeletedAndRecreatedIsReadAgain(t *testing.T) {
	root := realTree(t)
	r := openReal(t, root, nil)
	card := filepath.Join(root, "cards", "0123456789ab")
	removeAll(t, card)
	writeFile(t, filepath.Join(card, "card.md"), "---\ntitle: Recreated\n---\n")
	writeFile(t, filepath.Join(card, "checklist", "0000000000b2", "item.md"), "---\nkind: open_question\n---\nA different item.\n")
	r.until("naming the recreated card", func(p Published) bool {
		for _, path := range p.Paths {
			if strings.HasPrefix(path, "cards/0123456789ab") || path == "cards" {
				return true
			}
		}
		return false
	})
}

// removeAll removes a directory tree, trying again while the removal meets a
// sharing violation. Go's os.Open does not share delete, so a file the
// resident is reading at that instant, which a late last-write notification
// can make it do at any moment, refuses a concurrent delete; the violation
// passes as soon as the read closes the file. The retry is bounded by the
// test deadline.
func removeAll(t *testing.T, path string) {
	t.Helper()
	stop := time.After(realDeadline)
	for {
		err := os.RemoveAll(path)
		if err == nil {
			return
		}
		if !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			t.Fatal(err)
		}
		select {
		case <-stop:
			t.Fatalf("the removal kept meeting a sharing violation for %s: %v", realDeadline, err)
		case <-time.After(time.Millisecond):
		}
	}
}

// TestTheRootReplacedIsDetected is part of dinah-619/criteria/8. The root is
// renamed away and a different tree written where it was; the next Current
// answers nil, and after the rebuild the snapshot mirrors the new root.
func TestTheRootReplacedIsDetected(t *testing.T) {
	root := realTree(t)
	r := openReal(t, root, nil)
	if err := os.Rename(root, root+"-old"); err != nil {
		t.Fatalf("rename the watched root away: %v", err)
	}
	writeFile(t, filepath.Join(root, "workbench.md"), "---\ntitle: Replacement\n---\n")
	writeFile(t, filepath.Join(root, "cards", "0123456789ef", "card.md"), "---\ntitle: New\n---\n")
	if pick := r.w.Current(time.Now()); pick.Snapshot != nil {
		t.Error("Current answered a snapshot after the root was replaced")
	}
	r.until("rebuilding the new root", func(p Published) bool { return p.Rebuilt && p.Generation > 1 })
}

// TestABrokenWatchDegradesAndRecovers is part of dinah-619/criteria/9. The
// outstanding call is cancelled without the closing flag, as a failing
// handle would end it; Current answers nil, a rebuild follows, and a file
// written after the recovery appears in a publish, which a watcher that
// stopped after the re-arm would never produce.
func TestABrokenWatchDegradesAndRecovers(t *testing.T) {
	root := realTree(t)
	r := openReal(t, root, nil)
	notifier, ok := r.w.notifier.(*winNotifier)
	if !ok {
		t.Fatalf("the resident's notifier is %T, not the Windows watcher", r.w.notifier)
	}
	release := r.holdPublishes(func(p Published) bool { return p.Rebuilt })
	waitReal(t, "a call to be outstanding", func() bool {
		notifier.mu.Lock()
		defer notifier.mu.Unlock()
		return notifier.issued
	})
	if err := notifier.breakWatch(); err != nil {
		t.Fatalf("break the watch: %v", err)
	}
	select {
	case p := <-r.holding:
		if !p.Rebuilt {
			t.Fatalf("the pass after the broken watch was not a rebuild: %+v", p)
		}
	case <-time.After(realDeadline):
		t.Fatal("no rebuild followed the broken watch")
	}
	if pick := r.w.Current(time.Now()); pick.Snapshot != nil {
		t.Error("Current answered a snapshot while the watch was being restored")
	}
	release()
	r.until("rebuilding after the broken watch", func(p Published) bool { return p.Rebuilt && p.Generation > 1 })
	writeFile(t, filepath.Join(root, "cards", "0123456789cd", "after.md"), "after the recovery")
	r.until("naming the file written after the recovery", naming("cards/0123456789cd/after.md"))
}

// waitReal polls a condition the notifier reaches on its own goroutine,
// against the test deadline.
func waitReal(t *testing.T, what string, done func() bool) {
	t.Helper()
	stop := time.After(realDeadline)
	for !done() {
		select {
		case <-stop:
			t.Fatalf("waited %s for %s", realDeadline, what)
		case <-time.After(time.Millisecond):
		}
	}
}

// TestCloseRacingACompletionReturns is part of dinah-619/criteria/9. The
// watcher is held after consuming a completion and before it takes n.mu to
// issue the next call, which is the position where CancelIoEx finds nothing
// to cancel. Close, called then, returns once the watcher is released, and
// the root can be removed.
//
// The root starts empty and the completion is a directory created directly
// below it, which is reported once and leaves no file whose size or
// last-write time Windows reports later when the cache is flushed. So once
// the completion is consumed nothing is pending, and a call issued after
// Close had looked for one to cancel would stay outstanding.
//
// Arming: moving Next's check of closing after the issue of the call leaves
// a call outstanding that nobody cancels, and Close misses the deadline.
func TestCloseRacingACompletionReturns(t *testing.T) {
	root := filepath.Join(t.TempDir(), "empty")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	blocked := make(chan struct{})
	release := make(chan struct{})
	var once, released sync.Once
	r := openReal(t, root, &Hooks{BeforeRearm: func() {
		once.Do(func() {
			close(blocked)
			<-release
		})
	}})
	t.Cleanup(func() { released.Do(func() { close(release) }) })
	if err := os.Mkdir(filepath.Join(root, "first"), 0o755); err != nil {
		t.Fatal(err)
	}
	select {
	case <-blocked:
	case <-time.After(realDeadline):
		t.Fatal("the watcher consumed no completion within the deadline")
	}
	closed := make(chan error, 1)
	go func() { closed <- r.w.Close() }()
	waitReal(t, "Close to set closing", func() bool {
		notifier := r.w.notifier.(*winNotifier)
		notifier.mu.Lock()
		defer notifier.mu.Unlock()
		return notifier.closing
	})
	released.Do(func() { close(release) })
	select {
	case err := <-closed:
		if err != nil {
			t.Errorf("Close answered %v", err)
		}
	case <-time.After(realDeadline):
		t.Fatal("Close did not return after a completion landed before it")
	}
	if err := os.RemoveAll(root); err != nil {
		t.Errorf("the root could not be removed after Close: %v", err)
	}
}

// TestConcurrentClosesReleaseOnce is part of dinah-619/criteria/9. Two
// sequential Closes both answer nil, and over 100 rounds two concurrent
// Closes both answer nil and release the handles once, after which the root
// can be removed.
//
// Arming: leaving armed set at the end of Close makes the second sequential
// call close the handles again and answer an error.
func TestConcurrentClosesReleaseOnce(t *testing.T) {
	root := realTree(t)
	n := newWinNotifier(nil)
	if err := n.Arm(root); err != nil {
		t.Fatal(err)
	}
	if err := n.Close(); err != nil {
		t.Errorf("the first Close answered %v", err)
	}
	if err := n.Close(); err != nil {
		t.Errorf("the second sequential Close answered %v", err)
	}
	const rounds = 100
	for round := 0; round < rounds; round++ {
		n := newWinNotifier(nil)
		if err := n.Arm(root); err != nil {
			t.Fatalf("round %d: arm: %v", round, err)
		}
		results := make(chan error, 2)
		for i := 0; i < 2; i++ {
			go func() { results <- n.Close() }()
		}
		for i := 0; i < 2; i++ {
			select {
			case err := <-results:
				if err != nil {
					t.Errorf("round %d: a concurrent Close answered %v", round, err)
				}
			case <-time.After(realDeadline):
				t.Fatalf("round %d: a concurrent Close did not return", round)
			}
		}
	}
	if err := os.RemoveAll(root); err != nil {
		t.Errorf("the root could not be removed after %d rounds of concurrent Closes: %v", rounds, err)
	}
	t.Logf("%d rounds of two concurrent Closes", rounds)
}

// TestCloseReleasesTheDirectory is part of dinah-619/criteria/16: after
// Close, the root can be removed, and Close gives back every handle Open
// took.
//
// The removal alone cannot see a handle left open, because the watch handle
// shares delete on purpose, so that the watcher never stops anybody deleting
// the workbench. So the test also opens and closes the resident twenty times
// and reads the process's handle count, documented as GetProcessHandleCount,
// before and after: a Close that leaked the directory or the event would
// leave twenty or forty behind, and the runtime's own threads do not come in
// twenties.
//
// Arming: skipping the directory handle's CloseHandle in Close leaves the
// count twenty higher.
func TestCloseReleasesTheDirectory(t *testing.T) {
	root := realTree(t)
	const cycles = 20
	before := handleCount(t)
	for i := 0; i < cycles; i++ {
		w, err := Open(root, Options{})
		if err != nil {
			t.Fatal(err)
		}
		<-w.Ready()
		if err := w.Close(); err != nil {
			t.Fatalf("Close answered %v", err)
		}
	}
	after := handleCount(t)
	if after-before >= cycles/2 {
		t.Errorf("the process held %d handles before %d opens and closes and %d after, so Close leaves handles behind", before, cycles, after)
	}
	if err := os.RemoveAll(root); err != nil {
		t.Errorf("the root could not be removed after Close: %v", err)
	}
	t.Logf("%d handles before %d opens and closes, %d after", before, cycles, after)
}

// handleCount is GetProcessHandleCount for this process.
func handleCount(t *testing.T) int {
	t.Helper()
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetProcessHandleCount")
	var count uint32
	if ok, _, err := proc.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&count))); ok == 0 {
		t.Fatalf("GetProcessHandleCount: %v", err)
	}
	return int(count)
}

// TestOnlyAFixedVolumeIsWatched is part of dinah-619/criteria/16: watchable
// answers true for DRIVE_FIXED alone, over all seven documented drive types.
func TestOnlyAFixedVolumeIsWatched(t *testing.T) {
	types := map[uint32]string{
		windows.DRIVE_UNKNOWN:     "DRIVE_UNKNOWN",
		windows.DRIVE_NO_ROOT_DIR: "DRIVE_NO_ROOT_DIR",
		windows.DRIVE_REMOVABLE:   "DRIVE_REMOVABLE",
		windows.DRIVE_FIXED:       "DRIVE_FIXED",
		windows.DRIVE_REMOTE:      "DRIVE_REMOTE",
		windows.DRIVE_CDROM:       "DRIVE_CDROM",
		windows.DRIVE_RAMDISK:     "DRIVE_RAMDISK",
	}
	if len(types) != 7 {
		t.Fatalf("checked %d drive types, wanted the seven documented", len(types))
	}
	for driveType, name := range types {
		if got, want := watchable(driveType), driveType == windows.DRIVE_FIXED; got != want {
			t.Errorf("watchable(%s) is %v, wanted %v", name, got, want)
		}
	}
}

// TestAResidentReadNeverRefusesADelete holds the resident's own reads to
// sharing delete: while a file is open for the resident's read, another
// process may still rename it away or delete it, which is how Dinah archives,
// restores and removes an entity.
//
// Arming: opening with os.Open, which shares read and write alone, makes the
// rename and the delete fail with a sharing violation.
func TestAResidentReadNeverRefusesADelete(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "card.md")
	writeFile(t, target, "held open")
	file, err := openShared(target)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	moved := filepath.Join(dir, "moved.md")
	if err := os.Rename(target, moved); err != nil {
		t.Errorf("renaming away a file the resident holds open failed: %v", err)
	}
	if err := os.Remove(moved); err != nil {
		t.Errorf("deleting a file the resident holds open failed: %v", err)
	}
}
