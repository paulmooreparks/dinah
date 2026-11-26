//go:build darwin && cgo

package main

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework AppKit
#include <stdlib.h>
#include <string.h>
#import <AppKit/AppKit.h>

// The results of dinahReadPasteboard.
enum { dinahNoPicture = 0, dinahPicture = 1, dinahPictureUnreadable = 2 };

// dinahReadPasteboardHere reads the general pasteboard's picture as PNG on
// the calling thread. On dinahPicture, *bytes is memory from malloc that the
// caller frees, and *size is its length.
//
// The calls, as Apple documents them in AppKit:
//   - [NSPasteboard generalPasteboard] is the pasteboard of Copy and Paste.
//   - availableTypeFromArray: returns the first type in the array that the
//     pasteboard holds, so PNG is preferred to TIFF, or nil for neither.
//   - dataForType: returns the data for the type, or nil.
//   - NSPasteboardTypePNG and NSPasteboardTypeTIFF are AppKit's PNG and
//     TIFF image types.
//   - [NSBitmapImageRep imageRepWithData:] makes a bitmap from the first
//     image in TIFF data, or returns nil when it cannot.
//   - representationUsingType:NSBitmapImageFileTypePNG properties: returns
//     the bitmap as PNG data.
//
// @autoreleasepool releases the autoreleased objects these calls return
// before the function returns ("Using Autorelease Pool Blocks", Apple's
// Advanced Memory Management Programming Guide).
static int dinahReadPasteboardHere(void **bytes, size_t *size) {
	@autoreleasepool {
		NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
		NSString *type = [pasteboard availableTypeFromArray:@[NSPasteboardTypePNG, NSPasteboardTypeTIFF]];
		if (type == nil) return dinahNoPicture;
		NSData *data = [pasteboard dataForType:type];
		if (data != nil && [type isEqualToString:NSPasteboardTypeTIFF]) {
			NSBitmapImageRep *bitmap = [NSBitmapImageRep imageRepWithData:data];
			data = bitmap == nil ? nil : [bitmap representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
		}
		if (data == nil || [data length] == 0) return dinahPictureUnreadable;
		void *copy = malloc([data length]);
		if (copy == NULL) return dinahPictureUnreadable;
		memcpy(copy, [data bytes], [data length]);
		*bytes = copy;
		*size = [data length];
		return dinahPicture;
	}
}

// dinahReadPasteboard reads the picture on the main thread, where AppKit's
// window programs keep their user interface work. From another thread it
// waits for the main queue with dispatch_sync, which Apple's Dispatch
// documentation says deadlocks when called on the queue it targets, so on
// the main thread itself ([NSThread isMainThread]) it reads directly.
// dispatch_get_main_queue is the queue the application's main thread
// serves. The block writes through bytes and size while the caller waits.
static int dinahReadPasteboard(void **bytes, size_t *size) {
	*bytes = NULL;
	*size = 0;
	if ([NSThread isMainThread]) return dinahReadPasteboardHere(bytes, size);
	__block int status = dinahNoPicture;
	dispatch_sync(dispatch_get_main_queue(), ^{
		status = dinahReadPasteboardHere(bytes, size);
	});
	return status;
}
*/
import "C"

import (
	"errors"
	"math"
	"unsafe"
)

// readClipboardImage reads the picture on the general pasteboard: its PNG
// data when it has PNG, and otherwise its TIFF data converted to PNG by
// AppKit. No picture is an empty result, not an error.
func (a *App) readClipboardImage() (ClipboardImage, error) {
	var bytes unsafe.Pointer
	var size C.size_t
	switch C.dinahReadPasteboard(&bytes, &size) {
	case C.dinahNoPicture:
		return ClipboardImage{}, nil
	case C.dinahPicture:
		defer C.free(bytes)
		if size > math.MaxInt32 {
			return ClipboardImage{}, errors.New("the picture on the clipboard is too large to paste")
		}
		return ClipboardImage{Type: "image/png", Data: C.GoBytes(bytes, C.int(size))}, nil
	default:
		return ClipboardImage{}, errors.New("the picture on the clipboard could not be read")
	}
}
