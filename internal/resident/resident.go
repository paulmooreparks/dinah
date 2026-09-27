// Package resident holds one workbench in memory for dinah serve, kept
// current by the operating system's documented change notification, so that
// a read is answered without opening a file. It serves reads only: every act
// reads and writes the disk, and nothing writes through a resident.
//
// A Workbench publishes immutable snapshots through one atomic pointer. One
// goroutine, the applier, is the only code that builds a snapshot, stores the
// pointer, or arms and closes the notifier other than from Close; a request
// loads the pointer once and reads that one snapshot for its whole life.
//
// The resident opens a file or a directory below the workbench root only in
// three places: Open's Notifier.Arm, before the server listens; a pass or a
// re-arm that a queued Current asked for; and a pass that a queued Settle
// asked for. A received change, an overflow and a failed watch are only
// recorded when they happen, and the work they call for is done by the next
// pass a request asks for, while that request waits. So the resident reads
// below the root only while a request is being answered, which is when a
// server reading the disk per request reads too, and it holds nothing below
// the root once that request has answered. dinah-619/decisions/28 records
// this, after the operator's ruling on dinah-619/questions/1 that folders
// above a served workbench stay renamable, movable and deletable.
package resident

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// Options are what Open takes beside the root. The zero value is production.
type Options struct {
	// Notifier reports changes. Nil means the platform's own watcher, and
	// Open answers &Unsupported{WhyPlatform} where there is none.
	Notifier Notifier
	// Hooks are test levers, nil in production.
	Hooks *Hooks
}

// ErrUnsupported is what every *Unsupported answers to under errors.Is: this
// platform or this volume has no watcher the design may use. The caller
// serves from disk.
var ErrUnsupported = errors.New("resident: no documented watcher for this workbench")

// Why names the reason a workbench is served from disk rather than held.
type Why string

const (
	// WhyPlatform: this operating system has no watcher in this design.
	WhyPlatform Why = "platform"
	// WhyVolumeType: the workbench's volume is not a local fixed volume.
	WhyVolumeType Why = "volume-type"
	// WhyMountedInFolder: the workbench's volume is mounted in a folder of
	// another volume rather than at a drive letter.
	WhyMountedInFolder Why = "mounted-in-folder"
	// WhyNoDOSPath: Windows gives the workbench's volume GUID path but no DOS
	// path, as for a volume with no drive letter and no folder it is mounted
	// in.
	WhyNoDOSPath Why = "no-dos-path"
	// WhyError: Open failed for any other reason; the error is kept beside
	// it.
	WhyError Why = "error"
)

// Unsupported is Open's error, and Arm's on Windows, when the workbench is on
// a platform or a volume the watcher does not serve. Its Is answers true for
// ErrUnsupported.
type Unsupported struct{ Why Why }

func (u *Unsupported) Error() string {
	return "resident: no documented watcher for this workbench (" + string(u.Why) + ")"
}

// Is answers true for ErrUnsupported.
func (u *Unsupported) Is(target error) bool {
	return target == ErrUnsupported
}

// Hooks are test levers, nil in production. Every member may be nil.
type Hooks struct {
	// BeforePublish runs on the applier after a snapshot is built and before
	// it is published.
	BeforePublish func(Published)
	// AfterPublish runs on the applier after a snapshot is published.
	AfterPublish func(Published)
	// BeforeRearm runs on the Windows watcher's goroutine after a completion
	// is consumed and before the next ReadDirectoryChangesW call is issued.
	BeforeRearm func()
	// PassThrough runs whenever a snapshot answers a path below the root by
	// reading the disk.
	PassThrough func(path string)
	// AfterReceive runs on the watcher goroutine after a batch has been added
	// to the pending set, with the batch as the notifier answered it (on
	// Windows, already filtered to the workbench and relative to its root).
	// Tests wait on it where they once waited on a publish, because a
	// received change is published only when a request asks.
	AfterReceive func(Batch)
	// BeforePass runs on the applier at the start of every pass, before it
	// reads anything. Every pass is asked for by a queued Current or Settle.
	BeforePass func(Pass)
	// AfterPass runs on the applier when a pass ends, after its publish is
	// stored or after it is discarded, and before the requests it took are
	// released. A test that sees AfterPass run before its Current or Settle
	// returns has seen the pass end inside the request.
	AfterPass func(p Pass, published bool)
}

