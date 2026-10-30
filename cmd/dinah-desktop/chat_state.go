package main

import (
	"context"
	"strings"

	"studio/internal/bus"
	"studio/internal/sessionhost"
)

// A chat's state is worked out here and nowhere else (docs/plans/chat-states.md). It has a life, which says
// whether its session runs, and, only while it runs, an execution state with the reason for it as a full phrase.
// Whether the chat has a window is a separate fact. Every surface that shows a chat takes these fields as they
// are and works out neither the state nor the urgency of several chats.

// The types and the counting live in the session host, because a chat's information carries its state.
type (
	ChatExec    = sessionhost.ChatExec
	ChatState   = sessionhost.ChatState
	StateCounts = sessionhost.StateCounts
)

const (
	execStuck   = sessionhost.ExecStuck
	execWaiting = sessionhost.ExecWaiting
	execWorking = sessionhost.ExecWorking
	execIdle    = sessionhost.ExecIdle

	lifeOpen    = sessionhost.LifeOpen
	lifeClosed  = sessionhost.LifeClosed
	lifeRetired = sessionhost.LifeRetired
)

func countStates(states []ChatState) StateCounts { return sessionhost.CountStates(states) }

// chatState works out the state of one chat. chat is its ID in the window (empty for a session outside it), busID
// its session on the bus (empty when it has none), retired whether its team retired it, and outsideOpen whether a
// session outside the window is open on the bus. b is the bus, or nil when the caller has none to give, in which
// case a message waiting on a lead's relay is not seen.
func (a *App) chatState(ctx context.Context, b *bus.Bus, busID, chat string, retired, outsideOpen bool) ChatState {
	if retired {
		return ChatState{Life: lifeRetired}
	}
	s := a.session(chat)
	if chat == "" || s == nil {
		if chat == "" && outsideOpen {
			return ChatState{Life: lifeOpen, Exec: &ChatExec{State: execIdle, Why: "Activity unavailable outside this window"}}
		}
		return ChatState{Life: lifeClosed}
	}
	st := s.State()
	if !st.Open || st.Parked {
		return ChatState{Life: lifeClosed}
	}
	return ChatState{Life: lifeOpen, Window: !st.InBackground, Exec: a.execOf(ctx, b, busID, chat, st)}
}

// execOf is what an open chat is doing. Stuck is a failure: the last turn ended in an error, a switch failed, or the
// chat's agent could not start. Waiting is what only the person (or a lead's relay) can end: a full context, a
// permission or a question. Working is a running turn, and Idle is the rest.
func (a *App) execOf(ctx context.Context, b *bus.Bus, busID, chat string, st sessionhost.State) *ChatExec {
	info := st.Info
	switch {
	case turnFailure(chat) != "":
		return &ChatExec{State: execStuck, Why: reasonWith("Last turn failed", turnFailure(chat))}
	case info.Switch != nil && info.Switch.Failed != "":
		return &ChatExec{State: execStuck, Why: reasonWith("Switch failed", info.Switch.Failed)}
	}
	if failed, detail := a.providerFailed(info.Provider); failed {
		return &ChatExec{State: execStuck, Why: reasonWith("Agent could not start", detail)}
	}
	switch {
	case info.Hold != nil:
		return &ChatExec{State: execWaiting, Why: "Context full"}
	case a.permissions().HasPending(chat):
		return &ChatExec{State: execWaiting, Why: "Needs your answer or approval"}
	}
	if b != nil && busID != "" {
		if _, waiting, err := b.PendingRelayFrom(ctx, busID); err == nil && waiting {
			return &ChatExec{State: execWaiting, Why: "Needs lead to relay message"}
		}
	}
	if info.Working {
		return &ChatExec{State: execWorking, Why: "Turn in progress"}
	}
	return &ChatExec{State: execIdle, Why: "No work in progress"}
}

// reasonWith is a reason that carries a detail: the prefix, a colon and the detail as it was given, or the prefix alone
// when there is none. The surfaces wrap or cut it for their room; they are not told a shorter one.
func reasonWith(prefix, detail string) string {
	if strings.TrimSpace(detail) == "" {
		return prefix
	}
	return prefix + ": " + detail
}

// providerFailed says that a provider's agent failed to start, and the reason it gave.
func (a *App) providerFailed(provider string) (bool, string) {
	for _, p := range a.lifecycle().States() {
		if p.Provider == provider && p.State == "failed" {
			return true, p.Detail
		}
	}
	return false, ""
}
