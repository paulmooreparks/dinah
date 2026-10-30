package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"

	"studio/internal/acp"
	"studio/internal/judge"
)

// judgeConn is a provider connection whose agent answers a judgement call, and
// does what a test makes it do while the call is open.
type judgeConn struct {
	*testProviderConnection
	a  *App
	mu sync.Mutex
	// id is the session session/new gives.
	id string
	// beforeNew runs inside session/new, as an agent sends updates before it answers;
	// duringPrompt runs inside session/prompt, as the agent works.
	beforeNew, duringPrompt func(c *judgeConn)
	// newParams, and whether the folder was empty when the prompt was made.
	newParams   acp.NewSessionParams
	folderEmpty bool
	// answer is what the agent says, and used the usage it reports; none when
	// noUsage is set.
	answer  string
	used    int
	noUsage bool
	// asked is the answer to a permission request the agent made in the call.
	asked acp.PermissionOutcome
}

func (c *judgeConn) update(session string, update map[string]any) {
	params, _ := json.Marshal(map[string]any{"sessionId": session, "update": update})
	c.a.newAgentHandler().Notify("session/update", params)
}

func (c *judgeConn) Call(ctx context.Context, method string, p, out any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch method {
	case "session/new":
		c.calls = append(c.calls, providerCall{method, p})
		c.newParams = p.(acp.NewSessionParams)
		if c.beforeNew != nil {
			c.beforeNew(c)
		}
		*out.(*acp.NewSessionResult) = acp.NewSessionResult{SessionID: c.id, ConfigOptions: c.options}
		return nil
	case "session/prompt":
		c.calls = append(c.calls, providerCall{method, p})
		if entries, err := os.ReadDir(c.newParams.Cwd); err == nil {
			c.folderEmpty = len(entries) == 0
		}
		params := p.(acp.PromptParams)
		if c.duringPrompt != nil {
			c.duringPrompt(c)
		}
		c.update(params.SessionID, map[string]any{"sessionUpdate": "agent_thought_chunk", "content": map[string]string{"type": "text", "text": "thinking"}})
		c.update(params.SessionID, map[string]any{"sessionUpdate": "agent_message_chunk", "content": map[string]string{"type": "text", "text": c.answer}})
		if !c.noUsage {
			c.update(params.SessionID, map[string]any{"sessionUpdate": "usage_update", "used": c.used, "size": 200000})
		}
		*out.(*acp.PromptResult) = acp.PromptResult{StopReason: "end_turn"}
		return nil
	case "session/close", "session/delete":
		c.calls = append(c.calls, providerCall{method, p})
		return nil
	}
	return c.testProviderConnection.Call(ctx, method, p, out)
}

func judgeConnApp(t *testing.T) (*App, *judgeConn, *[]string) {
	t.Helper()
	a, c := providerTestApp(t)
	jc := &judgeConn{testProviderConnection: c, a: a, id: "judge-session", answer: `{"line":"Judged"}`, used: 4321}
	setTestConnection(a, "codex", jc)
	init := a.runtime.ProviderLocked("codex").Initialization()
	init.AgentCapabilities.SessionCapabilities.Close = &struct{}{}
	setTestInitialization(a, "codex", init)
	var mu sync.Mutex
	var events []string
	a.sink = func(name string, _ any) {
		mu.Lock()
		events = append(events, name)
		mu.Unlock()
	}
	return a, jc, &events
}

func (c *judgeConn) called(method string) int {
	n := 0
	for _, call := range c.calls {
		if call.method == method {
			n++
		}
	}
	return n
}

// A call runs in a session of its own, in an empty folder, with no tool servers,
// on the model it was asked for; the answer is the agent's message and not its
// thinking; its cost is the usage the agent reports, in the provider's unit; and the
// session is closed and the folder removed.
func TestAnAgentJudgeCallRunsInAnEmptyFolderAndCountsTheReportedUsage(t *testing.T) {
	a, c, events := judgeConnApp(t)
	j := &acpJudge{app: a, provider: "codex", model: "changed"}
	resp, err := j.Ask(context.Background(), judge.Request{System: "Name the chat.", Input: `{"x":1}`, JSON: true})
	if err != nil || resp.Text != `{"line":"Judged"}` || resp.Model != "changed" || resp.Provider != "codex" {
		t.Fatalf("response %+v %v", resp, err)
	}
	if resp.Cost != (judge.Cost{Unit: "tokens", In: 4321}) {
		t.Fatalf("cost %+v", resp.Cost)
	}
	if c.newParams.Cwd == "" || len(c.newParams.McpServers) != 0 || c.newParams.McpServers == nil || !c.folderEmpty {
		t.Fatalf("session params %+v, folder empty %v", c.newParams, c.folderEmpty)
	}
	if _, err := os.Stat(c.newParams.Cwd); !os.IsNotExist(err) {
		t.Fatalf("the folder was not removed: %v", err)
	}
	if c.called("session/set_config_option") != 1 || c.called("session/close") != 1 {
		t.Fatalf("calls %+v", c.calls)
	}
	prompt := c.calls[len(c.calls)-2].params.(acp.PromptParams).Prompt[0].Text
	if !strings.Contains(prompt, "Name the chat.") || !strings.Contains(prompt, `{"x":1}`) {
		t.Fatalf("prompt %q", prompt)
	}
	// Nothing of the call reached a chat or the page.
	for _, e := range *events {
		if strings.HasPrefix(e, "agent:") || strings.HasPrefix(e, "chat:") {
			t.Fatalf("the call was shown as %s: %v", e, *events)
		}
	}
}

