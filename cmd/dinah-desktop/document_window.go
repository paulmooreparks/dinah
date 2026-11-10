package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"sync"
	"time"

	wails "github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"studio/internal/docservice"
)

// documentWindowClass is the Windows class of a document's window, which
// the process finds its own window by.
var documentWindowClass = product.WindowClass + "Document"

// documentWebviewDataPath is the browser profile the document windows
// share. It is not the main window's: a document's page signs in with a
// cookie of its own.
func documentWebviewDataPath() string {
	p := filepath.Join(configDir(), "webview2", appVersion+"-documents")
	if os.MkdirAll(p, 0o700) != nil {
		return ""
	}
	return p
}

// documentStart is what a document's window is started with.
type documentStart struct {
	application, ticket, path string
}

// parseDocumentStart reads the words after documentOption.
func parseDocumentStart(args []string) (documentStart, error) {
	var s documentStart
	if len(args) == 0 {
		return s, errors.New("say which application to open")
	}
	s.application, args = args[0], args[1:]
	if _, ok := documentApplications[s.application]; !ok {
		return s, fmt.Errorf("there is no application called %q", s.application)
	}
	if len(args) >= 2 && args[0] == "--ticket" {
		s.ticket, args = args[1], args[2:]
	}
	switch len(args) {
	case 0:
	case 1:
		// The service keeps the whole path, so a file named from the
		// folder a command line is in means that file wherever the service
		// was started.
		abs, err := filepath.Abs(args[0])
		if err != nil {
			return s, err
		}
		s.path = abs
	default:
		return s, errors.New("a window shows one document")
	}
	return s, nil
}

// documentEntry is the line a window under test adds to the test's file.
type documentEntry struct {
	ID          string `json:"id"`
	Application string `json:"application"`
	Path        string `json:"path"`
	Entry       string `json:"entry,omitempty"`
	Existing    bool   `json:"existing"`
	PID         int    `json:"pid"`
	// Event is set on the lines that follow a window's first: what the
	// service asked of it, such as "focus".
	Event string `json:"event,omitempty"`
}

func appendDocumentEntry(file string, e documentEntry) {
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	f, err := os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(append(data, '\n'))
}

// runDocumentWindow is a document's window process. It finds the document
// service, starting it when there is none, and asks it for the document. A
// file that a window of the application already shows brings that window
// forward, and this process ends. Otherwise it opens one window on the
// page the service gave it, and from then on does what the service asks of
// the window: come forward, change the title, show one of the system's
// dialogs. Everything else is between the page and the service.
func runDocumentWindow(args []string) int {
	start, err := parseDocumentStart(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, product.WindowProgram, documentOption+":", err)
		return 2
	}
	if err := initializeStateDirectory(); err != nil {
		showStartupFailure(err)
		return 1
	}
	entryFile := os.Getenv(documentEntryVar())
	testing := entryFile != ""
	client, err := docservice.Ensure(documentsDir(), startDocumentService)
	if err != nil {
		showStartupFailure(fmt.Errorf("the document could not be opened: %w", err))
		return 1
	}
	client.PID = os.Getpid()
	ctx := context.Background()
	opened, err := client.Open(ctx, docservice.OpenRequest{Application: start.application, Path: start.path, Ticket: start.ticket})
	if err != nil {
		showStartupFailure(fmt.Errorf("the document could not be opened: %w", err))
		return 1
	}
	if opened.Existing {
		// This process was started by what the person just did, so it may
		// let the window's process come forward, and then asks it to.
		allowForeground(opened.PID)
		client.Focus(ctx, opened.ID)
		if testing {
			appendDocumentEntry(entryFile, documentEntry{ID: opened.ID, Application: start.application, Path: start.path, Existing: true, PID: os.Getpid()})
		}
		return 0
	}
	if testing {
		appendDocumentEntry(entryFile, documentEntry{ID: opened.ID, Application: start.application, Path: start.path, Entry: opened.Entry, PID: os.Getpid()})
	}
	app := documentApplications[start.application]
	setProcessIdentity(start.application)
	w := &documentWindow{client: client, id: opened.ID, entry: opened.Entry, testing: testing, entryFile: entryFile, application: start.application}
	err = wails.Run(&options.App{
		EnableDefaultContextMenu: true,
		Title:                    opened.Title,
		Width:                    1100,
		Height:                   800,
		MinWidth:                 480,
		MinHeight:                320,
		StartHidden:              testing,
		BackgroundColour:         &options.RGBA{R: 255, G: 255, B: 255, A: 1},
		AssetServer:              &assetserver.Options{Handler: http.HandlerFunc(w.servePage)},
		OnStartup:                w.startup,
		OnDomReady:               func(context.Context) { setWindowIconFor(documentWindowClass) },
		OnBeforeClose:            w.beforeClose,
		Windows:                  &windows.Options{WebviewUserDataPath: documentWebviewDataPath(), WindowClassName: documentWindowClass},
	})
	if err != nil {
		log.Print(err)
		logf("documents", "error", "%s could not show its window: %v", app.Name, err)
		return 1
	}
	return 0
}

// documentWindow is the window process's own state.
type documentWindow struct {
	client      *docservice.Client
	id          string
	application string
	testing     bool
	entryFile   string

	mu    sync.Mutex
	entry string
	ctx   context.Context
	// closing says the window is closing with the person's leave, so the
	// loop that hears from the service ends quietly.
	closing bool
}

