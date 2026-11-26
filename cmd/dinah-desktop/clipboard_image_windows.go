package main

import "errors"

// The standard clipboard formats of a device-independent bitmap ("Standard
// Clipboard Formats", Microsoft's Win32 documentation): CF_DIB is a
// BITMAPINFO structure followed by the bitmap bits, and CF_DIBV5 is a
// BITMAPV5HEADER followed by the colour space information and the bits.
// Windows synthesizes each of them, and CF_BITMAP, from the others
// ("Synthesized Clipboard Formats"), so a picture copied in any of the
// three can be read as one of these.
//
// The unregistered "PNG" format some programs also write is not read,
// because Microsoft does not document it.
const (
	cfDIB   = 8
	cfDIBV5 = 17
)

// readClipboardImage reads CF_DIBV5 when the clipboard has it, and CF_DIB
// otherwise, and converts the bitmap to PNG (clipboard_dib.go). No picture
// is an empty result, not an error.
func (a *App) readClipboardImage() (ClipboardImage, error) {
	if err := openClipboard(); err != nil {
		return ClipboardImage{}, err
	}
	defer procCloseClipboard.Call()
	for _, format := range []uintptr{cfDIBV5, cfDIB} {
		if r, _, _ := procIsClipboardFormatAvailable.Call(format); r == 0 {
			continue
		}
		dib := clipboardBytes(format)
		if dib == nil {
			return ClipboardImage{}, errors.New("the picture on the clipboard could not be read")
		}
		data, err := dibToPNG(dib)
		if err != nil {
			return ClipboardImage{}, err
		}
		return ClipboardImage{Type: "image/png", Data: data}, nil
	}
	return ClipboardImage{}, nil
}
