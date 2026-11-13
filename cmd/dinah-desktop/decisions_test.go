package main

import (
	"context"
	"fmt"
	"testing"

	"studio/internal/bus"
	"studio/internal/sessionhost"
)

func TestDecisionHostRoundTripAndBridgeClassification(t *testing.T) {
	a, _ := providerTestApp(t)
	var d bus.Decision
	err := a.busHost().With(func(b *bus.Bus) error {
		me, err := b.RegisterSession(context.Background(), "window-decision-asker", t.TempDir(), "Decision asker")
		if err != nil {
			return err
		}
		d, err = b.AskPerson(context.Background(), me, bus.AskPersonInput{OperationID: "host-ask", Questions: []bus.PersonQuestion{{Number: 1, Text: "May I proceed?", Recommendation: "Proceed."}}})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	page, err := a.Decisions(0)
	if err != nil || len(page.Items) != 1 || page.Items[0].ID != d.ID {
		t.Fatalf("page %+v %v", page, err)
	}
	receipt, err := a.AnswerDecision(bus.AnswerDecisionInput{OperationID: "host-answer", ID: d.ID, Revision: 1, Answers: []bus.DecisionAnswer{{Number: 1, AsRecommended: true}}})
	if err != nil || !receipt.Closed {
		t.Fatalf("answer %+v %v", receipt, err)
	}
	reconciled, err := a.DecisionOperation("host-answer")
	if err != nil || len(reconciled.MessageIDs) != 1 || reconciled.MessageIDs[0] != receipt.MessageIDs[0] {
		t.Fatalf("operation %+v %v", reconciled, err)
	}
	for _, name := range []string{"Decisions", "DecisionOperation", "PreviewDecisionBroadcast"} {
		if pageMethods[name] != pageQuery {
			t.Errorf("query classification %s", name)
		}
	}
	for _, name := range []string{"AnswerDecision", "DismissDecision", "ConfirmDecisionBroadcast"} {
		if pageMethods[name] != pageWork {
			t.Errorf("work classification %s", name)
		}
	}
}

func TestDecisionActiveBroadcastUsesRuntimeWorkingState(t *testing.T) {
	a, _ := providerTestApp(t)
	const native = "codex:active-broadcast"
	const session = "window-active-broadcast"
	a.saveBusSession(native, session)
	err := a.busHost().With(func(b *bus.Bus) error {
		_, err := b.RegisterSession(context.Background(), session, t.TempDir(), "Working now")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	a.runtime.PutConversationLocked(native, sessionhost.NewConversation(sessionhost.Identity{ID: native, Provider: "codex"}, sessionhost.InitialActivity{Open: true, Working: true}, false, false))
	a.mu.Unlock()
	preview, err := a.PreviewDecisionBroadcast("active", "Current work only")
	if err != nil || preview.Count != 1 || preview.Targets[0].Session != session {
		t.Fatalf("active preview %+v %v", preview, err)
	}
}

func TestDecisionBroadcastQueuesWithinBackgroundLimit(t *testing.T) {
	a, _ := providerTestApp(t)
	// No page or native window is created. The fake sink records connect requests.
	sink := watchConnects(a)
	a.settings.BackgroundChatLimit = 2
	a.ChatWindows(nil)
	err := a.busHost().With(func(b *bus.Bus) error {
		for i := 0; i < 5; i++ {
			id := fmt.Sprintf("codex:broadcast-%d", i)
			busID := bus.WindowSessionPrefix + fmt.Sprintf("codex-broadcast-%d", i)
			a.saveBusSession(id, busID)
			if _, err := b.RegisterSession(context.Background(), busID, t.TempDir(), fmt.Sprintf("Broadcast %d", i)); err != nil {
				return err
			}
			if err := b.EndSession(context.Background(), busID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := a.PreviewDecisionBroadcast("all", "Please follow the revised priority.")
	if err != nil || preview.Count != 5 {
		t.Fatalf("preview %+v %v", preview, err)
	}
	if got := sink.take(); len(got) != 0 {
		t.Fatal("preview requested a connection")
	}
	r, err := a.ConfirmDecisionBroadcast(bus.ConfirmDecisionBroadcastInput{OperationID: "host-broadcast", Token: preview.Token})
	if err != nil || len(r.MessageIDs) != 5 {
		t.Fatalf("confirmation %+v %v", r, err)
	}
	wakeRound(t, a)
	if got := sink.take(); len(got) != 2 {
		t.Fatalf("background limit admitted %d, want 2", len(got))
	}
	wakeRound(t, a)
	if got := sink.take(); len(got) != 0 {
		t.Fatalf("next pass exceeded background limit: %d", len(got))
	}
}
