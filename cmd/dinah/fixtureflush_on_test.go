//go:build nofixtureflush

package main

// fixtureFlushTags is the build tag this test binary was built with that a
// child go test of this package must carry too, so that its fixtures skip
// their flushes as this binary's do.
const fixtureFlushTags = "nofixtureflush"
