//go:build windows && native_drawing

package drawingpage

import (
	"context"
	"encoding/json"
	"errors"
	"golang.org/x/sys/windows"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestNativeDisposablePage(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	assets := os.DirFS(filepath.Join(filepath.Dir(file), "../../cmd/dinah-desktop/frontend"))
	root := os.Getenv("DRAWING_PAGE_EVIDENCE")
	if root == "" {
		t.Fatal("set DRAWING_PAGE_EVIDENCE to a fresh temporary evidence folder")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	foreground := user32.NewProc("GetForegroundWindow")
	before, _, _ := foreground.Call()
	var monitor sync.WaitGroup
	var mu sync.Mutex
	var focusMoved bool
	var visibleWindow bool
	enumCallback := syscall.NewCallback(func(hwnd, unused uintptr) uintptr {
		var pid uint32
		user32.NewProc("GetWindowThreadProcessId").Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		visible, _, _ := user32.NewProc("IsWindowVisible").Call(hwnd)
		if pid == windows.GetCurrentProcessId() && visible != 0 {
			mu.Lock()
			visibleWindow = true
			mu.Unlock()
		}
		return 1
	})
	done := make(chan struct{})
	monitor.Add(1)
	go func() {
		defer monitor.Done()
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				user32.NewProc("EnumWindows").Call(enumCallback, 0)
				now, _, _ := foreground.Call()
				if now != before {
					mu.Lock()
					focusMoved = true
					mu.Unlock()
				}
			}
		}
	}()
	defer func() {
		close(done)
		monitor.Wait()
		mu.Lock()
		defer mu.Unlock()
		if focusMoved {
			t.Error("foreground focus moved during native acceptance")
		}
		if visibleWindow {
			t.Error("a visible top-level process window appeared")
		}
	}()
	type record struct {
		Case          string
		Events        []string
		Error         string
		Width, Height int
	}
	var records []record
	canvas := Request{Kind: "canvas", Text: `{"nodes":[{"id":"a","type":"text","text":"Native Canvas","x":0,"y":0,"width":240,"height":80}],"edges":[]}`, Scale: 1}
	mermaid := Request{Kind: "mermaid", Text: "flowchart LR\n  A[Native] --> B[Picture]", Scale: 1}
	run := func(name string, request Request, script string, kind string) {
		t.Helper()
		if filter := os.Getenv("DRAWING_PAGE_CASE"); filter != "" && filter != name {
			return
		}
		var events []string
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		if kind == "timeout" {
			ctx, cancel = context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
		}
		options := Options{DataFolder: filepath.Join(root, name, "browser"), Event: func(s string) {
			events = append(events, s)
			if s == "script-started" && kind == "cancel" {
				cancel()
			}
		}}
		var picture Picture
		var err error
		if script == "" {
			picture, err = Draw(ctx, assets, request, options)
		} else {
			picture, err = draw(ctx, fs.FS(assets), options, script)
		}
		r := record{Case: name, Events: events, Width: picture.Width, Height: picture.Height}
		if err != nil {
			r.Error = err.Error()
		}
		records = append(records, r)
		for _, event := range []string{"controller-created", "controller-hidden", "controller-closed", "closed-controller-refused"} {
			if !slices.Contains(events, event) {
				t.Errorf("%s missing %s: %v (%v)", name, event, events, err)
			}
		}
		switch kind {
		case "success":
			if err != nil {
				t.Errorf("%s: %v", name, err)
			} else if err = os.WriteFile(filepath.Join(root, name+".png"), picture.PNG, 0600); err != nil {
				t.Fatal(err)
			}
		case "failure":
			if err == nil {
				t.Errorf("%s accepted invalid diagram", name)
			}
		case "cancel", "timeout":
			if !slices.Contains(events, "script-started") {
				t.Errorf("%s did not enter stuck script", name)
			}
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("%s: %v", name, err)
			}
		}
	}
	run("canvas", canvas, "", "success")
	run("mermaid", mermaid, "", "success")
	run("mermaid-no-frames", mermaid, "window.requestAnimationFrame=()=>0;"+scriptFor(mermaid), "success")
	bad := mermaid
	bad.Text = "flowchart LR\n A --> ["
	run("failure", bad, "", "failure")
	stuck := `chrome.webview.postMessage(JSON.stringify({started:true}));while(true){}`
	run("cancel-stuck", Request{}, stuck, "cancel")
	run("watchdog-stuck", Request{}, stuck, "timeout")
	run("fresh-after-stuck", canvas, "", "success")
	data, _ := json.MarshalIndent(records, "", "  ")
	if err := os.WriteFile(filepath.Join(root, "result.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
}
