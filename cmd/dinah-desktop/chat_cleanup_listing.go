package main

import (
	"path/filepath"
	"sort"
	"time"

	"studio/internal/layout"
	"studio/internal/sessionhost"
	"studio/internal/workspaces"
)

const chatCleanupFinalConfirmation = "This permanently deletes the selected chats from this application. Each agent program will be asked to delete its own copy where documented deletion is available. Unsupported or failed provider deletions leave that program's copy intact. This cannot be undone."

// ListChatCleanup reads local metadata and saved messages. It never discovers
// provider history, opens a chat or acquires a provider connection.
func (a *App) ListChatCleanup() (ChatCleanupListing, error) {
	a.mu.Lock()
	settings, err := a.settings.clone()
	live := map[string]SessionInfo{}
	for _, s := range a.runtime.ConversationsLocked() {
		live[s.ID()] = s.InfoVersioned()
	}
	a.mu.Unlock()
	if err != nil {
		return ChatCleanupListing{}, err
	}
	_, heads := sessionhost.ConversationLinks(configDir())
	known := []string{}
	for id := range settings.SessionProviders {
		known = append(known, id)
	}
	for id := range settings.SessionTitles {
		known = append(known, id)
	}
	for id := range settings.SessionOptions {
		known = append(known, id)
	}
	for id := range settings.NativeSessions {
		known = append(known, id)
	}
	for id := range settings.BusSessions {
		known = append(known, id)
	}
	for id := range live {
		known = append(known, id)
	}
	listing := ChatCleanupListing{Rows: []ChatCleanupRow{}, Removed: []ChatCleanupRow{}, RetentionDays: 30, GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano), ProviderDeletionEnabled: true, FinalDeleteConfirmation: chatCleanupFinalConfirmation}
	for _, ids := range cleanupChains(settings, heads, known) {
		anyRemoved := false
		for _, id := range ids {
			if _, ok := settings.RemovedChats[id]; ok {
				anyRemoved = true
			}
		}
		if anyRemoved {
			continue
		} // Retained rows come from their durable metadata below.
		id := cleanupDisplayIdentity(settings, ids)
		row := ChatCleanupRow{SessionID: id, SessionIDs: ids, Title: settings.SessionTitles[id], MessageCountKnown: true, Eligible: true, ProviderDeletion: "unknown", ProgramCopies: []ChatProgramCopy{}, FinalDeleteConfirmation: chatCleanupFinalConfirmation}
		members := map[string]bool{}
		for _, member := range ids {
			members[member] = true
		}
		for _, member := range ids {
			provider := settings.SessionProviders[member]
			if h := heads[member]; h != nil {
				if provider == "" {
					provider = h.Provider
				}
				if member == id {
					row.Workspace = h.Workspace
				}
			}
			if s, ok := live[member]; ok {
				if provider == "" {
					provider = s.Provider
				}
				row.Open = row.Open || s.Open
				row.Working = row.Working || s.Working
				if member == id {
					row.Workspace = s.Cwd
					if row.Title == "" {
						row.Title = s.Title
					}
				}
				if s.UpdatedAt > row.LastActivity {
					row.LastActivity = s.UpdatedAt
				}
			}
			if member == id {
				row.Provider = provider
				row.ProviderName = providerName(provider)
			}
			row.ProgramCopies = append(row.ProgramCopies, ChatProgramCopy{SessionID: member, Provider: provider, ProviderName: providerName(provider), Deletion: "unknown", CopyRemains: true, Reason: "Documented deletion will be checked at final deletion."})
			if at := a.recordedActivity(member); at > row.LastActivity {
				row.LastActivity = at
			}
			if tr, readErr := readTranscript(member); readErr == nil && !tr.Deleted {
				for _, m := range tr.Messages {
					if m.Role == "you" || m.Role == "agent" {
						row.MessageCount++
					}
				}
			} else {
				row.MessageCountKnown = false
			}
			if _, running := a.transferRunning.Load(member); running {
				row.Refusal = "Wait for this chat's switch or recovery operation to finish."
			}
			recipient := settings.BusSessions[member]
			if recipient != "" {
				for other, owner := range settings.BusSessions {
					if owner == recipient && !members[other] {
						row.Refusal = "This chat shares its bus queue with another chat."
					}
				}
			}
		}
		if row.Title == "" {
			row.Title = id
		}
		for _, ws := range settings.WorkspaceRecords {
			if layout.SamePath(workspaces.ExecutionPath(ws), row.Workspace) {
				row.WorkspaceID = ws.ID
				row.WorkspaceName = ws.Name
			}
			for _, pin := range ws.Pins {
				if pin.Kind == "chat" && members[pin.Session] {
					row.Pinned = true
				}
			}
		}
		if row.WorkspaceName == "" && row.Workspace != "" {
			row.WorkspaceName = filepath.Base(row.Workspace)
		}
		if row.Pinned {
			row.Refusal = "Unpin this chat before deleting it."
		}
		if row.Working {
			row.Refusal = "Wait for this chat to finish working before deleting it."
		}
		row.Eligible = row.Refusal == ""
		listing.Rows = append(listing.Rows, row)
	}
	sort.SliceStable(listing.Rows, func(i, j int) bool {
		a, b := listing.Rows[i], listing.Rows[j]
		if a.LastActivity != b.LastActivity {
			return a.LastActivity < b.LastActivity
		}
		return a.SessionID < b.SessionID
	})
	return listing, nil
}
