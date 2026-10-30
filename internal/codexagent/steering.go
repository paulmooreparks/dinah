package codexagent

import (
	"context"
	"errors"

	"studio/internal/acp"
)

// Steer uses the documented app-server turn/steer operation. The expected
// turn ID prevents a late submission from redirecting a different turn.
func (c *Conn) Steer(ctx context.Context, p acp.PromptParams, delivered func()) error {
	id, err := native(p.SessionID)
	if err != nil {
		return errors.Join(acp.ErrSteeringRejected, err)
	}
	inputs, err := promptInputs(p)
	if err != nil {
		return errors.Join(acp.ErrSteeringRejected, err)
	}
	c.mu.Lock()
	s := c.sessions[p.SessionID]
	if s == nil || s.completion == nil || s.turn == "" || s.cancelRequested {
		c.mu.Unlock()
		return acp.ErrNoActiveTurn
	}
	turnID := s.turn
	c.mu.Unlock()
	var receipt struct {
		TurnID string `json:"turnId"`
	}
	if err := c.Conn.Call(ctx, "turn/steer", map[string]any{"threadId": id, "expectedTurnId": turnID, "input": inputs}, &receipt); err != nil {
		// A failed or lost response does not establish whether the input was
		// inserted. Leave retry decisions to the host and person.
		return err
	}
	if receipt.TurnID != turnID {
		return errors.New("Codex acknowledged steering for a different turn")
	}
	if delivered != nil {
		delivered()
	}
	return nil
}

var _ acp.Steerer = (*Conn)(nil)

func (c *Conn) CanSteerContent(id string, types []string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.sessions[id]
	if s == nil {
		return false
	}
	for _, kind := range types {
		if kind != "image" {
			continue
		}
		for _, m := range c.models {
			if m.ID != s.model && m.Model != s.model {
				continue
			}
			for _, modality := range m.InputModalities {
				if modality == "image" {
					return true
				}
			}
		}
		return false
	}
	return true
}

// CanSteer reports the native runtime's support for loaded chat sessions.
// turn/steer accepts user input independently of model tool capabilities.
func (c *Conn) CanSteer(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sessions[id] != nil
}
