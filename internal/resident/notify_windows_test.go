//go:build windows

package resident

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"dinah/internal/bench"
	"dinah/internal/durable"
	"golang.org/x/sys/windows"
)

// realDeadline bounds every wait on a real notification. It bounds the test
// only: the documentation promises delivery and no latency, so a wait that
// reaches it is a finding to report, not a flake to retry.
const realDeadline = 30 * time.Second

// realWatch is a resident over a real directory with the platform's own
// watcher, whose hooks send received batches and publishes to channels.
//
// The watch is held on the root directory of the workbench's volume, and the
// resident reads below the workbench root only inside a request, so nothing
// here retries a write, a rename or a removal: no read of the resident's can
// be in the way unless the test itself asks for one.
type realWatch struct {
	t         *testing.T
	root      string
	w         *Workbench
	published chan Published
	received  chan Batch
	holdMu    sync.Mutex
	hold      chan struct{}
	holding   chan Published
	through   *throughs
}

// throughs records the paths a snapshot passed through to the disk.
type throughs struct {
	mu    sync.Mutex
	paths []string
}

func (p *throughs) add(path string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paths = append(p.paths, path)
}

func (p *throughs) all() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.paths...)
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
	realTreeAt(t, root)
	return root
}

// realTreeAt writes the tree at root.
func realTreeAt(t *testing.T, root string) {
	t.Helper()
	for rel, content := range map[string]string{
		"workbench.md":                                      "---\ntitle: Real\n---\n",
		"cards/0123456789ab/card.md":                        "---\ntitle: A card\n---\n",
		"cards/0123456789ab/journal.ndjson":                 "{\"event\":\"created\"}\n",
		"cards/0123456789ab/checklist/0000000000a1/item.md": "---\nkind: decision\n---\nOne.\n",
		"cards/0123456789cd/card.md":                        "---\ntitle: Another\n---\n",
	} {
		writeFile(t, filepath.Join(root, filepath.FromSlash(rel)), content)
	}
}

// openReal opens a resident with the platform's watcher and warms it.
func openReal(t *testing.T, root string, extra *Hooks) *realWatch {
	t.Helper()
	r := openRealCold(t, root, extra)
	Warm(t, r.w)
	r.drain()
	return r
}

