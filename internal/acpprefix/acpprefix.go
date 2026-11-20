// Package acpprefix gives the sessions of an agent program a prefix of their
// own on Dinah's side. The program's own session IDs have none, and Dinah
// tells which program a chat belongs to by the prefix of its ID
// (providers.Row.SessionPrefix), so the prefix is put on every session ID the
// program sends and taken off every one it is sent.
package acpprefix

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"studio/internal/acp"
)

// Add puts prefix on an ID that does not have it.
func Add(prefix string) func(string) string {
	return func(id string) string {
		if strings.HasPrefix(id, prefix) {
			return id
		}
		return prefix + id
	}
}

// Strip takes prefix off an ID.
func Strip(prefix string) func(string) string {
	return func(id string) string { return strings.TrimPrefix(id, prefix) }
}

// Rewrite applies change to every "sessionId" string in a JSON value,
// however deep, and to every "subagentSessionId", which the ACP subagent
// RFD's subagent_spawned and subagent_state_update use for a child session
// whose own updates then carry it as their "sessionId". It returns the
// value as JSON. Numbers keep their text.
func Rewrite(v any, change func(string) string) (json.RawMessage, error) {
	data, ok := v.(json.RawMessage)
	if !ok {
		var err error
		if data, err = json.Marshal(v); err != nil {
			return nil, err
		}
	}
	if len(data) == 0 || (!bytes.Contains(data, []byte(`"sessionId"`)) && !bytes.Contains(data, []byte(`"subagentSessionId"`))) {
		return data, nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var tree any
	if err := dec.Decode(&tree); err != nil {
		return nil, err
	}
	walk(tree, change)
	return json.Marshal(tree)
}

func walk(v any, change func(string) string) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			if s, ok := child.(string); ok && (k == "sessionId" || k == "subagentSessionId") {
				x[k] = change(s)
				continue
			}
			walk(child, change)
		}
	case []any:
		for _, child := range x {
			walk(child, change)
		}
	}
}

// Handler puts the prefix on the session IDs of what the program sends.
type Handler struct {
	H      acp.Handler
	Prefix string
}

func (p Handler) Notify(method string, params json.RawMessage) {
	if fixed, err := Rewrite(params, Add(p.Prefix)); err == nil {
		params = fixed
	}
	p.H.Notify(method, params)
}

func (p Handler) Request(ctx context.Context, method string, params json.RawMessage) (any, *acp.Error) {
	if fixed, err := Rewrite(params, Add(p.Prefix)); err == nil {
		params = fixed
	}
	return p.H.Request(ctx, method, params)
}

// Conn is a running program whose session IDs have Prefix on Dinah's side
// and not on the program's.
type Conn struct {
	*acp.Conn
	Prefix string
}

// Call sends a request with the prefix taken off its session IDs, and puts
// the prefix on the session IDs of the answer.
func (c *Conn) Call(ctx context.Context, method string, params, out any) error {
	p, err := Rewrite(params, Strip(c.Prefix))
	if err != nil {
		return err
	}
	var raw json.RawMessage
	if err := c.Conn.Call(ctx, method, p, &raw); err != nil {
		return err
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	fixed, err := Rewrite(raw, Add(c.Prefix))
	if err != nil {
		return err
	}
	return json.Unmarshal(fixed, out)
}

// Notify sends a notification with the prefix taken off its session IDs.
func (c *Conn) Notify(method string, params any) error {
	p, err := Rewrite(params, Strip(c.Prefix))
	if err != nil {
		return err
	}
	return c.Conn.Notify(method, p)
}
