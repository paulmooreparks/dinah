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
	if applet.Asks(applet.Action{Capabilities: bundle.Manifest.Actions[1].Capabilities}) {
		t.Fatal("proposing a change asks first, though it changes nothing until the person accepts it")
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
