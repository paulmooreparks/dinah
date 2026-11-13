package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"studio/internal/applet"
)

// An agent reads a document and proposes new text; each changed paragraph
// becomes a suggestion beside the draft, and the document itself is not
// changed until the person accepts one.
func TestAnAgentsSuggestionsWaitForThePerson(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := closeBusWhenDone(t, NewApp())
	path := t.TempDir()
	if err := os.WriteFile(filepath.Join(path, "plan.md"), []byte("# Plan\n\nWe ship in March.\n\nRisks are low.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetWorkspace(path); err != nil {
		t.Fatal(err)
	}
	bundle, err := a.documentsAppletBundle()
	if err != nil {
		t.Fatal(err)
	}
	r, err := applet.New(bundle)
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range bundle.Manifest.Actions {
		if applet.Asks(applet.Action{Capabilities: action.Capabilities}) {
			t.Fatalf("%s asks first, though it changes nothing until the person accepts it", action.Ref)
		}
	}
	ctx := builtinTestContext(path)
	res, err := r.Execute(ctx, documentsReadAction, `{"path":"plan.md"}`)
	if err != nil {
		t.Fatal(err)
	}
	var read struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(res.OutputJSON), &read); err != nil || read.Text == "" {
		t.Fatalf("read %q %v", res.OutputJSON, err)
	}
	proposed := "# Plan\n\nWe ship in April.\n\nRisks are moderate.\n"
	in, _ := json.Marshal(map[string]any{"path": "plan.md", "base": read.Text, "text": proposed, "reason": "The schedule moved."})
	res, err = r.Execute(ctx, documentsSuggestAction, string(in))
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Added       int `json:"added"`
		Suggestions []struct {
			ID, Before, After, Author string
		} `json:"suggestions"`
	}
	if err := json.Unmarshal([]byte(res.OutputJSON), &out); err != nil || out.Added != 2 || out.Suggestions[0].Author == "" {
		t.Fatalf("suggested %s %v", res.OutputJSON, err)
	}
	data, _ := os.ReadFile(filepath.Join(path, "plan.md"))
	if string(data) != "# Plan\n\nWe ship in March.\n\nRisks are low.\n" {
		t.Fatalf("a suggestion changed the file: %q", data)
	}
	res, err = r.Execute(ctx, documentsReadAction, `{"path":"plan.md"}`)
	if err != nil {
		t.Fatal(err)
	}
	var again struct {
		Text        string            `json:"text"`
		Suggestions []json.RawMessage `json:"suggestions"`
	}
	if err := json.Unmarshal([]byte(res.OutputJSON), &again); err != nil || again.Text != read.Text || len(again.Suggestions) != 2 {
		t.Fatalf("the draft after suggesting %s %v", res.OutputJSON, err)
	}
}

// A document of another attached source is read and proposed to by that
// source's ID, and a source the workspace does not have is refused.
func TestAnAgentSuggestsToADocumentOfAnotherSource(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := closeBusWhenDone(t, NewApp())
	path, other := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(other, "page.md"), []byte("# Page\n\nOld words.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetWorkspace(path); err != nil {
		t.Fatal(err)
	}
	var id string
	for _, r := range a.settings.WorkspaceRecords {
		for _, s := range r.Sources {
			if s.Path == path {
				id = r.ID
			}
		}
	}
	record, err := a.AttachLocalWorkspaceSource(id, other, "Other")
	if err != nil {
		t.Fatal(err)
	}
	var sourceID string
	for _, s := range record.Sources {
		if s.Path == other {
			sourceID = s.ID
		}
	}
	if sourceID == "" || sourceID == record.PrimarySourceID {
		t.Fatalf("the second source: %+v", record.Sources)
	}
	bundle, err := a.documentsAppletBundle()
	if err != nil {
		t.Fatal(err)
	}
	r, err := applet.New(bundle)
	if err != nil {
		t.Fatal(err)
	}
	ctx := builtinTestContext(path)
	if _, err := r.Execute(ctx, documentsReadAction, `{"path":"page.md"}`); err == nil {
		t.Fatal("the other source's document was read from the main folder")
	}
	in, _ := json.Marshal(map[string]any{"path": "page.md", "sourceId": sourceID})
	res, err := r.Execute(ctx, documentsReadAction, string(in))
	if err != nil {
		t.Fatal(err)
	}
	var read struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(res.OutputJSON), &read); err != nil || read.Text != "# Page\n\nOld words.\n" {
		t.Fatalf("read %q %v", res.OutputJSON, err)
	}
	in, _ = json.Marshal(map[string]any{"path": "page.md", "sourceId": sourceID, "base": read.Text, "text": "# Page\n\nNew words.\n"})
	res, err = r.Execute(ctx, documentsSuggestAction, string(in))
	var out struct {
		Added int `json:"added"`
	}
	if err != nil || json.Unmarshal([]byte(res.OutputJSON), &out) != nil || out.Added != 1 {
		t.Fatalf("suggested %s %v", res.OutputJSON, err)
	}
	if data, _ := os.ReadFile(filepath.Join(other, "page.md")); string(data) != "# Page\n\nOld words.\n" {
		t.Fatalf("a suggestion changed the file: %q", data)
	}
	in, _ = json.Marshal(map[string]any{"path": "page.md", "sourceId": "no-such-source"})
	if _, err := r.Execute(ctx, documentsReadAction, string(in)); err == nil {
		t.Fatal("a source the workspace does not have was read")
	}
}

