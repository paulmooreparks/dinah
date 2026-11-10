package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"studio/internal/applet"
	"studio/internal/docservice"
	"studio/internal/resource/localfs"
)

//go:embed applet_standalone.go
var standaloneAppletSource []byte

/* The stand-alone document applications, for the window and for agents
   (docs/design/standalone-documents.md). A stand-alone application is a
   program of its own with its own windows, so nothing here reaches into
   one: this program starts a window process for a file, and speaks to the
   document service's control interface, as any other program of the
   product would.

   An agent's action on a document open there names the application and the
   document, and never means "the window in front". The document's own
   window asks the person before the action runs, so the consent is given
   where the document is. */

// standaloneActTimeout is how long an action may wait, which includes the
// person's answer in the document's window.
const standaloneActTimeout = 10 * time.Minute

// OpenStandalone opens a file on the computer in a stand-alone application,
// in a window of its own, or brings forward the window that already shows
// it. The main window hands a document over with it.
func (a *App) OpenStandalone(application, path string) error {
	if _, ok := documentApplications[application]; !ok {
		return fmt.Errorf("there is no application called %q", application)
	}
	if path == "" || !filepath.IsAbs(path) {
		return errors.New("a document is handed over by its file's full path")
	}
	return startDocumentWindow(application, path, "")
}

