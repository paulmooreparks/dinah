package resident_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dinah/internal/resident"
	"dinah/internal/resident/residenttest"
)

// deadline bounds every wait in these tests. It bounds the test only; a wait
// that reaches it is a finding, not a flake.
const deadline = 30 * time.Second

// tree writes files below root from a map of slash-separated relative paths
// to contents.
func tree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, content := range files {
		write(t, filepath.Join(root, filepath.FromSlash(rel)), content)
	}
}

// write puts a file on disk, creating the directories above it.
func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// watched is a resident over a test tree with a hand-driven notifier, a
// channel of its publishes and of its received batches, and one ordered list
// of what its hooks and the test observed.
//
// Under the waiting rules of dinah-619's specification, section 12.3, a
// delivered change is published only when a request asks, and every pass
// runs inside the request that asked for it. So a test waits on
// AfterReceive for a delivered change, then calls Current or Settle, and
// makes its assertions on what that call answers and on the publish it
// caused, which has already happened when the call returns.
type watched struct {
	t         *testing.T
	root      string
	w         *resident.Workbench
	manual    *residenttest.Manual
	published chan resident.Published
	received  chan resident.Batch
	// hold, when set, makes BeforePublish wait on it.
	hold atomic.Pointer[chan struct{}]
	// holding receives a publish BeforePublish is waiting on.
	holding chan resident.Published
	// blockPass, when set, makes BeforePass wait on it.
	blockPass atomic.Pointer[chan struct{}]
	// blocking receives a pass BeforePass is waiting on.
	blocking chan resident.Pass
	passes   atomic.Int64

	mu     sync.Mutex
	events []string
}

// open opens a resident over root with a hand-driven notifier and reads
// nothing.
func open(t *testing.T, root string) *watched {
	t.Helper()
	v := &watched{t: t, root: root, manual: residenttest.NewManual(),
		published: make(chan resident.Published, 1024), received: make(chan resident.Batch, 1024),
		holding: make(chan resident.Published, 16), blocking: make(chan resident.Pass, 16)}
	hooks := &resident.Hooks{
		BeforePublish: func(p resident.Published) {
			if hold := v.hold.Load(); hold != nil {
				v.holding <- p
				<-*hold
			}
		},
		AfterPublish: func(p resident.Published) { v.published <- p },
		AfterReceive: func(b resident.Batch) { v.received <- b },
		BeforePass: func(p resident.Pass) {
			v.passes.Add(1)
			v.event(fmt.Sprintf("before-pass rebuild=%v", p.Rebuild))
			if block := v.blockPass.Load(); block != nil {
				v.blocking <- p
				<-*block
			}
		},
		AfterPass: func(p resident.Pass, published bool) {
			v.event(fmt.Sprintf("after-pass rebuild=%v published=%v", p.Rebuild, published))
		},
	}
	w, err := resident.Open(root, resident.Options{Notifier: v.manual, Hooks: hooks})
	if err != nil {
		t.Fatalf("open the resident: %v", err)
	}
	v.w = w
	t.Cleanup(func() { w.Close() })
	return v
}

// watch opens a resident over root and warms it: the first request builds
// the snapshot inside itself.
func watch(t *testing.T, root string) *watched {
	t.Helper()
	v := open(t, root)
	resident.Warm(t, v.w)
	select {
	case first := <-v.published:
		if !first.Rebuilt {
			t.Fatalf("the first publish was not a rebuild: %+v", first)
		}
	default:
		t.Fatal("the first request answered a snapshot and nothing was published")
	}
	return v
}

// event appends to the ordered list.
func (v *watched) event(what string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.events = append(v.events, what)
}

// eventsSince answers the events from index from on.
func (v *watched) eventsSince(from int) []string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return append([]string(nil), v.events[from:]...)
}

// mark answers the length of the event list.
func (v *watched) mark() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return len(v.events)
}

// request is one Current, recorded in the event list when it returns.
func (v *watched) request() resident.Pick {
	v.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	pick := v.w.Current(ctx, time.Now())
	v.event("current returned")
	return pick
}

// current is request, failing when it answers no snapshot.
func (v *watched) current() *resident.Snapshot {
	v.t.Helper()
	pick := v.request()
	if pick.Snapshot == nil {
		v.t.Fatal("the resident answered no snapshot")
	}
	return pick.Snapshot
}

// deliver hands changes to the watcher and waits until it has received them.
func (v *watched) deliver(changes ...resident.Change) {
	v.t.Helper()
	v.manual.Deliver(changes...)
	v.receive()
}

// receive waits for the watcher to receive one batch.
func (v *watched) receive() resident.Batch {
	v.t.Helper()
	select {
	case b := <-v.received:
		return b
	case <-time.After(deadline):
		v.t.Fatalf("no batch was received within %s", deadline)
		return resident.Batch{}
	}
}

