//go:build windows && native_drawing

package drawingpage

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"unsafe"

	"github.com/wailsapp/go-webview2/pkg/edge"
	"github.com/wailsapp/go-webview2/webviewloader"
)

// This is an upstream reproduction and a check of the removable local binding.
func TestNativeResponseBody(t *testing.T) {
	root := os.Getenv("DRAWING_PAGE_EVIDENCE")
	if root == "" {
		t.Fatal("set DRAWING_PAGE_EVIDENCE")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := ole32.NewProc("CoInitializeEx").Call(0, 2)
	if int32(hr) < 0 {
		t.Fatalf("COM: %x", hr)
	}
	defer ole32.NewProc("CoUninitialize").Call()
	var msg message
	peekMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 0)
	done := false
	handler := &environmentDone{f: func(code webviewloader.HRESULT, env *webviewloader.ICoreWebView2Environment) webviewloader.HRESULT {
		defer func() { done = true }()
		if code < 0 || env == nil {
			t.Errorf("create environment: %x", uint32(code))
			return 0
		}
		environment := (*edge.ICoreWebView2Environment)(unsafe.Pointer(env))
		bad, err := environment.CreateWebResourceResponse([]byte("body-marker"), 200, "OK", "")
		if err != nil {
			t.Error(err)
			return 0
		}
		defer bad.Release()
		var missing *edge.IStream
		if _, err = call(unsafe.Pointer(bad), 3, uintptr(unsafe.Pointer(&missing))); err != nil {
			t.Error(err)
			return 0
		}
		if missing != nil {
			missing.Release()
			t.Error("dependency no longer has the shadowed-stream defect; remove this reproduction and local workaround")
		}
		good, err := responseFor(environment, []byte("body-marker"), 200, "")
		if err != nil {
			t.Error(err)
			return 0
		}
		defer good.Release()
		var content *edge.IStream
		if _, err = call(unsafe.Pointer(good), 3, uintptr(unsafe.Pointer(&content))); err != nil || content == nil {
			t.Errorf("local binding has no body: %v", err)
			return 0
		}
		defer content.Release()
		bytes := make([]byte, len("body-marker"))
		n, err := content.Read(bytes)
		if err != nil || n != len(bytes) || string(bytes) != "body-marker" {
			t.Errorf("body=%q n=%d err=%v", bytes, n, err)
		}
		return 0
	}}
	if err := webviewloader.CreateCoreWebView2EnvironmentWithOptions(handler, webviewloader.WithUserDataFolder(filepath.Join(root, "response-body-browser"))); err != nil {
		t.Fatal(err)
	}
	for !done {
		// Completion is a COM callback delivered through this apartment's pump.
		// PeekMessage also dispatches pending sent messages before returning.
		peekMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 1)
		if msg.ID != 0 {
			dispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
			msg = message{}
		}
		if !done {
			user32.NewProc("MsgWaitForMultipleObjects").Call(0, 0, 0, 100, 0x04FF)
		}
	}
	runtime.KeepAlive(handler)
}
