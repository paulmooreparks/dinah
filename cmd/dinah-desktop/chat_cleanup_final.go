package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"studio/internal/acp"
	"studio/internal/bus"
)

func (a *App) cleanupDeleteProvider(member LogicalChatMember) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if a.chatCleanupHooks.DeleteProvider != nil {
		return a.chatCleanupHooks.DeleteProvider(ctx, member.Provider, member.NativeID)
	}
	if member.Provider == "" || member.NativeID == "" {
		return false, errors.New("The program's native identity is unknown.")
	}
	conn, init, err := a.lifecycle().Live(member.Provider)
	if err != nil {
		return false, err
	}
	if init.AgentCapabilities.SessionCapabilities.Delete == nil {
		return false, nil
	}
	err = conn.Call(ctx, "session/delete", acp.DeleteSessionParams{SessionID: member.NativeID}, nil)
	var missing *acp.Error
	if errors.As(err, &missing) && missing.Code == acp.CodeResourceNotFound {
		return true, nil
	}
	return true, err
}

func (a *App) DeleteRemovedChat(id string) ChatCleanupResult {
	a.chatCleanupMu.Lock()
	defer a.chatCleanupMu.Unlock()
	record, ok := a.cleanupRecord(id)
	if !ok || record.Group == "" || record.Stage != "retained" && record.Stage != "finalizing" {
		return a.cleanupResult(id, "deleted", refuseChatCleanup("Only a Recycle Bin entry can be permanently deleted."))
	}
	result, err := a.finalizeChatCleanup(record.Group)
	result.SessionID = id
	if err != nil {
		return a.cleanupResult(id, "deleted", err)
	}
	return result
}

func (a *App) EmptyChatRecycleBin() []ChatCleanupResult {
	listing, err := a.ListChatCleanup()
	if err != nil {
		return []ChatCleanupResult{{OK: false, Status: "failed", Error: err.Error()}}
	}
	results := make([]ChatCleanupResult, 0, len(listing.Removed))
	for _, row := range listing.Removed {
		results = append(results, a.DeleteRemovedChat(row.SessionID))
	}
	return results
}

