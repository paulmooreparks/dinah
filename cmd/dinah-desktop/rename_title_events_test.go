package main

import (
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestClosedChatRenameEmitsTitlesAfterSuccessfulSave(t *testing.T) {
	a, conn := providerTestApp(t)
	const id = "closed-without-recipient"
	if a.session(id) != nil || a.settings.BusSessions[id] != "" {
		t.Fatal("fixture has a chat window or recipient")
	}
	var events testCollector[map[string]string]
	a.sink = func(name string, data any) {
		if name != "agent:titles" {
			return
		}
		titles := data.(map[string]string)
		saved, err := loadSettings()
		if err != nil || !maps.Equal(saved.SessionTitles, titles) {
			t.Errorf("event preceded durable title save: %+v %v", saved.SessionTitles, err)
		}
		events.Add(titles)
	}
	for _, title := range []string{"Renamed closed chat", ""} {
		returned, err := a.RenameSession(id, title)
		if err != nil || len(events.Snapshot()) == 0 || !reflect.DeepEqual(returned, events.Snapshot()[len(events.Snapshot())-1]) {
			t.Fatalf("rename result %+v %v, events %+v", returned, err, events.Snapshot())
		}
	}
	if len(events.Snapshot()) != 2 || events.Snapshot()[0][id] != "Renamed closed chat" || events.Snapshot()[1][id] != "" {
		t.Fatalf("title events %+v", events.Snapshot())
	}
	if len(conn.calls.Snapshot()) != 0 {
		t.Fatal("renaming a closed chat contacted a provider")
	}
}

func TestFailedClosedChatRenameEmitsNoTitles(t *testing.T) {
	a, _ := providerTestApp(t)
	path := filepath.Join(configDir(), "settings.json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	events := 0
	a.sink = func(name string, _ any) {
		if name == "agent:titles" {
			events++
		}
	}
	if _, err := a.RenameSession("closed", "Must not appear"); err == nil {
		t.Fatal("fixture did not fail the settings write")
	}
	if events != 0 || a.settings.SessionTitles["closed"] != "" {
		t.Fatal("failed rename published or retained a title")
	}
}
