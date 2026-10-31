package main

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"studio/internal/acp"
	"studio/internal/sessionhost"
)

func choiceOption(id, name, category, value, valueName string) acp.ConfigOption {
	return acp.ConfigOption{ID: id, Name: name, Category: category, Type: "select", CurrentValue: json.RawMessage(`"` + value + `"`),
		Options: json.RawMessage(`[{"value":"` + value + `","name":"` + valueName + `"}]`)}
}

// switchSource is a chat of a provider with a model, a permission mode and an
// effort chosen in it, with one finished turn.
func switchSource(t *testing.T, a *App, provider string) *Session {
	t.Helper()
	s := newSessionState(&sessionSeed{ID: "source-" + provider, Provider: provider, Cwd: t.TempDir(), Title: "Source Chat"}, sessionInitial{Open: true})
	a.runtime.PutConversationLocked(s.ID(), s)
	a.transcriptCapture().Initialize(s, false)
	a.transcriptCapture().RecordMessage(s, "you", "Create a report.", false, false)
	a.transcriptCapture().RecordMessage(s, "agent", "The report is saved.", false, false)
	a.transcriptCapture().FinishTurn(s, nil, "end_turn")
	if err := a.optionService().Update(s, []acp.ConfigOption{
		choiceOption("model", "Model", "model", "m1", "Model One"),
		choiceOption("mode", "Mode", "mode", "bypass", "Bypass permissions"),
		choiceOption("effort", "Effort", "thought_level", "high", "High"),
	}, false); err != nil {
		t.Fatal(err)
	}
	return s
}

// switchEvents collects the chat's state as the page is told it, with its stage.
type switchEvent struct {
	Session string `json:"sessionId"`
	Switch  *struct {
		Kind   string `json:"kind"`
		To     string `json:"to"`
		Stage  string `json:"stage"`
		Label  string `json:"label"`
		Failed string `json:"failed"`
	} `json:"switch"`
	StateVer float64 `json:"stateVer"`
}

func watchSwitch(a *App) *[]switchEvent {
	var events []switchEvent
	a.sink = func(name string, data any) {
		if name != "agent:session" {
			return
		}
		raw, _ := json.Marshal(data)
		var e switchEvent
		if json.Unmarshal(raw, &e) == nil {
			events = append(events, e)
		}
	}
	return &events
}

func stagesOf(events []switchEvent, id string) (stages []string, labels []string, last *switchEvent) {
	for i, e := range events {
		if e.Session != id {
			continue
		}
		last = &events[i]
		// A stage begins once. The chat's information is sent again whenever something else in it changes, such as its
		// state when the engine first works it out (chat_state_events.go), and that snapshot still says the stage
		// the switch is in, so a snapshot that repeats the stage just reported is not the stage beginning again.
		if e.Switch != nil && e.Switch.Stage != "" && (len(stages) == 0 || stages[len(stages)-1] != e.Switch.Stage) {
			stages = append(stages, e.Switch.Stage)
			labels = append(labels, e.Switch.Label)
		}
	}
	return stages, labels, last
}

// The confirmation of a provider switch is built from facts the engine has: the
// two ends by their display names, what the handoff holds, which of the chat's
// settings the new chat starts without, and what must finish first.
func TestTheConfirmationOfAProviderSwitchIsBuiltFromFacts(t *testing.T) {
	a, _ := providerTestApp(t)
	s := switchSource(t, a, "devin")
	p, err := a.PreviewSessionContinuation(s.ID(), "codex")
	if err != nil {
		t.Fatal(err)
	}
	f := p.Facts
	if f.Kind != "provider" || f.From.Provider != "devin" || f.From.ProviderName != "Devin" || f.From.Model != "Model One" || f.To.Provider != "codex" || f.To.ProviderName != "Codex" ||
		f.From.Workspace != s.Cwd() || f.From.WorkspaceName != filepath.Base(s.Cwd()) || f.To.Workspace != s.Cwd() {
		t.Fatalf("the ends %+v", f)
	}
	if f.Handoff != nil || p.Context != "" || len(p.History) != 0 {
		t.Fatalf("confirmation prepared a handoff: %+v", p)
	}
	var notCarried []string
	for _, o := range f.NotCarried {
		notCarried = append(notCarried, o.Name+"="+o.Value)
	}
	if !slices.Equal(notCarried, []string{"Model=Model One", "Mode=Bypass permissions", "Effort=High"}) || len(f.Carried) != 0 {
		t.Fatalf("settings: not carried %v, carried %+v", notCarried, f.Carried)
	}
	if f.Running.Turn || f.Running.Queued != 0 || f.Running.Approvals || f.Running.Applets != 0 || f.Running.CanStop {
		t.Fatalf("running %+v", f.Running)
	}
}

