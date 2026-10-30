//go:build !windows

package main

import wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

// On macOS and Linux, Wails reads the clipboard's plain text; the Editor's
// Paste command then pastes that text, read as Markdown on the page.
func (a *App) readClipboard() (ClipboardContent, error) {
	text, err := wruntime.ClipboardGetText(a.ctx)
	return ClipboardContent{Text: text}, err
}
