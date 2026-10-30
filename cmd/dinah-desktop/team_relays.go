package main

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"studio/internal/bus"
	"studio/internal/sessionhost"
)

// A team member's message to the person is rerouted to its lead on the bus (docs/plans/teams.md). The
// lead's choice not to relay is its own judgement. A relay that fails, because the delivery to the
// lead could not be made, the lead's turn on it ended in an error, or the lead cannot run, is
// marked on the bus with its reason, and the person is told directly, once, with a card in Notifications
// and an operating-system notification when the window is not the active one.

// relayTurns are the bus messages each chat's current turn was given, so that a turn that ends in an error
// can fail the relays among them.
var relayTurns = struct {
	sync.Mutex
	byChat map[string][]int64
}{byChat: map[string][]int64{}}

// noteRelayTurnStart remembers the bus messages a chat's turn was given.
func (a *App) noteRelayTurnStart(s *Session, p *prompt) {
	noteTurnResult(s.ID(), nil)
	var ids []int64
	for _, e := range p.Delivered() {
		if e.Delivered != nil && e.Delivered.ID != 0 {
			ids = append(ids, e.Delivered.ID)
		}
	}
	relayTurns.Lock()
	defer relayTurns.Unlock()
	if len(ids) == 0 {
		delete(relayTurns.byChat, s.ID())
		return
	}
	relayTurns.byChat[s.ID()] = ids
}

// relayTurnEnded fails the relays a chat's turn was given when the turn ended in an error.
func (a *App) relayTurnEnded(s *Session, err error) {
	noteTurnResult(s.ID(), err)
	a.recordTurnUsage(s)
	a.teamsChanged()
	relayTurns.Lock()
	ids := relayTurns.byChat[s.ID()]
	delete(relayTurns.byChat, s.ID())
	relayTurns.Unlock()
	if err == nil || len(ids) == 0 {
		return
	}
	if !a.teamsOpen() {
		return
	}
	busID := a.knownBusID(s.ID())
	if busID == "" {
		return
	}
	reason := fmt.Sprintf("%s's turn on it ended in an error: %v", a.relayLeadName(s.ID()), err)
	_ = a.withTeamBus(func(b *bus.Bus) error {
		_, ferr := b.FailRelaysIn(context.Background(), busID, ids, reason)
		return ferr
	})
}

// relayDeliveryFailed fails the relays waiting for a chat that could not be connected to receive them.
func (a *App) relayDeliveryFailed(chat string, err error) {
	if !a.teamsOpen() {
		return
	}
	busID := a.knownBusID(chat)
	if busID == "" {
		return
	}
	reason := fmt.Sprintf("%s could not be opened to receive it: %v", a.relayLeadName(chat), err)
	_ = a.withTeamBus(func(b *bus.Bus) error {
		_, ferr := b.FailPendingRelaysTo(context.Background(), busID, reason)
		return ferr
	})
}

// relayLeadName is how a reason calls a lead: its title, or "the lead".
func (a *App) relayLeadName(chat string) string {
	a.mu.Lock()
	title := a.settings.SessionTitles[chat]
	a.mu.Unlock()
	if title == "" {
		return "the lead"
	}
	return title
}

// leadCannotRun says why a lead cannot take a request now, or "" when it can: its context window is full
// and a send is held, its provider is not available, or the group's turn budget is used up.
func (a *App) leadCannotRun(b *bus.Bus, ctx context.Context, chat string, m bus.Message) string {
	name := a.relayLeadName(chat)
	if s := a.session(chat); s != nil {
		if s.Hold() != nil {
			return name + "'s context window is full and a send is held"
		}
		return ""
	}
	a.mu.Lock()
	provider := a.settings.SessionProviders[chat]
	a.mu.Unlock()
	if provider != "" {
		if info := a.providerInfoOf(provider); !info.Installed || !info.Available {
			why := info.StatusWords
			if why == "" {
				why = providerName(provider) + " is not available"
			}
			return name + " cannot run: " + why
		}
	}
	if m.Room != "" {
		if room, err := b.Room(ctx, m.Room); err == nil && room.Budget >= 0 && room.Used >= room.Budget {
			return "the group's turn budget is used up, so " + name + " is not asked to answer"
		}
	}
	return ""
}

