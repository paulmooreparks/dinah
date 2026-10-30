package main

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"studio/internal/diagram"
	"studio/internal/layout"
)

// openInBrowser opens a web or mail address in the person's browser. It is
// the one way the app opens an address, for the page and for Go alike, so
// no other kind of address, such as a file path or a network share, can be
// handed to the operating system this way.
func (a *App) openInBrowser(address string) error {
	u, err := url.Parse(address)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http" && u.Scheme != "mailto") {
		return errors.New("only a web or mail address can be opened in the browser")
	}
	if a.ctx == nil {
		return errors.New("addresses can be opened only from " + product.Name + "'s window")
	}
	wruntime.BrowserOpenURL(a.ctx, u.String())
	return nil
}

func managedExecutionRoot(path string) bool { return stateLayout().Managed(path) }

func managedExecutionRoots(records []WorkspaceRecord) []string {
	var roots []string
	for _, record := range records {
		if layout.ManagedID(record.ID) {
			path := stateLayout().ManagedRoot(record.ID)
			if layout.SamePath(record.ExecutionPath, path) {
				roots = append(roots, path)
			}
		}
	}
	return roots
}

// RenderD2 draws a d2 diagram from a message, for the chat. It is the same
// engine that draws d2 in the Editor's preview and draws flows.
func (a *App) RenderD2(src string) (string, error) {
	svg, err := diagram.Render(src)
	if err != nil {
		return "", err
	}
	return string(svg), nil
}

// openableExt are the kinds of file a link in a message may open with the
// program Windows has for them: documents, pictures and text. Anything else,
// such as a program or a script, is shown in Explorer instead, so that one
// click on a link in the agent's message never starts a program.
var openableExt = map[string]bool{
	".md": true, ".txt": true, ".log": true, ".csv": true, ".json": true, ".yaml": true, ".yml": true, ".toml": true, ".xml": true,
	".pdf": true, ".docx": true, ".doc": true, ".xlsx": true, ".xls": true, ".pptx": true, ".ppt": true, ".odt": true, ".ods": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true, ".webp": true, ".bmp": true,
	".eml": true, ".msg": true, ".d2": true, ".mmd": true,
}

// OpenPath opens a local file that a link in a message points to and that
// no part of Dinah's window opens (document_links.go): a document with the
// program Windows has for its kind, anything else selected in Explorer, and
// a folder in Explorer. It says which it did.
func (a *App) OpenPath(path string) (string, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	st, err := os.Stat(path)
	if err != nil {
		return "", errors.New("there is no file at " + path)
	}
	if st.IsDir() {
		return "folder", shellOpen(path)
	}
	if openableExt[strings.ToLower(filepath.Ext(path))] {
		return "opened", shellOpen(path)
	}
	return "shown", showInFolder(path)
}
