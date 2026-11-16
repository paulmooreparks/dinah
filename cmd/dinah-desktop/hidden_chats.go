package main

import (
	"sort"

	"studio/internal/sessionhost"
)

// Builds before the Recycle Bin had no way to delete a chat of a program
// without a documented deletion, so the person's Delete only put the chat on
// the hidden list. Such a chat is in no list, and yet it kept its saved
// conversation and its bus recipient, and a message could still wake it. The
// person decided on 16 November 2026 that these chats go to the Recycle Bin,
// where they are deleted for good after 30 days unless restored.

// hiddenChatsToRemove are the hidden sessions that are a chat of their own:
// on the hidden list, kept out of the lists by it (hiddenChats), the newest
// stage of their chat, with a saved conversation or a bus recipient, and
// without a removal record. The target of a switch that did not finish is not
// one of them: it goes with the chat it was made for.
func (a *App) hiddenChatsToRemove() ([]string, error) {
	a.mu.Lock()
	settings, err := a.settings.clone()
	a.mu.Unlock()
	if err != nil {
		return nil, err
	}
	_, headers := sessionhost.ConversationLinks(configDir())
	unfinished := unfinishedTargets(headers, settings)
	notListed := hiddenChats(headers, settings)
	bindings := map[string]string{}
	for source, target := range settings.ChatBindings {
		if tr, ok := settings.ChatTransfers[source]; ok && tr.Stage != "committed" {
			continue
		}
		bindings[source] = target
	}
	seen := map[string]bool{}
	out := []string{}
	for _, id := range settings.Hidden {
		if id == "" || seen[id] || sessionhost.NotChat(id) {
			continue
		}
		seen[id] = true
		if _, removed := settings.RemovedChats[id]; removed || unfinished[id] || !notListed(id) {
			continue
		}
		header := headers[id]
		if header != nil && header.Deleted {
			continue
		}
		if header == nil && settings.BusSessions[id] == "" {
			continue
		}
		if sessionhost.NewestStage(id, headers, bindings, unfinished) != id {
			continue
		}
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}

// removeHiddenChats moves each of them to the Recycle Bin, as the person's
// Delete does today. One that cannot be removed is left and said so.
func (a *App) removeHiddenChats() error {
	ids, err := a.hiddenChatsToRemove()
	if err != nil {
		return err
	}
	for _, id := range ids {
		title := a.sessionTitle(id, "")
		if err := a.RemoveChat(id); err != nil {
			logf("chat", "warning", "the hidden chat %s (%s) was left: %v", id, title, err)
			continue
		}
		logf("chat", "warning", "moved the hidden chat %s (%s) to the Recycle Bin", id, title)
	}
	return nil
}
