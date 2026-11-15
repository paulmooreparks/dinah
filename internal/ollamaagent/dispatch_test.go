package ollamaagent

import (
	"context"
	"testing"

	"studio/internal/acp"
)

func TestPromptDispatchHasCancellableOwnedTurn(t *testing.T) {
	c, _, _, _, id := windowChat(t)
	observed := 0
	var result acp.PromptResult
	err := c.PromptWithDispatch(context.Background(), acp.PromptParams{SessionID: id, Prompt: []acp.ContentBlock{{Type: "text", Text: "invented"}}}, &result, func() {
		observed++
		if err := c.Notify("session/cancel", acp.CancelParams{SessionID: id}); err != nil {
			t.Error(err)
		}
	})
	if err != nil || observed != 1 || result.StopReason != "cancelled" {
		t.Fatalf("observed=%d result=%+v err=%v", observed, result, err)
	}
	c.mu.Lock()
	busy := c.sessions[id].busy
	c.mu.Unlock()
	if busy {
		t.Fatal("cancelled turn retained its reservation")
	}
	if err := c.Call(context.Background(), "session/prompt", acp.PromptParams{SessionID: id, Prompt: []acp.ContentBlock{{Type: "text", Text: "next invented turn"}}}, nil); err != nil {
		t.Fatal(err)
	}
}
