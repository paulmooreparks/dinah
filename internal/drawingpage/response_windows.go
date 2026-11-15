//go:build windows

package drawingpage

import (
	"errors"
	"runtime"
	"unsafe"

	"github.com/wailsapp/go-webview2/pkg/edge"
	"golang.org/x/sys/windows"
)

// Use the documented SHCreateMemStream and Environment.CreateWebResourceResponse
// directly. edge v1.0.22 shadows its stream variable and passes a null body.
// Remove this binding when go-webview2 fixes that nonempty-content branch.
// API: https://learn.microsoft.com/windows/win32/api/shlwapi/nf-shlwapi-shcreatememstream
func responseFor(environment *edge.ICoreWebView2Environment, body []byte, status int, headers string) (*edge.ICoreWebView2WebResourceResponse, error) {
	var stream uintptr
	if len(body) > 0 {
		stream, _, _ = windows.NewLazySystemDLL("shlwapi.dll").NewProc("SHCreateMemStream").Call(uintptr(unsafe.Pointer(&body[0])), uintptr(len(body)))
		if stream == 0 {
			return nil, errors.New("create drawing asset stream")
		}
		defer call(unsafe.Pointer(stream), 2)
	}
	reason, _ := windows.UTF16PtrFromString("Drawing resource")
	header, _ := windows.UTF16PtrFromString(headers)
	var result *edge.ICoreWebView2WebResourceResponse
	_, err := call(unsafe.Pointer(environment), 4, stream, uintptr(status), uintptr(unsafe.Pointer(reason)), uintptr(unsafe.Pointer(header)), uintptr(unsafe.Pointer(&result)))
	runtime.KeepAlive(body)
	return result, err
}
