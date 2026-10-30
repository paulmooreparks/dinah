package codexagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"studio/internal/acp"
	"studio/internal/profile"
)

// MCP tool approvals use the same documented request as option questions.
// Return the selected label under the exact question ID supplied by Codex.
func (c *Conn) userInput(ctx context.Context, params json.RawMessage) (any, *acp.Error) {
	var p struct {
		ThreadID  string `json:"threadId"`
		ItemID    string `json:"itemId"`
		Questions []struct {
			ID       string `json:"id"`
			Header   string `json:"header"`
			Question string `json:"question"`
			Secret   bool   `json:"isSecret"`
			Options  []struct {
				Label       string `json:"label"`
				Description string `json:"description"`
			} `json:"options"`
		} `json:"questions"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &acp.Error{Code: acp.CodeInternal, Message: "Invalid Codex question"}
	}
	answers := map[string]any{}
	c.mu.Lock()
	s := c.sessions["codex:"+p.ThreadID]
	if s != nil && s.requestContext != nil {
		ctx = s.requestContext
	}
	c.mu.Unlock()
	for _, q := range p.Questions {
		if q.ID == "" || q.Secret || len(q.Options) == 0 {
			return nil, &acp.Error{Code: acp.CodeMethodNotFound, Message: profile.Current().Name + " currently supports Codex questions with selectable options only"}
		}
		if _, exists := answers[q.ID]; exists {
			return nil, &acp.Error{Code: acp.CodeInternal, Message: "Codex repeated a question ID"}
		}
		answers[q.ID] = map[string]any{"answers": []string{}}
		if c.handler == nil || ctx.Err() != nil {
			continue
		}
		options := make([]acp.PermissionOption, 0, len(q.Options))
		text := q.Header + "\n" + q.Question
		for i, option := range q.Options {
			kind := "allow_once"
			switch strings.ToLower(option.Label) {
			case "decline", "cancel", "reject", "deny":
				kind = "reject_once"
			}
			options = append(options, acp.PermissionOption{OptionID: fmt.Sprintf("choice_%d", i), Name: option.Label, Kind: kind})
			if option.Description != "" {
				text += "\n" + option.Label + ": " + option.Description
			}
		}
		response, err := c.handler.Request(ctx, "session/request_permission", acp.Raw(acp.RequestPermissionParams{SessionID: "codex:" + p.ThreadID, ToolCall: acp.Raw(map[string]any{"toolCallId": p.ItemID, "title": text, "rawInput": json.RawMessage(params)}), Options: options}))
		if err != nil {
			continue
		}
		var outcome acp.PermissionOutcome
		if acp.Decode(response, &outcome) != nil || outcome.Outcome.Outcome != "selected" {
			continue
		}
		for i, option := range options {
			if option.OptionID == outcome.Outcome.OptionID {
				answers[q.ID] = map[string]any{"answers": []string{q.Options[i].Label}}
				break
			}
		}
	}
	return map[string]any{"answers": answers}, nil
}
