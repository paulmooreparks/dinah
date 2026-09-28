//go:build !nofixtureflush

package main

// fixtureFlushTags is empty in a test binary built without the
// nofixtureflush tag, whose child go test carries no tag either.
const fixtureFlushTags = ""
