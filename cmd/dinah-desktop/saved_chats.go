package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"studio/internal/bus"
	"studio/internal/providers"
	"studio/internal/sessionhost"
)

type SavedChatInfo struct {
	SessionInfo
	ChatID            string `json:"chatId"`
	Resolution        string `json:"resolution"`
	Reason            string `json:"reason"`
	ObservationSource string `json:"observationSource,omitempty"`
	ObservedAt        string `json:"observedAt,omitempty"`
	Freshness         string `json:"freshness,omitempty"`
}

type SavedChatInventory struct {
	Generation string          `json:"generation"`
	Chats      []SavedChatInfo `json:"chats"`
}

// SavedChats observes local ownership and already-present runtimes. Native
// history discovery remains in the explicit session-list entry points.
func (a *App) SavedChats() (SavedChatInventory, error) {
	view := SavedChatInventory{Chats: []SavedChatInfo{}}
	if err := a.captureRuntimeInventory(); err != nil {
		return view, err
	}
	a.mu.Lock()
	if a.startupErr != nil {
		err := a.startupErr
		a.mu.Unlock()
		return view, err
	}
	stamp := savedChatsSettingsStamp(a.settings)
	saved, err := a.settings.clone()
	runtimes := map[string]SessionInfo{}
	if err == nil {
		for _, chat := range a.runtime.ConversationsLocked() {
			runtimes[chat.ID()] = chat.InfoVersioned()
		}
	}
	a.mu.Unlock()
	if err != nil {
		return view, err
	}
	busTitles, err := a.inventoryBusTitles(saved)
	if err != nil {
		return view, err
	}
	_, headers := sessionhost.ConversationLinks(configDir())
	headerStamp := orgChartTranscriptStamp(headers)
	saved.logicalEvidence = map[string]sessionhost.TranscriptHead{}
	for id, header := range headers {
		saved.logicalEvidence[id] = *header
	}
	for id, info := range runtimes {
		if _, exists := saved.logicalEvidence[id]; !exists {
			saved.logicalEvidence[id] = sessionhost.TranscriptHead{SessionID: id, Provider: info.Provider, Workspace: info.Cwd}
		}
	}
	hidden := map[string]bool{}
	for _, id := range saved.Hidden {
		hidden[id] = true
	}
	seen := map[string]bool{}
	for _, reference := range logicalChatReferencesFromSettings(saved) {
		members, resolveErr := logicalChatMembersFromSettings(saved, reference)
		metadata, metadataErr := logicalChatMetadataFromSettings(saved, reference)
		if resolveErr == nil {
			resolveErr = metadataErr
		}
		if metadata.Lifecycle != "" && metadata.Lifecycle != "active" {
			continue
		}
		excluded := hidden[reference] || hidden[members.Head] || sessionhost.NotChat(reference) || sessionhost.NotChat(members.Head)
		if _, reserved := saved.RemovedChats[reference]; reserved {
			excluded = true
		}
		for _, member := range members.Members {
			if _, reserved := saved.RemovedChats[member.SessionID]; reserved {
				excluded = true
			}
		}
		if excluded {
			continue
		}
		id := members.ChatID
		if id == "" {
			id = reference
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		row := SavedChatInfo{SessionInfo: SessionInfo{ID: members.Head, Title: metadata.Title, Cwd: metadata.Workspace}, ChatID: id, Resolution: "resolved"}
		cached, recorded := saved.ListedChats[members.Head]
		if recorded {
			row.UpdatedAt, row.Provider = cached.UpdatedAt, cached.Provider
			row.ObservationSource, row.ObservedAt, row.Freshness = "providerListing", cached.ObservedAt, cached.Freshness
			if row.Freshness == "" {
				row.Freshness = "lastKnown"
			}
		}
		if info, present := runtimes[members.Head]; present {
			row.SessionInfo = info
			row.Title = inventoryTitle(saved, members.Head, info.Title, busTitles[members.Head].Title)
			row.ObservationSource, row.Freshness = "runtime", "current"
		} else {
			if header, local := saved.logicalEvidence[members.Head]; local && !header.Deleted {
				row.State = &ChatState{Life: lifeClosed}
			}
			providerTitle := cached.Title
			if providerTitle == "" {
				providerTitle = cached.LocalTitle
			}
			row.Title = inventoryTitle(saved, members.Head, providerTitle, busTitles[members.Head].Title)
			if provider := saved.SessionProviders[members.Head]; provider != "" {
				row.Provider = provider
			}
			if row.Provider == "" {
				row.Provider = saved.logicalEvidence[members.Head].Provider
			}
		}
		if busTitles[members.Head].Retired {
			row.State = &ChatState{Life: lifeRetired}
		}
		if resolveErr != nil {
			row.ID, row.Resolution, row.Reason = id, "repair", resolveErr.Error()
			if title := saved.SessionTitles[reference]; title != "" {
				row.Title = title
			}
			if evidence := saved.logicalEvidence[reference]; evidence.Workspace != "" && evidence.Provider != "" {
				row.Cwd, row.Provider = evidence.Workspace, evidence.Provider
			}
		}
		if row.Resolution == "resolved" && (row.Cwd == "" || row.Provider == "") {
			row.ID, row.Resolution = id, "repair"
			row.Reason = "This saved chat is missing workspace or program metadata."
		}
		for _, member := range members.Members {
			provider := member.Provider
			if provider == "" {
				provider = saved.logicalEvidence[member.SessionID].Provider
			}
			at := a.journals.Latest(member.SessionID, func(id string) string {
				return sessionhost.JournalPath(configDir(), id, providers.For(provider).JournalFolder)
			})
			if !at.IsZero() {
				row.UpdatedAt = sessionhost.LatestActivity(row.UpdatedAt, at.UTC().Format(time.RFC3339Nano))
			}
		}
		view.Chats = append(view.Chats, row)
	}
	sort.Slice(view.Chats, func(i, j int) bool {
		left, _ := time.Parse(time.RFC3339Nano, view.Chats[i].UpdatedAt)
		right, _ := time.Parse(time.RFC3339Nano, view.Chats[j].UpdatedAt)
		if !left.Equal(right) {
			return left.After(right)
		}
		return view.Chats[i].ChatID < view.Chats[j].ChatID
	})
	a.mu.Lock()
	current := savedChatsSettingsStamp(a.settings)
	currentRuntimes := map[string]SessionInfo{}
	for _, chat := range a.runtime.ConversationsLocked() {
		currentRuntimes[chat.ID()] = chat.InfoVersioned()
	}
	a.mu.Unlock()
	currentBusTitles, err := a.inventoryBusTitles(saved)
	if err != nil {
		return SavedChatInventory{}, err
	}
	_, currentHeaders := sessionhost.ConversationLinks(configDir())
	if current != stamp || orgChartTranscriptStamp(currentHeaders) != headerStamp || inventoryRuntimeHash(runtimes) != inventoryRuntimeHash(currentRuntimes) || inventoryHash(busTitles) != inventoryHash(currentBusTitles) {
		return SavedChatInventory{}, errors.New("Saved chat identities changed while the list was read. Refresh the list.")
	}
	stable := append([]SavedChatInfo(nil), view.Chats...)
	for i := range stable {
		stable[i].StateGen, stable[i].StateVer = "", 0
	}
	data, err := json.Marshal([]any{stamp, headerStamp, busTitles, stable})
	if err != nil {
		return SavedChatInventory{}, err
	}
	view.Generation = fmt.Sprintf("%x", sha256.Sum256(data))
	return view, nil
}

func savedChatsSettingsStamp(saved Settings) string {
	data, _ := json.Marshal([]any{saved, saved.WorkspaceRecords})
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func inventoryHash(value any) string {
	data, _ := json.Marshal(value)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func inventoryRuntimeHash(rows map[string]SessionInfo) string {
	stable := map[string]SessionInfo{}
	for id, row := range rows {
		row.StateGen, row.StateVer = "", 0
		stable[id] = row
	}
	return inventoryHash(stable)
}

type inventoryBusMetadata struct {
	Title   string
	Retired bool
}

func (a *App) inventoryBusTitles(saved Settings) (map[string]inventoryBusMetadata, error) {
	titles := map[string]inventoryBusMetadata{}
	if len(saved.BusSessions) == 0 {
		return titles, nil
	}
	err := a.busHost().With(func(b *bus.Bus) error {
		for id, recipient := range saved.BusSessions {
			if recipient == "" {
				continue
			}
			row, err := b.Session(context.Background(), recipient)
			if errors.Is(err, bus.ErrNotFound) {
				continue
			}
			if err != nil {
				return err
			}
			_, retired, err := b.LeadOf(context.Background(), recipient)
			if err != nil {
				return err
			}
			titles[id] = inventoryBusMetadata{Title: row.Title, Retired: retired}
		}
		return nil
	})
	return titles, err
}
