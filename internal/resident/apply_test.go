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

// watched is a resident over a test tree with a hand-driven notifier and a
// channel of its publishes.
type watched struct {
	t         *testing.T
	root      string
	w         *resident.Workbench
	manual    *residenttest.Manual
	published chan resident.Published
	// hold, when set, makes BeforePublish wait on it.
	hold atomic.Pointer[chan struct{}]
	// holding receives a publish BeforePublish is waiting on.
	holding chan resident.Published
}

// watch opens a resident over root and waits for its first publish.
func watch(t *testing.T, root string) *watched {
	t.Helper()
	v := &watched{t: t, root: root, manual: residenttest.NewManual(), published: make(chan resident.Published, 256), holding: make(chan resident.Published, 16)}
	hooks := &resident.Hooks{
		BeforePublish: func(p resident.Published) {
			if hold := v.hold.Load(); hold != nil {
				v.holding <- p
				<-*hold
			}
		},
		AfterPublish: func(p resident.Published) { v.published <- p },
	}
	w, err := resident.Open(root, resident.Options{Notifier: v.manual, Hooks: hooks})
	if err != nil {
		t.Fatalf("open the resident: %v", err)
	}
	v.w = w
	t.Cleanup(func() { w.Close() })
	first := v.next()
	if !first.Rebuilt {
		t.Fatalf("the first publish was not a rebuild: %+v", first)
	}
	return v
}

// next waits for the next publish.
func (v *watched) next() resident.Published {
	v.t.Helper()
	select {
	case p := <-v.published:
		return p
	case <-time.After(deadline):
		v.t.Fatalf("no publish arrived within %s", deadline)
		return resident.Published{}
	}
}

// until waits for a publish satisfying want, answering it.
func (v *watched) until(what string, want func(resident.Published) bool) resident.Published {
	v.t.Helper()
	timeout := time.After(deadline)
	for {
		select {
		case p := <-v.published:
			if want(p) {
				return p
			}
		case <-timeout:
			v.t.Fatalf("no publish %s arrived within %s", what, deadline)
			return resident.Published{}
		}
	}
}

