package main

import (
	"context"
	"strings"
	"sync"
	"testing"

	"studio/internal/acp"
	"studio/internal/bus"
	"studio/internal/chatlog"
)

func TestNativeReceiptPreservesDeliveryEffectsAfterGateRelease(t *testing.T) {
	for _, deferred := range []bool{false, true} {
		t.Run(map[bool]string{false: "synchronous", true: "deferred"}[deferred], func(t *testing.T) {
			a, base, s, _, _ := busChat(t)
			s.StartWork()
			const body = "receipt delivery order"
			p := s.WithReply(context.Background(), newPrompt(body, []acp.ContentBlock{{Type: "text", Text: body}}).WithLabel("Other chat"))
			s.Reserve(p)
			var order []string
			pendingCount := func() int {
				n := 0
				if err := a.busHost().With(func(b *bus.Bus) error {
					rows, err := b.RecoverSteeringSubmissions(context.Background(), s.ID())
					n = len(rows)
					return err
				}); err != nil {
					t.Error(err)
				}
				return n
			}
			transcriptHasPrompt := func() bool {
				a.cmu.Lock()
				defer a.cmu.Unlock()
				for _, m := range s.HistorySavedLocked().Messages {
					if m.Text == body {
						return true
					}
				}
				return false
			}
			a.logs().chats = chatlog.New(t.TempDir(), func(text string) string {
				if text == body {
					if strings.Join(order, ",") != "sent" || pendingCount() != 0 || !transcriptHasPrompt() {
						t.Error("log preceded sent event, ledger or transcript")
					}
					order = append(order, "log")
				}
				return text
			})
			a.sink = func(name string, data any) {
				switch name {
				case "agent:sent":
					if pendingCount() != 1 || transcriptHasPrompt() {
						t.Error("sent event followed ledger or transcript")
					}
					order = append(order, "sent")
				case "agent:steer":
					if data.(map[string]any)["state"] == "accepted" {
						order = append(order, "accepted")
					}
				}
			}
			var confirm func()
			setTestConnection(a, "codex", &steeringTestConnection{testProviderConnection: base, receive: func(_ acp.PromptParams, delivered func()) error {
				confirm = delivered
				if !deferred {
					delivered()
				}
				return nil
			}})
			gate := s.BeginSubmission()
			defer gate.Release()
			if err := a.turnIO().SubmitSteer(s, gate, p, body, nil, nil); err != nil {
				t.Fatal(err)
			}
			gate.Release()
			var wg sync.WaitGroup
			for i := 0; i < 8; i++ {
				wg.Add(1)
				go func() { defer wg.Done(); confirm() }()
			}
			wg.Wait()
			if got := strings.Join(order, ","); got != "sent,log,accepted" {
				t.Fatalf("delivery order: %s", got)
			}
			if late := a.turnIO().SettleSteers(s); len(late) != 0 {
				t.Fatal("confirmed steer was queued again")
			}
		})
	}
}
