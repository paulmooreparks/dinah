package bus

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"

	"studio/internal/requestsettlement"
)

func TestRequestSettlementRefusesPlainMentionWithoutChangingItsChat(t *testing.T) {
	ctx := context.Background()
	b, owner, _ := personRequestFixture(t)
	message, err := b.Send(ctx, Sender{Session: &owner}, Human, "Invented ordinary mention", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	in := requestsettlement.Input{ID: "mention:" + strconv.FormatInt(message.ID, 10), Revision: 1, OperationID: "removed-mention-source"}
	for _, command := range []string{"Dismiss", "Withdraw"} {
		var receipt requestsettlement.Receipt
		if command == "Dismiss" {
			receipt, err = b.DismissRequest(ctx, in)
		} else {
			receipt, err = b.WithdrawRequest(ctx, owner, in)
		}
		if err != nil || receipt.State != "refused" || receipt.Code != "unsupported_source" {
			t.Fatal("plain mention accepted settlement", receipt, err)
		}
	}
	after, err := b.Message(ctx, message.ID)
	if err != nil || !reflect.DeepEqual(after, message) {
		t.Fatal("settlement changed the source message", after, err)
	}
	var count int
	if err := b.db.QueryRow(`SELECT COUNT(*) FROM request_completion_obligations WHERE request_id=?`, in.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("plain mention created a settlement completion", count, err)
	}
}

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

func TestAvailableGroupRequestCanBeDismissedWithoutAnsweringItsMembers(t *testing.T) {
	ctx := context.Background()
	b, owner, _ := personRequestFixture(t)
	other, err := b.RegisterSession(ctx, "window-group-peer", owner.WorkspacePath, "Peer")
	if err != nil {
		t.Fatal(err)
	}
	group, err := b.CreateGroup(ctx, Sender{}, "Dismiss group fixture", []string{owner.ID, other.ID})
	if err != nil {
		t.Fatal(err)
	}
	d, err := b.AskPerson(ctx, owner, AskPersonInput{OperationID: "group-asking", Group: "#" + group.Name, Questions: []PersonQuestion{{Number: 1, Text: "Continue?", Recommendation: "Wait."}}})
	if err != nil {
		t.Fatal(err)
	}
	r, err := b.DismissRequest(ctx, requestsettlement.Input{ID: d.ID, Revision: 1, OperationID: "dismiss-group-request"})
	if err != nil || r.State != "committed" || r.Outcome != "dismissed" {
		t.Fatal(r, err)
	}
	var replies, members int
	if err := b.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE reply_to=?`, d.MessageID).Scan(&replies); err != nil || replies != 0 {
		t.Fatal("group dismissal answered the request", replies, err)
	}
	if err := b.db.QueryRow(`SELECT COUNT(*) FROM room_members WHERE room=? AND member IN (?,?)`, group.Name, owner.ID, other.ID).Scan(&members); err != nil || members != 2 {
		t.Fatal("group dismissal changed membership", members, err)
	}
}

func TestLegacyDismissalReceiptsSurviveTheNewAdapter(t *testing.T) {
	ctx := context.Background()
	b, _, d := personRequestFixture(t)
	in := DismissDecisionInput{OperationID: "legacy-refusal", ID: d.ID, Revision: 1}
	old := DecisionReceipt{OperationID: in.OperationID, ID: in.ID, Revision: 1, State: "rejected", MessageIDs: []int64{}, Reason: &DecisionRejection{Code: "dismissal_unavailable", Message: "This request can receive an answer."}}
	if err := b.tx(ctx, func(tx *sql.Tx) error { return storeDecisionReceipt(tx, decisionFingerprint("dismiss", in), old) }); err != nil {
		t.Fatal(err)
	}
	r, err := b.DismissDecision(ctx, in)
	if err != nil || !reflect.DeepEqual(r, old) {
		t.Fatal("legacy operation was reinterpreted", r, err)
	}
	in.OperationID = "newly-authorized-dismissal"
	r, err = b.DismissDecision(ctx, in)
	if err != nil || r.State != "committed" {
		t.Fatal("new operation did not use approved blanket dismissal", r, err)
	}
}
