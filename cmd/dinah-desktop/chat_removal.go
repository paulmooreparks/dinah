package main

import (
	"context"
	"time"

	"studio/internal/bus"
)

type chatRemoval struct {
	Group           string              `json:"group,omitempty"`
	Expires         time.Time           `json:"expires,omitempty"`
	Snapshot        *LogicalChatMembers `json:"snapshot,omitempty"`
	UnknownAttempts []string            `json:"unknownAttempts,omitempty"`
	Recipient       string              `json:"recipient,omitempty"`
	Stage           string              `json:"stage"`
	Created         time.Time           `json:"created"`
}

func (a *App) removedChat(id string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, ok := a.settings.RemovedChats[id]
	return ok
}

// ChatRemoved says whether a chat is in the Recycle Bin or deleted. The page
// asks when a window of its saved layout cannot be opened: a window of a chat
// that is gone is closed without a word, and any other failure is reported.
func (a *App) ChatRemoved(id string) bool { return a.removedChat(id) }

func (a *App) finishChatRemoval(id string) error {
	a.mu.Lock()
	record, ok := a.settings.RemovedChats[id]
	a.mu.Unlock()
	if !ok || record.Stage == "removed" {
		return nil
	}
	if err := a.busHost().With(func(b *bus.Bus) error { return b.RemoveRecipient(context.Background(), record.Recipient) }); err != nil {
		return err
	}
	if err := a.busCoordinator().RemoveRecoveryRecipient(id, record.Recipient); err != nil {
		return err
	}
	if err := a.transcriptCapture().Tombstone(id); err != nil {
		return err
	}
	a.forgetTeam(id)
	a.sessionsLifecycle().Forget(id)
	a.queueStore.Forget(id)
	if err := a.updateSettings(func(s *Settings) error {
		v := s.RemovedChats[id]
		v.Stage = "removed"
		s.RemovedChats[id] = v
		return nil
	}); err != nil {
		return err
	}
	a.emit("notification:delivery", nil)
	return nil
}

func (a *App) reconcileChatRemovals() error {
	a.chatCleanupMu.Lock()
	defer a.chatCleanupMu.Unlock()
	return a.sweepChatCleanup()
}
