package main

import (
	"context"
	"errors"
	"os"

	"studio/internal/sessionhost"
)

func (a *App) codexDeletionGuard(ctx context.Context, root string, descendants []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	a.mu.Lock()
	snapshot, err := a.settings.clone()
	a.mu.Unlock()
	if err != nil {
		return err
	}
	known, err := codexDeletionTranscriptIDs(configDir(), descendants)
	if err != nil {
		return err
	}
	return codexDeletionOwnership(snapshot, known, root, descendants)
}

func codexDeletionOwnership(s Settings, transcriptIDs map[string]bool, root string, descendants []string) error {
	owned := map[string]bool{root: true}
	members, err := logicalChatMembersFromSettings(s, root)
	if err != nil {
		return err
	}
	for _, member := range members.Members {
		owned[member.SessionID] = true
	}
	for _, id := range descendants {
		if owned[id] {
			continue
		}
		known := transcriptIDs[id] || s.SessionTitles[id] != "" || s.BusSessions[id] != ""
		known = known || codexDeletionRecordedReference(s, id)
		if known {
			return errors.New("Codex keeps this copy because a spawned descendant is an independently owned chat")
		}
	}
	return nil
}

// Transfer metadata establishes local membership without joining separate recipients.
func codexDeletionRecordedReference(s Settings, id string) bool {
	if _, ok := s.RemovedChats[id]; ok {
		return true
	}
	for source, target := range s.ChatBindings {
		if source == id || target == id {
			return true
		}
	}
	for source, tr := range s.ChatTransfers {
		if source == id || tr.Source == id || tr.Target == id {
			return true
		}
	}
	for _, tr := range s.ChatTransferHistory {
		if tr.Source == id || tr.Target == id {
			return true
		}
	}
	return false
}

// A transcript path is local identity evidence even when its contents are damaged.
func codexDeletionTranscriptIDs(stateDir string, descendants []string) (map[string]bool, error) {
	known := map[string]bool{}
	for _, id := range descendants {
		_, err := os.Lstat(sessionhost.TranscriptPath(stateDir, id))
		if err == nil {
			known[id] = true
		} else if !os.IsNotExist(err) {
			return nil, errors.New("Codex deletion cannot verify local transcript ownership")
		}
	}
	return known, nil
}
