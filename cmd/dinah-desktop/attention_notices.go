package main

import (
	"crypto/rand"
	"encoding/json"
	"strings"
)

// Permission keys restart at one; the process epoch keeps historical alerts
// from being mistaken for a new process's permission with the same key.
var permissionNoticeEpoch = rand.Text()

func permissionNoticeID(key string) string { return "attention_" + permissionNoticeEpoch + "_" + key }

func (a *App) keepQuestion(session, key string, toolCall json.RawMessage) {
	a.mu.Lock()
	chat := a.settings.SessionTitles[session]
	if chat == "" {
		if s := a.runtime.ConversationLocked(session); s != nil {
			chat = s.Title()
		}
	}
	a.mu.Unlock()
	if chat == "" {
		chat = "A chat"
	}
	var call struct {
		Title string `json:"title"`
	}
	_ = json.Unmarshal(toolCall, &call)
	line := strings.Split(strings.TrimSpace(call.Title), "\n")[0]
	if line == "" {
		line = "The agent asks for your permission."
	}
	item := Notification{ID: permissionNoticeID(key), Kind: "attention", Severity: "info", Title: chat + " has a question for you", Body: cutText(line, mentionBodyLimit), SessionID: session, SessionTitle: chat, PermissionKey: key}
	saved, created, err := a.saveNotification(item)
	if err != nil {
		logf("core", "error", "keeping permission attention: %v", err)
	} else if created {
		a.emit("notifications:added", saved)
	}
	a.noticeQuestion(session, toolCall)
}

func (a *App) answerAttention(key string) {
	id := permissionNoticeID(key)
	if err := a.DismissNotification(id); err == nil {
		a.emit("notifications:dismissed", map[string]string{"id": id})
	}
}
