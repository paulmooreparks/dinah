package main

import (
	"studio/internal/sessionhost"
)

// Subagents are the workers an agent starts while it handles a prompt. An
// agent that implements the ACP subagent RFD (the draft of 31 August 2026,
// agentclientprotocol/agent-client-protocol PR 1992) reports each one as a
// child session: subagent_spawned on the immediate parent, which may itself
// be a subagent, then the child's own session updates under the child's
// session ID, then one subagent_state_update on the parent with completed,
// failed, cancelled or disconnected. The Claude adapter sends this draft to
// a client that declares nativeSubagentSessions among its AIR capabilities
// (acp.ClientCaps). Nothing here is specific to the Claude adapter: any
// agent that sends these updates is shown the same way, and an agent Dinah
// translates itself (Codex, Ollama) can report its workers in the same form.
//
// Dinah keeps the subagents on the chat they belong to, the root of their
// ancestry, so that the page can draw them as a tree, and keeps each one's
// latest updates in memory so that a window can show what it did. A
// subagent's updates are not the chat's conversation: they go into neither
// the chat's saved copy nor its log (Paul, 26 October 2026).
//
// Dinah keeps nothing of a subagent on disk itself. A chat opened again
// learns its earlier subagents from the agent, which replays them with the
// chat's history; those are marked Replayed and are the chat's earlier
// subagents, which the page lists apart from this run's (Paul, 28 October
// 2026). Dinah remembers, for the length of the run, which subagents it saw
// start, so that a chat closed and opened again in the same run keeps them
// as this run's.

// subagentUpdatesKept, subagentBytesKept and subagentBuffersKept bound what
// Dinah keeps of the subagents' updates: a subagent's latest 4,000 updates or
// 4 MiB of them, whichever is less, with the consecutive pieces of one
// message or thought joined into one update, and the updates of a chat's 64
// latest subagents. An older subagent keeps its place in the tree.
const (
	subagentUpdatesKept = sessionhost.SubagentUpdatesKept
	subagentBytesKept   = sessionhost.SubagentBytesKept
	subagentBuffersKept = sessionhost.SubagentBuffersKept
)

// The states of a subagent. Running is what spawning means; the others are
// terminal. Disconnected means the outcome is unknown.
const (
	subagentRunning      = "running"
	subagentDisconnected = "disconnected"
)

func terminalSubagentState(state string) bool {
	switch state {
	case "completed", "failed", "cancelled", subagentDisconnected:
		return true
	}
	return false
}

// SubagentInfo is what the page shows of one subagent.
type SubagentInfo = sessionhost.SubagentInfo

// SubagentRecord is a subagent with the updates Dinah keeps of it, for a
// window that shows its work.
type SubagentRecord = sessionhost.SubagentRecord

// runSubagent is when a subagent of this run started and ended.
type runSubagent = sessionhost.RunSubagent

// SubagentUpdates is a subagent with the updates Dinah keeps of it, for a
// window that shows its work, running or finished.
func (a *App) SubagentUpdates(id string) (SubagentRecord, error) {
	return a.subagentService().Updates(id)
}

// CancelSubagent stops one subagent and its descendants, with the
// session/cancel notification for its session, which the RFD allows only
// for a subagent whose announcement said it can be cancelled. The
// subagent's open questions are answered cancelled first, as the RFD
// requires; the agent then reports it cancelled.
func (a *App) CancelSubagent(id string) error {
	return a.subagentService().Cancel(id)
}

// subagentAsks is the page's description of the subagent behind a question
// or a file, or nil.
func subagentAsks(sub *SubagentInfo) map[string]string {
	if sub == nil {
		return nil
	}
	return map[string]string{"id": sub.ID, "name": sub.Name}
}
