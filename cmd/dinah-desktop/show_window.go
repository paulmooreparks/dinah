package main

import (
	"context"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	wails "github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// showOption starts the window program as a slide show's window in place of
// the main window (docs/design/detached-slide-show.md). The toolkit gives a
// program one window, so a show on another display is a second copy of the
// program, which the main program starts and ends (show_sessions.go).
const showOption = "--slide-show"

// showWindowClass is the Windows class of the show's window, which the
// process finds its own window by.
var showWindowClass = product.WindowClass + "Show"

// showWebviewDataPath is the browser profile of the show's window. It is
// not the main window's: the show's page signs in with a cookie of its own,
// which lets it read what a slide needs and nothing else.
func showWebviewDataPath() string {
	p := filepath.Join(configDir(), "webview2", appVersion+"-show")
	if os.MkdirAll(p, 0o700) != nil {
		return ""
	}
	return p
}

// loopbackEntry says an address is on this machine's loopback interface,
// where the main program serves its pages.
func loopbackEntry(entry string) bool {
	u, err := url.Parse(entry)
	return err == nil && u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost")
}

// runSlideShow is the show's process. It opens one borderless window with no
// page of its own, sends it to the address the main program gave it, and on
// the first page puts the window on the display it was told, makes it full
// screen there with the toolkit's own full screen, shows it and asks for the
// keyboard. The main program ends the process when the show ends; closing
// the window ends it too, which the main program sees.
func runSlideShow() int {
	entry := os.Getenv(showEntryVar())
	place, placed := parseShowRect(os.Getenv(showPlaceVar()))
	if !loopbackEntry(entry) || !placed {
		fmt.Fprintln(os.Stderr, product.WindowProgram, showOption, "is started by", product.Name, "itself.")
		return 2
	}
	hidden := os.Getenv(showHiddenVar()) != ""
	title := product.Name + " Slide Show"
	var once sync.Once
	err := wails.Run(&options.App{
		Title:            title,
		Width:            place.Width,
		Height:           place.Height,
		Frameless:        true,
		StartHidden:      true,
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 255},
		AssetServer: &assetserver.Options{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			fmt.Fprintf(w, `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>%s</title>`+
				`<meta http-equiv="refresh" content="0; url=%s"></head><body style="background:#000"></body></html>`, html.EscapeString(title), html.EscapeString(entry))
		})},
		OnDomReady: func(ctx context.Context) {
			once.Do(func() {
				moveShowWindow(showWindowClass, place)
				// Under test the window is placed and never shown.
				if hidden {
					return
				}
				wruntime.WindowShow(ctx)
				wruntime.WindowFullscreen(ctx)
				focusShowWindow(showWindowClass)
				keepDisplayAwake()
			})
		},
		Windows: &windows.Options{WebviewUserDataPath: showWebviewDataPath(), WindowClassName: showWindowClass},
	})
	if err != nil {
		log.Print(err)
		return 1
	}
	return 0
}
