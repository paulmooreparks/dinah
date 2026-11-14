package bus

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDecisionOriginSnapshotsPartialAnswersAndPersists(t *testing.T) {
	b, s, d := personRequestFixture(t)
	ctx := context.Background()
	in := AnswerDecisionInput{OperationID: "origin-partial", ID: d.ID, Revision: 1, Answers: []DecisionAnswer{{Number: 7, AsRecommended: true}}}
	first, err := b.AnswerDecision(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	again, err := b.AnswerDecision(ctx, in)
	if err != nil || !reflect.DeepEqual(first, again) {
		t.Fatalf("retry %+v %v", again, err)
	}
	m, err := b.Message(ctx, first.MessageIDs[0])
	if err != nil {
		t.Fatal(err)
	}
	want := &AnswerOrigin{Version: 1, RequestID: d.ID, MessageID: d.MessageID, Kind: "request", Questions: []AnsweredQuestion{{Number: 7, Text: d.Questions[1].Text, Answer: d.Questions[1].Recommendation, AsRecommended: true}}}
	if !reflect.DeepEqual(m.AnswerOrigin, want) {
		t.Fatalf("origin %+v", m.AnswerOrigin)
	}
	detail, err := b.DecisionByID(ctx, d.ID)
	if err != nil || detail.Closed || !detail.ReadOnly || detail.Decision.CanDismiss {
		t.Fatalf("partial lookup %+v %v", detail, err)
	}
	final, err := b.AnswerDecision(ctx, AnswerDecisionInput{OperationID: "origin-final", ID: d.ID, Revision: 2, Answers: []DecisionAnswer{{Number: 2, Text: d.Questions[0].Recommendation}}})
	if err != nil {
		t.Fatal(err)
	}
	last, err := b.Message(ctx, final.MessageIDs[0])
	if err != nil || len(last.AnswerOrigin.Questions) != 1 || last.AnswerOrigin.Questions[0].AsRecommended {
		t.Fatalf("explicit recommendation text %+v %v", last, err)
	}
	reopened, err := OpenAt(filepath.Join(b.dir, "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	persisted, err := reopened.Message(ctx, m.ID)
	if err != nil || !reflect.DeepEqual(persisted.AnswerOrigin, want) {
		t.Fatalf("persisted %+v %v", persisted, err)
	}
	pending, _, err := reopened.Arriving(ctx, s.ID, 20)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range pending {
		if p.ID == m.ID {
			found = reflect.DeepEqual(p.AnswerOrigin, want)
		}
	}
	if !found {
		t.Fatal("arriving lost origin")
	}
	delivered, _, err := reopened.Deliver(ctx, s.ID, "tool")
	if err != nil {
		t.Fatal(err)
	}
	found = false
	for _, p := range delivered {
		if p.ID == m.ID {
			found = reflect.DeepEqual(p.AnswerOrigin, want)
		}
	}
	if !found {
		t.Fatal("delivery lost origin")
	}
	counter, _ := b.ChangeCounter(ctx)
	detail, err = b.DecisionByID(ctx, d.ID)
	if err != nil || !detail.Closed || !detail.ReadOnly || detail.Decision.ID != d.ID || !detail.Decision.Questions[0].Answered {
		t.Fatalf("closed lookup %+v %v", detail, err)
	}
	after, _ := b.ChangeCounter(ctx)
	if counter != after {
		t.Fatal("lookup mutated bus")
	}
}

func TestDecisionOriginPlainMentionsAndLookupEligibility(t *testing.T) {
	b, s, _ := personRequestFixture(t)
	ctx := context.Background()
	group, err := b.CreateGroup(ctx, Sender{}, "Origin fixture", []string{s.ID})
	if err != nil {
		t.Fatal(err)
	}
	m, err := b.SendMentioning(ctx, Sender{Session: &s}, "#"+group.Name, "Choose the next step.", nil, 0, []string{Human})
	if err != nil {
		t.Fatal(err)
	}
	identity := fmt.Sprintf("mention:%d", m.ID)
	r, err := b.AnswerDecision(ctx, AnswerDecisionInput{OperationID: "plain-origin", ID: identity, Revision: 1, Answers: []DecisionAnswer{{Number: 0, Text: "Proceed tomorrow."}}})
	if err != nil {
		t.Fatal(err)
	}
	reply, err := b.Message(ctx, r.MessageIDs[0])
	if err != nil {
		t.Fatal(err)
	}
	want := &AnswerOrigin{Version: 1, RequestID: identity, MessageID: m.ID, Kind: "mention", Questions: []AnsweredQuestion{{Number: 0, Text: m.Text, Answer: "Proceed tomorrow."}}}
	if !reflect.DeepEqual(reply.AnswerOrigin, want) {
		t.Fatalf("plain %+v", reply.AnswerOrigin)
	}
	messages, err := b.RoomMessages(ctx, group.Name, 20)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range messages {
		if p.ID == reply.ID {
			found = reflect.DeepEqual(p.AnswerOrigin, want)
		}
	}
	if !found {
		t.Fatal("group read lost origin")
	}
	detail, err := b.DecisionByID(ctx, identity)
	if err != nil || !detail.Closed || !detail.ReadOnly || detail.Decision.Text != m.Text {
		t.Fatalf("mention lookup %+v %v", detail, err)
	}
	plain, err := b.Send(ctx, Sender{}, s.Address(), "Ordinary reply", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	if plain.AnswerOrigin != nil {
		t.Fatal("ordinary send acquired origin")
	}
	for _, bad := range []string{fmt.Sprintf("request:%d", m.ID), fmt.Sprintf("mention:%d", plain.ID), "request:999999", "invalid"} {
		if _, err := b.DecisionByID(ctx, bad); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
}

func TestDecisionOriginRollsBackWithReceipt(t *testing.T) {
	b, _, d := personRequestFixture(t)
	if _, err := b.db.Exec(`CREATE TRIGGER fail_origin_receipt BEFORE INSERT ON decision_operations BEGIN SELECT RAISE(ABORT,'fixture'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := b.AnswerDecision(context.Background(), AnswerDecisionInput{OperationID: "origin-rollback", ID: d.ID, Revision: 1, Answers: []DecisionAnswer{{Number: 2, AsRecommended: true}}}); err == nil {
		t.Fatal("receipt failure accepted")
	}
	var count int
	if err := b.db.QueryRow(`SELECT COUNT(*) FROM decision_answer_origins`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("origin escaped rollback %d %v", count, err)
	}
}