// current answers the current snapshot, failing when there is none.
func (v *watched) current() *resident.Snapshot {
	v.t.Helper()
	pick := v.w.Current(time.Now())
	if pick.Snapshot == nil {
		v.t.Fatal("the resident answered no snapshot")
	}
	return pick.Snapshot
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

// TestAReconcileMatchesTheDiskForEveryChange is part of
// dinah-619/criteria/8. Each case changes the disk, delivers the changes the
// watcher would report, waits for the publish, and compares the snapshot with
// a fresh build of the disk.
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
			v.manual.Deliver(c.changes...)
			p := v.next()
			if p.Rebuilt {
				t.Fatalf("the change was applied by a rebuild, which proves nothing about the reconcile")
			}
			mirrors(t, v.current(), root)
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
	v.manual.Deliver(change("cards/ZZZZZZ~1/card.md", resident.Modified))
	p := v.next()
	if !names(p, "cards") {
		t.Errorf("the publish reconciled %v, wanted cards", p.Paths)
	}
	mirrors(t, v.current(), root)
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
				snapshot := v.w.Current(time.Now()).Snapshot
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
		v.manual.Deliver(batch...)
		want := strconv.Itoa(round)
		for {
			p := v.next()
			published++
			data, err := v.current().ReadFile(filepath.Join(root, filepath.FromSlash(files[len(files)-1])))
			if p.Rebuilt || (err == nil && string(data) == want) {
				break
			}
		}
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
// changed with no change delivered, then an overflow is reported. While the
// rebuild is held before its publish, Current answers nil; after it, the
// publish is a rebuild and the snapshot mirrors the disk.
func TestOverflowRebuildsFromDisk(t *testing.T) {
	root := t.TempDir()
	tree(t, root, baseTree)
	v := watch(t, root)
	write(t, filepath.Join(root, "d1", "unreported.txt"), "never reported")
	os.RemoveAll(filepath.Join(root, "d2"))
	hold := make(chan struct{})
	v.hold.Store(&hold)
	v.manual.Overflow()
	select {
	case <-v.holding:
	case <-time.After(deadline):
		t.Fatal("no rebuild reached BeforePublish")
	}
	if pick := v.w.Current(time.Now()); pick.Snapshot != nil {
		t.Error("Current answered a snapshot while the overflow's rebuild was held")
	}
	v.hold.Store(nil)
	close(hold)
	p := v.next()
	if !p.Rebuilt {
		t.Errorf("the publish after an overflow was not a rebuild: %+v", p)
	}
	mirrors(t, v.current(), root)
}

// recovers is the part of the two failed-watch tests that proves the watcher
// resumed Next: a change written and delivered after the recovery is
// published.
func recovers(t *testing.T, v *watched) {
	t.Helper()
	p := v.until("rebuilding after the failure", func(p resident.Published) bool { return p.Rebuilt })
	if p.Generation < 2 {
		t.Errorf("the rebuild's generation is %d", p.Generation)
	}
	mirrors(t, v.current(), v.root)
	write(t, filepath.Join(v.root, "d1", "after.txt"), "after the recovery")
	v.manual.Deliver(change("d1/after.txt", resident.Added))
	v.until("naming d1/after.txt", func(p resident.Published) bool { return names(p, "d1/after.txt") })
	mirrors(t, v.current(), v.root)
}

// TestAFailedWatchServesFromDiskUntilRearmed is part of dinah-619/criteria/9.
// A watch failure gives Current nil, the notifier is re-armed once, a
// rebuild publishes, and a change delivered afterwards is published, which
// only a watcher that resumed Next can produce.
//
// Arming: making the watcher goroutine return on ErrClosed in every state
// leaves the change after the recovery untaken.
func TestAFailedWatchServesFromDiskUntilRearmed(t *testing.T) {
	root := t.TempDir()
	tree(t, root, baseTree)
	v := watch(t, root)
	arms := v.manual.Arms()
	// The rebuild after the re-arm is held before its publish, so the window
	// in which the watch is down cannot close before the test looks at it.
	hold := make(chan struct{})
	v.hold.Store(&hold)
	v.manual.Fail(errors.New("the watch handle failed"))
	select {
	case p := <-v.holding:
		if !p.Rebuilt {
			t.Fatalf("the pass after the failure was not a rebuild: %+v", p)
		}
	case <-time.After(deadline):
		t.Fatal("no rebuild followed the failure")
	}
	if pick := v.w.Current(time.Now()); pick.Snapshot != nil {
		t.Error("Current answered a snapshot while the watch was being restored")
	}
	v.hold.Store(nil)
	close(hold)
	recovers(t, v)
	if got := v.manual.Arms(); got != arms+1 {
		t.Errorf("the notifier was armed %d times after the failure, wanted once", got-arms)
	}
}

// TestAnInvalidRootRebuilds is part of dinah-619/criteria/9. A root that no
// longer names the watched directory gives Current nil, and the notifier is
// re-armed, a rebuild publishes, and a change delivered afterwards is
// published.
//
// Arming: as TestAFailedWatchServesFromDiskUntilRearmed.
func TestAnInvalidRootRebuilds(t *testing.T) {
	root := t.TempDir()
	tree(t, root, baseTree)
	v := watch(t, root)
	arms := v.manual.Arms()
	v.manual.Invalidate()
	if pick := v.w.Current(time.Now()); pick.Snapshot != nil {
		t.Error("Current answered a snapshot for a root that no longer names the watched directory")
	}
	recovers(t, v)
	if got := v.manual.Arms(); got != arms+1 {
		t.Errorf("the notifier was armed %d times after the invalid root, wanted once", got-arms)
	}
}

// TestARearmFailingKeepsServingFromDisk is part of dinah-619/criteria/9.
// While Arm keeps failing, every request's Current answers nil and no arm
// succeeds; a Settle made while parked returns after the next failed attempt
// with no publish; and once the failing attempts are spent, the next
// request's Current lets Arm succeed, a rebuild publishes, and a change
// delivered afterwards is published.
//
// The attempts are counted rather than the requests, because a request's
// signal to the applier coalesces with any other pending one, so which
// request an attempt answers is the scheduler's business. What the design
// promises, and this asserts, is that no attempt succeeds while FailArm
// holds and that a request made after it is spent is what re-arms.
func TestARearmFailingKeepsServingFromDisk(t *testing.T) {
	root := t.TempDir()
	tree(t, root, baseTree)
	v := watch(t, root)
	arms, attempts := v.manual.Arms(), v.manual.Attempts()
	v.manual.FailArm(3)
	v.manual.Fail(errors.New("the watch handle failed"))
	waitFor(t, "the first attempt to fail", func() bool { return v.manual.Attempts() == attempts+1 })
	settled := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), deadline)
		defer cancel()
		settled <- v.w.Settle(ctx, filepath.Join(root, "d1"))
	}()
	select {
	case err := <-settled:
		if err != nil {
			t.Errorf("the Settle made while parked answered %v", err)
		}
	case <-time.After(deadline):
		t.Fatal("the Settle made while parked did not return after a failed Arm")
	}
	requests := 0
	for v.manual.Attempts() < attempts+3 {
		before := v.manual.Attempts()
		if pick := v.w.Current(time.Now()); pick.Snapshot != nil {
			t.Errorf("a request read a snapshot while Arm was failing")
		}
		requests++
		waitFor(t, "the request's attempt", func() bool { return v.manual.Attempts() > before })
		if v.manual.Arms() != arms {
			t.Fatalf("an Arm succeeded while FailArm held")
		}
	}
	select {
	case p := <-v.published:
		t.Errorf("a snapshot was published while Arm was failing: %+v", p)
	default:
	}
	if pick := v.w.Current(time.Now()); pick.Snapshot != nil {
		t.Errorf("the request after the failing attempts read a snapshot before the re-arm's rebuild")
	}
	waitFor(t, "the Arm after the failing attempts to succeed", func() bool { return v.manual.Arms() == arms+1 })
	t.Logf("three failing attempts, %d of them answering a request's signal", requests)
	recovers(t, v)
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
// panics with a send on a closed channel.
func TestCloseWhileParkedReturns(t *testing.T) {
	root := t.TempDir()
	tree(t, root, baseTree)
	v := watch(t, root)
	v.manual.FailArm(1000)
	v.manual.Fail(errors.New("the watch handle failed"))
	waitFor(t, "Current to answer nil", func() bool { return v.w.Current(time.Now()).Snapshot == nil })
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
	// Close has set closed once a Settle of another directory returns at
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
// it names the directory.
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
	data, err := v.current().ReadFile(filepath.Join(root, "d1", "f1.txt"))
	if err != nil || string(data) != "written, never reported" {
		t.Errorf("the next Current read %q, %v", data, err)
	}
	p := v.next()
	if !names(p, "d1") {
		t.Errorf("the publish that released the Settle reconciled %v, wanted d1", p.Paths)
	}
}