// openRealCold opens a resident with the platform's watcher and reads nothing.
func openRealCold(t *testing.T, root string, extra *Hooks) *realWatch {
	t.Helper()
	r := &realWatch{t: t, root: root, published: make(chan Published, 1024), received: make(chan Batch, 4096),
		holding: make(chan Published, 16), through: &throughs{}}
	hooks := &Hooks{
		BeforePublish: func(p Published) {
			r.holdMu.Lock()
			hold := r.hold
			r.holdMu.Unlock()
			if hold != nil {
				r.holding <- p
				<-hold
			}
		},
		AfterPublish: func(p Published) { r.published <- p },
		AfterReceive: func(b Batch) {
			select {
			case r.received <- b:
			default:
			}
		},
		PassThrough: r.through.add,
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
	return r
}

// drain empties the publish channel.
func (r *realWatch) drain() []Published {
	var got []Published
	for {
		select {
		case p := <-r.published:
			got = append(got, p)
		default:
			return got
		}
	}
}

// current is one request.
func (r *realWatch) current() Pick {
	ctx, cancel := context.WithTimeout(context.Background(), realDeadline)
	defer cancel()
	return r.w.Current(ctx, time.Now())
}

// receiveNaming waits for a received batch with a change whose path, slash
// separated, satisfies want, or for an overflow when overflow is set. Its
// failure says that no notification naming the path arrived within the
// deadline, which is a finding, because the documentation promises delivery
// and no latency.
func (r *realWatch) receiveNaming(what string, overflow bool, want func(string) bool) {
	r.t.Helper()
	timeout := time.After(realDeadline)
	for {
		select {
		case b := <-r.received:
			if overflow && b.Overflow {
				return
			}
			for _, c := range b.Changes {
				if want(filepath.ToSlash(c.Path)) {
					return
				}
			}
		case <-timeout:
			r.t.Fatalf("no notification %s arrived within %s; the documentation promises delivery and no latency, so this is a finding", what, realDeadline)
		}
	}
}

// settles asks until the snapshot a request answers mirrors the disk, since
// one write may arrive as several completions, each received and applied
// when a request asks. It answers the publishes the requests caused.
func (r *realWatch) settles(what string) []Published {
	r.t.Helper()
	timeout := time.After(realDeadline)
	var publishes []Published
	for {
		pick := r.current()
		publishes = append(publishes, r.drain()...)
		if pick.Snapshot != nil && len(MirrorDiff(pick.Snapshot, r.root)) == 0 {
			return publishes
		}
		select {
		case <-r.received:
		case <-timeout:
			detail := ""
			if pick.Snapshot != nil {
				detail = strings.Join(MirrorDiff(pick.Snapshot, r.root), "; ")
			}
			r.t.Fatalf("after %s the snapshot never came to mirror the disk within %s: %s", what, realDeadline, detail)
		}
	}
}

// exactly answers a predicate naming one slash-separated path.
func exactly(rel string) func(string) bool {
	return func(path string) bool { return path == rel }
}

// namedIn reports whether any publish names a slash-separated path.
func namedIn(publishes []Published, rel string) bool {
	for _, p := range publishes {
		for _, path := range p.Paths {
			if path == rel {
				return true
			}
		}
	}
	return false
}

// TestTheWatcherReportsAChangeMadeAfterArming is part of
// dinah-619/criteria/6. After the first request, a file written with
// bench.WriteText and a line appended with bench.AppendEvent are each
// received, appear in a publish's paths once a request asks, and the
// snapshot mirrors the disk.
func TestTheWatcherReportsAChangeMadeAfterArming(t *testing.T) {
	root := realTree(t)
	r := openReal(t, root, nil)
	anchor := "cards/0123456789ab/card.md"
	if err := bench.WriteText(filepath.Join(root, filepath.FromSlash(anchor)), "---\ntitle: Written\n---\n"); err != nil {
		t.Fatal(err)
	}
	r.receiveNaming("naming the written card", false, exactly(anchor))
	if publishes := r.settles("the write"); !namedIn(publishes, anchor) {
		t.Errorf("no publish named %s: %+v", anchor, publishes)
	}
	journal := "cards/0123456789ab/journal.ndjson"
	journalPath := filepath.Join(root, filepath.FromSlash(journal))
	held, err := bench.Acquire(filepath.Dir(journalPath), "alka", "2026-09-26T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	err = bench.AppendEvent(held, journalPath, bench.Event{TS: "2026-09-26T00:00:00Z", Event: "moved", Actor: bench.Actor{Name: "alka"}})
	held.Release()
	if err != nil {
		t.Fatal(err)
	}
	r.receiveNaming("naming the appended journal", false, exactly(journal))
	if publishes := r.settles("the append"); !namedIn(publishes, journal) {
		t.Errorf("no publish named %s: %+v", journal, publishes)
	}
}

// TestTheWatcherOverflowsWhenItsBufferIsExceeded is part of
// dinah-619/criteria/7. The watcher is held after its first completion,
// before it issues its next call, while 4,000 files with 100-character names
// are created: at least 800,000 bytes of records against a 65,536-byte
// buffer. The reported overflow is received, and the next request rebuilds
// inside itself and answers a snapshot that mirrors the disk.
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
	for received := false; !received; {
		select {
		case b := <-r.received:
			received = b.Overflow
		case <-timeout:
			t.Fatalf("no overflow was reported after %d files with %d-character names (at least %d bytes of records) were created while the watcher held no call; the documentation fixes the system buffer at the first call, and its size on this machine is the finding, not a flake", files, nameLength, files*2*nameLength)
		}
	}
	pick := r.current()
	if pick.Snapshot == nil {
		t.Fatal("the request after the overflow answered no snapshot")
	}
	rebuilt := false
	for _, p := range r.drain() {
		rebuilt = rebuilt || p.Rebuilt
	}
	if !rebuilt {
		t.Error("the request after the overflow published no rebuild")
	}
	if diffs := MirrorDiff(pick.Snapshot, root); len(diffs) > 0 {
		t.Errorf("the rebuild after the overflow does not mirror the disk: %v", diffs[:min(len(diffs), 5)])
	}
}

// TestADirectoryDeletedAndRecreatedIsReadAgain is part of
// dinah-619/criteria/8. A card's directory removed and recreated with the
// same identifier and different items is received, and read again whole once
// a request asks.
func TestADirectoryDeletedAndRecreatedIsReadAgain(t *testing.T) {
	root := realTree(t)
	r := openReal(t, root, nil)
	card := filepath.Join(root, "cards", "0123456789ab")
	if err := os.RemoveAll(card); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(card, "card.md"), "---\ntitle: Recreated\n---\n")
	writeFile(t, filepath.Join(card, "checklist", "0000000000b2", "item.md"), "---\nkind: open_question\n---\nA different item.\n")
	r.receiveNaming("naming the recreated card", false, func(path string) bool {
		return strings.HasPrefix(path, "cards/0123456789ab")
	})
	publishes := r.settles("the recreation")
	named := false
	for _, p := range publishes {
		for _, path := range p.Paths {
			named = named || strings.HasPrefix(path, "cards/0123456789ab") || path == "cards"
		}
	}
	if !named {
		t.Errorf("no publish named the recreated card: %+v", publishes)
	}
}

