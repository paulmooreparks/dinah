package perfstore_test

import (
	"os"
	"testing"

	"dinah/internal/testenv"
)

// TestMain moves the temporary directory out of the user's home where it sits
// there, and clears for the whole run the variables production code reads, so
// a developer's shell does not reach a generated store or the binary the
// budget test builds. The list is the one cmd/dinah's TestMain clears,
// because the budget test runs the same verbs and the same binary.
func TestMain(m *testing.M) {
	restoreTemp := testenv.IsolateTempDir()
	restoreIsolated := testenv.ClearVars(isolatedEnv...)
	code := m.Run()
	restoreIsolated()
	restoreTemp()
	os.Exit(code)
}

// isolatedEnv names the variables this package's tests clear. It is a twin of
// cmd/dinah's list rather than a reference to it, because cmd/dinah is package
// main and nothing can import it.
var isolatedEnv = []string{
	"DINAH_FORMAT",
	"COLUMNS", "DINAH_EDITOR", "VISUAL", "EDITOR",
	"LC_ALL", "LC_MESSAGES", "LANG",
	"DINAH_HARNESS", "DINAH_PROVIDER", "DINAH_MODEL", "DINAH_SERVER",
	"DINAH_COMPLETE_WORDS",
}

// TestIsolatedEnvNamesEveryVariableTheBinaryClears is the twin of the test of
// the same name in cmd/dinah, internal/bench and internal/mcp, over this
// package's own list.
//
// The first half compares isolatedEnv against a copy written out here, which
// catches a name dropped from isolatedEnv later and fails on any machine,
// including a CI runner exporting none of the thirteen. The second half
// asserts that TestMain actually made the call, by reading each name back
// while tests run, which only fails on a machine that exports one of them.
// Both halves are needed for the same reason the other three packages carry
// both: without the first, a dropped name is invisible on CI; without the
// second, a list nothing acts on passes.
func TestIsolatedEnvNamesEveryVariableTheBinaryClears(t *testing.T) {
	want := []string{
		"DINAH_FORMAT",
		"COLUMNS", "DINAH_EDITOR", "VISUAL", "EDITOR",
		"LC_ALL", "LC_MESSAGES", "LANG",
		"DINAH_HARNESS", "DINAH_PROVIDER", "DINAH_MODEL", "DINAH_SERVER",
		"DINAH_COMPLETE_WORDS",
	}
	for _, name := range want {
		if !namesVariable(isolatedEnv, name) {
			t.Errorf("isolatedEnv no longer names %s, so this binary inherits it from whoever runs the tests", name)
		}
	}
	for _, name := range isolatedEnv {
		if !namesVariable(want, name) {
			t.Errorf("isolatedEnv names %s, which this test does not expect: add it here with the reason, or take it out of the list", name)
		}
	}
	if len(isolatedEnv) != len(want) {
		t.Errorf("isolatedEnv carries %d names, wanted %d: %v", len(isolatedEnv), len(want), isolatedEnv)
	}

	for _, name := range want {
		if value, set := os.LookupEnv(name); set {
			t.Errorf("%s is still set to %q while tests run, so TestMain did not clear it", name, value)
		}
	}
}

// namesVariable reports whether a list of environment variable names carries
// one, which is what lets the guard above name the variable that went missing
// rather than print two slices and leave the reader to diff them. A twin of
// the same helper in cmd/dinah, internal/bench and internal/mcp, kept local
// for the same reason those stay separate from each other.
func namesVariable(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}
