package taskhost

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"studio/internal/requestsettlement"
	"testing"
)

func TestQuestionWriteBoundariesExposeSourceAndObligationTogether(t *testing.T) {
	w, _, now, _ := fixture(t)
	q, asked, ended, scope := questionFixture(t)
	result := make(chan error, 1)
	w.options.Backend = backendFunc(func(ctx context.Context, s Scope, _ Operation, _ Capture) Outcome {
		_, err := q.Ask(ctx, s, "Continue?", []QuestionOption{{OptionID: "yes", Name: "Yes"}})
		result <- err
		return Outcome{State: "failed", Code: "worker-returned"}
	})
	p := plan(*now)
	p.Scope = scope
	p.Permits = []Permit{{Kind: "AskPerson", Target: "question"}}
	h := begin(t, w, p)
	op := Operation{ID: "asking", Kind: "AskPerson", Target: "question"}
	if _, err := w.Start(context.Background(), h, op); err != nil {
		t.Fatal(err)
	}
	view := <-asked
	observed := []QuestionRecord{}
	observe := func(r Receipt) error {
		if r.Disposition != "dismissed" {
			return nil
		}
		// This reads the actual records.Store, independently of the Window's
		// in-memory record and its receipt returned to the caller.
		source, err := ReadQuestionRecord(w.options.State, view.Key)
		if err == nil {
			observed = append(observed, source)
		}
		return err
	}
	w.options.Save = observe
	w.options.AfterSave = func(r Receipt) error {
		if err := observe(r); err != nil {
			return err
		}
		if r.Disposition == "dismissed" {
			return errors.New("injected loss of the committed response")
		}
		return nil
	}
	in := requestsettlement.Input{ID: view.Key, Revision: 1, OperationID: "lost-response"}
	_, err := w.DismissQuestion(in)
	var unknown *Error
	if !errors.As(err, &unknown) || unknown.Code != "outcome_unknown" {
		t.Fatal("the injected response loss was reported as a known result", err)
	}
	if len(observed) != 2 || observed[0].Status != "open" || observed[0].Completion != nil || observed[1].Status != "dismissed" || observed[1].Completion == nil || observed[1].Completion.Outcome != "dismissed" {
		t.Fatal("independent checkpoints separated the closed source from its obligation", observed)
	}
	if err := <-result; err == nil {
		t.Fatal("the reconciled waiter received an answer")
	}
	if e := <-ended; e.Disposition != "dismissed" {
		t.Fatal("the committed disposition was lost after response loss", e)
	}
	repeat, err := w.DismissQuestion(in)
	if err != nil || repeat.State != "committed" || repeat.Outcome != "dismissed" || len(observed) != 2 {
		t.Fatal("receipt reconciliation repeated settlement", repeat, err, observed)
	}
}

func TestReviewedQuestionDismissAndWithdrawalPersistBeforeReleasingTheWaiter(t *testing.T) {
	for _, outcome := range []string{"dismissed", "withdrawn"} {
		t.Run(outcome, func(t *testing.T) {
			w, _, now, _ := fixture(t)
			q, asked, ended, scope := questionFixture(t)
			result := make(chan error, 1)
			w.options.Backend = backendFunc(func(ctx context.Context, s Scope, o Operation, _ Capture) Outcome {
				_, err := q.Ask(ctx, s, "Continue?", []QuestionOption{{OptionID: "yes", Name: "Yes"}})
				result <- err
				return Outcome{State: "failed", Code: "worker-returned"}
			})
			p := plan(*now)
			p.Scope = scope
			p.Permits = []Permit{{Kind: "AskPerson", Target: "question"}}
			h := begin(t, w, p)
			op := Operation{ID: "asking", Kind: "AskPerson", Target: "question"}
			if _, err := w.Start(context.Background(), h, op); err != nil {
				t.Fatal(err)
			}
			view := <-asked
			if _, err := w.QuestionRecord(view.Key); err != nil {
				t.Fatal("the offered asking is not durably readable", err)
			}
			in := requestsettlement.Input{ID: view.Key, Revision: 1, OperationID: "close-once"}
			var receipt requestsettlement.Receipt
			var err error
			if outcome == "dismissed" {
				receipt, err = w.DismissQuestion(in)
			} else {
				foreign := scope
				foreign.OwnerChat = "another-chat"
				refused, e := w.ForService(scope.Service).WithdrawQuestion(context.Background(), foreign, view.Key, "wrong-owner")
				if e != nil || refused.State != "refused" || len(q.Pending()) != 1 {
					t.Fatal("foreign withdrawal changed pending question", refused, e)
				}
				receipt, err = w.ForService(scope.Service).WithdrawQuestion(context.Background(), scope, view.Key, in.OperationID)
			}
			if err != nil || receipt.State != "committed" || receipt.Outcome != outcome {
				t.Fatal(receipt, err)
			}
			record, err := w.QuestionRecord(view.Key)
			if err != nil || record.Status != outcome || record.Completion == nil || record.Completion.Outcome != outcome {
				t.Fatal("settlement was not durable", record, err)
			}
			operation, err := w.Read(context.Background(), scope, op.ID)
			if err != nil || operation.Disposition != outcome || len(operation.Result) != 0 {
				t.Fatal("operation was not durably settled without an answer", operation, err)
			}
			if err := <-result; err == nil {
				t.Fatal("waiter received an answer")
			}
			if event := <-ended; event.Disposition != outcome {
				t.Fatal("question end lost disposition", event)
			}
			if len(q.Pending()) != 0 {
				t.Fatal("settled question remained pending")
			}
			if err = w.Shutdown(context.Background()); err != nil {
				t.Fatal(err)
			}
			reopened, err := New(w.options)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Shutdown(context.Background())
			recovery, err := reopened.Rebind(context.Background(), scope)
			if err != nil || len(recovery.Receipts) != 1 || !reflect.DeepEqual(recovery.Receipts[0], operation) {
				t.Fatal("restart changed disposition", recovery, err)
			}
			var repeat requestsettlement.Receipt
			if outcome == "dismissed" {
				repeat, err = reopened.DismissQuestion(in)
			} else {
				repeat, err = reopened.ForService(scope.Service).WithdrawQuestion(context.Background(), scope, view.Key, in.OperationID)
			}
			if err != nil || !reflect.DeepEqual(repeat, receipt) {
				t.Fatal("original receipt was not retained", repeat, err)
			}
		})
	}
}

