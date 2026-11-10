package workspaces

import "sync"

// A source whose contents are kept somewhere else, on a site for one, can
// find that they changed while a window was showing them. A kind that keeps
// what it has read is the usual case: it gives a listing it already has at
// once, reads the site behind it, and learns that the listing is no longer
// true. The kind says so with SourceChanged, and the program that shows
// sources, which gave a function with OnSourceChanged when it started, has
// its windows list the source again (docs/design/source-kinds.md).
//
// A program that shows nothing, such as a command that runs once and ends,
// gives no function. A kind asks SourceChangesHeard before it gives an
// answer it would have to correct later, and gives only current answers when
// nobody would hear the correction.

var (
	changesMu     sync.RWMutex
	sourceChanged func(sourceID string)
)

// OnSourceChanged gives the function SourceChanged calls with a source's ID.
// The function must return at once, since a kind calls it from the work that
// read the source. A nil function takes away the one given before.
func OnSourceChanged(fn func(sourceID string)) {
	changesMu.Lock()
	defer changesMu.Unlock()
	sourceChanged = fn
}

// SourceChangesHeard reports whether a function was given, which is whether
// anything would act on SourceChanged.
func SourceChangesHeard() bool {
	changesMu.RLock()
	defer changesMu.RUnlock()
	return sourceChanged != nil
}

// SourceChanged says that what a source lists or reads may have changed by
// some means other than a write through its provider. It does nothing in a
// program that gave no function.
func SourceChanged(sourceID string) {
	changesMu.RLock()
	fn := sourceChanged
	changesMu.RUnlock()
	if fn != nil && sourceID != "" {
		fn(sourceID)
	}
}