// A session is deleted when the agent can delete one, so that it is not among the
// agent's own sessions.
func TestAnAgentJudgeSessionIsDeletedWhenTheAgentCan(t *testing.T) {
	a, c, _ := judgeConnApp(t)
	init := a.runtime.ProviderLocked("codex").Initialization()
	init.AgentCapabilities.SessionCapabilities.Delete = &struct{}{}
	init.AgentCapabilities.SessionCapabilities.Close = &struct{}{}
	setTestInitialization(a, "codex", init)
	if _, err := (&acpJudge{app: a, provider: "codex"}).Ask(context.Background(), judge.Request{Input: "{}"}); err != nil {
		t.Fatal(err)
	}
	if c.called("session/delete") != 1 || c.called("session/close") != 0 {
		t.Fatalf("calls %+v", c.calls)
	}
}

// An agent that asks to act is refused with its own rejection, the call is
// cancelled and fails, and a request for a file is an error.
func TestAnAgentJudgeIsRefusedEveryRequest(t *testing.T) {
	a, c, _ := judgeConnApp(t)
	var fileErr *acp.Error
	var chosen any
	c.duringPrompt = func(c *judgeConn) {
		perm, _ := json.Marshal(acp.RequestPermissionParams{SessionID: c.id, ToolCall: json.RawMessage(`{"title":"rm"}`), Options: []acp.PermissionOption{{OptionID: "yes", Kind: "allow_once"}, {OptionID: "no", Kind: "reject_once"}}})
		chosen, _ = c.a.newAgentHandler().Request(context.Background(), "session/request_permission", perm)
		read, _ := json.Marshal(acp.ReadTextFileParams{SessionID: c.id, Path: "C:/Windows/win.ini"})
		_, fileErr = c.a.newAgentHandler().Request(context.Background(), "fs/read_text_file", read)
	}
	_, err := (&acpJudge{app: a, provider: "codex"}).Ask(context.Background(), judge.Request{Input: "{}"})
	if err == nil || !strings.Contains(err.Error(), "asked to act") {
		t.Fatalf("a call whose agent asked to act: %v", err)
	}
	if out, ok := chosen.(acp.PermissionOutcome); !ok || out.Outcome.OptionID != "no" || out.Outcome.Outcome != "selected" {
		t.Fatalf("the permission answer %+v", chosen)
	}
	if fileErr == nil {
		t.Fatal("a judge read a file")
	}
	if c.called("session/close") != 1 {
		t.Fatalf("the session was not ended: %+v", c.calls)
	}
	// With no rejection offered the request is answered as cancelled.
	c.duringPrompt = func(c *judgeConn) {
		perm, _ := json.Marshal(acp.RequestPermissionParams{SessionID: c.id, Options: []acp.PermissionOption{{OptionID: "yes", Kind: "allow_always"}}})
		chosen, _ = c.a.newAgentHandler().Request(context.Background(), "session/request_permission", perm)
	}
	if _, err := (&acpJudge{app: a, provider: "codex"}).Ask(context.Background(), judge.Request{Input: "{}"}); err == nil {
		t.Fatal("a call that was only offered an allow succeeded")
	}
	if out, ok := chosen.(acp.PermissionOutcome); !ok || out.Outcome.Outcome != "cancelled" {
		t.Fatalf("the answer with no rejection %+v", chosen)
	}
}

// An agent that reports no usage has a call whose cost is unknown, which is kept as
// such: no figure is made up, and the budget says calls were not counted.
func TestACallWhoseUsageIsNotReportedIsUnknownAndTheBudgetSaysSo(t *testing.T) {
	a, c, _ := judgeConnApp(t)
	c.noUsage = true
	resp, err := (&acpJudge{app: a, provider: "codex"}).Ask(context.Background(), judge.Request{Input: "{}"})
	if err != nil || !resp.Cost.Unknown || resp.Cost.In != 0 || resp.Cost.Out != 0 {
		t.Fatalf("response %+v %v", resp, err)
	}
	a.judging.noAuto = true
	limit := 1000
	choose(t, a, func(s *JudgementSettings) {
		s.Provider = "codex"
		s.Model = "changed"
		for i := range s.Budgets {
			if s.Budgets[i].Provider == "codex" {
				s.Budgets[i].Limit = &limit
			}
		}
	})
	c.answer = "Judged line"
	set, _ := a.judges()
	r := judge.Call(context.Background(), set, windowTestKind, windowTestIn{Line: "Rule line"}, judge.Effect{})
	if r.Why == judge.WhyBudget {
		t.Fatalf("a call with no spending was held: %+v", r)
	}
	uses, err := set.BudgetUses()
	if err != nil || len(uses) != 1 || uses[0].Provider != "codex" || uses[0].Spent != 0 || uses[0].Unmeasured != 1 {
		t.Fatalf("budget %+v %v", uses, err)
	}
	// With usage reported the call is counted in tokens.
	c.noUsage = false
	judge.Call(context.Background(), set, windowTestKind, windowTestIn{Line: "Rule line 2"}, judge.Effect{})
	uses, _ = set.BudgetUses()
	if len(uses) != 1 || uses[0].Spent != 4321 || uses[0].Unmeasured != 1 {
		t.Fatalf("budget after a counted call %+v", uses)
	}
}

