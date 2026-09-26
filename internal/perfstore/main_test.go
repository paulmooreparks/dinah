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
