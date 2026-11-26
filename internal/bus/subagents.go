package bus

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// The subagents an agent has running, as the agent program's hooks report
// them, so that what is meant for the agent is not handed to a subagent.
//
// A steer, and a message for a chat outside the window, is handed over by
// the PostToolUse hook as context for the agent's next step. The hook also
// fires for a subagent's tool calls, and what it returned went to whichever
// called next, which while a subagent works is the subagent (Paul, 19
// November 2026: a steering message reached the subagent and not the agent
// he was looking at).
//
// Claude Code documents a SubagentStart and a SubagentStop hook, each with
// the subagent's agent_id (https://code.claude.com/docs/en/hooks). They are
// kept here by the agent's own session, and while any is running nothing is
// handed over at PostToolUse. The agent takes what waits when its last
// subagent stops, or at its own Stop, which a subagent does not fire. An
// agent that started a subagent in the foreground is waiting for it and
// could not have acted on a steer sooner.
//
// A hook's input also says, on a tool call, whether a subagent made it. That
// is not used: these hooks are given their input by substitution, and what
// the substitution gives where the field is absent, as it is in the agent's
// own calls, is not documented.

const subagentSchema = `
CREATE TABLE IF NOT EXISTS agent_subagents (
	agent_session TEXT NOT NULL,
	agent_id      TEXT NOT NULL,
	started_at    TEXT NOT NULL,
	PRIMARY KEY (agent_session, agent_id)
);
`

// SubagentStaleAfter is how long a subagent is taken to be running when no
// stop was reported for it. A program that is killed reports none, and a
// subagent that is counted for good would hold every later steer until the
// end of its turn.
const SubagentStaleAfter = 6 * time.Hour

// usableAgentID says a hook gave a subagent's ID: not empty, and not a
// placeholder that was left as it was written.
func usableAgentID(id string) bool {
	id = strings.TrimSpace(id)
	return id != "" && !strings.Contains(id, "${")
}

// SubagentStarted records that an agent's subagent started, or started
// again. The agent is given by its own session, as its hooks give it.
func (b *Bus) SubagentStarted(ctx context.Context, agentSession, agentID string) error {
	if agentSession == "" || !usableAgentID(agentID) {
		return nil
	}
	return b.tx(ctx, func(t *sql.Tx) error {
		_, err := t.Exec(`INSERT INTO agent_subagents (agent_session, agent_id, started_at) VALUES (?, ?, ?)
			ON CONFLICT (agent_session, agent_id) DO UPDATE SET started_at = excluded.started_at`, agentSession, strings.TrimSpace(agentID), b.stamp())
		return err
	})
}

// SubagentStopped records that an agent's subagent stopped. A stop for a
// subagent that was never recorded, as the program's own internal agents
// report, changes nothing.
func (b *Bus) SubagentStopped(ctx context.Context, agentSession, agentID string) error {
	if agentSession == "" || !usableAgentID(agentID) {
		return nil
	}
	return b.tx(ctx, func(t *sql.Tx) error {
		_, err := t.Exec(`DELETE FROM agent_subagents WHERE agent_session = ? AND agent_id = ?`, agentSession, strings.TrimSpace(agentID))
		return err
	})
}

// SubagentsRunning is how many of an agent's subagents are running: those
// that started within SubagentStaleAfter and have not stopped.
func (b *Bus) SubagentsRunning(ctx context.Context, agentSession string) (int, error) {
	var n int
	since := b.now().Add(-SubagentStaleAfter).UTC().Format(time.RFC3339Nano)
	err := b.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM agent_subagents WHERE agent_session = ? AND started_at >= ?`, agentSession, since).Scan(&n)
	return n, err
}

// ObservedSubagentsRunning counts unmatched start hooks for this native session
// only when it belongs to the supplied chat. The steering timeout is not evidence
// that work finished, so this observation has no age cutoff.
func (b *Bus) ObservedSubagentsRunning(ctx context.Context, session, agentSession string) (int, error) {
	var n int
	err := b.db.QueryRowContext(ctx, `SELECT COUNT(s.agent_id) FROM agent_sessions a
		LEFT JOIN agent_subagents s ON a.agent_session = s.agent_session
		WHERE a.session = ? AND a.agent_session = ? GROUP BY a.agent_session`, session, agentSession).Scan(&n)
	if err == sql.ErrNoRows {
		return -1, nil // An unbound session has no authoritative observation yet.
	}
	return n, err
}

// ForgetSubagents forgets an agent's subagents, when its session ends.
func (b *Bus) ForgetSubagents(ctx context.Context, agentSession string) error {
	return b.tx(ctx, func(t *sql.Tx) error {
		_, err := t.Exec(`DELETE FROM agent_subagents WHERE agent_session = ?`, agentSession)
		return err
	})
}
