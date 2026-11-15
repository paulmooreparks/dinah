package main

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"studio/internal/bus"
	"studio/internal/taskhost"
)

// repairRequestProjections derives notification closure from the authoritative
// source. It does not answer, dismiss or withdraw any request itself.
func (a *App) repairRequestProjections() error {
	notificationStoreMu.Lock()
	store, err := readNotifications()
	notificationStoreMu.Unlock()
	if err != nil {
		return err
	}
	closed := map[string]Notification{}
	for _, item := range store.Items {
		if item.Dismissed {
			continue
		}
		if strings.HasPrefix(item.PermissionKey, "task-q-") {
			source, err := taskhost.ReadQuestionRecord(configDir(), item.PermissionKey)
			if errors.Is(err, os.ErrNotExist) {
				continue // Older, unbound questions have no durable source here.
			}
			if err != nil {
				return err
			}
			if source.Status != "open" {
				closed[item.ID] = item
			}
			continue
		}
		if item.Kind != "mention" || item.Message == 0 {
			continue
		}
		if _, err := os.Stat(filepath.Join(configDir(), "bus.db")); errors.Is(err, os.ErrNotExist) {
			continue // Reading old local history must not create a bus store.
		} else if err != nil {
			return err
		}
		err := a.busHost().With(func(b *bus.Bus) error {
			source, err := b.DecisionForMessage(context.Background(), item.Message)
			if errors.Is(err, sql.ErrNoRows) {
				return nil // A pruned historical message grants no new authority.
			}
			if err != nil {
				return err
			}
			if source.Status != "open" {
				closed[item.ID] = item
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	if len(closed) == 0 {
		return nil
	}
	notificationStoreMu.Lock()
	current, err := readNotifications()
	changed := []string{}
	if err == nil {
		for i := range current.Items {
			item := &current.Items[i]
			original, ok := closed[item.ID]
			if ok && !item.Dismissed && item.Message == original.Message && item.PermissionKey == original.PermissionKey && item.Kind == original.Kind {
				item.Dismissed = true
				changed = append(changed, item.ID)
			}
		}
		if len(changed) != 0 {
			err = writeNotifications(current)
		}
	}
	notificationStoreMu.Unlock()
	if err != nil {
		return err
	}
	for _, id := range changed {
		a.emit("notifications:dismissed", map[string]string{"id": id})
	}
	return nil
}
