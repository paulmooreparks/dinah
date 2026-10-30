package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"studio/internal/acp"
	"studio/internal/bus"
)

type steeringTestConnection struct {
	*testProviderConnection
	receive     func(acp.PromptParams, func()) error
	unsupported bool
}

func (c *steeringTestConnection) CanSteer(string) bool { return !c.unsupported }

func TestDefaultSendingChecksTheSelectedRuntimeCapability(t *testing.T) {
	a, base, s, me, _ := busChat(t)
	conn := &steeringTestConnection{testProviderConnection: base}
	setTestConnection(a, "codex", conn)
	if !a.CanSteer(s.ID(), nil) {
		t.Fatal("a supported model did not enable default steering")
	}
	conn.unsupported = true
	if a.CanSteer(s.ID(), nil) {
		t.Fatal("an unsupported model enabled default steering")
	}
	setTestConnection(a, "codex", base)
	if a.CanSteer(s.ID(), nil) || a.CanSteer("missing", nil) {
		t.Fatal("an absent runtime capability enabled default steering")
	}
	s = replaceTestIdentity(a, s, "Provider", "claude")
	if a.CanSteer(s.ID(), nil) {
		t.Fatal("unreached hooks enabled default steering")
	}
	if err := a.busHost().With(func(b *bus.Bus) error {
		return b.BindAgentSession(context.Background(), "claude-agent", me.ID)
	}); err != nil {
		t.Fatal(err)
	}
	if !a.CanSteer(s.ID(), nil) || a.CanSteer(s.ID(), []Attachment{{Kind: "file"}}) {
		t.Fatal("hook support did not account for attachments")
	}
}

func TestDevinSteeringUsesReachedHooksWithoutCancelling(t *testing.T) {
	a, base, s, me, _ := busChat(t)
	s = replaceTestIdentity(a, s, "Provider", "devin")
	s.StartWork()
	if marker := steerMarker(s); marker != bus.SteerMarker(me.ID) {
		t.Fatalf("Devin prompt marker: %q", marker)
	}
	if err := a.busHost().With(func(b *bus.Bus) error {
		return b.BindAgentSession(context.Background(), "devin:agent", me.ID)
	}); err != nil {
		t.Fatal(err)
	}
	if err := a.Steer(s.ID(), "Keep the API.", nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := a.busHost().With(func(b *bus.Bus) error {
		steers, err := b.TakeSteers(context.Background(), me.ID)
		if len(steers) != 1 || !strings.Contains(steers[0].Text, "Keep the API.") {
			t.Fatalf("Devin hook queue: %+v", steers)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if late := a.turnIO().SettleSteers(s); len(late) != 0 {
		t.Fatal("a delivered Devin steer was queued again")
	}
	for _, call := range base.calls {
		if call.method == "session/cancel" {
			t.Fatal("Devin steering cancelled the turn")
		}
	}
}

func (c *steeringTestConnection) Steer(_ context.Context, p acp.PromptParams, delivered func()) error {
	return c.receive(p, delivered)
}

func TestNativeSteeringDeliveryAndFallback(t *testing.T) {
	for _, mode := range []string{"accepted", "pending", "idle", "uncertain", "rejected", "unsupported"} {
		t.Run(mode, func(t *testing.T) {
			a, base, s, _, _ := busChat(t)
			s.StartWork()
			var states []string
			a.sink = func(name string, data any) {
				if name == "agent:steer" {
					states = append(states, data.(map[string]any)["state"].(string))
				}
			}
			conn := &steeringTestConnection{testProviderConnection: base, receive: func(p acp.PromptParams, delivered func()) error {
				if len(p.Prompt) == 0 || !strings.Contains(p.Prompt[0].Text, "Keep the API.") {
					t.Fatalf("input: %+v", p)
				}
				switch mode {
				case "accepted":
					delivered()
				case "idle":
					return acp.ErrNoActiveTurn
				case "rejected":
					return errors.Join(acp.ErrSteeringRejected, errors.New("invalid input"))
				case "uncertain":
					return errors.New("connection lost after submission")
				}
				return nil
			}}
			if mode != "unsupported" {
				setTestConnection(a, "codex", conn)
			}
			err := a.Steer(s.ID(), "Keep the API.", nil, nil)
			if (err != nil) != (mode == "uncertain" || mode == "rejected") {
				t.Fatalf("steer: %v", err)
			}
			late := a.turnIO().SettleSteers(s)
			wantLate := mode == "pending" || mode == "idle" || mode == "unsupported"
			if (len(late) == 1) != wantLate {
				t.Fatalf("late: %+v", late)
			}
			if wantLate && (!late[0].Late() || !strings.Contains(promptText(&late[0]), "Keep the API.")) {
				t.Fatalf("lost late input: %+v", late)
			}
			for _, call := range base.calls {
				if call.method == "session/cancel" {
					t.Fatal("Steer cancelled the turn")
				}
			}
			want := "pending,queued"
			if mode == "accepted" {
				want = "pending,accepted"
			}
			if mode == "uncertain" {
				want = "pending,failed"
			}
			if mode == "rejected" {
				want = "pending,rejected"
			}
			if strings.Join(states, ",") != want {
				t.Fatalf("states %v, want %s", states, want)
			}
			if late := a.turnIO().SettleSteers(s); len(late) != 0 {
				t.Fatal("late steer sent twice")
			}
		})
	}
}

func TestUnconfirmedPersistenceIsRecoverableWithoutFallback(t *testing.T) {
	a, base, s, _, _ := busChat(t)
	s.StartWork()
	setTestConnection(a, "codex", &steeringTestConnection{testProviderConnection: base, receive: func(_ acp.PromptParams, _ func()) error { return nil }})
	if err := a.Steer(s.ID(), "Retain this guidance", nil, nil); err != nil {
		t.Fatal(err)
	}
	submission := s.BeginSubmission()
	defer submission.Release()
	a.turnIO().UnconfirmNativeSteers(s, submission)
	late := a.turnIO().SettleSteersWithSubmission(s, submission)
	submission.Release()
	if len(late) != 0 {
		t.Fatal("unconfirmed persistence was queued again")
	}
	s.ReleaseWork()
	records, err := a.RecoverSteering(s.ID())
	if err != nil || len(records) != 1 || records[0].State != "failed" || !strings.Contains(records[0].Payload, "Retain this guidance") {
		t.Fatalf("recovery: %+v %v", records, err)
	}
	if err := a.DismissSteering(s.ID(), records[0].ID); err != nil {
		t.Fatal(err)
	}
	records, err = a.RecoverSteering(s.ID())
	if err != nil || len(records) != 0 {
		t.Fatalf("dismissed recovery: %+v %v", records, err)
	}
}
