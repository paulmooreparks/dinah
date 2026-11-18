package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/profile"
	"studio/internal/programs"
)

// renamedProfile is a product that renamed its command line from "ex" to
// "example-cli" and gave "example-cli", its tool program's earlier name, to
// the command line; the tool program is "example-tools" now.
func renamedProfile() profile.Profile {
	return profile.Profile{
		ID: "example", CommandProgram: "example-cli", EarlierCommandPrograms: []string{"ex"}, WindowProgram: "example-desktop",
		ToolServers:     []profile.ToolServer{{Name: "example-cli", Program: "example-tools", Args: []string{"serve"}, EarlierPrograms: []string{"example-cli"}}},
		ProgramCommands: []profile.ProgramCommand{{Name: "serve", Program: "example-tools", Summary: "serve"}},
	}
}

// fakeProgram writes a file that stands for a program. A program that knows
// what it is has its identity as its content; an earlier build has text
// that is no identity, as an earlier build cannot answer.
func fakeProgram(t *testing.T, bin, name string, id *programs.Identity) string {
	t.Helper()
	body := "an earlier build of " + name
	if id != nil {
		body = id.Line()
	}
	path := filepath.Join(bin, programs.FileName(name))
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return body
}

func identityOf(program, version string) *programs.Identity {
	return &programs.Identity{Product: "example", Program: program, Version: version}
}

// readIdentity is the test's identify: it reads the fake program's content.
func readIdentity(path string) (programs.Identity, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return programs.Identity{}, err
	}
	return programs.ParseIdentity(string(b))
}

func mover(p profile.Profile, bin string, referenced bool) *programNameMover {
	return &programNameMover{profile: p, bin: bin, version: "2.0.5", identify: readIdentity, place: programs.Place, remove: os.Remove,
		referenced: func([]string) bool { return referenced }}
}

func content(t *testing.T, bin, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(bin, programs.FileName(name)))
	if err != nil {
		return ""
	}
	return string(b)
}

func setAside(t *testing.T, bin, name string) []string {
	t.Helper()
	matches, _ := filepath.Glob(filepath.Join(bin, programs.FileName(name)+".old-*"))
	var bodies []string
	for _, m := range matches {
		b, _ := os.ReadFile(m)
		bodies = append(bodies, string(b))
	}
	return bodies
}

// An earlier release's updater installed the new release under the earlier
// names: the command line as "ex" and the tool program as "example-cli".
// The window copies the tool program to its new name, places the command
// line under its own, which sets the tool program's file aside so that a
// running copy keeps it, and removes "ex", which nothing runs.
func TestAnOldLayoutBinIsMovedToTheCurrentNames(t *testing.T) {
	bin := t.TempDir()
	command := fakeProgram(t, bin, "ex", identityOf("example-cli", "2.0.5"))
	tools := fakeProgram(t, bin, "example-cli", identityOf("example-tools", "2.0.5"))
	notes := mover(renamedProfile(), bin, false).run()
	if got := content(t, bin, "example-tools"); got != tools {
		t.Fatalf("example-tools is %q, want the tool program (%v)", got, notes)
	}
	if got := content(t, bin, "example-cli"); got != command {
		t.Fatalf("example-cli is %q, want the command line (%v)", got, notes)
	}
	if aside := setAside(t, bin, "example-cli"); len(aside) != 1 || aside[0] != tools {
		t.Fatalf("the tool program's file was not set aside: %q", aside)
	}
	if exists(filepath.Join(bin, programs.FileName("ex"))) {
		t.Fatalf("ex was not removed (%v)", notes)
	}
	// A second start changes nothing.
	if again := mover(renamedProfile(), bin, false).run(); len(again) != 0 {
		t.Fatalf("a second start did %v", again)
	}
}

