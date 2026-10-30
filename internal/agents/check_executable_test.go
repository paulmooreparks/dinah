//go:build !windows

package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// An application package chosen in place of the command line inside it is
// refused with a message that says so; the command line inside is taken.
func TestCheckExecutableExplainsAnApplicationPackage(t *testing.T) {
	app := filepath.Join(t.TempDir(), "Devin.app")
	bin := filepath.Join(app, "Contents", "MacOS")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := CheckExecutable(app); err == nil || !strings.Contains(err.Error(), "application package") {
		t.Fatalf("package = %v", err)
	}
	cli := filepath.Join(bin, "devin")
	if err := os.WriteFile(cli, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, err := CheckExecutable(cli); err != nil || filepath.Base(got) != "devin" {
		t.Fatalf("command line = %q, %v", got, err)
	}
}
