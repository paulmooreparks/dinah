package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"studio/internal/acp"
)

// An agent's question about calling one of Dinah's own applet tools is
// answered by Dinah with "allow once" and never shown, since Dinah asks the
// person itself before the action has an effect; any other question is
// shown as before.
func TestDinahAnswersTheAgentsQuestionAboutItsOwnAppletTools(t *testing.T) {
	a := &App{}
	shown := make(chan string, 4)
	a.sink = func(name string, data any) {
		if name == "agent:permission" {
			shown <- data.(map[string]any)["key"].(string)
		}
	}
	options := `[{"optionId":"yes-always","kind":"allow_always","name":"Yes, always"},{"optionId":"yes","kind":"allow_once","name":"Yes"},{"optionId":"no","kind":"reject_once","name":"No"}]`
	for _, title := range []string{"mcp__dinah-applets__applet_discover", "mcp__dinah-applets__applet_invoke", "applet_status"} {
		params := json.RawMessage(`{"sessionId":"claude:test","toolCall":{"toolCallId":"t1","title":"` + title + `","kind":"other","rawInput":{"ref":"yavc.docs/insert@1"}},"options":` + options + `}`)
		r, err := a.newAgentHandler().Request(context.Background(), "session/request_permission", params)
		if err != nil {
			t.Fatal(err)
		}
		out := r.(acp.PermissionOutcome).Outcome
		if out.Outcome != "selected" || out.OptionID != "yes" {
			t.Fatalf("%s: %#v", title, out)
		}
	}
	select {
	case key := <-shown:
		t.Fatalf("a question about Dinah's own tool was shown: %s", key)
	case <-time.After(50 * time.Millisecond):
	}
	// Any other tool's question is the person's to answer.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		a.newAgentHandler().Request(ctx, "session/request_permission", json.RawMessage(`{"sessionId":"claude:test","toolCall":{"toolCallId":"t2","title":"Bash","rawInput":{"command":"rm -rf"}},"options":`+options+`}`))
		close(done)
	}()
	select {
	case <-shown:
	case <-time.After(time.Second):
		t.Fatal("a question about another tool was not shown")
	}
	cancel()
	<-done
	// An agent that offers no "allow once" is not answered for it.
	if _, ok := ownToolPermission(json.RawMessage(`{"title":"mcp__dinah-applets__applet_invoke"}`), []acp.PermissionOption{{OptionID: "always", Kind: "allow_always"}}); ok {
		t.Fatal("answered with no allow-once option")
	}
}
