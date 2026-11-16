package main

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"time"

	"studio/internal/acp"
	"studio/internal/bus"
	"studio/internal/sessionhost"
)

type chatCleanupAcceptanceHooks struct {
	Now            func() time.Time
	DeleteProvider func(context.Context, string, string) (bool, error)
	UnloadProvider func(context.Context, string, string) (bool, error)
}

type chatCleanupRefusal struct{ reason string }

func (e *chatCleanupRefusal) Error() string { return e.reason }
func refuseChatCleanup(reason string) error { return &chatCleanupRefusal{reason} }
func (a *App) cleanupNow() time.Time {
	if a.chatCleanupHooks.Now != nil {
		return a.chatCleanupHooks.Now().UTC()
	}
	return time.Now().UTC()
}

func (a *App) cleanupRecord(id string) (chatRemoval, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	r, ok := a.settings.RemovedChats[id]
	return r, ok
}

// RemoveChat stages the complete logical chat. Native deletion is reserved
// for expiry or an explicit final-deletion call.
func (a *App) RemoveChat(id string) error {
	a.chatCleanupMu.Lock()
	defer a.chatCleanupMu.Unlock()
	return a.stageChatCleanup(id)
}

func (a *App) stageChatCleanup(id string) error {
	if r, ok := a.cleanupRecord(id); ok {
		switch r.Stage {
		case "retained":
			return nil
		case "retaining":
			return a.finishRetainingChat(r.Group)
		default:
			return refuseChatCleanup("This chat is already removed or another cleanup operation is in progress.")
		}
	}
	if err := a.captureRuntimeInventory(); err != nil {
		return err
	}
	membership, err := a.chatRemovalMembers(id)
	if err != nil {
		return refuseChatCleanup(err.Error())
	}
	listing, err := a.listChatCleanupWithStages(nil, true)
	if err != nil {
		return err
	}
	// The row is the one the listing built for this membership. An earlier
	// stage is also among the identities of the chat that continues it, so a
	// search of each row's identities could find that chat's row instead.
	var row *ChatCleanupRow
	for _, wanted := range []string{membership.Head, id, membership.ChatID} {
		for _, candidate := range listing.Rows {
			if candidate.SessionID == wanted {
				v := candidate
				row = &v
				break
			}
		}
		if row != nil {
			break
		}
	}
	if row == nil {
		return refuseChatCleanup("This chat is not in the local inventory.")
	}
	if !row.Eligible {
		return refuseChatCleanup(row.Refusal)
	}
	ids := make([]string, 0, len(membership.Members))
	held := []*Session{}
	defer func() {
		for _, s := range held {
			s.SetSwitching(false)
		}
	}()
	for _, m := range membership.Members {
		ids = append(ids, m.SessionID)
		if _, active := a.transferRunning.Load(m.SessionID); active {
			return refuseChatCleanup("Wait for this chat's switch or recovery to finish.")
		}
		if _, _, active := a.busCoordinator().CurrentDeliveryAttempt(m.SessionID); active {
			return refuseChatCleanup("Wait for this chat's current opening to finish.")
		}
		if s := a.session(m.SessionID); s != nil {
			if !s.BeginSwitch() {
				return refuseChatCleanup("Wait for this chat's provider change to finish.")
			}
			held = append(held, s)
			if s.Working() || s.Receiving() {
				return refuseChatCleanup("Wait for this chat to finish working or opening.")
			}
		}
	}
	a.mu.Lock()
	settings, err := a.settings.clone()
	a.mu.Unlock()
	if err != nil {
		return err
	}
	now := a.cleanupNow()
	row.SessionIDs = ids
	row.RemovedAt = now.Format(time.RFC3339Nano)
	row.ExpiresAt = now.Add(30 * 24 * time.Hour).Format(time.RFC3339Nano)
	row.Open = false
	row.Working = false
	row.Eligible = false
	row.Restorable = true
	archive := &chatCleanupArchive{Version: 1, Row: *row, Settings: cleanupSavedSettings(settings, ids), Membership: membership, Progress: map[string]ChatProgramCopy{}}
	for i, m := range archive.Membership.Members {
		if m.Provider == "" {
			for _, copy := range row.ProgramCopies {
				if copy.SessionID == m.SessionID {
					archive.Membership.Members[i].Provider = copy.Provider
				}
			}
		}
	}
	if a.queueStore != nil {
		a.queueStore.Flush()
	}
	for _, member := range ids {
		for _, path := range []string{transcriptPath(member), journalPath(member), filepath.Join(configDir(), "queues", base64.RawURLEncoding.EncodeToString([]byte(member))+".json"), a.chatLogs().Path(member)} {
			if path != "" {
				if err := archive.addFile(path); err != nil {
					return err
				}
			}
		}
	}
	if err = archive.save(); err != nil {
		return err
	}
	group := row.SessionID
	if err = a.updateSettings(func(s *Settings) error {
		_, headers := sessionhost.ConversationLinks(configDir())
		current, err := chatWithEarlierStages(*s, headers, id)
		if err != nil {
			return err
		}
		if current.Generation != membership.Generation {
			return refuseChatCleanup("This chat's membership changed. Refresh the cleanup page and try again.")
		}
		for _, ws := range s.WorkspaceRecords {
			for _, pin := range ws.Pins {
				if pin.Kind == "chat" && slices.Contains(ids, pin.Session) {
					return refuseChatCleanup(unpinFirst(ws.Name))
				}
			}
		}
		for other, recipient := range s.BusSessions {
			if !slices.Contains(ids, other) {
				for _, member := range ids {
					if recipient != "" && recipient == s.BusSessions[member] {
						return refuseChatCleanup("This chat shares its bus queue with another chat.")
					}
				}
			}
		}
		if s.RemovedChats == nil {
			s.RemovedChats = map[string]chatRemoval{}
		}
		for _, member := range ids {
			if _, exists := s.RemovedChats[member]; exists {
				return refuseChatCleanup("Another cleanup operation already reserved this chat.")
			}
		}
		for _, member := range ids {
			s.RemovedChats[member] = chatRemoval{Group: group, Recipient: s.BusSessions[member], Stage: "retaining", Created: now, Expires: now.Add(30 * 24 * time.Hour), Snapshot: &archive.Membership}
		}
		return nil
	}); err != nil {
		// No files move before the reservation commits. A refused generation
		// must not leave an unowned retained settings payload behind.
		return errors.Join(err, removeChatCleanupArchive(group))
	}
	err = a.finishRetainingChat(group)
	var lead *bus.TeamLeadRefusal
	if errors.As(err, &lead) {
		// This refusal happens before the atomic bus withdrawal and before any
		// file moves. Only this fresh reservation may be released here.
		rollback := a.updateSettings(func(s *Settings) error {
			for _, member := range ids {
				if row, ok := s.RemovedChats[member]; !ok || row.Group != group || row.Stage != "retaining" {
					return errors.New("cleanup reservation changed before refusal recovery")
				}
			}
			for _, member := range ids {
				delete(s.RemovedChats, member)
			}
			return nil
		})
		if rollback != nil {
			return errors.Join(err, rollback)
		}
		return errors.Join(refuseChatCleanup(lead.Error()), removeChatCleanupArchive(group))
	}
	return err
}

