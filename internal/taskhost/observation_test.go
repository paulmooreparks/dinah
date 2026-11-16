package taskhost

import (
	"context"
	"testing"
)

func TestConsumerObservationPortsPreserveServiceScope(t *testing.T) {
	w, _, now, calls := fixture(t)
	s := plan(*now).Scope
	var queries int
	w.options.PendingQuestions = func(_ context.Context, got Scope) ([]Question, error) {
		queries++
		if got != s {
			t.Fatalf("foreign scope reached question port: %+v", got)
		}
		return []Question{{QuestionOwner: QuestionOwner{Service: s.Service, Workspace: s.Workspace, Task: s.Task, Generation: s.Generation}, Key: "host-key", Options: []QuestionOption{{OptionID: "yes"}}}}, nil
	}
	var changes []Change
	w.options.SavedChange = func(change Change) { changes = append(changes, change) }
	c := w.ForService("test")
	questions, err := c.PendingQuestions(context.Background(), s)
	if err != nil || len(questions) != 1 || questions[0].Key != "host-key" {
		t.Fatal(questions, err)
	}
	questions[0].Options[0].OptionID = "forged"
	again, err := c.PendingQuestions(context.Background(), s)
	if err != nil || again[0].Options[0].OptionID != "yes" {
		t.Fatal(again, err)
	}
	if err := c.PublishChange(context.Background(), s, 7); err != nil {
		t.Fatal(err)
	}
	if len(changes) != 1 || changes[0] != (Change{Service: s.Service, Workspace: s.Workspace, Task: s.Task, Generation: s.Generation, Sequence: 7}) {
		t.Fatal(changes)
	}
	foreign := s
	foreign.Service = "other"
	for _, client := range []Client{c, w.ForService("other")} {
		scope := foreign
		if client.service == "other" {
			scope = s
		}
		if _, err := client.PendingQuestions(context.Background(), scope); err == nil {
			t.Fatal("cross-service question read accepted")
		}
		if err := client.PublishChange(context.Background(), scope, 8); err == nil {
			t.Fatal("cross-service event accepted")
		}
	}
	if queries != 2 || len(changes) != 1 || *calls != 0 || w.Snapshot().Handles != 0 {
		t.Fatal("refusal crossed the observation boundary", queries, changes, *calls)
	}
	if err := c.PublishChange(context.Background(), s, 0); err == nil {
		t.Fatal("zero sequence accepted")
	}
	w.CloseAdmission()
	if _, err := c.PendingQuestions(context.Background(), s); err == nil {
		t.Fatal("read survived closed window")
	}
	if err := c.PublishChange(context.Background(), s, 9); err == nil {
		t.Fatal("event survived closed window")
	}
}

func TestConsumerQuestionReadRefusesForeignHostResults(t *testing.T) {
	w, _, now, _ := fixture(t)
	s := plan(*now).Scope
	for _, field := range []string{"service", "workspace", "task", "generation"} {
		t.Run(field, func(t *testing.T) {
			owner := QuestionOwner{Service: s.Service, Workspace: s.Workspace, Task: s.Task, Generation: s.Generation}
			switch field {
			case "service":
				owner.Service = "other"
			case "workspace":
				owner.Workspace = "other"
			case "task":
				owner.Task = "other"
			case "generation":
				owner.Generation = "other"
			}
			w.options.PendingQuestions = func(context.Context, Scope) ([]Question, error) {
				return []Question{{QuestionOwner: owner, Key: "foreign-key"}}, nil
			}
			if result, err := w.ForService("test").PendingQuestions(context.Background(), s); err == nil || result != nil {
				t.Fatal("foreign host result escaped", result, err)
			}
		})
	}
}