// publish answers a publish that has already happened, failing when there is
// none.
func (v *watched) publish() resident.Published {
	v.t.Helper()
	select {
	case p := <-v.published:
		return p
	default:
		v.t.Fatal("no publish had happened")
		return resident.Published{}
	}
}

// drain answers every publish that has happened.
func (v *watched) drain() []resident.Published {
	var got []resident.Published
	for {
		select {
		case p := <-v.published:
			got = append(got, p)
		default:
			return got
		}
	}
}

// passEndedInside fails the test unless, among the events from index from,
// a pass ended before the next request returned.
func (v *watched) passEndedInside(from int, rebuild bool) {
	v.t.Helper()
	events := v.eventsSince(from)
	ended := -1
	for i, e := range events {
		if strings.HasPrefix(e, fmt.Sprintf("after-pass rebuild=%v", rebuild)) && ended < 0 {
			ended = i
		}
		if e == "current returned" {
			if ended < 0 {
				v.t.Errorf("the request returned before its pass ended: %v", events)
			}
			return
		}
	}
	v.t.Errorf("no request returned: %v", events)
}

// mirrors fails the test unless the snapshot mirrors the disk below root.
func mirrors(t *testing.T, snapshot *resident.Snapshot, root string) {
	t.Helper()
	if diffs := resident.MirrorDiff(snapshot, root); len(diffs) > 0 {
		t.Errorf("the snapshot does not mirror the disk:\n  %s", strings.Join(diffs, "\n  "))
	}
}

// names reports whether a publish's paths carry a relative path.
func names(p resident.Published, rel string) bool {
	for _, path := range p.Paths {
		if path == rel {
			return true
		}
	}
	return false
}

// change is a Change with a slash-separated path turned native.
func change(rel string, action resident.Action) resident.Change {
	return resident.Change{Path: filepath.FromSlash(rel), Action: action}
}

// baseTree is the tree every reconcile case starts from.
var baseTree = map[string]string{
	"a.txt":         "a",
	"d1/f1.txt":     "f1",
	"d1/sub/g.txt":  "g",
	"d2/h.txt":      "h",
	"d2/deep/i.txt": "i",
}

// TestNothingIsReadUntilARequestAsks is part of dinah-619/criteria/19. After
// Open and quiesce, no pass has started, the notifier was armed once, and
// Ready is not closed. The first Current answers a snapshot from a rebuild
// whose AfterPass ran before it returned. A hundred received batches, an
// overflow and a failure start no pass and no Arm; the next Current re-arms
// and rebuilds inside itself.
//
// Arming: letting the applier start a pass on a signal with no request queued
// calls BeforePass before any Current; and the third revision's Current,
// which answers nil at once on a nil pointer and leaves the rebuild to run
// after it, answers nil and returns before AfterPass.
func TestNothingIsReadUntilARequestAsks(t *testing.T) {
	root := t.TempDir()
	tree(t, root, baseTree)
	v := open(t, root)
	resident.Quiesce(v.w)
	if n := v.passes.Load(); n != 0 {
		t.Errorf("%d passes started before any request asked", n)
	}
	if got := v.manual.Arms(); got != 1 {
		t.Errorf("the notifier was armed %d times by Open, wanted once", got)
	}
	select {
	case <-v.w.Ready():
		t.Error("Ready was closed by Open alone")
	default:
	}
	from := v.mark()
	v.current()
	if first := v.publish(); !first.Rebuilt {
		t.Errorf("the first request's pass was not a rebuild: %+v", first)
	}
	v.passEndedInside(from, true)

	const batches = 100
	for i := 0; i < batches; i++ {
		write(t, filepath.Join(root, "d1", "f1.txt"), strconv.Itoa(i))
		v.deliver(change("d1/f1.txt", resident.Modified))
	}
	v.manual.Overflow()
	v.receive()
	v.manual.Fail(errors.New("the watch handle failed"))
	waitFor(t, "the failure to park the watch", func() bool { return resident.Parked(v.w) })
	resident.Quiesce(v.w)
	if n := v.passes.Load(); n != 1 {
		t.Errorf("%d passes ran after %d batches, an overflow and a failure with no request, wanted the first one alone", n, batches)
	}
	if got := v.manual.Arms(); got != 1 {
		t.Errorf("the notifier was armed %d times with no request asking, wanted once", got)
	}
	from = v.mark()
	mirrors(t, v.current(), root)
	if got := v.manual.Arms(); got != 2 {
		t.Errorf("the notifier was armed %d times after the request, wanted twice", got)
	}
	if n := v.passes.Load(); n != 2 {
		t.Errorf("%d passes ran after the request, wanted two", n)
	}
	v.passEndedInside(from, true)
	t.Logf("%d batches received with no pass; events %v", batches, v.eventsSince(0))
}

