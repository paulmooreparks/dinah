package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"sort"

	"studio/internal/bus"
	"studio/internal/workspaces"
)

type OrgChartChoice struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Workspace string `json:"workspace"`
	Provider  string `json:"provider"`
}
type OrgChartAction struct {
	Kind         string           `json:"kind"`
	Label        string           `json:"label"`
	Token        string           `json:"token"`
	Href         string           `json:"href"`
	Explanation  string           `json:"explanation"`
	Confirmation string           `json:"confirmation"`
	Chats        []OrgChartChoice `json:"chats"`
	Leads        []OrgChartChoice `json:"leads"`
	Groups       []OrgChartChoice `json:"groups"`
	Workspaces   []OrgChartChoice `json:"workspaces"`
	Providers    []OrgChartChoice `json:"providers"`
}
type OrgChartUnavailable struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
}
type OrgChartActionView struct {
	ChatID      string                `json:"chatId"`
	Title       string                `json:"title"`
	Actions     []OrgChartAction      `json:"actions"`
	Unavailable []OrgChartUnavailable `json:"unavailable"`
}
type OrgChartActionRequest struct {
	Kind        string   `json:"kind"`
	Token       string   `json:"token"`
	ChatID      string   `json:"chatId"`
	LeadID      string   `json:"leadId"`
	GroupID     string   `json:"groupId"`
	Title       string   `json:"title"`
	WorkspaceID string   `json:"workspaceId"`
	Provider    string   `json:"provider"`
	MemberIDs   []string `json:"memberIds"`
}
type OrgChartMemberOutcome struct {
	ChatID string `json:"chatId"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}
type OrgChartRecoveryTarget struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Href  string `json:"href"`
	Label string `json:"label"`
}
type OrgChartActionResult struct {
	Kind     string                   `json:"kind"`
	Status   string                   `json:"status"`
	ChatID   string                   `json:"chatId"`
	GroupID  string                   `json:"groupId"`
	Message  string                   `json:"message"`
	Members  []OrgChartMemberOutcome  `json:"members"`
	Recovery []OrgChartRecoveryTarget `json:"recovery"`
	Created  *SessionInfo             `json:"created"`
	Cleanup  *ChatCleanupResult       `json:"cleanup"`
}

type orgChartActionSnapshot struct {
	view    OrgChartView
	saved   Settings
	chats   map[string]OrgChartChat
	busIDs  map[string]string
	aliases map[string]string
	stamp   string
}

// The chart singleton and view state use the existing workbench address format.
func orgChartActionHref(kind, chat, group string) string {
	state := url.Values{}
	if kind != "" {
		state.Set("action", kind)
	}
	if chat != "" {
		state.Set("chat", chat)
	}
	if group != "" {
		state.Set("group", group)
	}
	query := url.Values{"open": {"org-chart"}, "view.org-chart": {state.Encode()}}
	return "?" + query.Encode()
}

func (a *App) orgChartActionSnapshot() (orgChartActionSnapshot, error) {
	x := orgChartActionSnapshot{chats: map[string]OrgChartChat{}, busIDs: map[string]string{}, aliases: map[string]string{}}
	var err error
	x.view, err = a.OrgChart()
	if err != nil {
		return x, err
	}
	a.mu.Lock()
	x.saved, err = a.settings.clone()
	a.mu.Unlock()
	if err != nil {
		return x, err
	}
	facts := []any{}
	conflicts := map[string]bool{}
	for _, chat := range x.view.Chats {
		m, err := logicalChatMembersFromSettings(x.saved, chat.ChatID)
		if err != nil || m.ChatID != chat.ChatID || m.Head != chat.Head || chat.Resolution != "resolved" {
			continue
		}
		held := len(m.UnresolvedAllocations) > 0
		for _, member := range m.Members {
			if _, reserved := x.saved.RemovedChats[member.SessionID]; reserved {
				held = true
			}
			if _, running := a.transferRunning.Load(member.SessionID); running {
				held = true
			}
			if s := a.session(member.SessionID); s != nil && s.Switching() {
				held = true
			}
		}
		if held {
			continue
		}
		id := x.saved.BusSessions[m.Head]
		x.chats[m.ChatID], x.busIDs[m.ChatID] = chat, id
		for _, member := range m.Members {
			place := x.saved.BusSessions[member.SessionID]
			if place == "" {
				continue
			}
			if previous := x.aliases[place]; previous != "" && previous != m.ChatID {
				conflicts[previous], conflicts[m.ChatID] = true, true
			}
			x.aliases[place] = m.ChatID
		}
		facts = append(facts, []any{m, chat.Workspace, chat.Provider})
	}
	for id := range conflicts {
		delete(x.chats, id)
		delete(x.busIDs, id)
	}
	for place, id := range x.aliases {
		if conflicts[id] {
			delete(x.aliases, place)
		}
	}
	// Display text and transient execution state do not invalidate form tokens.
	for _, team := range x.view.Teams {
		facts = append(facts, []any{team.ID, team.LeadID, team.MemberIDs, team.Resolution})
	}
	for _, group := range x.view.Groups {
		facts = append(facts, []any{group.ID, group.OwnerID, group.MemberIDs, group.Resolution})
	}
	for _, workspace := range x.saved.WorkspaceRecords {
		facts = append(facts, []string{workspace.ID, workspaces.ExecutionPath(workspace)})
	}
	facts = append(facts, x.saved.RemovedChats, product.Providers)
	data, err := json.Marshal(facts)
	if err == nil {
		x.stamp = fmt.Sprintf("%x", sha256.Sum256(data))
	}
	return x, err
}

func orgChartChoice(chat OrgChartChat) OrgChartChoice {
	return OrgChartChoice{ID: chat.ChatID, Title: chat.Title, Workspace: chat.Workspace, Provider: chat.Provider}
}

func (a *App) OrgChartActions(chatID string) (OrgChartActionView, error) {
	x, err := a.orgChartActionSnapshot()
	if err != nil {
		return OrgChartActionView{}, err
	}
	return a.orgChartActions(x, chatID)
}

func (a *App) orgChartActions(x orgChartActionSnapshot, chatID string) (OrgChartActionView, error) {
	out := OrgChartActionView{ChatID: chatID, Actions: []OrgChartAction{}, Unavailable: []OrgChartUnavailable{}}
	kinds := []string{"assign_team", "make_lead", "create_team", "create_group", "start_chat", "delete_chat"}
	if chatID != "" {
		chat, ok := x.chats[chatID]
		if !ok {
			for _, kind := range kinds {
				out.Unavailable = append(out.Unavailable, OrgChartUnavailable{kind, "This chat is unavailable, held or needs ownership repair. Refresh the chart."})
			}
			return out, nil
		}
		out.Title = chat.Title
	}
	choices := []OrgChartChoice{}
	for _, chat := range x.chats {
		choices = append(choices, orgChartChoice(chat))
	}
	sort.Slice(choices, func(i, j int) bool { return choices[i].ID < choices[j].ID })
	groupChoices := []OrgChartChoice{}
	for _, choice := range choices {
		if filepath.IsAbs(choice.Workspace) {
			groupChoices = append(groupChoices, choice)
		}
	}
	add := func(kind, label string) *OrgChartAction {
		data, _ := json.Marshal([]string{x.stamp, chatID, kind})
		out.Actions = append(out.Actions, OrgChartAction{Kind: kind, Label: label, Token: fmt.Sprintf("%x", sha256.Sum256(data)), Href: orgChartActionHref(kind, chatID, ""), Chats: []OrgChartChoice{}, Leads: []OrgChartChoice{}, Groups: []OrgChartChoice{}, Workspaces: []OrgChartChoice{}, Providers: []OrgChartChoice{}})
		return &out.Actions[len(out.Actions)-1]
	}
	if chatID != "" {
		var leads, groups []OrgChartChoice
		err := a.busHost().With(func(b *bus.Bus) error {
			for _, team := range x.view.Teams {
				if chat, ok := x.chats[team.LeadID]; ok && team.Resolution == "resolved" && b.CheckTeamLead(x.busIDs[chatID], x.busIDs[chat.ChatID]) == nil {
					leads = append(leads, orgChartChoice(chat))
				}
			}
			for _, group := range x.view.Groups {
				if group.OwnerID == "" && group.Resolution == "resolved" && b.CheckGroupTeam(group.ID, x.busIDs[chatID]) == nil {
					groups = append(groups, OrgChartChoice{ID: group.ID, Title: group.Title})
				}
			}
			return nil
		})
		if err != nil {
			return out, err
		}
		if len(leads) > 0 {
			add("assign_team", "Assign to a Team").Chats = leads
		} else {
			out.Unavailable = append(out.Unavailable, OrgChartUnavailable{"assign_team", "No eligible team is available."})
		}
		if len(groups) > 0 {
			v := add("make_lead", "Make Lead")
			v.Groups = groups
			v.Explanation = "Turn the selected ordinary group into a team led by this chat."
		} else {
			out.Unavailable = append(out.Unavailable, OrgChartUnavailable{"make_lead", "No eligible ordinary group contains this chat. This action does not replace an existing team's lead."})
		}
	}
	teamChoices := []OrgChartChoice{}
	for _, chat := range groupChoices {
		leads := false
		for _, team := range x.view.Teams {
			if team.LeadID == chat.ID {
				leads = true
			}
		}
		// Include a lead-only channel even when it has no reporting members yet.
		for _, group := range x.view.Groups {
			if group.OwnerID == chat.ID {
				leads = true
			}
		}
		if !leads {
			teamChoices = append(teamChoices, chat)
		}
	}
	if len(groupChoices) > 1 && len(teamChoices) > 0 && (chatID == "" || slices.ContainsFunc(teamChoices, func(c OrgChartChoice) bool { return c.ID == chatID })) {
		v := add("create_team", "Create Team")
		v.Chats = groupChoices
		v.Leads = teamChoices
	} else {
		out.Unavailable = append(out.Unavailable, OrgChartUnavailable{"create_team", "Choose a lead without a team channel and at least one other chat."})
	}
	if len(groupChoices) > 0 {
		add("create_group", "Create Group").Chats = groupChoices
	} else {
		out.Unavailable = append(out.Unavailable, OrgChartUnavailable{"create_group", "No eligible local chat is available."})
	}
	if len(x.saved.WorkspaceRecords) > 0 && len(product.Providers) > 0 {
		v := add("start_chat", "Start a Chat")
		for _, w := range x.saved.WorkspaceRecords {
			if workspaces.ExecutionPath(w) != "" {
				v.Workspaces = append(v.Workspaces, OrgChartChoice{ID: w.ID, Title: w.Name})
			}
		}
		for _, p := range product.Providers {
			v.Providers = append(v.Providers, OrgChartChoice{ID: p, Title: providerName(p)})
		}
		if len(v.Workspaces) == 0 {
			out.Actions = out.Actions[:len(out.Actions)-1]
		}
	}
	if chatID != "" {
		listing, err := a.ListChatCleanup()
		if err != nil {
			return out, err
		}
		eligible, reason := false, "This chat cannot be deleted from the local inventory."
		for _, row := range listing.Rows {
			if row.SessionID == chatID || slices.Contains(row.SessionIDs, chatID) {
				eligible, reason = row.Eligible, row.Refusal
				break
			}
		}
		if eligible {
			v := add("delete_chat", "Delete Chat…")
			v.Confirmation = "Move this chat to the Recycle Bin? It will disappear from Dinah now and can be restored for 30 days. Its agent program's history is kept until final deletion. Linked documents and document drafts are unchanged."
		} else {
			out.Unavailable = append(out.Unavailable, OrgChartUnavailable{"delete_chat", reason})
		}
	}
	return out, nil
}