// checkRelays marks failed the relays whose lead cannot run, and tells the person of every failed relay once. It
// runs with each pass over the bus, beside the mentions of the person.
func (a *App) checkRelays(ctx context.Context, b *bus.Bus) {
	pending, err := b.PendingRelays(ctx)
	if err != nil {
		logf("bus", "error", "looking for relays that wait for their lead: %v", err)
		return
	}
	for _, r := range pending {
		chat := a.chatOfBus(r.Lead)
		if chat == "" {
			continue
		}
		m, err := b.Message(ctx, r.MessageID)
		if err != nil {
			continue
		}
		if why := a.leadCannotRun(b, ctx, chat, m); why != "" {
			if err := b.FailRelay(ctx, r.MessageID, why); err != nil {
				logf("bus", "error", "marking relay %d failed: %v", r.MessageID, err)
			}
		}
	}
	failed, messages, err := b.UnreportedFailures(ctx)
	if err != nil {
		logf("bus", "error", "looking for relays that failed: %v", err)
		return
	}
	for i, r := range failed {
		card := relayCard(messages[i], r.Reason)
		card.SessionID = a.chatOfBus(messages[i].FromSession)
		card.Lead = a.chatOfBus(messages[i].RelayLead)
		saved, created, err := a.saveNotification(card)
		if err != nil {
			logf("bus", "error", "making a card for the failed relay of message %d: %v", r.MessageID, err)
			continue
		}
		if err := b.RelayReported(ctx, r.MessageID); err != nil {
			logf("bus", "error", "recording that the failed relay of message %d was reported: %v", r.MessageID, err)
		}
		if !created {
			continue
		}
		a.emit("notifications:added", saved)
		a.notices.show(saved.Title, saved.Body, true, a.notifyWhenNeeded())
	}
}

// relayCard is the report that a message for the person could not be relayed: the member's message and why.
func relayCard(m bus.Message, reason string) Notification {
	from := m.From
	if from == "" {
		from = m.FromName
	}
	lead := m.RelayLeadTitle
	if lead == "" {
		lead = "its lead"
	}
	title := from + " could not reach you through " + lead
	body := cutText(m.Text, mentionBodyLimit)
	n := Notification{ID: fmt.Sprintf("relay-%d", m.ID), Kind: "relay", Severity: "warning", Title: title, Body: body,
		SessionTitle: from, Group: m.Room, GroupTitle: m.RoomTitle, Message: m.ID,
		Details: []string{"It could not be relayed: " + strings.TrimSpace(reason)}}
	return n
}

// forgetTeam is called when a chat is deleted: the members it led become ordinary chats that reach the
// person directly, and the chat's own place in a team ends.
func (a *App) forgetTeam(chat string) {
	busID := a.knownBusID(chat)
	if busID == "" {
		return
	}
	_ = a.withTeamBus(func(b *bus.Bus) error {
		orphans, err := b.LeadRemoved(context.Background(), busID)
		if err != nil {
			logf("bus", "error", "ending the team of the deleted chat %s: %v", chat, err)
			return nil
		}
		if len(orphans) > 0 {
			logf("bus", "info", "the chat %s was deleted, so %d members it led now reach the person directly", chat, len(orphans))
		}
		return nil
	})
}

// recordTeamSends marks, in the history of the chat that sent it, each message a chat sent in a team's group chat and the bus took,
// with the group and the message's number, once for each message. A send that failed never reached the bus, so it is not marked and
// keeps its visible error; the marks are what a window folds a run of the team's traffic by, restored history included.
func (a *App) recordTeamSends(ctx context.Context, b *bus.Bus) {
	if !a.teamsActive.Load() {
		return
	}
	a.teamSendMu.Lock()
	defer a.teamSendMu.Unlock()
	if !a.teamSendSet {
		highest, err := b.HighestMessage(ctx)
		if err != nil {
			return
		}
		a.teamSendAfter, a.teamSendSet = highest, true
		return
	}
	sends, err := b.TeamSends(ctx, a.teamSendAfter)
	if err != nil {
		logf("bus", "error", "looking for the messages team chats sent: %v", err)
		return
	}
	for _, m := range sends {
		a.teamSendAfter = max(a.teamSendAfter, m.ID)
		chat := a.chatOfBus(m.FromSession)
		s := a.session(chat)
		if s == nil {
			continue
		}
		d := sessionhost.DeliveredMessage{ID: m.ID, Kind: m.Kind, From: m.From, FromSession: m.FromSession, Time: m.CreatedAt, Room: m.Room, RoomTitle: m.RoomTitle}
		a.transcriptCapture().RecordSent(s, d, m.Text)
	}
}
