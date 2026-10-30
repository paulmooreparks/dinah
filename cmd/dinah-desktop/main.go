// Command dinah-desktop is Dinah's window: a Wails window over a PUDL page,
// with agents behind it reached through the Agent Client Protocol.
//
// Build it with `dinah install` from the top of the repository, or
//
//	go build -tags desktop,production -ldflags "-H windowsgui -s -w" ./cmd/dinah-desktop
//
// On Windows the build links the window's icon, manifest and version
// information from the rsrc_windows_*.syso files in this folder, which go
// build picks up by itself. They are made from build/windows by the
// generate directive below; run `go generate ./cmd/dinah-desktop` after
// changing anything there.
package main

//go:generate go run github.com/tc-hib/go-winres@v0.3.3 make --in build/windows/winres.json --out rsrc --arch amd64,arm64

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"flag"
	"fmt"
	"html"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	wails "github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"studio/internal/pty"
)

// appVersion is the release the app belongs to, the same as the command's
// version, stamped by the build with -ldflags "-X main.appVersion=...".
var appVersion = "0.0.0-dev"

// shownVersion is the version the About box shows: appVersion and the commit
// the build was made from, which Go records in a program built from a
// checkout (go help build, under -buildvcs), so two builds of one version
// can be told apart.
func shownVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return appVersion
	}
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" && len(s.Value) >= 7 {
			return appVersion + " (" + s.Value[:7] + ")"
		}
	}
	return appVersion
}

//go:embed frontend/*.html frontend/*.css frontend/*.js frontend/*.svg frontend/pudl frontend/vendor
var assets embed.FS

// webviewDataPath versions the WebView2 profile by app version, so a page
// cached by an earlier build is never served.
func webviewDataPath() string {
	p := filepath.Join(configDir(), "webview2", appVersion)
	if os.MkdirAll(p, 0o700) != nil {
		return ""
	}
	return p
}

