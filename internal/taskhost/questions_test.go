package taskhost

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func questionFixture(t *testing.T) (*Questions, chan Question, chan QuestionEnd, Scope) {
	t.Helper()
	asked, ended := make(chan Question, 4), make(chan QuestionEnd, 4)
	scope := plan(time.Now()).Scope
	q, err := NewQuestions(QuestionPorts{
		Describe: func(ctx context.Context, s Scope) (QuestionOwner, error) {
			return QuestionOwner{Service: s.Service, ServiceName: "Example Service", Workspace: s.Workspace, WorkspaceName: "Example Workspace", Task: s.Task, Generation: s.Generation, TaskTitle: "Review a change", Target: QuestionTarget{Reason: "The task page is unavailable in this fixture."}}, nil
		}, Check: func(context.Context, Scope) error { return nil },
		Emit: func(name string, value any) {
			switch name {
			case "task:permission":
				asked <- value.(Question)
			case "task:answered":
				ended <- value.(QuestionEnd)
			default:
				t.Errorf("task question entered a chat event: %s", name)
			}
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(q.Close)
	return q, asked, ended, scope
}

func TestTaskQuestionHasExplicitOwnerAndRejectsInvalidOrRepeatedAnswers(t *testing.T) {
	q, asked, ended, scope := questionFixture(t)
	result := make(chan string, 1)
	go func() {
		answer, err := q.Ask(context.Background(), scope, "Continue?", []QuestionOption{{OptionID: "yes", Name: "Continue", Kind: "allow_once"}})
		if err != nil {
			result <- err.Error()
		} else {
			result <- answer
		}
	}()
	question := <-asked
	bytes, _ := json.Marshal(question)
	if strings.Contains(string(bytes), "sessionId") || strings.Contains(string(bytes), "rawInput") || question.TaskTitle == scope.Task {
		t.Fatalf("question borrowed chat or caller metadata: %s", bytes)
	}
	if err := q.Answer(question.Key, "unoffered"); err == nil {
		t.Fatal("unoffered answer accepted")
	}
	if len(q.Pending()) != 1 {
		t.Fatal("invalid answer retired the pending question")
	}
	if err := q.Answer(question.Key, "yes"); err != nil {
		t.Fatal(err)
	}
	if answer := <-result; answer != "yes" {
		t.Fatal(answer)
	}
	if end := <-ended; end.Key != question.Key || end.Task != scope.Task {
		t.Fatal(end)
	}
	if err := q.Answer(question.Key, "yes"); err == nil {
		t.Fatal("answer key replayed")
	}
	if len(q.Pending()) != 0 {
		t.Fatal("answered question revived")
	}
}

func TestTaskQuestionCancellationAndGenerationChangeRetireIt(t *testing.T) {
	for _, mode := range []string{"cancel", "generation", "shutdown"} {
		t.Run(mode, func(t *testing.T) {
			q, asked, ended, scope := questionFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				_, err := q.Ask(ctx, scope, "Continue?", []QuestionOption{{OptionID: "yes", Name: "Continue"}})
				done <- err
			}()
			question := <-asked
			switch mode {
			case "cancel":
				cancel()
			case "generation":
				q.ports.Check = func(context.Context, Scope) error { return refusal("stale", "task generation changed") }
			case "shutdown":
				q.Close()
			}
			if err := q.Answer(question.Key, "yes"); err == nil {
				t.Fatal("revoked or stale question accepted an answer")
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("cancelled question returned success")
				}
			case <-time.After(time.Second):
				t.Fatal("pending question did not finish")
			}
			if end := <-ended; end.Key != question.Key {
				t.Fatal(end)
			}
			if len(q.Pending()) != 0 {
				t.Fatal("question remained pending")
			}
		})
	}
}

func TestShutdownCancelsPendingReviewConfirmation(t *testing.T) {
	w, _, now, _ := fixture(t)
	entered := make(chan struct{})
	w.options.Confirm = func(ctx context.Context, r Review) error { close(entered); <-ctx.Done(); return ctx.Err() }
	r, err := w.Review(context.Background(), plan(*now))
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := w.Begin(context.Background(), r.ID); done <- err }()
	<-entered
	w.CloseAdmission()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("shutdown admitted a pending review")
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown left review question active")
	}
}
