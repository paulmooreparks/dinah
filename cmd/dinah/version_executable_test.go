package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"dinah/internal/verb"
)

// TestTheVersionReportSaysWhereThisBinaryIs asserts that `--json version`
// carries an absolute `executable` naming a file that exists.
//
// The process this arm observes is the go test binary, not a shipped dinah:
// runCLI calls run() inside the test process and spawns nothing, so
// os.Executable here reports the test binary's own path. What this proves is
// that the field is populated, that it survives marshalling, and that it is
// absolute. What a shipped dinah reports is proven by
// editors/vscode/test/unit/versionExecutable-live.test.ts, which builds one
// and spawns it, and that file is the only route in the tree that can.
func TestTheVersionReportSaysWhereThisBinaryIs(t *testing.T) {
	root := newBench(t)
	asked := runCLI(t, root, "--json", "version")
	if asked.code != 0 {
		t.Fatalf("version --json: %d %s", asked.code, asked.errw)
	}
	var reported struct {
		Executable string `json:"executable"`
	}
	if err := json.Unmarshal([]byte(asked.out), &reported); err != nil {
		t.Fatalf("read the version report: %v\n%s", err, asked.out)
	}
	if reported.Executable == "" {
		t.Fatalf("the report carries no executable at all:\n%s", asked.out)
	}
	if !filepath.IsAbs(reported.Executable) {
		t.Errorf("the report says this binary is at %q, which is not an absolute path, and a client handing that to a third party has nothing it can run", reported.Executable)
	}
	if _, err := os.Stat(reported.Executable); err != nil {
		t.Errorf("the report says this binary is at %q and that path does not stat: %v", reported.Executable, err)
	}
}

// TestAVersionReportOmitsAnExecutableTheSystemWouldNotName asserts that the
// key is absent rather than empty when the seam fails, which is what
// `json:"executable,omitempty"` buys and what lets the extension tell an
// answer apart from a silence.
//
// The seam is swapped for the whole of this run, so the process observed is
// again the test process; nothing here spawns anything either.
func TestAVersionReportOmitsAnExecutableTheSystemWouldNotName(t *testing.T) {
	previous := verb.ExecutablePath
	verb.ExecutablePath = func() (string, error) {
		return "", errors.New("this system will not say where the binary is")
	}
	t.Cleanup(func() { verb.ExecutablePath = previous })

	root := newBench(t)
	asked := runCLI(t, root, "--json", "version")
	if asked.code != 0 {
		t.Fatalf("version --json: %d %s", asked.code, asked.errw)
	}
	var keyed map[string]any
	if err := json.Unmarshal([]byte(asked.out), &keyed); err != nil {
		t.Fatalf("read the version report: %v\n%s", err, asked.out)
	}
	if value, present := keyed["executable"]; present {
		t.Errorf("the report carries an executable key reading %#v where the system named no path, and a reader cannot tell that from an answer", value)
	}
}
