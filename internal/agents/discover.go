// Package agents reaches the agents Dinah works with, Devin through
// `devin acp` and Codex through its app server, for the programs that run
// agent steps: finding their executables, opening sessions, sending prompts,
// choosing models and answering the agents' own requests, which are to read
// and write files and to ask the person's permission. The protocols are
// packages acp and codexagent; this package is what a host does with them.
package agents

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"studio/internal/layout"
)

// FindExecutable finds an agent's executable on the search path, and in its
// usual install folder on Windows. It never runs a candidate or searches the
// current folder, and it skips any candidate inside one of the workspaces,
// following links, so that a workspace cannot supply the agent that works
// in it.
//
// Devin is found by FindDevin, which also checks that what it finds is the
// Devin command line.
func FindExecutable(name string, workspaces []string) string {
	if name == "devin" {
		path, _ := FindDevin(workspaces)
		return path
	}
	if all := executablesIn(name, workspaces, searchPath()); len(all) > 0 {
		return all[0]
	}
	return ""
}

// searchPath is the search path, with the one the person's account has now
// added, since the app's own may predate an install.
func searchPath() string {
	path := os.Getenv("PATH")
	if current := installedPath(); current != "" {
		path += string(os.PathListSeparator) + current
	}
	return path
}

func findExecutableIn(name string, workspaces []string, searchPath string) string {
	if all := executablesIn(name, workspaces, searchPath); len(all) > 0 {
		return all[0]
	}
	return ""
}

// executablesIn lists every executable of the name on the search path, and in
// its usual install folder on Windows, in search order and each once, leaving
// out those inside a workspace.
func executablesIn(name string, workspaces []string, searchPath string) []string {
	var found []string
	seen := map[string]bool{}
	var candidates []string
	for _, dir := range filepath.SplitList(searchPath) {
		if !filepath.IsAbs(dir) {
			continue
		}
		for _, ext := range []string{"", ".exe", ".cmd"} {
			candidates = append(candidates, filepath.Join(dir, name+ext))
		}
	}
	if runtime.GOOS == "windows" {
		switch name {
		case "devin":
			candidates = append(candidates, filepath.Join(os.Getenv("LOCALAPPDATA"), "devin", "cli", "bin", "devin.exe"))
		case "ollama":
			candidates = append(candidates, filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Ollama", "ollama.exe"))
		case "claude":
			candidates = append(candidates, filepath.Join(os.Getenv("APPDATA"), "npm", "claude.cmd"))
		case "node":
			// Node.js's Windows installer puts it here as well as on the
			// search path; Claude's adapter runs on it.
			candidates = append(candidates, filepath.Join(os.Getenv("ProgramFiles"), "nodejs", "node.exe"))
		case "codex":
			candidates = append(candidates, filepath.Join(os.Getenv("APPDATA"), "npm", "codex.cmd"))
		}
	}
	for _, candidate := range candidates {
		if !filepath.IsAbs(candidate) {
			continue
		}
		if runtime.GOOS == "windows" && !strings.EqualFold(filepath.Ext(candidate), ".exe") && !strings.EqualFold(filepath.Ext(candidate), ".cmd") {
			continue
		}
		resolved, err := ResolvePath(candidate)
		if err != nil {
			continue
		}
		blocked := false
		for _, ws := range workspaces {
			if ws == "" {
				continue
			}
			if layout.Inside(ws, candidate) || layout.SamePath(ws, candidate) {
				blocked = true
				break
			}
			if real, err := ResolvePath(ws); err == nil && (layout.Inside(real, resolved) || layout.SamePath(real, resolved)) {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}
		if st, err := os.Stat(resolved); err == nil && !st.IsDir() {
			if runtime.GOOS != "windows" && st.Mode()&0o111 == 0 {
				continue
			}
			key := strings.ToLower(resolved)
			if runtime.GOOS != "windows" {
				key = resolved
			}
			if !seen[key] {
				seen[key] = true
				found = append(found, resolved)
			}
		}
	}
	return found
}

// CheckExecutable checks an executable the person chose for an agent and
// returns it with its links resolved. It never runs it.
func CheckExecutable(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", errors.New("choose an absolute executable path")
	}
	if runtime.GOOS == "windows" {
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".exe" && ext != ".cmd" && ext != ".bat" {
			return "", errors.New("choose an .exe, .cmd or .bat file")
		}
	}
	resolved, err := ResolvePath(path)
	if err != nil {
		return "", fmt.Errorf("cannot access the selected executable: %w", err)
	}
	info, err := os.Stat(resolved)
	if err == nil && info.IsDir() && strings.EqualFold(filepath.Ext(resolved), ".app") {
		return "", errors.New("the selected item is an application package; choose the agent's command line inside it, or install the command line and use automatic discovery")
	}
	if err != nil || !info.Mode().IsRegular() {
		return "", errors.New("the selected path is not an executable file")
	}
	if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
		return "", errors.New("the selected file is not executable")
	}
	return resolved, nil
}