// TestATakenRequestWaitsForItsPass is part of dinah-619/criteria/19. A
// Current whose context ends while the pass that took it is held does not
// return until the pass ends; a Current whose context has ended while it is
// still queued returns a nil Snapshot at once and starts no pass.
//
// Arming: letting a taken request return on its context's end returns the
// first Current while the pass is held.
func TestATakenRequestWaitsForItsPass(t *testing.T) {
	root := t.TempDir()
	tree(t, root, baseTree)
	v := watch(t, root)

	v.manual.Overflow()
	v.receive()
	resident.Quiesce(v.w)
	hold := make(chan struct{})
	v.hold.Store(&hold)
	ctx, cancel := context.WithCancel(context.Background())
	returned := make(chan resident.Pick, 1)
	go func() {
		pick := v.w.Current(ctx, time.Now())
		v.event("current returned")
		returned <- pick
	}()
	select {
	case <-v.holding:
	case <-time.After(deadline):
		t.Fatal("the rebuild never reached BeforePublish")
	}
	cancel()
	// A request that returned on its context would do so at once; this
	// window only bounds how long the test looks.
	select {
	case <-returned:
		t.Error("the Current returned on its context's end while the pass that took it was held")
	case <-time.After(200 * time.Millisecond):
	}
	from := v.mark()
	v.hold.Store(nil)
	close(hold)
	var pick resident.Pick
	select {
	case pick = <-returned:
	case <-time.After(deadline):
		t.Fatal("the Current did not return after its pass was released")
	}
	if pick.Snapshot == nil {
		t.Error("the Current answered no snapshot after the rebuild it waited for published")
	}
	v.passEndedInside(from, true)
	v.drain()

	block := make(chan struct{})
	v.blockPass.Store(&block)
	settled := make(chan error, 1)
	go func() { settled <- v.w.Settle(context.Background(), filepath.Join(root, "d1")) }()
	select {
	case <-v.blocking:
	case <-time.After(deadline):
		t.Fatal("the Settle's pass never started")
	}
	before := v.passes.Load()
	write(t, filepath.Join(root, "d2", "h.txt"), "changed while the pass was blocked")
	v.deliver(change("d2/h.txt", resident.Modified))
	cancelled, cancelNow := context.WithCancel(context.Background())
	cancelNow()
	if got := v.w.Current(cancelled, time.Now()); got.Snapshot != nil {
		t.Error("a Current cancelled while it was queued answered a snapshot")
	}
	v.blockPass.Store(nil)
	close(block)
	if err := <-settled; err != nil {
		t.Errorf("the Settle answered %v", err)
	}
	resident.Quiesce(v.w)
	if after := v.passes.Load(); after != before {
		t.Errorf("%d passes started after the Settle's, wanted none for the cancelled request", after-before)
	}
}

// TestARequestCarriesEveryReceivedChange is part of dinah-619/criteria/19.
// Three files in three directories are written and delivered as three
// batches, each received, with no request between them; one Current then
// answers a snapshot carrying all three, and the publish it caused names all
// three. A Current after that, with nothing received, starts no pass.
//
// Arming: letting Current skip the wait when changes are pending answers the
// snapshot from before the writes.
func TestARequestCarriesEveryReceivedChange(t *testing.T) {
	root := t.TempDir()
	tree(t, root, baseTree)
	v := watch(t, root)
	written := map[string]string{"a.txt": "a, again", "d1/f1.txt": "f1, again", "d2/h.txt": "h, again"}
	for rel, content := range written {
		write(t, filepath.Join(root, filepath.FromSlash(rel)), content)
		v.deliver(change(rel, resident.Modified))
	}
	snapshot := v.current()
	for rel, content := range written {
		data, err := snapshot.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil || string(data) != content {
			t.Errorf("%s read %q, %v through the snapshot, wanted %q", rel, data, err, content)
		}
	}
	p := v.publish()
	for rel := range written {
		if !names(p, rel) {
			t.Errorf("the publish the request caused reconciled %v, which does not name %s", p.Paths, rel)
		}
	}
	before := v.passes.Load()
	v.current()
	if after := v.passes.Load(); after != before {
		t.Errorf("a request with nothing received started %d passes", after-before)
	}
}

