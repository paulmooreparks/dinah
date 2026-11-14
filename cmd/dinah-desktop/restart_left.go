package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"

	"studio/internal/statefile"
)

func restartAwaitingPath() string { return filepath.Join(configDir(), "restart-awaiting.json") }
func writeRestartAwaiting(r pausedRecord) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return statefile.Write(restartAwaitingPath(), data)
}

// restartChatOpened consumes only a recorded restart's first open outcome.
// Messages stay on the bus when recovery needs the person's choice.
func (a *App) restartChatOpened(chat, cwd string, openErr error) {
	pausedMu.Lock()
	defer pausedMu.Unlock()
	data, err := statefile.ReadRegular(restartAwaitingPath())
	if err != nil {
		return
	}
	var r pausedRecord
	if json.Unmarshal(data, &r) != nil {
		return
	}
	for i, c := range r.Chats {
		if c.Chat != chat {
			continue
		}
		if openErr != nil {
			if cwd == "" {
				cwd = c.Cwd
			}
			if cwd == "" {
				if tr, e := readTranscript(chat); e == nil && tr != nil {
					cwd = tr.Workspace
				}
			}
			if err = a.saveRestartLeft(r.Run, BusChatRef{ID: c.Chat, Cwd: cwd, Title: c.Title}); err != nil {
				logf("core", "error", "saving restart notification: %v", err)
				return
			}
			logf("core", "warn", "restart left chat %q waiting for the person: %v", c.Title, openErr)
		}
		r.Chats = append(r.Chats[:i], r.Chats[i+1:]...)
		if err = writeRestartAwaiting(r); err != nil {
			logf("core", "error", "saving restart continuation outcome: %v", err)
		}
		return
	}
}

func (a *App) saveRestartLeft(run string, chat BusChatRef) error {
	notificationStoreMu.Lock()
	changed := false
	defer func() {
		notificationStoreMu.Unlock()
		if changed {
			a.emit("notification:delivery", nil)
		}
	}()
	store, err := readNotifications()
	if err != nil {
		return err
	}
	id := fmt.Sprintf("restart-left-%x", sha256.Sum256([]byte(run)))
	item := Notification{ID: id, Kind: "restart-left", Severity: "warning", Title: "Chats need your help after restart", Body: "These chats could not reopen. Their carry-on messages are waiting. Open each chat to review its recovery options.", Created: notificationNow().UTC()}
	index := -1
	for i, n := range store.Items {
		if n.ID == id {
			item = n
			index = i
			break
		}
	}
	for _, c := range item.Chats {
		if c.ID == chat.ID {
			return nil
		}
	}
	item.Chats = append(item.Chats, chat)
	item.Dismissed = false
	if err = validateNotification(item); err != nil {
		return err
	}
	if index < 0 {
		store.Items = append(store.Items, item)
	} else {
		store.Items[index] = item
	}
	if err = writeNotifications(store); err != nil {
		return err
	}
	changed = true
	return nil
}