func main() {
	if len(os.Args) >= 2 && os.Args[1] == "--applet-mcp" {
		flags := flag.NewFlagSet("applet-mcp", flag.ExitOnError)
		stateDir := flags.String("state-dir", "", product.Name+" state directory")
		_ = flags.Parse(os.Args[2:])
		if flags.NArg() != 0 {
			log.Print("unexpected applet helper arguments")
			os.Exit(1)
		}
		if *stateDir == "" {
			if err := initializeStateDirectory(); err != nil {
				log.Print(err)
				os.Exit(1)
			}
			*stateDir = configDir()
		}
		if err := runAppletMCP(*stateDir); err != nil {
			log.Print(err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) >= 2 && os.Args[1] == "--bus-mcp" {
		if err := runBusMCP(os.Args[2:]); err != nil {
			log.Print(err)
			os.Exit(1)
		}
		return
	}
	// A new build started by Restart to Update waits for the old window to
	// end, which lets go of the single-instance lock (update.go).
	os.Args = afterProcessEnds(os.Args)
	if err := initializeStateDirectory(); err != nil {
		showStartupFailure(err)
		return
	}
	// The Terminal's shells take the pseudoconsole's buffers as their
	// standard handles only if this process has none of its own to pass on.
	if err := pty.ReleaseStandardHandles(); err != nil {
		log.Print(err)
	}
	crashed := captureCrashes()
	app := NewApp()
	if app.startupErr != nil {
		showStartupFailure(app.startupErr)
		return
	}
	app.reportPreviousCrash(crashed)
	frontend, err := fs.Sub(assets, "frontend")
	if err != nil {
		log.Fatal(err)
	}
	// A browser test drives the live page by signing in with a code the
	// window writes to this file. Whoever can set the window's environment
	// already controls it, so the hook grants nothing new. Under test the
	// window stays hidden and serves on a free port, so it never appears on
	// the person's screen or meets a Dinah they are using.
	entryFile := os.Getenv("DINAH_SHELL_ENTRY_FILE")
	testing := entryFile != ""
	addr := shellAddr
	if testing {
		addr = "127.0.0.1:0"
	}
	// The page's port is taken in OnStartup, which runs only in the window
	// that holds the single-instance lock: a second launch brings the first
	// window forward and ends before it would meet the first one's port
	// (27 October 2026). OnStartup runs alongside the loading of the
	// toolkit's own page, so that page's handler waits for the outcome.
	var shell *shellServer
	var failure error
	served := make(chan struct{})
	// The window binds nothing: the page reaches the App through the shell
	// server's bridge (shell_server.go). The toolkit's own page only signs
	// the window in and sends it there.
	// A window closed maximised starts maximised through Wails' own start
	// state. Maximising it later, once the page is up, left Windows showing
	// it as maximised at its normal size (23 October 2026).
	startState := options.Normal
	if app.nativeWindow.Maximized && !testing {
		startState = options.Maximised
	}
	err = wails.Run(&options.App{
		EnableDefaultContextMenu: true,
		Title:                    product.Name,
		Width:                    max(app.nativeWindow.Width, 720),
		Height:                   max(app.nativeWindow.Height, 480),
		MinWidth:                 720,
		MinHeight:                480,
		WindowStartState:         startState,
		StartHidden:              testing,
		AssetServer: &assetserver.Options{
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				<-served
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.Header().Set("Cache-Control", "no-store")
				title := html.EscapeString(product.Name)
				if failure != nil {
					fmt.Fprintf(w, `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>%s</title></head><body></body></html>`, title)
					return
				}
				fmt.Fprintf(w, `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>%s</title>`+
					`<meta http-equiv="refresh" content="0; url=%s"></head><body></body></html>`, title, html.EscapeString(shell.entry()))
			}),
		},
		OnStartup: func(ctx context.Context) {
			shell, failure = newShellServer(app, frontend, addr)
			close(served)
			if failure != nil {
				wruntime.Quit(ctx)
				return
			}
			go shell.serve()
			if testing {
				os.WriteFile(entryFile, []byte(shell.entry()), 0o600)
			}
			app.startup(ctx)
		},
		OnDomReady: app.restoreNativeWindow,
		// A window whose page could not be served closes without asking the
		// page, which never loaded, and has nothing of the App's to shut down.
		OnBeforeClose: func(ctx context.Context) bool {
			<-served
			if failure != nil {
				return false
			}
			return app.beforeClose(ctx)
		},
		OnShutdown: func(ctx context.Context) {
			<-served
			if failure == nil {
				app.shutdown(ctx)
			}
		},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1},
		// A second launch brings this window forward rather than starting a
		// second App, which would keep its own settings in memory and save
		// over the first's.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: instanceID(),
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				// A window under test stays hidden.
				if app.ctx != nil && !testing {
					wruntime.WindowUnminimise(app.ctx)
					wruntime.Show(app.ctx)
				}
			},
		},
		Windows: &windows.Options{WebviewUserDataPath: webviewDataPath(), WindowClassName: windowClass},
	})
	if shell != nil {
		shell.close()
	}
	if err != nil {
		log.Fatal(err)
	}
	if failure != nil {
		showStartupFailure(failure)
	}
}

// instanceID is the single-instance lock's name. The lock keeps one window
// per state folder, since it exists so that a second window cannot save
// over the first's settings: the person's own folder keeps the name it has
// always had, and a window with the profile's home variable (DINAH_HOME)
// set, such as one under test, gets a name of its own for that folder.
func instanceID() string {
	base := product.InstanceLock
	home := os.Getenv(product.HomeVar())
	if home == "" {
		return base
	}
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(home))))
	return base + "-" + hex.EncodeToString(sum[:6])
}

// A settings failure must be visible in a Windows GUI build, which has no
// console. This window binds no application methods and cannot reset data.
func showStartupFailure(failure error) {
	log.Print(failure)
	logf("core", "error", "%s could not start: %v", product.Name, failure)
	// A window under test shows nothing on the person's screen.
	if os.Getenv("DINAH_SHELL_ENTRY_FILE") != "" {
		return
	}
	title := product.Name + " Could Not Start"
	name := html.EscapeString(product.Name)
	err := wails.Run(&options.App{Title: title, Width: 800, Height: 420,
		AssetServer: &assetserver.Options{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte("<!doctype html><html><head><title>" + html.EscapeString(title) + "</title></head><body style='font:16px system-ui;padding:24px'><h1>" + html.EscapeString(title) + "</h1><p>The reason is below. Resolve it, then open " + name + " again. Your files have not been reset.</p><pre style='white-space:pre-wrap'>" + html.EscapeString(failure.Error()) + "</pre></body></html>"))
		})}})
	if err != nil {
		log.Print(err)
	}
}
