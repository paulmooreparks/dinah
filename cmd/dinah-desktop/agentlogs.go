package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"studio/internal/sessionhost"
	"sync"
	"time"

	"studio/internal/chatlog"
	"studio/internal/rotlog"
)

// The logs of the agents' work: a readable log of
// each chat in logs/chats, and the protocol trace, which is kept across
// starts of Devin and rotated. Both are written through package rotlog, so
// each file is buffered and has a size limit, and old chat logs are pruned
// when Dinah starts.

// How much the logs keep.
const (
	traceMaxSize   = 32 << 20
	traceKeep      = 4
	chatLogsMaxAge = 30 * 24 * time.Hour
	chatLogsMax    = 512 << 20
)

type agentLogs struct {
	once  sync.Once
	chats *chatlog.Book
	trace *rotlog.Writer
}

func (a *App) logs() *agentLogs {
	a.agentLogs.once.Do(func() {
		dir := logDir()
		a.agentLogs.chats = chatlog.New(filepath.Join(dir, "chats"), redactAppletContexts)
		a.agentLogs.trace = rotlog.Open(acpTracePath(), rotlog.Options{MaxSize: traceMaxSize, Keep: traceKeep})
	})
	return &a.agentLogs
}

func (a *App) chatLogs() *chatlog.Book { return a.logs().chats }

// traceWriter is the protocol trace, one for the life of the app, so that a
// restart of Devin adds to it rather than erasing what came before.
func (a *App) traceWriter() *rotlog.Writer {
	t := a.logs().trace
	fmt.Fprintf(t, "=== %s %s %s starts Devin\n", time.Now().Format(time.RFC3339), product.Name, appVersion)
	return t
}

// pruneAgentLogs removes chat logs older than a month, and the oldest beyond
// what the folder may hold.
func pruneAgentLogs() {
	if n := rotlog.Prune(filepath.Join(logDir(), "chats"), chatLogsMaxAge, chatLogsMax); n > 0 {
		logf("core", "info", "removed %d old chat logs", n)
	}
}

// closeAgentLogs writes and closes every log, as the app ends.
func (a *App) closeAgentLogs() {
	l := a.logs()
	l.chats.CloseAll()
	_ = l.trace.Close()
	closeAppLog()
	closeFolder()
	a.closeTeams()
}

// promptText is what a prompt asked, as the chat log shows it: its words and
// the text of what was attached, without what Dinah adds for itself.
func promptText(p *prompt) string { return sessionhost.PromptText(p, visibleConversationText) }

// ChatLogPath is the file of a chat's log.
func (a *App) ChatLogPath(sessionID string) string { return a.chatLogs().Path(sessionID) }

// ShowChatLog shows a chat's log in the system's file manager.
func (a *App) ShowChatLog(sessionID string) error {
	if sessionID == "" {
		return errors.New("choose a chat first")
	}
	a.chatLogs().Flush()
	p := a.chatLogs().Path(sessionID)
	if _, err := os.Stat(p); err != nil {
		return errors.New("this chat has no log yet; its log begins with its next message")
	}
	return showInFolder(p)
}
