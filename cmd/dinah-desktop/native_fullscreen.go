package main

import (
	"context"
	"errors"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type nativeWindowAPI interface {
	fullscreen() bool
	maximized() bool
	normalPlacement() (int, int, int, int)
	enter()
	leave()
	maximize()
	minimize()
	close()
}
type wailsNativeWindow struct{ ctx context.Context }

func (w wailsNativeWindow) fullscreen() bool { return wruntime.WindowIsFullscreen(w.ctx) }
func (w wailsNativeWindow) maximized() bool  { return wruntime.WindowIsMaximised(w.ctx) }
func (w wailsNativeWindow) normalPlacement() (int, int, int, int) {
	width, height := wruntime.WindowGetSize(w.ctx)
	x, y := wruntime.WindowGetPosition(w.ctx)
	return width, height, x, y
}
func (w wailsNativeWindow) enter()    { wruntime.WindowFullscreen(w.ctx) }
func (w wailsNativeWindow) leave()    { wruntime.WindowUnfullscreen(w.ctx) }
func (w wailsNativeWindow) maximize() { wruntime.WindowMaximise(w.ctx) }
func (w wailsNativeWindow) minimize() { wruntime.WindowMinimise(w.ctx) }
func (w wailsNativeWindow) close()    { wruntime.Quit(w.ctx) }

type nativeChromeState struct {
	Available  bool `json:"available"`
	Fullscreen bool `json:"fullscreen"`
}

func (a *App) nativeHost() nativeWindowAPI {
	if a.nativeAPI != nil {
		return a.nativeAPI
	}
	if a.ctx == nil {
		return nil
	}
	return wailsNativeWindow{a.ctx}
}
func (a *App) NativeWindowState() nativeChromeState {
	a.nativeMu.Lock()
	defer a.nativeMu.Unlock()
	host := a.nativeHost()
	if host == nil {
		return nativeChromeState{}
	}
	return nativeChromeState{Available: true, Fullscreen: host.fullscreen()}
}

// NativeWindowAction uses the native host, never the browser fullscreen API.
// Wails documents that leaving fullscreen restores the preceding dimensions
// and position. Maximization is restored explicitly.
func (a *App) NativeWindowAction(action string) error {
	a.nativeMu.Lock()
	defer a.nativeMu.Unlock()
	host := a.nativeHost()
	if host == nil {
		return errors.New("native window is unavailable")
	}
	switch action {
	case "toggle", "restore":
		wasFull := host.fullscreen()
		if action == "restore" && !wasFull {
			return nil
		}
		if wasFull {
			host.leave()
		} else {
			a.nativeRestoreMax = host.maximized()
			var width, height, x, y int
			if !a.nativeRestoreMax {
				width, height, x, y = host.normalPlacement()
			}
			a.mu.Lock()
			a.nativeWindow.Maximized = a.nativeRestoreMax
			if !a.nativeRestoreMax {
				a.nativeWindow.Width, a.nativeWindow.Height, a.nativeWindow.X, a.nativeWindow.Y = width, height, x, y
				a.nativeWindow.HasPosition = true
			}
			a.mu.Unlock()
			host.enter()
		}
		deadline := time.Now().Add(3 * time.Second)
		for host.fullscreen() == wasFull {
			if time.Now().After(deadline) {
				return errors.New("the native window did not finish changing full-screen mode")
			}
			time.Sleep(20 * time.Millisecond)
		}
		if wasFull && a.nativeRestoreMax {
			host.maximize()
		}
	case "minimize":
		host.minimize()
	case "close":
		host.close()
	default:
		return errors.New("unknown native window action")
	}
	return nil
}
