//go:build live && darwin && cgo

package main

// Helpers for clipboard_image_darwin_live_test.go, which cannot use cgo
// itself because cgo is not allowed in test files. The live tag keeps them
// out of every ordinary build.

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#import <AppKit/AppKit.h>

// dinahTestWritePasteboard replaces the general pasteboard's contents with
// a picture, as PNG, or as TIFF made from it by NSBitmapImageRep's
// TIFFRepresentation. clearContents and setData:forType: are NSPasteboard's
// documented way to write; setData:forType: returns NO on failure. The
// write runs on the main queue, as the read does.
static int dinahTestWritePasteboard(const void *bytes, size_t size, int asTIFF) {
	__block int ok = 0;
	dispatch_sync(dispatch_get_main_queue(), ^{
		@autoreleasepool {
			NSData *data = [NSData dataWithBytes:bytes length:size];
			NSPasteboardType type = NSPasteboardTypePNG;
			if (asTIFF) {
				NSBitmapImageRep *bitmap = [NSBitmapImageRep imageRepWithData:data];
				data = bitmap == nil ? nil : [bitmap TIFFRepresentation];
				type = NSPasteboardTypeTIFF;
			}
			if (data == nil) return;
			NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
			[pasteboard clearContents];
			ok = [pasteboard setData:data forType:type] ? 1 : 0;
		}
	});
	return ok;
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

// setPasteboardPicture puts png on the general pasteboard, as PNG, or as
// TIFF when asTIFF is true. It must not be called on the main thread.
func setPasteboardPicture(png []byte, asTIFF bool) error {
	tiff := C.int(0)
	if asTIFF {
		tiff = 1
	}
	if C.dinahTestWritePasteboard(unsafe.Pointer(&png[0]), C.size_t(len(png)), tiff) == 0 {
		return errors.New("the picture could not be written to the pasteboard")
	}
	return nil
}

// serveMainQueue gives the calling thread, which must be the main thread,
// to the main dispatch queue for good. Apple documents dispatch_main as
// parking the main thread to run the blocks submitted to the main queue,
// and as never returning.
func serveMainQueue() {
	C.dispatch_main()
}