// Pass describes one pass of the applier.
type Pass struct {
	// Rebuild marks a pass that reads the whole tree rather than reconciling
	// paths.
	Rebuild bool
	// Requests is how many queued Current and Settle requests the pass took.
	// It is at least one for every pass.
	Requests int
}

// Published describes one publish.
type Published struct {
	Generation uint64
	// Paths are the paths reconciled, relative to the root and slash
	// separated; empty on a rebuild.
	Paths []string
	// Rebuilt marks a snapshot read whole from disk.
	Rebuilt bool
}

// Pick is Current's answer.
type Pick struct {
	// Snapshot is the snapshot to read, nil when the request reads the disk.
	Snapshot *Snapshot
	// Lapsing are the directories of the cards whose claims have lapsed by
	// now. They are non-empty only when Snapshot is nil for that reason.
	Lapsing []string
}

// watchState is the state of the watch, which w.mu guards.
type watchState int

const (
	// watching: the notifier is armed and the watcher goroutine calls Next.
	watching watchState = iota
	// parked: the watch is down; the watcher goroutine waits and calls
	// nothing on the notifier.
	parked
	// closed: Workbench.Close has run. Terminal.
	closed
)

// request is one Current or Settle waiting in the queue.
type request struct {
	// settle marks a Settle, which reconciles the root's own files and
	// listing and each of dirs, deep; a Current carries neither.
	settle bool
	dirs   []string // relative keys, the platform's separators
	done   chan struct{}
	once   sync.Once
	// taken reports, under w.mu, that a pass has taken the request, which
	// then waits for that pass to end whatever its context says.
	taken bool
}

// release closes the request's channel once.
func (r *request) release() {
	r.once.Do(func() { close(r.done) })
}

// Workbench is one workbench held in memory and kept current by change
// notification. It serves reads only; nothing writes through it.
//
// Three invariants hold at every instant. The published pointer is non-nil
// only while the watch state is watching: every transition out of watching
// stores nil in the same critical section that changes the state, and the
// applier stores a snapshot only after checking, under w.mu, that the state
// is still watching and no failure is pending. Only Close ends the watcher
// goroutine, which leaves its loop only when it observes closed. And every
// pass and every re-arm runs while a request waits on it: the applier starts
// one only while at least one request is queued, a pass takes the queued
// requests before it reads, and a request a pass has taken waits until that
// pass has ended, whatever its context says.
type Workbench struct {
	root     string
	notifier Notifier
	hooks    *Hooks
	// longForm resolves a path prefix to its long form, which on Windows is
	// GetLongPathNameW and elsewhere the path unchanged.
	longForm func(path string) (string, error)

	published atomic.Pointer[Snapshot]
	// dirty is set by the watcher when it records a change or an overflow,
	// and cleared under w.mu when a pass that took the pending set
	// publishes with nothing newer pending, so a Current that finds it clear
	// waits for nothing and the snapshot it finds carries every change
	// received before it looked.
	dirty     atomic.Bool
	ready     chan struct{}
	readyOnce sync.Once

	// signal wakes the applier. It has capacity one and is never closed,
	// because the watcher, Current and Settle send on it without holding
	// w.mu and may do so after Close.
	signal chan struct{}
	// probe is how a test asks the applier to handle every signal sent so
	// far and answer; nothing in production sends on it.
	probe chan chan struct{}
	// done is closed by Close, and the applier selects on it beside signal.
	done chan struct{}
	// goroutines counts the watcher and the applier.
	goroutines sync.WaitGroup

	mu         sync.Mutex
	cond       *sync.Cond
	state      watchState
	pending    []Change
	overflow   bool
	failed     bool
	queue      []*request
	inNext     bool
	rebuildDue bool
	// overflows counts the overflows the watcher has reported, which a test
	// reads to tell a rebuild an overflow caused from one a failure caused.
	overflows int

	// base is the last snapshot a pass built, which the next reconcile
	// starts from, and gen the generation of the last publish. The applier
	// alone touches them.
	base *Snapshot
	gen  uint64
}

