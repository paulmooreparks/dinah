package sessionhost

import (
	"context"
	_ "embed"
	"errors"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"studio/internal/acp"
	"studio/internal/bus"
	"studio/internal/providers"
)

//go:embed steering_io.go
var SteeringIOSource []byte

// Steering preserves active work. Claude and Devin use documented command
// hooks and the bus queue. Native adapters implement acp.Steerer. Inputs
// that miss the turn boundary remain queued, and uncertain native responses
// are reported to the person without retrying them automatically.
//
// agent:steer carries a submission ID and one of pending, accepted,
// delivered, queued, sent or failed. Acceptance does not mean consumption.

// hooksReach says whether a chat's agent runs the hooks that hand over a
// steer, which Claude and Devin can once their hooks
// have tied its agent's session to it.
func (t *TurnIO) HooksReach(s *Conversation) bool {
	id, provider := s.BusID(), normalizeProvider(s.Provider())

	if id == "" || !providers.For(provider).HookSteering {
		return false
	}
	ok, _ := t.host.HooksReach(id)
	return ok
}

// steerMarker is the marker a chat's prompt carries, or "" for a chat whose
// agent runs no supported hooks.
func SteerMarker(s *Conversation) string {
	provider := normalizeProvider(s.Provider())
	id := s.BusID()
	if id == "" || !providers.For(provider).HookSteering {
		return ""
	}
	return bus.SteerMarker(id)
}

func (t *TurnIO) steerByHook(s *Conversation, submission *Submission, text string, doc *DocRef, pr Prompt, ledgerID int64) error {
	body := t.promptText(&pr)
	busID := s.BusID()
	id, err := t.host.QueueHookSteer(busID, body)
	if err != nil {
		// A transaction error can follow a committed insert. Keep its
		// recovery record and do not make the composer resend it implicitly.
		t.host.SteeringState(s.ID(), ledgerID, "failed")
		EmitChat(t.host.Emit, s, EventSteer, map[string]any{"sessionId": s.ID(), "steer": -ledgerID, "state": "pending"})
		EmitChat(t.host.Emit, s, EventSteer, map[string]any{"sessionId": s.ID(), "steer": -ledgerID, "state": "failed"})
		return err
	}
	submission.RegisterHook(id, ledgerID, body, pr)
	t.host.Remember(s.Cwd(), s.ID(), "steer", text, doc)
	EmitChat(t.host.Emit, s, EventSteer, map[string]any{"sessionId": s.ID(), "steer": id, "state": "pending"})
	return nil
}

// NoticeSteers checks delivery synchronously with the watcher's borrowed
// operation. It never retains that operation or its bus handle.
func (t *TurnIO) NoticeSteers(delivered func(context.Context, int64) (bool, error), s *Conversation) {
	submission := s.TrySubmission()
	if submission == nil {
		return
	}
	defer submission.Release()
	pending := submission.Pending()
	for _, p := range pending {
		if p.Receipt != nil {
			continue
		}
		done, err := delivered(context.Background(), p.ID)
		if err != nil || !done {
			continue
		}
		t.steeringPromptDelivered(s, p.Fallback)
		submission.Drop(p.ID)
		t.host.SteeringState(s.ID(), p.LedgerID, "delivered")
		if p.Fallback != nil {
			t.host.RecordPrompt(s, p.Fallback)
			t.host.LogPrompt(s.ID(), steeringPromptLabel(*p.Fallback), t.promptText(p.Fallback))
		}
		EmitChat(t.host.Emit, s, EventSteer, map[string]any{"sessionId": s.ID(), "steer": p.ID, "state": "delivered"})
	}
}

// settleSteers ends a turn's waiting steers: one a hook took is delivered,
// and one no hook took is withdrawn and returned, to go as the next prompt.
func (t *TurnIO) SettleSteers(s *Conversation) []Prompt {
	submission := s.BeginSubmission()
	defer submission.Release()
	return t.SettleSteersWithSubmission(s, submission)
}

