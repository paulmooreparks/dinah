package hooks

import (
	"context"
	"encoding/json"
	"io"

	"studio/internal/bus"
	"studio/internal/mcpserver"
	"studio/internal/providers"
)

// Steering is the command hook of the provider whose row declares a hook
// namespace. It shares the delivery implementation
// with Claude's hosted MCP hook without registering a second bus session.
func Steering(ctx context.Context, b *bus.Bus, r io.Reader, w io.Writer) error {
	var in struct {
		SessionID string `json:"session_id"`
		PromptID  string `json:"prompt_id"`
		Event     string `json:"hook_event_name"`
		Prompt    string `json:"prompt"`
	}
	if err := json.NewDecoder(io.LimitReader(r, 4<<20)).Decode(&in); err != nil {
		return err
	}
	if in.SessionID == "" {
		_, err := io.WriteString(w, "{}\n")
		return err
	}
	out := mcpserver.HostedTurnHook(ctx, b, providers.CommandHookNamespace()+in.SessionID, in.Event, in.Prompt, in.PromptID)
	_, err := io.WriteString(w, out+"\n")
	return err
}
