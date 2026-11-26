package main

import (
	"context"
	"strings"
	"time"

	"studio/internal/bus"
	"studio/internal/providers"
)

// Hook observations use the current Claude native session and its registered
// bus binding. Neither a provider switch nor another chat can lend it work.
func (a *App) observeChatHookWork(ctx context.Context, b *bus.Bus, s *Session) {
	row := providers.Registry[s.Provider()]
	st := s.State()
	if !row.SubagentHooks || row.SessionPrefix == "" || !strings.HasPrefix(s.ID(), row.SessionPrefix) || !st.Open || st.Parked {
		return
	}
	busID := s.BusID()
	if busID == "" {
		return
	}
	n, err := b.ObservedSubagentsRunning(ctx, busID, row.HookNamespace+strings.TrimPrefix(s.ID(), row.SessionPrefix))
	if err != nil {
		// A failed observation cannot establish that previously observed work ended.
		logf("bus", "error", "reading running subagent hooks: %v", err)
		return
	}
	if n < 0 {
		return
	}
	if s.ObserveHookSubagents(n) {
		a.chatStatesChanged()
	}
	if n == 0 && !s.Working() {
		a.subagentService().TurnEnded(s)
	}
}

func (a *App) observeHookWork(ctx context.Context, b *bus.Bus) {
	a.mu.Lock()
	chats := a.runtime.ConversationsLocked()
	a.mu.Unlock()
	for _, s := range chats {
		a.observeChatHookWork(ctx, b, s)
		st := s.State()
		if st.Ready && !st.Parked {
			if observer, ok := a.sessionConn(s).(interface{ ObserveBackgroundWork(string) }); ok {
				observer.ObserveBackgroundWork(s.ID())
			}
		}
	}
}

func (a *App) turnSubagentsEnded(s *Session) {
	if observer, ok := a.sessionConn(s).(interface {
		RefreshBackgroundWork(context.Context, string) error
	}); ok {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		if err := observer.RefreshBackgroundWork(ctx, s.ID()); err != nil {
			logf("agent", "error", "reading background work at turn end: %v", err)
		}
		cancel()
	}
	_, err := a.busHost().IfOpen(func(b *bus.Bus) error {
		a.observeChatHookWork(context.Background(), b, s)
		return nil
	})
	if err != nil {
		logf("bus", "error", "reading subagent hooks at turn end: %v", err)
	}
	a.subagentService().TurnEnded(s)
}