// A switch reports each stage as it begins, in the chat's numbered state with a
// label from the engine, and ends with no stage and no failure.
func TestASwitchReportsEachStageInTheChatsState(t *testing.T) {
	a, _ := providerTestApp(t)
	events := watchSwitch(a)
	s := switchSource(t, a, "devin")
	p, _ := a.PreviewSessionContinuation(s.ID(), "codex")
	if _, err := a.ContinueSessionWithProvider(s.ID(), "codex", p.Revision); err != nil {
		t.Fatal(err)
	}
	stages, labels, last := stagesOf(*events, s.ID())
	if !slices.Equal(stages, []string{"summary", "starting", "handover", "opening"}) ||
		!slices.Equal(labels, []string{"Building the summary of the conversation", "Starting Codex", "Handing the conversation over", "Opening the new chat"}) {
		t.Fatalf("stages %v, labels %v", stages, labels)
	}
	if last == nil || last.Switch != nil || s.SwitchState() != nil {
		t.Fatalf("after the switch %+v", last)
	}
	var versions []float64
	for _, e := range *events {
		if e.Session == s.ID() {
			versions = append(versions, e.StateVer)
		}
	}
	if !slices.IsSorted(versions) || len(versions) < 5 {
		t.Fatalf("the events are not numbered in order: %v", versions)
	}
}

// A switch that fails at any stage leaves the chat as it was and says why until the
// person dismisses it: the chat is open, not superseded, not linked to a new chat,
// and can be switched again.
func TestASwitchThatFailsLeavesTheChatAsItWas(t *testing.T) {
	a, c := providerTestApp(t)
	events := watchSwitch(a)
	s := switchSource(t, a, "devin")
	unchanged := func(name string, wantStages []string, reason string) {
		t.Helper()
		stages, _, last := stagesOf(*events, s.ID())
		if !slices.Equal(stages, wantStages) || last == nil || last.Switch == nil || last.Switch.Stage != "" || !strings.Contains(last.Switch.Failed, reason) || last.Switch.Kind != "provider" || last.Switch.To != "Codex" {
			t.Fatalf("%s: stages %v, last %+v", name, stages, last)
		}
		st := s.State()
		if !st.Info.Open || st.Superseded || st.Switching || st.Info.Working || a.session(s.ID()) != s || st.Info.Switch == nil || st.Info.Switch.Failed == "" {
			t.Fatalf("%s: the chat changed: %+v", name, st)
		}
		if head, err := a.ResolveSessionContinuation(s.ID()); err != nil || head != s.ID() {
			t.Fatalf("%s: the chat is linked to %q (%v)", name, head, err)
		}
		*events = nil
	}
	p, _ := a.PreviewSessionContinuation(s.ID(), "codex")
	// The new provider cannot start a session.
	c.newErr = errors.New("the provider would not start")
	if _, err := a.ContinueSessionWithProvider(s.ID(), "codex", p.Revision); err == nil {
		t.Fatal("a switch to a provider that would not start succeeded")
	}
	unchanged("starting", []string{"summary", "starting"}, "would not start")
	// The conversation changes while the new provider starts.
	c.newErr = nil
	c.onNew = func() {
		a.transcriptCapture().RecordMessage(s, "agent", "Something new", false, false)
		a.transcriptCapture().FinishTurn(s, nil, "end_turn")
		c.onNew = nil
	}
	p, _ = a.PreviewSessionContinuation(s.ID(), "codex")
	if _, err := a.ContinueSessionWithProvider(s.ID(), "codex", p.Revision); err == nil {
		t.Fatal("a switch of a conversation that changed succeeded")
	}
	unchanged("handover", []string{"summary", "starting", "handover"}, "changed while connecting")
	// A stale confirmation fails at the first stage.
	if _, err := a.ContinueSessionWithProvider(s.ID(), "codex", "stale"); err == nil {
		t.Fatal("a stale confirmation succeeded")
	}
	unchanged("summary", []string{"summary"}, "review the provider switch again")
	// The person dismisses the failure, and the chat can be switched after all.
	if err := a.DismissSwitchFailure(s.ID()); err != nil || s.SwitchState() != nil {
		t.Fatalf("the failure was not dismissed: %v %+v", err, s.SwitchState())
	}
	p, _ = a.PreviewSessionContinuation(s.ID(), "codex")
	if _, err := a.ContinueSessionWithProvider(s.ID(), "codex", p.Revision); err != nil || s.SwitchState() != nil {
		t.Fatalf("a switch after the failures: %v", err)
	}
}

