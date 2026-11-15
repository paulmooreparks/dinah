package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"time"

	"studio/internal/bus"
	"studio/internal/layout"
	"studio/internal/sessionhost"
	"studio/internal/workspaces"
)

type OrgChartView struct {
	Generation  string          `json:"generation"`
	GeneratedAt string          `json:"generatedAt"`
	Chats       []OrgChartChat  `json:"chats"`
	Teams       []OrgChartTeam  `json:"teams"`
	Groups      []OrgChartGroup `json:"groups"`
}
type OrgChartChat struct {
	ChatID        string    `json:"chatId"`
	Head          string    `json:"head"`
	Title         string    `json:"title"`
	Workspace     string    `json:"workspace"`
	WorkspaceName string    `json:"workspaceName"`
	Provider      string    `json:"provider"`
	ProviderName  string    `json:"providerName"`
	State         ChatState `json:"state"`
	Resolution    string    `json:"resolution"`
	Reason        string    `json:"reason"`
}
type OrgChartTeam struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	LeadID     string   `json:"leadId"`
	MemberIDs  []string `json:"memberIds"`
	Resolution string   `json:"resolution"`
	Reason     string   `json:"reason"`
}
type OrgChartGroup struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	OwnerID    string   `json:"ownerId"`
	MemberIDs  []string `json:"memberIds"`
	Resolution string   `json:"resolution"`
	Reason     string   `json:"reason"`
}

