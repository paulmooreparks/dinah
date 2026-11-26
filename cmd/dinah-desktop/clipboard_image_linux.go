//go:build linux && cgo

package main

/*
#cgo pkg-config: gtk+-3.0
#include <stdint.h>
#include <glib.h>

// In clipboard_image_linux.c. This file exports a Go function, so cgo
// allows only declarations here.
void dinahRequestClipboardImage(uintptr_t handle);
*/
import "C"

import (
	"errors"
	"math"
	"runtime/cgo"
	"time"
	"unsafe"
)

// clipboardImageTimeout is how long Paste waits for GTK to give the
// clipboard's picture. The program that owns the clipboard sends it, and a
// program that has hung would otherwise keep Paste waiting forever.
const clipboardImageTimeout = 5 * time.Second

// clipboardImageResult is what the GTK callback sends back: the PNG bytes,
// nil for no picture, or an error.
type clipboardImageResult struct {
	data []byte
	err  error
}

// readClipboardImage asks GTK 3 for the clipboard's picture as a GdkPixbuf
// and has it saved as PNG (clipboard_image_linux.c). GTK answers on its
// main loop, which the window runs, and the answer comes back through a
// channel found by a cgo.Handle. No picture is an empty result, not an
// error.
//
// When GTK does not answer in time, the handle is left for the late
// callback to delete; the channel has room for that one answer, so the
// callback never blocks the main loop.
func (a *App) readClipboardImage() (ClipboardImage, error) {
	done := make(chan clipboardImageResult, 1)
	C.dinahRequestClipboardImage(C.uintptr_t(cgo.NewHandle(done)))
	select {
	case r := <-done:
		if r.err != nil {
			return ClipboardImage{}, r.err
		}
		if r.data == nil {
			return ClipboardImage{}, nil
		}
		return ClipboardImage{Type: "image/png", Data: r.data}, nil
	case <-time.After(clipboardImageTimeout):
		return ClipboardImage{}, errors.New("the clipboard did not give its picture within 5 seconds")
	}
}

// dinahClipboardImageDone receives GTK's answer on the main loop: PNG bytes
// that the C side frees after this returns, no bytes for no picture, or the
// message of the error that stopped the conversion to PNG.
//
//export dinahClipboardImageDone
func dinahClipboardImageDone(handle C.uintptr_t, data *C.guchar, size C.gsize, message *C.char) {
	h := cgo.Handle(handle)
	done := h.Value().(chan clipboardImageResult)
	h.Delete()
	var r clipboardImageResult
	switch {
	case message != nil:
		r.err = errors.New("the picture on the clipboard could not be converted to PNG: " + C.GoString(message))
	case data != nil && size > math.MaxInt32:
		r.err = errors.New("the picture on the clipboard is too large to paste")
	case data != nil:
		r.data = C.GoBytes(unsafe.Pointer(data), C.int(size))
	}
	done <- r
}