// StandaloneApplications lists the stand-alone applications this build
// offers, for the window's menus.
func (a *App) StandaloneApplications() []StandaloneApplication {
	out := make([]StandaloneApplication, 0, len(documentApplications))
	for id, app := range documentApplications {
		out = append(out, StandaloneApplication{ID: id, Name: app.Name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// StandaloneApplication is one stand-alone application as a menu shows it.
type StandaloneApplication struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func standaloneApplicationIDs() []string {
	ids := make([]string, 0, len(documentApplications))
	for id := range documentApplications {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// standaloneWindowActions are the applets' actions that change which
// document a window shows. A call here is for one named document, so they
// are not offered: a file is opened with Open in a Stand-Alone Application,
// and the person closes their own windows.
var standaloneWindowActions = map[string]bool{"open-file": true, "new": true, "close": true}

// standaloneLiveAction finds an action of an application's applet that
// works on the open document, by its reference in the release
// (yavc.editor/replace@1), and returns the operation its applet knows it by.
func standaloneLiveAction(application, ref string) (string, error) {
	app, ok := documentApplications[application]
	if !ok {
		return "", fmt.Errorf("there is no application called %q", application)
	}
	data, err := fs.ReadFile(assets, "frontend/yavc/content/"+app.Applet+"/actions.json")
	if err != nil {
		return "", err
	}
	var m contentManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return "", err
	}
	var live []string
	for _, action := range m.Actions {
		_, rest, _ := strings.Cut(action.Ref, "/")
		name, _, _ := strings.Cut(rest, "@")
		if !action.Live || standaloneWindowActions[name] {
			continue
		}
		live = append(live, action.Ref)
		if action.Ref == ref {
			_, rest, _ := strings.Cut(ref, "/")
			operation, _, _ := strings.Cut(rest, "@")
			return operation, nil
		}
	}
	return "", fmt.Errorf("%s is not an action on an open document of %s; those are %s", ref, app.Name, strings.Join(live, ", "))
}

func (a *App) standaloneAppletBundle() (applet.Bundle, error) {
	ids, _ := json.Marshal(standaloneApplicationIDs())
	application := `{"type":"string","enum":` + string(ids) + `}`
	document := `{"type":"object","additionalProperties":false,"required":["document","application","name","path","changed"],"properties":{"document":{"type":"string"},"application":` + application + `,"name":{"type":"string"},"path":{"type":"string"},"changed":{"type":"boolean"}}}`
	actions := []applet.Action{
		{Ref: "dinah.standalone/list@1", Title: "List Stand-Alone Documents",
			Description: "List the documents open in the stand-alone document applications, each with the ID that Act on a Stand-Alone Document takes. A document with no file yet has an empty path. The list is empty when no stand-alone window is open.",
			Replay:      "pure", Capabilities: []string{"window.read"},
			InputSchema:  json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`),
			OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["documents"],"properties":{"documents":{"type":"array","items":` + document + `}}}`)},
		{Ref: "dinah.standalone/open@1", Title: "Open in a Stand-Alone Application",
			Description: "Open a file of your workspace, by its path there such as notes/plan.md, in a stand-alone application, in a window of its own. A file that is already open there has its window brought forward. The window appears a moment after the answer, and List Stand-Alone Documents then has it.",
			Replay:      "once", Capabilities: []string{"window.command", "workspace.read"},
			InputSchema:  json.RawMessage(`{"type":"object","additionalProperties":false,"required":["application","path"],"properties":{"application":` + application + `,"path":{"type":"string","minLength":1,"maxLength":4096}}}`),
			OutputSchema: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["application","path"],"properties":{"application":` + application + `,"path":{"type":"string"}}}`)},
		{Ref: "dinah.standalone/act@1", Title: "Act on a Stand-Alone Document",
			Description: "Run one of an application's actions on a document open in that stand-alone application. Give the application, the document's ID from List Stand-Alone Documents, the action by its reference (for the Editor, an action of yavc.editor that works on the open document, such as yavc.editor/document@1 to read it), and that action's own input. The person is asked in the document's own window before the action runs and may refuse. Nothing is saved to the file unless the action saves; an edit is left in the window as unsaved text, as the person's own typing is.",
			Replay:      "once", Capabilities: []string{"window.command"},
			InputSchema:  json.RawMessage(`{"type":"object","additionalProperties":false,"required":["application","document","action"],"properties":{"application":` + application + `,"document":{"type":"string","minLength":1,"maxLength":64},"action":{"type":"string","minLength":1,"maxLength":128},"input":{"type":"object"}}}`),
			OutputSchema: json.RawMessage(`{"type":"object","required":["output"],"properties":{"output":{}}}`)},
	}
	bundle := applet.Bundle{
		Manifest: applet.Manifest{ID: "dinah.standalone", Version: "1.0.0", APIVersion: 1, Title: "Stand-Alone Documents",
			Description: "The stand-alone document applications (" + strings.Join(standaloneNames(), ", ") + "): list the documents open in them, open a workspace file in one, and act on a document open there by naming the application and the document.", Actions: actions},
		Handlers: map[string]applet.Handler{}, Sources: map[string][]byte{"applet_standalone.go": standaloneAppletSource}}
	for _, action := range actions {
		ref := action.Ref
		bundle.Handlers[ref] = func(ctx context.Context, input string) (applet.Result, error) {
			return a.executeStandaloneApplet(ctx, ref, input)
		}
	}
	return bundle, nil
}

func standaloneNames() []string {
	var names []string
	for _, id := range standaloneApplicationIDs() {
		names = append(names, documentApplications[id].Name)
	}
	return names
}

// standaloneDocument is an open document as an agent sees it.
type standaloneDocument struct {
	Document    string `json:"document"`
	Application string `json:"application"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	Changed     bool   `json:"changed"`
}

// findDocumentService is the running document service, or an error when
// none runs, which means no stand-alone window is open. A test replaces it.
var findDocumentService = func() (*docservice.Client, error) { return docservice.Find(documentsDir()) }

func (a *App) executeStandaloneApplet(ctx context.Context, ref, input string) (applet.Result, error) {
	if _, ok := applet.InvocationFromContext(ctx); !ok {
		return applet.Result{}, errors.New("a host invocation is required")
	}
	switch ref {
	case "dinah.standalone/list@1":
		out := struct {
			Documents []standaloneDocument `json:"documents"`
		}{Documents: []standaloneDocument{}}
		client, err := findDocumentService()
		if err != nil {
			return builtinJSON(out)
		}
		docs, err := client.Documents(ctx)
		if err != nil {
			return applet.Result{}, err
		}
		for _, d := range docs {
			out.Documents = append(out.Documents, standaloneDocument{Document: d.ID, Application: d.Application, Name: d.Name, Path: d.Path, Changed: d.Changed})
		}
		return builtinJSON(out)
	case "dinah.standalone/open@1":
		var r struct {
			Application string `json:"application"`
			Path        string `json:"path"`
		}
		if err := json.Unmarshal([]byte(input), &r); err != nil {
			return applet.Result{}, err
		}
		folder, _, err := a.mainFolder(ctx)
		if err != nil {
			return applet.Result{}, err
		}
		clean, err := localfs.CleanPath(r.Path)
		if err != nil || clean == "." {
			return applet.Result{}, errors.New("give the file's path in your workspace, such as notes/plan.md")
		}
		// The file is read first, so one that is missing, hidden or a link
		// is refused here as the workspace's own reads refuse it.
		if _, err := folder.ReadText(ctx, r.Path); err != nil {
			return applet.Result{}, fmt.Errorf("%s cannot be opened: %w", r.Path, err)
		}
		if err := a.OpenStandalone(r.Application, filepath.Join(folder.Root, clean)); err != nil {
			return applet.Result{}, err
		}
		return builtinJSON(map[string]string{"application": r.Application, "path": r.Path})
	case "dinah.standalone/act@1":
		var r struct {
			Application string          `json:"application"`
			Document    string          `json:"document"`
			Action      string          `json:"action"`
			Input       json.RawMessage `json:"input"`
		}
		if err := json.Unmarshal([]byte(input), &r); err != nil {
			return applet.Result{}, err
		}
		operation, err := standaloneLiveAction(r.Application, r.Action)
		if err != nil {
			return applet.Result{}, err
		}
		client, err := findDocumentService()
		if err != nil {
			return applet.Result{}, errors.New("no stand-alone document is open")
		}
		docs, err := client.Documents(ctx)
		if err != nil {
			return applet.Result{}, err
		}
		found := false
		for _, d := range docs {
			if d.ID == r.Document {
				if d.Application != r.Application {
					return applet.Result{}, fmt.Errorf("that document is open in %s, not in %s", documentApplications[d.Application].Name, documentApplications[r.Application].Name)
				}
				found = true
				// The document's window comes forward to ask the person.
				// This program is the one in front, so it lets that
				// window's process do so.
				allowForeground(d.PID)
			}
		}
		if !found {
			return applet.Result{}, errors.New("no stand-alone document has that ID now; list them again")
		}
		if len(r.Input) == 0 {
			r.Input = json.RawMessage(`{}`)
		}
		request, err := json.Marshal(map[string]any{"operation": operation, "input": r.Input})
		if err != nil {
			return applet.Result{}, err
		}
		waiting, cancel := context.WithTimeout(ctx, standaloneActTimeout)
		defer cancel()
		raw, err := client.Invoke(waiting, r.Document, request, "An agent in "+product.Name)
		if err != nil {
			return applet.Result{}, err
		}
		var answer struct {
			Output json.RawMessage `json:"output"`
		}
		if err := json.Unmarshal(raw, &answer); err != nil || len(answer.Output) == 0 {
			answer.Output = json.RawMessage("null")
		}
		return applet.Result{OutputJSON: `{"output":` + string(answer.Output) + `}`}, nil
	}
	return applet.Result{}, fmt.Errorf("unknown action %s", ref)
}
