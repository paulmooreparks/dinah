package bus

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Steering a working agent without ending its turn. Dinah's window queues a
// steer for one of its chats, and the person's Claude Code hooks, which run
// in the chat's agent, hand it over: PostToolUse as context before the
// agent's next step, or Stop by continuing the turn. A hook knows only the
// agent's own session ID, so each prompt the window sends carries a marker
// with the chat's bus session (SteerMarker), and the UserPromptSubmit hook,
// whose documented input includes the prompt, ties the two together
// (BindAgentSession). A steer no hook takes is withdrawn by the window when
// the turn ends and sent as the next prompt instead.

// SteerMarker is the line a window prompt carries so its chat can be found
// from the agent's own session. The markers are written and read back, so
// they keep Dinah's name whatever the product's profile says.
func SteerMarker(session string) string { return "[Dinah chat: " + session + "]" }

func SteerTurnMarker(session, token string) string {
	return SteerMarker(session) + "\n[Dinah turn: " + token + "]"
}

var turnMarkerRE = regexp.MustCompile(`\[Dinah turn: ([A-Za-z0-9_-]{1,128})\]`)

func MarkedTurn(prompt string) string {
	m := turnMarkerRE.FindAllStringSubmatch(prompt, -1)
	if len(m) == 0 {
		return ""
	}
	return m[len(m)-1][1]
}

var steerMarkerRE = regexp.MustCompile(`\[Dinah chat: ([A-Za-z0-9_.:-]{1,128})\]`)

// StripMarkers removes the chat markers from a prompt's text, which are for
// the hooks and are not part of what the person said.
func StripMarkers(text string) string {
	return turnMarkerRE.ReplaceAllString(steerMarkerRE.ReplaceAllString(text, ""), "")
}

// MarkedSession returns the chat's bus session a prompt's marker gives, or
// "". The last marker wins, since the window's own context comes last.
func MarkedSession(prompt string) string {
	m := steerMarkerRE.FindAllStringSubmatch(prompt, -1)
	if len(m) == 0 {
		return ""
	}
	return m[len(m)-1][1]
}

// Steer is a message the person sent a working agent.
type Steer struct {
	ID   int64
	Text string
}

// BindAgentSession records that an agent's own session ID belongs to a bus
// session, which must exist.
func (b *Bus) BindAgentSession(ctx context.Context, agentSession, session string) error {
	if agentSession == "" || session == "" {
		return errors.New("both sessions are needed")
	}
	return b.tx(ctx, func(t *sql.Tx) error {
		if !taken(t, `SELECT 1 FROM sessions WHERE id = ?`, session) {
			return fmt.Errorf("session %s: %w", session, ErrNotFound)
		}
		_, err := t.Exec(`INSERT INTO agent_sessions (agent_session, session, bound_at) VALUES (?, ?, ?)
			ON CONFLICT (agent_session) DO UPDATE SET session = excluded.session, bound_at = excluded.bound_at`, agentSession, session, b.stamp())
		return err
	})
}

// SessionOfAgent returns the bus session an agent's own session belongs to,
// or "".
func (b *Bus) SessionOfAgent(ctx context.Context, agentSession string) (string, error) {
	var s string
	err := b.db.QueryRowContext(ctx, `SELECT session FROM agent_sessions WHERE agent_session = ?`, agentSession).Scan(&s)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return s, err
}

