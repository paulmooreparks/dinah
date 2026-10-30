package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tailscale/hujson"
)

func TestAppletRegistrationPreservesConfigAndRepairsPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp_config.json")
	initial := []byte("{\n// Keep this note.\n\"other\":123,\n\"mcpServers\":{\n/* Keep this server. */ \"other\":{\"command\":\"another.exe\",\"env\":{\"TOKEN\":\"test-secret\"}},\n},\n}\n")
	if err := os.WriteFile(path, initial, 0o600); err != nil {
		t.Fatal(err)
	}
	entry := appletRegistrationEntry{Command: filepath.Join(dir, "old.exe"), Args: []string{"--applet-mcp", "--state-dir", dir}}
	if err := registerAppletMCPAt(path, entry); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(path)
	for _, retained := range []string{"// Keep this note.", "/* Keep this server. */", `"env":{"TOKEN":"test-secret"}`, `"other":123`} {
		if !bytes.Contains(first, []byte(retained)) {
			t.Fatalf("unrelated configuration changed: missing %q", retained)
		}
	}
	if err := registerAppletMCPAt(path, entry); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(path)
	if !bytes.Equal(first, second) {
		t.Fatal("matching registration rewrote the file")
	}
	entry.Command = filepath.Join(dir, "new.exe")
	if err := registerAppletMCPAt(path, entry); err != nil {
		t.Fatal(err)
	}
	updated, _ := os.ReadFile(path)
	standard, err := hujson.Standardize(updated)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Servers map[string]appletRegistrationEntry `json:"mcpServers"`
	}
	if err := json.Unmarshal(standard, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Servers["dinah-applets"].Command != entry.Command {
		t.Fatal("stale helper path was not repaired")
	}
}

// An entry a build from before the state folder moved wrote is Dinah's
// own, so it is replaced; any other folder is another program's.
func TestOwnStateDirIncludesTheFolderBeforeTheMove(t *testing.T) {
	home := filepath.Join(string(filepath.Separator)+"Users", "p")
	current, legacy := filepath.Join(home, ".dinah"), filepath.Join(home, "AppData", "Roaming", "dinah")
	if !ownStateDir(current, current, legacy) || !ownStateDir(legacy+string(filepath.Separator), current, legacy) {
		t.Fatal("Dinah's own state folders were not recognised")
	}
	// Windows compares paths without regard to case.
	if runtime.GOOS == "windows" && !ownStateDir(strings.ToLower(legacy), current, legacy) {
		t.Fatal("a state folder written in other case was not recognised")
	}
	elsewhere := filepath.Join(string(filepath.Separator)+"Elsewhere", "x")
	if ownStateDir(elsewhere, current, legacy) || ownStateDir(elsewhere, current, "") {
		t.Fatal("another program's folder was taken for Dinah's")
	}
}

func TestAppletRegistrationRefusesInvalidAndConflictingConfig(t *testing.T) {
	for _, input := range []string{
		"", "null", "[]", "{bad-json-secret}", `{"mcpServers":null}`, `{"mcpServers":[]}`,
		`{"mcpServers":{},"mcpServers":{}}`,
		`{"mcpServers":{"dinah-applets":{"command":"foreign.exe","args":[]}}}`,
	} {
		t.Run(input, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "mcp_config.json")
			if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
				t.Fatal(err)
			}
			err := registerAppletMCPAt(path, appletRegistrationEntry{Command: filepath.Join(dir, "dinah.exe"), Args: []string{"--applet-mcp", "--state-dir", dir}})
			if err == nil {
				t.Fatal("unsafe configuration accepted")
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatal("error exposed config contents")
			}
			after, _ := os.ReadFile(path)
			if string(after) != input {
				t.Fatal("rejected configuration was changed")
			}
		})
	}
}

func TestAppletRegistrationUsesOnlyThrowawayWorkspace(t *testing.T) {
	workspace := t.TempDir()
	oldHelper := appletHelperExecutable
	appletHelperExecutable = func() (string, error) { return filepath.Join(workspace, "helper.exe"), nil }
	t.Cleanup(func() { appletHelperExecutable = oldHelper })
	if configOverride == "" {
		t.Fatal("test configuration isolation is missing")
	}
	if err := ensureAppletMCPRegistration(workspace); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(workspace, ".devin", "mcp_config.local.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	standard, err := hujson.Standardize(data)
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Servers map[string]map[string]any `json:"mcpServers"`
	}
	if err := json.Unmarshal(standard, &cfg); err != nil {
		t.Fatal(err)
	}
	server := cfg.Servers["dinah-applets"]
	if len(server) != 2 || server["env"] != nil {
		t.Fatal("registered entry must contain command and args only")
	}
	if err := ensureAppletMCPRegistration(""); err == nil {
		t.Fatal("test registration accepted an unscoped workspace")
	}
}

