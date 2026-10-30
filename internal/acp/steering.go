package acp

import (
	"context"
	"errors"
)

// ErrNoActiveTurn means no input was accepted. The host may queue it for
// the next turn. Other errors must not trigger an automatic retry because
// the provider may already have received the input.
var ErrNoActiveTurn = errors.New("there is no active turn to steer")

// ErrSteeringUnconfirmed means an asynchronous insertion may have reached
// durable history. The host must retain it for review instead of replaying it.
var ErrSteeringUnconfirmed = errors.New("steering persistence is unconfirmed")

// ErrSteeringRejected is a local validation failure before any delivery.
var ErrSteeringRejected = errors.New("steering was not sent")

// Steerer is an internal adapter capability, not an ACP wire extension.
// Steer preserves the active turn. delivered is called once the input has
// been inserted into the conversation or accepted by the native runtime.
// Acceptance does not promise that the model has consumed the input.
// Queued input not delivered by the end of the turn belongs to the host
// again and must not remain in the adapter for a later turn.
type Steerer interface {
	Steer(context.Context, PromptParams, func()) error
}

// SteeringCapability reports support for the session's selected model.
// The host treats an absent capability as unsupported for automatic sends.
type SteeringCapability interface {
	CanSteer(string) bool
}

// SteeringContentCapability checks content types without reading attachments
// or changing the conversation. Unknown attachment support must queue.
type SteeringContentCapability interface {
	CanSteerContent(sessionID string, contentTypes []string) bool
}
