package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A file in a workspace's local folder, attached with +, goes to the agent
// as a link to the file where it is, and one outside the folder is refused.
func TestAWorkspaceFileAttachedWithPlusIsLinkedWhereItIs(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := closeBusWhenDone(t, NewApp())
	dir := t.TempDir()
	state, err := a.SetWorkspace(dir)
	if err != nil {
		t.Fatal(err)
	}
	record := state.WorkspaceRecords[0]
	os.MkdirAll(filepath.Join(dir, "review"), 0o755)
	os.WriteFile(filepath.Join(dir, "review", "findings.md"), []byte("# Findings\n"), 0o644)
	ref := ResourceRef{WorkspaceID: record.ID, SourceID: record.Sources[0].ID, Path: "review/findings.md"}
	blocks, err := a.blocks(newSessionState(&sessionSeed{Cwd: dir}), "Look at this", []Attachment{{Kind: "resource", Name: "findings.md", Ref: &ref}})
	if err != nil {
		t.Fatal(err)
	}
	last := blocks[len(blocks)-1]
	if len(blocks) != 2 || last.Type != "resource_link" || !strings.HasSuffix(last.URI, "/review/findings.md") || !strings.HasPrefix(last.URI, "file:///") {
		t.Fatalf("%+v", blocks)
	}
	bad := ResourceRef{WorkspaceID: record.ID, SourceID: record.Sources[0].ID, Path: "../outside.md"}
	if _, err := a.blocks(newSessionState(&sessionSeed{Cwd: dir}), "x", []Attachment{{Kind: "resource", Name: "outside", Ref: &bad}}); err == nil {
		t.Fatal("a path outside the folder was attached")
	}
}

// A document open in the Editor, mentioned with @, is told to the agent; a
// draft ID that is not one is refused.
func TestAnOpenDocumentMentionedWithAtIsToldToTheAgent(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := closeBusWhenDone(t, NewApp())
	id := strings.Repeat("ab", 16)
	blocks, err := a.blocks(newSessionState(&sessionSeed{Cwd: t.TempDir()}), "Check @Note_09", []Attachment{{Kind: "draft", Name: "Note 09", DraftID: id}})
	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 2 || !strings.Contains(blocks[1].Text, "open in the Editor") || !strings.Contains(blocks[1].Text, `"Note 09"`) {
		t.Fatalf("%+v", blocks)
	}
	if _, err := a.blocks(newSessionState(&sessionSeed{Cwd: t.TempDir()}), "x", []Attachment{{Kind: "draft", Name: "x", DraftID: "../../etc"}}); err == nil {
		t.Fatal("a malformed draft ID was accepted")
	}
}