func (a *App) finishRetainingChat(group string) error {
	archive, err := readChatCleanupArchive(group)
	if err != nil {
		return err
	}
	recipients := map[string]bool{}
	for _, member := range archive.Membership.Members {
		if recipient := archive.Settings.BusSessions[member.SessionID]; recipient != "" {
			recipients[recipient] = true
		}
	}
	if err = a.busHost().With(func(b *bus.Bus) error {
		ids := make([]string, 0, len(recipients))
		for recipient := range recipients {
			ids = append(ids, recipient)
		}
		return b.BinRecipients(context.Background(), ids)
	}); err != nil {
		return err
	}
	for _, member := range archive.Membership.Members {
		id := member.SessionID
		a.revokeAppletSession(id)
		a.cancelChatAppletWork(archive.Row.Workspace, id)
		a.permissions().CancelSession(id)
		if s := a.session(id); s != nil {
			a.subagentService().Disconnect(s)
			a.chatLogs().Close(id)
			s.Close()
			a.mu.Lock()
			a.runtime.RemoveConversationLocked(id)
			a.mu.Unlock()
			supported, closeErr := a.cleanupUnload(member)
			if !supported || closeErr != nil {
				a.mu.Lock()
				if a.chatCleanupLoaded == nil {
					a.chatCleanupLoaded = map[string]string{}
				}
				a.chatCleanupLoaded[id] = member.Provider
				a.mu.Unlock()
			}
		}
	}
	if a.queueStore != nil {
		a.queueStore.Flush()
	}
	if err = archive.moveFiles(false); err != nil {
		return err
	}
	for _, member := range archive.Membership.Members {
		if a.queueStore != nil {
			a.queueStore.Forget(member.SessionID)
		}
	}
	if err = a.updateSettings(func(s *Settings) error {
		for _, member := range archive.Membership.Members {
			r := s.RemovedChats[member.SessionID]
			r.Stage = "retained"
			s.RemovedChats[member.SessionID] = r
		}
		return nil
	}); err != nil {
		return err
	}
	a.releaseUnusedCleanupProviders()
	// Removing a member changes the remaining chats' lead and team state.
	// Publish those changes before a caller starts its next inventory read.
	a.sendTeamStates()
	a.sendChatStates()
	for _, member := range archive.Membership.Members {
		a.emit("chat:removed", map[string]string{"sessionId": member.SessionID})
	}
	a.emit("notification:delivery", nil)
	return nil
}

