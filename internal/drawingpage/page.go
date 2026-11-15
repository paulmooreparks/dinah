// Package drawingpage owns a disposable native diagram page.
package drawingpage

import "errors"

// ErrUnavailable identifies a missing native capability before drawing starts.
var ErrUnavailable = errors.New("the disposable native drawing page is unavailable")

type Request struct {
	Kind       string            `json:"kind"`
	Text       string            `json:"text"`
	Scale      int               `json:"scale"`
	SourcePath string            `json:"sourcePath"`
	Images     map[string][]byte `json:"images"`
}

// DisposalError means the controller could not be discarded. The owner must
// disable further drawing instead of reusing capacity after this failure.
type DisposalError struct{ Err error }

func (e *DisposalError) Error() string { return "dispose native drawing page: " + e.Err.Error() }
func (e *DisposalError) Unwrap() error { return e.Err }

// StartupError keeps the native apartment alive until creation settles.
// The owner must refuse new drawing until Settled confirms disposal.
type StartupError struct{ Settled <-chan error }

func (e *StartupError) Error() string {
	return "Drawing is unavailable until the earlier page has finished starting, or until the window restarts. Nothing was uploaded."
}

type Picture struct {
	PNG           []byte
	Width, Height int
}
type Options struct {
	// DataFolder must be a fresh, private folder supplied by the host.
	DataFolder string
	// Event records native lifecycle transitions, never source text.
	Event func(string)
}
