//go:build live && linux && cgo

package main

import (
	"runtime"
	"testing"
)

// TestLiveClipboardPicture starts GTK on a thread of its own, as the window
// does on its main thread, puts a small picture on the clipboard and reads
// it back through readClipboardImage, as a chat box's menu Paste does. It
// needs a display (X11 or Wayland) and replaces the clipboard's contents.
func TestLiveClipboardPicture(t *testing.T) {
	requireLiveClipboardPicture(t)
	started := make(chan bool)
	go func() {
		runtime.LockOSThread()
		ok := startGTK()
		started <- ok
		if ok {
			runGTK()
		}
	}()
	if !<-started {
		t.Fatal("GTK could not open a display")
	}
	data, want := samplePNG(t)
	if err := setClipboardPicture(data); err != nil {
		t.Fatal(err)
	}
	got, err := NewApp().readClipboardImage()
	if err != nil {
		t.Fatal(err)
	}
	checkClipboardPicture(t, got, want)
}
