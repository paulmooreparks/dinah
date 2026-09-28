//go:build !nofixtureflush

package durable

// This is the file every production binary is built with. A test binary built
// with the nofixtureflush tag takes flushskip_fixtures.go in its place.

// fixtureFlushSkipping reports that this build ignores SkipFlushUnder.
const fixtureFlushSkipping = false

// SkipFlushUnder does nothing in this build, so every durable operation
// flushes. A test binary's setup calls it for the directory its fixtures are
// made in, and only a binary built with the nofixtureflush tag honours it.
func SkipFlushUnder(string) (restore func()) {
	return func() {}
}

// KeepFlushingUnder does nothing in this build, where every durable operation
// flushes already.
func KeepFlushingUnder(string) (restore func()) {
	return func() {}
}

// skipsFlush answers false: no operation skips its flush in this build.
func skipsFlush(string) bool {
	return false
}
