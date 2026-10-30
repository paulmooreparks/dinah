package sessionhost

import (
	"errors"
	"reflect"
	"studio/internal/bus"
	"testing"
)

func TestSteeringLedgerPayloadAndFailurePolicy(t *testing.T) {
	failure := errors.New("bus unavailable")
	var calls []string
	ledger := NewSteeringLedger(SteeringLedgerHost{
		Save: func(session, payload string) (int64, error) {
			if session != "chat" || payload != `{"text":"draft","attachments":null,"blocks":null}` {
				t.Fatalf("save %q %s", session, payload)
			}
			return 17, failure
		},
		SetState: func(session string, id int64, state string) error {
			if session != "chat" || id != 17 {
				t.Fatalf("state %q %d", session, id)
			}
			calls = append(calls, state)
			return failure
		},
		LogError: func(err error) {
			if err != failure {
				t.Fatal(err)
			}
			calls = append(calls, "logged")
		},
	})
	c := NewConversation(Identity{ID: "chat"}, InitialActivity{}, false, false)
	id, err := ledger.Save(c, "draft", nil, nil, Prompt{})
	if id != 17 || err != failure {
		t.Fatalf("save result %d %v", id, err)
	}
	ledger.State("chat", 0, "ignored")
	ledger.State("chat", 17, "sent")
	if err := ledger.Dismiss("chat", 17); err != failure {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"sent", "logged", "dismissed"}) {
		t.Fatal(calls)
	}
}

func TestSteeringLedgerRecoveryChecksWorkingAndQueueBeforeBus(t *testing.T) {
	for _, mode := range []string{"working", "queued", "idle", "missing"} {
		c := NewConversation(Identity{ID: "chat"}, InitialActivity{Working: mode == "working"}, false, false)
		if mode == "queued" {
			c.activity.EnqueueLocked(NewPrompt("queued", nil))
		}
		if mode == "missing" {
			c = nil
		}
		var order []string
		failure := errors.New("read failed")
		ledger := NewSteeringLedger(SteeringLedgerHost{
			Conversation: func(string) *Conversation { order = append(order, "lookup"); return c },
			Recover:      func(string) ([]bus.SteeringSubmission, error) { order = append(order, "recover"); return nil, failure },
		})
		got, err := ledger.Recover("chat")
		if mode == "working" || mode == "queued" {
			if got == nil || len(got) != 0 || err != nil || !reflect.DeepEqual(order, []string{"lookup"}) {
				t.Fatalf("busy: %#v %v %v", got, err, order)
			}
		} else if got != nil || err != failure || !reflect.DeepEqual(order, []string{"lookup", "recover"}) {
			t.Fatalf("idle: %#v %v %v", got, err, order)
		}
	}
}
