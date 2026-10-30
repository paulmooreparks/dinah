//go:build darwin

package agents

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// An app opened from the Finder is started by launchd with a short system
// search path, not the one the person's shell builds from its startup
// files, which is where Devin's install script and Homebrew put their
// commands. So the search path is asked of the person's own login shell,
// started interactively so that it reads the same files a Terminal window
// does (zsh and bash document which files a login and an interactive shell
// read). It is asked once, and an answer that does not come within a few
// seconds is left out.

var shellPath struct {
	once  sync.Once
	value string
}

const pathMarker = "__DINAH_PATH__"

func installedPath() string {
	shellPath.once.Do(func() { shellPath.value = loginShellPath() })
	return shellPath.value
}

func loginShellPath() string {
	shell := os.Getenv("SHELL")
	if !filepath.IsAbs(shell) {
		shell = "/bin/zsh"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, shell, "-l", "-i", "-c", `printf '%s%s%s' '`+pathMarker+`' "$PATH" '`+pathMarker+`'`)
	cmd.Stdin = nil
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	// Startup files may print their own lines; the path is between the
	// markers.
	_, rest, ok := strings.Cut(string(out), pathMarker)
	if !ok {
		return ""
	}
	path, _, ok := strings.Cut(rest, pathMarker)
	if !ok {
		return ""
	}
	return strings.TrimSpace(path)
}