// TestAReconcileMatchesTheDiskForEveryChange is part of
// dinah-619/criteria/8. Each case changes the disk, delivers the changes the
// watcher would report, waits until they are received, asks with a Current,
// and compares the snapshot it answers with a fresh build of the disk.
//
// Arming: making every directory change take the shallow row, which is the
// widest form of making a Removed on a directory shallow, reddens the
// deleted-and-recreated case, whose recreated directory keeps its old
// file's bytes; and skipping the parent re-list reddens the moved-out case,
// whose directory the snapshot still holds.
func TestAReconcileMatchesTheDiskForEveryChange(t *testing.T) {
	outside := t.TempDir()
	cases := []struct {
		name    string
		change  func(t *testing.T, root string)
		changes []resident.Change
	}{
		{"a file modified", func(t *testing.T, root string) {
			write(t, filepath.Join(root, "d1", "f1.txt"), "f1, longer now")
		}, []resident.Change{change("d1/f1.txt", resident.Modified)}},
		{"a file added", func(t *testing.T, root string) {
			write(t, filepath.Join(root, "d2", "new.txt"), "new")
		}, []resident.Change{change("d2/new.txt", resident.Added)}},
		{"a file removed", func(t *testing.T, root string) {
			os.Remove(filepath.Join(root, "a.txt"))
		}, []resident.Change{change("a.txt", resident.Removed)}},
		{"a file renamed within a directory", func(t *testing.T, root string) {
			os.Rename(filepath.Join(root, "d1", "f1.txt"), filepath.Join(root, "d1", "f2.txt"))
		}, []resident.Change{change("d1/f1.txt", resident.RenamedOld), change("d1/f2.txt", resident.RenamedNew)}},
		{"a file replaced by a rename, reported as RenamedNew alone", func(t *testing.T, root string) {
			write(t, filepath.Join(root, "d1", ".tmp"), "replaced")
			os.Rename(filepath.Join(root, "d1", ".tmp"), filepath.Join(root, "d1", "f1.txt"))
		}, []resident.Change{change("d1/f1.txt", resident.RenamedNew)}},
		{"a directory added with contents", func(t *testing.T, root string) {
			tree(t, root, map[string]string{"d3/x.txt": "x", "d3/y/z.txt": "z"})
		}, []resident.Change{change("d3", resident.Added)}},
		{"a directory removed", func(t *testing.T, root string) {
			os.RemoveAll(filepath.Join(root, "d2"))
		}, []resident.Change{change("d2", resident.Removed)}},
		{"a directory renamed within the tree", func(t *testing.T, root string) {
			os.Rename(filepath.Join(root, "d1", "sub"), filepath.Join(root, "d2", "sub2"))
		}, []resident.Change{change("d1/sub", resident.RenamedOld), change("d2/sub2", resident.RenamedNew)}},
		{"a directory moved out", func(t *testing.T, root string) {
			os.Rename(filepath.Join(root, "d2"), filepath.Join(outside, "moved-out-"+strconv.Itoa(int(time.Now().UnixNano()))))
		}, []resident.Change{change("d2", resident.Removed)}},
		{"a directory moved in", func(t *testing.T, root string) {
			from := filepath.Join(outside, "moved-in-"+strconv.Itoa(int(time.Now().UnixNano())))
			tree(t, from, map[string]string{"m.txt": "m", "n/o.txt": "o"})
			os.Rename(from, filepath.Join(root, "d4"))
		}, []resident.Change{change("d4", resident.Added)}},
		{"a directory deleted and recreated with different contents", func(t *testing.T, root string) {
			os.RemoveAll(filepath.Join(root, "d2"))
			tree(t, root, map[string]string{"d2/h.txt": "h, recreated", "d2/other.txt": "other"})
		}, []resident.Change{change("d2", resident.Removed), change("d2", resident.Added)}},
		{"Modified on a directory that gained a child", func(t *testing.T, root string) {
			write(t, filepath.Join(root, "d1", "gained.txt"), "gained")
		}, []resident.Change{change("d1", resident.Modified)}},
		{"the same changes delivered in reverse order", func(t *testing.T, root string) {
			os.Rename(filepath.Join(root, "d1", "f1.txt"), filepath.Join(root, "d1", "f2.txt"))
		}, []resident.Change{change("d1/f2.txt", resident.RenamedNew), change("d1/f1.txt", resident.RenamedOld)}},
		{"one change delivered twice", func(t *testing.T, root string) {
			write(t, filepath.Join(root, "d2", "h.txt"), "h, twice")
		}, []resident.Change{change("d2/h.txt", resident.Modified), change("d2/h.txt", resident.Modified)}},
	}
	if len(cases) != 14 {
		t.Fatalf("the table holds %d cases, wanted the fourteen dinah-619 section 12.3 names", len(cases))
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			tree(t, root, baseTree)
			v := watch(t, root)
			mirrors(t, v.current(), root)
			c.change(t, root)
			v.deliver(c.changes...)
			snapshot := v.current()
			if p := v.publish(); p.Rebuilt {
				t.Fatalf("the change was applied by a rebuild, which proves nothing about the reconcile")
			}
			mirrors(t, snapshot, root)
		})
	}
}

