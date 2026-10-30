package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	if _, err := exec.LookPath(product.CommandProgram); err != nil {
		return fmt.Errorf("Devin steering requires the installed %s command on PATH: %w", product.CommandProgram, err)
	}
	return registerDevinSteeringHooks(filepath.Join(workspace, ".devin", "hooks.v1.json"))
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
