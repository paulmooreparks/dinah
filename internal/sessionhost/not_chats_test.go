package sessionhost

import "testing"

func TestNotChat(t *testing.T) {
	old := notChats
	t.Cleanup(func() { notChats = old })
	notChats = nil
	if NotChat("a") {
		t.Fatal("with nothing registered a session was called not a chat")
	}
	RegisterNotChat(func(id string) bool { return id == "a" })
	RegisterNotChat(func(id string) bool { return id == "b" })
	for id, want := range map[string]bool{"a": true, "b": true, "c": false} {
		if got := NotChat(id); got != want {
			t.Errorf("NotChat(%q) = %v, want %v", id, got, want)
		}
	}
}
