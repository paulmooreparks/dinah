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
	manifest := applet.Manifest{ID: "dinah.team", Version: "1.0.0", APIVersion: 1, Title: "Team", Description: "Start, assign and retire the chats of the calling chat's own team. A chat can act only on the chats it leads.", Actions: []applet.Action{
		{Ref: teamStartAction, Title: "Start a Team Member", Description: "Start a new chat as a member of the calling chat's team, and give it its brief as its first message in the team's group chat. It is in the calling chat's workspace, with the calling chat's provider and model, unless told otherwise. It has its provider's default for every setting, a permission mode included: authority is not handed on. Returns the new chat's session ID.", InputSchema: startInput, OutputSchema: startOutput, Capabilities: []string{"team.manage"}, Replay: "once"},
		{Ref: teamAssignAction, Title: "Assign Work to a Team Member", Description: "Post a message in the team's group chat that mentions one of the calling chat's own team members, which wakes it, and keep it as the member's current task. The member is given by its session ID, which dinah.team/list@1 returns.", InputSchema: assignInput, OutputSchema: assignOutput, Capabilities: []string{"team.manage"}, Replay: "once"},
		{Ref: teamRetireAction, Title: "Retire a Team Member", Description: "End a team member's session and keep its history. It stays listed in the team, marked retired, is not woken by the team's messages, and can be reopened from the window.", InputSchema: retireInput, OutputSchema: retireOutput, Capabilities: []string{"team.manage"}, Replay: "once"},
		{Ref: teamListAction, Title: "List the Team", Description: "List the calling chat's team members with their session IDs, states and current tasks, nested teams under their leads.", InputSchema: listInput, OutputSchema: listOutput, Capabilities: []string{"chat.read"}, Replay: "pure"},
	}}
	handlers := map[string]applet.Handler{
		teamStartAction:  a.teamStartHandler,
		teamAssignAction: a.teamAssignHandler,
		teamRetireAction: a.teamRetireHandler,
		teamListAction:   a.teamListHandler,
	}
	return applet.Bundle{Manifest: manifest, Handlers: handlers, Sources: map[string][]byte{"applet_team.go": teamAppletSource, "team_views.go": teamViewsSource}}, nil
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
	a.teamsActive.Store(true)
	var in struct {
		Title, Brief, Workspace, Provider, Model string
	}
	if err := json.Unmarshal([]byte(input), &in); err != nil {
		return applet.Result{}, err
	}
	in.Title, in.Brief = strings.TrimSpace(in.Title), strings.TrimSpace(in.Brief)
	if in.Title == "" || in.Brief == "" {
		return applet.Result{}, errors.New("a team member needs a title and a brief")
	}
	caller, leadBus, err := a.teamCaller(ctx)
	if err != nil {
		return applet.Result{}, err
	}
	workspace := caller.Cwd()
	if in.Workspace != "" {
		workspace = filepath.Clean(in.Workspace)
		if !filepath.IsAbs(workspace) || !a.knownWorkspace(workspace) {
			return applet.Result{}, errors.New("the workspace must be the folder of one of the workspaces " + product.Name + " has open")
		}
	}
	provider := strings.TrimSpace(in.Provider)
	if provider == "" {
		provider = caller.Provider()
	}
	info, err := a.NewSessionWithProviderAt(workspace, provider)
	if err != nil {
		return applet.Result{}, fmt.Errorf("the team member could not be started: %w", err)
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
	memberBus := a.knownBusID(info.ID)
	if memberBus == "" {
		return applet.Result{}, errors.New("the new team member is not on the bus")
	}
	var group string
	err = a.busHost().With(func(b *bus.Bus) error {
		var err error
		if group, err = b.SetLead(ctx, memberBus, leadBus); err != nil {
			return err
		}
		lead, err := b.Session(ctx, leadBus)
		if err != nil {
			return err
		}
		if err := b.TeamStartLine(ctx, leadBus, group, startLine(in.Title, provider, model, in.Brief)); err != nil {
			return err
		}
		_, err = b.SendMentioning(ctx, bus.Sender{Session: &lead}, "#"+group, in.Brief, nil, 0, []string{memberBus})
		return err
	})
	if err != nil {
		return applet.Result{}, fmt.Errorf("the team member was started, but it could not be made one of your team or given its brief: %w", err)
	}
	a.teamsChanged()
	return builtinJSON(map[string]any{"sessionId": info.ID, "busId": memberBus, "title": in.Title, "group": group})
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
