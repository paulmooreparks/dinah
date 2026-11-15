package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestGroupReferenceMetadataUsesOriginalWordsAndSurvivesReload(t *testing.T) {
	a, _, s, _, _ := busChat(t)
	if _, err := a.RenameSession(s.ID(), "Fixture"); err != nil {
		t.Fatal(err)
	}
	g, err := a.BusStartGroup("Reference fixture", []BusChatRef{{ID: s.ID(), Cwd: s.Cwd(), Title: "Fixture"}})
	if err != nil {
		t.Fatal(err)
	}
	words := "Look at @Fixture and @Files."
	sent, err := a.BusSendToGroup(g.Name, words, nil, []Attachment{{Kind: "chat", Name: "Fixture", SessionID: s.ID(), Token: "@Fixture"}, {Kind: "window", Name: "Files", Window: "files", Token: "@Files"}})
	if err != nil {
		t.Fatal(err)
	}
	if sent.ShownText == nil || *sent.ShownText != words || len(sent.References) != 2 || sent.References[0].Token != "@Fixture" || sent.References[1].Kind != "window" || sent.References[0].Workspace == "" {
		t.Fatalf("missing presentation: %+v", sent)
	}
	if !strings.Contains(sent.Text, "The user mentioned the chat") || !strings.Contains(sent.Text, "window \"Files\"") {
		t.Fatalf("member context changed: %s", sent.Text)
	}
	loaded, err := a.BusGroupMessages(g.Name)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, m := range loaded {
		if m.ID == sent.ID {
			found = true
			if m.ShownText == nil || *m.ShownText != words || !reflect.DeepEqual(m.References, sent.References) || m.Text != sent.Text {
				t.Fatalf("reload changed presentation: %+v", m)
			}
		}
	}
	if !found {
		t.Fatal("sent message missing after reload")
	}
	empty, err := a.BusSendToGroup(g.Name, "", nil, []Attachment{{Kind: "window", Name: "Files", Window: "files"}})
	if err != nil {
		t.Fatal(err)
	}
	if empty.ShownText == nil || *empty.ShownText != "" || len(empty.References) != 1 || empty.References[0].Token != "" {
		t.Fatalf("empty original or Add reference lost: %+v", empty)
	}
}