// Open arms the watcher on root and reads nothing else: the first build runs
// inside the first request that asks for a snapshot. It returns once the
// watcher is armed, so a change made after it returns is certain to be
// reported.
func Open(root string, opts Options) (*Workbench, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return nil, errors.New("resident: the root must be an absolute, clean path")
	}
	notifier := opts.Notifier
	if notifier == nil {
		platform, err := platformNotifier(opts.Hooks)
		if err != nil {
			return nil, err
		}
		notifier = platform
	}
	w := &Workbench{
		root:       root,
		notifier:   notifier,
		hooks:      opts.Hooks,
		longForm:   longFormOf,
		ready:      make(chan struct{}),
		signal:     make(chan struct{}, 1),
		probe:      make(chan chan struct{}),
		done:       make(chan struct{}),
		state:      watching,
		rebuildDue: true,
	}
	w.cond = sync.NewCond(&w.mu)
	if err := notifier.Arm(root); err != nil {
		return nil, err
	}
	w.goroutines.Add(2)
	go w.watch()
	go w.apply()
	return w, nil
}

// longFormOf is the resolver Open gives each workbench: GetLongPathNameW on
// Windows and the path unchanged elsewhere. Only a test replaces it.
var longFormOf = longPath

// poke signals the applier without waiting.
func (w *Workbench) poke() {
	select {
	case w.signal <- struct{}{}:
	default:
	}
}

// Ready is closed when the first pass ends, whether it published a snapshot,
// failed, or was discarded because the watch went down while it ran. Nothing
// starts a pass until a Current or a Settle asks, so Ready is never closed by
// Open alone. Ready says only that the first pass is over; Current, not
// Ready, says whether a snapshot is served.
func (w *Workbench) Ready() <-chan struct{} {
	return w.ready
}

// markReady closes Ready once.
func (w *Workbench) markReady() {
	w.readyOnce.Do(func() { close(w.ready) })
}

// Current answers the snapshot a read at now may use, or a Pick whose
// Snapshot is nil when the request must read the disk. When no snapshot is
// published, when the root is no longer valid, or when changes have been
// received and not yet applied, it asks the applier for the work that is due
// (a re-arm, a rebuild or a reconcile) and waits for it, so that the snapshot
// it answers carries every change received before the call. ctx ends the wait
// only while the request is still queued; once a pass has taken it, Current
// waits for that pass to end whatever ctx says, so no pass outlives the
// requests waiting on it. When ctx ends while it is queued, it answers a nil
// Snapshot.
//
// Its checks run in this order: the pointer, Valid, the dirty flag, and then
// the expiry. After the one wait, it loads the pointer again and repeats the
// pointer and Valid checks without waiting a second time.
func (w *Workbench) Current(ctx context.Context, now time.Time) Pick {
	// dirty is read before the pointer. It is cleared only by a publish,
	// after that publish's pointer is stored, so a request that finds it
	// clear then loads that pointer or a later one; read the other way
	// round, a request could load the pointer a pass is about to replace
	// and then find dirty cleared by that pass.
	dirty := w.dirty.Load()
	snapshot := w.published.Load()
	valid := snapshot != nil && w.notifier.Valid()
	if snapshot == nil || !valid || dirty {
		r := &request{done: make(chan struct{})}
		w.mu.Lock()
		if w.state == closed {
			w.mu.Unlock()
			return Pick{}
		}
		if snapshot != nil && !valid {
			// A failed Valid is recorded in the same critical section as
			// the queueing, so the applier parks and re-arms for this
			// request.
			w.failed = true
		}
		w.queue = append(w.queue, r)
		w.mu.Unlock()
		w.poke()
		if w.wait(ctx, r) != nil {
			return Pick{}
		}
		snapshot = w.published.Load()
		if snapshot == nil || !w.notifier.Valid() {
			return Pick{}
		}
	}
	if !snapshot.earliestExpiry.IsZero() && !now.Before(snapshot.earliestExpiry) {
		return Pick{Lapsing: snapshot.lapsedAt(now)}
	}
	return Pick{Snapshot: snapshot}
}

// wait waits for a queued request to be released. The request may leave when
// ctx ends only while it is still queued, and it is then removed from the
// queue under w.mu; a request a pass has taken waits for the pass to end.
func (w *Workbench) wait(ctx context.Context, r *request) error {
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
	}
	w.mu.Lock()
	select {
	case <-r.done:
		w.mu.Unlock()
		return nil
	default:
	}
	if r.taken {
		w.mu.Unlock()
		<-r.done
		return nil
	}
	for i, queued := range w.queue {
		if queued == r {
			w.queue = append(w.queue[:i:i], w.queue[i+1:]...)
			break
		}
	}
	w.mu.Unlock()
	return ctx.Err()
}