func TestReviewedQuestionAnswerRetainsItsLegacyResult(t *testing.T) {
	w, _, now, _ := fixture(t)
	q, asked, _, scope := questionFixture(t)
	w.options.Backend = backendFunc(func(ctx context.Context, s Scope, o Operation, _ Capture) Outcome {
		_, err := q.Ask(ctx, s, "Continue?", []QuestionOption{{OptionID: "yes", Name: "Yes"}})
		if err != nil {
			return Outcome{State: "failed"}
		}
		return Outcome{State: "succeeded"}
	})
	p := plan(*now)
	p.Scope = scope
	p.Permits = []Permit{{Kind: "AskPerson", Target: "question"}}
	h := begin(t, w, p)
	if _, err := w.Start(context.Background(), h, Operation{ID: "asking", Kind: "AskPerson", Target: "question"}); err != nil {
		t.Fatal(err)
	}
	view := <-asked
	if err := q.Answer(view.Key, "yes"); err != nil {
		t.Fatal(err)
	}
	r := await(t, w, scope, "asking")
	var answer map[string]string
	if err := json.Unmarshal(r.Result, &answer); err != nil || answer["answer"] != "yes" || r.State != "succeeded" {
		t.Fatal("legacy answer changed", r, err)
	}
}

func TestReviewedQuestionRereadRepairsACommittedDispositionWithoutResettlement(t *testing.T) {
	w, _, now, _ := fixture(t)
	q, asked, ended, scope := questionFixture(t)
	result := make(chan error, 1)
	w.options.Backend = backendFunc(func(ctx context.Context, s Scope, o Operation, _ Capture) Outcome {
		_, err := q.Ask(ctx, s, "Continue?", []QuestionOption{{OptionID: "yes", Name: "Yes"}})
		result <- err
		return Outcome{State: "failed", Code: "worker-returned"}
	})
	p := plan(*now)
	p.Scope = scope
	p.Permits = []Permit{{Kind: "AskPerson", Target: "question"}}
	h := begin(t, w, p)
	if _, err := w.Start(context.Background(), h, Operation{ID: "asking", Kind: "AskPerson", Target: "question"}); err != nil {
		t.Fatal(err)
	}
	view := <-asked
	// Commit at the real durable boundary, with the response and event path
	// intentionally absent. Pending must observe this result without writing.
	in := requestsettlement.Input{ID: view.Key, Revision: 1, OperationID: "lost-response"}
	committed, err := w.commitQuestion(scope, view.Key, "dismissed", "person", in, nil)
	if err != nil || committed.State != "committed" {
		t.Fatal(committed, err)
	}
	before, err := w.Read(context.Background(), scope, "asking")
	if err != nil {
		t.Fatal(err)
	}
	if len(q.Pending()) != 0 {
		t.Fatal("committed question remained pending")
	}
	if err := <-result; err == nil {
		t.Fatal("reconciliation invented an answer")
	}
	if event := <-ended; event.Disposition != "dismissed" {
		t.Fatal(event)
	}
	after, err := w.Read(context.Background(), scope, "asking")
	if err != nil || !reflect.DeepEqual(after, before) {
		t.Fatal("reread resettled the operation", before, after, err)
	}
	if err := w.RecordQuestionNotice(view.Key, 77); err != nil {
		t.Fatal(err)
	}
	acknowledged, err := w.Read(context.Background(), scope, "asking")
	if err != nil || !reflect.DeepEqual(acknowledged, before) {
		t.Fatal("notice acknowledgement changed the operation receipt", before, acknowledged, err)
	}
}