// The command line under its earlier name is placed under its current name
// only when it is the window's own release. One of another release is left
// where it is, and so is the tool program's file at that name.
func TestAMismatchedCommandLineIsNotPlaced(t *testing.T) {
	bin := t.TempDir()
	fakeProgram(t, bin, "ex", identityOf("example-cli", "2.0.4"))
	tools := fakeProgram(t, bin, "example-cli", identityOf("example-tools", "2.0.5"))
	notes := mover(renamedProfile(), bin, false).run()
	if got := content(t, bin, "example-cli"); got != tools {
		t.Fatalf("example-cli was replaced by a command line of another release (%v)", notes)
	}
	if !exists(filepath.Join(bin, programs.FileName("ex"))) {
		t.Fatal("ex was removed although the command line is not in place")
	}
	if len(notes) == 0 || !strings.Contains(strings.Join(notes, "\n"), "not the command line of release 2.0.5") {
		t.Fatalf("the refusal was not noted: %v", notes)
	}
}

// Programs built before they could say what they are are never moved, and
// a file that cannot say what it is is never set aside for the command line.
func TestEarlierBuildsAreNeverPlacedUnderCurrentNames(t *testing.T) {
	bin := t.TempDir()
	command := fakeProgram(t, bin, "ex", nil)
	tools := fakeProgram(t, bin, "example-cli", nil)
	mover(renamedProfile(), bin, false).run()
	if exists(filepath.Join(bin, programs.FileName("example-tools"))) || content(t, bin, "example-cli") != tools || content(t, bin, "ex") != command {
		t.Fatal("an earlier build was moved")
	}
	// A new command line beside a tool program that cannot say what it is
	// leaves that program where it is.
	fakeProgram(t, bin, "ex", identityOf("example-cli", "2.0.5"))
	notes := mover(renamedProfile(), bin, false).run()
	if content(t, bin, "example-cli") != tools || len(setAside(t, bin, "example-cli")) != 0 {
		t.Fatalf("a program that cannot say what it is was set aside (%v)", notes)
	}
}

// The command line's earlier file is kept while something still runs it,
// and is removed at a later start once nothing does.
func TestTheEarlierCommandLineIsRemovedOnlyWhenNothingRunsIt(t *testing.T) {
	bin := t.TempDir()
	fakeProgram(t, bin, "ex", identityOf("example-cli", "2.0.5"))
	fakeProgram(t, bin, "example-cli", identityOf("example-tools", "2.0.5"))
	var asked []string
	m := mover(renamedProfile(), bin, true)
	m.referenced = func(commands []string) bool { asked = commands; return true }
	m.run()
	if !exists(filepath.Join(bin, programs.FileName("ex"))) {
		t.Fatal("ex was removed while something runs it")
	}
	if len(asked) != 1 || asked[0] != "ex steering-hook" {
		t.Fatalf("the references asked about: %v", asked)
	}
	mover(renamedProfile(), bin, false).run()
	if exists(filepath.Join(bin, programs.FileName("ex"))) {
		t.Fatal("ex was kept when nothing runs it")
	}
	// A removal that fails is tried again at the next start.
	fakeProgram(t, bin, "ex", identityOf("example-cli", "2.0.5"))
	failing := mover(renamedProfile(), bin, false)
	failing.remove = func(string) error { return errors.New("in use") }
	if notes := failing.run(); len(notes) != 1 || !strings.Contains(notes[0], "tried again") {
		t.Fatalf("a failed removal: %v", notes)
	}
}

// A product that renamed nothing, such as Dinah, moves nothing, whatever
// files are in the folder.
func TestAProductThatRenamedNothingMovesNothing(t *testing.T) {
	bin := t.TempDir()
	p := renamedProfile()
	p.EarlierCommandPrograms = nil
	p.ToolServers[0].EarlierPrograms = nil
	if renamesPrograms(p) || renamesPrograms(profile.Source()) {
		t.Fatal("a product that renamed nothing is said to rename")
	}
	fakeProgram(t, bin, "ex", identityOf("example-cli", "2.0.5"))
	tools := fakeProgram(t, bin, "example-cli", identityOf("example-tools", "2.0.5"))
	if notes := mover(p, bin, false).run(); len(notes) != 0 || content(t, bin, "example-cli") != tools || !exists(filepath.Join(bin, programs.FileName("ex"))) {
		t.Fatalf("something moved: %v", notes)
	}
}

