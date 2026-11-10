package main

import (
	"bytes"
	"debug/buildinfo"
	"os"
	"path/filepath"
	"strings"
	"testing"

	buildpolicy "studio"
)

func writeProgram(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func readProgram(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s is missing: %v", path, err)
	}
	return string(b)
}

func TestInstallRefusesWrongCompilerBeforeReplacingEitherProgram(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	info, err := buildinfo.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(self)
	if err != nil {
		t.Fatal(err)
	}
	// Keep a real executable's metadata layout and substitute an older version
	// of the same length. The resulting fixture is inspected, never executed.
	old := "go1.20" + strings.Repeat(" ", len(info.GoVersion)-len("go1.20"))
	data = bytes.ReplaceAll(data, []byte(info.GoVersion), []byte(old))
	dir := t.TempDir()
	older := filepath.Join(dir, "older.exe")
	if err := os.WriteFile(older, data, 0o755); err != nil {
		t.Fatal(err)
	}
	required, err := buildpolicy.Toolchain()
	if err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{{older, self}, {self, older}} {
		cli, desktop := filepath.Join(dir, "cli"), filepath.Join(dir, "desktop")
		writeProgram(t, cli, "old CLI")
		writeProgram(t, desktop, "old desktop")
		err := installPrograms([]installProgram{{pair[0], cli}, {pair[1], desktop}}, required)
		if err == nil || !strings.Contains(err.Error(), "refusing to install") {
			t.Fatalf("wrong compiler was not refused: %v", err)
		}
		if readProgram(t, cli) != "old CLI" || readProgram(t, desktop) != "old desktop" {
			t.Fatal("a program was replaced before both compiler identities passed")
		}
	}
	if err := installPrograms([]installProgram{{self, filepath.Join(dir, "accepted")}}, required); err != nil {
		t.Fatalf("correct compiler refused: %v", err)
	}
}