// TestTheRootReplacedIsDetected is part of dinah-619/criteria/8. The root is
// renamed away, once and with no retry, and a different tree written where it
// was. The test waits for the new tree's records to be received; the next
// Current finds Valid false, waits while the watch is re-armed on the path
// and rebuilt, and answers a snapshot of the new root, the rebuild's
// AfterPass preceding its return.
func TestTheRootReplacedIsDetected(t *testing.T) {
	root := realTree(t)
	r := openReal(t, root, nil)
	if err := os.Rename(root, root+"-old"); err != nil {
		t.Fatalf("the rename of the workbench root away was refused: %v", err)
	}
	writeFile(t, filepath.Join(root, "workbench.md"), "---\ntitle: Replacement\n---\n")
	writeFile(t, filepath.Join(root, "cards", "0123456789ef", "card.md"), "---\ntitle: New\n---\n")
	r.receiveNaming("from the replacement tree", false, func(string) bool { return true })
	pick := r.current()
	if pick.Snapshot == nil {
		t.Fatal("the request after the root was replaced answered no snapshot")
	}
	rebuilt := false
	for _, p := range r.drain() {
		rebuilt = rebuilt || p.Rebuilt
	}
	if !rebuilt {
		t.Error("the request after the root was replaced published no rebuild")
	}
	if diffs := MirrorDiff(pick.Snapshot, root); len(diffs) > 0 {
		t.Errorf("the snapshot after the root was replaced does not mirror the new root: %v", diffs)
	}
}

// mklinkJunction makes a directory junction with mklink /J, which needs no
// privilege, and skips the test with the reason when it is refused.
func mklinkJunction(t *testing.T, link, target string) {
	t.Helper()
	out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
	if err != nil {
		t.Skipf("mklink /J was refused (%v: %s)", err, strings.TrimSpace(string(out)))
	}
}

// TestTheServedPathResolvingElsewhereIsDetected is part of
// dinah-619/criteria/8. The workbench is served through a junction, via, to
// a/wb. After the first request, the junction is removed and made again
// pointing to b/wb, a different tree; no record under a/wb reports that. The
// next Current finds Valid false because the final path differs, waits for
// the re-arm and the rebuild, and answers a snapshot of b/wb.
//
// Arming: making Valid read only the moved flag answers the snapshot of a/wb.
func TestTheServedPathResolvingElsewhereIsDetected(t *testing.T) {
	base := t.TempDir()
	a, b := filepath.Join(base, "a", "wb"), filepath.Join(base, "b", "wb")
	realTreeAt(t, a)
	realTreeAt(t, b)
	writeFile(t, filepath.Join(b, "cards", "0123456789ef", "card.md"), "---\ntitle: Only in b\n---\n")
	via := filepath.Join(base, "via")
	mklinkJunction(t, via, a)
	r := openReal(t, via, nil)
	if err := os.Remove(via); err != nil {
		t.Fatalf("remove the junction: %v", err)
	}
	mklinkJunction(t, via, b)
	pick := r.current()
	if pick.Snapshot == nil {
		t.Fatal("the request after the junction was re-pointed answered no snapshot")
	}
	// Late records of the tree's own creation can name a/wb, and a reconcile
	// they cause reads through via, which now resolves to b/wb, so the
	// snapshot can come to mirror b/wb without the re-point being detected.
	// Only the final-path comparison makes this request rebuild.
	rebuilt := false
	for _, p := range r.drain() {
		rebuilt = rebuilt || p.Rebuilt
	}
	if !rebuilt {
		t.Error("the request after the junction was re-pointed did not rebuild, so the re-point was not detected")
	}
	if diffs := MirrorDiff(pick.Snapshot, via); len(diffs) > 0 {
		t.Errorf("the snapshot after the junction was re-pointed does not mirror its new target: %v", diffs)
	}
	if _, err := pick.Snapshot.Stat(filepath.Join(via, "cards", "0123456789ef", "card.md")); err != nil {
		t.Errorf("the card only b/wb holds is not in the snapshot: %v", err)
	}
}

