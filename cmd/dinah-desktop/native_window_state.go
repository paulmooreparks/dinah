package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type nativeWindowState struct {
	Version     int  `json:"version"`
	Width       int  `json:"width"`
	Height      int  `json:"height"`
	X           int  `json:"x"`
	Y           int  `json:"y"`
	HasPosition bool `json:"hasPosition"`
	Maximized   bool `json:"maximized"`
}

func readNativeWindowState() (nativeWindowState, error) {
	state := nativeWindowState{Version: 1, Width: 1280, Height: 860}
	data, err := readStateFile(filepath.Join(configDir(), "native-window.json"))
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	state = nativeWindowState{}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, fmt.Errorf("read native window state: %w", err)
	}
	if state.Version != 1 || state.Width < 1 || state.Height < 1 || state.Width > 32768 || state.Height > 32768 {
		return state, errors.New("native window state is invalid or from an unsupported version")
	}
	return state, nil
}

// Wails documents positions relative to the current monitor. Restoring those
// coordinates does not restore a monitor identity, which Wails does not expose.
func clampNativeWindow(state nativeWindowState, width, height int) nativeWindowState {
	if width <= 0 || height <= 0 {
		state.HasPosition = false
		return state
	}
	state.Width = min(max(state.Width, 720), width)
	state.Height = min(max(state.Height, 480), height)
	state.X = min(max(state.X, 0), max(width-state.Width, 0))
	state.Y = min(max(state.Y, 0), max(height-state.Height, 0))
	return state
}

// restoreNativeWindow runs as Wails' OnDomReady, which Wails calls on its
// own goroutine after every page load, the toolkit's redirect page
// included, and so possibly while it is still showing the window. The
// placement is restored once. A window that starts maximised gets its size
// and start state from the options in main.go and is not touched here:
// sizing it while Windows maximised it left it marked maximised at its
// normal size (23 October 2026).
func (a *App) restoreNativeWindow(ctx context.Context) {
	a.placed.Do(func() {
		setWindowIcon()
		if !a.nativeWindow.Maximized {
			a.placeNativeWindow(ctx)
		}
	})
}

func (a *App) placeNativeWindow(ctx context.Context) {
	state := a.nativeWindow
	screens, err := wruntime.ScreenGetAll(ctx)
	current := false
	if err == nil {
		for _, screen := range screens {
			if screen.IsCurrent {
				state = clampNativeWindow(state, screen.Width, screen.Height)
				wruntime.WindowSetMinSize(ctx, min(720, screen.Width), min(480, screen.Height))
				current = true
				break
			}
		}
	}
	wruntime.WindowSetSize(ctx, state.Width, state.Height)
	if current && state.HasPosition {
		wruntime.WindowSetPosition(ctx, state.X, state.Y)
	} else {
		wruntime.WindowCenter(ctx)
	}
}

func (a *App) beforeClose(ctx context.Context) bool {
	if !a.confirmExitIfNeeded(ctx) {
		return true
	}
	if !a.requestUIStateFlush() {
		return true
	}
	a.mu.Lock()
	state := a.nativeWindow
	a.mu.Unlock()
	if !wruntime.WindowIsMinimised(ctx) && !wruntime.WindowIsFullscreen(ctx) {
		state.Maximized = wruntime.WindowIsMaximised(ctx)
		if wruntime.WindowIsNormal(ctx) {
			state.Width, state.Height = wruntime.WindowGetSize(ctx)
			state.X, state.Y = wruntime.WindowGetPosition(ctx)
			state.HasPosition = true
		}
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err == nil {
		err = writeStateFile(filepath.Join(configDir(), "native-window.json"), data)
	}
	if err != nil {
		_, _ = wruntime.MessageDialog(ctx, wruntime.MessageDialogOptions{Type: wruntime.ErrorDialog, Title: "Window Settings Could Not Be Saved", Message: err.Error()})
		a.abandonClose()
		return true
	}
	a.mu.Lock()
	a.nativeWindow = state
	a.mu.Unlock()
	return false
}

func (a *App) requestUIStateFlush() bool {
	a.mu.Lock()
	if a.closeApproved {
		a.closeApproved = false
		a.mu.Unlock()
		return true
	}
	if a.closeRequest != "" {
		a.mu.Unlock()
		return false
	}
	id := strconv.FormatInt(a.qseq.Add(1), 10)
	a.closeRequest = id
	a.closeTimer = time.AfterFunc(15*time.Second, func() { a.expireUIStateFlush(id) })
	a.mu.Unlock()
	a.emit("dinah:ui-state-flush", map[string]string{"id": id})
	return false
}

func (a *App) expireUIStateFlush(id string) {
	a.mu.Lock()
	if a.closeRequest != id {
		a.mu.Unlock()
		return
	}
	a.closeRequest = ""
	a.exitConfirmed = false
	if a.closeTimer != nil {
		a.closeTimer.Stop()
		a.closeTimer = nil
	}
	a.mu.Unlock()
	a.restartAbandoned()
	a.emit("dinah:ui-state-flush-timeout", map[string]string{"id": id, "detail": "The window stayed open because its layout save did not finish. Please retry closing it."})
}

func (a *App) acceptUIStateFlush(id string, success bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if id == "" || a.closeRequest != id {
		return errors.New("the layout-save request is no longer current; the window has stayed open")
	}
	if a.closeTimer != nil {
		a.closeTimer.Stop()
		a.closeTimer = nil
	}
	a.closeRequest = ""
	a.closeApproved = success
	if !success {
		a.exitConfirmed = false
	}
	return nil
}

func (a *App) AckUIStateFlush(id string, success bool) error {
	if err := a.acceptUIStateFlush(id, success); err != nil {
		return err
	}
	if success {
		wruntime.Quit(a.ctx)
	}
	return nil
}
