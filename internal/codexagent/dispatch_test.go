package codexagent

import (
	"context"
	"testing"
	"time"

	"studio/internal/acp"
)

func TestPromptDispatchReportsNativeTurnStartBeforeLostResult(t *testing.T) {
	observed := make(chan struct{})
	c := fake(t, nil, func(m message, send func(message)) {
		if m.Method != "turn/start" {
			t.Error(m.Method)
			return
		}
		<-observed
		send(message{ID: m.ID, Error: &acp.Error{Code: acp.CodeInternal, Message: "invented consumed request with lost result"}})
	})
	c.sessions["codex:thread"] = &session{}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := c.PromptWithDispatch(ctx, acp.PromptParams{SessionID: "codex:thread", Prompt: []acp.ContentBlock{{Type: "text", Text: "invented"}}}, nil, func() { close(observed) })
	if err == nil {
		t.Fatal("lost result claimed success")
	}
	select {
	case <-observed:
	default:
		t.Fatal("native dispatch was not observed")
	}
	if c.Context().Err() != nil {
		t.Fatal("one owned result failure closed the shared provider")
	}
}
