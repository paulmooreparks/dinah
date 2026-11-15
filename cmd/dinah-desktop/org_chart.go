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
	ChatID        string     `json:"chatId"`
	Head          string     `json:"head"`
	Title         string     `json:"title"`
	UpdatedAt     string     `json:"updatedAt"`
	Workspace     string     `json:"workspace"`
	WorkspaceName string     `json:"workspaceName"`
	Provider      string     `json:"provider"`
	ProviderName  string     `json:"providerName"`
	State         *ChatState `json:"state"`
	Resolution    string     `json:"resolution"`
	Reason        string     `json:"reason"`
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
	inventory, err := a.SavedChats()
	if err != nil {
		return view, err
	}
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
	headerStamp := orgChartTranscriptStamp(headers)
	saved.logicalEvidence = map[string]sessionhost.TranscriptHead{}
	for id, header := range headers {
		saved.logicalEvidence[id] = *header
	}
	aliases, excluded := map[string]string{}, map[string]bool{}
	byChat := map[string]int{}
	for _, reference := range logicalChatReferencesFromSettings(saved) {
		members, _ := logicalChatMembersFromSettings(saved, reference)
		removed := false
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
		}
	}
	// Relationships augment the same captured rows used by the Chats panel.
	view.Chats = []OrgChartChat{}
	aliases, byChat = map[string]string{}, map[string]int{}
	for _, row := range inventory.Chats {
		chat := OrgChartChat{ChatID: row.ChatID, Head: row.ID, Title: row.Title, UpdatedAt: row.UpdatedAt, Workspace: row.Cwd, Provider: row.Provider, ProviderName: providerName(row.Provider), State: row.State, Resolution: row.Resolution, Reason: row.Reason}
		if row.Resolution != "resolved" {
			chat.Head = ""
		}
		for _, ws := range saved.WorkspaceRecords {
			if layout.SamePath(workspaces.ExecutionPath(ws), row.Cwd) {
				chat.WorkspaceName = ws.Name
			}
		}
		if row.Resolution == "resolved" {
			members, err := logicalChatMembersFromSettings(saved, row.ID)
			if err != nil {
				return OrgChartView{}, err
			}
			for _, member := range members.Members {
				aliases[member.SessionID] = row.ChatID
			}
		}
		byChat[row.ChatID] = len(view.Chats)
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
		relations, err := b.TeamRelations(ctx)
		if err != nil {
			return err
		}
		channels := map[string]bus.Room{}
		for _, room := range relations.Rooms {
			if room.Lead != "" {
				channels[room.Lead] = room
			}
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
			members := []bus.TeamMember{}
			for _, member := range relations.Members {
				if member.Lead == recipient {
					members = append(members, member)
				}
			}
			if len(members) == 0 {
				continue
			}
			team := teams[key]
			if team == nil {
				team = &OrgChartTeam{ID: "team:" + key, Title: "Team needs repair", LeadID: lead, MemberIDs: []string{}, Resolution: "resolved"}
				if channel, ok := channels[recipient]; ok {
					team.ID = "team:" + channel.Name
					team.Title = channel.Title
				}
				if lead == "" {
					team.Resolution, team.Reason = "repair", "The team's lead cannot be resolved to a local chat."
				} else if _, ok := channels[recipient]; !ok {
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
	// Validate the captured headers without rebuilding any row from a later view.
	_, currentHeaders := sessionhost.ConversationLinks(configDir())
	if orgChartTranscriptStamp(currentHeaders) != headerStamp {
		return OrgChartView{}, errors.New("Saved chat identities changed while the chart was read. Refresh the chart.")
	}
	currentInventory, err := a.SavedChats()
	if err != nil {
		return OrgChartView{}, err
	}
	if currentInventory.Generation != inventory.Generation {
		return OrgChartView{}, errors.New("Chat inventory changed while the chart was read. Refresh the chart.")
	}
	data, err := json.Marshal([]any{stamp, headerStamp, inventory.Generation, view})
	if err != nil {
		return OrgChartView{}, err
	}
	view.Generation = fmt.Sprintf("%x", sha256.Sum256(data))
	view.GeneratedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return view, nil
}

func orgChartTranscriptStamp(headers map[string]*sessionhost.TranscriptHead) string {
	data, _ := json.Marshal(headers)
	return fmt.Sprintf("%x", sha256.Sum256(data))
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
