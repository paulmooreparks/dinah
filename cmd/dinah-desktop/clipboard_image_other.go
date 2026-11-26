//go:build !windows && !(darwin && cgo) && !(linux && cgo)

package main

// readClipboardImage finds no picture where Dinah has no clipboard picture
// reader: macOS and Linux built without cgo, and any other system. A chat
// box's menu Paste then pastes the clipboard's text. Windows reads the
// picture in clipboard_image_windows.go, macOS in clipboard_image_darwin.go
// and Linux in clipboard_image_linux.go, with this same method and result.
func (a *App) readClipboardImage() (ClipboardImage, error) {
	return ClipboardImage{}, nil
}
