//go:build chatcleanup_acceptance

package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"studio/internal/bus"
)

type chatCleanupAcceptanceState struct {
	SessionIDs                                                           []string
	Binned, Tombstoned, TranscriptPresent, SettingsPresent, QueuePresent bool
	BusActive, RuntimeLoaded, ProviderLoaded                             bool
	RemovedAt, ExpiresAt                                                 time.Time
}

func TestCleanupAcceptanceObserverReadsRetainedAndErasedArtifacts(t *testing.T) {
	a, _ := providerTestApp(t)
	id := "observer-chat"
	cleanupFixtureChat(t, a, id, "codex", t.TempDir(), "observer-recipient")
	restore := a.chatCleanupAcceptanceInstall(chatCleanupAcceptanceHooks{DeleteProvider: func(context.Context, string, string) (bool, error) { return true, nil }})
	defer restore()
	check := func(binned, tombstoned, present, active bool) {
		t.Helper()
		state, err := a.chatCleanupAcceptanceObserve(id)
		if err != nil || state.Binned != binned || state.Tombstoned != tombstoned || state.TranscriptPresent != present || state.SettingsPresent != present || state.BusActive != active {
			t.Fatalf("observed %+v, error %v", state, err)
		}
	}
	check(false, false, true, true)
	if err := a.RemoveChat(id); err != nil {
		t.Fatal(err)
	}
	check(true, false, true, false)
	if result := a.RestoreRemovedChat(id); !result.OK {
		t.Fatal(result)
	}
	check(false, false, true, true)
	if err := a.RemoveChat(id); err != nil {
		t.Fatal(err)
	}
	if result := a.DeleteRemovedChat(id); !result.OK {
		t.Fatal(result)
	}
	check(false, true, false, false)
}

func (a *App) chatCleanupAcceptanceInstall(h chatCleanupAcceptanceHooks) func() {
	a.chatCleanupMu.Lock()
	previous := a.chatCleanupHooks
	a.chatCleanupHooks = h
	a.chatCleanupMu.Unlock()
	return func() {
		a.chatCleanupMu.Lock()
		a.chatCleanupHooks = previous
		a.chatCleanupMu.Unlock()
	}
}

func (a *App) chatCleanupAcceptanceSweep() error { return a.reconcileChatRemovals() }

// The observer reads the persisted settings and archive, without projecting
// expected outcomes onto them or starting a provider.
func (a *App) chatCleanupAcceptanceObserve(id string) (chatCleanupAcceptanceState, error) {
	a.chatCleanupMu.Lock()
	defer a.chatCleanupMu.Unlock()
	out := chatCleanupAcceptanceState{SessionIDs: []string{id}}
	data, err := readStateFile(filepath.Join(configDir(), "settings.json"))
	if err != nil {
		return out, err
	}
	var saved Settings
	if err := json.Unmarshal(data, &saved); err != nil {
		return out, err
	}
	record, removed := saved.RemovedChats[id]
	if record.Snapshot != nil {
		out.SessionIDs = nil
		for _, member := range record.Snapshot.Members {
			out.SessionIDs = append(out.SessionIDs, member.SessionID)
		}
	} else if !removed {
		members, resolveErr := logicalChatMembersFromSettings(saved, id)
		if resolveErr != nil {
			return out, resolveErr
		}
		out.SessionIDs = nil
		for _, member := range members.Members {
			out.SessionIDs = append(out.SessionIDs, member.SessionID)
		}
	}
	out.RemovedAt, out.ExpiresAt = record.Created, record.Expires
	out.Tombstoned = removed
	recipients := map[string]string{}
	for key, value := range saved.BusSessions {
		recipients[key] = value
	}
	if record.Group != "" {
		archive, archiveErr := readChatCleanupArchive(record.Group)
		if archiveErr != nil && !errors.Is(archiveErr, os.ErrNotExist) {
			return out, archiveErr
		}
		if archiveErr == nil {
			for key, value := range archive.Settings.BusSessions {
				recipients[key] = value
			}
			for _, file := range archive.Files {
				for _, member := range out.SessionIDs {
					if filepath.Join(configDir(), file.Path) == transcriptPath(member) {
						_, statErr := os.Stat(filepath.Join(chatCleanupArchiveDir(record.Group), file.Slot))
						out.TranscriptPresent = out.TranscriptPresent || statErr == nil
					}
				}
			}
			bytes, marshalErr := json.Marshal(archive.Settings)
			if marshalErr != nil {
				return out, marshalErr
			}
			out.SettingsPresent = string(bytes) != "{}"
		}
	}
	for _, member := range out.SessionIDs {
		r, exists := saved.RemovedChats[member]
		out.Binned = out.Binned || exists && r.Stage != "deleted" && r.Stage != "removed"
		out.Tombstoned = out.Tombstoned && exists && (r.Stage == "deleted" || r.Stage == "removed") && r.Group == "" && r.Snapshot == nil
		if _, statErr := os.Stat(transcriptPath(member)); statErr == nil {
			out.TranscriptPresent = true
		}
		payload, marshalErr := json.Marshal(cleanupSavedSettings(saved, []string{member}))
		if marshalErr != nil {
			return out, marshalErr
		}
		out.SettingsPresent = out.SettingsPresent || string(payload) != "{}"
		a.mu.Lock()
		out.RuntimeLoaded = out.RuntimeLoaded || a.runtime.ConversationLocked(member) != nil
		provider := saved.SessionProviders[member]
		if retained := a.chatCleanupLoaded[member]; retained != "" {
			provider = retained
		}
		out.ProviderLoaded = out.ProviderLoaded || provider != "" && a.providerConnLocked(provider) != nil
		a.mu.Unlock()
		if recipient := recipients[member]; recipient != "" {
			if err := a.busHost().With(func(b *bus.Bus) error {
				count, countErr := b.StoredSessionDeliveries(context.Background(), recipient)
				if countErr != nil {
					return countErr
				}
				out.QueuePresent = out.QueuePresent || count > 0
				active, activeErr := b.RecipientAvailable(context.Background(), recipient)
				out.BusActive = out.BusActive || active
				return activeErr
			}); err != nil {
				return out, err
			}
		}
	}
	return out, nil
}
