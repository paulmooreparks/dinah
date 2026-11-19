package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The install step tells the person which agent settings still run a
// program from the earlier folder, in any of the spellings JSON and TOML
// give a Windows path.
func TestSettingsUsingFindsTheEarlierFolderInEverySpelling(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", filepath.Join(home, "codex"))
	old := filepath.Join(home, ".example", "bin")
	os.MkdirAll(filepath.Join(home, ".claude"), 0o700)
	os.MkdirAll(filepath.Join(home, "codex"), 0o700)
	forward := strings.ReplaceAll(old, `\`, `/`)
	escaped := strings.ReplaceAll(old, `\`, `\`)
	os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte(`{"command": "`+forward+`/example.exe"}`), 0o600)
	os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"a": "`+escaped+`\example.exe", "b": "`+escaped+`\tools.exe", "c": "`+escaped+`x\other.exe"}`), 0o600)
	os.WriteFile(filepath.Join(home, "codex", "config.toml"), []byte("command = '"+strings.ToUpper(old)+`\example.exe'`), 0o600)
	got := strings.Join(settingsUsing(old), "\n")
	for _, want := range []string{"settings.json (1)", ".claude.json (2)", "config.toml (1)"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in\n%s", want, got)
		}
	}
	if found := settingsUsing(filepath.Join(home, "elsewhere")); len(found) != 0 {
		t.Errorf("another folder was found: %v", found)
	}
}
