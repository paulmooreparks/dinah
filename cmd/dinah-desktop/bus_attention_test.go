package main

import (
	"reflect"
	"testing"
)

func TestGroupAttentionPreviewAndSendBridge(t *testing.T) {
	a, _, s, _, _ := busChat(t)
	if _, err := a.RenameSession(s.ID(), "Alpha"); err != nil {
		t.Fatal(err)
	}
	g, err := a.BusStartGroup("Attention", []BusChatRef{{ID: s.ID(), Cwd: s.Cwd(), Title: "Alpha"}, {ID: "closed-attention", Cwd: t.TempDir(), Title: "Beta"}})
	if err != nil {
		t.Fatal(err)
	}
	empty, err := a.BusGroupReach(g.Name, nil)
	if err != nil || empty.Reason != "everyone" || len(empty.Reach) != 2 {
		t.Fatalf("preview %+v %v", empty, err)
	}
	settings, err := a.BusGroupAttentionSettings()
	if err != nil || settings.Limit != 3 || settings.Minutes != 30 || settings.LimitMin != 1 || settings.LimitMax != 10 {
		t.Fatalf("settings %+v %v", settings, err)
	}
	if err := a.SetBusGroupAttentionSettings(2, 15); err != nil {
		t.Fatal(err)
	}
	if err := a.SetBusGroupAttentionSettings(11, 15); err == nil {
		t.Fatal("invalid settings accepted")
	}
	member := empty.Reach[0].ID
	explicit, err := a.BusGroupReach(g.Name, []string{member})
	if err != nil || explicit.Reason != "explicit" || len(explicit.Reach) != 1 {
		t.Fatalf("explicit %+v %v", explicit, err)
	}
	sent, err := a.BusSendToGroup(g.Name, "Question", []string{member}, nil)
	if err != nil || sent.Reach == nil || !reflect.DeepEqual(sent.Reach.Reach, explicit.Reach) {
		t.Fatalf("sent %+v %v", sent, err)
	}
	follow, err := a.BusGroupReach(g.Name, nil)
	if err != nil || follow.Reason != "engaged" || follow.Expires == "" || len(follow.Engaged) != 1 {
		t.Fatalf("follow %+v %v", follow, err)
	}
	message, err := a.BusSendToGroup(g.Name, "Followup", nil, nil)
	if err != nil || message.Reach == nil || message.Reach.Reason != "engaged" || message.Reach.Reach[0].ID != member {
		t.Fatalf("follow send %+v %v", message, err)
	}
	if err := a.BusRemoveFromGroup(g.Name, member); err != nil {
		t.Fatal(err)
	}
	history, err := a.BusGroupMessages(g.Name)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range history {
		if m.ID == sent.ID {
			if !reflect.DeepEqual(m.Reach, sent.Reach) {
				t.Fatalf("historical routing changed %+v", m)
			}
			return
		}
	}
	t.Fatal("sent message absent from history")
}
