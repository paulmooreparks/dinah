package codexagent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"studio/internal/acp"
	"studio/internal/agentpolicy"
	"studio/internal/profile"
)

func (c *Conn) update(id string, u any) {
	if c.handler != nil {
		c.handler.Notify("session/update", acp.Raw(map[string]any{"sessionId": id, "update": u}))
	}
}
func (c *Conn) chunk(id, kind, text string) {
	if text != "" {
		c.update(id, map[string]any{"sessionUpdate": kind, "content": map[string]string{"type": "text", "text": text}})
	}
}

func (c *Conn) notification(method string, params json.RawMessage) {
	var p struct {
		ThreadID string          `json:"threadId"`
		ItemID   string          `json:"itemId"`
		Delta    string          `json:"delta"`
		Turn     turn            `json:"turn"`
		Item     json.RawMessage `json:"item"`
		Plan     []struct {
			Step   string `json:"step"`
			Status string `json:"status"`
		} `json:"plan"`
	}
	if json.Unmarshal(params, &p) != nil {
		return
	}
	id := "codex:" + p.ThreadID
	switch method {
	case "turn/started":
		c.mu.Lock()
		s := c.sessions[id]
		if s != nil {
			s.turn = p.Turn.ID
		}
		c.mu.Unlock()
	case "turn/completed":
		c.mu.Lock()
		s := c.sessions[id]
		if s != nil && s.completion != nil && (s.turn == "" || s.turn == p.Turn.ID) {
			select {
			case s.completion <- p.Turn:
			default:
			}
		}
		c.mu.Unlock()
	case "item/agentMessage/delta", "item/reasoning/summaryTextDelta", "item/plan/delta":
		c.mu.Lock()
		if s := c.sessions[id]; s != nil {
			s.streamed[p.ItemID] = true
		}
		c.mu.Unlock()
		kind := "agent_message_chunk"
		if method == "item/reasoning/summaryTextDelta" {
			kind = "agent_thought_chunk"
		}
		c.chunk(id, kind, p.Delta)
	case "item/started":
		c.item(id, p.Item, false, false)
	case "item/completed":
		c.item(id, p.Item, true, false)
	case "item/commandExecution/outputDelta":
		c.update(id, map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": p.ItemID, "content": []any{map[string]any{"type": "content", "content": map[string]string{"type": "text", "text": p.Delta}}}})
	case "turn/plan/updated":
		entries := []map[string]string{}
		for _, e := range p.Plan {
			status := e.Status
			if status == "inProgress" {
				status = "in_progress"
			}
			entries = append(entries, map[string]string{"content": e.Step, "priority": "medium", "status": status})
		}
		c.update(id, map[string]any{"sessionUpdate": "plan", "entries": entries})
	case "thread/tokenUsage/updated":
		var usage struct {
			TokenUsage struct {
				Window *int64 `json:"modelContextWindow"`
				Last   struct {
					Total int64 `json:"totalTokens"`
				} `json:"last"`
			} `json:"tokenUsage"`
		}
		if json.Unmarshal(params, &usage) == nil && usage.TokenUsage.Last.Total > 0 {
			// What the request used, which a team's budget counts (a turn_cost update is not shown).
			c.update(id, map[string]any{"sessionUpdate": "turn_cost", "tokens": usage.TokenUsage.Last.Total})
		}
		if json.Unmarshal(params, &usage) == nil && usage.TokenUsage.Window != nil && *usage.TokenUsage.Window > 0 {
			c.update(id, map[string]any{"sessionUpdate": "usage_update", "used": usage.TokenUsage.Last.Total, "size": *usage.TokenUsage.Window})
		}
	default:
		if strings.HasPrefix(method, "account/") && c.handler != nil {
			c.handler.Notify("codex/"+method, params)
		}
	}
}

func (c *Conn) item(id string, b json.RawMessage, completed, replay bool) {
	var p struct {
		ID      string            `json:"id"`
		Type    string            `json:"type"`
		Text    string            `json:"text"`
		Summary []string          `json:"summary"`
		Content []json.RawMessage `json:"content"`
		Command string            `json:"command"`
		Status  string            `json:"status"`
		Server  string            `json:"server"`
		Tool    string            `json:"tool"`
		Query   string            `json:"query"`
		Output  string            `json:"aggregatedOutput"`
	}
	if json.Unmarshal(b, &p) != nil {
		return
	}
	switch p.Type {
	case "agentMessage", "plan", "reasoning":
		if !completed {
			return
		}
		c.mu.Lock()
		streamed := false
		if s := c.sessions[id]; s != nil {
			streamed = s.streamed[p.ID]
		}
		c.mu.Unlock()
		if streamed && !replay {
			return
		}
		text := p.Text
		kind := "agent_message_chunk"
		if p.Type == "reasoning" {
			kind = "agent_thought_chunk"
			text = strings.Join(p.Summary, "\n")
		}
		c.chunk(id, kind, text)
	case "userMessage":
		if replay {
			for _, content := range p.Content {
				var v struct{ Type, Text string }
				if json.Unmarshal(content, &v) == nil {
					if v.Type == "text" {
						c.chunk(id, "user_message_chunk", v.Text)
					} else {
						// The host must know replay omitted non-text context. Do not
						// read local image paths or manufacture image data here.
						c.update(id, map[string]any{"sessionUpdate": "user_message_chunk", "content": acp.ContentBlock{Type: v.Type}})
					}
				}
			}
		}
	default:
		title := p.Type
		kind := "other"
		switch p.Type {
		case "commandExecution":
			title = p.Command
			kind = "execute"
		case "fileChange":
			title = "Change Files"
			kind = "edit"
		case "mcpToolCall":
			title = p.Server + ": " + p.Tool
		case "webSearch":
			title = "Search: " + p.Query
			kind = "search"
		}
		status := "in_progress"
		if completed {
			status = "completed"
		}
		if p.Status == "failed" || p.Status == "declined" {
			status = "failed"
		}
		update := "tool_call"
		if completed && !replay {
			update = "tool_call_update"
		}
		u := map[string]any{"sessionUpdate": update, "toolCallId": p.ID, "title": title, "kind": kind, "status": status, "rawInput": json.RawMessage(b)}
		if content := toolResultContent(b); len(content) > 0 {
			u["content"] = content
		}
		c.update(id, u)
	}
}

func (c *Conn) request(ctx context.Context, method string, params json.RawMessage) (any, *acp.Error) {
	if method == "item/tool/requestUserInput" {
		return c.userInput(ctx, params)
	}
	if method == "mcpServer/elicitation/request" {
		return c.elicitation(ctx, params)
	}
	if method == "item/permissions/requestApproval" {
		var p struct {
			ThreadID    string          `json:"threadId"`
			Permissions json.RawMessage `json:"permissions"`
		}
		if json.Unmarshal(params, &p) != nil {
			return nil, &acp.Error{Code: acp.CodeInternal, Message: "Invalid Codex permission request"}
		}
		c.mu.Lock()
		s := c.sessions["codex:"+p.ThreadID]
		all := s != nil && s.approvals == "all"
		c.mu.Unlock()
		if all && len(p.Permissions) > 0 && string(p.Permissions) != "null" {
			return map[string]any{"permissions": p.Permissions, "scope": "turn"}, nil
		}
		return map[string]any{"permissions": map[string]any{}, "scope": "turn"}, nil
	}
	if method != "item/commandExecution/requestApproval" && method != "item/fileChange/requestApproval" {
		return nil, &acp.Error{Code: acp.CodeMethodNotFound, Message: profile.Current().Name + " does not support Codex request " + method}
	}
	var p struct {
		ThreadID  string                           `json:"threadId"`
		ItemID    string                           `json:"itemId"`
		Command   string                           `json:"command"`
		Reason    string                           `json:"reason"`
		GrantRoot string                           `json:"grantRoot"`
		Network   *struct{ Host, Protocol string } `json:"networkApprovalContext"`
		Available []json.RawMessage                `json:"availableDecisions"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &acp.Error{Code: acp.CodeInternal, Message: "Invalid Codex approval request"}
	}
	if method == "item/commandExecution/requestApproval" && agentpolicy.ReservedCommand(p.Command) {
		return map[string]string{"decision": "decline"}, nil
	}
	if c.handler == nil {
		return map[string]string{"decision": "cancel"}, nil
	}
	c.mu.Lock()
	s := c.sessions["codex:"+p.ThreadID]
	policy := ""
	if s != nil {
		policy = s.approvals
	}
	if s != nil && s.requestContext != nil {
		ctx = s.requestContext
	}
	c.mu.Unlock()
	if ctx.Err() != nil {
		return map[string]string{"decision": "cancel"}, nil
	}
	if policy != ApprovalsAsk && policy != "" {
		// The person chose to let this chat run unattended, so an escalation
		// Codex puts to them is allowed here. One the sandbox refuses never
		// reaches this, which is the bound on what "unattended" means.
		for _, d := range p.Available {
			var v string
			if json.Unmarshal(d, &v) == nil && (v == "accept" || policy != "all" && (v == "acceptForSession" || v == "approve")) {
				return map[string]string{"decision": v}, nil
			}
		}
		if len(p.Available) == 0 {
			return map[string]string{"decision": "accept"}, nil
		}
	}
	title := p.Command
	if title == "" {
		title = "Change Files"
	}
	if p.Network != nil {
		title = fmt.Sprintf("Allow %s access to %s", p.Network.Protocol, p.Network.Host)
	}
	if p.Reason != "" {
		title += "\n" + p.Reason
	}
	if p.GrantRoot != "" {
		title += "\n" + p.GrantRoot
	}
	options := []acp.PermissionOption{}
	for _, v := range []struct{ id, name, kind string }{{"accept", "Allow Once", "allow_once"}, {"decline", "Reject", "reject_once"}, {"cancel", "Cancel Turn", "reject_once"}} {
		allowed := len(p.Available) == 0
		for _, decision := range p.Available {
			var s string
			if json.Unmarshal(decision, &s) == nil && s == v.id {
				allowed = true
			}
		}
		if allowed {
			options = append(options, acp.PermissionOption{OptionID: v.id, Name: v.name, Kind: v.kind})
		}
	}
	answer, err := c.handler.Request(ctx, "session/request_permission", acp.Raw(acp.RequestPermissionParams{SessionID: "codex:" + p.ThreadID, ToolCall: acp.Raw(map[string]any{"toolCallId": p.ItemID, "title": title, "rawInput": json.RawMessage(params)}), Options: options}))
	if err != nil {
		return map[string]string{"decision": "cancel"}, nil
	}
	var outcome acp.PermissionOutcome
	if acp.Decode(answer, &outcome) != nil || outcome.Outcome.Outcome != "selected" {
		return map[string]string{"decision": "cancel"}, nil
	}
	for _, v := range options {
		if v.OptionID == outcome.Outcome.OptionID {
			return map[string]string{"decision": v.OptionID}, nil
		}
	}
	return map[string]string{"decision": "cancel"}, nil
}
