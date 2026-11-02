package main

import (
	"context"
	"strings"

	"studio/internal/bus"
)

// inboxMessage keeps captured aliases out of the readable chat-name heading.
func (a *App) inboxMessage(m bus.Message) BusMessage {
	out := busMessage(m)
	if out.FromPerson {
		return out
	}
	out.From = strings.TrimSpace(m.FromTitle)
	if out.From != "" || m.FromSession == "" {
		return out
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for native, recipient := range a.settings.BusSessions {
		if recipient != m.FromSession {
			continue
		}
		title := strings.TrimSpace(a.settings.SessionTitles[native])
		if title == "" {
			continue
		}
		if out.From != "" && out.From != title {
			// Conflicting saved owners do not authorize choosing one by map order.
			out.From = ""
			break
		}
		out.From = title
	}
	// A session still on the bus that has no title of its own is shown by its
	// display title, as the rest of the bus shows it; only a sender that has
	// left the bus is "no longer available".
	if out.From == "" && m.FromKnown {
		out.From = m.From
	}
	return out
}

func (a *App) inboxMessages(ms []bus.Message) []BusMessage {
	out := make([]BusMessage, 0, len(ms))
	for _, m := range ms {
		out = append(out, a.inboxMessage(m))
	}
	return out
}

type BusInboxResult struct {
	Items      []BusMessage `json:"items"`
	Unread     int          `json:"unread"`
	NextBefore int64        `json:"nextBefore"`
	Newest     int64        `json:"newest"`
}

func (a *App) BusInboxPage(view string, before int64) (BusInboxResult, error) {
	var out BusInboxResult
	err := a.busHost().With(func(b *bus.Bus) error {
		page, err := b.HumanInboxPage(context.Background(), view, before, busInboxLimit)
		out = BusInboxResult{Items: a.inboxMessages(page.Items), Unread: page.Unread, NextBefore: page.NextBefore, Newest: page.Newest}
		return err
	})
	return out, err
}

func (a *App) BusInboxMessage(id int64) (BusMessage, error) {
	var out BusMessage
	err := a.busHost().With(func(b *bus.Bus) error {
		m, err := b.HumanInboxMessage(context.Background(), id)
		out = a.inboxMessage(m)
		return err
	})
	return out, err
}

func (a *App) changeBusInbox(id int64, through, dismiss bool) error {
	return a.busHost().With(func(b *bus.Bus) error {
		return b.ChangeHumanInbox(context.Background(), id, through, dismiss)
	})
}

func (a *App) BusMarkInboxRead(id int64) error {
	return a.changeBusInbox(id, false, false)
}

func (a *App) BusDismissInbox(id int64) error {
	return a.changeBusInbox(id, false, true)
}

func (a *App) BusMarkInboxReadThrough(maxID int64) error {
	return a.changeBusInbox(maxID, true, false)
}

func (a *App) BusDismissInboxThrough(maxID int64) error {
	return a.changeBusInbox(maxID, true, true)
}

func (a *App) BusGroupMessagesFrom(group string, fromID int64, contextCount int) ([]BusMessage, error) {
	group, err := groupName(group)
	if err != nil {
		return nil, err
	}
	var out []BusMessage
	err = a.busHost().With(func(b *bus.Bus) error {
		r, err := b.Room(context.Background(), group)
		if err != nil {
			return err
		}
		ms, err := b.RoomMessagesFrom(context.Background(), r.Name, fromID, contextCount)
		out = busMessages(ms)
		return err
	})
	return out, err
}
