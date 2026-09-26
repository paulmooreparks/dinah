// Package resident holds one workbench in memory for dinah serve, so that a
// read is answered without opening a file. It serves reads only: every act
// reads and writes the disk, and nothing writes through a resident.
//
// This is dinah-619's measuring step: one snapshot, built once, and nothing
// that keeps it current yet. The watcher, the applier and Settle follow once
// the warm page's library reads have been measured against 10ms.
package resident

import (
	"errors"
	"path/filepath"
	"sync/atomic"
	"time"
)

// Options are what Open takes beside the root. The zero value is production.
type Options struct {
	// Notifier reports changes. At this step it is required and not yet
	// armed.
	Notifier Notifier
	// Hooks are test levers, nil in production.
	Hooks *Hooks
}

// ErrUnsupported is Open's answer where this platform or this volume has no
// watcher the design may use. The caller serves from disk.
var ErrUnsupported = errors.New("resident: no documented watcher for this workbench")

// Hooks are test levers, nil in production. Every member may be nil.
type Hooks struct {
	// BeforePublish runs after a snapshot is built and before it is
	// published.
	BeforePublish func(Published)
	// AfterPublish runs after a snapshot is published.
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

// Workbench is one workbench held in memory. It serves reads only; nothing
// writes through it.
type Workbench struct {
	root      string
	hooks     *Hooks
	published atomic.Pointer[Snapshot]
	ready     chan struct{}
}

// Open starts the one build in the background.
func Open(root string, opts Options) (*Workbench, error) {
	if opts.Notifier == nil {
		return nil, ErrUnsupported
	}
	if !filepath.IsAbs(root) || filepath.Clean(root) != root {
		return nil, errors.New("resident: the root must be an absolute, clean path")
	}
	w := &Workbench{root: root, hooks: opts.Hooks, ready: make(chan struct{})}
	go func() {
		defer close(w.ready)
		snapshot := finish(root, buildAll(root), 1, w.hooks)
		published := Published{Generation: 1, Rebuilt: true}
		if w.hooks != nil && w.hooks.BeforePublish != nil {
			w.hooks.BeforePublish(published)
		}
		w.published.Store(snapshot)
		if w.hooks != nil && w.hooks.AfterPublish != nil {
			w.hooks.AfterPublish(published)
		}
	}()
	return w, nil
}

// Ready is closed when the first pass is over.
func (w *Workbench) Ready() <-chan struct{} {
	return w.ready
}

// Current answers the snapshot a read at now may use, or a Pick whose
// Snapshot is nil when the request must read the disk.
func (w *Workbench) Current(now time.Time) Pick {
	snapshot := w.published.Load()
	if snapshot == nil {
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

// Close waits for the build to finish.
func (w *Workbench) Close() error {
	<-w.ready
	return nil
}

// Held answers how many files the current snapshot holds and how many bytes.
func (w *Workbench) Held() (int, int64) {
	snapshot := w.published.Load()
	if snapshot == nil {
		return 0, 0
	}
	return snapshot.Held()
}
