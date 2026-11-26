package main

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"studio/internal/services"
	"studio/internal/taskhost"
)

func TestTaskQuestionPageTargetsStayWithinDeclaredPages(t *testing.T) {
	registration := services.WindowPresentation{Name: "Example Tasks", Pages: []string{"example-tasks"}}
	for _, page := range []string{"https://remote.example/", "chat", "other-service", "../example-tasks"} {
		if target := taskQuestionTarget(services.TaskPresentation{Page: page}, registration); target.Href != "" || target.Reason == "" {
			t.Fatalf("accepted foreign target %q: %+v", page, target)
		}
	}
	state := url.Values{"task": {"opaque/task?one"}, "workspace": {"workspace-a"}}.Encode()
	target := taskQuestionTarget(services.TaskPresentation{Page: "example-tasks", State: state}, registration)
	if target.Reason != "" || !strings.HasPrefix(target.Href, "?") {
		t.Fatal(target)
	}
	decoded, err := url.ParseQuery(strings.TrimPrefix(target.Href, "?"))
	if err != nil || decoded.Get("open") != "example-tasks" || decoded.Get("view.example-tasks") != state || len(decoded) != 2 {
		t.Fatalf("invalid page-state target: %+v %v", decoded, err)
	}
	if target := taskQuestionTarget(services.TaskPresentation{Unavailable: "No page is installed."}, registration); target.Href != "" || target.Reason != "No page is installed." {
		t.Fatal(target)
	}
}

func TestAppTaskQuestionUsesPersonAnswerWithoutChatState(t *testing.T) {
	a, conn := providerTestApp(t)
	scope := taskhost.Scope{Service: "example-service", Workspace: "workspace-a", Task: "task-a", Generation: "1"}
	a.taskQuestions.check = func(context.Context, taskhost.Scope) error { return nil }
	a.taskQuestions.describe = func(ctx context.Context, s taskhost.Scope) (taskhost.QuestionOwner, error) {
		return taskhost.QuestionOwner{Service: s.Service, ServiceName: "Example Service", Workspace: s.Workspace, WorkspaceName: "Example Workspace", Task: s.Task, Generation: s.Generation, TaskTitle: "Review an invented change", Target: taskhost.QuestionTarget{Reason: "No page in this fixture."}}, nil
	}
	asked, ended := make(chan taskhost.Question, 1), make(chan taskhost.QuestionEnd, 1)
	a.sink = func(name string, value any) {
		switch name {
		case "task:permission":
			asked <- value.(taskhost.Question)
		case "task:answered":
			ended <- value.(taskhost.QuestionEnd)
		case "agent:permission", "agent:answered", "chat:open":
			t.Errorf("task question touched a chat event: %s", name)
		}
	}
	before := a.runtime.ConversationCountLocked()
	if len(a.PendingTaskPermissions()) != 0 || a.taskQuestions.questions != nil {
		t.Fatal("pending query constructed the task question service")
	}
	result := make(chan string, 1)
	go func() {
		answer, err := a.askTaskQuestion(context.Background(), scope, "Continue?", []taskhost.QuestionOption{{OptionID: "yes", Name: "Continue"}})
		if err != nil {
			result <- err.Error()
		} else {
			result <- answer
		}
	}()
	question := <-asked
	bytes, _ := json.Marshal(a.PendingTaskPermissions())
	if strings.Contains(string(bytes), "sessionId") || len(a.PendingTaskPermissions()) != 1 || len(a.PendingPermissions()) != 0 {
		t.Fatalf("task/chat snapshots mixed: %s", bytes)
	}
	if err := a.Answer(question.Key, "unoffered"); err == nil {
		t.Fatal("person path accepted an unoffered task choice")
	}
	if err := a.Answer(question.Key, "yes"); err != nil {
		t.Fatal(err)
	}
	select {
	case answer := <-result:
		if answer != "yes" {
			t.Fatal(answer)
		}
	case <-time.After(testWait):
		t.Fatal("person answer did not settle task question")
	}
	if end := <-ended; end.Key != question.Key {
		t.Fatal(end)
	}
	if len(a.PendingTaskPermissions()) != 0 || a.runtime.ConversationCountLocked() != before || len(conn.calls.Snapshot()) != 0 {
		t.Fatal("task question changed chats or contacted a provider")
	}
	if err := a.Answer(question.Key, "yes"); err == nil {
		t.Fatal("old task key remained answerable")
	}
}
