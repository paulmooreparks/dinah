//go:build nofixtureflush

package durable

import (
	"path/filepath"
	"sync"
)

// This file is compiled only into a test binary built with the nofixtureflush
// tag, which the continuous integration workflow passes to go test. A
// production binary is built without it and takes flushskip_production.go in
// its place, where SkipFlushUnder does nothing and skipsFlush answers false,
// so nothing a production binary does can skip a flush.

// fixtureFlushSkipping reports that this build honours SkipFlushUnder.
const fixtureFlushSkipping = true

// flushScopes maps a directory, in pathKey form, to whether a durable
// operation on a path beneath it skips its flush. The nearest registered
// directory above a path decides.
var flushScopes = struct {
	sync.Mutex
	skip map[string]bool
}{skip: map[string]bool{}}

// SkipFlushUnder makes every durable operation on a path beneath dir skip the
// two calls that flush to stable storage: File.Sync, and MOVEFILE_WRITE_THROUGH
// on a Windows rename. Every step is still reported to Observe and every
// rename takes the same path. A test binary's setup calls it for the
// directory its fixtures are made in, and the answer undoes it.
func SkipFlushUnder(dir string) (restore func()) {
	return scopeFlush(dir, true)
}

// KeepFlushingUnder makes every durable operation on a path beneath dir flush
// as a production binary does, even beneath a directory SkipFlushUnder
// named. Every durability test calls it for its own workbench, and the answer
// undoes it.
func KeepFlushingUnder(dir string) (restore func()) {
	return scopeFlush(dir, false)
}

// scopeFlush registers dir with the given answer and returns what removes it.
func scopeFlush(dir string, skip bool) func() {
	key := pathKey(dir)
	flushScopes.Lock()
	flushScopes.skip[key] = skip
	flushScopes.Unlock()
	return func() {
		flushScopes.Lock()
		delete(flushScopes.skip, key)
		flushScopes.Unlock()
	}
}

// skipsFlush reports whether a durable operation on path skips its flush: the
// nearest registered directory above it says so, and no test is observing the
// steps, since a test that reads the steps is reading the durability they
// carry.
func skipsFlush(path string) bool {
	if Observe != nil {
		return false
	}
	target := pathKey(path)
	flushScopes.Lock()
	defer flushScopes.Unlock()
	for dir := target; ; {
		if skip, ok := flushScopes.skip[dir]; ok {
			return skip
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}
