package resident

import "errors"

// Notifier reports the changes below one root. The resident calls Next from
// its watcher goroutine alone, and Arm and Close from its applier or from
// Workbench.Close, never while Next is running except as stated on Close.
type Notifier interface {
	// Arm starts watching root. It returns once a change made after it
	// returns is certain to be reported, either as a Change or as an
	// Overflow. Arm may be called again after Close, and the notifier then
	// watches afresh.
	Arm(root string) error
	// Next blocks until changes are available and answers them. It answers
	// ErrClosed after Close, at once when Close has already run, and any
	// other error when the watch has failed.
	Next() (Batch, error)
	// Valid reports whether root still names the directory being watched.
	Valid() bool
	// Close stops watching and releases everything Arm acquired. It may be
	// called while Next is running, and that Next then answers ErrClosed or
	// the batch it had already received; Close returns only after that Next
	// has returned. Close on a notifier that is not armed answers nil.
	// Close may be called while another Close is running; the second
	// returns once the first has released everything, and answers nil.
	Close() error
}

// Batch is one delivery: changes, or the news that changes were lost.
type Batch struct {
	Changes  []Change
	Overflow bool
}

// Change is one reported change: a path relative to the root, as reported,
// with the platform's separators, and what was reported of it.
type Change struct {
	Path   string
	Action Action
}

// Action is what a change reports, which the shallow row of the reconcile
// reads and nothing else does.
type Action int

// The five actions a change reports.
const (
	Added Action = iota + 1
	Removed
	Modified
	RenamedOld
	RenamedNew
)

// ErrClosed is what Next answers after Close.
var ErrClosed = errors.New("resident: notifier closed")
