package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"studio/internal/acp"
)

// subagentEvents records what the App sends the page.
type subagentEvents struct {
	mu     sync.Mutex
	events []struct {
		name string
		data any
	}
}

func (e *subagentEvents) sink(name string, data any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.events = append(e.events, struct {
		name string
		data any
	}{name, data})
}

// of is every event of one name sent so far.
func (e *subagentEvents) of(name string) []any {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []any
	for _, ev := range e.events {
		if ev.name == name {
			out = append(out, ev.data)
		}
	}
	return out
}

// waitFor waits for an event of a name that passes match.
func (e *subagentEvents) waitFor(t *testing.T, name string, match func(map[string]any) bool) map[string]any {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, d := range e.of(name) {
			m, ok := d.(map[string]any)
			if strs, isStrings := d.(map[string]string); isStrings {
				m, ok = map[string]any{}, true
				for k, v := range strs {
					m[k] = v
				}
			}
			if ok && match(m) {
				return m
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("no %s event came", name)
	return nil
}

func notify(a *App, session, update string) {
	params, _ := json.Marshal(map[string]any{"sessionId": session, "update": json.RawMessage(update)})
	a.newAgentHandler().Notify("session/update", params)
}

func spawnUpdate(child, name, task string) string {
	return fmt.Sprintf(`{"sessionUpdate":"subagent_spawned","subagentSessionId":%q,"name":%q,"task":%q,"prompt":"Look at %s","capabilities":{}}`, child, name, task, task)
}

func stateUpdate(child, state string) string {
	return fmt.Sprintf(`{"sessionUpdate":"subagent_state_update","subagentSessionId":%q,"state":%q}`, child, state)
}

func subagentByID(list []SubagentInfo, id string) SubagentInfo {
	for _, sa := range list {
		if sa.ID == id {
			return sa
		}
	}
	return SubagentInfo{}
}

// A chat's subagents, announced on the chat and on one another as the ACP
// subagent RFD (31 August 2026 draft) describes, become a tree of the chat
// with their states; their updates are kept as theirs and never reach the
// chat's conversation, its saved copy or its updates; and a running one
// holds exit and the restart for an update.
func TestSubagentsFormATreeOfTheChat(t *testing.T) {
	a, _ := providerTestApp(t)
	var ev subagentEvents
	a.sink = ev.sink
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := a.session(info.ID)
	notify(a, s.ID(), spawnUpdate("child-1", "Explore", "Find the parser"))
	notify(a, "child-1", `{"sessionUpdate":"agent_thought_chunk","content":{"type":"text","text":"I should "}}`)
	notify(a, "child-1", `{"sessionUpdate":"agent_thought_chunk","content":{"type":"text","text":"look."}}`)
	notify(a, "child-1", `{"sessionUpdate":"tool_call","toolCallId":"t1","title":"Read parser.go","kind":"read","status":"in_progress"}`)
	notify(a, "child-1", spawnUpdate("grandchild", "Verify", "Check the tests"))
	notify(a, "grandchild", `{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"Tests pass."}}`)
	notify(a, "child-1", `{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"The parser "}}`)
	notify(a, "child-1", `{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"is in parse.go."}}`)

	list := s.Info().Subagents
	if len(list) != 2 || list[0].ID != "child-1" || list[0].ParentID != s.ID() || list[0].RootID != s.ID() || list[1].ParentID != "child-1" || list[1].RootID != s.ID() {
		t.Fatalf("the tree: %+v", list)
	}
	if list[0].State != "running" || list[0].Name != "Explore" || list[0].Task != "Find the parser" || list[0].Prompt != "" || list[0].Started == "" || list[0].CanCancel {
		t.Fatalf("a running subagent: %+v", list[0])
	}
	for _, d := range ev.of("agent:update") {
		if m := d.(map[string]any); m["sessionId"] != s.ID() {
			t.Fatalf("a subagent's update reached the page as a chat's: %v", m)
		}
	}
	spawned := 0
	for _, d := range ev.of("agent:update") {
		if strings.Contains(string(d.(map[string]any)["update"].(json.RawMessage)), "subagent_spawned") {
			spawned++
		}
	}
	if spawned != 1 {
		t.Fatalf("the chat's own spawn should reach the page once as its update, and the nested one not at all: %d", spawned)
	}
	live := ev.of("agent:subagent-update")
	if len(live) != 7 || live[0].(map[string]any)["subagentId"] != "child-1" || live[0].(map[string]any)["rootId"] != s.ID() {
		t.Fatalf("the subagents' live updates: %d %v", len(live), live)
	}
	a.cmu.Lock()
	for _, m := range s.HistorySavedLocked().Messages {
		if strings.Contains(m.Text, "parse.go") || strings.Contains(m.Text, "Tests pass") || strings.Contains(m.Text, "Read parser.go") {
			a.cmu.Unlock()
			t.Fatalf("a subagent's words reached the chat's saved copy: %+v", m)
		}
	}
	a.cmu.Unlock()

	rec, err := a.SubagentUpdates("child-1")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Seq != 6 || rec.Subagent.Prompt != "Look at Find the parser" || len(rec.Updates) != 4 {
		t.Fatalf("the kept updates: seq %d prompt %q %d updates %s", rec.Seq, rec.Subagent.Prompt, len(rec.Updates), rec.Updates)
	}
	if !strings.Contains(string(rec.Updates[0]), `"text":"I should look."`) || !strings.Contains(string(rec.Updates[3]), `"text":"The parser is in parse.go."`) || !strings.Contains(string(rec.Updates[2]), "Verify") {
		t.Fatalf("pieces of one thought or message were not joined, or the nested spawn is missing: %s", rec.Updates)
	}
	if st := a.ExitStatus(); st.Chats != 1 {
		t.Fatalf("running subagents did not hold exit: %+v", st)
	}
	if n := a.workingChats(); n != 1 {
		t.Fatalf("running subagents did not hold the restart: %d", n)
	}

	notify(a, "child-1", stateUpdate("grandchild", "completed"))
	notify(a, s.ID(), stateUpdate("child-1", "failed"))
	list = s.Info().Subagents
	if subagentByID(list, "grandchild").State != "completed" || subagentByID(list, "child-1").State != "failed" || subagentByID(list, "child-1").Ended == "" || subagentByID(list, "child-1").Local {
		t.Fatalf("terminal states: %+v", list)
	}
	notify(a, s.ID(), spawnUpdate("child-1", "Explore", "Find the parser"))
	notify(a, s.ID(), stateUpdate("child-1", "completed"))
	if got := subagentByID(s.Info().Subagents, "child-1").State; got != "failed" {
		t.Fatalf("an ended subagent ran again or changed its reported end: %s", got)
	}
	if st := a.ExitStatus(); st.Chats != 0 || a.workingChats() != 0 {
		t.Fatalf("ended subagents still hold exit: %+v", st)
	}
	if err := a.CancelSubagent("child-1"); err == nil {
		t.Fatal("a finished subagent could be cancelled")
	}
	if _, err := a.SubagentUpdates("nobody"); err == nil {
		t.Fatal("an unknown subagent had updates")
	}
}

// A subagent the agent has not reported ended when its chat's prompt
// returns, or when the agent's process ends, is disconnected: shown as an
// unknown outcome, never as running, failed or cancelled, and nothing is
// sent to the agent about it. A late report replaces that state but never
// makes the subagent run again.
func TestSubagentsLeftRunningAreDisconnected(t *testing.T) {
	a, c := providerTestApp(t)
	var ev subagentEvents
	a.sink = ev.sink
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := a.session(info.ID)
	conn := &spawningConnection{testProviderConnection: c, a: a, chat: s.ID()}
	setTestConnection(a, "codex", conn)
	a.turnIO().Run(s, promptPointer("Go", []acp.ContentBlock{{Type: "text", Text: "Go"}}))
	list := s.Info().Subagents
	if len(list) != 2 {
		t.Fatalf("the turn's subagents: %+v", list)
	}
	for _, sa := range list {
		if sa.State != "disconnected" || !sa.Local || sa.Ended == "" {
			t.Fatalf("a subagent left running by the turn: %+v", sa)
		}
	}
	for _, call := range c.calls {
		if call.method != "session/prompt" && call.method != "session/new" && call.method != "session/set_config_option" {
			t.Fatalf("Dinah told the agent about a disconnected subagent: %s", call.method)
		}
	}
	if st := a.ExitStatus(); st.Chats != 0 {
		t.Fatalf("a disconnected subagent holds exit: %+v", st)
	}
	notify(a, s.ID(), stateUpdate("turn-child", "completed"))
	if got := subagentByID(s.Info().Subagents, "turn-child"); got.State != "completed" || got.Local {
		t.Fatalf("a late report did not replace the local state: %+v", got)
	}
	notify(a, s.ID(), spawnUpdate("turn-child", "Explore", "again"))
	if got := subagentByID(s.Info().Subagents, "turn-child").State; got != "completed" {
		t.Fatalf("a second announcement made an ended subagent run: %s", got)
	}

	// The agent's process ends with a subagent running.
	notify(a, s.ID(), spawnUpdate("late", "Plan", "Plan the release"))
	p := a.runtime.ProviderLocked("codex")
	go a.lifecycle().Watch("codex", p)
	conn.Close()
	select {
	case <-p.Finished():
	case <-time.After(3 * time.Second):
		t.Fatal("provider cleanup did not finish")
	}
	if got := subagentByID(s.Info().Subagents, "late"); got.State != "disconnected" || !got.Local {
		t.Fatalf("a subagent outlived its agent's process: %+v", got)
	}
	last := ev.of("agent:session")
	if l := last[len(last)-1].(SessionInfo); subagentByID(l.Subagents, "late").State != "disconnected" {
		t.Fatalf("the chat's last information still shows the subagent running: %+v", l.Subagents)
	}
}

// spawningConnection announces two subagents during a prompt, one nested in
// the other, and lets the prompt return without their ends, which the RFD
// calls an agent bug the client must still handle.
type spawningConnection struct {
	*testProviderConnection
	a    *App
	chat string
}

func (c *spawningConnection) Call(ctx context.Context, method string, params, result any) error {
	if method == "session/prompt" {
		notify(c.a, c.chat, spawnUpdate("turn-child", "Explore", "Find the parser"))
		notify(c.a, "turn-child", spawnUpdate("turn-grandchild", "Verify", "Check"))
	}
	return c.testProviderConnection.Call(ctx, method, params, result)
}

// A chat opened again gets its subagents from the agent's replay, without
// times, and the RFD's disconnected for one the history left unfinished;
// one the agent leaves running all the same is disconnected once the
// replay ends.
func TestSubagentsComeBackWithTheReplay(t *testing.T) {
	a, c := providerTestApp(t)
	ws := t.TempDir()
	info, err := a.NewSessionAt(ws)
	if err != nil {
		t.Fatal(err)
	}
	notify(a, info.ID, spawnUpdate("old", "Before", "From the last run"))
	a.mu.Lock()
	a.runtime.RemoveConversationLocked(info.ID)
	a.mu.Unlock()
	conn := &replayingConnection{testProviderConnection: c, a: a, chat: info.ID}
	setTestConnection(a, "codex", conn)
	if _, err := a.OpenSessionAt(ws, info.ID); err != nil {
		t.Fatal(err)
	}
	s := a.session(info.ID)
	list := s.Info().Subagents
	if len(list) != 2 {
		t.Fatalf("the replayed tree: %+v", list)
	}
	done, open := subagentByID(list, "done"), subagentByID(list, "open")
	if done.State != "disconnected" || done.Local || !done.Replayed || done.Started != "" || done.Ended != "" {
		t.Fatalf("a subagent the agent ended as disconnected: %+v", done)
	}
	if open.State != "disconnected" || !open.Local {
		t.Fatalf("a subagent the replay left running: %+v", open)
	}
	if done.ThisRun || open.ThisRun {
		t.Fatalf("a subagent from an earlier run is marked as this run's: %+v", list)
	}
	if _, _, ok := a.subagentService().Find("old"); ok {
		t.Fatal("a subagent from before the chat was opened again is still known")
	}
	rec, err := a.SubagentUpdates("done")
	if err != nil || len(rec.Updates) != 1 {
		t.Fatalf("the replayed subagent's updates: %v %+v", err, rec)
	}
}

// A subagent Dinah saw start is this run's, and stays this run's, with the
// times Dinah saw, when the agent replays it because its chat was closed and
// opened again in the same run; the chat's Subagents menu and tree show it,
// while a subagent only the replay brings is an earlier one.
func TestSubagentsOfThisRunSurviveAReopen(t *testing.T) {
	a, c := providerTestApp(t)
	ws := t.TempDir()
	info, err := a.NewSessionAt(ws)
	if err != nil {
		t.Fatal(err)
	}
	notify(a, info.ID, spawnUpdate("done", "Explore", "Find the parser"))
	notify(a, info.ID, stateUpdate("done", "completed"))
	live := subagentByID(a.session(info.ID).Info().Subagents, "done")
	if !live.ThisRun || live.Replayed || live.Started == "" || live.Ended == "" {
		t.Fatalf("a subagent Dinah saw start: %+v", live)
	}
	a.mu.Lock()
	a.runtime.RemoveConversationLocked(info.ID)
	a.mu.Unlock()
	setTestConnection(a, "codex", &replayingConnection{testProviderConnection: c, a: a, chat: info.ID})
	if _, err := a.OpenSessionAt(ws, info.ID); err != nil {
		t.Fatal(err)
	}
	list := a.session(info.ID).Info().Subagents
	done, open := subagentByID(list, "done"), subagentByID(list, "open")
	if !done.ThisRun || done.Replayed || done.Started != live.Started || done.Ended != live.Ended {
		t.Fatalf("this run's subagent after its chat was opened again: %+v, before %+v", done, live)
	}
	if open.ThisRun || !open.Replayed {
		t.Fatalf("a subagent only the replay brought: %+v", open)
	}
}

type replayingConnection struct {
	*testProviderConnection
	a    *App
	chat string
}

func (c *replayingConnection) Call(ctx context.Context, method string, params, result any) error {
	if method == "session/load" {
		notify(c.a, c.chat, spawnUpdate("done", "Explore", "Find the parser"))
		notify(c.a, "done", `{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"Found it."}}`)
		notify(c.a, c.chat, stateUpdate("done", "disconnected"))
		notify(c.a, c.chat, spawnUpdate("open", "Plan", "Never ended"))
	}
	return c.testProviderConnection.Call(ctx, method, params, result)
}

// The agent's requests for a subagent go to its chat: a permission card in
// the chat saying which subagent asks, answered cancelled when the subagent
// ends, and a file written in the chat's workspace and shown in the chat.
// This is the regression to avoid: before native sessions such requests
// arrived on the chat's own ID.
func TestSubagentRequestsReachTheirChat(t *testing.T) {
	a, _ := providerTestApp(t)
	var ev subagentEvents
	a.sink = ev.sink
	ws := t.TempDir()
	info, err := a.NewSessionAt(ws)
	if err != nil {
		t.Fatal(err)
	}
	notify(a, info.ID, spawnUpdate("child", "Explore", "Find the parser"))
	notify(a, "child", spawnUpdate("grandchild", "Verify", "Check"))
	ask := func(session, call string) chan any {
		out := make(chan any, 1)
		params, _ := json.Marshal(map[string]any{"sessionId": session, "toolCall": map[string]string{"toolCallId": call, "title": "Run go test"},
			"options": []map[string]string{{"optionId": "allow", "name": "Allow", "kind": "allow_once"}}})
		go func() {
			res, rerr := a.newAgentHandler().Request(context.Background(), "session/request_permission", params)
			if rerr != nil {
				out <- rerr
				return
			}
			out <- res
		}()
		return out
	}
	first := ask("child", "c1")
	card := ev.waitFor(t, "agent:permission", func(m map[string]any) bool {
		return m["toolCall"] != nil && strings.Contains(string(m["toolCall"].(json.RawMessage)), "c1")
	})
	who, _ := card["subagent"].(map[string]string)
	if card["sessionId"] != info.ID || who["id"] != "child" || who["name"] != "Explore" {
		t.Fatalf("the subagent's question did not reach its chat saying who asks: %v", card)
	}
	a.Answer(card["key"].(string), "allow")
	if got := <-first; got != acp.Selected("allow") {
		t.Fatalf("the person's answer did not reach the agent: %v", got)
	}

	// The grandchild's question is open when the child's end arrives.
	second := ask("grandchild", "c2")
	card = ev.waitFor(t, "agent:permission", func(m map[string]any) bool { return strings.Contains(string(m["toolCall"].(json.RawMessage)), "c2") })
	notify(a, info.ID, stateUpdate("child", "cancelled"))
	select {
	case got := <-second:
		if got != acp.Cancelled() {
			t.Fatalf("an ended subagent's question was answered: %v", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("an ended subagent's question stayed open")
	}
	ev.waitFor(t, "agent:answered", func(m map[string]any) bool { return m["key"] == card["key"] })
	if got := <-ask("child", "c3"); got != acp.Cancelled() {
		t.Fatalf("an ended subagent could still ask: %v", got)
	}

	// A write by a subagent lands in the chat's workspace and is shown there.
	target := filepath.Join(ws, "notes", "found.txt")
	params, _ := json.Marshal(map[string]any{"sessionId": "grandchild", "path": target, "content": "parse.go"})
	if _, rerr := a.newAgentHandler().Request(context.Background(), "fs/write_text_file", params); rerr != nil {
		t.Fatal(rerr.Message)
	}
	if b, err := os.ReadFile(target); err != nil || string(b) != "parse.go" {
		t.Fatalf("the subagent's file: %q %v", b, err)
	}
	wrote := ev.waitFor(t, "agent:file", func(map[string]any) bool { return true })
	if wrote["sessionId"] != info.ID || wrote["path"] != "notes/found.txt" || wrote["subagent"].(map[string]string)["name"] != "Verify" {
		t.Fatalf("the subagent's write was not shown in its chat: %v", wrote)
	}
	params, _ = json.Marshal(map[string]any{"sessionId": "grandchild", "path": target})
	if res, rerr := a.newAgentHandler().Request(context.Background(), "fs/read_text_file", params); rerr != nil || res.(acp.ReadTextFileResult).Content != "parse.go" {
		t.Fatalf("the subagent's read: %v %v", res, rerr)
	}
}

// A subagent keeps only its latest updates, and a chat only the updates of
// its latest subagents; the tree keeps every subagent.
func TestSubagentUpdatesAreBounded(t *testing.T) {
	a := NewApp()
	s := newSessionState(&sessionSeed{ID: "claude:bounded", Provider: "claude"}, sessionInitial{Open: true})
	a.runtime.PutConversationLocked(s.ID(), s)
	notify(a, s.ID(), spawnUpdate("busy", "Busy", "Many steps"))
	for i := 0; i < subagentUpdatesKept+50; i++ {
		notify(a, "busy", fmt.Sprintf(`{"sessionUpdate":"tool_call","toolCallId":"t%d","title":"Step %d","status":"completed"}`, i, i))
	}
	rec, err := a.SubagentUpdates("busy")
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Updates) != subagentUpdatesKept || !rec.Subagent.Trimmed || rec.Seq != int64(subagentUpdatesKept+50) || !strings.Contains(string(rec.Updates[0]), `"t50"`) {
		t.Fatalf("the bound by count: %d kept, trimmed %v, seq %d, first %s", len(rec.Updates), rec.Subagent.Trimmed, rec.Seq, rec.Updates[0])
	}
	big := strings.Repeat("x", 512<<10)
	for i := 0; i < 12; i++ {
		notify(a, "busy", fmt.Sprintf(`{"sessionUpdate":"tool_call_update","toolCallId":"big%d","content":[{"type":"content","content":{"type":"text","text":%q}}]}`, i, big))
	}
	rec, _ = a.SubagentUpdates("busy")
	size := 0
	for _, u := range rec.Updates {
		size += len(u)
	}
	if size > subagentBytesKept {
		t.Fatalf("the bound by size: %d bytes kept", size)
	}
	notify(a, s.ID(), stateUpdate("busy", "completed"))
	for i := 0; i < subagentBuffersKept; i++ {
		id := fmt.Sprintf("worker-%d", i)
		notify(a, s.ID(), spawnUpdate(id, "Worker", "Work"))
		notify(a, id, `{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"done"}}`)
	}
	rec, _ = a.SubagentUpdates("busy")
	if len(rec.Updates) != 0 || !rec.Subagent.Trimmed || len(s.Info().Subagents) != subagentBuffersKept+1 {
		t.Fatalf("the oldest finished subagent kept its updates past the chat's bound: %d updates, %d subagents", len(rec.Updates), len(s.Info().Subagents))
	}
	if rec, _ := a.SubagentUpdates("worker-0"); len(rec.Updates) != 1 {
		t.Fatal("a recent subagent lost its updates")
	}
}

// A subagent whose announcement allows it is cancelled with session/cancel
// for its own session, and its open questions are answered cancelled.
func TestCancelSubagentUsesItsSession(t *testing.T) {
	a, c := providerTestApp(t)
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	notify(a, info.ID, spawnUpdate("fixed", "Fixed", "Cannot stop"))
	notify(a, info.ID, `{"sessionUpdate":"subagent_spawned","subagentSessionId":"stoppable","name":"Stoppable","task":"Can stop","capabilities":{"cancel":true}}`)
	if err := a.CancelSubagent("fixed"); err == nil {
		t.Fatal("a subagent without the cancel capability was cancelled")
	}
	if !subagentByID(a.session(info.ID).Info().Subagents, "stoppable").CanCancel {
		t.Fatal("the cancel capability was not kept")
	}
	if err := a.CancelSubagent("stoppable"); err != nil {
		t.Fatal(err)
	}
	last := c.calls[len(c.calls)-1]
	if last.method != "session/cancel" || last.params.(acp.CancelParams).SessionID != "stoppable" {
		t.Fatalf("the cancel did not go to the subagent's session: %+v", last)
	}
}
