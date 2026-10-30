package main

import (
	"encoding/json"
	"strings"
	"testing"

	"studio/internal/applet"
)

func TestChatDirectoryFindsAcrossWorkspacesWithoutGuessing(t *testing.T) {
	a, ws, chats := askTestApp(t)
	other := t.TempDir()
	chats["B"] = replaceTestIdentity(a, chats["B"], "Cwd", other)
	setTestTitle(chats["A"], "Dinah Development")
	setTestTitle(chats["B"], "Dinah Development")
	a.settings.WorkspaceRecords = []WorkspaceRecord{{ID: "dinah", Name: "Dinah", ExecutionPath: ws}, {ID: "orch", Name: "Orchard Notes", ExecutionPath: other}}
	a.settings.SessionTitles = map[string]string{"chat-C": "Dinah Development Notes"}
	bundle, err := a.chatAppletBundle()
	if err != nil {
		t.Fatal(err)
	}
	r, err := applet.New(bundle)
	if err != nil {
		t.Fatal(err)
	}
	lookup := func(input string) struct {
		Chats     []chatDirectoryEntry `json:"chats"`
		Match     string               `json:"match"`
		Truncated bool                 `json:"truncated"`
	} {
		t.Helper()
		res, err := r.Execute(builtinTestContext(ws), chatFindAction, input)
		if err != nil {
			t.Fatal(err)
		}
		var out struct {
			Chats     []chatDirectoryEntry `json:"chats"`
			Match     string               `json:"match"`
			Truncated bool                 `json:"truncated"`
		}
		if err := json.Unmarshal([]byte(res.OutputJSON), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	if out := lookup(`{"name":"dinah development","limit":1}`); out.Match != "ambiguous" || !out.Truncated {
		t.Fatalf("duplicates were silently resolved: %+v", out)
	}
	if out := lookup(`{"name":"Dinah Development","workspace":"Orchard Notes"}`); out.Match != "unique" || len(out.Chats) != 1 || out.Chats[0].SessionID != "chat-B" || out.Chats[0].WorkspaceID != "orch" {
		t.Fatalf("cross-workspace lookup: %+v", out)
	}
	if out := lookup(`{"name":"Development Notes"}`); out.Match != "unique" || out.Chats[0].SessionID != "chat-C" {
		t.Fatalf("partial title lookup: %+v", out)
	}
	// A closed chat is found too, marked closed, since asking it opens it.
	chats["B"].Close()
	if out := lookup(`{"name":"Dinah Development","workspace":"orch"}`); out.Match != "unique" || len(out.Chats) != 1 || out.Chats[0].State != chatStateClosed {
		t.Fatalf("a closed chat was not found as closed: %+v", out)
	}
	if _, err := r.Execute(builtinTestContext(ws), chatFindAction, `{}`); err == nil {
		t.Fatal("a lookup without a name was accepted")
	}
}

func TestAskAndMentionCrossWorkspaceChat(t *testing.T) {
	a, ws, chats := askTestApp(t)
	chats["B"] = replaceTestIdentity(a, chats["B"], "Cwd", t.TempDir())
	out, err := askFrom(t, a, ws, "chat-A", "chat-B", "Share the renderer design.")
	if err != nil || out["reply"] != "Answer to: Share the renderer design." {
		t.Fatalf("cross-workspace answer: %v, %v", out, err)
	}
	blocks, err := a.blocks(chats["A"], "Ask Chat B", []Attachment{{Kind: "chat", SessionID: "chat-B", Name: "Chat B"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(blocks[len(blocks)-1].Text, "chat-B") || !strings.Contains(blocks[len(blocks)-1].Text, chatAskAction) {
		t.Fatalf("mention lost its explicit destination: %+v", blocks)
	}
}