// The window's hook registration replaces an entry that runs the earlier
// command, keeps the person's own hooks, and adds no second entry.
func TestEarlierSteeringHooksAreReplacedNotDuplicated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.v1.json")
	initial := `{
  "UserPromptSubmit": [{"matcher": "", "hooks": [{"type": "command", "command": "ex steering-hook", "timeout": 10}]}],
  "PostToolUse": [
    {"matcher": "exec", "hooks": [{"type": "command", "command": "my-check"}]},
    {"matcher": "", "hooks": [{"type": "command", "command": "my-audit"}, {"type": "command", "command": "ex steering-hook", "timeout": 10}]}
  ],
  "Stop": [
    {"matcher": "", "hooks": [{"type": "command", "command": "example-cli steering-hook", "timeout": 10}]},
    {"matcher": "", "hooks": [{"type": "command", "command": "ex steering-hook", "timeout": 10}]}
  ],
  "SessionStart": [{"hooks": [{"type": "command", "command": "my-start"}]}]
}`
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := registerDevinSteeringHooksFor(path, "example-cli steering-hook", []string{"ex steering-hook"}); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	text := string(after)
	if strings.Contains(text, `"ex steering-hook"`) {
		t.Fatalf("an earlier entry is left: %s", text)
	}
	if got := strings.Count(text, "example-cli steering-hook"); got != 3 {
		t.Fatalf("the current command appears %d times, want once for each of 3 events: %s", got, text)
	}
	for _, kept := range []string{"my-check", "my-audit", "my-start"} {
		if !strings.Contains(text, kept) {
			t.Fatalf("the person's hook %s is gone: %s", kept, text)
		}
	}
	var file map[string][]struct {
		Hooks []map[string]any `json:"hooks"`
	}
	if err := json.Unmarshal(after, &file); err != nil {
		t.Fatal(err)
	}
	if len(file["Stop"]) != 1 {
		t.Fatalf("the Stop group left empty by the removal was kept: %s", text)
	}
	if hooks := file["PostToolUse"][1].Hooks; len(hooks) != 2 || hooks[0]["command"] != "my-audit" || hooks[1]["timeout"] != float64(10) {
		t.Fatalf("the replaced entry did not keep its place and fields: %v", hooks)
	}
	// Registering again changes nothing.
	if err := registerDevinSteeringHooksFor(path, "example-cli steering-hook", []string{"ex steering-hook"}); err != nil {
		t.Fatal(err)
	}
	if again, _ := os.ReadFile(path); string(again) != text {
		t.Fatal("a second registration changed the file")
	}
}