// TestAnUnplacedNameReconcilesItsDirectory is part of dinah-619/criteria/8. A
// change naming a short name under a card's directory, delivered after the
// directory was removed, with a resolver that fails, reconciles the last
// directory the walk reached, and the snapshot mirrors the disk.
func TestAnUnplacedNameReconcilesItsDirectory(t *testing.T) {
	restore := resident.SetLongForm(func(string) (string, error) { return "", errors.New("no such path") })
	defer restore()
	root := t.TempDir()
	tree(t, root, map[string]string{"cards/0123456789ab/card.md": "card", "cards/0123456789cd/card.md": "other"})
	v := watch(t, root)
	os.RemoveAll(filepath.Join(root, "cards", "0123456789ab"))
	v.deliver(change("cards/ZZZZZZ~1/card.md", resident.Modified))
	snapshot := v.current()
	if p := v.publish(); !names(p, "cards") {
		t.Errorf("the publish reconciled %v, wanted cards", p.Paths)
	}
	mirrors(t, snapshot, root)
}

// TestARequestNeverSeesAHalfAppliedBatch is part of dinah-619/criteria/5.
// Each round writes one generation number into twelve files across four
// directories and delivers the twelve changes as one batch, while eight
// readers loop on Current and read all twelve files through the snapshot
// they were handed. Every read of the twelve carries one generation.
//
// Arming: publishing after each reconciled path rather than after the batch
// lets a reader see the batch half applied.
func TestARequestNeverSeesAHalfAppliedBatch(t *testing.T) {
	const rounds, readers = 200, 8
	root := t.TempDir()
	var files []string
	for d := 0; d < 4; d++ {
		for f := 0; f < 3; f++ {
			files = append(files, fmt.Sprintf("dir%d/file%d.txt", d, f))
		}
	}
	for _, rel := range files {
		write(t, filepath.Join(root, filepath.FromSlash(rel)), "0")
	}
	v := watch(t, root)
	var stop atomic.Bool
	var reads atomic.Int64
	var failures sync.Map
	var group sync.WaitGroup
	for r := 0; r < readers; r++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for !stop.Load() {
				ctx, cancel := context.WithTimeout(context.Background(), deadline)
				snapshot := v.w.Current(ctx, time.Now()).Snapshot
				cancel()
				if snapshot == nil {
					continue
				}
				seen := map[string]bool{}
				for _, rel := range files {
					data, err := snapshot.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
					if err != nil {
						failures.Store("read "+rel, err.Error())
						continue
					}
					seen[string(data)] = true
				}
				if len(seen) != 1 {
					failures.Store(fmt.Sprint(seen), "a snapshot carried more than one generation")
				}
				reads.Add(1)
			}
		}()
	}
	published := 0
	for round := 1; round <= rounds; round++ {
		var batch []resident.Change
		for _, rel := range files {
			write(t, filepath.Join(root, filepath.FromSlash(rel)), strconv.Itoa(round))
			batch = append(batch, change(rel, resident.Modified))
		}
		v.deliver(batch...)
		data, err := v.current().ReadFile(filepath.Join(root, filepath.FromSlash(files[len(files)-1])))
		if err != nil || string(data) != strconv.Itoa(round) {
			t.Fatalf("round %d: the request after the batch was received read %q, %v", round, data, err)
		}
		published += len(v.drain())
	}
	stop.Store(true)
	group.Wait()
	failures.Range(func(key, value any) bool {
		t.Errorf("%v: %v", key, value)
		return true
	})
	t.Logf("%d rounds, %d publishes, %d snapshots read whole", rounds, published, reads.Load())
	if published < rounds || reads.Load() < int64(rounds) {
		t.Fatalf("%d publishes and %d snapshots read over %d rounds, and every round publishes and the readers read throughout", published, reads.Load(), rounds)
	}
}

// TestOverflowRebuildsFromDisk is part of dinah-619/criteria/7. The disk is
// changed with no change delivered, then an overflow is reported and handled.
// A Current then waits while the rebuild is held before its publish; after
// the release it answers the rebuilt snapshot, which mirrors the disk, and
// the rebuild's AfterPass preceded its return.
func TestOverflowRebuildsFromDisk(t *testing.T) {
	root := t.TempDir()
	tree(t, root, baseTree)
	v := watch(t, root)
	write(t, filepath.Join(root, "d1", "unreported.txt"), "never reported")
	os.RemoveAll(filepath.Join(root, "d2"))
	v.manual.Overflow()
	v.receive()
	resident.Quiesce(v.w)
	hold := make(chan struct{})
	v.hold.Store(&hold)
	returned := make(chan *resident.Snapshot, 1)
	go func() { returned <- v.current() }()
	select {
	case <-v.holding:
	case <-time.After(deadline):
		t.Fatal("no rebuild reached BeforePublish")
	}
	select {
	case <-returned:
		t.Error("Current returned while the overflow's rebuild was held")
	default:
	}
	from := v.mark()
	v.hold.Store(nil)
	close(hold)
	snapshot := <-returned
	if p := v.publish(); !p.Rebuilt {
		t.Errorf("the publish after an overflow was not a rebuild: %+v", p)
	}
	v.passEndedInside(from, true)
	mirrors(t, snapshot, root)
}

