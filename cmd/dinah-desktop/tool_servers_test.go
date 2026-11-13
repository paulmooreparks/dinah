package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tailscale/hujson"
)

// entryOf reads one server's entry from a Devin MCP configuration.
func entryOf(t *testing.T, path, name string) (appletRegistrationEntry, bool) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	standard, err := hujson.Standardize(data)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Servers map[string]appletRegistrationEntry `json:"mcpServers"`
	}
	if err := json.Unmarshal(standard, &doc); err != nil {
		t.Fatal(err)
	}
	e, ok := doc.Servers[name]
	return e, ok
}

// A new installation has no entry, and gets one. An entry that is already
// right is left untouched, byte for byte.
func TestAToolServerIsRegisteredOnceWithNoSetup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings", "mcp_config.json")
	server := toolServerEntry{Name: "example-tools", Entry: appletRegistrationEntry{Command: filepath.Join(dir, "bin", "example-tools.exe"), Args: []string{"serve"}}}
	notes, err := registerToolServersAt(path, []toolServerEntry{server})
	if err != nil || len(notes) != 1 {
		t.Fatalf("a first registration: %v %v", notes, err)
	}
	if got, ok := entryOf(t, path, "example-tools"); !ok || got.Command != server.Entry.Command || strings.Join(got.Args, " ") != "serve" {
		t.Fatalf("the entry: %+v %v", got, ok)
	}
	first, _ := os.ReadFile(path)
	notes, err = registerToolServersAt(path, []toolServerEntry{server})
	second, _ := os.ReadFile(path)
	if err != nil || len(notes) != 0 || !bytes.Equal(first, second) {
		t.Fatalf("a second registration changed something: %v %v", notes, err)
	}
}

// An entry from an earlier installation, which runs the same program from
// another folder, is brought to where the program is now, and everything
// else in the file is kept.
func TestAnEarlierInstallationsToolServerIsMovedToTheProgramsPlace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp_config.json")
	old := filepath.Join(dir, "old", "example-tools.exe")
	initial := "{\n// Keep this note.\n\"mcpServers\":{\n/* Keep this server. */ \"other\":{\"command\":\"another.exe\",\"env\":{\"TOKEN\":\"test-secret\"}},\n\"example-tools\":{\"command\":" + string(mustJSON(t, old)) + ",\"args\":[\"serve\"]},\n},\n}\n"
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	server := toolServerEntry{Name: "example-tools", Entry: appletRegistrationEntry{Command: filepath.Join(dir, "bin", "example-tools.exe"), Args: []string{"serve"}}}
	notes, err := registerToolServersAt(path, []toolServerEntry{server})
	if err != nil || len(notes) != 1 || !strings.Contains(notes[0], "now runs") {
		t.Fatalf("the move: %v %v", notes, err)
	}
	if got, _ := entryOf(t, path, "example-tools"); got.Command != server.Entry.Command {
		t.Fatalf("the entry still runs %s", got.Command)
	}
	after, _ := os.ReadFile(path)
	for _, kept := range []string{"// Keep this note.", "/* Keep this server. */", `"env":{"TOKEN":"test-secret"}`, "another.exe"} {
		if !bytes.Contains(after, []byte(kept)) {
			t.Fatalf("the rest of the file changed: %q is gone", kept)
		}
	}
}

// An entry a person wrote or changed is theirs: one with other arguments,
// with an environment of its own, or that runs another program. It is left
// as it is, and the registration says so.
func TestAPersonsOwnToolServerEntryIsLeftAlone(t *testing.T) {
	dir := t.TempDir()
	server := toolServerEntry{Name: "example-tools", Entry: appletRegistrationEntry{Command: filepath.Join(dir, "bin", "example-tools.exe"), Args: []string{"serve"}}}
	old := string(mustJSON(t, filepath.Join(dir, "old", "example-tools.exe")))
	for name, entry := range map[string]string{
		"other arguments":     `{"command":` + old + `,"args":["serve","--verbose"]}`,
		"an environment":      `{"command":` + old + `,"args":["serve"],"env":{"SPACE":"TEST"}}`,
		"another program":     `{"command":"C:/tools/wrapper.cmd","args":["serve"]}`,
		"not a server":        `"a string"`,
		"no arguments at all": `{"command":` + old + `}`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "mcp_config.json")
			initial := []byte(`{"mcpServers":{"example-tools":` + entry + `}}`)
			if err := os.WriteFile(path, initial, 0o600); err != nil {
				t.Fatal(err)
			}
			notes, err := registerToolServersAt(path, []toolServerEntry{server})
			after, _ := os.ReadFile(path)
			if err != nil || !bytes.Equal(initial, after) {
				t.Fatalf("the file was changed (%v): %s", err, after)
			}
			if len(notes) != 1 || !strings.Contains(notes[0], "left as it is") {
				t.Fatalf("what was said: %v", notes)
			}
		})
	}
}

// A file that cannot be read as a configuration is not written.
func TestAToolServerIsNotRegisteredInAFileThatIsNoConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp_config.json")
	initial := []byte("this is not JSON")
	if err := os.WriteFile(path, initial, 0o600); err != nil {
		t.Fatal(err)
	}
	server := toolServerEntry{Name: "example-tools", Entry: appletRegistrationEntry{Command: "C:/bin/example-tools.exe", Args: []string{"serve"}}}
	if _, err := registerToolServersAt(path, []toolServerEntry{server}); err == nil {
		t.Fatal("a file that is no configuration was accepted")
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(initial, after) {
		t.Fatalf("the file was changed: %s", after)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