// servePage is the toolkit's own page, which only sends the window on to
// the document's page with a way in. A way in is good once, so a page that
// is loaded again gets a new one. Under test the window stays empty: the
// test's browser is the page, and a second page would answer the service
// too.
func (w *documentWindow) servePage(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	rw.Header().Set("Cache-Control", "no-store")
	title := html.EscapeString(documentApplications[w.application].Name)
	if w.testing {
		fmt.Fprintf(rw, `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>%s</title></head><body></body></html>`, title)
		return
	}
	w.mu.Lock()
	entry := w.entry
	w.entry = ""
	w.mu.Unlock()
	if entry == "" {
		again, err := w.client.Entry(r.Context(), w.id)
		if err != nil {
			fmt.Fprintf(rw, `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>%s</title></head>`+
				`<body style="font:16px system-ui;padding:24px"><p>This document's window can no longer reach the document service. Close the window and open the document again.</p></body></html>`, title)
			return
		}
		entry = again.Entry
	}
	fmt.Fprintf(rw, `<!doctype html><html lang="en"><head><meta charset="utf-8"><title>%s</title>`+
		`<meta http-equiv="refresh" content="0; url=%s"></head><body></body></html>`, title, html.EscapeString(entry))
}

func (w *documentWindow) startup(ctx context.Context) {
	w.mu.Lock()
	w.ctx = ctx
	w.mu.Unlock()
	go w.listen(ctx)
}

// listen asks the service, over and over, what it wants of this window.
// The asking is also how the service knows the window is still here. When
// the service says the document is closed, or cannot be reached for a
// while, the window has nothing behind it and says so.
func (w *documentWindow) listen(ctx context.Context) {
	var after int64
	var failing time.Time
	for {
		commands, err := w.client.Next(ctx, w.id, after)
		w.mu.Lock()
		closing := w.closing
		w.mu.Unlock()
		if closing {
			return
		}
		if err != nil {
			if failing.IsZero() {
				failing = time.Now()
			}
			if errors.Is(err, docservice.ErrClosed) || time.Since(failing) > 5*time.Second {
				w.lost(ctx)
				return
			}
			time.Sleep(250 * time.Millisecond)
			continue
		}
		failing = time.Time{}
		for _, c := range commands {
			after = c.Seq
			w.obey(ctx, c)
		}
	}
}

// lost ends a window whose document the service no longer has. The page
// kept the unsaved text with the service as the person typed, so opening
// the document again offers it.
func (w *documentWindow) lost(ctx context.Context) {
	logf("documents", "error", "a document's window lost the document service")
	if !w.testing {
		wruntime.MessageDialog(ctx, wruntime.MessageDialogOptions{Type: wruntime.ErrorDialog, Title: documentApplications[w.application].Name,
			Message: "This window can no longer reach the document service, so it will close. What you had typed was kept as you typed it, and is offered again when you open the document."})
	}
	w.mu.Lock()
	w.closing = true
	w.mu.Unlock()
	wruntime.Quit(ctx)
}

// obey does one thing the service asked.
func (w *documentWindow) obey(ctx context.Context, c docservice.Command) {
	var data struct {
		Title  string `json:"title"`
		Folder string `json:"folder"`
		Name   string `json:"name"`
		PID    int    `json:"pid"`
	}
	json.Unmarshal(c.Data, &data)
	if w.testing {
		appendDocumentEntry(w.entryFile, documentEntry{ID: w.id, Application: w.application, PID: os.Getpid(), Event: c.Kind})
	}
	switch c.Kind {
	case "focus":
		if w.testing {
			return
		}
		wruntime.WindowUnminimise(ctx)
		wruntime.Show(ctx)
		focusOwnWindow(documentWindowClass)
	case "title":
		wruntime.WindowSetTitle(ctx, data.Title)
	case "close":
		// The toolkit asks beforeClose, as it does for the window's own
		// close button.
		wruntime.Quit(ctx)
	case "allow":
		allowForeground(data.PID)
		w.client.Reply(ctx, w.id, c.Seq, true)
	case "pick-open":
		chosen := ""
		if !w.testing {
			chosen, _ = wruntime.OpenFileDialog(ctx, wruntime.OpenDialogOptions{Title: "Open", DefaultDirectory: data.Folder})
		}
		w.client.Reply(ctx, w.id, c.Seq, chosen)
	case "pick-save":
		chosen := ""
		if !w.testing {
			chosen, _ = wruntime.SaveFileDialog(ctx, wruntime.SaveDialogOptions{Title: "Save As", DefaultDirectory: data.Folder, DefaultFilename: data.Name})
		}
		w.client.Reply(ctx, w.id, c.Seq, chosen)
	}
}

// beforeClose is asked when the person closes the window. The service has
// the page send what it still holds, and says whether to ask first: that is
// so for a changed document that has no file, which could not be opened
// again. A document that has a file closes without a question, and its
// unsaved text is offered when the file is next opened.
func (w *documentWindow) beforeClose(ctx context.Context) (prevent bool) {
	w.mu.Lock()
	if w.closing {
		w.mu.Unlock()
		return false
	}
	w.mu.Unlock()
	short, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ask, err := w.client.Closing(short, w.id)
	if err == nil && ask && !w.testing {
		// Windows' own question has Yes and No for its buttons whatever
		// names are given, so the question is one those two answer.
		leave, stay := "Close Without Saving", "Cancel"
		if goruntime.GOOS == "windows" {
			leave, stay = "Yes", "No"
		}
		chosen, derr := wruntime.MessageDialog(ctx, wruntime.MessageDialogOptions{Type: wruntime.QuestionDialog, Title: documentApplications[w.application].Name,
			Message: "This document has not been saved anywhere. Close it and lose what you typed?", Buttons: []string{leave, stay}, DefaultButton: stay, CancelButton: stay})
		if derr != nil || chosen != leave {
			return true
		}
	}
	w.mu.Lock()
	w.closing = true
	w.mu.Unlock()
	w.client.Closed(short, w.id)
	return false
}
