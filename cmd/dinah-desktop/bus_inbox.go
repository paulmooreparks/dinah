package main

import (
	"context"

	"studio/internal/bus"
)

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
		out = BusInboxResult{Items: busMessages(page.Items), Unread: page.Unread, NextBefore: page.NextBefore, Newest: page.Newest}
		return err
	})
	return out, err
}

func (a *App) BusInboxMessage(id int64) (BusMessage, error) {
	var out BusMessage
	err := a.busHost().With(func(b *bus.Bus) error {
		m, err := b.HumanInboxMessage(context.Background(), id)
		out = busMessage(m)
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
