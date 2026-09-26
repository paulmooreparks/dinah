//go:build tui

package main

// init makes this build dinah-tui: runTUI runs the terminal head in this
// process rather than launching another, and main reads its arguments as
// those of dinah tui when no launcher started it.
func init() {
	tuiHead = runTUIHead
	tuiEntry = true
}
