package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The Terminal's POSIX shell is built from tools/shwasm into
// frontend/vendor/gosh and committed. The build records the hash of the
// source it was built from, which must be the source's hash now; run
// tools/shwasm/build.ps1 after changing the source.
func TestShellBuildIsCurrent(t *testing.T) {
	dir := filepath.Join("..", "..", "tools", "shwasm")
	var text strings.Builder
	for _, name := range []string{"go.mod", "go.sum", "main.go"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		text.WriteString(name + "\n" + strings.ReplaceAll(string(data), "\r\n", "\n") + "\n")
	}
	sum := sha256.Sum256([]byte(text.String()))
	recorded, err := os.ReadFile(filepath.Join("frontend", "vendor", "gosh", "SOURCE.sha256"))
	if err != nil {
		t.Fatal(err)
	}
	if want := hex.EncodeToString(sum[:]); strings.TrimSpace(string(recorded)) != want {
		t.Fatalf("the shell's build is older than its source (built from %s, source is %s); run tools/shwasm/build.ps1", strings.TrimSpace(string(recorded)), want)
	}
	for _, name := range []string{"sh.wasm", "wasm_exec.js", "LICENSE-mvdan-sh", "LICENSE-go"} {
		if _, err := os.Stat(filepath.Join("frontend", "vendor", "gosh", name)); err != nil {
			t.Errorf("the shell's build has no %s", name)
		}
	}
}
