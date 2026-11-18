package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/acp"
	"studio/internal/applet"
	"studio/internal/bus"
)

// withoutBuiltinSkills is a catalogue without the skills built into the program, which every catalogue lists, for a
// test about the skills of the folders it made.
func withoutBuiltinSkills(c SkillCatalog) SkillCatalog {
	kept := []CatalogSkill{}
	for _, s := range c.Skills {
		if s.Scope != builtinSkillScope {
			kept = append(kept, s)
		}
	}
	c.Skills = kept
	return c
}

// duplicationOptions are a program's settings with a model, a permission mode and an effort, as an agent reports them
// for a new chat.
func duplicationOptions() []acp.ConfigOption {
	model := newTestProviderConnection().options[0]
	return []acp.ConfigOption{model,
		{ID: "mode", Name: "Mode", Category: "mode", Type: "select", CurrentValue: json.RawMessage(`"default"`),
			Options: json.RawMessage(`[{"value":"default","name":"Manual"},{"value":"acceptEdits","name":"Accept Edits"},{"value":"bypassPermissions","name":"Bypass Permissions"}]`)},
		{ID: "effort", Name: "Effort", Category: "thought_level", Type: "select", CurrentValue: json.RawMessage(`"medium"`),
			Options: json.RawMessage(`[{"value":"low","name":"Low"},{"value":"medium","name":"Medium"},{"value":"high","name":"High"}]`)},
	}
}

// duplicationApp is an app with a lead chat whose mode is Accept Edits and model Changed, in a workspace the app has.
// fresh makes the agent report its new-chat settings again, since the test agent keeps the last settings it was given.
func duplicationApp(t *testing.T, ids ...string) (a *App, c *testProviderConnection, lead *Session, fresh func()) {
	t.Helper()
	a, c = providerTestApp(t)
	fresh = func() { c.options = duplicationOptions() }
	fresh()
	c.newIDs = append([]string{"codex:lead"}, ids...)
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	lead = a.session(info.ID)
	a.settings.WorkspaceRecords = []WorkspaceRecord{{ID: "w", Name: "W", ExecutionPath: lead.Cwd()}}
	for id, value := range map[string]string{"mode": `"acceptEdits"`, "model": `"changed"`} {
		if _, err := a.SetSessionOption(lead.ID(), id, json.RawMessage(value)); err != nil {
			t.Fatal(err)
		}
	}
	fresh()
	return a, c, lead, fresh
}

func describeTeam(t *testing.T, a *App, ctx context.Context, member string) (teamDescribeNode, error) {
	t.Helper()
	input, _ := json.Marshal(map[string]any{"member": member})
	res, err := a.teamDescribeHandler(ctx, string(input))
	if err != nil {
		return teamDescribeNode{}, err
	}
	var out struct{ Team teamDescribeNode }
	if err := json.Unmarshal([]byte(res.OutputJSON), &out); err != nil {
		t.Fatal(err)
	}
	return out.Team, nil
}

func optionOf(s *Session, id string) string {
	for _, o := range s.Options() {
		if o.ID == id {
			return judgeOptionString(o.CurrentValue)
		}
	}
	return ""
}