// recovers is the part of the failed-watch tests that proves the watcher
// resumed Next: a change written and delivered after the recovery is
// received, within the deadline, and published when a request asks.
func recovers(t *testing.T, v *watched) {
	t.Helper()
	write(t, filepath.Join(v.root, "d1", "after.txt"), "after the recovery")
	v.deliver(change("d1/after.txt", resident.Added))
	snapshot := v.current()
	if p := v.publish(); !names(p, "d1/after.txt") {
		t.Errorf("the publish after the recovery reconciled %v, wanted d1/after.txt", p.Paths)
	}
	mirrors(t, snapshot, v.root)
}

// rearmsInside is the shared body of the failed-watch tests: after the failure
// is recorded, one Current re-arms and rebuilds inside itself.
func rearmsInside(t *testing.T, v *watched, arms int) {
	t.Helper()
	from := v.mark()
	snapshot := v.current()
	if got := v.manual.Arms(); got != arms+1 {
		t.Errorf("the notifier was armed %d times after the failure, wanted once", got-arms)
	}
	if p := v.publish(); !p.Rebuilt {
		t.Errorf("the request after the failure published %+v, wanted a rebuild", p)
	}
	v.passEndedInside(from, true)
	mirrors(t, snapshot, v.root)
	recovers(t, v)
}

// TestAFailedWatchRearmsInsideTheNextRequest is part of dinah-619/criteria/9.
// A failed Next parks the watch and reads nothing; the next Current re-arms
// and rebuilds inside itself, and the watcher resumes Next.
//
// Arming: making the watcher goroutine return after a failure, rather than
// park, leaves the change after the recovery never received.
func TestAFailedWatchRearmsInsideTheNextRequest(t *testing.T) {
	root := t.TempDir()
	tree(t, root, baseTree)
	v := watch(t, root)
	arms := v.manual.Arms()
	v.manual.Fail(errors.New("the watch handle failed"))
	waitFor(t, "the failure to park the watch", func() bool { return resident.Parked(v.w) })
	resident.Quiesce(v.w)
	rearmsInside(t, v, arms)
}

// TestAnInvalidRootRebuilds is part of dinah-619/criteria/9. A root that no
// longer names the watched directory makes the next Current record the
// failure, and it waits while the applier parks, closes and re-arms the
// notifier and rebuilds.
//
// Arming: making the watcher goroutine return on ErrClosed in every state
// leaves the change after the recovery never received.
func TestAnInvalidRootRebuilds(t *testing.T) {
	root := t.TempDir()
	tree(t, root, baseTree)
	v := watch(t, root)
	arms := v.manual.Arms()
	v.manual.Invalidate()
	rearmsInside(t, v, arms)
}

// TestARearmFailingKeepsServingFromDisk is part of dinah-619/criteria/9.
// While Arm keeps failing, each request's Current answers nil after exactly
// one failed attempt and no Arm succeeds; a Settle made while parked returns
// after its own failed attempt with no publish; and the next Current lets Arm
// succeed, answers the snapshot the rebuild inside it published, and a change
// delivered afterwards is received and published.
func TestARearmFailingKeepsServingFromDisk(t *testing.T) {
	root := t.TempDir()
	tree(t, root, baseTree)
	v := watch(t, root)
	arms := v.manual.Arms()
	v.manual.FailArm(4)
	v.manual.Fail(errors.New("the watch handle failed"))
	waitFor(t, "the failure to park the watch", func() bool { return resident.Parked(v.w) })
	resident.Quiesce(v.w)
	for i := 0; i < 3; i++ {
		before := v.manual.Attempts()
		if pick := v.request(); pick.Snapshot != nil {
			t.Errorf("request %d read a snapshot while Arm was failing", i+1)
		}
		if got := v.manual.Attempts() - before; got != 1 {
			t.Errorf("request %d returned after %d attempts to arm, wanted exactly one", i+1, got)
		}
		if v.manual.Arms() != arms {
			t.Fatalf("an Arm succeeded while FailArm held")
		}
	}
	before := v.manual.Attempts()
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	if err := v.w.Settle(ctx, filepath.Join(root, "d1")); err != nil {
		t.Errorf("the Settle made while parked answered %v", err)
	}
	if got := v.manual.Attempts() - before; got != 1 {
		t.Errorf("the Settle returned after %d attempts to arm, wanted exactly one", got)
	}
	if got := v.drain(); len(got) > 0 {
		t.Errorf("snapshots were published while Arm was failing: %+v", got)
	}
	rearmsInside(t, v, arms)
}