// An agent's entry for a tool server that runs the program's earlier name,
// from the program's folder or an earlier folder, with the server's
// arguments, was an installation's, and is brought to the program's current
// name. The same name in a folder of the person's own is theirs.
func TestAToolServerEntryWithAnEarlierProgramNameIsRewritten(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	server := toolServerEntry{Name: "example-cli", Entry: appletRegistrationEntry{Command: filepath.Join(bin, "example-tools.exe"), Args: []string{"serve"}},
		Earlier: []string{filepath.Join(dir, "old")}, EarlierFiles: []string{"example-cli.exe"}}
	for _, stale := range []string{filepath.Join(bin, "example-cli.exe"), filepath.Join(dir, "old", "example-cli.exe")} {
		path := filepath.Join(t.TempDir(), "mcp_config.json")
		initial := `{"mcpServers":{"example-cli":{"command":` + string(mustJSON(t, stale)) + `,"args":["serve"]}}}`
		if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
			t.Fatal(err)
		}
		notes, err := registerToolServersAt(path, []toolServerEntry{server})
		if err != nil || len(notes) != 1 || !strings.Contains(notes[0], "now runs") {
			t.Fatalf("%s: %v %v", stale, notes, err)
		}
		if got, _ := entryOf(t, path, "example-cli"); got.Command != server.Entry.Command {
			t.Fatalf("%s: the entry runs %s", stale, got.Command)
		}
	}
	own := filepath.Join(dir, "custom-build", "example-cli.exe")
	path := filepath.Join(t.TempDir(), "mcp_config.json")
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"example-cli":{"command":`+string(mustJSON(t, own))+`,"args":["serve"]}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := registerToolServersAt(path, []toolServerEntry{server}); err != nil {
		t.Fatal(err)
	}
	if got, _ := entryOf(t, path, "example-cli"); got.Command != own {
		t.Fatalf("a person's own entry was changed to %s", got.Command)
	}
	// Without the earlier name the stale entry is a person's.
	plain := server
	plain.EarlierFiles = nil
	stale := filepath.Join(bin, "example-cli.exe")
	if err := os.WriteFile(path, []byte(`{"mcpServers":{"example-cli":{"command":`+string(mustJSON(t, stale))+`,"args":["serve"]}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := registerToolServersAt(path, []toolServerEntry{plain}); err != nil {
		t.Fatal(err)
	}
	if got, _ := entryOf(t, path, "example-cli"); got.Command != stale {
		t.Fatalf("with no earlier names the entry was changed to %s", got.Command)
	}
}

// With no earlier names the registration behaves as it always did: an
// entry that runs another command, however it looks, is the person's.
func TestWithNoEarlierNamesSteeringHooksAreUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.v1.json")
	initial := `{"Stop":[{"matcher":"","hooks":[{"type":"command","command":"ex steering-hook","timeout":10}]}]}`
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := registerDevinSteeringHooksFor(path, "example-cli steering-hook", nil); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	if !strings.Contains(string(after), "ex steering-hook") || strings.Count(string(after), "example-cli steering-hook") != 3 {
		t.Fatalf("hooks with no earlier names: %s", after)
	}
	if len(earlierDevinSteeringCommands()) != len(product.EarlierCommandPrograms) {
		t.Fatal("the earlier commands do not follow the profile")
	}
}

// The check before removing the earlier command line brings each known
// folder's hooks to the current command, and reports a reference that is
// left: a hook a person added with a matcher of their own, or a file that
// cannot be read.
func TestEarlierSteeringReferencesAreFoundAfterMigration(t *testing.T) {
	if len(product.EarlierCommandPrograms) != 0 {
		t.Skip("this test runs on a build whose profile renamed nothing")
	}
	write := func(folder, text string) string {
		path := filepath.Join(folder, ".devin", "hooks.v1.json")
		os.MkdirAll(filepath.Dir(path), 0o700)
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	clean, missing := t.TempDir(), t.TempDir()
	write(clean, `{"Stop":[{"matcher":"","hooks":[{"type":"command","command":"other steering-hook"}]}]}`)
	if earlierSteeringReferenced([]string{clean, missing}, []string{"ex steering-hook"}) {
		t.Fatal("folders with no earlier entry were said to run it")
	}
	person := t.TempDir()
	write(person, `{"PostToolUse":[{"matcher":"exec","hooks":[{"type":"command","command":"ex steering-hook"}]}]}`)
	if !earlierSteeringReferenced([]string{person}, []string{"ex steering-hook"}) {
		t.Fatal("a person's own hook that runs the earlier command was not found")
	}
	broken := t.TempDir()
	write(broken, `{"Stop":42`)
	if !earlierSteeringReferenced([]string{broken}, []string{"ex steering-hook"}) {
		t.Fatal("a file that cannot be read was taken as clean")
	}
}
