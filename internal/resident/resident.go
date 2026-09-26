// Package resident holds one workbench in memory for dinah serve, kept
// current by the operating system's documented change notification, so that
// a read is answered without opening a file. It serves reads only: every act
// reads and writes the disk, and nothing writes through a resident.
//
// A Workbench publishes immutable snapshots through one atomic pointer. One
// goroutine, the applier, is the only code that builds a snapshot, stores the
// pointer, or arms and closes the notifier other than from Close; a request
// loads the pointer once and reads that one snapshot for its whole life.
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
	// Open answers ErrUnsupported where there is none.
	Notifier Notifier
	// Hooks are test levers, nil in production.
	Hooks *Hooks
}

// ErrUnsupported is Open's answer where this platform or this volume has no
// watcher the design may use. The caller serves from disk.
var ErrUnsupported = errors.New("resident: no documented watcher for this workbench")

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

// settleRequest is one Settle waiting for a publish.
type settleRequest struct {
	dirs []string // relative keys, the platform's separators
	done chan struct{}
	// released reports that done has been closed.
	released bool
}

// release closes the request's channel once.
func (r *settleRequest) release() {
	if !r.released {
		r.released = true
		close(r.done)
	}
}

// Workbench is one workbench held in memory and kept current by change
// notification. It serves reads only; nothing writes through it.
//
// Two invariants hold at every instant. The published pointer is non-nil only
// while the watch state is watching: every transition out of watching stores
// nil in the same critical section that changes the state, and the applier
// stores a snapshot only after checking, under w.mu, that the state is still
// watching and no failure is pending. And only Close ends the watcher
// goroutine, which leaves its loop only when it observes closed.
type Workbench struct {
	root     string
	notifier Notifier
	hooks    *Hooks
	// longForm resolves a path prefix to its long form, which on Windows is
	// GetLongPathNameW and elsewhere the path unchanged.
	longForm func(path string) (string, error)

	published atomic.Pointer[Snapshot]
	ready     chan struct{}
	readyOnce sync.Once

	// signal wakes the applier. It has capacity one and is never closed,
	// because the watcher, Current and Settle send on it without holding
	// w.mu and may do so after Close.
	signal chan struct{}
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
	settles    []*settleRequest
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

// Open arms the watcher on root and starts the first build in the
// background. It returns once the watcher is armed, before any file is read,
// so a change made after it returns is certain to be reported.
func Open(root string, opts Options) (*Workbench, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return nil, errors.New("resident: the root must be an absolute, clean path")
	}
	notifier := opts.Notifier
	if notifier == nil {
		platform, err := platformNotifier(root, opts.Hooks)
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
	w.poke()
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

// Ready is closed when the first snapshot is published, when the first build
// has failed, or when the first pass ends without publishing (a pass
// discarded because the watch went down while it ran). Ready says only that
// the first pass is over; Current, not Ready, says whether a snapshot is
// served.
func (w *Workbench) Ready() <-chan struct{} {
	return w.ready
}

// markReady closes Ready once.
func (w *Workbench) markReady() {
	w.readyOnce.Do(func() { close(w.ready) })
}

// Current answers the snapshot a read at now may use, or a Pick whose
// Snapshot is nil when the request must read the disk: when the workbench is
// not live, when the watched root no longer names the directory being
// watched, or when a card's claim has lapsed by now.
func (w *Workbench) Current(now time.Time) Pick {
	snapshot := w.published.Load()
	if snapshot == nil {
		w.mu.Lock()
		parkedNow := w.state == parked
		w.mu.Unlock()
		if parkedNow {
			// The applier retries a failed Arm on each signal, and a
			// request finding the watch down is what sends one.
			w.poke()
		}
		return Pick{}
	}
	if !w.notifier.Valid() {
		w.mu.Lock()
		if w.state != closed {
			w.failed = true
		}
		w.mu.Unlock()
		w.poke()
		return Pick{}
	}
	if !snapshot.earliestExpiry.IsZero() && !now.Before(snapshot.earliestExpiry) {
		return Pick{Lapsing: snapshot.lapsedAt(now)}
	}
	return Pick{Snapshot: snapshot}
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
// no publish can follow except one whose reads begin after the call, and it
// returns ctx's error when ctx ends first.
func (w *Workbench) Settle(ctx context.Context, dirs ...string) error {
	request := &settleRequest{done: make(chan struct{})}
	for _, dir := range dirs {
		if rel, ok := relativeTo(w.root, filepath.Clean(dir)); ok {
			request.dirs = append(request.dirs, rel)
		}
	}
	w.mu.Lock()
	if w.state == closed {
		w.mu.Unlock()
		return nil
	}
	w.settles = append(w.settles, request)
	w.mu.Unlock()
	w.poke()
	select {
	case <-request.done:
		return nil
	case <-ctx.Done():
		w.mu.Lock()
		for i, queued := range w.settles {
			if queued == request {
				w.settles = append(w.settles[:i:i], w.settles[i+1:]...)
				break
			}
		}
		w.mu.Unlock()
		return ctx.Err()
	}
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
	for _, request := range w.settles {
		request.release()
	}
	w.settles = nil
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

// watch is the watcher goroutine, the only caller of Notifier.Next.
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
		w.poke()
	}
}

// apply is the applier goroutine.
func (w *Workbench) apply() {
	defer w.goroutines.Done()
	defer w.markReady()
	for {
		select {
		case <-w.done:
			return
		case <-w.signal:
		}
		for w.step() {
		}
	}
}

// step runs one turn of the applier and reports whether another turn should
// follow at once, which it does when requests were queued while a pass ran.
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
		w.mu.Unlock()
		// A Close that begins after this unlock may call Notifier.Close
		// while this call runs; the contract makes a second Close safe.
		w.notifier.Close()
		w.mu.Lock()
		for w.inNext {
			w.cond.Wait()
		}
	}
	if w.state == closed {
		w.mu.Unlock()
		return false
	}
	if w.state == parked {
		// Arm is called holding w.mu, so no Arm can start after Close has
		// marked the workbench closed, and none can be left outstanding.
		if err := w.notifier.Arm(w.root); err != nil {
			// While parked no snapshot can be published until a re-arm
			// succeeds and a rebuild runs, and that rebuild begins after
			// this attempt, so every queued request is released now.
			for _, request := range w.settles {
				request.release()
			}
			w.settles = nil
			w.mu.Unlock()
			return false
		}
		w.state = watching
		w.pending = nil
		w.overflow = false
		w.failed = false
		w.rebuildDue = true
		w.cond.Broadcast()
	}
	if w.overflow {
		w.published.Store(nil)
		w.pending = nil
		w.overflow = false
		w.rebuildDue = true
	}
	rebuild := w.rebuildDue
	w.rebuildDue = false
	changes := w.pending
	w.pending = nil
	taken := w.settles
	w.settles = nil
	w.mu.Unlock()

	if !rebuild && len(changes) == 0 && len(taken) == 0 {
		return false
	}
	var snapshot *Snapshot
	var published Published
	if rebuild || w.base == nil {
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
		// Invariant 1: a pass that finds the watch down is discarded. Its
		// requests go back to the head of the queue for the pass that
		// follows the re-arm, or are released when the workbench closed,
		// because Close has already released the queue and no pass follows.
		if w.state == closed {
			for _, request := range taken {
				request.release()
			}
		} else {
			w.settles = append(taken, w.settles...)
		}
		w.mu.Unlock()
		w.markReady()
		return false
	}
	w.base = snapshot
	w.gen = snapshot.gen
	w.published.Store(snapshot)
	for _, request := range taken {
		request.release()
	}
	more := len(w.settles) > 0 || len(w.pending) > 0 || w.overflow || w.failed
	w.mu.Unlock()
	w.markReady()
	if w.hooks != nil && w.hooks.AfterPublish != nil {
		w.hooks.AfterPublish(published)
	}
	return more
}