func (t *TurnIO) SettleSteersWithSubmission(s *Conversation, submission *Submission) []Prompt {
	pending := submission.TakePending()
	var next []Prompt
	for _, p := range pending {
		if p.Receipt != nil {
			delivered := p.Receipt.Delivered()
			if !delivered {
				pr := p.Receipt.Prompt()
				if pr.Cancelled() {
					t.host.SteeringState(s.ID(), p.LedgerID, "dismissed")
					continue
				}
				pr = pr.AsLateSteer(p.ID, p.LedgerID)
				t.host.SteeringState(s.ID(), p.LedgerID, "queued")
				next = append(next, pr)
				EmitChat(t.host.Emit, s, EventSteer, map[string]any{"sessionId": s.ID(), "steer": p.ID, "state": "queued"})
			}
			continue
		}
		withdrawn, err := t.host.WithdrawHookSteer(p.ID)
		if err != nil {
			// Retain ownership until withdrawal or delivery is confirmed.
			submission.Restore(p)
			t.host.LogError("bus", "settling steering delivery: %v", err)
			continue
		}
		if !withdrawn {
			t.steeringPromptDelivered(s, p.Fallback)
			t.host.SteeringState(s.ID(), p.LedgerID, "delivered")
			if p.Fallback != nil {
				t.host.RecordPrompt(s, p.Fallback)
				t.host.LogPrompt(s.ID(), steeringPromptLabel(*p.Fallback), t.promptText(p.Fallback))
			}
			EmitChat(t.host.Emit, s, EventSteer, map[string]any{"sessionId": s.ID(), "steer": p.ID, "state": "delivered"})
			continue
		}
		var pr Prompt
		if p.Fallback != nil {
			pr = *p.Fallback
		} else {
			pr = NewPrompt(p.Text, []acp.ContentBlock{{Type: "text", Text: p.Text}})
		}
		if pr.Cancelled() {
			t.host.SteeringState(s.ID(), p.LedgerID, "dismissed")
			continue
		}
		pr = pr.AsLateSteer(p.ID, p.LedgerID)
		t.host.SteeringState(s.ID(), p.LedgerID, "queued")
		next = append(next, pr)
		EmitChat(t.host.Emit, s, EventSteer, map[string]any{"sessionId": s.ID(), "steer": p.ID, "state": "queued"})
	}
	return next
}

// An unsuccessful transactional save may still have committed its payload.
// Retain the recovery record without queuing a duplicate user message.
func (t *TurnIO) UnconfirmNativeSteers(s *Conversation, submission *Submission) {
	pending := submission.Pending()
	for _, p := range pending {
		if p.Receipt == nil {
			continue
		}
		delivered := p.Receipt.Delivered()
		if delivered {
			continue
		}
		submission.Drop(p.ID)
		t.host.SteeringState(s.ID(), p.LedgerID, "failed")
		EmitChat(t.host.Emit, s, EventSteer, map[string]any{"sessionId": s.ID(), "steer": p.ID, "state": "failed"})
	}
}

// CanSteer resolves the active provider and model before an ordinary user
// send. Explicit Queue and Steer bypass this automatic choice.
func (t *TurnIO) CanSteer(id string, atts []Attachment) bool {
	s := t.host.LookupConversation(id)
	if s == nil {
		return false
	}
	conn := t.host.SessionConnection(s)
	if capability, ok := conn.(acp.SteeringCapability); ok {
		if _, delivers := conn.(acp.Steerer); delivers {
			if len(atts) > 0 {
				content, ok := conn.(acp.SteeringContentCapability)
				if !ok {
					return false
				}
				types := []string{"text"}
				for _, at := range atts {
					mt := at.MimeType
					if mt == "" {
						mt = mime.TypeByExtension(filepath.Ext(at.Name))
					}
					if at.Kind == "file" && strings.HasPrefix(mt, "image/") {
						types = append(types, "image")
					}
				}
				if !content.CanSteerContent(id, types) {
					return false
				}
			}
			return capability.CanSteer(id)
		}
	}
	return len(atts) == 0 && t.HooksReach(s)
}

