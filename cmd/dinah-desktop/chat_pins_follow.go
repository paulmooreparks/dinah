package main

import (
	"studio/internal/sessionhost"
	"studio/internal/workspaces"
)

// A pin of a chat is on the chat's newest stage. The page knows a pin by the
// session it is on, so a pin left on an earlier stage showed the chat as not
// pinned, with no way to take the pin off, while Clean Up Chats refused to
// delete the chat until it was unpinned.

// repinChat moves the pins of a stage to the session that continues it. It is
// called in the settings write that commits a switch, so no saved state has
// the chat continued and its pin left behind.
func repinChat(s *Settings, from, to string) {
	if from == "" || to == "" || from == to {
		return
	}
	for i := range s.WorkspaceRecords {
		s.WorkspaceRecords[i].Pins = pinsMoved(s.WorkspaceRecords[i].Pins, func(session string) string {
			if session == from {
				return to
			}
			return session
		})
	}
}

// pinsFollowChats moves every chat pin that is on an earlier stage to the
// chat's newest stage (sessionhost.NewestStage). Switches made before pins
// moved at the commit left such pins. It reports whether a pin moved.
func pinsFollowChats(s *Settings, headers map[string]*sessionhost.TranscriptHead) bool {
	bindings := map[string]string{}
	for source, target := range s.ChatBindings {
		if tr, ok := s.ChatTransfers[source]; ok && tr.Stage != "committed" {
			continue
		}
		bindings[source] = target
	}
	unfinished := unfinishedTargets(headers, *s)
	moved := false
	for i := range s.WorkspaceRecords {
		s.WorkspaceRecords[i].Pins = pinsMoved(s.WorkspaceRecords[i].Pins, func(session string) string {
			newest := sessionhost.NewestStage(session, headers, bindings, unfinished)
			if newest == session {
				return session
			}
			// A pin stays where it is when the chat it would move to was
			// removed: the removal's own rules decide about its pins.
			if _, removed := s.RemovedChats[newest]; removed {
				return session
			}
			moved = true
			return newest
		})
	}
	return moved
}

// pinsMoved gives the pins with each chat pin on the session to gives for it,
// in their order, without a second pin of one chat.
func pinsMoved(pins []WorkspacePin, to func(string) string) []WorkspacePin {
	out := make([]WorkspacePin, 0, len(pins))
	seen := map[string]bool{}
	for _, pin := range pins {
		if pin.Kind == "chat" {
			pin.Session = to(pin.Session)
		}
		if pin.Kind == "chat" {
			key := workspaces.PinKey(pin)
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		out = append(out, pin)
	}
	return out
}

// repinContinuedChats is the start's pass over the saved pins.
func (a *App) repinContinuedChats() error {
	_, headers := sessionhost.ConversationLinks(configDir())
	return a.updateSettings(func(s *Settings) error {
		pinsFollowChats(s, headers)
		return nil
	})
}
