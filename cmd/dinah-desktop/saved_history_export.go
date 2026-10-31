package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ShowSavedChatHistory reveals an independent snapshot of all available saved
// entries. It neither opens a provider nor changes the transcript or chat log.
func (a *App) ShowSavedChatHistory(sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return errors.New("choose a chat first")
	}
	view := a.SavedConversation(sessionID)
	if !view.Available {
		return errors.New("this chat has no available saved history")
	}
	p, err := writeSavedHistoryExport(filepath.Join(configDir(), "history-exports"), sessionID, view, time.Now())
	if err != nil {
		return err
	}
	return showInFolder(p)
}

func writeSavedHistoryExport(dir, id string, view SavedConversationView, at time.Time) (path string, err error) {
	if err = os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "saved-chat-*.md")
	if err != nil {
		return "", err
	}
	path = f.Name()
	defer func() {
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(path)
			path = ""
		}
	}()
	w := bufio.NewWriter(f)
	if err = renderSavedHistory(w, id, view, at); err == nil {
		err = w.Flush()
	}
	return path, err
}

func savedHistoryBlock(text string) string {
	// A delimiter longer than every backtick run preserves arbitrary saved text.
	longest, run := 0, 0
	for _, c := range text {
		if c == '`' {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	fence := strings.Repeat("`", max(3, longest+1))
	return fence + "\n" + text + "\n" + fence + "\n\n"
}

func renderSavedHistory(w io.Writer, id string, view SavedConversationView, at time.Time) error {
	if _, err := fmt.Fprintf(w, "# Saved chat history snapshot\n\nCaptured at %s. This file does not update with the running chat.\n\n%s", at.Format(time.RFC3339Nano), savedHistoryBlock(redactAppletContexts(id))); err != nil {
		return err
	}
	if !view.Complete || view.Warning != "" {
		warning := view.Warning
		if warning == "" {
			warning = "This saved history is incomplete."
		}
		if _, err := io.WriteString(w, "## Saved-history warning\n\n"+savedHistoryBlock(redactAppletContexts(warning))); err != nil {
			return err
		}
	}
	for i, message := range view.Messages {
		text := message.Text
		message.Text = ""
		metadata, err := json.MarshalIndent(message, "", "  ")
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "## Entry %d\n\nSaved metadata (text follows separately):\n\n%s%s", i+1, savedHistoryBlock(redactAppletContexts(string(metadata))), savedHistoryBlock(redactAppletContexts(text))); err != nil {
			return err
		}
	}
	return nil
}
