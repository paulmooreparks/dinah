package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/applet"
)

// The write actions write in the main folder only, refuse what the policy
// excludes, and replace a file only at the revision the caller read.
func TestWorkspaceWriteActions(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := closeBusWhenDone(t, NewApp())
	path := t.TempDir()
	if _, err := a.SetWorkspace(path); err != nil {
		t.Fatal(err)
	}
	bundle, err := a.workspaceAppletBundle()
	if err != nil {
		t.Fatal(err)
	}
	r, err := applet.New(bundle)
	if err != nil {
		t.Fatal(err)
	}
	ctx := builtinTestContext(path)
	run := func(action, input string) (map[string]any, error) {
		res, err := r.Execute(ctx, action, input)
		if err != nil {
			return nil, err
		}
		var out map[string]any
		return out, json.Unmarshal([]byte(res.OutputJSON), &out)
	}

	if _, err := run(workspaceMakeFolderAction, `{"path":"PARSER/design"}`); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(path, "PARSER", "design")); err != nil || !info.IsDir() {
		t.Fatalf("the folder was not made: %v", err)
	}
	if _, err := run(workspaceMakeFolderAction, `{"path":"PARSER/design"}`); err != nil {
		t.Fatalf("an existing folder was refused: %v", err)
	}
	written, err := run(workspaceWriteJSONAction, `{"path":"PARSER/project.json","value":{"code":"PARSER","projects":["PARSER-CLI"]}}`)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(path, "PARSER", "project.json"))
	if string(data) != "{\n  \"code\": \"PARSER\",\n  \"projects\": [\n    \"PARSER-CLI\"\n  ]\n}\n" {
		t.Fatalf("the JSON file %q", data)
	}
	if _, err := run(workspaceWriteFileAction, `{"path":"PARSER/project.json","text":"x"}`); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("a file was replaced without its revision: %v", err)
	}
	if _, err := run(workspaceWriteFileAction, `{"path":"PARSER/project.json","text":"x","revision":"`+strings.Repeat("0", 64)+`"}`); err == nil || !strings.Contains(err.Error(), "changed since") {
		t.Fatalf("a file was replaced at the wrong revision: %v", err)
	}
	if _, err := run(workspaceWriteFileAction, `{"path":"PARSER/project.json","text":"{}\n","revision":"`+written["revision"].(string)+`"}`); err != nil {
		t.Fatalf("a file was not replaced at its revision: %v", err)
	}
	// An agent's approved reply gives the value in its one fenced json block.
	reply, _ := json.Marshal("| Field | Value |\n| --- | --- |\n| code | ROADMAP |\n\n```json\n{\"code\": \"ROADMAP\"}\n```\n")
	if _, err := run(workspaceWriteJSONAction, `{"path":"ROADMAP/project.json","value":`+string(reply)+`}`); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(path, "ROADMAP", "project.json")); string(data) != "{\n  \"code\": \"ROADMAP\"\n}\n" {
		t.Fatalf("the reply's JSON %q", data)
	}
	for _, bad := range []string{`"no json here"`, `"` + "```json\\n{}\\n```\\n```json\\n{}\\n```" + `"`, `"42"`} {
		if _, err := run(workspaceWriteJSONAction, `{"path":"ROADMAP/bad.json","value":`+bad+`}`); err == nil {
			t.Fatalf("%s was written", bad)
		}
	}
	if _, err := run(workspaceWriteFileAction, `{"path":"PARSER/notes/README.md","text":"# Notes\n"}`); err != nil {
		t.Fatalf("a file in a new folder: %v", err)
	}
	for _, refused := range []string{".devin/x.md", "PARSER/AGENTS.md", "../outside.md", "PARSER/.hidden/x.md", "PARSER/AGENTS.md.", "drafts./x.md", "DRAFTS~1/y.md", "parser/agents.MD"} {
		if _, err := run(workspaceWriteFileAction, `{"path":"`+refused+`","text":"x"}`); err == nil {
			t.Fatalf("%s was written", refused)
		}
	}
	if _, err := os.Stat(filepath.Join(path, ".devin")); !os.IsNotExist(err) {
		t.Fatal("a refused write made a folder")
	}
}
