//go:build !windows

package main

// On Linux and macOS the window toolkit gives a window its identity and
// brings it forward (document_window.go asks it to show the window), and
// this program does nothing more through an interface of the system's own.
// What a desktop needs beyond that, a desktop entry on Linux and a bundle
// on macOS, is the installer's and is not built yet
// (docs/design/standalone-documents.md).

func setProcessIdentity(application string) {}

func allowForeground(pid int) {}

func focusOwnWindow(class string) {}

func setWindowIconFor(class string) {}