func TestAppletRegistrationRejectsWorkspaceOverrides(t *testing.T) {
	for _, test := range []struct{ path, object string }{
		{".devin/mcp_config.json", "mcpServers"}, {".devin/config.json", "mcpServers"},
		{".devin/config.local.json", "mcpServers"}, {".cursor/mcp.json", "mcpServers"},
		{".mcp.json", "mcpServers"}, {".claude/settings.local.json", "mcpServers"},
		{"opencode.json", "mcp"}, {".vscode/mcp.json", "servers"}, {".zed/settings.json", "context_servers"},
	} {
		t.Run(test.path, func(t *testing.T) {
			workspace := t.TempDir()
			path := filepath.Join(workspace, filepath.FromSlash(test.path))
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			data := []byte(`{"` + test.object + `":{"dinah-applets":{"command":"foreign.exe","args":[]}}}`)
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
			err := ensureAppletMCPRegistration(workspace)
			if err == nil || !strings.Contains(err.Error(), "conflicts") {
				t.Fatalf("override accepted: %v", err)
			}
			if _, err := os.Stat(filepath.Join(workspace, ".devin", "mcp_config.local.json")); !os.IsNotExist(err) {
				t.Fatal("registration written before override rejection")
			}
			after, _ := os.ReadFile(path)
			if !bytes.Equal(data, after) {
				t.Fatal("workspace configuration was changed")
			}
		})
	}
}

// Another program's configuration that cannot be parsed, or that has
// duplicate properties, or that starts with a byte order mark, does not keep
// the registration from being made; a conflicting server behind a byte
// order mark is still found.
func TestAppletRegistrationPassesOverUnreadableSources(t *testing.T) {
	for _, test := range []struct {
		name, content string
		conflict      bool
	}{
		{"garbage", "{bad json", false},
		{"empty", "", false},
		{"duplicates", `{"mcpServers":{},"mcpServers":{"other":{"command":"x"}}}`, false},
		{"bom", "\xef\xbb\xbf{\"mcpServers\":{\"other\":{\"command\":\"x\"}}}", false},
		{"bom-conflict", "\xef\xbb\xbf{\"mcpServers\":{\"dinah-applets\":{\"command\":\"foreign.exe\",\"args\":[]}}}", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			workspace := t.TempDir()
			path := filepath.Join(workspace, ".cursor", "mcp.json")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}
			err := ensureAppletMCPRegistration(workspace)
			if test.conflict && (err == nil || !strings.Contains(err.Error(), "conflicts")) {
				t.Fatalf("conflict behind a byte order mark not found: %v", err)
			}
			if !test.conflict && err != nil {
				t.Fatalf("another program's file kept the registration from being made: %v", err)
			}
		})
	}
}

