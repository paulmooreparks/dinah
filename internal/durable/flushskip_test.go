package durable

import (
	"path/filepath"
	"testing"
)

// TestOnlyAFixtureBuildSkipsAFlush asserts that a path beneath a directory
// SkipFlushUnder named skips its flush in a binary built with the
// nofixtureflush tag and in no other, which is the build every production
// binary is; that a path beneath a directory KeepFlushingUnder named inside it
// flushes in every build; that a path beneath neither flushes; and that no path
// skips its flush while a test observes the steps.
func TestOnlyAFixtureBuildSkipsAFlush(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(SkipFlushUnder(dir))
	kept := filepath.Join(dir, "durability")
	t.Cleanup(KeepFlushingUnder(kept))
	cases := []struct {
		path string
		skip bool
	}{
		{filepath.Join(dir, "cards", "card.md"), fixtureFlushSkipping},
		{filepath.Join(kept, "cards", "card.md"), false},
		{filepath.Join(filepath.Dir(dir), "elsewhere", "card.md"), false},
	}
	for _, c := range cases {
		if got := skipsFlush(c.path); got != c.skip {
			t.Errorf("skipsFlush(%s) answered %v in a build whose fixture skipping is %v, wanted %v", c.path, got, fixtureFlushSkipping, c.skip)
		}
	}
	Observe = func(Step) error { return nil }
	t.Cleanup(func() { Observe = nil })
	if skipsFlush(cases[0].path) {
		t.Error("a path skipped its flush while a test observed the steps")
	}
}
