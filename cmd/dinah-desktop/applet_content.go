package main

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"time"

	"studio/internal/applet"
)

//go:embed applet_content.go
var contentAppletSource []byte

/* The yavc.app content applets' actions, for agents (docs/design/
   content-applets.md, decision 2). Each package of the release in
   frontend/yavc publishes its actions in its own actions.json; Dinah reads
   that file as it is and offers each action under its own reference, so an
   agent in a chat calls yavc.editor/insert@1 as the release describes it.
   The call goes to the page, which runs it in the window of that
   application the person last had in front in the chat's workspace,
   opening one where the action opens a document (content-agent.js).

   What an action may do in Dinah is Dinah's to say, so the capabilities
   the release declares are put in Dinah's own words, which decide whether
   the person is asked first: reading a file is reading the workspace,
   writing one or changing an open document (which Dinah saves to its file)
   is writing the workspace, and opening a window is a window command. */

// contentApps are the release's packages whose actions Dinah offers.
var contentApps = []string{"editor", "docs", "diagrams", "slides"}

// contentAskTimeout is how long a content action may run in the page; an
// export of a long document takes longer than the page's usual answer.
const contentAskTimeout = 3 * time.Minute

// contentNote is added to each package's description, for an agent reading
// the catalogue.
var contentNote = " In " + product.Name + " each document has a window of its own, so a live action works on the document of the window of this application most recently in front in your workspace, and index, where an action takes one, is always 1. open-file takes a path in your workspace, such as notes/plan.md, and opens it in a window of its own."

// contentManifest is one package's actions.json as the release has it.
type contentManifest struct {
	ID          string `json:"id"`
	Version     string `json:"version"`
	APIVersion  int    `json:"apiVersion"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Actions     []struct {
		applet.Action
		Live    bool   `json:"live"`
		Handles string `json:"handles"`
		// Changes is "document" for an action that changes a document's
		// text, saves it, or makes, closes or replaces one, and "none" for
		// one that does not, as the release says of every action of its
		// own. The release gives one or the other, so an action with
		// neither is taken to change a document.
		Changes string `json:"changes"`
	} `json:"actions"`
}

// proposeNote tells an agent, in a product whose agents propose their
// changes, where the actions that change a document went.
const proposeNote = " In this product an agent does not change a document itself, so the actions that change, save, make or close one are not offered here. Read a document with dinah.documents/read@1 and propose your changes with dinah.documents/suggest@1; the person accepts or rejects each one in the document's window."

// contentCapabilities says what an action of the release does, in Dinah's
// capabilities.
func contentCapabilities(declared []string, live bool, replay string) []string {
	var out []string
	add := func(c string) {
		for _, have := range out {
			if have == c {
				return
			}
		}
		out = append(out, c)
	}
	for _, c := range declared {
		switch c {
		case "files.read":
			add("workspace.read")
		case "files.write":
			add("workspace.write")
		case "window.open":
			add("window.command")
		case "artifact.create":
			add("artifact.create")
		}
	}
	// A live action that may change the document changes a file, since
	// Dinah saves an open document to its file as it changes.
	if live && replay != "pure" {
		add("workspace.write")
	}
	if live && replay == "pure" {
		add("window.read")
	}
	return out
}

func (a *App) contentAppletBundle(app string) (applet.Bundle, error) {
	name := "frontend/yavc/content/" + app + "/actions.json"
	data, err := fs.ReadFile(assets, name)
	if err != nil {
		return applet.Bundle{}, err
	}
	return a.contentBundleFrom(app, name, data, product.AgentsProposeDocumentChanges && suggestionApps[app])
}

// suggestionApps are the applications whose windows show an agent's
// suggestions for the person to accept or reject. Only there can an agent
// propose in place of changing, so only their actions are left out where a
// product's agents propose; a deck or a diagram has no suggestions yet.
var suggestionApps = map[string]bool{"editor": true, "docs": true}

// contentBundleFrom makes a package's bundle from its actions.json. With
// propose set, every action the release does not mark as leaving the
// document alone is left out, so that no agent is offered one and none can be called: what
// an agent may not do is absent, and the description says what to do
// instead.
func (a *App) contentBundleFrom(app, name string, data []byte, propose bool) (applet.Bundle, error) {
	var m contentManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return applet.Bundle{}, fmt.Errorf("%s: %w", name, err)
	}
	manifest := applet.Manifest{ID: m.ID, Version: m.Version, APIVersion: m.APIVersion, Title: m.Title, Description: m.Description + contentNote}
	handlers := map[string]applet.Handler{}
	left := false
	for _, action := range m.Actions {
		if propose && action.Changes != "none" {
			left = true
			continue
		}
		act := action.Action
		act.Capabilities = contentCapabilities(action.Capabilities, action.Live, action.Replay)
		manifest.Actions = append(manifest.Actions, act)
		ref, live, handles := act.Ref, action.Live, action.Handles
		handlers[ref] = func(ctx context.Context, input string) (applet.Result, error) {
			return a.runContentAction(ctx, app, ref, live, handles, input)
		}
	}
	if left {
		manifest.Description += proposeNote
	}
	return applet.Bundle{Manifest: manifest, Handlers: handlers,
		Sources: map[string][]byte{"applet_content.go": contentAppletSource, "content/" + app + "/actions.json": data}}, nil
}

// contentAnswer is the page's answer to a content action.
type contentAnswer struct {
	Output    json.RawMessage `json:"output"`
	Artifacts []struct {
		Key  string `json:"key"`
		Name string `json:"name"`
		MIME string `json:"mime"`
		Data string `json:"data"`
	} `json:"artifacts"`
}

func (a *App) runContentAction(ctx context.Context, app, ref string, live bool, handles, input string) (applet.Result, error) {
	ws, err := builtinWorkspace(ctx)
	if err != nil {
		return applet.Result{}, err
	}
	workspaceID := ""
	if vfs, err := a.scopedWorkspaceVFS(ctx); err == nil {
		if records := vfs.records(); len(records) == 1 {
			workspaceID = records[0].ID
		}
	}
	args := map[string]any{"app": app, "action": ref, "input": json.RawMessage(input), "live": live, "handles": handles,
		"workspace": ws, "workspaceId": workspaceID}
	out, err := a.askPageWithin(ctx, "content", args, contentAskTimeout)
	if err != nil {
		return applet.Result{}, err
	}
	var answer contentAnswer
	if err := json.Unmarshal([]byte(out), &answer); err != nil {
		return applet.Result{}, fmt.Errorf("the page's answer to %s could not be read: %w", ref, err)
	}
	result := applet.Result{OutputJSON: string(answer.Output)}
	for _, art := range answer.Artifacts {
		bytes, err := base64.StdEncoding.DecodeString(art.Data)
		if err != nil {
			return applet.Result{}, fmt.Errorf("%s's artifact %s could not be read: %w", ref, art.Key, err)
		}
		result.Artifacts = append(result.Artifacts, applet.Artifact{Key: art.Key, Name: art.Name, MIME: art.MIME, Data: bytes})
	}
	return result, nil
}
