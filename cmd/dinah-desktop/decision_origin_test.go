package main

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"studio/internal/bus"
)

func TestDecisionOriginSurvivesWindowDeliveryAndProviderReplay(t *testing.T) {
	d := newDeliveredChat(t)
	ctx := context.Background()
	var request bus.Decision
	var reply bus.Message
	err := d.a.busHost().With(func(b *bus.Bus) error {
		var err error
		request, err = b.AskPerson(ctx, d.me, bus.AskPersonInput{OperationID: "replay-origin", Questions: []bus.PersonQuestion{{Number: 1, Text: "Should we continue?", Recommendation: "Continue tomorrow."}}})
		if err != nil {
			return err
		}
		receipt, err := b.AnswerDecision(ctx, bus.AnswerDecisionInput{OperationID: "replay-origin-answer", ID: request.ID, Revision: 1, Answers: []bus.DecisionAnswer{{Number: 1, AsRecommended: true}}})
		if err != nil {
			return err
		}
		reply, err = b.Message(ctx, receipt.MessageIDs[0])
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	view := busMessage(reply)
	if !reflect.DeepEqual(view.AnswerOrigin, reply.AnswerOrigin) {
		t.Fatal("window DTO lost origin")
	}
	detail, err := d.a.DecisionByID(request.ID)
	if err != nil || !detail.Closed || !detail.ReadOnly || detail.Decision.ID != request.ID || pageMethods["DecisionByID"] != pageQuery {
		t.Fatalf("lookup %+v %v", detail, err)
	}
	ring(t, d.a)
	waitTurn(t, d.turns)
	check := func(messages []ConversationMessage) {
		t.Helper()
		for _, m := range messages {
			if m.Delivered != nil && m.Delivered.ID == reply.ID {
				if !reflect.DeepEqual(m.Delivered.AnswerOrigin, reply.AnswerOrigin) {
					t.Fatalf("origin %+v", m.Delivered)
				}
				return
			}
		}
		t.Fatal("answer delivery absent")
	}
	check(d.saved(t))
	check(d.sentDelivered(t))
	d.replay(t, "A new unmatched prompt.")
	check(d.saved(t))
	found := false
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, raw := range d.page {
		var update struct {
			Kind    string              `json:"sessionUpdate"`
			Message ConversationMessage `json:"message"`
		}
		if err := json.Unmarshal(raw, &update); err != nil {
			t.Fatal(err)
		}
		if update.Kind == deliveredUpdate && update.Message.Delivered != nil && update.Message.Delivered.ID == reply.ID {
			found = reflect.DeepEqual(update.Message.Delivered.AnswerOrigin, reply.AnswerOrigin)
		}
	}
	if !found {
		t.Fatal("page replay lost typed origin")
	}
}