// TestABrokenWatchDegradesAndRecovers is part of dinah-619/criteria/9. The
// outstanding call is cancelled without the closing flag, as a failing
// handle would end it, and the watch parks. The next Current waits for the
// re-arm and the rebuild and answers the rebuilt snapshot, and a file written
// after the recovery is received and appears in a publish, which a watcher
// that stopped after the re-arm would never produce.
func TestABrokenWatchDegradesAndRecovers(t *testing.T) {
	root := realTree(t)
	r := openReal(t, root, nil)
	notifier, ok := r.w.notifier.(*winNotifier)
	if !ok {
		t.Fatalf("the resident's notifier is %T, not the Windows watcher", r.w.notifier)
	}
	// A record anywhere on the volume can complete the outstanding call just
	// before the cancel reaches it, which CancelIoEx answers ERROR_NOT_FOUND;
	// the cancel is tried again on the call the watcher issues next.
	waitReal(t, "a cancel to reach an outstanding call", func() bool { return notifier.breakWatch() == nil })
	waitReal(t, "the failure to park the watch", func() bool { return Parked(r.w) })
	pick := r.current()
	if pick.Snapshot == nil {
		t.Fatal("the request after the broken watch answered no snapshot")
	}
	rebuilt := false
	for _, p := range r.drain() {
		rebuilt = rebuilt || p.Rebuilt
	}
	if !rebuilt {
		t.Error("no rebuild followed the broken watch inside the next request")
	}
	after := "cards/0123456789cd/after.md"
	writeFile(t, filepath.Join(root, filepath.FromSlash(after)), "after the recovery")
	r.receiveNaming("naming the file written after the recovery", false, exactly(after))
	if publishes := r.settles("the write after the recovery"); !namedIn(publishes, after) {
		t.Errorf("no publish named %s: %+v", after, publishes)
	}
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

// TestAJunctionInsideTheWorkbenchIsReadFromDisk is part of
// dinah-619/criteria/20, the Windows twin of the portable link test. A card's
// comments directory is a junction to a directory outside the root. After the
// first request, the comment outside the root is changed, with the real
// watcher running; the next Current's snapshot answers the new bytes through
// a pass-through, and holds no file below the junction.
//
// Arming: making isLink answer false for every entry answers the old bytes.
func TestAJunctionInsideTheWorkbenchIsReadFromDisk(t *testing.T) {
	root := realTree(t)
	outside := filepath.Join(t.TempDir(), "outside")
	comment := filepath.Join(outside, "comments", "0000000000c1", "comment.md")
	writeFile(t, comment, "before")
	junction := filepath.Join(root, "cards", "0123456789cd", "comments")
	mklinkJunction(t, junction, filepath.Join(outside, "comments"))
	r := openReal(t, root, nil)
	files, _ := r.w.Held()
	writeFile(t, comment, "after, changed outside the root")
	pick := r.current()
	if pick.Snapshot == nil {
		t.Fatal("the request answered no snapshot")
	}
	below := filepath.Join(junction, "0000000000c1", "comment.md")
	data, err := pick.Snapshot.ReadFile(below)
	if err != nil || string(data) != "after, changed outside the root" {
		t.Errorf("the comment below the junction read %q, %v", data, err)
	}
	passed := false
	for _, path := range r.through.all() {
		passed = passed || path == below
	}
	if !passed {
		t.Errorf("the read of %s was not passed through to the disk", below)
	}
	if want := 5; files != want {
		t.Errorf("the snapshot holds %d files, wanted the %d regular files outside the junction", files, want)
	}
}

// TestCloseRacingACompletionReturns is part of dinah-619/criteria/9. The
// watcher is held after consuming a completion and before it takes n.mu to
// issue the next call, which is the position where CancelIoEx finds nothing
// to cancel. Close, called then, returns once the watcher is released, no
// call is issued after Close has begun, and the root can be removed.
//
// The watch is on the volume's root directory, so a call issued after Close
// had looked for one to cancel would be completed by the next record anywhere
// on the volume and Close would return anyway. The test therefore also reads
// strayIssue, which issue sets when it runs after Close has begun.
//
// Arming: moving Next's check of closing after the issue of the call issues
// a call after Close began, and strayIssue is set.
func TestCloseRacingACompletionReturns(t *testing.T) {
	root := filepath.Join(t.TempDir(), "empty")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	blocked := make(chan struct{})
	release := make(chan struct{})
	var once, released sync.Once
	r := openRealCold(t, root, &Hooks{BeforeRearm: func() {
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
	notifier := r.w.notifier.(*winNotifier)
	closed := make(chan error, 1)
	go func() { closed <- r.w.Close() }()
	waitReal(t, "Close to set closing", func() bool {
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
	notifier.mu.Lock()
	stray := notifier.strayIssue
	notifier.mu.Unlock()
	if stray {
		t.Error("a call was issued after Close had begun, which nobody would cancel")
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

// TestCloseReleasesTheDirectory is kept from earlier revisions: after Close,
// the root can be removed, and Close gives back every handle Open took. It no
// longer stands for the property the operator ruled on;
// TestAFolderAboveAServedWorkbenchStaysRenamable in cmd/dinah does.
//
// The removal alone cannot see a handle left open, because the watch handle
// shares delete, so the test also opens, warms and closes the resident twenty
// times and reads the process's handle count, documented as
// GetProcessHandleCount, before and after: a Close that leaked the directory
// or the event would leave twenty or forty behind, and the runtime's own
// threads do not come in twenties.
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
		Warm(t, w)
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
	file, err := durable.Open(target)
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

// TestAResidentListingNeverRefusesADelete is the directory form of
// TestAResidentReadNeverRefusesADelete (dinah-619/comments/12's minor
// finding): a directory the resident is listing, whose handle is open for the
// length of the listing, can still be renamed and deleted by another
// process. os.Open does not share delete, so a listing through it refused
// both. The directory holds a file, so the handle is listing something, and
// the listing runs while the handle is held.
func TestAResidentListingNeverRefusesADelete(t *testing.T) {
	dir := t.TempDir()
	listed := filepath.Join(dir, "comments")
	if err := os.Mkdir(listed, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(listed, "comment.md"), "listed")
	file, err := durable.OpenDir(listed)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	entries, err := file.ReadDir(-1)
	if err != nil || len(entries) != 1 {
		t.Fatalf("the held handle listed %d entries (%v), wanted the one file", len(entries), err)
	}
	moved := filepath.Join(dir, "moved")
	if err := os.Rename(listed, moved); err != nil {
		t.Errorf("renaming away a directory the resident holds open failed: %v", err)
		moved = listed
	}
	if err := os.RemoveAll(moved); err != nil {
		t.Errorf("deleting a directory the resident holds open failed: %v", err)
	}
}

// TestTheBuilderListingNeverRefusesADelete holds the same property through
// the two places the resident lists a directory, the builder's readDir (a
// full read) and its readListing (an update), rather than through the helper
// they call: listDirHeld renames and deletes the directory while the
// listing's handle is open. A call site that went back to os.ReadDir would
// never reach listDirHeld, and one whose listing did not share delete would
// refuse the rename, so either goes red here (dinah-619/comments/15).
func TestTheBuilderListingNeverRefusesADelete(t *testing.T) {
	for _, path := range []struct {
		name string
		list func(b *builder, rel string)
	}{
		{"readDir", func(b *builder, rel string) { b.readDir(rel, false) }},
		{"readListing", func(b *builder, rel string) { b.readListing(rel) }},
	} {
		t.Run(path.name, func(t *testing.T) {
			root := t.TempDir()
			listed := filepath.Join(root, "comments")
			if err := os.Mkdir(listed, 0o755); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(listed, "comment.md"), "listed")
			held := 0
			listDirHeld = func(at string) {
				if at != listed {
					return
				}
				held++
				moved := filepath.Join(root, "moved")
				if err := os.Rename(listed, moved); err != nil {
					t.Errorf("%s: renaming away a directory the resident is listing failed: %v", path.name, err)
					moved = listed
				}
				if err := os.RemoveAll(moved); err != nil {
					t.Errorf("%s: deleting a directory the resident is listing failed: %v", path.name, err)
				}
			}
			t.Cleanup(func() { listDirHeld = nil })
			path.list(&builder{root: root, dirs: map[string]*dirNode{}}, "comments")
			if held != 1 {
				t.Errorf("%s listed the directory %d times through listDir, wanted once: the resident's listing no longer goes through the handle that shares delete", path.name, held)
			}
		})
	}
}