func (a *App) cleanupUnload(member LogicalChatMember) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if a.chatCleanupHooks.UnloadProvider != nil {
		return a.chatCleanupHooks.UnloadProvider(ctx, member.Provider, member.NativeID)
	}
	a.mu.Lock()
	conn, init := a.providerInitLocked(member.Provider)
	a.mu.Unlock()
	if conn == nil {
		return true, nil
	}
	if init.AgentCapabilities.SessionCapabilities.Close == nil {
		return false, nil
	}
	return true, conn.Call(ctx, "session/close", acp.CloseSessionParams{SessionID: member.NativeID}, nil)
}

func (a *App) releaseUnusedCleanupProviders() {
	a.mu.Lock()
	loaded := map[string]string{}
	for id, p := range a.chatCleanupLoaded {
		loaded[id] = p
	}
	a.mu.Unlock()
	for id, provider := range loaded {
		a.mu.Lock()
		needed := false
		for _, s := range a.runtime.ConversationsLocked() {
			if s.Provider() == provider {
				needed = true
				break
			}
		}
		conn := a.providerConnLocked(provider)
		a.mu.Unlock()
		if !needed {
			if conn != nil {
				a.lifecycle().Stop(provider)
			}
			a.mu.Lock()
			for other, p := range a.chatCleanupLoaded {
				if p == provider {
					delete(a.chatCleanupLoaded, other)
				}
			}
			a.mu.Unlock()
		} else if conn == nil {
			a.mu.Lock()
			delete(a.chatCleanupLoaded, id)
			a.mu.Unlock()
		}
	}
}

func (a *App) cleanupResult(id, status string, err error) ChatCleanupResult {
	result := ChatCleanupResult{SessionID: id, OK: err == nil, Status: status, ProgramCopies: []ChatProgramCopy{}}
	if err != nil {
		result.Status = "failed"
		var refused *chatCleanupRefusal
		if errors.As(err, &refused) {
			result.Status = "refused"
		}
		result.Error = err.Error()
	}
	if listing, listErr := a.ListChatCleanup(); listErr == nil {
		for _, row := range append(listing.Rows, listing.Removed...) {
			if row.SessionID == id || slices.Contains(row.SessionIDs, id) {
				v := row
				result.Row = &v
				break
			}
		}
	}
	return result
}

func (a *App) RemoveChats(ids []string) []ChatCleanupResult {
	results := make([]ChatCleanupResult, 0, len(ids))
	for _, id := range ids {
		results = append(results, a.cleanupResult(id, "removed", a.RemoveChat(id)))
	}
	return results
}

func (a *App) RestoreRemovedChat(id string) ChatCleanupResult {
	a.chatCleanupMu.Lock()
	defer a.chatCleanupMu.Unlock()
	err := a.restoreChatCleanup(id)
	return a.cleanupResult(id, "restored", err)
}

