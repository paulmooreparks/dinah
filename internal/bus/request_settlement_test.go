package bus

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"

	"studio/internal/requestsettlement"
)

func TestRequestSettlementPreservesPartialAnswersAndReconcilesOriginalReceipt(t *testing.T) {
	for _, command := range []string{"Dismiss", "Withdraw"} {
		t.Run(command, func(t *testing.T) {
			ctx := context.Background()
			b, owner, d := personRequestFixture(t)
			partial, err := b.AnswerDecision(ctx, AnswerDecisionInput{OperationID: "partial", ID: d.ID, Revision: 1, Answers: []DecisionAnswer{{Number: 7, AsRecommended: true}}})
			if err != nil || partial.Revision != 2 || partial.Closed {
				t.Fatal(partial, err)
			}
			before, err := b.DecisionByID(ctx, d.ID)
			if err != nil {
				t.Fatal(err)
			}
			in := requestsettlement.Input{ID: d.ID, Revision: 2, OperationID: "close-once"}
			settle := func(b *Bus, in requestsettlement.Input) (requestsettlement.Receipt, error) {
				if command == "Withdraw" {
					return b.WithdrawRequest(ctx, owner, in)
				}
				return b.DismissRequest(ctx, in)
			}
			first, err := settle(b, in)
			if err != nil || first.State != "committed" {
				t.Fatal(first, err)
			}
			after, err := b.DecisionByID(ctx, d.ID)
			if err != nil || after.Decision.Status != first.Outcome || !reflect.DeepEqual(after.Decision.Questions, before.Decision.Questions) {
				t.Fatal("settlement changed prior answers", before, after, err)
			}
			reopened, err := OpenAt(filepath.Join(b.dir, "bus.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			repeat, err := settle(reopened, in)
			if err != nil || !reflect.DeepEqual(repeat, first) {
				t.Fatal("repeated settlement changed receipt", repeat, err)
			}
			actor := "person"
			if command == "Withdraw" {
				actor = owner.ID
			}
			lookup, err := reopened.RequestOperation(ctx, actor, command, in.OperationID)
			if err != nil || !reflect.DeepEqual(lookup, first) {
				t.Fatal("restart lost receipt", lookup, err)
			}
			in.Revision++
			changed, err := settle(reopened, in)
			if err != nil || changed.Code != "operation_changed" {
				t.Fatal("changed-input reuse accepted", changed, err)
			}
			var obligations, notices int
			if err := reopened.db.QueryRow(`SELECT COUNT(*),COUNT(NULLIF(message_id,0)) FROM request_completion_obligations WHERE request_id=?`, d.ID).Scan(&obligations, &notices); err != nil || obligations != 1 || notices != 1 {
				t.Fatal("completion was lost or repeated", obligations, notices, err)
			}
			var replies int
			if err := reopened.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE reply_to=?`, d.MessageID).Scan(&replies); err != nil || replies != 1 {
				t.Fatal("settlement invented an answer", replies, err)
			}
		})
	}
}

func TestRequestSettlementDefersOfflineCompletionUntilTheOriginalChatReturns(t *testing.T) {
	ctx := context.Background()
	b, owner, d := personRequestFixture(t)
	if err := b.EndSession(ctx, owner.ID); err != nil {
		t.Fatal(err)
	}
	r, err := b.DismissRequest(ctx, requestsettlement.Input{ID: d.ID, Revision: 1, OperationID: "offline-dismiss"})
	if err != nil || r.State != "committed" {
		t.Fatal(r, err)
	}
	var message int64
	var destination string
	if err := b.db.QueryRow(`SELECT recipient,message_id FROM request_completion_obligations WHERE request_id=?`, d.ID).Scan(&destination, &message); err != nil || message != 0 || destination != owner.ID {
		t.Fatal("offline completion dispatched or changed recipient", destination, message, err)
	}
	reopened, err := OpenAt(filepath.Join(b.dir, "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if _, err := reopened.RegisterSession(ctx, owner.ID, owner.WorkspacePath, owner.Title); err != nil {
		t.Fatal(err)
	}
	if err := reopened.DispatchRequestCompletions(ctx); err != nil {
		t.Fatal(err)
	}
	if err := reopened.db.QueryRow(`SELECT message_id FROM request_completion_obligations WHERE request_id=?`, d.ID).Scan(&message); err != nil || message == 0 {
		t.Fatal("reconnect did not dispatch obligation", message, err)
	}
	m, err := reopened.Message(ctx, message)
	if err != nil || m.Kind != ResultKind {
		t.Fatal(m, err)
	}
	var report HostReport
	if err := json.Unmarshal([]byte(m.Text), &report); err != nil {
		t.Fatal(err)
	}
	var output map[string]string
	if err := json.Unmarshal(report.Output, &output); err != nil || len(output) != 2 || output["id"] != d.ID || output["outcome"] != "dismissed" {
		t.Fatal("completion invented an answer or lost disposition", output, err)
	}
}
