package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/tailscale/hujson"
	"studio/internal/statefile"
)

var devinSteeringMu sync.Mutex

// devinSteeringCommand is the hook command, run by the profile's command
// program (product.CommandProgram), "dinah" for Dinah.
var devinSteeringCommand = product.CommandProgram + " steering-hook"

// Devin documents command hooks in .devin/hooks.v1.json. Preserve the
// person's existing hooks and add one owned command for each delivery event.
func ensureDevinSteeringHooks(workspace string) error {
	if _, err := commandProgramDir(); err != nil {
		return err
	}
	return registerDevinSteeringHooks(filepath.Join(workspace, ".devin", "hooks.v1.json"))
}

// devinSteeringExecutable is the window's own program. A test replaces it.
var devinSteeringExecutable = os.Executable

// commandProgramDir is the folder of the command program installed beside
// the window, which is the one the hook command should run. A Devin chat is
// started with that folder first on its PATH (devinEnvironment), so the hook
// is found when the person's PATH does not have the folder, and the window's
// own command is run when another of the same name is on PATH. A product
// installed in a folder that was not on PATH could not start a Devin chat
// before this (10 November 2026). A window with no command program beside
// it, such as a test build, needs the command on PATH, and the folder
// returned is then empty.
func commandProgramDir() (string, error) {
	name := product.CommandProgram
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if self, err := devinSteeringExecutable(); err == nil {
		dir := filepath.Dir(self)
		if info, err := os.Stat(filepath.Join(dir, name)); err == nil && info.Mode().IsRegular() {
			return dir, nil
		}
	}
	if _, err := exec.LookPath(product.CommandProgram); err != nil {
		return "", fmt.Errorf("Devin steering requires the %s command beside %s or on PATH: %w", product.CommandProgram, product.Name, err)
	}
	return "", nil
}

// devinEnvironment is what a Devin chat is started with: the window's own
// environment, the product's home folder, which the hook command opens the
// bus database from, and the command program's folder first on PATH.
func devinEnvironment() []string {
	env := append(os.Environ(), product.HomeVar()+"="+configDir())
	dir, err := commandProgramDir()
	if err != nil || dir == "" {
		return env
	}
	return withPathFirst(env, dir)
}

// withPathFirst is env with dir at the front of its PATH. Windows spells the
// variable Path and reads its name in any case.
func withPathFirst(env []string, dir string) []string {
	out := make([]string, 0, len(env)+1)
	found := false
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if ok && (key == "PATH" || runtime.GOOS == "windows" && strings.EqualFold(key, "PATH")) {
			found = true
			entry = key + "=" + dir + string(os.PathListSeparator) + value
		}
		out = append(out, entry)
	}
	if !found {
		out = append(out, "PATH="+dir)
	}
	return out
}

// earlierDevinSteeringCommands are the hook commands that earlier releases
// of a product that renamed its command program wrote
// (profile.EarlierCommandPrograms). An entry with one of them is the
// window's own, and is replaced by the current command rather than left
// beside it, since it would run a program that is no longer installed.
func earlierDevinSteeringCommands() []string {
	var out []string
	for _, name := range product.EarlierCommandPrograms {
		out = append(out, name+" steering-hook")
	}
	return out
}

// devinSteeringEvents are the events the steering hook is registered for.
var devinSteeringEvents = []string{"UserPromptSubmit", "PostToolUse", "Stop"}

func registerDevinSteeringHooks(path string) error {
	return registerDevinSteeringHooksFor(path, devinSteeringCommand, earlierDevinSteeringCommands())
}

