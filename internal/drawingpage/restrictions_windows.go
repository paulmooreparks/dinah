//go:build windows

package drawingpage

import (
	"errors"
	"github.com/wailsapp/go-webview2/pkg/combridge"
	"github.com/wailsapp/go-webview2/pkg/edge"
	"golang.org/x/sys/windows"
	"unsafe"
)

// These bindings follow Microsoft's WebView2 SDK 1.0.902.49 WebView2.h.
type navigationHandler interface {
	Navigation(unsafe.Pointer, unsafe.Pointer) uintptr
}
type navigationCallback struct {
	f func(unsafe.Pointer, unsafe.Pointer) uintptr
}

func (c *navigationCallback) Navigation(a, b unsafe.Pointer) uintptr { return c.f(a, b) }

type popupHandler interface {
	Popup(unsafe.Pointer, unsafe.Pointer) uintptr
}
type popupCallback struct {
	f func(unsafe.Pointer, unsafe.Pointer) uintptr
}

func (c *popupCallback) Popup(a, b unsafe.Pointer) uintptr { return c.f(a, b) }

type downloadHandler interface {
	Download(unsafe.Pointer, unsafe.Pointer) uintptr
}
type downloadCallback struct {
	f func(unsafe.Pointer, unsafe.Pointer) uintptr
}

func (c *downloadCallback) Download(a, b unsafe.Pointer) uintptr { return c.f(a, b) }
func init() {
	combridge.RegisterVTable[combridge.IUnknown, navigationHandler]("{9ADBE429-F36D-432B-9DDC-F8881FBD76E3}", func(this uintptr, a, b unsafe.Pointer) uintptr {
		return combridge.Resolve[navigationHandler](this).Navigation(a, b)
	})
	combridge.RegisterVTable[combridge.IUnknown, popupHandler]("{D4C185FE-C81C-4989-97AF-2D3FA7AB5651}", func(this uintptr, a, b unsafe.Pointer) uintptr {
		return combridge.Resolve[popupHandler](this).Popup(a, b)
	})
	combridge.RegisterVTable[combridge.IUnknown, downloadHandler]("{EFEDC989-C396-41CA-83F7-07F845A55724}", func(this uintptr, a, b unsafe.Pointer) uintptr {
		return combridge.Resolve[downloadHandler](this).Download(a, b)
	})
}

// Same-package tests can inject a failed Close result without a product route.
var closeNativeController = func(controller unsafe.Pointer) error { _, err := call(uintptr(controller), 24); return err }

// Same-package tests can delay handler delivery while retaining its COM state.
var delayStartupCompletion func() <-chan struct{}

func restrictPage(webview *edge.ICoreWebView2, emit func(string), finish func(error)) (func(), error) {
	first := true
	nav := combridge.New[navigationHandler](&navigationCallback{f: func(sender, args unsafe.Pointer) uintptr {
		var address *uint16
		_, err := call(uintptr(args), 3, uintptr(unsafe.Pointer(&address)))
		uri := ""
		if address != nil {
			uri = windows.UTF16PtrToString(address)
			ole32.NewProc("CoTaskMemFree").Call(uintptr(unsafe.Pointer(address)))
		}
		if err == nil && first && uri == origin+"/" {
			first = false
			return 0
		}
		if _, e := call(uintptr(args), 8, 1); e != nil {
			finish(e)
		}
		emit("navigation-refused")
		return 0
	}})
	popup := combridge.New[popupHandler](&popupCallback{f: func(sender, args unsafe.Pointer) uintptr {
		if _, err := call(uintptr(args), 6, 1); err != nil {
			finish(err)
		}
		emit("popup-refused")
		return 0
	}})
	download := combridge.New[downloadHandler](&downloadCallback{f: func(sender, args unsafe.Pointer) uintptr {
		if _, err := call(uintptr(args), 5, 1); err != nil {
			finish(err)
		}
		emit("download-refused")
		return 0
	}})
	clean := func() { nav.Close(); popup.Close(); download.Close() }
	var token int64
	for _, binding := range []struct {
		slot int
		ref  uintptr
	}{{7, nav.Ref()}, {17, nav.Ref()}, {44, popup.Ref()}} {
		if _, err := call(uintptr(unsafe.Pointer(webview)), binding.slot, binding.ref, uintptr(unsafe.Pointer(&token))); err != nil {
			return clean, err
		}
	}
	iid := windows.GUID{Data1: 0x20d02d59, Data2: 0x6df2, Data3: 0x42dc, Data4: [8]byte{0xbd, 0x06, 0xf9, 0x8a, 0x69, 0x4b, 0x13, 0x02}}
	var newer unsafe.Pointer
	if _, err := call(uintptr(unsafe.Pointer(webview)), 0, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&newer))); err != nil || newer == nil {
		return clean, errors.New("download refusal requires WebView2 interface 4")
	}
	defer call(uintptr(newer), 2)
	if _, err := call(uintptr(newer), 75, download.Ref(), uintptr(unsafe.Pointer(&token))); err != nil {
		return clean, err
	}
	return clean, nil
}
