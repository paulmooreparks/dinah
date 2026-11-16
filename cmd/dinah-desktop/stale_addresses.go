package main

import (
	"context"
	"errors"
	"sort"

	"studio/internal/bus"
	"studio/internal/sessionhost"
)

// Stale addresses are bus recipients that belong to no chat any more. Builds
// before a switch moved a chat's recipient to its new session, and before a
// removal took every stage of a chat, left them behind: each earlier stage of
// a switched chat kept a recipient of its own, and a chat that an older build
// deleted kept its recipient. They share their titles with the live chats, so
// a chat could not be addressed by its title, and a message sent to one of
// them waited for a session that would never read it.
//
// A recipient is stale only when every session that owns it is one of these:
//
//   - an earlier stage of a listed chat, whose newest stage has a recipient
//     of its own;
//   - a session whose saved conversation is marked deleted, or whose removal
//     is finished for good;
//   - a session of a chat whose newest stage is one of those.
//
// A recipient that a listed chat's newest stage owns, that an open chat is
// joined to, or that a hidden chat with a saved conversation owns is never
// stale. A recipient that no chat in the settings owns is left alone: such a
// session was registered from outside the window.

type staleAddress struct {
	Recipient string   `json:"recipient"`
	Title     string   `json:"title"`
	Workspace string   `json:"workspace"`
	Reason    string   `json:"reason"`
	Sessions  []string `json:"sessions"`
}

func (a *App) staleAddresses() ([]staleAddress, error) {
	inventory, err := a.savedChatsReadAgain(nil)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	settings, err := a.settings.clone()
	joined := map[string]bool{}
	for _, chat := range a.runtime.ConversationsLocked() {
		if id := chat.BusID(); id != "" {
			joined[id] = true
		}
	}
	a.mu.Unlock()
	if err != nil {
		return nil, err
	}
	_, headers := sessionhost.ConversationLinks(configDir())
	bindings := map[string]string{}
	for source, target := range settings.ChatBindings {
		if tr, ok := settings.ChatTransfers[source]; ok && tr.Stage != "committed" {
			continue
		}
		bindings[source] = target
	}
	unfinished := unfinishedTargets(headers, settings)
	// gone says that a session is of a chat the person deleted for good.
	gone := func(id string) bool {
		if record, ok := settings.RemovedChats[id]; ok {
			return record.Stage == "removed" || record.Stage == "deleted"
		}
		header := headers[id]
		return header != nil && header.Deleted
	}
	live := map[string]bool{}
	earlier := map[string]string{}
	for _, chat := range inventory.Chats {
		if recipient := settings.BusSessions[chat.ID]; recipient != "" {
			live[recipient] = true
		}
		members, err := chatWithEarlierStages(settings, headers, chat.ID)
		if err != nil {
			continue
		}
		for _, member := range members.Members {
			if member.SessionID != chat.ID {
				earlier[member.SessionID] = chat.Title
			}
		}
	}
	owners := map[string][]string{}
	for session, recipient := range settings.BusSessions {
		if recipient != "" {
			owners[recipient] = append(owners[recipient], session)
		}
	}
	out := []staleAddress{}
	err = a.busHost().With(func(b *bus.Bus) error {
		for recipient, sessions := range owners {
			if live[recipient] || joined[recipient] {
				continue
			}
			sort.Strings(sessions)
			reason := ""
			for _, session := range sessions {
				why := ""
				if title, ok := earlier[session]; ok {
					why = "It is an earlier stage of the chat " + title + ", which has its own address."
				} else if gone(session) {
					why = "Its chat was deleted."
				} else if newest := sessionhost.NewestStage(session, headers, bindings, unfinished); newest != session && gone(newest) {
					why = "It is an earlier stage of a chat that was deleted."
				}
				if why == "" {
					reason = ""
					break
				}
				if reason == "" {
					reason = why
				}
			}
			if reason == "" {
				continue
			}
			available, err := b.RecipientAvailable(context.Background(), recipient)
			if err != nil {
				return err
			}
			if !available {
				continue
			}
			row, err := b.Session(context.Background(), recipient)
			if errors.Is(err, bus.ErrNotFound) {
				continue
			}
			if err != nil {
				return err
			}
			out = append(out, staleAddress{Recipient: recipient, Title: row.Title, Workspace: row.WorkspaceName, Reason: reason, Sessions: sessions})
		}
		return nil
	})
	sort.Slice(out, func(i, j int) bool {
		if out[i].Title != out[j].Title {
			return out[i].Title < out[j].Title
		}
		return out[i].Recipient < out[j].Recipient
	})
	return out, err
}

// retireStaleAddresses withdraws each stale address the way a chat's removal
// withdraws its own (bus.BinRecipient): the address takes no new message and
// is in no list, its queue is kept, and bus.RestoreRecipient brings it back.
// An address that still leads a team with members is left, and said so.
func (a *App) retireStaleAddresses() error {
	stale, err := a.staleAddresses()
	if err != nil {
		return err
	}
	var result error
	for _, address := range stale {
		err := a.busHost().With(func(b *bus.Bus) error { return b.BinRecipient(context.Background(), address.Recipient) })
		var lead *bus.TeamLeadRefusal
		switch {
		case errors.As(err, &lead):
			logf("bus", "warning", "the stale address %s (%s) was left: %v", address.Recipient, address.Title, err)
		case err != nil:
			result = errors.Join(result, err)
		default:
			logf("bus", "warning", "retired the stale address %s (%s): %s", address.Recipient, address.Title, address.Reason)
		}
	}
	return result
}
