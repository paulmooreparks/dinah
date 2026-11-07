package main

import (
	"reflect"
	"testing"
)

type fakeNativeWindow struct {
	full, max         bool
	minimized, closed bool
	maxCalls          int
	unmaxCalls        int
	priorMax          bool
	placements        []string
}

func (w *fakeNativeWindow) fullscreen() bool                      { return w.full }
func (w *fakeNativeWindow) maximized() bool                       { return w.max }
func (w *fakeNativeWindow) normalPlacement() (int, int, int, int) { return 930, 710, 42, 57 }
func (w *fakeNativeWindow) enter()                                { w.priorMax = w.max; w.full = true; w.max = false }
func (w *fakeNativeWindow) leave() {
	w.full = false
	w.max = w.priorMax
	w.placements = append(w.placements, "leave")
}
func (w *fakeNativeWindow) maximize() {
	w.max = true
	w.maxCalls++
	w.placements = append(w.placements, "maximize")
}
func (w *fakeNativeWindow) unmaximize() {
	w.max = false
	w.unmaxCalls++
	w.placements = append(w.placements, "restore")
}
func (w *fakeNativeWindow) minimize() { w.minimized = true }
func (w *fakeNativeWindow) close()    { w.closed = true }
func TestNativeFullscreenPlacementAndActions(t *testing.T) {
	for _, maximized := range []bool{false, true} {
		host := &fakeNativeWindow{max: maximized}
		app := &App{nativeAPI: host, nativeWindow: nativeWindowState{Version: 1, Width: 800, Height: 600}}
		if err := app.NativeWindowAction("toggle"); err != nil {
			t.Fatal(err)
		}
		if !app.NativeWindowState().Fullscreen || app.nativeWindow.Maximized != maximized {
			t.Fatal("native state was inferred incorrectly")
		}
		if !maximized && (app.nativeWindow.Width != 930 || app.nativeWindow.X != 42) {
			t.Fatal("normal placement was not captured before fullscreen")
		}
		if err := app.NativeWindowAction("restore"); err != nil {
			t.Fatal(err)
		}
		if host.full || host.max != maximized {
			t.Fatal("prior maximization was not restored")
		}
		if maximized && host.unmaxCalls != 1 || !maximized && host.unmaxCalls != 0 {
			t.Fatal("maximized fullscreen exit must restore normal placement before maximizing")
		}
		want := []string{"leave"}
		if maximized {
			want = append(want, "restore", "maximize")
		}
		if !reflect.DeepEqual(host.placements, want) {
			t.Fatalf("placement sequence = %v, want %v", host.placements, want)
		}
		count := host.maxCalls
		app.NativeWindowAction("restore")
		if host.maxCalls != count {
			t.Fatal("restore outside fullscreen changed placement")
		}
		app.NativeWindowAction("minimize")
		app.NativeWindowAction("close")
		if !host.minimized || !host.closed {
			t.Fatal("native controls did not use the host")
		}
		if app.NativeWindowAction("arbitrary") == nil {
			t.Fatal("unknown native action accepted")
		}
	}
	app := &App{}
	if app.NativeWindowState().Available || app.NativeWindowAction("toggle") == nil {
		t.Fatal("missing host was presented as available")
	}
}