// An agent attaches a file of the workspace to a document's draft before it
// proposes text that refers to it. The file is kept with the draft, the
// document can show it at once, and it is written beside the document only
// when the person saves a text that refers to it.
func TestAnAgentAttachesAFileItsSuggestionRefersTo(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := closeBusWhenDone(t, NewApp())
	path := t.TempDir()
	os.MkdirAll(filepath.Join(path, "notes"), 0o755)
	os.MkdirAll(filepath.Join(path, "diagrams"), 0o755)
	if err := os.WriteFile(filepath.Join(path, "notes", "plan.md"), []byte("# Plan\n\nThe flow is below.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	canvas := `{"nodes":[{"id":"a","type":"text","text":"Start","x":0,"y":0,"width":100,"height":40}],"edges":[]}`
	if err := os.WriteFile(filepath.Join(path, "diagrams", "flow.canvas"), []byte(canvas), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetWorkspace(path); err != nil {
		t.Fatal(err)
	}
	bundle, err := a.documentsAppletBundle()
	if err != nil {
		t.Fatal(err)
	}
	r, err := applet.New(bundle)
	if err != nil {
		t.Fatal(err)
	}
	ctx := builtinTestContext(path)
	res, err := r.Execute(ctx, documentsAttachAction, `{"path":"notes/plan.md","file":"diagrams/flow.canvas"}`)
	if err != nil {
		t.Fatal(err)
	}
	var kept struct {
		Name  string `json:"name"`
		Bytes int    `json:"bytes"`
	}
	if err := json.Unmarshal([]byte(res.OutputJSON), &kept); err != nil || kept.Name != "flow.canvas" || kept.Bytes != len(canvas) {
		t.Fatalf("attached %s %v", res.OutputJSON, err)
	}
	// The document's window finds the file among those its draft keeps.
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		t.Fatal(err)
	}
	all, err := o.Drafts.Store.List()
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	for _, d := range all {
		if d.Origin.Path == "notes/plan.md" {
			found = append(found, d.ID)
		}
	}
	if len(found) != 1 {
		t.Fatalf("the document's drafts: %v", found)
	}
	text, err := a.DraftKeptText(found[0], "flow.canvas", 0)
	if err != nil || !text.Found || text.Text != canvas {
		t.Fatalf("the kept file as the window reads it: %+v %v", text, err)
	}
	if text, err := a.DraftKeptText(found[0], "other.canvas", 0); err != nil || text.Found {
		t.Fatalf("a file the draft does not keep: %+v %v", text, err)
	}
	// Nothing is written beside the document by attaching.
	if _, err := os.Stat(filepath.Join(path, "notes", "flow.canvas")); err == nil {
		t.Fatal("attaching wrote the file beside the document")
	}
	// A file that is not in the workspace, and one outside it, are refused.
	for _, file := range []string{"diagrams/none.canvas", "../outside.canvas"} {
		in, _ := json.Marshal(map[string]any{"path": "notes/plan.md", "file": file})
		if _, err := r.Execute(ctx, documentsAttachAction, string(in)); err == nil {
			t.Fatalf("%s was attached", file)
		}
	}
}
