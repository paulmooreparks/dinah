//go:build !windows

package main

// readClipboardImage finds no picture on macOS and Linux for now, so a chat
// box's menu Paste pastes the clipboard's text there. macOS (NSPasteboard)
// and Linux (GTK 3's gtk_clipboard_request_image) follow in their own
// branch, with this same method and result.
func (a *App) readClipboardImage() (ClipboardImage, error) {
	return ClipboardImage{}, nil
}
