package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A draft's text comes back as the editor's document, with source blocks
// drawn as the preview draws them; the edited document comes back as text
// in which only the edited block changed; and an edited source block is
// drawn again.
func TestPageDocument(t *testing.T) {
	ws := t.TempDir()
	text := "# Notes\n\nSee **this**.\n\n```\ncode\n```\n"
	os.WriteFile(filepath.Join(ws, "notes.md"), []byte(text), 0o644)
	a := NewApp()
	record := newWorkspaceRecord(ws)
	a.settings.WorkspaceRecords = []WorkspaceRecord{record}
	v, err := a.OpenDraft(ResourceRef{WorkspaceID: record.ID, SourceID: record.Sources[0].ID, Path: "notes.md"})
	if err != nil {
		t.Fatal(err)
	}

	doc, err := a.PageDocumentOf(v.ID, text)
	if err != nil || len(doc.Content) != 3 {
		t.Fatalf("%+v %v", doc, err)
	}
	if p := doc.Content[1]; p.Type != "paragraph" || len(p.Content) != 3 || p.Content[1].Marks[0].Type != "strong" {
		t.Fatalf("paragraph %+v", p)
	}
	code := doc.Content[2]
	if code.Type != "source_block" || !strings.Contains(code.Attrs["html"].(string), "code") {
		t.Fatalf("code %+v", code)
	}

	doc.Content[1].Content[0].Text = "Read "
	raw, _ := json.Marshal(doc)
	md, err := a.PageDocumentMarkdown(raw)
	if err != nil || md != strings.Replace(text, "See ", "Read ", 1) {
		t.Fatalf("%q %v", md, err)
	}

	// A code block Insert puts in is its source.
	if blocks, err := a.PageBlocksOf(v.ID, "```\ncode\n```"); err != nil || len(blocks) != 1 || blocks[0].Type != "source_block" || blocks[0].Attrs["src"] != "```\ncode\n```" {
		t.Fatalf("code: %+v %v", blocks, err)
	}

	src, err := a.PageSourceOf(v.ID, "```\nother\n```")
	if err != nil || !strings.Contains(src.HTML, "other") || strings.Contains(src.HTML, "dinah-src") {
		t.Fatalf("%+v %v", src, err)
	}
}