// A session's first updates can arrive before the call knows its id. The commands the
// agent offers in a judge's empty folder never replace the chats', and the update of
// another session that arrives in the same moment is not lost.
func TestAJudgeSessionsCommandsNeverReachTheChats(t *testing.T) {
	a, c, events := judgeConnApp(t)
	a.mu.Lock()
	a.commands = nil
	a.mu.Unlock()
	commands := func(name string) map[string]any {
		return map[string]any{"sessionUpdate": "available_commands_update", "availableCommands": []map[string]string{{"name": name, "description": "d"}}}
	}
	c.beforeNew = func(c *judgeConn) {
		c.update("judge-session", commands("judge-only"))
		c.update("a-chat-session", commands("chat-command"))
	}
	c.duringPrompt = func(c *judgeConn) { c.update("judge-session", commands("judge-late")) }
	if _, err := (&acpJudge{app: a, provider: "codex"}).Ask(context.Background(), judge.Request{Input: "{}"}); err != nil {
		t.Fatal(err)
	}
	// A late update after the call is dropped too.
	c.update("judge-session", commands("judge-after"))
	a.mu.Lock()
	got := append([]Skill(nil), a.commands...)
	a.mu.Unlock()
	if len(got) != 1 || got[0].Name != "chat-command" {
		t.Fatalf("the commands are %+v", got)
	}
	n := 0
	for _, e := range *events {
		if e == "agent:commands" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d command events: %v", n, *events)
	}
}

// The models a provider offers come from its model setting, none of them resident,
// and the session that listed them is ended.
func TestAnAgentProvidersModelsComeFromItsModelSetting(t *testing.T) {
	a, c, _ := judgeConnApp(t)
	models, err := acpJudgeModels(a, context.Background(), "codex")
	if err != nil || len(models) != 4 || models[0].Name != "initial" || models[3].Name != "changed" {
		t.Fatalf("models %+v %v", models, err)
	}
	for _, m := range models {
		if m.Resident {
			t.Fatalf("%s is resident", m.Name)
		}
	}
	if c.called("session/close") != 1 {
		t.Fatalf("calls %+v", c.calls)
	}
}

// Providers that use the person's plan are listed with a note and are not asked for
// their models unless chosen; Ollama judges until the person chooses another.
func TestHostedProvidersAreListedWithANoteAndDoNotJudgeByDefault(t *testing.T) {
	a, _, _ := judgeConnApp(t)
	list, err := a.JudgementProviders()
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]JudgementProvider{}
	for _, p := range list {
		byID[p.ID] = p
	}
	if byID["codex"].Note == "" || byID["claude"].Note == "" || byID["ollama"].Note != "" || byID["codex"].Unit != "tokens" {
		t.Fatalf("providers %+v", list)
	}
	if defaultJudgementSettings().Provider != "ollama" {
		t.Fatalf("the default judge is %s", defaultJudgementSettings().Provider)
	}
	models, _ := a.JudgementModels()
	for _, m := range models {
		if m.Provider != "ollama" {
			t.Fatalf("a provider that was not chosen was asked for models: %+v", models)
		}
	}
}

// A hosted provider's typical cost is the median of the calls it has recorded, shown
// to the page with how many calls it rests on, and nothing before it has any.
func TestAHostedProvidersTypicalCostComesFromItsRecordedCalls(t *testing.T) {
	a, c, _ := judgeConnApp(t)
	cost := func() JudgementProvider {
		list, err := a.JudgementProviders()
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range list {
			if p.ID == "codex" {
				return p
			}
		}
		t.Fatal("codex is not listed")
		return JudgementProvider{}
	}
	if p := cost(); p.TypicalCost != 0 || p.Calls != 0 {
		t.Fatalf("before any call %+v", p)
	}
	a.judging.noAuto = true
	choose(t, a, func(s *JudgementSettings) { s.Provider, s.Model = "codex", "changed" })
	set, _ := a.judges()
	for _, used := range []int{26000, 42000, 28000} {
		c.used, c.answer = used, "Judged line"
		judge.Call(context.Background(), set, windowTestKind, windowTestIn{Line: "Rule line"}, judge.Effect{})
	}
	if p := cost(); p.TypicalCost != 28000 || p.Calls != 3 {
		t.Fatalf("after three calls %+v", p)
	}
}
