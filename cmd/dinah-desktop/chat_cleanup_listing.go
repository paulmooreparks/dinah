package main

import (
	"errors"
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
	return a.listChatCleanupAtBoundary(nil)
}

func (a *App) listChatCleanupAtBoundary(afterInventory func()) (ChatCleanupListing, error) {
	return a.listChatCleanupWithStages(afterInventory, false)
}

// listChatCleanupWithStages also gives the earlier stages of a chat as rows
// when stages is set (savedChatsWithStages). The page's listing never does.
func (a *App) listChatCleanupWithStages(afterInventory func(), stages bool) (ChatCleanupListing, error) {
	inventory, err := a.savedChatsWithStages(nil, stages)
	if err != nil {
		return ChatCleanupListing{}, err
	}
	if afterInventory != nil {
		afterInventory()
	}
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
	teamRefusals, err := a.cleanupTeamRefusals(settings)
	if err != nil {
		return ChatCleanupListing{}, err
	}
	_, heads := sessionhost.ConversationLinks(configDir())
	known := []string{}
	for id := range settings.ListedChats {
		known = append(known, id)
	}
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
	for id := range heads {
		known = append(known, id)
	}
	for source, tr := range settings.ChatTransfers {
		known = append(known, source)
		if tr.Target != "" {
			known = append(known, tr.Target)
		}
	}
	for _, tr := range settings.ChatTransferHistory {
		known = append(known, tr.Source)
		if tr.Target != "" {
			known = append(known, tr.Target)
		}
	}
	sort.Strings(known)
	seenReferences, seenChats := map[string]bool{}, map[string]bool{}
	// A stage is a member of its own row and of the row of every chat that
	// continues it, so its saved conversation is read once for all of them.
	type savedMessages struct {
		count int
		known bool
	}
	messages := map[string]savedMessages{}
	for _, reference := range known {
		if reference == "" || seenReferences[reference] {
			continue
		}
		seenReferences[reference] = true
		// A row has what removing it takes, which includes the chat's earlier
		// stages (chatWithEarlierStages).
		membership, resolveErr := chatWithEarlierStages(settings, heads, reference)
		if resolveErr != nil {
			listing.Rows = append(listing.Rows, ChatCleanupRow{SessionID: reference, SessionIDs: []string{reference}, Title: settings.SessionTitles[reference], Refusal: resolveErr.Error(), ProgramCopies: []ChatProgramCopy{}})
			continue
		}
		if seenChats[membership.ChatID] {
			continue
		}
		seenChats[membership.ChatID] = true
		ids := make([]string, 0, len(membership.Members))
		for _, member := range membership.Members {
			ids = append(ids, member.SessionID)
		}

		anyRemoved := false
		for _, id := range ids {
			if _, ok := settings.RemovedChats[id]; ok {
				anyRemoved = true
			}
		}
		if anyRemoved {
			continue
		} // Retained rows come from their durable metadata below.
		id := membership.Head
		row := ChatCleanupRow{SessionID: id, SessionIDs: ids, Title: settings.SessionTitles[id], MessageCountKnown: true, Eligible: true, ProviderDeletion: "unknown", ProgramCopies: []ChatProgramCopy{}, FinalDeleteConfirmation: chatCleanupFinalConfirmation}
		if recorded, ok := settings.ListedChats[id]; ok {
			row.Workspace = recorded.Workspace
			row.LastActivity = recorded.UpdatedAt
			if row.Title == "" {
				row.Title = a.sessionTitle(id, recorded.Title)
			}
		}
		members := map[string]bool{}
		for _, member := range ids {
			members[member] = true
		}
		for _, member := range ids {
			provider := settings.SessionProviders[member]
			if provider == "" {
				provider = settings.ListedChats[member].Provider
			}
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
				if cleanupActivityAfter(s.UpdatedAt, row.LastActivity) {
					row.LastActivity = s.UpdatedAt
				}
			}
			if member == id {
				row.Provider = provider
				row.ProviderName = providerName(provider)
			}
			copy := ChatProgramCopy{SessionID: member, Provider: provider, ProviderName: providerName(provider), Deletion: "unknown", CopyRemains: true, Reason: "Documented deletion will be checked at final deletion."}
			a.mu.Lock()
			conn, init := a.providerInitLocked(provider)
			a.mu.Unlock()
			if conn != nil {
				if init.AgentCapabilities.SessionCapabilities.Delete != nil {
					copy.Deletion = "supported"
					copy.Reason = "The program will be asked to delete its own copy at final deletion."
				} else {
					copy.Deletion = "unsupported"
					copy.Reason = "This program has no documented deletion; its copy will remain."
				}
			}
			if copy.Deletion == "unsupported" {
				row.FinalDeleteConfirmation += " " + copy.ProviderName + " will keep its own copy."
			}
			row.ProgramCopies = append(row.ProgramCopies, copy)
			if at := a.recordedActivity(member); cleanupActivityAfter(at, row.LastActivity) {
				row.LastActivity = at
			}
			saved, read := messages[member]
			if !read {
				if tr, readErr := readTranscript(member); readErr == nil && !tr.Deleted {
					saved.known = true
					for _, m := range tr.Messages {
						if m.Role == "you" || m.Role == "agent" {
							saved.count++
						}
					}
				}
				messages[member] = saved
			}
			if saved.known {
				row.MessageCount += saved.count
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
		if len(row.ProgramCopies) > 0 {
			row.ProviderDeletion = row.ProgramCopies[0].Deletion
			for _, copy := range row.ProgramCopies {
				if copy.Deletion != row.ProviderDeletion {
					row.ProviderDeletion = "unknown"
					break
				}
			}
		}
		for _, member := range ids {
			if reason := teamRefusals[settings.BusSessions[member]]; reason != "" {
				row.Refusal = reason
				break
			}
		}
		row.Eligible = row.Refusal == ""
		listing.Rows = append(listing.Rows, row)
	}
	sort.SliceStable(listing.Rows, func(i, j int) bool {
		a, b := listing.Rows[i], listing.Rows[j]
		if a.LastActivity != b.LastActivity {
			return cleanupActivityAfter(b.LastActivity, a.LastActivity)
		}
		return a.SessionID < b.SessionID
	})
	groups := map[string]chatRemoval{}
	for _, record := range settings.RemovedChats {
		if record.Group != "" && record.Stage != "deleted" {
			groups[record.Group] = record
		}
	}
	for group, record := range groups {
		archive, readErr := readChatCleanupArchive(group)
		if readErr != nil {
			return ChatCleanupListing{}, readErr
		}
		row := archive.Row
		row.Open = false
		row.Working = false
		row.Eligible = false
		row.Restorable = record.Stage == "retained" && a.cleanupNow().Before(record.Expires)
		if !row.Restorable {
			row.RestoreRefusal = "This entry is expired or a cleanup operation is in progress."
		}
		row.RuntimeNote = ""
		a.mu.Lock()
		for _, member := range archive.Membership.Members {
			if provider := a.chatCleanupLoaded[member.SessionID]; provider != "" && a.providerConnLocked(provider) != nil {
				row.RuntimeNote += "Still loaded by " + providerName(provider) + " until the program stops or the window restarts. "
			}
		}
		a.mu.Unlock()
		for _, unknown := range archive.Membership.UnresolvedAllocations {
			row.RuntimeNote += "An allocation by " + providerName(unknown.Provider) + " has an unknown outcome; its native copy cannot be identified. "
		}
		listing.Removed = append(listing.Removed, row)
	}
	sort.SliceStable(listing.Removed, func(i, j int) bool {
		if listing.Removed[i].RemovedAt != listing.Removed[j].RemovedAt {
			return listing.Removed[i].RemovedAt < listing.Removed[j].RemovedAt
		}
		return listing.Removed[i].SessionID < listing.Removed[j].SessionID
	})

	byHead := map[string]ChatCleanupRow{}
	for _, row := range listing.Rows {
		byHead[row.SessionID] = row
	}
	listing.Rows = []ChatCleanupRow{}
	for _, chat := range inventory.Chats {
		row, ok := byHead[chat.ID]
		if !ok {
			row = ChatCleanupRow{SessionID: chat.ID, SessionIDs: []string{chat.ID}, Refusal: "This chat needs ownership repair before cleanup."}
		}
		row.Title, row.Workspace, row.Provider = chat.Title, chat.Cwd, chat.Provider
		if chat.Resolution != "resolved" {
			row.Refusal = chat.Reason
		}
		row.Eligible = row.Refusal == ""
		listing.Rows = append(listing.Rows, row)
	}
	current, err := a.SavedChats()
	if err != nil {
		return ChatCleanupListing{}, err
	}
	if current.identity != inventory.identity {
		return ChatCleanupListing{}, errors.New("Chat inventory changed while cleanup was read. Refresh the list.")
	}
	return listing, nil
}

// RFC3339 timestamps may have different offsets and fractional precision.
func cleanupActivityAfter(candidate, current string) bool {
	next, err := time.Parse(time.RFC3339Nano, candidate)
	if err != nil {
		return false
	}
	previous, err := time.Parse(time.RFC3339Nano, current)
	return err != nil || next.After(previous)
}