// A chat moves to another workspace by continuation: the same provider, in the
// chosen folder, linked to the old chat, which stays where it was. The settings that
// grant no authority go with it, those that do are listed as not carried and not
// applied, and the link records the workspace it crossed so the new chat can be
// continued again.
func TestAChatMovesToAnotherWorkspaceByContinuation(t *testing.T) {
	a, c := providerTestApp(t)
	events := watchSwitch(a)
	s := switchSource(t, a, "codex")
	other := t.TempDir()
	p, err := a.PreviewSessionWorkspaceMove(s.ID(), other)
	if err != nil {
		t.Fatal(err)
	}
	f := p.Facts
	var carried, notCarried []string
	for _, o := range f.Carried {
		carried = append(carried, o.Name+"="+o.Value)
	}
	for _, o := range f.NotCarried {
		notCarried = append(notCarried, o.Name+"="+o.Value)
	}
	if f.Kind != "workspace" || f.From.Workspace != s.Cwd() || f.To.Workspace != other || f.To.WorkspaceName != filepath.Base(other) || f.From.Provider != "codex" || f.To.Provider != "codex" ||
		!slices.Equal(carried, []string{"Model=Model One", "Effort=High"}) || !slices.Equal(notCarried, []string{"Mode=Bypass permissions"}) || p.Revision == "" {
		t.Fatalf("facts %+v (carried %v, not carried %v)", f, carried, notCarried)
	}
	// The two new sessions, the move and the compaction after it, get their own IDs.
	c.newIDs = []string{"codex:moved", "codex:compacted"}
	// The new session offers the same settings, with the provider's own mode.
	c.options = []acp.ConfigOption{
		choiceOption("model", "Model", "model", "m1", "Model One"),
		choiceOption("mode", "Mode", "mode", "default", "Manual"),
		choiceOption("effort", "Effort", "thought_level", "high", "High"),
	}
	result, err := a.ContinueSessionInWorkspace(s.ID(), other, p.Revision)
	if err != nil {
		t.Fatal(err)
	}
	next := a.session(result.Session.ID)
	if next == nil || next.Cwd() != other || next.Provider() != "codex" || s.Cwd() == other || a.session(s.ID()) != s {
		t.Fatalf("the chats: new %v, old folder %s", next, s.Cwd())
	}
	if head, err := a.ResolveSessionContinuation(s.ID()); err != nil || head != result.Session.ID {
		t.Fatalf("the old chat links to %q (%v)", head, err)
	}
	tr, err := readTranscript(result.Session.ID)
	if err != nil || tr.ParentSessionID != s.ID() || !strings.EqualFold(filepath.ToSlash(tr.ParentWorkspace), filepath.ToSlash(s.Cwd())) || !strings.EqualFold(filepath.ToSlash(tr.Workspace), filepath.ToSlash(other)) {
		t.Fatalf("the new chat's record %+v %v", tr, err)
	}
	// The settings that grant no authority were applied to the new chat, and the
	// permission mode was not.
	applied := map[string]bool{}
	for _, call := range c.calls {
		if p, ok := call.params.(acp.SetConfigOptionParams); ok && call.method == "session/set_config_option" && p.SessionID == result.Session.ID {
			applied[p.ConfigID] = true
		}
	}
	if !applied["model"] || !applied["effort"] || applied["mode"] || len(result.OptionsNotCarried) != 0 {
		t.Fatalf("applied %v, refused %v", applied, result.OptionsNotCarried)
	}
	stages, labels, _ := stagesOf(*events, s.ID())
	if !slices.Equal(stages, []string{"summary", "starting", "handover", "opening"}) || labels[1] != "Starting the chat in "+filepath.Base(other) {
		t.Fatalf("stages %v labels %v", stages, labels)
	}
	// The history of the moved chat reads through the link across workspaces, since it
	// was recorded, and holds the earlier conversation.
	chain, err := sessionhost.TranscriptHistory(configDir, next.ID(), next.Cwd(), map[string]bool{})
	if err != nil || len(chain) < 2 || chain[0].Text != "Create a report." {
		t.Fatalf("the moved chat's history: %d messages, %v", len(chain), err)
	}
	// The moved chat can itself be continued.
	after, err := a.continuations().Compact(next)
	if err != nil {
		t.Fatalf("the moved chat could not be compacted: %v", err)
	}
	if after.ParentSessionID != next.ID() || after.Session.Cwd != other {
		t.Fatalf("the compaction %+v", after)
	}
}

// A move to the workspace the chat is already in, to a folder that is not absolute,
// and while a turn is running, is not made, and the last is a fact the confirmation
// shows.
func TestAWorkspaceMoveRefusesWhatCannotBeDone(t *testing.T) {
	a, _ := providerTestApp(t)
	s := switchSource(t, a, "codex")
	if _, err := a.PreviewSessionWorkspaceMove(s.ID(), s.Cwd()); err == nil || !strings.Contains(err.Error(), "already in that workspace") {
		t.Fatalf("the same workspace: %v", err)
	}
	if _, err := a.PreviewSessionWorkspaceMove(s.ID(), "relative/path"); err == nil {
		t.Fatal("a folder that is not absolute was accepted")
	}
	if _, err := a.ContinueSessionInWorkspace(s.ID(), s.Cwd(), "x"); err == nil {
		t.Fatal("a move to the same workspace was made")
	}
	if _, err := a.PreviewSessionWorkspaceMove("no-such-chat", t.TempDir()); err == nil {
		t.Fatal("a chat that is not open was moved")
	}
	s.StartWork()
	defer s.ReleaseWork()
	other := t.TempDir()
	p, err := a.PreviewSessionWorkspaceMove(s.ID(), other)
	if err != nil || p.Blocked == "" || !p.Facts.Running.Turn || !p.Facts.Running.CanStop || p.Facts.Kind != "workspace" || p.Facts.To.Workspace != other {
		t.Fatalf("a running turn: %+v %v", p, err)
	}
	if _, err := a.ContinueSessionInWorkspace(s.ID(), other, p.Revision); err == nil {
		t.Fatal("a chat with a turn running was moved")
	}
}