// Devin's own configuration written with a byte order mark is read and
// updated, and the mark is not kept.
func TestAppletRegistrationAcceptsByteOrderMark(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp_config.json")
	if err := os.WriteFile(path, []byte("\xef\xbb\xbf{\"mcpServers\":{}}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	entry := appletRegistrationEntry{Command: filepath.Join(dir, "dinah.exe"), Args: []string{"--applet-mcp", "--state-dir", dir}}
	if err := registerAppletMCPAt(path, entry); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if bytes.HasPrefix(after, []byte("\xef\xbb\xbf")) || !bytes.Contains(after, []byte("dinah-applets")) {
		t.Fatalf("configuration after registration: %q", after)
	}
}

func TestAppletRegistrationAllowsEquivalentWorkspaceOverride(t *testing.T) {
	workspace := t.TempDir()
	executable, err := appletHelperExecutable()
	if err != nil {
		t.Fatal(err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		t.Fatal(err)
	}
	entry := appletRegistrationEntry{Command: executable, Args: []string{"--applet-mcp", "--state-dir", configDir()}}
	data, _ := json.Marshal(map[string]any{"servers": map[string]any{"dinah-applets": entry}})
	path := filepath.Join(workspace, ".vscode", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ensureAppletMCPRegistration(workspace); err != nil {
		t.Fatal(err)
	}
}

// A Dinah run with a state folder other than the person's own, such as a trial copy,
// never writes Devin's user-wide settings, which belong to the person's own Dinah; it
// registers its helper in the workspace's local file. The person's own Dinah writes
// the user-wide file.
func TestATrialCopyNeverWritesDevinsUserWideSettings(t *testing.T) {
	// The system's temporary folder is a folder of this test's own, so that a real
	// project can be shown to be outside it.
	base := t.TempDir()
	tempArea, project := filepath.Join(base, "tmp"), filepath.Join(base, "project")
	home, workspace := filepath.Join(tempArea, "home"), filepath.Join(tempArea, "workspace")
	for _, dir := range []string{home, workspace, project} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("TMP", tempArea)
	t.Setenv("TEMP", tempArea)
	t.Setenv("TMPDIR", tempArea)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	t.Setenv("APPDATA", filepath.Join(home, "AppData", "Roaming"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	oldHelper, oldOverride, oldState := appletHelperExecutable, configOverride, applicationStateDir
	appletHelperExecutable = func() (string, error) { return filepath.Join(workspace, "helper.exe"), nil }
	t.Cleanup(func() { appletHelperExecutable, configOverride, applicationStateDir = oldHelper, oldOverride, oldState })
	configOverride = ""
	userWide := func() string {
		base, err := os.UserConfigDir()
		if err != nil {
			t.Fatal(err)
		}
		return filepath.Join(base, "devin", "mcp_config.json")
	}
	local := filepath.Join(workspace, ".devin", "mcp_config.local.json")

	applicationStateDir = filepath.Join(t.TempDir(), "trial-state")
	if err := ensureAppletMCPRegistration(workspace); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(local); err != nil {
		t.Fatalf("a trial copy did not register in the workspace's local file: %v", err)
	}
	if _, err := os.Stat(userWide()); !os.IsNotExist(err) {
		t.Fatalf("a trial copy wrote Devin's user-wide settings: %v", err)
	}
	if err := ensureAppletMCPRegistration(""); err == nil {
		t.Fatal("a trial copy registered with no workspace")
	}
	// A real project, outside its state folder and the temporary folder, is refused
	// and nothing is written in it; a workspace inside the state folder is not.
	err := ensureAppletMCPRegistration(project)
	if err == nil || !strings.Contains(err.Error(), "not the installed one") || !strings.Contains(err.Error(), project) {
		t.Fatalf("a trial copy registered in a real project: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(project, ".devin")); !os.IsNotExist(statErr) {
		t.Fatalf("a trial copy wrote in a real project: %v", statErr)
	}
	// A folder that only starts with the temporary folder's name is not inside it.
	sibling := tempArea + "2"
	if err := os.MkdirAll(sibling, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := ensureAppletMCPRegistration(sibling); err == nil {
		t.Fatal("a trial copy registered in a folder beside the temporary folder")
	}
	inState := filepath.Join(applicationStateDir, "workspaces", "one")
	if err := os.MkdirAll(inState, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := ensureAppletMCPRegistration(inState); err != nil {
		t.Fatalf("a workspace inside the state folder was refused: %v", err)
	}

	os.Remove(local)
	applicationStateDir = filepath.Join(home, ".dinah")
	if err := ensureAppletMCPRegistration(workspace); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(userWide()); err != nil {
		t.Fatalf("the person's own Dinah did not register in Devin's user-wide settings: %v", err)
	}
	if _, err := os.Stat(local); !os.IsNotExist(err) {
		t.Fatalf("the person's own Dinah wrote a workspace file: %v", err)
	}
}

// A registration in the way says which file it is in, what it runs, and that removing
// it lets Dinah register its own.
func TestARegistrationInTheWayIsReportedWithItsFileAndCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp_config.json")
	other := appletRegistrationEntry{Command: filepath.Join(dir, "trial", "dinah.exe"), Args: []string{"--applet-mcp", "--state-dir", filepath.Join(dir, "trial-state")}}
	data, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{"dinah-applets": other}})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	entry := appletRegistrationEntry{Command: filepath.Join(dir, "dinah.exe"), Args: []string{"--applet-mcp", "--state-dir", filepath.Join(dir, "state")}}
	for name, err := range map[string]error{
		"on registering": registerAppletMCPAt(path, entry),
		"on checking":    checkAppletRegistrationSource(path, entry),
	} {
		if err == nil {
			t.Fatalf("%s: a conflicting entry was accepted", name)
		}
		for _, want := range []string{path, other.Command, "trial-state", "Remove that entry"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("%s: the error does not say %q: %v", name, want, err)
			}
		}
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(data) {
		t.Fatal("the entry in the way was changed")
	}
}
