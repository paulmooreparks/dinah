package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
)

// servingAct is one cli invocation the sweep below drives, and whether that
// invocation is one of the acts that serve the instruction chain.
type servingAct struct {
	// argv is the command, without the output form the sweep prepends.
	argv []string
	// serves says the act composes the chain, so its payload has to carry
	// every layer rather than a marker naming one it left out.
	serves bool
}

// TestTheCliHeadNeverWithholdsALayer asserts that the withholding rule reaches
// no terminal. A cli invocation is one process serving one act, so it carries
// no connection to remember anything on, its request carries no held set, and
// every layer is served in full.
//
// The sweep runs a whole cycle of acts and then runs it again in the next
// output form, and it does that four times, because a head that had started
// remembering what it served would show that on the second act of a pair rather
// than on the first. The cycle leaves the board where it found it, so each pass
// drives the same acts against the same positions. A pass naming --quiet is
// here because --quiet suppresses the human rendering and never the payload, so
// a marker reaching the payload under it would be invisible to the rendered
// passes alone.
func TestTheCliHeadNeverWithholdsALayer(t *testing.T) {
	root := newBench(t)
	writeGlobalLayer(t)

	for _, card := range []string{"A card", "A second card"} {
		if got := runCLI(t, root, "add", card); got.code != 0 {
			t.Fatalf("add %q: %d %s", card, got.code, got.errw)
		}
	}
	carryToDoing(t, root, "fx-1")
	carryToDoing(t, root, "fx-2")

	cycle := []servingAct{
		{argv: []string{"claim", "fx-1"}, serves: true},
		{argv: []string{"instructions", "fx-1"}, serves: true},
		{argv: []string{"instructions", "doing"}, serves: true},
		{argv: []string{"release", "fx-1"}},
		{argv: []string{"move", "fx-1", "done"}, serves: true},
		{argv: []string{"move", "fx-1", "doing"}, serves: true},
		{argv: []string{"claim", "fx-2"}, serves: true},
		{argv: []string{"instructions", "fx-2"}, serves: true},
		{argv: []string{"release", "fx-2"}},
	}
	forms := [][]string{
		{},
		{"--json"},
		{"--quiet"},
		{"--format", "compact"},
	}
	for _, form := range forms {
		for _, act := range cycle {
			argv := append(append([]string{}, form...), act.argv...)
			got := runCLI(t, root, argv...)
			if got.code != 0 {
				t.Fatalf("%v: %d %s", argv, got.code, got.errw)
			}
			printed := got.out + got.errw
			if strings.Contains(printed, "withheld") || strings.Contains(printed, "reread") {
				t.Errorf("%v printed a withholding marker:\n%s", argv, printed)
			}
			// The layers themselves are checked on the form that carries the
			// payload verbatim. The rendered form labels them in prose, the
			// compact form abbreviates them, and --quiet prints none of it, so
			// a text comparison against those three would assert the renderer
			// rather than the chain.
			if act.serves && len(form) == 1 && form[0] == "--json" {
				wantLayer(t, argv, printed, "Global layer.")
			}
		}
	}
}

// wantLayer fails unless a serving act's payload carried a layer's text.
func wantLayer(t *testing.T, argv []string, printed, text string) {
	t.Helper()
	if !strings.Contains(printed, text) {
		t.Errorf("%v served no layer carrying %q:\n%s", argv, text, printed)
	}
}

// writeGlobalLayer writes the user-global instruction layer under the home
// newBench pointed DINAH_HOME at, so the chain the cli serves has all three
// layers rather than two.
func writeGlobalLayer(t *testing.T) {
	t.Helper()
	dir := bench.UserBase(os.Getenv("DINAH_HOME"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("the user base: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, bench.InstructionsName), []byte("Global layer.\n"), 0o644); err != nil {
		t.Fatalf("the global layer: %v", err)
	}
}