// lapsedAt answers the directories of the cards whose claims have lapsed at
// now, which is Card.Lapsed's rule: now is not before the expiry.
func (s *Snapshot) lapsedAt(now time.Time) []string {
	var dirs []string
	for _, entry := range s.lapsing {
		if now.Before(entry.at) {
			break
		}
		dirs = append(dirs, entry.dir)
	}
	return dirs
}

// Settle makes every snapshot published after it returns reflect the disk as
// it stood when Settle was called, for the workbench root's own files and
// listing and for each named directory's whole subtree. It returns nil at
// once only when the workbench is closed. Otherwise it waits for a publish
// whose reads all began after the call, or for the applier to establish that
// no publish can follow except one whose reads begin after the call. It
// returns ctx's error when ctx ends while the request is still queued; once a
// pass has taken it, it waits for that pass to end, as Current does.
func (w *Workbench) Settle(ctx context.Context, dirs ...string) error {
	r := &request{settle: true, done: make(chan struct{})}
	for _, dir := range dirs {
		if rel, ok := relativeTo(w.root, filepath.Clean(dir)); ok {
			r.dirs = append(r.dirs, rel)
		}
	}
	w.mu.Lock()
	if w.state == closed {
		w.mu.Unlock()
		return nil
	}
	w.queue = append(w.queue, r)
	w.mu.Unlock()
	// Without this signal an act on an idle workbench would wait for some
	// unrelated request to wake the applier.
	w.poke()
	return w.wait(ctx, r)
}

// Close stops the watcher, waits for both goroutines to finish, and releases
// every handle and every byte outside the Go heap.
func (w *Workbench) Close() error {
	w.mu.Lock()
	if w.state == closed {
		w.mu.Unlock()
		return nil
	}
	w.state = closed
	w.published.Store(nil)
	for _, r := range w.queue {
		r.release()
	}
	w.queue = nil
	w.cond.Broadcast()
	w.mu.Unlock()
	err := w.notifier.Close()
	close(w.done)
	w.goroutines.Wait()
	w.markReady()
	return err
}

// Held answers how many files the current snapshot holds and how many bytes.
func (w *Workbench) Held() (int, int64) {
	snapshot := w.published.Load()
	if snapshot == nil {
		return 0, 0
	}
	return snapshot.Held()
}

// watch is the watcher goroutine, the only caller of Notifier.Next. It never
// waits on the applier, so a slow pass never delays the next Next, and it
// reads nothing: a change is only recorded.
func (w *Workbench) watch() {
	defer w.goroutines.Done()
	for {
		w.mu.Lock()
		for w.state == parked {
			w.cond.Wait()
		}
		if w.state == closed {
			w.mu.Unlock()
			return
		}
		w.inNext = true
		w.mu.Unlock()

		batch, err := w.notifier.Next()

		w.mu.Lock()
		w.inNext = false
		w.cond.Broadcast()
		received := false
		switch {
		case err == nil:
			// A batch arriving in any state but watching is dropped, because
			// the rebuild that ends every recovery reads everything.
			if w.state == watching {
				if batch.Overflow {
					w.overflow = true
					w.overflows++
				} else {
					w.pending = append(w.pending, batch.Changes...)
				}
				w.dirty.Store(true)
				received = true
			}
		case errors.Is(err, ErrClosed):
			// The applier closed the notifier to re-arm it, or Close ran:
			// the next turn of the loop parks or exits.
		default:
			if w.state == watching {
				w.state = parked
				w.published.Store(nil)
				w.failed = true
			}
		}
		w.mu.Unlock()
		if received && w.hooks != nil && w.hooks.AfterReceive != nil {
			w.hooks.AfterReceive(batch)
		}
		w.poke()
	}
}

// apply is the applier goroutine.
func (w *Workbench) apply() {
	defer w.goroutines.Done()
	for {
		select {
		case <-w.done:
			return
		case <-w.signal:
			for w.step() {
			}
		case reply := <-w.probe:
			select {
			case <-w.signal:
				for w.step() {
				}
			default:
			}
			close(reply)
		}
	}
}

