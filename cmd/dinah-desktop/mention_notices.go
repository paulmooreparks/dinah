package main

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"studio/internal/bus"
)

// A message that mentions the person, which an agent does by putting paul in
// the mentions of its send or reply, makes a card in Notifications and, when
// the window is not the active one, a notification from the operating system.
// The window finds such messages in the bus's data, whichever program sent
// them (docs/plans/mention-notification.md).

// mentionBodyLimit is how much of a message its card shows.
const mentionBodyLimit = 400

// noticeMentions makes the cards for mentions that have none and dismisses
// those whose message the person has read. The bus watcher calls it each time
// the bus has changed, and it runs nowhere else, so two passes never overlap.
func (a *App) noticeMentions(ctx context.Context, b *bus.Bus) {
	baseline, err := a.mentionBaseline(ctx, b)
	if err != nil {
		logf("bus", "error", "finding where the mentions of the person begin: %v", err)
		return
	}
	mentions, err := b.Mentions(ctx, baseline)
	if err != nil {
		logf("bus", "error", "looking for chats that mention the person: %v", err)
		return
	}
	for _, m := range mentions {
		item := mentionCard(m)
		item.SessionID = a.chatOfBus(m.FromSession)
		saved, created, err := a.saveNotification(item)
		if err != nil {
			logf("bus", "error", "making a card for message %d: %v", m.ID, err)
			continue
		}
		if !created {
			continue
		}
		a.emit("notifications:added", saved)
		a.notices.show(saved.Title, saved.Body, true, a.notifyWhenNeeded())
	}
	a.dismissReadMentions(ctx, b)
	a.checkRelays(ctx, b)
	a.recordTeamSends(ctx, b)
}

// mentionCard is the report for a message that mentions the person.
func mentionCard(m bus.Message) Notification {
	from := m.From
	if from == "" {
		from = m.FromName
	}
	title := from + " sent you a message"
	if m.Room != "" {
		title = from + " mentioned you in " + m.RoomTitle
	}
	n := Notification{ID: fmt.Sprintf("mention-%d", m.ID), Kind: "mention", Severity: "info", Title: title, Body: cutText(m.Text, mentionBodyLimit),
		SessionTitle: from, Group: m.Room, GroupTitle: m.RoomTitle, Message: m.ID}
	if m.FromWorkspace != "" {
		n.Details = []string{"From the workspace " + m.FromWorkspace + "."}
	}
	return n
}

// cutText cuts text to at most limit characters, ending with an ellipsis when
// it cut anything, and not in the middle of a character.
func cutText(text string, limit int) string {
	text = strings.TrimSpace(text)
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	runes := []rune(text)
	return string(runes[:limit]) + "…"
}

// dismissReadMentions dismisses the cards of mentions the person has read, in
// the inbox or in the group's window. A card that is dismissed does not mark
// its message read.
func (a *App) dismissReadMentions(ctx context.Context, b *bus.Bus) {
	notificationStoreMu.Lock()
	store, err := readNotifications()
	notificationStoreMu.Unlock()
	if err != nil {
		return
	}
	var ids []int64
	for _, item := range store.Items {
		if item.Kind == "mention" && !item.Dismissed {
			ids = append(ids, item.Message)
		}
	}
	read, err := b.MentionsRead(ctx, ids)
	if err != nil {
		logf("bus", "error", "looking for mentions the person has read: %v", err)
		return
	}
	for _, id := range read {
		card := fmt.Sprintf("mention-%d", id)
		if err := a.DismissNotification(card); err != nil {
			continue
		}
		a.emit("notifications:dismissed", map[string]string{"id": card})
	}
}

// mentionBaseline is the number of the newest message on the bus when this
// window first looked for mentions. Only mentions after it become cards: the
// unread ones before it, when this build first runs, stay in the inbox as they
// are, so that a backlog does not become a heap of cards at once. It is kept
// in the window's settings.
func (a *App) mentionBaseline(ctx context.Context, b *bus.Bus) (int64, error) {
	a.mu.Lock()
	kept := a.settings.MentionBaseline
	a.mu.Unlock()
	if kept != nil {
		return *kept, nil
	}
	highest, err := b.HighestMessage(ctx)
	if err != nil {
		return 0, err
	}
	if err := a.updateSettings(func(s *Settings) error {
		if s.MentionBaseline == nil {
			s.MentionBaseline = &highest
		}
		return nil
	}); err != nil {
		return 0, err
	}
	return highest, nil
}
