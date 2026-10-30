package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"studio/internal/acp"
	"studio/internal/applet"
	"studio/internal/bus"
)

func asChat(a *App, chat *Session) context.Context {
	return applet.WithInvocation(context.Background(), applet.Invocation{Workspace: chat.Cwd(), OwnerKind: "chat", OwnerID: chat.ID(), ID: "team-" + chat.ID()})
}

func call(t *testing.T, h applet.Handler, ctx context.Context, input map[string]any) (map[string]any, error) {
	t.Helper()
	data, _ := json.Marshal(input)
	res, err := h(ctx, string(data))
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(res.OutputJSON), &out); err != nil {
		t.Fatal(err)
	}
	return out, nil
}

// A lead starts a member, which is its own chat in its workspace with its model, led by it, and given its brief as a
// message in the team's group; the lead's settings are not copied beyond the model.
func TestALeadStartsAssignsAndRetiresItsOwnMembers(t *testing.T) {
	a, c := providerTestApp(t)
	c.newIDs = []string{"codex:lead", "codex:new"}
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	lead := a.session(info.ID)
	ctx := asChat(a, lead)
	a.settings.WorkspaceRecords = []WorkspaceRecord{{ID: "w", Name: "W", ExecutionPath: lead.Cwd()}}
	if _, err := a.SetSessionOption(lead.ID(), "model", json.RawMessage(`"changed"`)); err != nil {
		t.Fatal(err)
	}
	before := len(c.calls)
	out, err := call(t, a.teamStartHandler, ctx, map[string]any{"title": "Fix the Parser", "brief": "Fix the parser.\nSee the notes."})
	if err != nil {
		t.Fatal(err)
	}
	member := a.session(out["sessionId"].(string))
	if member == nil || member.Cwd() != lead.Cwd() || out["group"] == "" {
		t.Fatalf("the member %v, result %v", member, out)
	}
	for _, o := range member.Options() {
		if o.Category == "model" && judgeOptionString(o.CurrentValue) != "changed" {
			t.Fatalf("the member's model is %s, not its lead's", o.CurrentValue)
		}
	}
	for _, call := range c.calls[before:] {
		if p, ok := call.params.(acp.SetConfigOptionParams); ok && p.ConfigID != "model" {
			t.Fatalf("a setting other than the model was given to the new member: %+v", p)
		}
	}
	err = a.busHost().With(func(b *bus.Bus) error {
		leadBus, memberBus := a.busSessionID(lead.ID()), a.busSessionID(member.ID())
		l, _, _ := b.LeadOf(context.Background(), memberBus)
		if l != leadBus {
			t.Fatalf("the member's lead is %q", l)
		}
		asg, ok, _ := b.LastAssignment(context.Background(), leadBus, memberBus)
		if !ok || !strings.HasPrefix(asg.Text, "Fix the parser.") || asg.Room != out["group"] {
			t.Fatalf("the brief: %+v", asg)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Assign: only to the lead's own members.
	if _, err := call(t, a.teamAssignHandler, ctx, map[string]any{"member": member.ID(), "text": "Also check the lexer."}); err != nil {
		t.Fatal(err)
	}
	if _, err := call(t, a.teamAssignHandler, ctx, map[string]any{"member": lead.ID(), "text": "Myself."}); err == nil || !strings.Contains(err.Error(), "not one of your team members") {
		t.Fatalf("an assignment to a chat that is not a member: %v", err)
	}
	if _, err := call(t, a.teamAssignHandler, ctx, map[string]any{"member": "codex:nobody", "text": "x"}); err == nil || !strings.Contains(err.Error(), "no chat") {
		t.Fatalf("an assignment to an unknown chat: %v", err)
	}
	// Another chat's member is not the lead's to act on.
	other := asChat(a, member)
	if _, err := call(t, a.teamRetireHandler, other, map[string]any{"member": lead.ID()}); err == nil {
		t.Fatal("a member retired its lead")
	}
	list, err := call(t, a.teamListHandler, ctx, map[string]any{})
	if err != nil || len(list["members"].([]any)) != 1 {
		t.Fatalf("the list %v %v", list, err)
	}
	// Retire: the session ends, the history stays, the row is marked and nothing wakes it.
	if _, err := call(t, a.teamRetireHandler, ctx, map[string]any{"member": member.ID()}); err != nil {
		t.Fatal(err)
	}
	if a.session(member.ID()) != nil {
		t.Fatal("a retired member's session is still open")
	}
	if _, err := call(t, a.teamAssignHandler, ctx, map[string]any{"member": member.ID(), "text": "More."}); err == nil || !strings.Contains(err.Error(), "retired") {
		t.Fatalf("an assignment to a retired member: %v", err)
	}
	list, _ = call(t, a.teamListHandler, ctx, map[string]any{})
	if m := list["members"].([]any)[0].(map[string]any); m["retired"] != true {
		t.Fatalf("the retired member is listed as %v", m)
	}
	// Only a chat's agent acts on a team.
	if _, err := call(t, a.teamStartHandler, context.Background(), map[string]any{"title": "x", "brief": "y"}); err == nil {
		t.Fatal("a call with no chat started a member")
	}
}
