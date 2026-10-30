package main

import (
	"strings"
	"unicode/utf8"
)

// Dinah appends its applet context to every prompt, and an agent that
// titles a session from its first prompt can take its title from that block
// instead of from what the person asked, as Devin did with
// "functions.applet_discover:0{"context": "dinah-applet-v1:...". Such a title
// shows part of a private context and says nothing about the chat, so it is
// not shown: the chat is titled from the person's first message instead.

// titleLimit is the longest title made from a message, in characters.
const titleLimit = 60

// fromAppletContext reports whether a title was made from the applet context.
func fromAppletContext(title string) bool {
	for _, mark := range []string{appletContextPrefix, strings.TrimSpace(appletContextHeading), "applet_discover", "applet_invoke", "applet_status", "applet_artifact", "dinah-applets"} {
		if strings.Contains(title, mark) {
			return true
		}
	}
	return false
}

// sessionTitle is the title to show for a session the agent titled title:
// that title, or, when it was made from the applet context, the first line of
// the person's first message, or nothing. The conversation of an open session
// is read as it stands, since the agent titles a chat during its first turn
// and the conversation is saved when a turn ends.
func (a *App) sessionTitle(id, title string) string {
	return a.withJudgedTitle(id, a.agentTitle(id, title))
}

// agentTitle is sessionTitle before a judged title is considered.
func (a *App) agentTitle(id, title string) string {
	if !fromAppletContext(title) {
		return title
	}
	a.mu.Lock()
	s := a.runtime.ConversationLocked(id)
	a.mu.Unlock()
	var messages []ConversationMessage
	if s != nil {
		a.cmu.Lock()
		if tr := s.HistorySavedLocked(); tr != nil {
			messages = append(messages, tr.Messages...)
		}
		a.cmu.Unlock()
	}
	if messages == nil {
		if tr, err := readTranscript(id); err == nil {
			messages = tr.Messages
		}
	}
	for _, m := range messages {
		if m.Role != "you" {
			continue
		}
		return titleOf(visibleConversationText(m.Text))
	}
	return ""
}

// titleOf is a title made from a message: its first line that has text,
// shortened at a word to at most titleLimit characters.
func titleOf(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" {
			continue
		}
		if utf8.RuneCountInString(line) <= titleLimit {
			return line
		}
		cut := string([]rune(line)[:titleLimit])
		if at := strings.LastIndex(cut, " "); at > titleLimit/2 {
			cut = cut[:at]
		}
		return strings.TrimRight(cut, " ,.;:") + "…"
	}
	return ""
}
