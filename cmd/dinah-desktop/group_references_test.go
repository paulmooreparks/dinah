package main

import (
	"strings"
	"testing"
)

// A group message's references never change who receives it
// (docs/design/group-mentions.md). Only the members it mentions are asked;
// a reference to a member chat, a window or an applet goes with the text and
// asks no one, even when the referred chat is itself a member.
func TestGroupReferencesNeverChangeWhoReceivesAMessage(t *testing.T) {
	a, _, s, _, _ := busChat(t)
	if _, err := a.RenameSession(s.ID(), "Alpha"); err != nil {
		t.Fatal(err)
	}
	g, err := a.BusStartGroup("References", []BusChatRef{{ID: s.ID(), Cwd: s.Cwd(), Title: "Alpha"}, {ID: "closed-reference", Cwd: t.TempDir(), Title: "Beta"}})
	if err != nil {
		t.Fatal(err)
	}
	everyone, err := a.BusGroupReach(g.Name, nil)
	if err != nil || len(everyone.Reach) != 2 {
		t.Fatalf("reach %+v %v", everyone, err)
	}
	var beta string
	for _, m := range everyone.Reach {
		if m.ID != s.ID() {
			beta = m.ID
		}
	}
	if beta == "" {
		t.Fatalf("no second member in %+v", everyone.Reach)
	}
	refs := []Attachment{
		{Kind: "chat", SessionID: s.ID(), Name: "Alpha"},
		{Kind: "window", Window: "files", Name: "Files"},
	}
	sent, err := a.BusSendToGroup(g.Name, "Beta, see @Alpha and @Files.", []string{beta}, refs)
	if err != nil {
		t.Fatal(err)
	}
	if sent.Reach == nil || len(sent.Reach.Reach) != 1 || sent.Reach.Reach[0].ID != beta {
		t.Fatalf("a reference changed who the message reaches: %+v", sent.Reach)
	}
	if !strings.Contains(sent.Text, "Beta, see @Alpha and @Files.") {
		t.Fatalf("the message lost its words: %q", sent.Text)
	}
	// With no member mentioned, references leave the group's own rule in charge.
	plain, err := a.BusGroupReach(g.Name, nil)
	if err != nil {
		t.Fatal(err)
	}
	only, err := a.BusSendToGroup(g.Name, "Context only: @Files.", nil, []Attachment{{Kind: "window", Window: "files", Name: "Files"}})
	if err != nil {
		t.Fatal(err)
	}
	if only.Reach == nil || only.Reach.Reason != plain.Reason || len(only.Reach.Reach) != len(plain.Reach) {
		t.Fatalf("references changed the default reach: sent %+v, preview %+v", only.Reach, plain)
	}
}
