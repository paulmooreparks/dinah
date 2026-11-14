package claudeagent

import (
	"context"
	"errors"
	"fmt"

	"studio/internal/acp"
)

// CanSteer uses the adapter's advertised extension, independently of hooks.
func (c *Conn) CanSteer(string) bool { return c.steering.Load() }

func (c *Conn) CanSteerContent(id string, types []string) bool {
	for _, kind := range types {
		if kind != "text" {
			return false
		}
	}
	return c.CanSteer(id)
}

// Steer follows the producer's examples/steering.ts contract. The explicit
// idle policy prevents the adapter's legacy detached-turn fallback.
func (c *Conn) Steer(ctx context.Context, p acp.PromptParams, delivered func()) error {
	if !c.CanSteer(p.SessionID) {
		return errors.Join(acp.ErrSteeringRejected, errors.New("the Claude adapter does not advertise steering"))
	}
	for _, block := range p.Prompt {
		if block.Type != "text" {
			return errors.Join(acp.ErrSteeringRejected, errors.New("Claude steering accepts text only"))
		}
	}
	var receipt struct {
		Outcome string `json:"outcome"`
		Reason  string `json:"reason"`
	}
	err := c.Call(ctx, "_session/steering", map[string]any{
		"sessionId": p.SessionID, "prompt": p.Prompt,
		"_meta": map[string]any{"steering": map[string]string{"idleBehavior": "promptRequired"}},
	}, &receipt)
	if err != nil {
		return err
	}
	if receipt.Outcome == "promptRequired" && receipt.Reason == "noRunningTurn" {
		return acp.ErrNoActiveTurn
	}
	if receipt.Outcome != "injected" {
		return fmt.Errorf("%w: unexpected Claude steering outcome %q", acp.ErrSteeringUnconfirmed, receipt.Outcome)
	}
	if delivered != nil {
		delivered()
	}
	return nil
}

var _ acp.Steerer = (*Conn)(nil)
