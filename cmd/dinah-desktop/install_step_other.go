//go:build !windows

package main

// windowShortcuts makes no shortcut on this system yet: the stand-alone
// applications' desktop entries (document_register_other.go) are the
// pattern a window's entry would follow.
func windowShortcuts(exe, earlier string) ([]string, error) { return nil, nil }