// devinHooksRunAny reports whether the Devin hooks file at path has a hook,
// of any event, that runs one of commands. A file that is not there has
// none; a file that cannot be read is reported as an error.
func devinHooksRunAny(path string, commands []string) (bool, error) {
	devinSteeringMu.Lock()
	defer devinSteeringMu.Unlock()
	file, _, err := readDevinHooks(path)
	if err != nil {
		return false, err
	}
	for event, b := range file {
		var groups []json.RawMessage
		if json.Unmarshal(b, &groups) != nil {
			continue
		}
		for _, raw := range groups {
			g, err := readSteeringHookGroup(raw)
			if err != nil {
				return false, fmt.Errorf("read Devin %s hooks: %w", event, err)
			}
			for _, hook := range g.hooks {
				if slices.Contains(commands, hookCommand(hook)) {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

// readDevinHooks reads a Devin hooks file, which may have comments. A file
// that is not there is an empty object.
func readDevinHooks(path string) (map[string]json.RawMessage, []byte, error) {
	file := map[string]json.RawMessage{}
	original, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, err
	}
	if err == nil {
		standard, err := hujson.Standardize(original)
		if err != nil {
			return nil, nil, err
		}
		if err := json.Unmarshal(standard, &file); err != nil || file == nil {
			return nil, nil, errors.New("Devin hooks must contain a JSON object")
		}
	}
	return file, original, nil
}

// steeringHookGroup is one group of a Devin hooks event, read with every
// field kept, so that a group written back has what it had.
type steeringHookGroup struct {
	fields map[string]json.RawMessage
	hooks  []map[string]json.RawMessage
}

func readSteeringHookGroup(raw json.RawMessage) (steeringHookGroup, error) {
	g := steeringHookGroup{}
	if err := json.Unmarshal(raw, &g.fields); err != nil {
		return g, err
	}
	if b := g.fields["hooks"]; len(b) > 0 {
		if err := json.Unmarshal(b, &g.hooks); err != nil {
			return g, err
		}
	}
	return g, nil
}

// matcher is the group's matcher, "" when it has none.
func (g steeringHookGroup) matcher() string {
	var m string
	_ = json.Unmarshal(g.fields["matcher"], &m)
	return m
}

// hookCommand is the command of a hook of type command, or "".
func hookCommand(hook map[string]json.RawMessage) string {
	var kind, command string
	if json.Unmarshal(hook["type"], &kind) != nil || kind != "command" {
		return ""
	}
	_ = json.Unmarshal(hook["command"], &command)
	return command
}

// registerDevinSteeringHooksFor gives each steering event one entry that
// runs command, and replaces an entry that runs one of the earlier commands:
// the first such entry takes the current command, keeping its other fields,
// when the event has no entry with the current command, and every other one
// is removed. A group left with no hooks by a removal is removed. The
// person's own hooks, in any group, are kept as they are.
func registerDevinSteeringHooksFor(path, command string, earlier []string) error {
	devinSteeringMu.Lock()
	defer devinSteeringMu.Unlock()
	file, original, err := readDevinHooks(path)
	if err != nil {
		return err
	}
	isEarlier := func(c string) bool { return c != "" && c != command && slices.Contains(earlier, c) }
	for _, event := range devinSteeringEvents {
		var groups []json.RawMessage
		if b := file[event]; len(b) > 0 {
			if err := json.Unmarshal(b, &groups); err != nil {
				return fmt.Errorf("read Devin %s hooks: %w", event, err)
			}
		}
		read := make([]steeringHookGroup, len(groups))
		found := false
		for i, raw := range groups {
			g, err := readSteeringHookGroup(raw)
			if err != nil {
				return err
			}
			read[i] = g
			for _, hook := range g.hooks {
				if g.matcher() == "" && hookCommand(hook) == command {
					found = true
				}
			}
		}
		var kept []json.RawMessage
		for i, g := range read {
			if g.matcher() != "" {
				kept = append(kept, groups[i])
				continue
			}
			changed := false
			var hooks []map[string]json.RawMessage
			for _, hook := range g.hooks {
				if !isEarlier(hookCommand(hook)) {
					hooks = append(hooks, hook)
					continue
				}
				changed = true
				if !found {
					hook["command"], _ = json.Marshal(command)
					hooks = append(hooks, hook)
					found = true
				}
			}
			if !changed {
				kept = append(kept, groups[i])
				continue
			}
			if len(hooks) == 0 {
				continue
			}
			g.fields["hooks"], _ = json.Marshal(hooks)
			raw, _ := json.Marshal(g.fields)
			kept = append(kept, raw)
		}
		groups = kept
		if !found {
			group, _ := json.Marshal(map[string]any{"matcher": "", "hooks": []any{map[string]any{"type": "command", "command": command, "timeout": 10}}})
			groups = append(groups, group)
		}
		file[event], _ = json.Marshal(groups)
	}
	out, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	if bytes.Equal(original, out) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return statefile.Write(path, out)
}
