package bus

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func personRequestFixture(t *testing.T) (*Bus, Session, Decision) {
	t.Helper()
	b, root := open(t)
	s, err := b.RegisterSession(context.Background(), WindowSessionPrefix+"asker", workspace(t, root, "work"), "Asking chat")
	if err != nil {
		t.Fatal(err)
	}
	d, err := b.AskPerson(context.Background(), s, AskPersonInput{OperationID: "ask-1", Questions: []PersonQuestion{
		{Number: 2, Text: "Which release should we use?", Recommendation: "Use the reviewed release."},
		{Number: 7, Text: "When should it start?", Recommendation: "Start tomorrow.", Choices: []string{"Today", "Tomorrow"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return b, s, d
}

func TestDecisionPartialAnswerPersistenceAndReceipt(t *testing.T) {
	ctx := context.Background()
	b, s, d := personRequestFixture(t)
	if err := b.EndSession(ctx, s.ID); err != nil {
		t.Fatal(err)
	}
	input := AnswerDecisionInput{OperationID: "answer-1", ID: d.ID, Revision: 1, Answers: []DecisionAnswer{{Number: 7, AsRecommended: true}}}
	r, err := b.AnswerDecision(ctx, input)
	if err != nil || r.Closed || r.Revision != 2 || len(r.MessageIDs) != 1 {
		t.Fatalf("partial %+v %v", r, err)
	}
	m, err := b.Message(ctx, r.MessageIDs[0])
	if err != nil || m.FromName != Human || m.ReplyTo != d.MessageID || !strings.Contains(m.Text, "Question 7. When should it start?") || !strings.Contains(m.Text, "Start tomorrow.") {
		t.Fatalf("reply %+v %v", m, err)
	}
	var target string
	if err := b.db.QueryRow(`SELECT recipient FROM deliveries WHERE message_id=? AND recipient_kind='session'`, m.ID).Scan(&target); err != nil || target != s.ID {
		t.Fatalf("closed asker target %q %v", target, err)
	}
	page, err := b.Decisions(ctx, 0)
	if err != nil || len(page.Items) != 1 || page.Items[0].Revision != 2 || page.Items[0].Questions[0].Answered || !page.Items[0].Questions[1].Answered {
		t.Fatalf("partial list %+v %v", page, err)
	}
	if stale, err := b.AnswerDecision(ctx, AnswerDecisionInput{OperationID: "stale", ID: d.ID, Revision: 1, Answers: []DecisionAnswer{{Number: 2, Text: "Use another release."}}}); err != nil || stale.State != "rejected" || stale.Reason.Code != "revision_changed" {
		t.Fatalf("stale answer %+v %v", stale, err)
	}
	// A second connection models reopening the window after losing the response.
	reopened, err := OpenAt(filepath.Join(b.dir, "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.DecisionOperation(ctx, input.OperationID)
	if err != nil || !reflect.DeepEqual(got, r) {
		t.Fatalf("reconciliation %+v %v", got, err)
	}
	got, err = reopened.AnswerDecision(ctx, input)
	if err != nil || !reflect.DeepEqual(got, r) {
		t.Fatalf("retry %+v %v", got, err)
	}
	input.Answers[0] = DecisionAnswer{Number: 2, Text: "Changed input"}
	if _, err = b.AnswerDecision(ctx, input); err == nil {
		t.Fatal("operation collision accepted")
	}
	final, err := b.AnswerDecision(ctx, AnswerDecisionInput{OperationID: "answer-2", ID: d.ID, Revision: 2, Answers: []DecisionAnswer{{Number: 2, Text: "Use the current release."}}})
	if err != nil || !final.Closed || final.Revision != 3 {
		t.Fatalf("final %+v %v", final, err)
	}
	page, err = b.Decisions(ctx, 0)
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("closed list %+v %v", page, err)
	}
	unknown, err := b.DecisionOperation(ctx, "not-attempted")
	if err != nil || unknown.State != "unknown" || unknown.MessageIDs == nil {
		t.Fatalf("unknown %+v %v", unknown, err)
	}
}

func TestAskPersonIdempotencyAndInputValidation(t *testing.T) {
	ctx := context.Background()
	b, s, d := personRequestFixture(t)
	in := AskPersonInput{OperationID: "ask-1", Questions: []PersonQuestion{{Number: 2, Text: "Which release should we use?", Recommendation: "Use the reviewed release."}, {Number: 7, Text: "When should it start?", Recommendation: "Start tomorrow.", Choices: []string{"Today", "Tomorrow"}}}}
	again, err := b.AskPerson(ctx, s, in)
	if err != nil || again.ID != d.ID {
		t.Fatalf("retry %+v %v", again, err)
	}
	in.Questions[0].Recommendation = "Different recommendation"
	if _, err = b.AskPerson(ctx, s, in); err == nil {
		t.Fatal("changed retry accepted")
	}
	for name, questions := range map[string][]PersonQuestion{
		"empty": {}, "missing recommendation": {{Number: 1, Text: "Question"}},
		"duplicate":        {{Number: 1, Text: "Q", Recommendation: "R"}, {Number: 1, Text: "Q", Recommendation: "R"}},
		"zero":             {{Number: 0, Text: "Q", Recommendation: "R"}},
		"answer injection": {{Number: 1, Text: "Q", Recommendation: "R", Answered: true}},
		"empty choice":     {{Number: 1, Text: "Q", Recommendation: "R", Choices: []string{""}}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := b.AskPerson(ctx, s, AskPersonInput{OperationID: name, Questions: questions}); err == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
	page, _ := b.Decisions(ctx, 0)
	if len(page.Items) != 1 {
		t.Fatalf("failed calls left rows %+v", page)
	}
}

func TestDecisionGroupAnswerAndPlainMentionResolution(t *testing.T) {
	ctx := context.Background()
	b, s, _ := personRequestFixture(t)
	other, err := b.RegisterSession(ctx, "window-other", s.WorkspacePath, "Other")
	if err != nil {
		t.Fatal(err)
	}
	outsider, err := b.RegisterSession(ctx, "window-outsider", s.WorkspacePath, "Outsider")
	if err != nil {
		t.Fatal(err)
	}
	room, err := b.CreateGroup(ctx, Sender{}, "Decisions fixture", []string{s.ID, other.ID})
	if err != nil {
		t.Fatal(err)
	}
	request := AskPersonInput{OperationID: "group-request", Group: "#" + room.Name, Questions: []PersonQuestion{{Number: 1, Text: "May we proceed?", Recommendation: "Proceed."}}}
	if _, err = b.AskPerson(ctx, outsider, request); err == nil {
		t.Fatal("outsider asked in group")
	}
	d, err := b.AskPerson(ctx, s, request)
	if err != nil {
		t.Fatal(err)
	}
	if d.Group != room.Name || d.Destination != "#"+room.Title {
		t.Fatalf("group DTO %+v", d)
	}
	if _, err = b.DismissDecision(ctx, DismissDecisionInput{OperationID: "bad-dismiss", ID: d.ID, Revision: 1}); err == nil {
		t.Fatal("structured request dismissed")
	}
	r, err := b.AnswerDecision(ctx, AnswerDecisionInput{OperationID: "group-answer", ID: d.ID, Revision: 1, Answers: []DecisionAnswer{{Number: 1, AsRecommended: true}}})
	if err != nil {
		t.Fatal(err)
	}
	m, err := b.Message(ctx, r.MessageIDs[0])
	if err != nil {
		t.Fatal(err)
	}
	if m.Room != room.Name || m.mentions != ","+s.ID+"," {
		t.Fatalf("answer did not address asker %+v", m)
	}
	mention := func(text string) Message {
		t.Helper()
		m, err := b.SendMentioning(ctx, Sender{Session: &s}, "#"+room.Name, text, nil, 0, []string{Human})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	m1, m2, m3 := mention("First decision"), mention("Second decision"), mention("Third decision")
	if _, err = b.Send(ctx, Sender{Session: &other}, "", "Another agent replied", nil, m1.ID); err != nil {
		t.Fatal(err)
	}
	if err = b.ChangeHumanInbox(ctx, m1.ID, false, false); err != nil {
		t.Fatal(err)
	}
	page, err := b.Decisions(ctx, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 4 {
		t.Fatalf("read or agent reply resolved attention: %+v", page)
	}
	if _, err = b.Send(ctx, Sender{}, "", "The person replied", nil, m1.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = b.DismissDecision(ctx, DismissDecisionInput{OperationID: "dismiss-2", ID: fmt.Sprintf("mention:%d", m2.ID), Revision: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err = b.AnswerDecision(ctx, AnswerDecisionInput{OperationID: "mention-answer", ID: fmt.Sprintf("mention:%d", m3.ID), Revision: 1, Answers: []DecisionAnswer{{Number: 0, Text: "Do this tomorrow."}}}); err != nil {
		t.Fatal(err)
	}
	page, err = b.Decisions(ctx, 0)
	if err != nil || len(page.Items) != 1 || page.Items[0].Kind != "request" {
		t.Fatalf("mention resolution %+v %v", page, err)
	}
}

func TestDecisionAnswerRollbackAndConcurrentRevision(t *testing.T) {
	ctx := context.Background()
	b, _, d := personRequestFixture(t)
	if _, err := b.AnswerDecision(ctx, AnswerDecisionInput{OperationID: "invalid", ID: d.ID, Revision: 1, Answers: []DecisionAnswer{{Number: 2, Text: "Valid"}, {Number: 99, Text: "Invalid"}}}); err == nil {
		t.Fatal("unknown question accepted")
	}
	page, _ := b.Decisions(ctx, 0)
	if page.Items[0].Questions[0].Answered || page.Items[0].Revision != 1 {
		t.Fatal("partial work escaped failed operation")
	}
	// Force receipt persistence to fail after reply insertion and request update.
	if _, err := b.db.Exec(`CREATE TRIGGER reject_receipt BEFORE INSERT ON decision_operations BEGIN SELECT RAISE(ABORT,'fixture'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := b.AnswerDecision(ctx, AnswerDecisionInput{OperationID: "rollback", ID: d.ID, Revision: 1, Answers: []DecisionAnswer{{Number: 2, Text: "Valid"}}}); err == nil {
		t.Fatal("receipt failure was hidden")
	}
	var replies int
	b.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE reply_to=?`, d.MessageID).Scan(&replies)
	if replies != 0 {
		t.Fatal("reply escaped rollback")
	}
	if _, err := b.db.Exec(`DROP TRIGGER reject_receipt`); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	states := make(chan string, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r, err := b.AnswerDecision(ctx, AnswerDecisionInput{OperationID: fmt.Sprint("race-", i), ID: d.ID, Revision: 1, Answers: []DecisionAnswer{{Number: 2, Text: "Choice"}}})
			if err != nil {
				states <- "error"
			} else {
				states <- r.State
			}
		}(i)
	}
	wg.Wait()
	close(states)
	successes := 0
	for state := range states {
		if state == "committed" {
			successes++
		} else if state != "rejected" {
			t.Fatal("concurrent operation did not settle")
		}
	}
	if successes != 1 {
		t.Fatalf("revision winners %d", successes)
	}
}

func TestDecisionPagingIncludesReadMentions(t *testing.T) {
	ctx := context.Background()
	b, s, _ := personRequestFixture(t)
	// A single transaction keeps this fixture cheap while exercising real inserts.
	err := b.tx(ctx, func(tx *sql.Tx) error {
		for i := 0; i < 103; i++ {
			r := resolved{label: "#fixture", room: "fixture", mentions: ",paul,", recipients: []recipient{{"human", Human}}}
			if _, err := b.insertMessage(tx, Sender{Session: &s}, "text", r, 0, "Question", "", "", nil); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	page, err := b.Decisions(ctx, 0)
	if err != nil || len(page.Items) != 100 || page.NextBefore == 0 {
		t.Fatalf("first page %+v %v", page, err)
	}
	next, err := b.Decisions(ctx, page.NextBefore)
	if err != nil || len(next.Items) != 4 || next.NextBefore != 0 {
		t.Fatalf("next page %+v %v", next, err)
	}
	if next.Items[0].MessageID >= page.Items[99].MessageID {
		t.Fatal("cursor repeated a row")
	}
}

func TestDecisionBroadcastPreviewFreezeExpirationAndReplay(t *testing.T) {
	ctx := context.Background()
	b, s, _ := personRequestFixture(t)
	now := time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC)
	b.now = func() time.Time { return now }
	ended, err := b.RegisterSession(ctx, "window-ended", s.WorkspacePath, "Closed chat")
	if err != nil {
		t.Fatal(err)
	}
	external, err := b.RegisterSession(ctx, "external", s.WorkspacePath, "External chat")
	if err != nil {
		t.Fatal(err)
	}
	oldExternal, err := b.RegisterSession(ctx, "ended-external", s.WorkspacePath, "Old external")
	if err != nil {
		t.Fatal(err)
	}
	b.EndSession(ctx, ended.ID)
	b.EndSession(ctx, oldExternal.ID)
	if err = b.BeginHookTurn(ctx, external.ID, "fixture-turn"); err != nil {
		t.Fatal(err)
	}
	active, err := b.PreviewDecisionBroadcast(ctx, "active", "Pause your work.", []string{s.ID})
	if err != nil || active.Count != 2 {
		t.Fatalf("active %+v %v", active, err)
	}
	all, err := b.PreviewDecisionBroadcast(ctx, "all", "A message for every chat.", nil)
	if err != nil || all.Count != 3 {
		t.Fatalf("all %+v %v", all, err)
	}
	var count int
	b.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE from_session=''`).Scan(&count)
	if count != 0 {
		t.Fatal("preview sent a message")
	}
	late, err := b.RegisterSession(ctx, "window-late", s.WorkspacePath, "Late arrival")
	if err != nil {
		t.Fatal(err)
	}
	b.SetTitle(ctx, ended.ID, "Renamed after preview")
	input := ConfirmDecisionBroadcastInput{OperationID: "broadcast-1", Token: all.Token}
	receipt, err := b.ConfirmDecisionBroadcast(ctx, input)
	if err != nil || len(receipt.MessageIDs) != 3 {
		t.Fatalf("confirm %+v %v", receipt, err)
	}
	var lateCount int
	b.db.QueryRow(`SELECT COUNT(*) FROM deliveries WHERE recipient=?`, late.ID).Scan(&lateCount)
	if lateCount != 0 {
		t.Fatal("broadcast retargeted to arrival")
	}
	var closedCount int
	b.db.QueryRow(`SELECT COUNT(*) FROM deliveries WHERE recipient=? AND delivered_at IS NULL`, ended.ID).Scan(&closedCount)
	if closedCount != 1 {
		t.Fatal("closed chat did not receive queued message")
	}
	now = now.Add(6 * time.Minute)
	retry, err := b.ConfirmDecisionBroadcast(ctx, input)
	if err != nil || !reflect.DeepEqual(retry, receipt) {
		t.Fatalf("retry after expiry %+v %v", retry, err)
	}
	if r, err := b.ConfirmDecisionBroadcast(ctx, ConfirmDecisionBroadcastInput{OperationID: "different-operation", Token: all.Token}); err != nil || r.State != "rejected" || r.Reason.Code != "preview_consumed" {
		t.Fatalf("consumed preview %+v %v", r, err)
	}
	if r, err := b.ConfirmDecisionBroadcast(ctx, ConfirmDecisionBroadcastInput{OperationID: "expired", Token: active.Token}); err != nil || r.State != "rejected" || r.Reason.Code != "preview_expired" {
		t.Fatalf("expired preview %+v %v", r, err)
	}
}

func TestDecisionRejectionsPersistAndMalformedInputStaysUnknown(t *testing.T) {
	ctx := context.Background()
	b, s, d := personRequestFixture(t)
	first, err := b.AnswerDecision(ctx, AnswerDecisionInput{OperationID: "first-answer", ID: d.ID, Revision: 1, Answers: []DecisionAnswer{{Number: 7, AsRecommended: true}}})
	if err != nil || first.State != "committed" {
		t.Fatalf("first %+v %v", first, err)
	}
	input := AnswerDecisionInput{OperationID: "answered-again", ID: d.ID, Revision: 2, Answers: []DecisionAnswer{{Number: 7, Text: "Another answer"}}}
	rejected, err := b.AnswerDecision(ctx, input)
	if err != nil || rejected.State != "rejected" || rejected.Reason.Code != "already_answered" || len(rejected.MessageIDs) != 0 {
		t.Fatalf("rejection %+v %v", rejected, err)
	}
	saved, err := b.DecisionOperation(ctx, input.OperationID)
	if err != nil || !reflect.DeepEqual(saved, rejected) {
		t.Fatalf("saved rejection %+v %v", saved, err)
	}
	retry, err := b.AnswerDecision(ctx, input)
	if err != nil || !reflect.DeepEqual(retry, rejected) {
		t.Fatalf("repeated rejection %+v %v", retry, err)
	}
	page, _ := b.Decisions(ctx, 0)
	if len(page.Items) != 1 || page.Items[0].Revision != 2 {
		t.Fatal("rejection changed request")
	}
	invalid := AnswerDecisionInput{OperationID: "malformed", ID: d.ID, Revision: 1, Answers: []DecisionAnswer{{Number: 2, Text: "", AsRecommended: false}}}
	if _, err = b.AnswerDecision(ctx, invalid); err == nil {
		t.Fatal("malformed answer accepted")
	}
	unknown, err := b.DecisionOperation(ctx, invalid.OperationID)
	if err != nil || unknown.State != "unknown" {
		t.Fatalf("malformed operation stored %+v %v", unknown, err)
	}
	room, err := b.CreateGroup(ctx, Sender{}, "Dismissal fixture", []string{s.ID})
	if err != nil {
		t.Fatal(err)
	}
	mention, err := b.SendMentioning(ctx, Sender{Session: &s}, "#"+room.Name, "Please choose.", nil, 0, []string{Human})
	if err != nil {
		t.Fatal(err)
	}
	dismiss := DismissDecisionInput{OperationID: "dismiss-once", ID: fmt.Sprintf("mention:%d", mention.ID), Revision: 1}
	r, err := b.DismissDecision(ctx, dismiss)
	if err != nil || r.State != "committed" {
		t.Fatalf("dismiss %+v %v", r, err)
	}
	again, err := b.DismissDecision(ctx, dismiss)
	if err != nil || !reflect.DeepEqual(again, r) {
		t.Fatalf("dismiss retry %+v %v", again, err)
	}
	dismiss.OperationID = "dismiss-closed"
	r, err = b.DismissDecision(ctx, dismiss)
	if err != nil || r.State != "rejected" || r.Reason.Code != "decision_closed" {
		t.Fatalf("closed dismissal %+v %v", r, err)
	}
	reopened, err := OpenAt(filepath.Join(b.dir, "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	saved, err = reopened.DecisionOperation(ctx, input.OperationID)
	if err != nil || !reflect.DeepEqual(saved, rejected) {
		t.Fatalf("reopened rejection %+v %v", saved, err)
	}
}

func TestDecisionRemovedDestinationsRejectWithoutDelivery(t *testing.T) {
	ctx := context.Background()
	b, s, d := personRequestFixture(t)
	preview, err := b.PreviewDecisionBroadcast(ctx, "all", "Frozen targets", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = b.db.Exec(`UPDATE sessions SET removed_at=? WHERE id=?`, b.stamp(), s.ID); err != nil {
		t.Fatal(err)
	}
	answer, err := b.AnswerDecision(ctx, AnswerDecisionInput{OperationID: "removed-answer", ID: d.ID, Revision: 1, Answers: []DecisionAnswer{{Number: 2, AsRecommended: true}}})
	if err != nil || answer.State != "rejected" || answer.Reason.Code != "decision_unavailable" {
		t.Fatalf("removed answer %+v %v", answer, err)
	}
	broadcast, err := b.ConfirmDecisionBroadcast(ctx, ConfirmDecisionBroadcastInput{OperationID: "removed-broadcast", Token: preview.Token})
	if err != nil || broadcast.State != "rejected" || broadcast.Reason.Code != "preview_unavailable" {
		t.Fatalf("removed broadcast %+v %v", broadcast, err)
	}
	var sent int
	if err = b.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE from_session=''`).Scan(&sent); err != nil || sent != 0 {
		t.Fatalf("unavailable destinations sent %d %v", sent, err)
	}
}

func TestDecisionRetiredChatsExcludedFromPreview(t *testing.T) {
	ctx := context.Background()
	b, s, _ := personRequestFixture(t)
	lead, err := b.RegisterSession(ctx, "window-lead", s.WorkspacePath, "Lead")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = b.SetLead(ctx, s.ID, lead.ID); err != nil {
		t.Fatal(err)
	}
	if err = b.SetRetired(ctx, s.ID, true); err != nil {
		t.Fatal(err)
	}
	preview, err := b.PreviewDecisionBroadcast(ctx, "all", "Everyone currently available", nil)
	if err != nil || preview.Count != 1 || preview.Targets[0].Session != lead.ID {
		t.Fatalf("retired preview %+v %v", preview, err)
	}
}

func TestDecisionBroadcastRollbackAndRemovedTarget(t *testing.T) {
	ctx := context.Background()
	b, s, _ := personRequestFixture(t)
	other, err := b.RegisterSession(ctx, "window-removed", s.WorkspacePath, "Removed chat")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = b.db.Exec(`UPDATE sessions SET removed_at=? WHERE id=?`, b.stamp(), other.ID); err != nil {
		t.Fatal(err)
	}
	preview, err := b.PreviewDecisionBroadcast(ctx, "all", "Message", nil)
	if err != nil || preview.Count != 1 {
		t.Fatalf("removed preview %+v %v", preview, err)
	}
	if _, err = b.db.Exec(`CREATE TRIGGER reject_broadcast_receipt BEFORE INSERT ON decision_operations BEGIN SELECT RAISE(ABORT,'fixture'); END`); err != nil {
		t.Fatal(err)
	}
	in := ConfirmDecisionBroadcastInput{OperationID: "atomic-broadcast", Token: preview.Token}
	if _, err = b.ConfirmDecisionBroadcast(ctx, in); err == nil {
		t.Fatal("receipt failure hidden")
	}
	var sent int
	if err = b.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE from_session=''`).Scan(&sent); err != nil || sent != 0 {
		t.Fatalf("failed broadcast sent %d %v", sent, err)
	}
	if _, err = b.db.Exec(`DROP TRIGGER reject_broadcast_receipt`); err != nil {
		t.Fatal(err)
	}
	r, err := b.ConfirmDecisionBroadcast(ctx, in)
	if err != nil || r.State != "committed" || len(r.MessageIDs) != 1 {
		t.Fatalf("rolled back token could not retry %+v %v", r, err)
	}
}
