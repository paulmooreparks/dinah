package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUIStateRoundTripAndWindowPreference(t *testing.T) {
	a, _ := providerTestApp(t)
	state, err := a.ReadUIState()
	if err != nil || state.Version != 1 || len(state.Values) != 0 {
		t.Fatalf("empty state: %+v %v", state, err)
	}
	key := "window:" + strings.Repeat("long-workspace-path", 25)
	state.Values[key] = json.RawMessage(`{"left":12,"top":14,"maximized":false}`)
	if err := a.SaveUIState(state); err != nil {
		t.Fatal(err)
	}
	restarted := closeBusWhenDone(t, NewApp())
	if restarted.startupErr != nil {
		t.Fatal(restarted.startupErr)
	}
	got, err := restarted.ReadUIState()
	if err != nil || string(got.Values[key]) == "" {
		t.Fatalf("layout did not survive restart: %+v %v", got, err)
	}
	if a.State().NewWindowMode != "floating" {
		t.Fatal("floating is not the default")
	}
	// Several full editor drafts must fit alongside their layout state.
	draft, _ := json.Marshal(strings.Repeat("x", 1<<20))
	state.Values["flow-one-draft"] = draft
	state.Values["flow-two-draft"] = draft
	if err := a.SaveUIState(state); err != nil {
		t.Fatalf("multiple editor drafts should fit: %v", err)
	}
	if err := a.SetNewWindowMode("maximized"); err != nil {
		t.Fatal(err)
	}
	if closeBusWhenDone(t, NewApp()).State().NewWindowMode != "maximized" {
		t.Fatal("new-window preference did not persist")
	}
	if err := a.SetNewWindowMode("minimized"); err == nil {
		t.Fatal("unknown preference accepted")
	}
}

func TestUIStateRefusesCorruptionAndFutureVersions(t *testing.T) {
	for _, content := range []string{`{`, `null`, `{"version":2,"values":{}}`, `{"version":1,"values":null}`} {
		t.Run(content, func(t *testing.T) {
			a, _ := providerTestApp(t)
			path := filepath.Join(configDir(), "ui-state.json")
			stateTestWrite(t, path, content)
			if _, err := a.ReadUIState(); err == nil {
				t.Fatal("invalid state accepted")
			}
			if err := a.SaveUIState(UIState{Version: 1, Values: map[string]json.RawMessage{}}); err == nil {
				t.Fatal("invalid state replaced")
			}
			got, _ := os.ReadFile(path)
			if string(got) != content {
				t.Fatal("invalid state was modified")
			}
		})
	}
	a, _ := providerTestApp(t)
	huge, _ := json.Marshal(strings.Repeat("x", maxUIStateBytes))
	if err := a.SaveUIState(UIState{Version: 1, Values: map[string]json.RawMessage{"large": huge}}); err == nil {
		t.Fatal("oversized UI state accepted")
	}
}

func TestSettingsCorruptionIsExplicit(t *testing.T) {
	for _, content := range []string{`{`, `null`, `{"newWindowMode":"unknown"}`} {
		t.Run(content, func(t *testing.T) {
			_, _ = providerTestApp(t)
			stateTestWrite(t, filepath.Join(configDir(), "settings.json"), content)
			if closeBusWhenDone(t, NewApp()).startupErr == nil {
				t.Fatal("corrupt settings silently reset")
			}
		})
	}
}

func TestNativeWindowClampsToCurrentMonitor(t *testing.T) {
	state := nativeWindowState{Version: 1, Width: 4000, Height: 3000, X: -800, Y: 6000, HasPosition: true, Maximized: true}
	got := clampNativeWindow(state, 1920, 1080)
	if got.Width != 1920 || got.Height != 1080 || got.X != 0 || got.Y != 0 || !got.Maximized {
		t.Fatalf("unsafe geometry: %+v", got)
	}
	got = clampNativeWindow(nativeWindowState{Width: 900, Height: 700, X: 1800, Y: 900, HasPosition: true}, 1920, 1080)
	if got.X != 1020 || got.Y != 380 {
		t.Fatalf("off-screen position not clamped: %+v", got)
	}
	if clampNativeWindow(state, 0, 0).HasPosition {
		t.Fatal("unknown monitor should use centering")
	}
}

func TestNativeCloseFlushHandshake(t *testing.T) {
	a, _ := providerTestApp(t)
	var requests, timeouts int
	a.sink = func(name string, _ any) {
		if name == "dinah:ui-state-flush" {
			requests++
		}
		if name == "dinah:ui-state-flush-timeout" {
			timeouts++
		}
	}
	if a.requestUIStateFlush() {
		t.Fatal("close allowed before flush")
	}
	id := a.closeRequest
	if a.requestUIStateFlush() || requests != 1 {
		t.Fatal("repeat close created another flush")
	}
	if err := a.acceptUIStateFlush("stale", true); err == nil {
		t.Fatal("stale acknowledgment accepted")
	}
	if err := a.acceptUIStateFlush(id, false); err != nil {
		t.Fatal(err)
	}
	if a.closeApproved || a.closeTimer != nil || a.closeRequest != "" {
		t.Fatal("failed flush did not clear its timer")
	}
	a.requestUIStateFlush()
	id = a.closeRequest
	a.expireUIStateFlush(id)
	if timeouts != 1 || a.closeApproved || a.closeTimer != nil {
		t.Fatal("timeout did not keep window open and clear timer")
	}
	if err := a.acceptUIStateFlush(id, true); err == nil {
		t.Fatal("timed-out acknowledgment accepted")
	}
	a.requestUIStateFlush()
	id = a.closeRequest
	if err := a.acceptUIStateFlush(id, true); err != nil {
		t.Fatal(err)
	}
	if !a.requestUIStateFlush() || a.closeApproved {
		t.Fatal("successful flush did not approve exactly one close")
	}
}