// step runs one turn of the applier and reports whether another turn should
// follow at once. The rows that read nothing (the park after a failure, and
// the overflow) run on every signal. The re-arm and the pass read, so they
// run only while a request is queued, and a signal with none leaves them
// where they are for the next request's signal.
func (w *Workbench) step() bool {
	w.mu.Lock()
	if w.state == closed {
		w.mu.Unlock()
		return false
	}
	if w.failed {
		if w.state == watching {
			w.state = parked
			w.published.Store(nil)
		}
		w.failed = false
		w.mu.Unlock()
		// A Close that begins after this unlock may call Notifier.Close
		// while this call runs; the contract makes a second Close safe.
		w.notifier.Close()
		w.mu.Lock()
		for w.inNext {
			w.cond.Wait()
		}
		if w.state == closed {
			w.mu.Unlock()
			return false
		}
	}
	if w.overflow {
		// dirty stays set until the rebuild publishes.
		w.published.Store(nil)
		w.pending = nil
		w.overflow = false
		w.rebuildDue = true
	}
	if len(w.queue) == 0 {
		w.mu.Unlock()
		return false
	}
	if w.state == parked {
		// Arm is called holding w.mu, so no Arm can start after Close has
		// marked the workbench closed, and none can be left outstanding.
		if err := w.notifier.Arm(w.root); err != nil {
			// While parked no snapshot can be published until a re-arm
			// succeeds and a rebuild runs, and that rebuild begins after
			// this attempt, so every queued request is released now: a
			// failing Arm is tried once per request, not in a loop.
			for _, r := range w.queue {
				r.release()
			}
			w.queue = nil
			w.mu.Unlock()
			return false
		}
		w.state = watching
		w.pending = nil
		w.overflow = false
		w.rebuildDue = true
		w.cond.Broadcast()
	}
	rebuild := w.rebuildDue || w.base == nil
	due := rebuild || len(w.pending) > 0
	for _, r := range w.queue {
		due = due || r.settle
	}
	if !due {
		// Nothing is due: every change received before any queued request
		// was made has been taken by a pass that has since published, so the
		// snapshot published now already carries it. No pass runs for them.
		for _, r := range w.queue {
			r.release()
		}
		w.queue = nil
		w.mu.Unlock()
		return false
	}
	// dirty stays set while this pass runs, and is cleared at its publish
	// when nothing arrived meanwhile: a Current that found it clear while
	// the pass ran would answer the published snapshot, which lacks the
	// changes this pass took, though they were received before the call.
	changes := w.pending
	w.pending = nil
	w.rebuildDue = false
	taken := w.queue
	w.queue = nil
	for _, r := range taken {
		r.taken = true
	}
	w.mu.Unlock()

	pass := Pass{Rebuild: rebuild, Requests: len(taken)}
	if w.hooks != nil && w.hooks.BeforePass != nil {
		w.hooks.BeforePass(pass)
	}
	var snapshot *Snapshot
	var published Published
	if rebuild {
		snapshot = finish(w.root, buildAll(w.root), w.gen+1, w.hooks)
		published = Published{Generation: w.gen + 1, Rebuilt: true}
	} else {
		dirs, paths := reconcile(w.root, w.base.dirs, changes, taken, w.longForm)
		snapshot = finish(w.root, dirs, w.gen+1, w.hooks)
		published = Published{Generation: w.gen + 1, Paths: paths}
	}
	if w.hooks != nil && w.hooks.BeforePublish != nil {
		w.hooks.BeforePublish(published)
	}

	w.mu.Lock()
	if w.state != watching || w.failed {
		// A pass that finds the watch down is discarded. Its requests go
		// back to the head of the queue for the pass that follows the
		// re-arm, or are released when the workbench closed, because Close
		// has already released the queue and no pass follows.
		closedNow := w.state == closed
		if !closedNow {
			for _, r := range taken {
				r.taken = false
			}
			w.queue = append(taken, w.queue...)
			if !w.failed {
				w.rebuildDue = true
			}
		}
		w.mu.Unlock()
		w.markReady()
		w.afterPass(pass, false)
		if closedNow {
			for _, r := range taken {
				r.release()
			}
			return false
		}
		return true
	}
	w.base = snapshot
	w.gen = snapshot.gen
	w.published.Store(snapshot)
	if len(w.pending) == 0 && !w.overflow {
		w.dirty.Store(false)
	}
	more := len(w.queue) > 0 || w.failed || w.overflow
	w.mu.Unlock()
	w.markReady()
	if w.hooks != nil && w.hooks.AfterPublish != nil {
		w.hooks.AfterPublish(published)
	}
	w.afterPass(pass, true)
	for _, r := range taken {
		r.release()
	}
	return more
}

// afterPass calls Hooks.AfterPass when a test set it.
func (w *Workbench) afterPass(pass Pass, published bool) {
	if w.hooks != nil && w.hooks.AfterPass != nil {
		w.hooks.AfterPass(pass, published)
	}
}