func (a *App) finalizeChatCleanup(group string) (ChatCleanupResult, error) {
	result := ChatCleanupResult{SessionID: group, Status: "deleted", ProgramCopies: []ChatProgramCopy{}}
	archive, err := readChatCleanupArchive(group)
	if err != nil {
		return result, err
	}
	if err = a.updateSettings(func(s *Settings) error {
		for _, member := range archive.Membership.Members {
			r, ok := s.RemovedChats[member.SessionID]
			if !ok || r.Group != group || r.Stage != "retained" && r.Stage != "finalizing" {
				return refuseChatCleanup("Another operation changed this Recycle Bin entry.")
			}
			r.Stage = "finalizing"
			s.RemovedChats[member.SessionID] = r
		}
		return nil
	}); err != nil {
		return result, err
	}
	if archive.Progress == nil {
		archive.Progress = map[string]ChatProgramCopy{}
	}
	for _, member := range archive.Membership.Members {
		copy, done := archive.Progress[member.SessionID]
		if !done {
			supported, nativeErr := a.cleanupDeleteProvider(member)
			copy = ChatProgramCopy{SessionID: member.SessionID, Provider: member.Provider, ProviderName: providerName(member.Provider), Deletion: "supported", CopyRemains: !supported || nativeErr != nil}
			if !supported {
				copy.Deletion = "unsupported"
				copy.Reason = "This program has no available documented deletion; its copy remains."
			}
			if nativeErr != nil {
				copy.Reason = "The program's copy remains: " + nativeErr.Error()
			}
			archive.Progress[member.SessionID] = copy
			if err = archive.save(); err != nil {
				return result, err
			}
		}
		result.ProgramCopies = append(result.ProgramCopies, copy)
	}
	for _, unknown := range archive.Membership.UnresolvedAllocations {
		result.ProgramCopies = append(result.ProgramCopies, ChatProgramCopy{Provider: unknown.Provider, ProviderName: providerName(unknown.Provider), Deletion: "unknown", CopyRemains: true, Reason: "An allocation outcome is unknown; no native identity exists to request deletion."})
	}
	// All native attempts finish before any local chain erasure begins.
	for _, member := range archive.Membership.Members {
		a.revokeAppletSession(member.SessionID)
		a.cancelChatAppletWork(archive.Row.Workspace, member.SessionID)
		a.forgetTeam(member.SessionID)
	}
	if err = a.busHost().With(func(b *bus.Bus) error {
		seen := map[string]bool{}
		for _, recipient := range archive.Settings.BusSessions {
			if recipient != "" && !seen[recipient] {
				seen[recipient] = true
				if err := b.ForgetBinnedRecipient(context.Background(), recipient); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		return result, err
	}
	// SaveUIState uses the same cleanup mutex. Finalizing identities are filtered
	// before any full-state replacement, including a stale page's later save.
	ui, err := a.ReadUIState()
	if err != nil {
		return result, err
	}
	for _, member := range archive.Membership.Members {
		delete(ui.Values, "dinah-draft:chat:"+member.SessionID)
	}
	if err = a.saveUIStateForCleanup(ui); err != nil {
		return result, err
	}
	if err = cleanupEraseNotifications(archive.Row.SessionIDs); err != nil {
		return result, err
	}
	if err = a.updateSettings(func(s *Settings) error {
		ids := []string{}
		for _, member := range archive.Membership.Members {
			ids = append(ids, member.SessionID)
		}
		cleanupEraseSettings(s, ids)
		for _, member := range archive.Membership.Members {
			r := s.RemovedChats[member.SessionID]
			r.Stage = "erasing"
			s.RemovedChats[member.SessionID] = r
		}
		return nil
	}); err != nil {
		return result, err
	}
	if err = a.finishErasingChat(group); err != nil {
		return result, err
	}
	result.OK = true
	reasons := []string{}
	for _, copy := range result.ProgramCopies {
		if copy.CopyRemains {
			result.ProviderCopyRemains = true
			reasons = append(reasons, fmt.Sprintf("%s: %s", copy.ProviderName, copy.Reason))
		}
	}
	result.ProviderDeletionReason = strings.Join(reasons, " ")
	a.emit("chat:finalized", map[string]any{"sessionId": group, "sessionIds": archive.Row.SessionIDs, "result": result})
	a.emit("notification:delivery", nil)
	return result, nil
}

func cleanupEraseSettings(s *Settings, ids []string) {
	for _, id := range ids {
		delete(s.SessionProviders, id)
		delete(s.SessionOptions, id)
		delete(s.SessionTitles, id)
		delete(s.JudgedTitles, id)
		delete(s.NativeSessions, id)
		delete(s.NativeRecoveryAttempts, id)
		delete(s.BusSessions, id)
		delete(s.ChatBindings, id)
		delete(s.ChatTransfers, id)
	}
	s.ChatTransferHistory = slices.DeleteFunc(s.ChatTransferHistory, func(tr chatTransfer) bool { return slices.Contains(ids, tr.Source) })
	s.Hidden = slices.DeleteFunc(s.Hidden, func(id string) bool { return slices.Contains(ids, id) })
	for i := range s.WorkspaceRecords {
		s.WorkspaceRecords[i].Pins = slices.DeleteFunc(s.WorkspaceRecords[i].Pins, func(pin WorkspacePin) bool { return pin.Kind == "chat" && slices.Contains(ids, pin.Session) })
	}
}

func (a *App) finishErasingChat(group string) error {
	// The durable erasing phase proves native attempts and local record erasure
	// completed. Removing an already absent archive is safe after a crash.
	a.mu.Lock()
	ids := []string{}
	for id, record := range a.settings.RemovedChats {
		if record.Group == group {
			ids = append(ids, id)
		}
	}
	a.mu.Unlock()
	archive, readErr := readChatCleanupArchive(group)
	if readErr == nil {
		reasons := []string{}
		for _, member := range archive.Membership.Members {
			if copy := archive.Progress[member.SessionID]; copy.CopyRemains {
				reasons = append(reasons, providerName(copy.Provider)+": "+copy.Reason)
			}
		}
		for _, allocation := range archive.Membership.UnresolvedAllocations {
			reasons = append(reasons, providerName(allocation.Provider)+": an allocation outcome is unknown and no native identity is available for deletion.")
		}
		if len(reasons) != 0 {
			body := []rune("The chat was permanently deleted from this application. " + strings.Join(reasons, " "))
			if len(body) > 12000 {
				body = append(body[:12000], []rune(" Details were shortened to fit this report.")...)
			}
			_, err := a.SaveNotification(Notification{
				ID:   fmt.Sprintf("chat-copy-remains-%x", sha256.Sum256([]byte(group))),
				Kind: "app", Severity: "warning", Title: "An agent program kept chat history",
				Body: string(body),
			})
			if err != nil {
				return err
			}
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	if err := cleanupRedactBackups(ids); err != nil {
		return err
	}
	if err := removeChatCleanupArchive(group); err != nil {
		return err
	}
	return a.updateSettings(func(s *Settings) error {
		for id, r := range s.RemovedChats {
			if r.Group != group {
				continue
			}
			tombstone := chatRemoval{Stage: "deleted"}
			if r.Snapshot != nil && id == r.Snapshot.Head {
				for _, unknown := range r.Snapshot.UnresolvedAllocations {
					if unknown.AttemptID != "" {
						tombstone.UnknownAttempts = append(tombstone.UnknownAttempts, unknown.AttemptID)
					}
				}
			}
			s.RemovedChats[id] = tombstone
		}
		return nil
	})
}