// waitFor polls a condition against the test deadline. The condition is a
// state the resident reaches on its own goroutines; the poll bounds the test
// and sleeps for no latency the design rests on.
func waitFor(t *testing.T, what string, done func() bool) {
	t.Helper()
	stop := time.After(deadline)
	for !done() {
		select {
		case <-stop:
			t.Fatalf("waited %s for %s", deadline, what)
		case <-time.After(time.Millisecond):
		}
	}
}

// TestCloseWhileParkedReturns is part of dinah-619/criteria/9. With the
// watch parked and a Settle queued, Close returns within the deadline,
// which proves both goroutines returned, and the queued Settle returns nil. A
// second Close answers nil and a Settle after Close returns nil at once.
// Then 200 rounds, each on a fresh workbench, of a failure followed at once
// by Close all return inside the deadline.
//
// Arming: closing the applier's signal channel in Close instead of w.done
// panics with a send on a closed channel, or leaves the applier spinning.
func TestCloseWhileParkedReturns(t *testing.T) {
	root := t.TempDir()
	tree(t, root, baseTree)
	v := watch(t, root)
	v.manual.FailArm(1000)
	v.manual.Fail(errors.New("the watch handle failed"))
	waitFor(t, "the failure to park the watch", func() bool { return resident.Parked(v.w) })
	queued := make(chan error, 1)
	go func() { queued <- v.w.Settle(context.Background(), filepath.Join(root, "d1")) }()
	closed := make(chan error, 1)
	go func() { closed <- v.w.Close() }()
	select {
	case <-closed:
	case <-time.After(deadline):
		t.Fatal("Close did not return while the watch was parked")
	}
	select {
	case err := <-queued:
		if err != nil {
			t.Errorf("the queued Settle answered %v", err)
		}
	case <-time.After(deadline):
		t.Fatal("the queued Settle did not return after Close")
	}
	if err := v.w.Close(); err != nil {
		t.Errorf("a second Close answered %v", err)
	}
	if err := v.w.Settle(context.Background(), root); err != nil {
		t.Errorf("Settle after Close answered %v", err)
	}

	const rounds = 200
	small := t.TempDir()
	write(t, filepath.Join(small, "only.txt"), "only")
	for round := 0; round < rounds; round++ {
		manual := residenttest.NewManual()
		w, err := resident.Open(small, resident.Options{Notifier: manual})
		if err != nil {
			t.Fatal(err)
		}
		manual.Fail(errors.New("the watch handle failed"))
		done := make(chan struct{})
		go func() { w.Close(); close(done) }()
		select {
		case <-done:
		case <-time.After(deadline):
			t.Fatalf("round %d: Close did not return", round)
		}
	}
	t.Logf("%d rounds of a failure followed at once by Close", rounds)
}

// TestCloseReleasesASettleItsPassHadTaken is part of dinah-619/criteria/9. A
// Settle taken by a pass that Close overtakes returns nil: the pass finds the
// workbench closed and releases what it took, because Close has already
// released the queue and no pass follows.
//
// Arming: putting a discarded pass's requests back on the queue when the
// state is closed leaves the Settle waiting until the deadline.
func TestCloseReleasesASettleItsPassHadTaken(t *testing.T) {
	root := t.TempDir()
	tree(t, root, baseTree)
	v := watch(t, root)
	hold := make(chan struct{})
	v.hold.Store(&hold)
	first := make(chan error, 1)
	go func() { first <- v.w.Settle(context.Background(), filepath.Join(root, "d1")) }()
	select {
	case <-v.holding:
	case <-time.After(deadline):
		t.Fatal("the pass taking the Settle never reached BeforePublish")
	}
	closed := make(chan error, 1)
	go func() { closed <- v.w.Close() }()
	// Close has set closed once a Settle of another directory returns nil at
	// once.
	waitFor(t, "Close to mark the workbench closed", func() bool {
		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
		defer cancel()
		return v.w.Settle(ctx, filepath.Join(root, "d2")) == nil
	})
	v.hold.Store(nil)
	close(hold)
	for name, done := range map[string]chan error{"Close": closed, "the first Settle": first} {
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("%s answered %v", name, err)
			}
		case <-time.After(deadline):
			t.Fatalf("%s did not return within %s", name, deadline)
		}
	}
}

