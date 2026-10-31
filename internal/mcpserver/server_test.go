package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"studio/internal/bus"
)

// In Dinah's window the person's hooks reach the stand-in server: the
// prompt's marker ties the agent's session to its chat, PostToolUse hands
// over a waiting steer as context, and Stop continues the turn with one.
func TestTheWindowsHooksHandOverASteer(t *testing.T) {
	ctx := context.Background()
	b, err := bus.OpenAt(filepath.Join(t.TempDir(), "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	chat, _ := b.RegisterSession(ctx, "window-1", t.TempDir(), "")
	s := NewIdle(b)
	if out := s.hostedHook(ctx, hookIn{SessionID: "claude-1", Event: "PostToolUse"}); out != "{}" {
		t.Fatalf("an unbound session got %s", out)
	}
	s.hostedHook(ctx, hookIn{SessionID: "claude-1", Event: "UserPromptSubmit", Prompt: "Build it.\n" + bus.SteerMarker(chat.ID)})
	b.QueueSteer(ctx, chat.ID, "Use port 8081.")
	var post map[string]map[string]string
	out := s.hostedHook(ctx, hookIn{SessionID: "claude-1", Event: "PostToolUse"})
	if json.Unmarshal([]byte(out), &post) != nil || !strings.Contains(post["hookSpecificOutput"]["additionalContext"], "Use port 8081.") {
		t.Fatalf("PostToolUse gave %s", out)
	}
	if out := s.hostedHook(ctx, hookIn{SessionID: "claude-1", Event: "PostToolUse"}); out != "{}" {
		t.Fatalf("a steer was handed over twice: %s", out)
	}
	b.QueueSteer(ctx, chat.ID, "Then stop.")
	var stop map[string]string
	out = s.hostedHook(ctx, hookIn{SessionID: "claude-1", Event: "Stop"})
	if json.Unmarshal([]byte(out), &stop) != nil || stop["decision"] != "block" || !strings.Contains(stop["reason"], "Then stop.") {
		t.Fatalf("Stop gave %s", out)
	}
}

func TestHookOutputShapes(t *testing.T) {
	msgs := []bus.Message{{ID: 1, FromName: "alpha-1", ToLabel: "beta", Text: "hi", Kind: "text"}}
	out, _ := HookOutput("UserPromptSubmit", msgs, false, bus.PersonFallback)
	var v map[string]map[string]string
	if err := json.Unmarshal([]byte(out), &v); err != nil || v["hookSpecificOutput"]["hookEventName"] != "UserPromptSubmit" ||
		!strings.Contains(v["hookSpecificOutput"]["additionalContext"], "hi") {
		t.Fatalf("prompt output: %s", out)
	}
	out, _ = HookOutput("Stop", msgs, true, bus.PersonFallback)
	var stop map[string]string
	if err := json.Unmarshal([]byte(out), &stop); err != nil || stop["decision"] != "block" {
		t.Fatalf("stop output: %s", out)
	}
	if out, _ = HookOutput("Stop", nil, false, bus.PersonFallback); out != "{}" {
		t.Fatalf("nothing waiting: %s", out)
	}
}

// What whoami and sessions tell an agent gives each chat by its title and
// workspace, as the agent sends to it, and never the handle the bus gave it.
func TestTheDirectoryGivesChatsByTitle(t *testing.T) {
	ctx := context.Background()
	b, err := bus.OpenAt(filepath.Join(t.TempDir(), "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	dir := filepath.Join(t.TempDir(), "parser")
	if _, err := b.LabelWorkspace(ctx, dir, "Parser Library"); err != nil {
		t.Fatal(err)
	}
	me, _ := b.RegisterSession(ctx, "s-me", dir, "Parser Release")
	other, _ := b.RegisterSession(ctx, "s-other", dir, "")
	who, err := Directory(ctx, b, me.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"- Parser Release (parser), which is you", "- Untitled Chat in Parser Library (parser)", `- parser (` + dir + `), shown to`, `as "Parser Library": 2 chats open`} {
		if !strings.Contains(who, want) {
			t.Fatalf("the directory lacks %q:\n%s", want, who)
		}
	}
	if strings.Contains(who, "parser-1") || strings.Contains(who, "parser-2") || strings.Contains(who, other.ID) {
		t.Fatalf("the directory shows a handle:\n%s", who)
	}
	if got := Whoami(me); got != `You are the chat "Parser Release", in workspace parser (`+dir+`). Others send to you as Parser Release (parser).` {
		t.Fatalf("whoami: %s", got)
	}
}

// connect serves the window's bus tools for a chat and returns a client
// session on them.
func connect(t *testing.T, b *bus.Bus, id string) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	srv := mcp.NewServer(&mcp.Implementation{Name: "dinah", Version: Version}, nil)
	NewWindow(b, id).register(srv, "Paul")
	st, ct := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func call(t *testing.T, cs *mcp.ClientSession, tool string, args map[string]any) (string, bool) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			out.WriteString(tc.Text)
		}
	}
	return out.String(), res.IsError
}

// The tools' schemas say that a group chat's members are chats: send and
// reply take the chats a message mentions as a list of titles, and
// room_create takes its members by title.
func TestTheToolSchemasDescribeGroupsOfChats(t *testing.T) {
	ctx := context.Background()
	b, err := bus.OpenAt(filepath.Join(t.TempDir(), "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	me, _ := b.RegisterSession(ctx, "window-me", t.TempDir(), "Lead")
	tools, err := connect(t, b, me.ID).ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	props := map[string]map[string]any{}
	descriptions := map[string]string{}
	for _, tool := range tools.Tools {
		data, _ := json.Marshal(tool.InputSchema)
		var schema struct {
			Properties map[string]any `json:"properties"`
		}
		json.Unmarshal(data, &schema)
		props[tool.Name] = schema.Properties
		descriptions[tool.Name] = tool.Description
	}
	for _, name := range []string{"send", "reply"} {
		a, _ := props[name]["actionRecipients"].(map[string]any)
		if a == nil || !strings.Contains(fmt.Sprint(a["type"]), "array") {
			t.Fatalf("missing actionRecipients: %+v", props[name])
		}
		m, _ := props[name]["mentions"].(map[string]any)
		items, _ := m["items"].(map[string]any)
		if m == nil || !strings.Contains(fmt.Sprint(m["type"]), "array") || items["type"] != "string" || !strings.Contains(m["description"].(string), "title") {
			t.Fatalf("%s's mentions: %+v", name, props[name]["mentions"])
		}
	}
	members, _ := props["room_create"]["members"].(map[string]any)
	if members == nil || !strings.Contains(members["description"].(string), "chats to put in the group") || strings.Contains(members["description"].(string), "workspace names") {
		t.Fatalf("room_create's members: %+v", members)
	}
	for _, name := range []string{"room_create", "room_join", "room_leave"} {
		if !strings.Contains(descriptions[name], "group chat") {
			t.Fatalf("%s says %q", name, descriptions[name])
		}
	}
	if _, ok := props["rename"]; ok {
		t.Fatal("a chat in the window was offered rename")
	}
}

// An agent starts a group of chats by title, sends to it with mentions, is
// refused a mention of a chat outside it with the reason, and joins and
// leaves a group as its own chat.
func TestTheRoomToolsActOnChats(t *testing.T) {
	ctx := context.Background()
	b, err := bus.OpenAt(filepath.Join(t.TempDir(), "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	dir := filepath.Join(t.TempDir(), "studio")
	me, _ := b.RegisterSession(ctx, "window-me", dir, "Lead")
	docs, _ := b.RegisterSession(ctx, "window-docs", dir, "Docs Pass")
	b.RegisterSession(ctx, "window-out", dir, "Outsider")
	cs := connect(t, b, me.ID)
	out, failed := call(t, cs, "room_create", map[string]any{"name": "Release Review", "members": []string{"Docs Pass"}})
	if failed || !strings.Contains(out, `Started the group chat "Release Review" with Lead (studio), Docs Pass (studio), Paul`) {
		t.Fatalf("room_create: %s", out)
	}
	if out, failed = call(t, cs, "send", map[string]any{"to": "#Release Review", "text": "Docs?", "mentions": []string{"Docs Pass"}}); failed {
		t.Fatalf("send: %s", out)
	}
	if n, _, _ := b.Doorbell(ctx, docs.ID); n != 1 {
		t.Fatal("the mention did not wake the chat")
	}
	out, failed = call(t, cs, "send", map[string]any{"to": "#Release Review", "text": "You too?", "mentions": []string{"Outsider"}})
	if !failed || !strings.Contains(out, "Outsider (studio) is not in the group chat") {
		t.Fatalf("a mention of a chat outside the group: %v %s", failed, out)
	}
	if out, failed = call(t, cs, "room_leave", map[string]any{"room": "release review"}); failed || !strings.Contains(out, `left the group chat "Release Review"`) {
		t.Fatalf("room_leave: %s", out)
	}
	if out, failed = call(t, cs, "room_join", map[string]any{"room": "#Release Review"}); failed || !strings.Contains(out, `is in the group chat "Release Review"`) {
		t.Fatalf("room_join: %s", out)
	}
	who, _ := call(t, cs, "sessions", nil)
	if !strings.Contains(who, "- #Release Review: Docs Pass (studio), Lead (studio), the person") {
		t.Fatalf("the directory's group chats:\n%s", who)
	}
}

func TestAProcessServingTwoSessionsRefusesToGuess(t *testing.T) {
	ctx := context.Background()
	b, err := bus.OpenAt(filepath.Join(t.TempDir(), "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	s := New(b)
	dir := t.TempDir()
	if _, err := s.hookDeliver(ctx, hookIn{SessionID: "one", Event: "UserPromptSubmit", Cwd: dir}); err != nil {
		t.Fatal(err)
	}
	if me, err := s.current(ctx); err != nil || me.ID != "one" {
		t.Fatalf("bound: %v %+v", err, me)
	}
	s.hookDeliver(ctx, hookIn{SessionID: "two", Event: "UserPromptSubmit", Cwd: dir})
	if _, err := s.current(ctx); err == nil {
		t.Fatal("a process serving two sessions attributed a tool call")
	}
}

// A launched server keeps its session when the agent's own Claude Code
// hooks call it with another ID.
func TestALaunchedServerIgnoresHooks(t *testing.T) {
	ctx := context.Background()
	b, err := bus.OpenAt(filepath.Join(t.TempDir(), "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if _, err := b.RegisterSession(ctx, "hosted", t.TempDir(), ""); err != nil {
		t.Fatal(err)
	}
	s := NewLaunched(b, "hosted")
	if out, err := s.hookDeliver(ctx, hookIn{SessionID: "claude-internal", Event: "UserPromptSubmit", Cwd: t.TempDir()}); err != nil || out != "{}" {
		t.Fatalf("the hook got %q, %v", out, err)
	}
	if me, err := s.current(ctx); err != nil || me.ID != "hosted" {
		t.Fatalf("after a hook call the server serves %+v, %v", me, err)
	}
}

// The receipt for a message to a group that has used its turns says that it woke
// nobody, and says nothing of the kind otherwise or for the person.
func TestTheReceiptSaysWhenAGroupHasUsedItsTurns(t *testing.T) {
	ctx := context.Background()
	b, err := bus.OpenAt(filepath.Join(t.TempDir(), "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	dir := t.TempDir()
	lead, _ := b.RegisterSession(ctx, "window-lead", dir, "Lead")
	b.RegisterSession(ctx, "window-docs", dir, "Docs Pass")
	g, err := b.CreateGroup(ctx, bus.Sender{}, "Review", []string{"Lead", "Docs Pass"})
	if err != nil {
		t.Fatal(err)
	}
	s := New(b)
	say := func() string {
		t.Helper()
		m, err := b.Send(ctx, bus.Sender{Session: &lead}, "#"+g.Name, "No mentions.", nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprint(s.sentReceipt(ctx, m).Content[0].(*mcp.TextContent).Text)
	}
	if got := say(); strings.Contains(got, "woke nobody") {
		t.Fatalf("an unlimited group's receipt: %s", got)
	}
	if err := b.SetGroupBudget(ctx, g.Name, 0); err != nil {
		t.Fatal(err)
	}
	if got := say(); !strings.Contains(got, "has used its turns for now, so this message woke nobody") {
		t.Fatalf("a spent group's receipt: %s", got)
	}
}
