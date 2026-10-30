package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWorkspaceSkills reads the skills of a workspace's .devin/skills folder
// with their descriptions, and leaves out a folder with no SKILL.md.
func TestWorkspaceSkills(t *testing.T) {
	ws := t.TempDir()
	write := func(name, body string) {
		d := filepath.Join(ws, ".devin", "skills", name)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if body != "" {
			if err := os.WriteFile(filepath.Join(d, "SKILL.md"), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	write("edit-request", "---\nname: edit-request\ndescription: \"Turn a request into a written edit request.\"\nargument-hint: '[\"<document.md>\"] <the change, in your words>'\n---\n\n# Body\n")
	write("changelog-post", "---\ndescription: Publish the changelog to the wiki.\n---\n")
	write("empty", "")
	a := NewApp()
	a.settings.Workspace = ws
	got := a.WorkspaceSkills()
	if len(got) != 2 || got[0].Name != "changelog-post" || got[1].Description != "Turn a request into a written edit request." || got[0].Source != "workspace" ||
		got[1].Hint != `["<document.md>"] <the change, in your words>` || got[0].Hint != "" {
		t.Fatalf("got %+v", got)
	}
}