// TestSettleIsVisibleToTheNextCurrent is part of dinah-619/criteria/4. A
// write with no change delivered, followed by a Settle with no Current call
// between them, is visible to the next Current, and the publish that released
// the Settle names the directory.
//
// Arming: removing Settle's signal to the applier leaves the Settle waiting
// until the deadline.
func TestSettleIsVisibleToTheNextCurrent(t *testing.T) {
	root := t.TempDir()
	tree(t, root, baseTree)
	v := watch(t, root)
	write(t, filepath.Join(root, "d1", "f1.txt"), "written, never reported")
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	if err := v.w.Settle(ctx, filepath.Join(root, "d1")); err != nil {
		t.Fatalf("Settle answered %v", err)
	}
	if p := v.publish(); !names(p, "d1") {
		t.Errorf("the publish that released the Settle reconciled %v, wanted d1", p.Paths)
	}
	data, err := v.current().ReadFile(filepath.Join(root, "d1", "f1.txt"))
	if err != nil || string(data) != "written, never reported" {
		t.Errorf("the next Current read %q, %v", data, err)
	}
}

// TestASettleDuringARebuildWaitsForTheNextPass is part of
// dinah-619/criteria/4. An overflow marks a rebuild due, and a Current asks
// for it and waits on it; the rebuild is held after its build has read the
// card's directory. The card's anchor is then written and a Settle made. The
// Current returns only after the held rebuild is released. The rebuild's
// publish, carrying the old anchor, is stored while the Settle has not
// returned; the Settle returns after a later publish naming the card's
// directory; and the snapshot a request reads after the Settle carries the
// new anchor.
//
// Arming: releasing every queued request at a pass's publish rather than
// only those taken at its start returns the Settle with the rebuild's stale
// publish.
func TestASettleDuringARebuildWaitsForTheNextPass(t *testing.T) {
	root := t.TempDir()
	tree(t, root, map[string]string{"cards/0123456789ab/card.md": "column: old"})
	v := watch(t, root)
	v.manual.Overflow()
	v.receive()
	resident.Quiesce(v.w)
	hold := make(chan struct{})
	v.hold.Store(&hold)
	current := make(chan *resident.Snapshot, 1)
	go func() { current <- v.current() }()
	var rebuilt resident.Published
	select {
	case rebuilt = <-v.holding:
	case <-time.After(deadline):
		t.Fatal("the rebuild never reached BeforePublish")
	}
	if !rebuilt.Rebuilt {
		t.Fatalf("the held pass was not the rebuild: %+v", rebuilt)
	}
	card := filepath.Join(root, "cards", "0123456789ab")
	write(t, filepath.Join(card, "card.md"), "column: new")
	type seen struct {
		err        error
		generation uint64
		anchor     string
	}
	settled := make(chan seen, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), deadline)
		defer cancel()
		err := v.w.Settle(ctx, card)
		got := seen{err: err}
		// What the Settle's caller sees the moment it returns: the snapshot a
		// request would then read.
		if snapshot := v.w.Current(ctx, time.Now()).Snapshot; snapshot != nil {
			got.generation = snapshot.Generation()
			data, _ := snapshot.ReadFile(filepath.Join(card, "card.md"))
			got.anchor = string(data)
		}
		settled <- got
	}()
	waitFor(t, "the Settle to be queued", func() bool { return resident.QueuedRequests(v.w) == 1 })
	select {
	case <-current:
		t.Fatal("the Current returned while the rebuild it asked for was held")
	default:
	}
	v.hold.Store(nil)
	close(hold)
	var snapshot *resident.Snapshot
	select {
	case snapshot = <-current:
	case <-time.After(deadline):
		t.Fatal("the Current did not return after the rebuild was released")
	}
	if snapshot.Generation() < rebuilt.Generation {
		t.Errorf("the Current answered generation %d, older than the rebuild's %d", snapshot.Generation(), rebuilt.Generation)
	}
	var got seen
	select {
	case got = <-settled:
	case <-time.After(deadline):
		t.Fatal("the Settle did not return")
	}
	if got.err != nil {
		t.Fatalf("Settle answered %v", got.err)
	}
	publishes := v.drain()
	if len(publishes) < 2 || !publishes[0].Rebuilt || publishes[0].Generation != rebuilt.Generation {
		t.Fatalf("the publishes were %+v, wanted the rebuild first and a later one", publishes)
	}
	later := publishes[1]
	if !names(later, "cards/0123456789ab") {
		t.Errorf("the publish after the rebuild reconciled %v, wanted the card's directory", later.Paths)
	}
	if got.generation <= rebuilt.Generation {
		t.Fatalf("the Settle returned while the snapshot served was generation %d, the rebuild's, whose reads began before the Settle", got.generation)
	}
	if got.anchor != "column: new" {
		t.Errorf("the snapshot served when the Settle returned carries %q, not the write made before it", got.anchor)
	}
}
