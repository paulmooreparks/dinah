package main

import (
	"strings"
	"testing"

	"studio/internal/workspaces"
)

// Pins are kept in the person's order: pinning an item twice keeps one pin,
// a pin moves to the place asked for, unpinning takes it off, a pin into
// another workspace or an unknown kind is refused, and detaching a source
// takes its pins with it.
func TestWorkspacePins(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := closeBusWhenDone(t, NewApp())
	state, err := a.SetWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	record := state.WorkspaceRecords[0]
	source := record.Sources[0].ID
	keys := func(r WorkspaceRecord) string {
		out := []string{}
		for _, p := range r.Pins {
			out = append(out, workspaces.PinKey(p))
		}
		return strings.Join(out, " ")
	}
	pin := func(p WorkspacePin) WorkspaceRecord {
		t.Helper()
		r, err := a.PinWorkspaceItem(record.ID, p)
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	pin(WorkspacePin{Kind: "chat", Title: "Design notes", Session: "s1"})
	pin(WorkspacePin{Kind: "chat", Title: "Planning", Session: "s2"})
	r := pin(WorkspacePin{Kind: "resource", Title: "Note 09", Ref: &ResourceRef{WorkspaceID: record.ID, SourceID: source, Path: "note09.md"}, ItemKind: "file", Session: "ignored"})
	if keys(r) != "chat:s1 chat:s2 resource:"+source+"/note09.md" || r.Pins[2].Session != "" {
		t.Fatalf("pins %+v", r.Pins)
	}
	if r = pin(WorkspacePin{Kind: "chat", Title: "Design notes, renamed", Session: "s1"}); len(r.Pins) != 3 || r.Pins[0].Title != "Design notes, renamed" {
		t.Fatalf("a second pin of one chat: %+v", r.Pins)
	}
	if r, err = a.MoveWorkspacePin(record.ID, "resource:"+source+"/note09.md", 0); err != nil || keys(r) != "resource:"+source+"/note09.md chat:s1 chat:s2" {
		t.Fatalf("move %v %v", keys(r), err)
	}
	if r, err = a.UnpinWorkspaceItem(record.ID, "chat:s1"); err != nil || keys(r) != "resource:"+source+"/note09.md chat:s2" {
		t.Fatalf("unpin %v %v", keys(r), err)
	}
	if _, err = a.PinWorkspaceItem(record.ID, WorkspacePin{Kind: "resource", Title: "Elsewhere", Ref: &ResourceRef{WorkspaceID: "other", SourceID: source, Path: "x.md"}}); err == nil {
		t.Fatal("a pin into another workspace was taken")
	}
	if _, err = a.PinWorkspaceItem(record.ID, WorkspacePin{Kind: "terminal", Title: "Shell"}); err == nil {
		t.Fatal("an unknown kind was pinned")
	}
	if got := keys(a.State().WorkspaceRecords[0]); got != "resource:"+source+"/note09.md chat:s2" {
		t.Fatalf("the pins were not kept: %s", got)
	}
	if r, err = a.DetachWorkspaceSource(record.ID, source); err != nil || keys(r) != "chat:s2" {
		t.Fatalf("detaching kept the source's pins: %v %v", keys(r), err)
	}
}
