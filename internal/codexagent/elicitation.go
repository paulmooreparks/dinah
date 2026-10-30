package codexagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"studio/internal/acp"
	"studio/internal/profile"
)

type elicitationSchema struct {
	Schema     string                     `json:"$schema"`
	Type       string                     `json:"type"`
	Properties map[string]json.RawMessage `json:"properties"`
	Required   []string                   `json:"required"`
}
type elicitationField struct {
	Type        string          `json:"type"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Default     json.RawMessage `json:"default"`
	Enum        []string        `json:"enum"`
	EnumNames   []string        `json:"enumNames"`
	OneOf       []struct {
		Value string `json:"const"`
		Title string `json:"title"`
	} `json:"oneOf"`
}
type elicitationChoice struct {
	name  string
	value any
}

func strictDecode(b json.RawMessage, out any) error {
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	return d.Decode(out)
}

// Form values follow the MCP requestedSchema. Defaults never authorize an
// action; every submitted value must come from the person's selection.
func (c *Conn) elicitation(ctx context.Context, params json.RawMessage) (any, *acp.Error) {
	var p struct {
		ThreadID   string          `json:"threadId"`
		ServerName string          `json:"serverName"`
		Mode       string          `json:"mode"`
		Message    string          `json:"message"`
		Schema     json.RawMessage `json:"requestedSchema"`
	}
	if json.Unmarshal(params, &p) != nil {
		return nil, &acp.Error{Code: acp.CodeInternal, Message: "Invalid Codex MCP elicitation"}
	}
	unsupported := func(detail string) (any, *acp.Error) {
		return nil, &acp.Error{Code: acp.CodeMethodNotFound, Message: profile.Current().Name + " cannot display this MCP request: " + detail}
	}
	if p.Mode != "form" {
		return unsupported("only standard forms with selectable options are supported")
	}
	var schema elicitationSchema
	if err := strictDecode(p.Schema, &schema); err != nil || schema.Type != "object" {
		return unsupported("the requested form schema is unsupported")
	}
	required := map[string]bool{}
	for _, key := range schema.Required {
		if _, ok := schema.Properties[key]; !ok {
			return unsupported("a required field is missing from the schema")
		}
		required[key] = true
	}
	keys := make([]string, 0, len(schema.Properties))
	fields := map[string]elicitationField{}
	choices := map[string][]elicitationChoice{}
	for key, b := range schema.Properties {
		var field elicitationField
		if err := strictDecode(b, &field); err != nil {
			return unsupported("field " + key + " uses unsupported constraints")
		}
		var values []elicitationChoice
		switch field.Type {
		case "boolean":
			if len(field.Enum) > 0 || len(field.OneOf) > 0 {
				return unsupported("field " + key + " has invalid boolean choices")
			}
			values = []elicitationChoice{{"Yes", true}, {"No", false}}
		case "string":
			if len(field.Enum) > 0 && len(field.OneOf) > 0 {
				return unsupported("field " + key + " combines enum and oneOf")
			}
			if len(field.EnumNames) > 0 && len(field.EnumNames) != len(field.Enum) {
				return unsupported("field " + key + " has mismatched choice labels")
			}
			for i, value := range field.Enum {
				name := value
				if len(field.EnumNames) > 0 {
					name = field.EnumNames[i]
				}
				values = append(values, elicitationChoice{name, value})
			}
			for _, value := range field.OneOf {
				values = append(values, elicitationChoice{value.Title, value.Value})
			}
		default:
			return unsupported("field " + key + " requires a value that the current interface cannot collect")
		}
		if len(values) == 0 {
			return unsupported("field " + key + " has no selectable choices")
		}
		keys = append(keys, key)
		fields[key] = field
		choices[key] = values
	}
	sort.Strings(keys)
	c.mu.Lock()
	s := c.sessions["codex:"+p.ThreadID]
	if s != nil && s.requestContext != nil {
		ctx = s.requestContext
	}
	c.mu.Unlock()
	baseTitle := p.ServerName + "\n" + p.Message
	if len(keys) == 0 {
		choice := c.elicitationPrompt(ctx, p.ThreadID, baseTitle, params, []acp.PermissionOption{{OptionID: "accept", Name: "Accept", Kind: "allow_once"}, {OptionID: "decline", Name: "Decline", Kind: "reject_once"}, {OptionID: "cancel", Name: "Cancel", Kind: "reject_once"}})
		if choice == "accept" {
			return map[string]any{"action": "accept", "content": map[string]any{}}, nil
		}
		if choice == "decline" {
			return map[string]any{"action": "decline", "content": nil}, nil
		}
		return map[string]any{"action": "cancel", "content": nil}, nil
	}
	content := map[string]any{}
	for _, key := range keys {
		field := fields[key]
		title := baseTitle + "\n" + key
		if field.Title != "" {
			title = baseTitle + "\n" + field.Title
		}
		if field.Description != "" {
			title += "\n" + field.Description
		}
		options := []acp.PermissionOption{}
		for i, value := range choices[key] {
			kind := "allow_once"
			switch strings.ToLower(value.name) {
			case "no", "decline", "cancel", "reject", "deny":
				kind = "reject_once"
			}
			options = append(options, acp.PermissionOption{OptionID: fmt.Sprintf("value_%d", i), Name: value.name, Kind: kind})
		}
		if !required[key] {
			options = append(options, acp.PermissionOption{OptionID: "omit", Name: "Skip This Field", Kind: "reject_once"})
		}
		options = append(options, acp.PermissionOption{OptionID: "cancel", Name: "Cancel Request", Kind: "reject_once"})
		selection := c.elicitationPrompt(ctx, p.ThreadID, title, params, options)
		if selection == "omit" && !required[key] {
			continue
		}
		found := false
		for i, value := range choices[key] {
			if selection == fmt.Sprintf("value_%d", i) {
				content[key] = value.value
				found = true
				break
			}
		}
		if !found {
			return map[string]any{"action": "cancel", "content": nil}, nil
		}
	}
	return map[string]any{"action": "accept", "content": content}, nil
}

func (c *Conn) elicitationPrompt(ctx context.Context, threadID, title string, params json.RawMessage, options []acp.PermissionOption) string {
	if c.handler == nil || ctx.Err() != nil {
		return ""
	}
	res, err := c.handler.Request(ctx, "session/request_permission", acp.Raw(acp.RequestPermissionParams{SessionID: "codex:" + threadID, ToolCall: acp.Raw(map[string]any{"title": title, "rawInput": json.RawMessage(params)}), Options: options}))
	if err != nil {
		return ""
	}
	var outcome acp.PermissionOutcome
	if acp.Decode(res, &outcome) != nil || outcome.Outcome.Outcome != "selected" {
		return ""
	}
	for _, option := range options {
		if option.OptionID == outcome.Outcome.OptionID {
			return option.OptionID
		}
	}
	return ""
}
