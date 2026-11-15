package main

import (
	"context"
	"testing"

	"studio/internal/taskhost"
)

func TestTaskInspectionReadsOnlyLocalWorkspaceChatsAndConfiguration(t *testing.T) {
	a, connection := providerTestApp(t)
	root, other := t.TempDir(), t.TempDir()
	a.settings.WorkspaceRecords = []WorkspaceRecord{{ID: "workspace-a", Name: "Example", ExecutionPath: root}, {ID: "workspace-b", Name: "Other", ExecutionPath: other}}
	cleanupFixtureChat(t, a, "owned-chat", "codex", root, "")
	cleanupFixtureChat(t, a, "other-chat", "codex", other, "")
	scope := taskhost.Scope{Service: "example", Workspace: "workspace-a", Task: "task", Generation: "1"}
	chats, err := a.describeTaskChats(context.Background(), scope)
	if err != nil || len(chats) != 1 || chats[0].ChatID != "owned-chat" || chats[0].Workspace != scope.Workspace {
		t.Fatalf("local inventory: %+v %v", chats, err)
	}
	if offered, err := a.checkTaskProgram(context.Background(), scope, "codex"); err != nil || !offered.Offered {
		t.Fatalf("offered: %+v %v", offered, err)
	}
	if missing, err := a.checkTaskProgram(context.Background(), scope, "unknown-program"); err != nil || missing.Offered || missing.Reason == "" {
		t.Fatalf("missing: %+v %v", missing, err)
	}
	if action, err := a.describeTaskAction(context.Background(), scope, "dinah.notifications/post@1"); err != nil || action.Digest == "" || len(action.InputSchema) == 0 {
		t.Fatalf("action: %+v %v", action, err)
	}
	if len(connection.calls.Snapshot()) != 0 || a.runtime.ConversationCountLocked() != 0 {
		t.Fatal("local inspection loaded or queried a native session")
	}
}
