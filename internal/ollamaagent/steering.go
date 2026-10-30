package ollamaagent

import (
	"context"
	"errors"
	"fmt"

	"studio/internal/acp"
)

type steeringInput struct {
	message   message
	delivered func()
}

// Steer queues input for the next complete model exchange. Running tools
// and streamed responses finish normally before the input is inserted.
func (c *Conn) Steer(ctx context.Context, p acp.PromptParams, delivered func()) error {
	if err := ctx.Err(); err != nil {
		return errors.Join(acp.ErrSteeringRejected, err)
	}
	input, err := promptMessage(p)
	if err != nil {
		return errors.Join(acp.ErrSteeringRejected, err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.sessions[p.SessionID]
	if s == nil || !s.busy || !s.steeringOpen || s.cancel == nil {
		return acp.ErrNoActiveTurn
	}
	// Model capabilities were fetched before the active exchange began.
	shown := c.shown[s.rec.Model]
	if len(input.Images) > 0 && !shown.can("vision") {
		return errors.Join(acp.ErrSteeringRejected, errors.New("the active Ollama model cannot read images"))
	}
	s.steers = append(s.steers, steeringInput{message: input, delivered: delivered})
	return nil
}

var _ acp.Steerer = (*Conn)(nil)

// CanSteer reports support for a loaded chat model. Dinah owns the loop,
// so text-only models also accept guidance through the next chat request.
func (c *Conn) CanSteer(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.sessions[id]
	return s != nil && s.rec.Model != ""
}

func (c *Conn) CanSteerContent(id string, types []string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.sessions[id]
	if s == nil || s.rec.Model == "" {
		return false
	}
	for _, kind := range types {
		if kind == "image" && !c.shown[s.rec.Model].can("vision") {
			return false
		}
	}
	return true
}

// persistSteeringMessages acknowledges each input only after its history save.
func persistSteeringMessages(rec *record, pending []steeringInput, save func() error) error {
	for _, input := range pending {
		rec.Messages = append(rec.Messages, input.message)
		if err := save(); err != nil {
			return fmt.Errorf("%w: %v", acp.ErrSteeringUnconfirmed, err)
		}
		if input.delivered != nil {
			input.delivered()
		}
	}
	return nil
}
