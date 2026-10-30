//go:build windows

package main

import (
	"os/exec"

	"golang.org/x/sys/windows"
)

// shellOpen opens a file or folder as a double-click in Explorer would,
// through ShellExecute with the "open" verb
// (https://learn.microsoft.com/windows/win32/api/shellapi/nf-shellapi-shellexecutew).
func shellOpen(path string) error {
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL)
}

// showInFolder opens Explorer with the file selected, through Explorer's
// documented /select switch.
func showInFolder(path string) error {
	return exec.Command("explorer.exe", "/select,", path).Start()
}
