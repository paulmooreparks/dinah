package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"studio/internal/sessionhost"
)

// A chat's queued messages are kept on disk with the chat, so that a restart or a
// crash picks them up again (docs/plans/ollama-context-window.md, Queued Messages
// Are Kept). The files are in the state folder; sessionhost keeps and restores them.

// newQueueStore keeps the queue files in the state folder.
func newQueueStore() *sessionhost.QueueStore {
	q := sessionhost.NewQueueStore(filepath.Join(configDir(), "queues"))
	q.Logf = func(format string, args ...any) { logf("chat", "warning", format, args...) }
	return q
}

// queueRestorer is what puts kept queues back.
func (a *App) queueRestorer() *sessionhost.QueueRestorer {
	a.queueRestorerMu.Lock()
	defer a.queueRestorerMu.Unlock()
	if a.queueRestorerValue == nil {
		a.queueRestorerValue = sessionhost.NewQueueRestorer(a.queueStore, sessionhost.QueueRestoreHost{
			Head: func(id string) string {
				head, err := a.continuations().Resolve(id)
				if err != nil {
					return ""
				}
				return head
			},
			Deleted: func(id string) bool {
				tr, err := readTranscript(id)
				return err == nil && tr.Deleted
			},
			Started: func(chat, queueID string) bool {
				tr, err := readTranscript(chat)
				return err == nil && slices.Contains(tr.StartedQueueIDs, queueID)
			},
			Session: func(id string) *Session { return a.session(id) },
			Send:    func(id string, p sessionhost.Prompt) error { return a.turnIO().SendPrompt(id, p) },
			Discarded: func(id string, texts []string) {
				a.queueNotice(id, "Queued messages for a deleted chat were discarded",
					fmt.Sprintf("%d queued messages were waiting in %s, which was deleted while %s was not running. Their text is below, so you can copy it.", len(texts), a.queueChatName(id), product.Name), texts)
			},
			Damaged: func(id string, err error) {
				a.queueNotice(id, "Queued messages could not be read",
					fmt.Sprintf("The queued messages of %s could not be read, so they were set aside and the chat opened with none. Some messages may have been lost.", a.queueChatName(id)), nil)
			},
			Log: func(level, format string, args ...any) { logf("chat", level, format, args...) },
		})
	}
	return a.queueRestorerValue
}

// queueChatName is how a notice calls a chat: its title when there is one.
func (a *App) queueChatName(id string) string {
	a.mu.Lock()
	title := a.settings.SessionTitles[id]
	a.mu.Unlock()
	if title == "" {
		return "a chat"
	}
	return "the chat " + title
}

// queueNotice keeps a report of what became of a chat's queued messages in
// Notifications, once for each chat and time.
func (a *App) queueNotice(chat, title, body string, details []string) {
	sum := sha256.Sum256([]byte(chat))
	item := Notification{
		ID: "queue-" + hex.EncodeToString(sum[:6]) + "-" + fmt.Sprint(time.Now().Unix()), Kind: "app", Severity: "warning",
		Title: title, Body: body, Details: details,
	}
	saved, fresh, err := a.saveNotification(item)
	if err != nil {
		logf("chat", "warning", "the notice about the queued messages of chat %s could not be kept: %v", chat, strings.TrimSpace(err.Error()))
		return
	}
	if fresh {
		a.emit("notifications:added", saved)
	}
}
