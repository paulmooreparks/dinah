package main

import (
	"context"
	"errors"
	"slices"
	"time"

	"studio/internal/bus"
)

type chatRemoval struct {
	Recipient string    `json:"recipient,omitempty"`
	Stage     string    `json:"stage"`
	Created   time.Time `json:"created"`
}

func (a *App) removedChat(id string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, ok := a.settings.RemovedChats[id]
	return ok
}

// RemoveChat removes precisely the supplied chat from this application. The
// native provider copy is unchanged; DeleteSession performs provider deletion.
func (a *App) RemoveChat(id string) error {
	if id == "" {
		return errors.New("choose the chat to remove")
	}
	if _, running := a.transferRunning.Load(id); running {
		return errors.New("wait for this chat's recovery operation to finish before removing it")
	}
	if _, _, active := a.busCoordinator().CurrentDeliveryAttempt(id); active {
		return errors.New("wait for this chat's current opening to finish before removing it")
	}
	if err := a.updateSettings(func(s *Settings) error {
		if _, ok := s.RemovedChats[id]; ok {
			return nil
		}
		recipient := s.BusSessions[id]
		if recipient != "" {
			for other, owner := range s.BusSessions {
				if other != id && owner == recipient {
					return errors.New("this recipient has another chat owner; removal cannot discard a shared queue")
				}
			}
		}
		if s.RemovedChats == nil {
			s.RemovedChats = map[string]chatRemoval{}
		}
		s.RemovedChats[id] = chatRemoval{Recipient: recipient, Stage: "removing", Created: time.Now().UTC()}
		if !slices.Contains(s.Hidden, id) {
			s.Hidden = append(s.Hidden, id)
		}
		return nil
	}); err != nil {
		return err
	}
	return a.finishChatRemoval(id)
}

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
	a.mu.Lock()
	ids := []string{}
	for id, r := range a.settings.RemovedChats {
		if r.Stage != "removed" {
			ids = append(ids, id)
		}
	}
	a.mu.Unlock()
	var result error
	for _, id := range ids {
		result = errors.Join(result, a.finishChatRemoval(id))
	}
	return result
}
