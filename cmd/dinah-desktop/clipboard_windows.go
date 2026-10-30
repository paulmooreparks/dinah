package main

import (
	"errors"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The clipboard functions of user32 and kernel32, as Microsoft documents
// them under "Clipboard" and "Memory Management" in the Win32 API.
var (
	user32                         = windows.NewLazySystemDLL("user32.dll")
	kernel32                       = windows.NewLazySystemDLL("kernel32.dll")
	procOpenClipboard              = user32.NewProc("OpenClipboard")
	procCloseClipboard             = user32.NewProc("CloseClipboard")
	procGetClipboardData           = user32.NewProc("GetClipboardData")
	procIsClipboardFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	procRegisterClipboardFormatW   = user32.NewProc("RegisterClipboardFormatW")
	procGlobalLock                 = kernel32.NewProc("GlobalLock")
	procGlobalUnlock               = kernel32.NewProc("GlobalUnlock")
	procGlobalSize                 = kernel32.NewProc("GlobalSize")
)

// cfUnicodeText is the standard clipboard format of UTF-16 text.
const cfUnicodeText = 13

func (a *App) readClipboard() (ClipboardContent, error) {
	name, _ := windows.UTF16PtrFromString("HTML Format")
	htmlFormat, _, _ := procRegisterClipboardFormatW.Call(uintptr(unsafe.Pointer(name)))
	// Another program may have the clipboard open for a moment.
	opened := false
	for i := 0; i < 10 && !opened; i++ {
		if r, _, _ := procOpenClipboard.Call(0); r != 0 {
			opened = true
		} else {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if !opened {
		return ClipboardContent{}, errors.New("the clipboard is in use by another program; try again")
	}
	defer procCloseClipboard.Call()
	var out ClipboardContent
	if htmlFormat != 0 {
		if data := clipboardBytes(htmlFormat); data != nil {
			out.HTML = htmlFragment(data)
		}
	}
	if data := clipboardBytes(cfUnicodeText); len(data) >= 2 {
		u := unsafe.Slice((*uint16)(unsafe.Pointer(&data[0])), len(data)/2)
		out.Text = windows.UTF16ToString(u)
	}
	return out, nil
}

// clipboardBytes copies the clipboard's data in one format, or nil when it
// has none. The clipboard must be open.
func clipboardBytes(format uintptr) []byte {
	if r, _, _ := procIsClipboardFormatAvailable.Call(format); r == 0 {
		return nil
	}
	h, _, _ := procGetClipboardData.Call(format)
	if h == 0 {
		return nil
	}
	p, _, _ := procGlobalLock.Call(h)
	if p == 0 {
		return nil
	}
	defer procGlobalUnlock.Call(h)
	size, _, _ := procGlobalSize.Call(h)
	if size == 0 {
		return nil
	}
	// The locked memory's address, which Windows returns as a number, as a
	// pointer; it stays valid until GlobalUnlock, after the copy.
	start := *(*unsafe.Pointer)(unsafe.Pointer(&p))
	return append([]byte(nil), unsafe.Slice((*byte)(start), size)...)
}
