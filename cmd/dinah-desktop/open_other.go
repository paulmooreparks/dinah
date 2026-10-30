//go:build !windows

package main

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

// shellOpen opens a file or folder with the system's handler: open on macOS,
// xdg-open on Linux.
func shellOpen(path string) error {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", path).Start()
	}
	return exec.Command("xdg-open", path).Start()
}

// showInFolder shows the file's folder; macOS selects the file in Finder.
func showInFolder(path string) error {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", "-R", path).Start()
	}
	return exec.Command("xdg-open", filepath.Dir(path)).Start()
}
