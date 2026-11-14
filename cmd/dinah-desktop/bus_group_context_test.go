package main

import (
	"reflect"
	"testing"
)

func TestBusGroupMessageContextBridge(t *testing.T) {
	a, _, s, _, _ := busChat(t)
	if _, err := a.RenameSession(s.ID(), "Fixture"); err != nil {
		t.Fatal(err)
	}
	g, err := a.BusStartGroup("Context fixture", []BusChatRef{{ID: s.ID(), Cwd: s.Cwd(), Title: "Fixture"}})
	if err != nil {
		t.Fatal(err)
	}
	var sent []int64
	for i := 0; i < 30; i++ {
		m, err := a.BusSendToGroup(g.Name, "Invented context.", nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		sent = append(sent, m.ID)
	}
	got, err := a.BusGroupMessageContext(g.Name, sent[8])
	if err != nil {
		t.Fatal(err)
	}
	ids := []int64{}
	for _, m := range got {
		ids = append(ids, m.ID)
	}
	if !reflect.DeepEqual(ids, sent[3:8]) {
		t.Fatalf("old target context: %v", ids)
	}
	all, err := a.BusGroupMessages(g.Name)
	if err != nil {
		t.Fatal(err)
	}
	got, err = a.BusGroupMessageContext(g.Name, all[2].ID)
	if err != nil || len(got) != 2 || got[0].ID != all[0].ID || got[1].ID != all[1].ID {
		t.Fatalf("early context: %+v %v", got, err)
	}
	got, err = a.BusGroupMessageContext(g.Name, all[0].ID)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty context: %+v %v", got, err)
	}
	other, err := a.BusStartGroup("Other fixture", []BusChatRef{{ID: s.ID(), Cwd: s.Cwd(), Title: "Fixture"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.BusGroupMessageContext(other.Name, sent[8]); err == nil {
		t.Fatal("wrong group target accepted")
	}
	if pageMethods["BusGroupMessageContext"] != pageQuery {
		t.Fatal("context is not a page query")
	}
}
