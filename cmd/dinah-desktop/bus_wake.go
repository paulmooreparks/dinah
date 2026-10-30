package main

import (
	"studio/internal/applet/broker"
	"studio/internal/bushost"
	"studio/internal/sessionhost"
	"time"
)

const (
	backgroundSlots = bushost.BackgroundSlots
	backgroundIdle  = sessionhost.BackgroundIdle
	connectPatience = bushost.ConnectPatience
)

func (a *App) wakeClock() time.Time {
	if a.wakeNow != nil {
		return a.wakeNow()
	}
	return time.Now()
}

// ChatWindows reports the chats whose windows are open.
func (a *App) ChatWindows(ids []string) { a.busCoordinator().ChatWindows(ids) }

// backgroundUsedLocked requires the shared App.mu.
func (a *App) backgroundUsedLocked() int { return a.runtime.BackgroundUsedLocked() }

func (a *App) busCoordinator() *sessionhost.BusChats {
	a.busChatsOnce.Do(func() {
		a.busChatsService = sessionhost.NewBusChats(&a.mu, &a.runtime, sessionhost.BusChatsHost{
			BusService:       a.busHost,
			ResultBroker:     func() (*broker.Broker, error) { return a.appletHostService().Broker() },
			SavedBusIdentity: func(id string) string { a.mu.Lock(); defer a.mu.Unlock(); return a.settings.BusSessions[id] },
			TurnIO:           a.turnIO, AgentPerson: a.agentPerson, CanSteer: a.CanSteer,
			BusPictures: a.busPictures, PreparePrompt: a.promptFrom, AskPrompt: askPrompt,
			EnsureBusChat: func(target sessionhost.BusAskTarget) (string, error) {
				return a.busChat(BusChatRef{ID: target.SessionID, Cwd: target.Cwd, Title: target.Title})
			},
			EndClosed:    func() { a.closedSessions().EndClosed() },
			HoldingTurns: a.holdingTurns, ChatOfBus: a.chatOfBus, HiddenLocked: a.hidden,
			Clock: a.wakeClock, AllocateBusIdentity: a.busSessionID, SaveBusIdentity: a.saveBusSession,
			ShownTitle: a.shownTitle, Emit: a.emit,
			OpenChat: func(chat, cwd string) error {
				_, err := a.OpenSessionAt(cwd, chat)
				return err
			},
			ConnectGrace:  a.connectGrace(),
			ConnectFailed: a.relayDeliveryFailed,
			Log:           func(level, format string, args ...any) { logf("bus", level, format, args...) },
		})
	})
	return a.busChatsService
}

// connectGraceDefault is how long the page is given to open a chat it was
// asked to connect before the engine opens it.
const connectGraceDefault = 5 * time.Second

// connectGrace is the grace in force, which a test shortens.
func (a *App) connectGrace() time.Duration {
	switch {
	case a.connectGraceOverride < 0:
		return 0
	case a.connectGraceOverride > 0:
		return a.connectGraceOverride
	}
	return connectGraceDefault
}
