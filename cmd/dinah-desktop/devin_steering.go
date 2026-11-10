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

func registerDevinSteeringHooks(path string) error {
	devinSteeringMu.Lock()
	defer devinSteeringMu.Unlock()
	file := map[string]json.RawMessage{}
	original, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil {
		standard, err := hujson.Standardize(original)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(standard, &file); err != nil || file == nil {
			return errors.New("Devin hooks must contain a JSON object")
		}
	}
	for _, event := range []string{"UserPromptSubmit", "PostToolUse", "Stop"} {
		var groups []json.RawMessage
		if b := file[event]; len(b) > 0 {
			if err := json.Unmarshal(b, &groups); err != nil {
				return fmt.Errorf("read Devin %s hooks: %w", event, err)
			}
		}
		found := false
		for _, group := range groups {
			var entry struct {
				Matcher string `json:"matcher"`
				Hooks   []struct {
					Type    string `json:"type"`
					Command string `json:"command"`
				} `json:"hooks"`
			}
			if err := json.Unmarshal(group, &entry); err != nil {
				return err
			}
			for _, hook := range entry.Hooks {
				if entry.Matcher == "" && hook.Type == "command" && hook.Command == devinSteeringCommand {
					found = true
				}
			}
		}
		if !found {
			group, _ := json.Marshal(map[string]any{"matcher": "", "hooks": []any{map[string]any{"type": "command", "command": devinSteeringCommand, "timeout": 10}}})
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
