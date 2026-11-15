package taskhost

import (
	"context"
	"testing"
)

func TestLocalInspectionIsScopedAndCannotExecute(t *testing.T) {
	w, _, now, calls := fixture(t)
	s := plan(*now).Scope
	w.options.Operations = func(context.Context, Scope) ([]string, error) { return []string{"OpenChat"}, nil }
	w.options.DescribeChats = func(ctx context.Context, scope Scope) ([]ChatDescription, error) {
		if scope.Service != "test" || scope.Workspace != s.Workspace {
			t.Error("foreign inspection scope")
		}
		return []ChatDescription{{ChatID: "logical-chat", Head: "native-chat", Workspace: scope.Workspace}}, nil
	}
	w.options.CheckProgram = func(context.Context, Scope, string) (ProgramDescription, error) {
		return ProgramDescription{Program: "example", Offered: true}, nil
	}
	w.options.DescribeAction = func(context.Context, Scope, string) (ActionDescription, error) {
		return ActionDescription{Reference: "example/read@1", Digest: "current", Capabilities: []string{"workspace.read"}}, nil
	}
	c := w.ForService("test")
	view, err := c.Inspect(context.Background(), s)
	if err != nil || view.Rooms || len(view.Operations) != 1 {
		t.Fatalf("inspect: %+v %v", view, err)
	}
	if chats, err := c.DescribeChats(context.Background(), s); err != nil || len(chats) != 1 || chats[0].ChatID != "logical-chat" {
		t.Fatalf("chats: %+v %v", chats, err)
	}
	if p, err := c.CheckProgram(context.Background(), s, "example"); err != nil || !p.Offered {
		t.Fatalf("program: %+v %v", p, err)
	}
	if action, err := c.DescribeAction(context.Background(), s, "example/read@1"); err != nil || action.Digest != "current" {
		t.Fatalf("action: %+v %v", action, err)
	}
	if *calls != 0 || w.Snapshot().Handles != 0 || w.Snapshot().Workers != 0 {
		t.Fatal("inspection admitted execution")
	}
	foreign := s
	foreign.Service = "another-service"
	if _, err := c.DescribeChats(context.Background(), foreign); err == nil {
		t.Fatal("service selected another service's task")
	}
	w.CloseAdmission()
	if _, err := c.Inspect(context.Background(), s); err == nil {
		t.Fatal("inspection survived its window")
	}
}
