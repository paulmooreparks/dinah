package sessionhost

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"studio/internal/bus"
	"studio/internal/bushost"
)

// A chat that is working is not steered by a group's notice, which waits for its
// next turn so that notices that come meanwhile arrive together. A message that
// asks the chat to answer still steers it.
func TestOnlyActionRequestsStartOrSteerChats(t *testing.T) {
	for _, busy := range []bool{false, true} {
		t.Run(map[bool]string{false: "idle", true: "busy"}[busy], func(t *testing.T) {
			ctx := context.Background()
			service := bushost.New(bushost.Options{State: t.TempDir()})
			t.Cleanup(service.CloseHandle)
			b, err := service.Bus()
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			sender, err := b.RegisterSession(ctx, "sender", dir, "Sender")
			if err != nil {
				t.Fatal(err)
			}
			target, err := b.RegisterSession(ctx, "target", dir, "Target")
			if err != nil {
				t.Fatal(err)
			}
			g, err := b.CreateGroup(ctx, bus.Sender{}, "Work", []string{"Sender", "Target"})
			if err != nil {
				t.Fatal(err)
			}

			var mu sync.Mutex
			var runtime Runtime
			c := NewConversation(Identity{ID: "chat"}, InitialActivity{Open: true}, false, false)
			c.JoinBus("target")
			prepared := 0
			stop := errors.New("stop after preparing")
			owner := NewBusChats(&mu, &runtime, BusChatsHost{
				HoldingTurns: func() error { return nil },
				BusService:   func() *bushost.Service { return service },
				AgentPerson:  func() string { return "Person" },
				CanSteer:     func(string, []Attachment) bool { return true },
				BusPictures:  func(*Conversation, []bus.Message) ([]Attachment, string) { return nil, "" },
				PreparePrompt: func(_ *Conversation, text string, _ []Attachment) (Prompt, error) {
					if !strings.Contains(text, "No one in particular.") || !strings.Contains(text, "Target, please look.") {
						t.Fatalf("request omitted pending context: %s", text)
					}
					prepared++
					return Prompt{}, stop
				},
				Emit: func(string, any) {},
				Log:  func(string, string, ...any) {},
			})
			if busy {
				if ready, working := c.ClaimDelivery(); !ready || working {
					t.Fatalf("the chat could not be made to work: %v %v", ready, working)
				}

			}
			if _, err := b.Send(ctx, bus.Sender{Session: &sender}, "#"+g.Name, "No one in particular.", nil, 0); err != nil {
				t.Fatal(err)
			}
			owner.Ring(ctx, b, c)
			if p := owner.AfterTurn(c); p != nil {
				t.Fatal("passive notice started an after-turn prompt")
			}
			if n, _, _ := b.Doorbell(ctx, target.ID); prepared != 0 || n != 0 {
				t.Fatalf("passive traffic prepared %d prompts or rang %d times", prepared, n)
			}

			if _, err := b.SendMentioning(ctx, bus.Sender{Session: &sender}, "#"+g.Name, "Target, please look.", nil, 0, []string{target.ID}); err != nil {
				t.Fatal(err)
			}
			owner.Ring(ctx, b, c)
			if n, _, _ := b.Doorbell(ctx, target.ID); prepared != 1 || n != 0 {
				t.Fatalf("a mention did not steer a working chat (%d prepared, %d waiting)", prepared, n)
			}
		})
	}
}
