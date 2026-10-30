package main

import (
	"reflect"
	"testing"
)

// Up in a chat's prompt box brings back that chat's prompts, and those of
// the chats it was continued from, but never another chat's.
func TestChatHistoryKeepsToOneChat(t *testing.T) {
	previous := configOverride
	configOverride = t.TempDir()
	t.Cleanup(func() { configOverride = previous })
	ws := t.TempDir()
	if err := writeTranscript(&conversationTranscript{SessionID: "first", Workspace: ws}); err != nil {
		t.Fatal(err)
	}
	if err := writeTranscript(&conversationTranscript{SessionID: "second", Workspace: ws, ParentSessionID: "first"}); err != nil {
		t.Fatal(err)
	}
	a := NewApp()
	a.remember(ws, "", "send", "from before chats were recorded", nil)
	a.remember(ws, "first", "send", "one", nil)
	a.remember(ws, "other", "send", "another chat's prompt", nil)
	a.remember(ws, "second", "steer", "two", nil)
	a.remember(ws, "second", "queue", "three", nil)

	for _, c := range []struct {
		id   string
		want []string
	}{
		{"second", []string{"one", "two", "three"}},
		{"first", []string{"one"}},
		{"other", []string{"another chat's prompt"}},
		{"new", []string{}},
		{"", []string{}},
	} {
		got, err := a.ChatHistory(c.id)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("ChatHistory(%q) = %q, want %q", c.id, got, c.want)
		}
	}
}