// TestASettleDuringARebuildWaitsForTheNextPass is part of
// dinah-619/criteria/4. A rebuild is held after its build has read the
// card's directory; the card's anchor is then written and a Settle made. The
// rebuild's publish, carrying the old anchor, is stored while the Settle has
// not returned; the Settle returns after a later publish naming the card's
// directory; and the snapshot after it carries the new anchor.
//
// Arming: releasing every queued request at a pass's publish rather than
// only those taken at its start returns the Settle with the rebuild's stale
// publish.
func TestASettleDuringARebuildWaitsForTheNextPass(t *testing.T) {
	root := t.TempDir()
	tree(t, root, map[string]string{"cards/0123456789ab/card.md": "column: old"})
	v := watch(t, root)
	hold := make(chan struct{})
	v.hold.Store(&hold)
	v.manual.Overflow()
	select {
	case <-v.holding:
	case <-time.After(deadline):
		t.Fatal("the rebuild never reached BeforePublish")
	}
	card := filepath.Join(root, "cards", "0123456789ab")
	write(t, filepath.Join(card, "card.md"), "column: new")
	// What the Settle's caller sees the moment it returns: the snapshot a
	// request would then read. A Settle released by the rebuild's publish
	// sees the rebuild, which read the card before the write.
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
		if snapshot := v.w.Current(time.Now()).Snapshot; snapshot != nil {
			got.generation = snapshot.Generation()
			data, _ := snapshot.ReadFile(filepath.Join(card, "card.md"))
			got.anchor = string(data)
		}
		settled <- got
	}()
	// The Settle is queued before the rebuild is released.
	waitFor(t, "the Settle to be queued", func() bool { return resident.QueuedSettles(v.w) == 1 })
	v.hold.Store(nil)
	close(hold)
	rebuilt := v.next()
	if !rebuilt.Rebuilt {
		t.Fatalf("the held pass was not the rebuild: %+v", rebuilt)
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
	if got.generation <= rebuilt.Generation {
		t.Fatalf("the Settle returned while the snapshot served was generation %d, the rebuild's, whose reads began before the Settle", got.generation)
	}
	if got.anchor != "column: new" {
		t.Errorf("the snapshot served when the Settle returned carries %q, not the write made before it", got.anchor)
	}
	later := v.until("naming the card's directory", func(p resident.Published) bool {
		return names(p, "cards/0123456789ab")
	})
	if later.Generation <= rebuilt.Generation || later.Generation > got.generation {
		t.Errorf("the publish naming the card is generation %d, which is not after the rebuild's %d and at or before the one the Settle returned on, %d", later.Generation, rebuilt.Generation, got.generation)
	}
}
