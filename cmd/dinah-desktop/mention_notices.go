package main

import (
	"context"
	"fmt"
	"strings"
	"time"
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
// those whose message the person has answered. The bus watcher calls it each time
// the bus has changed, and it runs nowhere else, so two passes never overlap.
func (a *App) noticeMentions(ctx context.Context, b *bus.Bus) {
	baseline, err := a.mentionBaseline(ctx, b)
	if err != nil {
		logf("bus", "error", "finding where the mentions of the person begin: %v", err)
		return
	}
	a.mu.Lock()
	readSafe := a.settings.AttentionReadSafe
	a.mu.Unlock()
	// Older builds used a fixed baseline and pruned dismissed records. Import
	// only their unread backlog so historical acknowledgements do not return.
	migrationTop, topErr := b.HighestMessage(ctx)
	if topErr != nil {
		return
	}
	var mentions []bus.Message
	if readSafe {
		mentions, err = b.AttentionMentions(ctx, baseline)
	} else {
		mentions, err = b.Mentions(ctx, baseline)
	}
	if err != nil {
		logf("bus", "error", "looking for chats that mention the person: %v", err)
		return
	}
	highest := baseline
	complete := true
	for _, m := range mentions {
		item := mentionCard(m)
		item.SessionID = a.chatOfBus(m.FromSession)
		saved, created, err := a.saveNotification(item)
		if err != nil {
			logf("bus", "error", "making a card for message %d: %v", m.ID, err)
			complete = false
			break
		}
		highest = m.ID
		if !created {
			continue
		}
		a.emit("notifications:added", saved)
		a.notices.show(saved.Title, saved.Body, true, a.notifyWhenNeeded())
	}
	if !readSafe && complete {
		highest = max(highest, migrationTop)
	}
	if highest > baseline || !readSafe && complete {
		if err := a.updateSettings(func(s *Settings) error {
			s.MentionBaseline = &highest
			s.AttentionReadSafe = readSafe || complete
			return nil
		}); err != nil {
			logf("bus", "error", "keeping mention attention watermark: %v", err)
		}
	}
	a.dismissAnsweredMentions(ctx, b)
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
	at, _ := time.Parse(time.RFC3339Nano, m.CreatedAt)
	n := Notification{ID: fmt.Sprintf("mention-%d", m.ID), Kind: "mention", Severity: "info", Title: title, Body: cutText(strings.Split(strings.TrimSpace(m.Text), "\n")[0], mentionBodyLimit),
		SessionTitle: from, SourceID: m.FromSession, Group: m.Room, GroupTitle: m.RoomTitle, Message: m.ID, Created: at}
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

// dismissAnsweredMentions ends only requests with an actual reply by the person.
// Delivery read receipts cannot prove that an attention card was acknowledged.
func (a *App) dismissAnsweredMentions(ctx context.Context, b *bus.Bus) {
	notificationStoreMu.Lock()
	store, err := readNotifications()
	notificationStoreMu.Unlock()
	if err != nil {
		return
	}
	var ids []int64
	for _, item := range store.Items {
		if (item.Kind == "mention" || item.Kind == "relay") && !item.Dismissed {
			ids = append(ids, item.Message)
		}
	}
	read, err := b.MentionsAnswered(ctx, ids)
	if err != nil {
		logf("bus", "error", "looking for mentions the person has answered: %v", err)
		return
	}
	for _, id := range read {
		for _, item := range store.Items {
			if item.Message != id || item.Dismissed || (item.Kind != "mention" && item.Kind != "relay") {
				continue
			}
			if err := a.DismissNotification(item.ID); err != nil {
				continue
			}
			a.emit("notifications:dismissed", map[string]string{"id": item.ID})
		}
	}
}

// mentionBaseline starts at the newest message when a window first watches.
// Successful capture advances it, preventing old dismissed cards from returning
// after their tombstones expire. Older fixed baselines migrate unread items only.
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
			s.AttentionReadSafe = true
		}
		return nil
	}); err != nil {
		return 0, err
	}
	return highest, nil
}
