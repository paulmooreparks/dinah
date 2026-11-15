package main

import "testing"

func TestNewChatReturnsItsPublishedInitialState(t *testing.T) {
	a, _, ws := orgActionsFixture(t)
	info, err := a.NewSessionWithProviderAt(ws, "codex")
	if err != nil {
		t.Fatal(err)
	}
	if info.State == nil {
		t.Fatal("creation returned before publishing its initial state")
	}
	before := inventoryRuntimeHash(map[string]SessionInfo{info.ID: info})
	a.sendChatStates()
	after := a.session(info.ID).Info()
	if before != inventoryRuntimeHash(map[string]SessionInfo{info.ID: after}) {
		t.Fatal("the next publication changed the newly returned chat state")
	}
}
