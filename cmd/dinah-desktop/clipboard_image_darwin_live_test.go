//go:build live && darwin && cgo

package main

import (
	"os"
	"runtime"
	"testing"
)

// The read waits for the main dispatch queue, which only the main thread
// serves. A test binary's main thread runs TestMain and would never serve
// it, so when the test is turned on, the main goroutine is kept on the main
// thread (LockOSThread in an init function, as the runtime package
// documents) and given to the queue while the tests run on another
// goroutine, which ends the process when they finish.
func init() {
	if os.Getenv(liveClipboardPictureEnv) != "1" {
		return
	}
	runtime.LockOSThread()
	runTests = func(run func() int) int {
		go func() { os.Exit(run()) }()
		serveMainQueue()
		return 0
	}
}

// TestLiveClipboardPicture puts a small picture on the pasteboard, first as
// PNG and then as TIFF, and reads each back through readClipboardImage, as
// a chat box's menu Paste does. It replaces the pasteboard's contents.
func TestLiveClipboardPicture(t *testing.T) {
	requireLiveClipboardPicture(t)
	data, want := samplePNG(t)
	for _, c := range []struct {
		name   string
		asTIFF bool
	}{{"png", false}, {"tiff", true}} {
		t.Run(c.name, func(t *testing.T) {
			if err := setPasteboardPicture(data, c.asTIFF); err != nil {
				t.Fatal(err)
			}
			got, err := NewApp().readClipboardImage()
			if err != nil {
				t.Fatal(err)
			}
			checkClipboardPicture(t, got, want)
		})
	}
}