func (t *TurnIO) SteerActive(id, text string, doc *DocRef, atts []Attachment) error {
	text = strings.TrimSpace(text)
	if text == "" && len(atts) == 0 {
		return nil
	}
	if err := t.host.HoldingTurns(); err != nil {
		return err
	}
	if current, err := t.host.ResolveContinuation(id); err != nil {
		return err
	} else if current != id {
		return errors.New("this chat continues with another provider; open its current conversation before steering")
	}
	s := t.host.LookupConversation(id)
	if s == nil {
		return errors.New("that session is not open")
	}
	submission := s.BeginSubmission()
	defer submission.Release()

	state := s.State()
	blocked := !state.Ready
	working := state.Working

	if blocked {
		return errors.New("wait for the provider change to finish")
	}
	if !working {
		return t.Send(id, text, doc, atts)
	}
	p, err := t.host.MakePrompt(s, text, doc, atts)
	if err != nil {
		return err
	}
	return t.SubmitSteer(s, submission, p, text, doc, atts)
}

// submitSteerLocked shares delivery and recovery for prepared user and chat
// prompts. The caller owns submission through submission and acceptance.
func (t *TurnIO) SubmitSteer(s *Conversation, submission *Submission, p Prompt, text string, doc *DocRef, atts []Attachment) error {
	id := s.ID()
	ledgerID, err := t.host.SaveSteering(s, text, doc, atts, p)
	if err != nil {
		return err
	}
	if len(atts) == 0 && text != "" && t.HooksReach(s) {
		return t.steerByHook(s, submission, text, doc, p, ledgerID)
	}
	key := -ledgerID
	receipt := submission.RegisterNative(key, ledgerID, p)
	EmitChat(t.host.Emit, s, EventSteer, map[string]any{"sessionId": id, "steer": key, "state": "pending"})
	if conn, ok := t.host.SessionConnection(s).(acp.Steerer); ok && (len(atts) == 0 || t.CanSteer(id, atts)) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err = conn.Steer(ctx, acp.PromptParams{SessionID: id, Prompt: p.Blocks()}, func() {
			confirmed, ok := receipt.Confirm()
			if !ok {
				return
			}
			p := confirmed
			t.steeringPromptDelivered(s, &p)
			t.host.SteeringState(id, ledgerID, "delivered")
			t.host.RecordPrompt(s, &p)
			t.host.LogPrompt(id, steeringPromptLabel(p), t.promptText(&p))
			state := "delivered"
			if providers.For(normalizeProvider(s.Provider())).SteerAccepted {
				state = "accepted"
			}
			EmitChat(t.host.Emit, s, EventSteer, map[string]any{"sessionId": id, "steer": key, "state": state})
		})
		if err != nil && !errors.Is(err, acp.ErrNoActiveTurn) {
			submission.Drop(key)
			state := "failed"
			if errors.Is(err, acp.ErrSteeringRejected) {
				state = "rejected"
			}
			t.host.SteeringState(id, ledgerID, state)
			EmitChat(t.host.Emit, s, EventSteer, map[string]any{"sessionId": id, "steer": key, "state": state})
			return err
		}
	}
	t.host.Remember(s.Cwd(), s.ID(), "steer", text, doc)
	return nil
}

func (t *TurnIO) steeringPromptDelivered(s *Conversation, p *Prompt) {
	if p == nil {
		return
	}
	s.MarkDelivered(*p)
	kind := ""
	if p.Asked() || p.Label() == busChatLabel {
		kind = SentKind(p, "asked")
	}
	if kind != "" {
		EmitChat(t.host.Emit, s, EventSent, SentEvent(s.ID(), p.Text(), kind, p.Label(), p.Delivered()))
	}
}

func steeringPromptLabel(p Prompt) string {
	if p.Label() != "" {
		return p.Label()
	}
	return "You, steering"
}
