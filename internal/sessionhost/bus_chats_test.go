package sessionhost

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"studio/internal/bus"
	"studio/internal/bushost"
)

func TestBusPresenceFailureKeepsLifecycleAndEventOrder(t *testing.T) {
	var mu sync.Mutex
	var runtime Runtime
	now := time.Unix(10000, 0)
	c := NewConversation(Identity{ID: "chat", Cwd: t.TempDir()}, InitialActivity{Open: true}, false, false)
	c.BackgroundAt(func() time.Time { return now.Add(-BackgroundIdle) })
	c.JoinBus("previous")
	runtime.PutConversationLocked(c.ID(), c)
	service := bushost.New(bushost.Options{State: t.TempDir()})
	service.CloseHandle() // Both departure and registration fail through the real service.
	var events []string
	unlocked := func() {
		t.Helper()
		if !mu.TryLock() {
			t.Fatal("host effect ran under runtime state lock")
		}
		mu.Unlock()
	}
	owner := NewBusChats(&mu, &runtime, BusChatsHost{
		Clock: func() time.Time { events = append(events, "clock"); return now },
		BusService: func() *bushost.Service {
			unlocked()
			if c.BusID() != "" {
				t.Fatal("departure did not clear identity before transport")
			}
			events = append(events, "bus")
			return service
		},
		AllocateBusIdentity: func(string) string {
			unlocked()
			if c.Parked() {
				t.Fatal("registration preceded unpark")
			}
			events = append(events, "identity")
			return "next"
		},
		SaveBusIdentity: func(string, string) { t.Fatal("failed registration saved identity") },
		ShownTitle:      func(*Conversation) string { t.Fatal("closed bus reached title lookup"); return "" },
		Log:             func(string, string, ...any) { unlocked(); events = append(events, "log") },
		Emit: func(name string, value any) {
			unlocked()
			if name != "agent:session" {
				t.Fatal(name)
			}
			info := value.(Info)
			if info.Parked != c.Parked() {
				t.Fatal("event preceded lifecycle mutation")
			}
			events = append(events, "event")
		},
	})
	owner.CloseIdleBackground()
	if !c.Parked() || c.BusID() != "" {
		t.Fatal("idle chat did not depart")
	}
	owner.Unpark(c)
	if c.Parked() || !c.Info().InBackground || !c.IdleSince().Equal(now) || c.BusID() != "" {
		t.Fatal("registration failure changed background/event behavior")
	}
	want := []string{"clock", "clock", "bus", "event", "identity", "bus", "log", "clock", "event"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("effects = %v, want %v", events, want)
	}
}

func TestBusJoinPublishesOnlySuccessfulTitleAndSavesAfterIdentity(t *testing.T) {
	var mu sync.Mutex
	var runtime Runtime
	c := NewConversation(Identity{ID: "chat", Cwd: t.TempDir()}, InitialActivity{Open: true}, false, false)
	service := bushost.New(bushost.Options{State: t.TempDir()})
	t.Cleanup(service.CloseHandle)
	var effects []string
	owner := NewBusChats(&mu, &runtime, BusChatsHost{
		BusService: func() *bushost.Service { return service },
		SaveBusIdentity: func(chat, id string) {
			if !mu.TryLock() {
				t.Fatal("save ran under state lock")
			}
			mu.Unlock()
			if chat != c.ID() || c.BusID() != id {
				t.Fatal("save preceded join")
			}
			effects = append(effects, "save")
		},
		ShownTitle: func(*Conversation) string { effects = append(effects, "title"); return "Title" },
		Log:        func(string, string, ...any) { effects = append(effects, "log") },
	})
	if err := service.With(func(b *bus.Bus) error {
		_, err := b.RegisterSession(context.Background(), "id", c.Cwd(), "")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	owner.JoinBus(c, "id")
	if _, same := c.BusTitleState("Title"); !same {
		t.Fatal("successful title not recorded")
	}
	if !reflect.DeepEqual(effects, []string{"save", "title"}) {
		t.Fatal(effects)
	}
	service.CloseHandle()
	owner.JoinBus(c, "replacement")
	if _, same := c.BusTitleState("Title"); same {
		t.Fatal("failed title marked published")
	}
	if !reflect.DeepEqual(effects, []string{"save", "title", "save", "title", "log"}) {
		t.Fatal(effects)
	}
}
func TestBusWakeHiddenPolicySharesAdmissionTransaction(t *testing.T) {
	var mu sync.Mutex
	var runtime Runtime
	runtime.SetWindowsLocked(nil)
	service := bushost.New(bushost.Options{State: t.TempDir()})
	t.Cleanup(service.CloseHandle)
	b, err := service.Bus()
	if err != nil {
		t.Fatal(err)
	}
	s, err := b.RegisterSession(context.Background(), "target", t.TempDir(), "Target")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = b.Send(context.Background(), bus.Sender{}, s.Address(), "Wake", nil, 0); err != nil {
		t.Fatal(err)
	}
	if err = b.EndSession(context.Background(), "target"); err != nil {
		t.Fatal(err)
	}
	checked := false
	unlocked := func() {
		t.Helper()
		if !mu.TryLock() {
			t.Fatal("host callback ran under state lock")
		}
		mu.Unlock()
	}
	owner := NewBusChats(&mu, &runtime, BusChatsHost{
		Clock:        time.Now,
		EndClosed:    unlocked,
		HoldingTurns: func() error { unlocked(); return nil },
		ChatOfBus: func(id string) string {
			unlocked()
			if id != "target" {
				t.Fatal(id)
			}
			return "chat"
		},
		HiddenLocked: func(id string) bool {
			if mu.TryLock() {
				mu.Unlock()
				t.Fatal("hidden policy escaped admission lock")
			}
			if id != "chat" {
				t.Fatal(id)
			}
			checked = true
			return true
		},
		Emit: func(string, any) { t.Fatal("hidden chat was admitted") },
		Log:  func(string, string, ...any) { t.Fatal("unexpected bus error") },
	})
	owner.ConnectWaking(context.Background(), b)
	if !checked {
		t.Fatal("hidden policy was not consulted")
	}
}
