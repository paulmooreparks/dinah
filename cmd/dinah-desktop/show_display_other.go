//go:build !windows

package main

// On Linux and macOS a window is not yet placed on a chosen display by a
// documented route this program uses, so no other display is offered and a
// slide show stays in the main window (docs/design/detached-slide-show.md).
func systemDisplays() []showDisplay { return nil }

func moveShowWindow(class string, d showDisplay) bool { return false }

func focusShowWindow(class string) {}

func keepDisplayAwake() {}