// A lead builds a nested team with start@2, under itself and under its own member, with settings; describe returns the
// tree with each member's remit, model value and name, and settings, a closed member's from what was saved for it.
// Assignments do not change a remit.
func TestDescribeReturnsTheTreeAndStartUnderASubLeadAppliesSettings(t *testing.T) {
	a, _, lead, fresh := duplicationApp(t, "codex:checkout", "codex:docks")
	ctx := asChat(a, lead)
	out, err := call(t, a.teamStart2Handler, ctx, map[string]any{"title": "Checkout", "brief": "Lead the checkout work.\nDetails follow.", "remit": "Leads the checkout team.", "options": map[string]any{"effort": "high"}})
	if err != nil {
		t.Fatal(err)
	}
	checkout := a.session(out["sessionId"].(string))
	if checkout == nil || out["lead"] != lead.ID() || len(out["notApplied"].([]any)) != 0 {
		t.Fatalf("the first start: %v", out)
	}
	fresh()
	out, err = call(t, a.teamStart2Handler, ctx, map[string]any{"title": "Docks", "brief": "Keep the docks moving.", "lead": checkout.ID(), "options": map[string]any{"mode": "acceptEdits", "effort": "low"}})
	if err != nil {
		t.Fatal(err)
	}
	docks := a.session(out["sessionId"].(string))
	if docks == nil || out["lead"] != checkout.ID() || len(out["notApplied"].([]any)) != 0 {
		t.Fatalf("the start under the sub-lead: %v", out)
	}
	// The settings are the chat's own, as the window shows and saves them.
	if optionOf(checkout, "effort") != "high" || optionOf(docks, "mode") != "acceptEdits" || optionOf(docks, "effort") != "low" || optionOf(docks, "model") != "changed" {
		t.Fatalf("the settings: checkout %v, docks %v", checkout.Options(), docks.Options())
	}
	if string(a.settings.SessionOptions[docks.ID()]["mode"]) != `"acceptEdits"` || string(a.settings.SessionOptions[checkout.ID()]["effort"]) != `"high"` {
		t.Fatalf("the saved settings: %v %v", a.settings.SessionOptions[docks.ID()], a.settings.SessionOptions[checkout.ID()])
	}
	checkoutBus, docksBus := a.busSessionID(checkout.ID()), a.busSessionID(docks.ID())
	err = a.busHost().With(func(b *bus.Bus) error {
		if l, _, _ := b.LeadOf(context.Background(), docksBus); l != checkoutBus {
			t.Fatalf("Docks's lead is %q", l)
		}
		// The brief comes from Docks's own lead, in its group, and the line says who asked.
		asg, ok, _ := b.LastAssignment(context.Background(), checkoutBus, docksBus)
		if !ok || asg.Text != "Keep the docks moving." {
			t.Fatalf("Docks's brief: %+v", asg)
		}
		_, line, ok, _ := b.StartBrief(context.Background(), checkoutBus, docksBus)
		if !ok || !strings.Contains(line, "at the request of") || !strings.HasSuffix(line, ": Keep the docks moving.") {
			t.Fatalf("the start line: %q", line)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Later assignments change the task and not the remit.
	if _, err := call(t, a.teamAssignHandler, ctx, map[string]any{"member": checkout.ID(), "text": "Now look at refunds."}); err != nil {
		t.Fatal(err)
	}
	if _, err := call(t, a.teamAssignHandler, asChat(a, checkout), map[string]any{"member": docks.ID(), "text": "Unload the second ship."}); err != nil {
		t.Fatal(err)
	}
	// Docks closes; its settings are read from what was saved for it.
	a.sessionsLifecycle().Forget(docks.ID())
	if a.session(docks.ID()) != nil {
		t.Fatal("Docks is still open")
	}
	tree, err := describeTeam(t, a, ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if tree.SessionID != lead.ID() || tree.Provider != "codex" || tree.Model.Value != "changed" || tree.Model.Name != "Changed" || string(tree.Options["mode"]) != `"acceptEdits"` || len(tree.Members) != 1 {
		t.Fatalf("the root: %+v", tree)
	}
	c := tree.Members[0]
	if c.SessionID != checkout.ID() || c.Lead != lead.ID() || c.Title != "Checkout" || c.Remit != "Leads the checkout team." || c.RemitSource != "kept" || c.Model.Value != "changed" || c.Model.Name != "Changed" || string(c.Options["effort"]) != `"high"` || c.Workspace != lead.Cwd() || c.State == "" || len(c.Members) != 1 {
		t.Fatalf("Checkout: %+v", c)
	}
	if _, has := c.Options["model"]; has {
		t.Fatalf("the model is among Checkout's options: %v", c.Options)
	}
	d := c.Members[0]
	if d.SessionID != docks.ID() || d.Lead != checkout.ID() || d.Remit != "Keep the docks moving." || d.RemitSource != "kept" || d.State != "closed" || d.Provider != "codex" || d.Model.Value != "changed" || d.Model.Name != "Changed" ||
		string(d.Options["mode"]) != `"acceptEdits"` || string(d.Options["effort"]) != `"low"` || d.Workspace == "" {
		t.Fatalf("the closed member Docks: %+v", d)
	}
	var named bool
	for _, s := range d.Settings {
		named = named || (s.ID == "mode" && s.Category == "mode" && s.Name == "Mode" && s.ValueName == "Accept Edits")
	}
	if !named {
		t.Fatalf("Docks's settings have no names: %+v", d.Settings)
	}
	// The subtree under a member.
	sub, err := describeTeam(t, a, ctx, checkout.ID())
	if err != nil || sub.SessionID != checkout.ID() || len(sub.Members) != 1 || sub.Members[0].SessionID != docks.ID() {
		t.Fatalf("the subtree: %+v %v", sub, err)
	}
}

// A chat reads and builds only in its own tree: not under its own lead, not under another lead's team.
func TestDescribeAndStartRefuseAChatOutsideTheCallersTree(t *testing.T) {
	a, _, lead, fresh := duplicationApp(t, "codex:member", "codex:other", "codex:outside")
	ctx := asChat(a, lead)
	out, err := call(t, a.teamStartHandler, ctx, map[string]any{"title": "Member", "brief": "Work."})
	if err != nil {
		t.Fatal(err)
	}
	member := a.session(out["sessionId"].(string))
	fresh()
	other, err := a.NewSessionAt(lead.Cwd())
	if err != nil {
		t.Fatal(err)
	}
	fresh()
	if _, err := call(t, a.teamStartHandler, asChat(a, a.session(other.ID)), map[string]any{"title": "Outside", "brief": "Someone else's."}); err != nil {
		t.Fatal(err)
	}
	if _, err := describeTeam(t, a, ctx, other.ID); err == nil || !strings.Contains(err.Error(), "not in your team's tree") {
		t.Fatalf("describing another lead's team: %v", err)
	}
	if _, err := describeTeam(t, a, asChat(a, member), lead.ID()); err == nil || !strings.Contains(err.Error(), "not in your team's tree") {
		t.Fatalf("a member describing its own lead: %v", err)
	}
	if _, err := describeTeam(t, a, ctx, "codex:nobody"); err == nil {
		t.Fatal("an unknown chat was described")
	}
	fresh()
	if _, err := call(t, a.teamStart2Handler, ctx, map[string]any{"title": "X", "brief": "Y", "lead": other.ID}); err == nil || !strings.Contains(err.Error(), "not in your team's tree") {
		t.Fatalf("starting under another lead's chat: %v", err)
	}
	if _, err := call(t, a.teamStart2Handler, asChat(a, member), map[string]any{"title": "X", "brief": "Y", "lead": lead.ID()}); err == nil || !strings.Contains(err.Error(), "not in your team's tree") {
		t.Fatalf("a member starting under its own lead: %v", err)
	}
	// Nothing was started by the refusals.
	tree, _ := describeTeam(t, a, ctx, "")
	if len(tree.Members) != 1 || len(tree.Members[0].Members) != 0 {
		t.Fatalf("the tree after refusals: %+v", tree)
	}
}

// The permission rule: the caller's own mode on the same program and the program's default are accepted; a more
// permissive mode, or the caller's mode on another program, is refused with the modes that may be given.
func TestAPermissionModeIsNoMorePermissiveThanTheCallers(t *testing.T) {
	a, c, lead, fresh := duplicationApp(t, "codex:own", "codex:default", "claude:default")
	// Dinah gives the program's new chats Manual, so that is its default.
	a.settings.ProviderDefaults = map[string]map[string]json.RawMessage{"codex": {"mode": json.RawMessage(`"default"`)}}
	ctx := asChat(a, lead)
	start := func(input map[string]any) (map[string]any, error) {
		t.Helper()
		fresh()
		before := len(c.calls.Snapshot())
		out, err := call(t, a.teamStart2Handler, ctx, input)
		if err != nil {
			for _, call := range c.calls.Snapshot()[before:] {
				if call.method == "session/new" {
					t.Fatalf("a refused start made a chat: %v", err)
				}
			}
		}
		return out, err
	}
	out, err := start(map[string]any{"title": "Own", "brief": "B.", "options": map[string]any{"mode": "acceptEdits"}})
	if err != nil || optionOf(a.session(out["sessionId"].(string)), "mode") != "acceptEdits" {
		t.Fatalf("the caller's own mode: %v %v", out, err)
	}
	out, err = start(map[string]any{"title": "Default", "brief": "B.", "options": map[string]any{"mode": "default"}})
	if err != nil || optionOf(a.session(out["sessionId"].(string)), "mode") != "default" {
		t.Fatalf("the program's default: %v %v", out, err)
	}
	_, err = start(map[string]any{"title": "Bypass", "brief": "B.", "options": map[string]any{"mode": "bypassPermissions"}})
	if err == nil || !strings.Contains(err.Error(), "Bypass Permissions (bypassPermissions) cannot be given") || !strings.Contains(err.Error(), "you may give Accept Edits (acceptEdits) or Manual (default)") {
		t.Fatalf("a more permissive mode: %v", err)
	}
	// On another program only its default is accepted, not the caller's mode there.
	a.runtime.PutProviderLocked("claude", a.runtime.ProviderLocked("codex"))
	a.settings.ProviderCatalogs["claude"] = duplicationOptions()
	_, err = start(map[string]any{"title": "Cross", "brief": "B.", "provider": "claude", "options": map[string]any{"mode": "acceptEdits"}})
	if err == nil || !strings.Contains(err.Error(), "you may give Manual (default),") {
		t.Fatalf("the caller's mode on another program: %v", err)
	}
	out, err = start(map[string]any{"title": "Cross Default", "brief": "B.", "provider": "claude", "options": map[string]any{"mode": "default"}})
	if err != nil || out["sessionId"] != "claude:default" || optionOf(a.session("claude:default"), "mode") != "default" {
		t.Fatalf("the other program's default: %v %v", out, err)
	}
	// A setting the program does not offer, and a model among the settings, are refused before anything starts.
	if _, err := start(map[string]any{"title": "X", "brief": "B.", "options": map[string]any{"colour": "blue"}}); err == nil || !strings.Contains(err.Error(), "no setting") {
		t.Fatalf("an unknown setting: %v", err)
	}
	if _, err := start(map[string]any{"title": "X", "brief": "B.", "options": map[string]any{"model": "changed"}}); err == nil || !strings.Contains(err.Error(), "give the model as model") {
		t.Fatalf("a model among the settings: %v", err)
	}
}

// When Dinah gives a program's new chats no mode, the default is the one the new chat starts with: a mode equal to it is
// accepted, and any other mode the caller may not give is left out and reported, never set.
func TestAModeIsCheckedAgainstTheNewChatsOwnDefaultWhenDinahGivesNone(t *testing.T) {
	a, _, lead, fresh := duplicationApp(t, "codex:same", "codex:other")
	ctx := asChat(a, lead)
	fresh()
	out, err := call(t, a.teamStart2Handler, ctx, map[string]any{"title": "Same", "brief": "B.", "options": map[string]any{"mode": "default"}})
	if err != nil || len(out["notApplied"].([]any)) != 0 || optionOf(a.session(out["sessionId"].(string)), "mode") != "default" {
		t.Fatalf("the new chat's own default: %v %v", out, err)
	}
	fresh()
	out, err = call(t, a.teamStart2Handler, ctx, map[string]any{"title": "Other", "brief": "B.", "options": map[string]any{"mode": "bypassPermissions", "effort": "high"}})
	if err != nil {
		t.Fatal(err)
	}
	member := a.session(out["sessionId"].(string))
	left := out["notApplied"].([]any)
	if len(left) != 1 || left[0].(map[string]any)["id"] != "mode" || !strings.Contains(left[0].(map[string]any)["reason"].(string), "you may give Accept Edits (acceptEdits) or Manual (default)") {
		t.Fatalf("the settings left out: %v", left)
	}
	if optionOf(member, "mode") != "default" || optionOf(member, "effort") != "high" {
		t.Fatalf("the member's settings: %v", member.Options())
	}
}

// start@1 is unchanged: the same output, and no setting but the model is given; the brief is kept as the remit.
func TestStartOneIsUnchangedAndKeepsTheBriefAsTheRemit(t *testing.T) {
	a, c, lead, _ := duplicationApp(t, "codex:one")
	ctx := asChat(a, lead)
	before := len(c.calls.Snapshot())
	out, err := call(t, a.teamStartHandler, ctx, map[string]any{"title": "One", "brief": "Do one thing."})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 4 || out["sessionId"] != "codex:one" || out["title"] != "One" || out["busId"] == "" || out["group"] == "" {
		t.Fatalf("the start@1 output changed: %v", out)
	}
	for _, call := range c.calls.Snapshot()[before:] {
		if p, ok := call.params.(acp.SetConfigOptionParams); ok && p.ConfigID != "model" {
			t.Fatalf("start@1 gave a setting other than the model: %+v", p)
		}
	}
	if mode := optionOf(a.session("codex:one"), "mode"); mode != "default" {
		t.Fatalf("start@1 handed on the lead's mode: %s", mode)
	}
	// Its input is what it was: a lead, options or remit are start@2's.
	bundle, err := a.teamAppletBundle()
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range bundle.Manifest.Actions {
		if action.Ref != teamStartAction {
			continue
		}
		for _, extra := range []string{`"lead":"x"`, `"options":{}`, `"remit":"r"`} {
			if applet.Validate(action.InputSchema, `{"title":"T","brief":"B",`+extra+`}`) == nil {
				t.Fatalf("start@1 accepts %s", extra)
			}
		}
		if applet.Validate(action.InputSchema, `{"title":"T","brief":"B","workspace":"w","provider":"codex","model":"m"}`) != nil {
			t.Fatal("start@1 refuses its own input")
		}
	}
	tree, err := describeTeam(t, a, ctx, "")
	if err != nil || len(tree.Members) != 1 || tree.Members[0].Remit != "Do one thing." || tree.Members[0].RemitSource != "kept" {
		t.Fatalf("the remit of a start@1 member: %+v %v", tree, err)
	}
}

// A member started before remits were kept has its remit recovered from the brief its lead posted when it started it;
// a member made some other way, whose first message from its lead is an assignment, has none.
func TestAnOldMembersRemitIsRecoveredFromItsStartBrief(t *testing.T) {
	a, _, lead, fresh := duplicationApp(t, "codex:old", "codex:joined")
	ctx := asChat(a, lead)
	out, err := call(t, a.teamStartHandler, ctx, map[string]any{"title": "Old", "brief": "Keep the old harbour charts.\nAll of them."})
	if err != nil {
		t.Fatal(err)
	}
	old := out["sessionId"].(string)
	if _, err := call(t, a.teamAssignHandler, ctx, map[string]any{"member": old, "text": "Now the new ones."}); err != nil {
		t.Fatal(err)
	}
	fresh()
	joined, err := a.NewSessionAt(lead.Cwd())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetTeamLead(joined.ID, lead.ID()); err != nil {
		t.Fatal(err)
	}
	if _, err := call(t, a.teamAssignHandler, ctx, map[string]any{"member": joined.ID, "text": "Help with the charts."}); err != nil {
		t.Fatal(err)
	}
	// As a build before remits were kept left it.
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(configDir(), "bus.db"))+"?_pragma=busy_timeout(10000)")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE team_members SET remit = ''`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	tree, err := describeTeam(t, a, ctx, "")
	if err != nil || len(tree.Members) != 2 {
		t.Fatalf("the tree: %+v %v", tree, err)
	}
	for _, m := range tree.Members {
		switch m.SessionID {
		case old:
			if m.Remit != "Keep the old harbour charts.\nAll of them." || m.RemitSource != "start brief" {
				t.Fatalf("the old member's remit: %q from %q", m.Remit, m.RemitSource)
			}
		case joined.ID:
			if m.Remit != "" || m.RemitSource != "" {
				t.Fatalf("a member that was never started has the remit %q from %q", m.Remit, m.RemitSource)
			}
		}
	}
}

// The built-in skill is listed for the person and for agents, read only, and its instructions reach a chat that uses
// it; its procedure asks the person once before starting anyone.
func TestTheDuplicateATeamSkillIsListed(t *testing.T) {
	a, _ := providerTestApp(t)
	var found CatalogSkill
	for _, s := range a.SkillCatalog().Skills {
		if s.Name == "duplicate-a-team" {
			found = s
		}
	}
	if found.Scope != builtinSkillScope || found.Ref != nil || found.Revision == "" || !strings.HasPrefix(found.Description, "Duplicate a Team.") {
		t.Fatalf("the built-in skill: %+v", found)
	}
	pack, err := a.ReadSkill(found.ID)
	if err != nil || !strings.Contains(pack.Text, "ask_person") || !strings.Contains(pack.Text, "dinah.team/describe@1") || !strings.Contains(pack.Text, "dinah.team/start@2") {
		t.Fatalf("the skill's text: %v %q", err, pack.Text)
	}
	prompt, err := a.PrepareChatSkill(found.ID, found.Revision, t.TempDir(), "codex", "Copy the Checkout team for Keyboard.")
	if err != nil || !strings.Contains(prompt, "Copy the Checkout team for Keyboard.") || !strings.Contains(prompt, pack.Text) {
		t.Fatalf("the prepared prompt: %v", err)
	}
	// An agent finds it with dinah.skills/list@1 and reads it with dinah.skills/read@1.
	ws := t.TempDir()
	a.settings.WorkspaceRecords = []WorkspaceRecord{{ID: "w", Name: "W", ExecutionPath: ws}}
	agent := applet.WithInvocation(context.Background(), applet.Invocation{Workspace: ws, OwnerKind: "chat", OwnerID: "codex:any", ID: "skills-list"})
	bundle, err := a.skillsAppletBundle()
	if err != nil {
		t.Fatal(err)
	}
	listed, err := bundle.Handlers["dinah.skills/list@1"](agent, `{}`)
	if err != nil || !strings.Contains(listed.OutputJSON, found.ID) {
		t.Fatalf("the agent's list: %v %s", err, listed.OutputJSON)
	}
	read, err := bundle.Handlers["dinah.skills/read@1"](agent, `{"id":"`+found.ID+`"}`)
	if err != nil || !strings.Contains(read.OutputJSON, "ask_person") {
		t.Fatalf("the agent's read: %v %s", err, read.OutputJSON)
	}
	if _, err := a.StartSkill(found.ID, found.Revision, ws, "codex", "", "x"); err == nil {
		t.Fatal("a built-in skill was run in a new chat of its own")
	}
}
