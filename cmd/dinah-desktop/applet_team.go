package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"studio/internal/applet"
	"studio/internal/bus"
	"studio/internal/layout"
)

//go:embed applet_team.go
var teamAppletSource []byte

// The lead's applet actions (docs/plans/teams.md): start a team member, assign it work and retire it, each scoped to the
// calling chat's own direct members, and read the team. They hold the authority a lead has and no more. A chat that leads
// no one has nothing to assign or retire; starting a member makes it one's own. A new member follows its lead's provider
// and model unless told otherwise, and has its provider's defaults for every setting, a permission mode included: the
// authority a lead holds is not handed on, and the member's own requests for permission reach the person through
// Notifications as any chat's do.

// dinah.team/describe@1 and dinah.team/start@2 (team_duplication.go) read a team anywhere in the calling chat's own
// tree and build under any chat in it, for duplicating a team (docs/design/team-duplication.md).

const (
	teamStartAction  = "dinah.team/start@1"
	teamAssignAction = "dinah.team/assign@1"
	teamRetireAction = "dinah.team/retire@1"
	teamListAction   = "dinah.team/list@1"
)

func (a *App) teamAppletBundle() (applet.Bundle, error) {
	startInput := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["title","brief"],"properties":{"title":{"type":"string","minLength":1,"maxLength":200},"brief":{"type":"string","minLength":1,"maxLength":32768},"workspace":{"type":"string","maxLength":4096},"provider":{"type":"string","maxLength":64},"model":{"type":"string","maxLength":256}}}`)
	startOutput := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["sessionId","busId","title","group"],"properties":{"sessionId":{"type":"string"},"busId":{"type":"string"},"title":{"type":"string"},"group":{"type":"string"}}}`)
	assignInput := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["member","text"],"properties":{"member":{"type":"string","minLength":1,"maxLength":512},"text":{"type":"string","minLength":1,"maxLength":32768}}}`)
	assignOutput := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["message","group"],"properties":{"message":{"type":"integer"},"group":{"type":"string"}}}`)
	retireInput := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["member"],"properties":{"member":{"type":"string","minLength":1,"maxLength":512}}}`)
	retireOutput := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["member","retired"],"properties":{"member":{"type":"string"},"retired":{"type":"boolean"}}}`)
	listInput := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{}}`)
	listOutput := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["group","members"],"properties":{"group":{"type":"string"},"members":{"type":"array","items":{"type":"object"}}}}`)
	optionValue := `{"type":["string","boolean","number"],"maxLength":1024}`
	start2Input := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["title","brief"],"properties":{"title":{"type":"string","minLength":1,"maxLength":200},"brief":{"type":"string","minLength":1,"maxLength":32768},"remit":{"type":"string","maxLength":32768},"lead":{"type":"string","maxLength":512},"workspace":{"type":"string","maxLength":4096},"provider":{"type":"string","maxLength":64},"model":{"type":"string","maxLength":256},"options":{"type":"object","maxProperties":64,"additionalProperties":` + optionValue + `}}}`)
	start2Output := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["sessionId","busId","title","group","lead","notApplied"],"properties":{"sessionId":{"type":"string"},"busId":{"type":"string"},"title":{"type":"string"},"group":{"type":"string"},"lead":{"type":"string"},"notApplied":{"type":"array","items":{"type":"object","additionalProperties":false,"required":["id","reason"],"properties":{"id":{"type":"string"},"reason":{"type":"string"}}}}}}`)
	describeInput := json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"member":{"type":"string","maxLength":512}}}`)
	describeOutput := json.RawMessage(`{"type":"object","additionalProperties":false,"required":["team"],"properties":{"team":{"type":"object"}}}`)
	manifest := applet.Manifest{ID: "dinah.team", Version: "1.1.0", APIVersion: 1, Title: "Team", Description: "Start, assign and retire the chats of the calling chat's own team, and read and build teams in its own tree. A chat can act only on the chats it leads, directly or through its members.", Actions: []applet.Action{
		{Ref: teamStartAction, Title: "Start a Team Member", Description: "Start a new chat as a member of the calling chat's team, and give it its brief as its first message in the team's group chat. It is in the calling chat's workspace, with the calling chat's provider and model, unless told otherwise. It has its provider's default for every setting, a permission mode included: authority is not handed on. Returns the new chat's session ID.", InputSchema: startInput, OutputSchema: startOutput, Capabilities: []string{"team.manage"}, Replay: "once"},
		{Ref: teamAssignAction, Title: "Assign Work to a Team Member", Description: "Post a message in the team's group chat that mentions one of the calling chat's own team members, which wakes it, and keep it as the member's current task. The member is given by its session ID, which dinah.team/list@1 returns.", InputSchema: assignInput, OutputSchema: assignOutput, Capabilities: []string{"team.manage"}, Replay: "once"},
		{Ref: teamRetireAction, Title: "Retire a Team Member", Description: "End a team member's session and keep its history. It stays listed in the team, marked retired, is not woken by the team's messages, and can be reopened from the window.", InputSchema: retireInput, OutputSchema: retireOutput, Capabilities: []string{"team.manage"}, Replay: "once"},
		{Ref: teamListAction, Title: "List the Team", Description: "List the calling chat's team members with their session IDs, states and current tasks, nested teams under their leads.", InputSchema: listInput, OutputSchema: listOutput, Capabilities: []string{"chat.read"}, Replay: "pure"},
		{Ref: teamDescribeAction, Title: "Describe a Team", Description: "Return the calling chat's tree as data, or the subtree under one chat in it given by its session ID as member. Each chat has its sessionId, busId, title, remit (the brief it was started with, kept apart from its current task) and remitSource, provider, model (value and name), options (its settings other than the model, by ID, as dinah.team/start@2 takes them), settings (the same with names and categories; category mode is a permission mode), workspace (its folder), workspaceName, lead (its lead's session ID), state, stateWhy, retired and members. A chat outside the caller's tree is refused. It reads and changes nothing, and works for closed chats.", InputSchema: describeInput, OutputSchema: describeOutput, Capabilities: []string{"chat.read"}, Replay: "pure"},
		{Ref: teamStart2Action, Title: "Start a Team Member Under a Chat", Description: "Start a new chat as dinah.team/start@1 does, with three additions. lead is the session ID of a chat in the calling chat's tree to start it under, the calling chat when left out; its brief is then posted by that lead in that lead's group. remit is kept as what the member is for, the brief when left out; later assignments do not change it. options are settings for the new chat by ID, as dinah.team/describe@1 returns them; a model is given as model. A permission mode (a setting of category mode) is accepted only when it is the calling chat's own mode on the same program or the program's default, since no order of the programs' modes is documented; a refusal says which modes may be given. A setting that could not be given once the chat was made is listed in notApplied with why. Returns the new chat's session ID.", InputSchema: start2Input, OutputSchema: start2Output, Capabilities: []string{"team.manage"}, Replay: "once"},
	}}
	handlers := map[string]applet.Handler{
		teamStartAction:    a.teamStartHandler,
		teamAssignAction:   a.teamAssignHandler,
		teamRetireAction:   a.teamRetireHandler,
		teamListAction:     a.teamListHandler,
		teamDescribeAction: a.teamDescribeHandler,
		teamStart2Action:   a.teamStart2Handler,
	}
	return applet.Bundle{Manifest: manifest, Handlers: handlers, Sources: map[string][]byte{"applet_team.go": teamAppletSource, "team_views.go": teamViewsSource, "team_duplication.go": teamDuplicationSource}}, nil
}

//go:embed team_views.go
var teamViewsSource []byte

// teamCaller is the chat whose agent invoked the action, and its session on the bus.
func (a *App) teamCaller(ctx context.Context) (*Session, string, error) {
	inv, ok := applet.InvocationFromContext(ctx)
	if !ok || inv.OwnerKind != "chat" || inv.OwnerID == "" {
		return nil, "", errors.New("only a chat's agent can act on its team")
	}
	caller := a.session(inv.OwnerID)
	if caller == nil {
		return nil, "", errors.New("the calling chat is not open")
	}
	leadBus := a.knownBusID(caller.ID())
	if leadBus == "" {
		return nil, "", errors.New("the calling chat is not on the bus")
	}
	return caller, leadBus, nil
}

// ownMember checks that a chat, by its ID, is a direct member of the caller's team, not retired where it matters, and
// returns its bus session.
func (a *App) ownMember(ctx context.Context, b *bus.Bus, leadBus, member string, allowRetired bool) (string, error) {
	memberChat := strings.TrimSpace(member)
	a.mu.Lock()
	memberBus := a.settings.BusSessions[memberChat]
	a.mu.Unlock()
	if memberBus == "" {
		return "", fmt.Errorf("there is no chat %s on your team; dinah.team/list@1 lists them with their session IDs", member)
	}
	lead, retired, err := b.LeadOf(ctx, memberBus)
	if err != nil {
		return "", err
	}
	if lead != leadBus {
		return "", fmt.Errorf("%s is not one of your team members, so you cannot act on it; dinah.team/list@1 lists them", member)
	}
	if retired && !allowRetired {
		return "", fmt.Errorf("%s is retired; reopen it from the window before giving it work", member)
	}
	return memberBus, nil
}

func (a *App) teamStartHandler(ctx context.Context, input string) (applet.Result, error) {
	var in struct {
		Title, Brief, Workspace, Provider, Model string
	}
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return applet.Result{}, err
	}
	out, err := a.teamStart(ctx, teamStartRequest{Title: in.Title, Brief: in.Brief, Workspace: in.Workspace, Provider: in.Provider, Model: in.Model})
	if err != nil {
		return applet.Result{}, err
	}
	return builtinJSON(map[string]any{"sessionId": out.SessionID, "busId": out.BusID, "title": out.Title, "group": out.Group})
}

// teamStart starts a member for start@1 and start@2. Its remit is kept as it joins the team: the request's, or its
// brief.
func (a *App) teamStart(ctx context.Context, in teamStartRequest) (teamStarted, error) {
	a.teamsActive.Store(true)
	in.Title, in.Brief, in.Remit = strings.TrimSpace(in.Title), strings.TrimSpace(in.Brief), strings.TrimSpace(in.Remit)
	if in.Title == "" || in.Brief == "" {
		return teamStarted{}, errors.New("a team member needs a title and a brief")
	}
	if in.Remit == "" {
		in.Remit = in.Brief
	}
	caller, callerBus, err := a.teamCaller(ctx)
	if err != nil {
		return teamStarted{}, err
	}
	leadChat, leadBus, err := a.teamLeadFor(ctx, caller, callerBus, in.Lead)
	if err != nil {
		return teamStarted{}, err
	}
	workspace := caller.Cwd()
	if in.Workspace != "" {
		workspace = filepath.Clean(in.Workspace)
		if !filepath.IsAbs(workspace) || !a.knownWorkspace(workspace) {
			return teamStarted{}, errors.New("the workspace must be the folder of one of the workspaces " + product.Name + " has open")
		}
	}
	provider := strings.TrimSpace(in.Provider)
	if provider == "" {
		provider = caller.Provider()
	}
	if len(in.Options) > 0 {
		if err := a.checkTeamOptions(caller, normalizedProvider(provider), in.Options); err != nil {
			return teamStarted{}, err
		}
	}
	info, err := a.NewSessionWithProviderAt(workspace, provider)
	if err != nil {
		return teamStarted{}, fmt.Errorf("the team member could not be started: %w", err)
	}
	if _, err := a.RenameSession(info.ID, in.Title); err != nil {
		logf("bus", "warning", "the new team member %s could not be given its title: %v", info.ID, err)
	}
	model := strings.TrimSpace(in.Model)
	if model == "" {
		for _, o := range caller.Options() {
			if o.Category == "model" {
				model = judgeOptionString(o.CurrentValue)
			}
		}
	}
	if model != "" {
		a.followModel(info.ID, model)
	}
	var notApplied []map[string]string
	if len(in.Options) > 0 {
		notApplied = a.applyTeamOptions(info.ID, caller, normalizedProvider(provider), in.Options)
	}
	memberBus := a.knownBusID(info.ID)
	if memberBus == "" {
		return teamStarted{}, errors.New("the new team member is not on the bus")
	}
	// A member started under another chat of the caller's tree is that chat's member: its brief comes from that
	// lead in that lead's group, so its task, wake and relays are a member's like any other, and the line in the group
	// says who asked.
	requester := ""
	var group string
	err = a.busHost().With(func(b *bus.Bus) error {
		var err error
		if group, err = b.SetLeadWithRemit(ctx, memberBus, leadBus, in.Remit); err != nil {
			return err
		}
		lead, err := b.Session(ctx, leadBus)
		if err != nil {
			return err
		}
		if leadBus != callerBus {
			requester = "a lead above it"
			if s, err := b.Session(ctx, callerBus); err == nil && strings.TrimSpace(s.Title) != "" {
				requester = bus.DisplayTitle(s.Title, s.WorkspaceLabel)
			}
		}
		if err := b.TeamStartLine(ctx, leadBus, group, startLineFor(in.Title, provider, model, in.Brief, requester)); err != nil {
			return err
		}
		_, err = b.SendMentioning(ctx, bus.Sender{Session: &lead}, "#"+group, in.Brief, nil, 0, []string{memberBus})
		return err
	})
	if err != nil {
		return teamStarted{}, fmt.Errorf("the team member was started, but it could not be made one of your team or given its brief: %w", err)
	}
	a.teamsChanged()
	return teamStarted{SessionID: info.ID, BusID: memberBus, Title: in.Title, Group: group, Lead: leadChat, NotApplied: notApplied}, nil
}

// followModel gives a new chat the model its lead has, when its provider offers the same.
func (a *App) followModel(chat, model string) {
	s := a.session(chat)
	if s == nil {
		return
	}
	for _, o := range s.Options() {
		if o.Category != "model" {
			continue
		}
		for _, v := range judgeOptionValues(o) {
			if v.Value == model {
				value, _ := json.Marshal(model)
				if _, err := a.SetSessionOption(chat, o.ID, value); err != nil {
					logf("bus", "warning", "the new team member %s could not be given the model %s: %v", chat, model, err)
				}
				return
			}
		}
	}
}

// knownWorkspace says a folder is the folder of a workspace Dinah has.
func (a *App) knownWorkspace(path string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, w := range a.settings.WorkspaceRecords {
		if layout.SamePath(w.ExecutionPath, path) {
			return true
		}
	}
	return false
}

func (a *App) teamAssignHandler(ctx context.Context, input string) (applet.Result, error) {
	var in struct{ Member, Text string }
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return applet.Result{}, err
	}
	if strings.TrimSpace(in.Text) == "" {
		return applet.Result{}, errors.New("the assignment needs words")
	}
	_, leadBus, err := a.teamCaller(ctx)
	if err != nil {
		return applet.Result{}, err
	}
	var id int64
	var group string
	err = a.busHost().With(func(b *bus.Bus) error {
		memberBus, err := a.ownMember(ctx, b, leadBus, in.Member, false)
		if err != nil {
			return err
		}
		if group, err = b.TeamGroup(ctx, leadBus); err != nil {
			return err
		}
		lead, err := b.Session(ctx, leadBus)
		if err != nil {
			return err
		}
		m, err := b.SendMentioning(ctx, bus.Sender{Session: &lead}, "#"+group, in.Text, nil, 0, []string{memberBus})
		id = m.ID
		return err
	})
	if err != nil {
		return applet.Result{}, err
	}
	a.teamsChanged()
	return builtinJSON(map[string]any{"message": id, "group": group})
}

func (a *App) teamRetireHandler(ctx context.Context, input string) (applet.Result, error) {
	var in struct{ Member string }
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return applet.Result{}, err
	}
	_, leadBus, err := a.teamCaller(ctx)
	if err != nil {
		return applet.Result{}, err
	}
	var memberBus string
	err = a.busHost().With(func(b *bus.Bus) error {
		var err error
		if memberBus, err = a.ownMember(ctx, b, leadBus, in.Member, true); err != nil {
			return err
		}
		return b.SetRetired(ctx, memberBus, true)
	})
	if err != nil {
		return applet.Result{}, err
	}
	// The session ends and its history stays: a chat that is open is let go of, which a provider cannot delete.
	if chat := a.chatOfBus(memberBus); chat != "" {
		if a.session(chat) != nil {
			a.sessionsLifecycle().Forget(chat)
		}
	}
	a.teamsChanged()
	return builtinJSON(map[string]any{"member": in.Member, "retired": true})
}

func (a *App) teamListHandler(ctx context.Context, input string) (applet.Result, error) {
	caller, _, err := a.teamCaller(ctx)
	if err != nil {
		return applet.Result{}, err
	}
	view, err := a.Team(caller.ID())
	if err != nil {
		return applet.Result{}, err
	}
	var members []map[string]any
	var add func(list []TeamMemberView)
	add = func(list []TeamMemberView) {
		for _, m := range list {
			members = append(members, map[string]any{"sessionId": m.Chat, "title": m.Title, "state": m.State, "why": m.StateWhy, "task": m.Task, "retired": m.Retired, "led": len(m.Members) > 0})
			add(m.Members)
		}
	}
	add(view.Members)
	if members == nil {
		members = []map[string]any{}
	}
	return builtinJSON(map[string]any{"group": view.Group, "members": members})
}
