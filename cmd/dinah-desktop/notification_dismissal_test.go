package main

import (
	"context"
	"testing"

	"studio/internal/bus"
)

// busTraffic is the newest message on the bus and the deliveries waiting for
// one chat. Neither changes unless somebody is sent something.
func busTraffic(t *testing.T, a *App, chat string) (int64, int) {
	t.Helper()
	var highest int64
	var waiting int
	if err := a.busHost().With(func(b *bus.Bus) error {
		var err error
		if highest, err = b.HighestMessage(context.Background()); err != nil {
			return err
		}
		waiting, err = b.PendingSessionDeliveries(context.Background(), chat)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return highest, waiting
}

// Dismissing or clearing a notice removes the notice and nothing else. The
// person decided on 17 November 2026 that it never settles the request the
// notice is about and tells the asking chat nothing.
func TestDismissingARequestNoticeLeavesTheRequestOpen(t *testing.T) {
	a, _, me, _, _, _ := mentionRig(t)
	pass(t, a)
	ask := func(operation, text string) bus.Decision {
		t.Helper()
		var request bus.Decision
		if err := a.busHost().With(func(b *bus.Bus) error {
			var err error
			request, err = b.AskPerson(context.Background(), me, bus.AskPersonInput{OperationID: operation, Questions: []bus.PersonQuestion{{Number: 1, Text: text, Recommendation: "Proceed."}}})
			return err
		}); err != nil {
			t.Fatal(err)
		}
		pass(t, a)
		return request
	}
	stillOpen := func(request bus.Decision, after string) {
		t.Helper()
		entries, err := a.DecisionEntries()
		if err != nil {
			t.Fatal(err)
		}
		listed := false
		for _, entry := range entries.Items {
			listed = listed || entry.ID == request.ID && entry.MessageID == request.MessageID
		}
		page, err := a.Decisions(0)
		if err != nil {
			t.Fatal(err)
		}
		inDecisions := false
		for _, item := range page.Items {
			inDecisions = inDecisions || item.ID == request.ID
		}
		if !listed || !inDecisions {
			t.Fatalf("%s took the request out of the open requests: %+v %+v", after, entries, page)
		}
		detail, err := a.DecisionByID(request.ID)
		if err != nil || detail.Closed || detail.Decision.Status != "open" || detail.Decision.Revision != request.Revision || detail.Decision.SettledAt != "" || detail.Decision.Questions[0].Answered {
			t.Fatalf("%s settled or answered the request: %+v %v", after, detail, err)
		}
		state, err := a.RequestState(request.ID)
		if err != nil || state.Status != "open" || state.Revision != request.Revision {
			t.Fatalf("%s changed the request's state: %+v %v", after, state, err)
		}
	}

	first := ask("notice-dismissed", "May the first step proceed?")
	cards := mentionCards(t, a)
	if len(cards) != 1 || cards[0].RequestID != first.ID || cards[0].Message != first.MessageID {
		t.Fatalf("the request did not make one notice of its own: %+v", cards)
	}
	highest, waiting := busTraffic(t, a, me.ID)
	if err := a.DismissNotification(cards[0].ID); err != nil {
		t.Fatal(err)
	}
	// A completion owed to the asking chat would be sent in the watcher's pass.
	pass(t, a)
	if left := mentionCards(t, a); len(left) != 0 {
		t.Fatalf("the dismissed notice is still listed: %+v", left)
	}
	stillOpen(first, "dismissing its notice")
	if h, w := busTraffic(t, a, me.ID); h != highest || w != waiting {
		t.Fatalf("dismissing a notice sent a message: newest %d then %d, waiting for the asking chat %d then %d", highest, h, waiting, w)
	}

	second := ask("notices-cleared", "May the second step proceed?")
	cards = mentionCards(t, a)
	if len(cards) != 1 || cards[0].RequestID != second.ID {
		t.Fatalf("the second request did not make one notice: %+v", cards)
	}
	highest, waiting = busTraffic(t, a, me.ID)
	if err := a.ClearNotifications(); err != nil {
		t.Fatal(err)
	}
	pass(t, a)
	if left := mentionCards(t, a); len(left) != 0 {
		t.Fatalf("a cleared notice is still listed: %+v", left)
	}
	stillOpen(first, "clearing the notices")
	stillOpen(second, "clearing the notices")
	if h, w := busTraffic(t, a, me.ID); h != highest || w != waiting {
		t.Fatalf("clearing the notices sent a message: newest %d then %d, waiting for the asking chat %d then %d", highest, h, waiting, w)
	}
}

// A report about a chat's turn is dismissed as a notice. Nothing is recorded
// as settled, and the chat is not told.
func TestDismissingAReportNoticeSettlesNothing(t *testing.T) {
	a, s, me, _, _, _ := mentionRig(t)
	pass(t, a)
	for _, clear := range []bool{false, true} {
		id := "report-dismissed"
		if clear {
			id = "report-cleared"
		}
		saved, err := a.SaveNotification(Notification{ID: id, Kind: "turn-failure", Severity: "error", Title: "The turn ended with an error.", Body: "An invented failure.", SessionID: s.ID(), Workspace: s.Cwd()})
		if err != nil || saved.RequestOwner != me.ID {
			t.Fatalf("the report is not owned by its chat: %+v %v", saved, err)
		}
		highest, waiting := busTraffic(t, a, me.ID)
		if clear {
			err = a.ClearNotifications()
		} else {
			err = a.DismissNotification(saved.ID)
		}
		if err != nil {
			t.Fatal(err)
		}
		if err := a.flushRequestCompletions(); err != nil {
			t.Fatal(err)
		}
		pass(t, a)
		items, err := a.Notifications()
		if err != nil || len(items) != 0 {
			t.Fatalf("the dismissed report is still listed: %+v %v", items, err)
		}
		notificationStoreMu.Lock()
		store, err := readNotifications()
		notificationStoreMu.Unlock()
		if err != nil || len(store.ReportSettlements) != 0 || len(store.ReportOperations) != 0 {
			t.Fatalf("dismissing a notice recorded a settlement: %+v %+v %v", store.ReportSettlements, store.ReportOperations, err)
		}
		if h, w := busTraffic(t, a, me.ID); h != highest || w != waiting {
			t.Fatalf("dismissing a report's notice sent a message: newest %d then %d, waiting for its chat %d then %d", highest, h, waiting, w)
		}
	}
}
