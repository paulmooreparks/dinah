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