// OrgChart reads one local ownership snapshot. Its bus and runtime observations
// confer no authority and never discover history or open a closed conversation.
func (a *App) OrgChart() (OrgChartView, error) {
	view := OrgChartView{Chats: []OrgChartChat{}, Teams: []OrgChartTeam{}, Groups: []OrgChartGroup{}}
	a.mu.Lock()
	if a.startupErr != nil {
		err := a.startupErr
		a.mu.Unlock()
		return view, err
	}
	// Stamp the original under the same lock. JSON cloning normalizes empty
	// optional maps to nil; that is not a concurrent ownership change.
	stamp := orgChartOwnershipStamp(a.settings)
	saved, err := a.settings.clone()
	a.mu.Unlock()
	if err != nil {
		return view, err
	}
	_, headers := sessionhost.ConversationLinks(configDir())
	aliases, excluded := map[string]string{}, map[string]bool{}
	byChat := map[string]int{}
	for _, reference := range logicalChatReferencesFromSettings(saved) {
		members, resolveErr := logicalChatMembersFromSettings(saved, reference)
		removed := false
		if saved.logical != nil {
			phase := saved.logical.Chats[members.ChatID].Lifecycle
			removed = phase != "" && phase != "active"
		}
		for _, member := range members.Members {
			if _, reserved := saved.RemovedChats[member.SessionID]; reserved {
				removed = true
			}
		}
		if _, reserved := saved.RemovedChats[reference]; reserved {
			removed = true
		}
		if removed {
			for _, member := range members.Members {
				excluded[saved.BusSessions[member.SessionID]] = true
			}
			continue
		}
		id := members.ChatID
		if id == "" {
			id = reference
		}
		if _, exists := byChat[id]; exists {
			continue
		}
		head := members.Head
		chat := OrgChartChat{ChatID: id, Head: head, Title: saved.SessionTitles[head], Provider: saved.SessionProviders[head], State: ChatState{Life: lifeClosed}, Resolution: "resolved"}
		if metadata := headers[head]; metadata != nil {
			chat.Workspace = metadata.Workspace
			if chat.Provider == "" {
				chat.Provider = metadata.Provider
			}
		}
		if saved.logical != nil {
			metadata := saved.logical.Chats[id]
			if metadata.Title != "" {
				chat.Title = metadata.Title
			}
			if metadata.Workspace != "" {
				chat.Workspace = metadata.Workspace
			}
		}
		for _, member := range members.Members {
			if member.SessionID == head && chat.Provider == "" {
				chat.Provider = member.Provider
			}
		}
		if chat.Title == "" {
			chat.Title = saved.SessionTitles[reference]
		}
		if chat.Title == "" {
			chat.Title = "Untitled chat"
		}
		for _, workspace := range saved.WorkspaceRecords {
			if chat.Workspace != "" && layout.SamePath(workspaces.ExecutionPath(workspace), chat.Workspace) {
				chat.WorkspaceName = workspace.Name
				break
			}
		}
		chat.ProviderName = providerName(chat.Provider)
		if resolveErr != nil {
			chat.Head, chat.Resolution, chat.Reason = "", "repair", resolveErr.Error()
		} else {
			for _, member := range members.Members {
				aliases[member.SessionID] = id
			}
		}
		byChat[id] = len(view.Chats)
		view.Chats = append(view.Chats, chat)
	}
	// A recipient shared by different logical owners cannot supply an edge.
	busAliases, conflicts := map[string]string{}, map[string]bool{}
	for physical, recipient := range saved.BusSessions {
		if recipient == "" {
			continue
		}
		if _, reserved := saved.RemovedChats[physical]; reserved {
			excluded[recipient] = true
			continue
		}
		if canonical := aliases[physical]; canonical != "" {
			if previous := busAliases[recipient]; previous != "" && previous != canonical {
				conflicts[recipient] = true
			}
			busAliases[recipient] = canonical
		}
	}
	for recipient := range conflicts {
		delete(busAliases, recipient)
	}
	ctx := context.Background()
	err = a.busHost().With(func(b *bus.Bus) error {
		rooms, err := b.Rooms(ctx)
		if err != nil {
			return err
		}
		for _, room := range rooms {
			group := OrgChartGroup{ID: room.Name, Title: room.Title, MemberIDs: []string{}, Resolution: "resolved"}
			if room.Lead != "" && !excluded[room.Lead] {
				group.OwnerID = busAliases[room.Lead]
				if group.OwnerID == "" {
					group.Resolution, group.Reason = "repair", "The group's owner cannot be resolved to a local chat."
				}
			}
			for _, member := range room.Members {
				if member.Kind != "session" || excluded[member.Key] {
					continue
				}
				if id := busAliases[member.Key]; id != "" {
					group.MemberIDs = append(group.MemberIDs, id)
				} else {
					group.Resolution, group.Reason = "repair", "A group member cannot be resolved to a local chat."
				}
			}
			slices.Sort(group.MemberIDs)
			group.MemberIDs = slices.Compact(group.MemberIDs)
			view.Groups = append(view.Groups, group)
		}
		teams := map[string]*OrgChartTeam{}
		recipientSet := map[string]bool{}
		for _, recipient := range saved.BusSessions {
			if recipient != "" {
				recipientSet[recipient] = true
			}
		}
		recipients := make([]string, 0, len(recipientSet))
		for recipient := range recipientSet {
			recipients = append(recipients, recipient)
		}
		slices.Sort(recipients)
		for _, recipient := range recipients {
			if excluded[recipient] {
				continue
			}
			lead := busAliases[recipient]
			key := lead
			if key == "" {
				key = "repair:" + recipient
			}
			members, err := b.TeamMembers(ctx, recipient)
			if err != nil {
				return err
			}
			if len(members) == 0 {
				continue
			}
			team := teams[key]
			if team == nil {
				team = &OrgChartTeam{ID: "team:" + key, Title: "Team needs repair", LeadID: lead, MemberIDs: []string{}, Resolution: "resolved"}
				if lead == "" {
					team.Resolution, team.Reason = "repair", "The team's lead cannot be resolved to a local chat."
				} else {
					team.Title = view.Chats[byChat[lead]].Title + "'s team"
				}
				teams[key] = team
			}
			for _, member := range members {
				if excluded[member.Session] {
					continue
				}
				if id := busAliases[member.Session]; id != "" {
					team.MemberIDs = append(team.MemberIDs, id)
				} else {
					team.Resolution, team.Reason = "repair", "A team member cannot be resolved to a local chat."
				}
			}
		}
		for _, team := range teams {
			slices.Sort(team.MemberIDs)
			team.MemberIDs = slices.Compact(team.MemberIDs)
			view.Teams = append(view.Teams, *team)
		}
		for index := range view.Chats {
			chat := &view.Chats[index]
			if chat.Resolution != "resolved" {
				continue
			}
			recipient := saved.BusSessions[chat.Head]
			_, retired, err := b.LeadOf(ctx, recipient)
			if err != nil {
				return err
			}
			chat.State = a.chatState(ctx, b, recipient, chat.Head, retired, false)
		}
		return nil
	})
	if err != nil {
		return OrgChartView{}, err
	}
	sort.Slice(view.Chats, func(i, j int) bool { return view.Chats[i].ChatID < view.Chats[j].ChatID })
	sort.Slice(view.Teams, func(i, j int) bool { return view.Teams[i].ID < view.Teams[j].ID })
	sort.Slice(view.Groups, func(i, j int) bool { return view.Groups[i].ID < view.Groups[j].ID })
	a.mu.Lock()
	current := orgChartOwnershipStamp(a.settings)
	a.mu.Unlock()
	if current != stamp {
		return OrgChartView{}, errors.New("Chat ownership changed while the chart was read. Refresh the chart.")
	}
	data, err := json.Marshal([]any{stamp, view})
	if err != nil {
		return OrgChartView{}, err
	}
	view.Generation = fmt.Sprintf("%x", sha256.Sum256(data))
	view.GeneratedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return view, nil
}

func orgChartOwnershipStamp(saved Settings) string {
	ownership := []any{}
	for _, reference := range logicalChatReferencesFromSettings(saved) {
		members, err := logicalChatMembersFromSettings(saved, reference)
		reason := ""
		if err != nil {
			reason = err.Error()
		}
		ownership = append(ownership, []string{reference, members.ChatID, members.Generation, reason})
	}
	data, _ := json.Marshal([]any{ownership, saved.SessionTitles, saved.SessionProviders, saved.BusSessions, saved.RemovedChats, saved.WorkspaceRecords})
	return fmt.Sprintf("%x", sha256.Sum256(data))
}