// HooksReach says whether a steer for a bus session can be handed over by the agent's hooks: the session has been tied
// to its agent's own session, which only the hooks do, and no hook turn has ended without another starting (a chat with no host-managed turns counts as reached). The turn's own hook need not have
// run yet: the first hook of a turn, UserPromptSubmit, runs a moment after the prompt is sent, and a steer queued before
// it carries the turn's token and is handed over by it, or by a later hook of the turn, so a person who types while the
// turn is starting is not told that steering is unavailable. A hook of an older turn never takes it (TakeAgentSteers).
func (b *Bus) HooksReach(ctx context.Context, session string) (bool, error) {
	var n int
	err := b.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM agent_sessions a WHERE session = ? AND
 NOT EXISTS (SELECT 1 FROM hook_turns h WHERE h.session = a.session AND h.active = 0)`, session).Scan(&n)
	return n > 0, err
}

// QueueSteer keeps a steer for a session until a hook takes it or the
// window withdraws it.
func (b *Bus) QueueSteer(ctx context.Context, session, text string) (int64, error) {
	if strings.TrimSpace(text) == "" {
		return 0, errors.New("the steer is empty")
	}
	var id int64
	err := b.tx(ctx, func(t *sql.Tx) error {
		r, err := t.Exec(`INSERT INTO steers (session, text, created_at, turn_token) VALUES (?, ?, ?, COALESCE((SELECT token FROM hook_turns WHERE session = ? AND active = 1), ''))`, session, text, b.stamp(), session)
		if err != nil {
			return err
		}
		id, err = r.LastInsertId()
		return err
	})
	return id, err
}

// TakeSteers hands over a session's waiting steers, oldest first, and marks
// them delivered. A steer is taken once.
func (b *Bus) TakeSteers(ctx context.Context, session string) ([]Steer, error) {
	return b.takeSteers(ctx, session, "", "")
}

func (b *Bus) TakeAgentSteers(ctx context.Context, agent, promptID string) ([]Steer, error) {
	return b.takeSteers(ctx, "", agent, promptID)
}

func (b *Bus) takeSteers(ctx context.Context, session, agent, promptID string) ([]Steer, error) {
	var out []Steer
	err := b.tx(ctx, func(t *sql.Tx) error {
		token := ""
		if agent != "" {
			var boundPrompt string
			err := t.QueryRow(`SELECT session, turn_token, prompt_id FROM agent_sessions WHERE agent_session = ?`, agent).Scan(&session, &token, &boundPrompt)
			if errors.Is(err, sql.ErrNoRows) {
				return nil
			}
			if err != nil {
				return err
			}
			if boundPrompt != promptID {
				return nil
			}
		}
		var active int
		var current string
		err := t.QueryRow(`SELECT token, active FROM hook_turns WHERE session = ?`, session).Scan(&current, &active)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil {
			if active == 0 || (agent != "" && token != current) {
				return nil
			}
			token = current
		}
		rows, err := t.Query(`SELECT id, text FROM steers WHERE session = ? AND turn_token = ? AND delivered_at IS NULL AND withdrawn_at IS NULL ORDER BY id`, session, token)
		if err != nil {
			return err
		}
		for rows.Next() {
			var s Steer
			if err := rows.Scan(&s.ID, &s.Text); err != nil {
				rows.Close()
				return err
			}
			out = append(out, s)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		now := b.stamp()
		for _, s := range out {
			if _, err := t.Exec(`UPDATE steers SET delivered_at = ? WHERE id = ?`, now, s.ID); err != nil {
				return err
			}
		}
		return nil
	})
	return out, err
}

// WithdrawSteer takes back a steer no hook has taken. It returns false when
// a hook took it first.
func (b *Bus) WithdrawSteer(ctx context.Context, id int64) (bool, error) {
	var ok bool
	err := b.tx(ctx, func(t *sql.Tx) error {
		r, err := t.Exec(`UPDATE steers SET withdrawn_at = ? WHERE id = ? AND delivered_at IS NULL AND withdrawn_at IS NULL`, b.stamp(), id)
		if err != nil {
			return err
		}
		n, err := r.RowsAffected()
		ok = n == 1
		return err
	})
	return ok, err
}

// SteerDelivered says whether a hook has taken a steer.
func (b *Bus) SteerDelivered(ctx context.Context, id int64) (bool, error) {
	var at sql.NullString
	err := b.db.QueryRowContext(ctx, `SELECT delivered_at FROM steers WHERE id = ?`, id).Scan(&at)
	if errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("steer %d: %w", id, ErrNotFound)
	}
	return at.Valid, err
}

// FormatSteers writes steers as the agent receives them from a hook, calling
// the person person.
func FormatSteers(steers []Steer, person string) string {
	var sb strings.Builder
	sb.WriteString(SentenceStart(person) + " steered you while you worked; carry on with this in mind, without stopping what you are doing unless it says to:\n")
	for _, s := range steers {
		sb.WriteString("\n")
		sb.WriteString(s.Text)
		sb.WriteString("\n")
	}
	return sb.String()
}

// BeginHookTurn closes delivery from previous bindings before sending a prompt.
func (b *Bus) BeginHookTurn(ctx context.Context, session, token string) error {
	return b.tx(ctx, func(t *sql.Tx) error {
		_, err := t.Exec(`INSERT INTO hook_turns (session, token, active, began_at, ended_at) VALUES (?, ?, 1, ?, '')
   ON CONFLICT(session) DO UPDATE SET token=excluded.token, active=1, began_at=excluded.began_at, ended_at=''`, session, token, b.stamp())
		return err
	})
}

// ReopenHookTurn marks a chat's hook turn running again, for a stop the agent was told not to make: the agent carries
// on in the turn, even when the window's prompt for it has already come back.
func (b *Bus) ReopenHookTurn(ctx context.Context, session string) error {
	return b.tx(ctx, func(t *sql.Tx) error {
		_, err := t.Exec(`UPDATE hook_turns SET active = 1, ended_at = '' WHERE session = ? AND active = 0`, session)
		return err
	})
}

func (b *Bus) EndHookTurn(ctx context.Context, session, token string) error {
	return b.tx(ctx, func(t *sql.Tx) error {
		_, err := t.Exec(`UPDATE hook_turns SET active=0, ended_at=? WHERE session=? AND token=?`, b.stamp(), session, token)
		return err
	})
}

// BindAgentTurn refuses a delayed UserPromptSubmit from an older host turn.
func (b *Bus) BindAgentTurn(ctx context.Context, agent, session, token, promptID string) error {
	return b.tx(ctx, func(t *sql.Tx) error {
		var current string
		var active int
		err := t.QueryRow(`SELECT token, active FROM hook_turns WHERE session=?`, session).Scan(&current, &active)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil && (active == 0 || current != token) {
			return nil
		}
		if !taken(t, `SELECT 1 FROM sessions WHERE id=?`, session) {
			return ErrNotFound
		}
		_, err = t.Exec(`INSERT INTO agent_sessions (agent_session, session, bound_at, turn_token, prompt_id) VALUES (?, ?, ?, ?, ?)
   ON CONFLICT(agent_session) DO UPDATE SET session=excluded.session, bound_at=excluded.bound_at, turn_token=excluded.turn_token, prompt_id=excluded.prompt_id`, agent, session, b.stamp(), token, promptID)
		return err
	})
}

// SteerFacts is what the bus knows about whether a chat can be steered, for the log line that explains a decision.
type SteerFacts struct {
	// Bound says an agent session is tied to the chat, and BindingToken is the hook turn token of its last binding.
	Bound        bool   `json:"bound"`
	BindingToken string `json:"bindingToken"`
	// HookTurn says the chat has had a hook turn; TurnToken is the latest one's token, Active whether it is running,
	// and BeganAt and EndedAt (empty while it runs) when it did.
	HookTurn  bool   `json:"hookTurn"`
	TurnToken string `json:"turnToken"`
	Active    bool   `json:"active"`
	BeganAt   string `json:"beganAt"`
	EndedAt   string `json:"endedAt"`
	// Reaches is HooksReach's answer.
	Reaches bool `json:"reaches"`
}

// SteerFactsOf reads the facts behind HooksReach for a chat. Tokens are cut to eight characters.
func (b *Bus) SteerFactsOf(ctx context.Context, session string) (SteerFacts, error) {
	var f SteerFacts
	var binding string
	err := b.db.QueryRowContext(ctx, `SELECT turn_token FROM agent_sessions WHERE session = ? ORDER BY bound_at DESC LIMIT 1`, session).Scan(&binding)
	switch {
	case err == nil:
		f.Bound, f.BindingToken = true, cutToken(binding)
	case !errors.Is(err, sql.ErrNoRows):
		return f, err
	}
	var token, began, ended string
	var active int
	err = b.db.QueryRowContext(ctx, `SELECT token, active, began_at, ended_at FROM hook_turns WHERE session = ?`, session).Scan(&token, &active, &began, &ended)
	switch {
	case err == nil:
		f.HookTurn, f.TurnToken, f.Active, f.BeganAt, f.EndedAt = true, cutToken(token), active == 1, began, ended
	case !errors.Is(err, sql.ErrNoRows):
		return f, err
	}
	f.Reaches, err = b.HooksReach(ctx, session)
	return f, err
}

func cutToken(t string) string {
	if len(t) > 8 {
		return t[:8]
	}
	return t
}
