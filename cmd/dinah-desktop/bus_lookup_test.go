package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBusGroupLookupCanonicalIdentity(t *testing.T) {
	a, _, s, _, _ := busChat(t)
	if _, err := a.RenameSession(s.ID(), "Lead"); err != nil {
		t.Fatal(err)
	}
	g, err := a.BusStartGroup("", []BusChatRef{{ID: s.ID(), Cwd: s.Cwd(), Title: "Lead"}, {ID: "closed-chat", Cwd: t.TempDir(), Title: "Other"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.BusRenameGroup(g.Name, "Lookup current title"); err != nil {
		t.Fatal(err)
	}
	for _, address := range []string{g.Name, g.Title, "Lookup current title"} {
		got, err := a.BusResolveGroup(address)
		if err != nil || got.State != "resolved" || got.Room == nil || got.Room.Name != g.Name || got.Room.Title != "Lookup current title" {
			t.Fatalf("%q: %+v %v", address, got, err)
		}
	}
	if err := a.BusDeleteGroup(g.Name); err != nil {
		t.Fatal(err)
	}
	for _, address := range []string{g.Name, "unknown room"} {
		got, err := a.BusResolveGroup(address)
		encoded, _ := json.Marshal(got)
		if err != nil || got.State != "not-found" || strings.Contains(string(encoded), "room") {
			t.Fatalf("%q: %s %v", address, encoded, err)
		}
	}
}