func (a *App) restoreChatCleanup(id string) error {
	record, ok := a.cleanupRecord(id)
	if !ok || record.Stage != "retained" {
		return refuseChatCleanup("This chat is not a restorable Recycle Bin entry.")
	}
	if !a.cleanupNow().Before(record.Expires) {
		return refuseChatCleanup("This chat's 30-day retention has expired.")
	}
	archive, err := readChatCleanupArchive(record.Group)
	if err != nil {
		return err
	}
	if err = a.updateSettings(func(s *Settings) error {
		for _, member := range archive.Membership.Members {
			r := s.RemovedChats[member.SessionID]
			if r.Stage != "retained" {
				return refuseChatCleanup("Another operation changed this chat.")
			}
			r.Stage = "restoring"
			s.RemovedChats[member.SessionID] = r
		}
		return nil
	}); err != nil {
		return err
	}
	return a.finishRestoringChat(record.Group)
}

func (a *App) finishRestoringChat(group string) error {
	archive, err := readChatCleanupArchive(group)
	if err != nil {
		return err
	}
	if err = archive.moveFiles(true); err != nil {
		return err
	}
	if err = a.busHost().With(func(b *bus.Bus) error {
		seen := map[string]bool{}
		for _, recipient := range archive.Settings.BusSessions {
			if !seen[recipient] {
				seen[recipient] = true
				if err := b.RestoreRecipient(context.Background(), recipient); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if err = a.updateSettings(func(s *Settings) error {
		cleanupRestoreSettings(s, archive.Settings)
		for _, member := range archive.Membership.Members {
			r := s.RemovedChats[member.SessionID]
			r.Stage = "restored_cleanup"
			s.RemovedChats[member.SessionID] = r
		}
		return nil
	}); err != nil {
		return err
	}
	return a.finishRestoredCleanup(group)
}

func (a *App) finishRestoredCleanup(group string) error {
	if err := removeChatCleanupArchive(group); err != nil {
		return err
	}
	ids := []string{}
	if err := a.updateSettings(func(s *Settings) error {
		for id, record := range s.RemovedChats {
			if record.Group == group {
				ids = append(ids, id)
				delete(s.RemovedChats, id)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	for _, id := range ids {
		a.emit("chat:restored", map[string]string{"sessionId": id})
	}
	a.emit("notification:delivery", nil)
	return nil
}

func removeChatCleanupArchive(id string) error {
	root, err := filepath.Abs(filepath.Join(configDir(), "recycle"))
	if err != nil {
		return err
	}
	target, err := filepath.Abs(chatCleanupArchiveDir(id))
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root, target)
	if err != nil || !filepath.IsLocal(relative) || relative == "." {
		return errors.New("the recycle archive is outside its owned directory")
	}
	return os.RemoveAll(target)
}

func (a *App) sweepChatCleanup() error {
	a.mu.Lock()
	records := map[string]chatRemoval{}
	for id, r := range a.settings.RemovedChats {
		if r.Group != "" {
			records[r.Group] = r
		} else if r.Stage == "removing" {
			records[id] = r
		}
	}
	a.mu.Unlock()
	var result error
	for group, r := range records {
		var err error
		switch r.Stage {
		case "removing":
			err = a.finishChatRemoval(group)
		case "retaining":
			err = a.finishRetainingChat(group)
		case "restoring":
			err = a.finishRestoringChat(group)
		case "restored_cleanup":
			err = a.finishRestoredCleanup(group)
		case "erasing":
			err = a.finishErasingChat(group)
		case "finalizing":
			_, err = a.finalizeChatCleanup(group)
		case "retained":
			if !a.cleanupNow().Before(r.Expires) {
				_, err = a.finalizeChatCleanup(group)
			}
		}
		result = errors.Join(result, err)
	}
	a.releaseUnusedCleanupProviders()
	return result
}

func (a *App) watchChatCleanup(ctx context.Context) {
	timer := time.NewTicker(time.Minute)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if err := a.reconcileChatRemovals(); err != nil {
				logf("chat", "error", "chat retention sweep: %v", err)
			}
		}
	}
}
