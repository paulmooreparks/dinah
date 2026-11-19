package bus

import (
	"context"
	"database/sql"
	"time"
)

// StaleAfter is how long a chat started outside the window may go unseen
// before the bus takes it as ended.
//
// The window ends its own chats. A chat started outside it, by an agent
// program in a terminal or an editor, is ended by that program's hook when
// the session ends, and a program that is killed, or a computer that is
// switched off, never runs the hook. Such a chat stayed open on the bus for
// good: it was listed to every agent as a chat to write to, and where it
// shared its title with a living chat of the same workspace, neither could
// be written to by title (Paul, 19 November 2026, on a chat last seen on
// 3 November).
//
// Seen means that the chat registered, took a delivery, or that the program
// serving its tools said it was still running (TouchSession), which it does
// every HeartbeatEvery. A chat that is ended here and is seen again is open
// again, as any ended chat is (RegisterSession, Deliver).
const StaleAfter = 7 * 24 * time.Hour

// HeartbeatEvery is how often the program serving a chat's tools outside
// the window says that it is still running.
const HeartbeatEvery = time.Hour

// TouchSession records that the program serving an open chat's tools is
// still running. It does not open a chat that has ended: a chat is opened
// again by what it does, not by a program that outlives it.
func (b *Bus) TouchSession(ctx context.Context, id string) error {
	_, err := b.db.ExecContext(ctx, `UPDATE sessions SET last_seen = ? WHERE id = ? AND ended_at IS NULL AND removed_at IS NULL`, b.stamp(), id)
	return err
}

// EndStaleSessions ends the chats started outside the window that have not
// been seen for StaleAfter, and reports how many it ended.
func (b *Bus) EndStaleSessions(ctx context.Context) (ended int64, err error) {
	err = b.tx(ctx, func(t *sql.Tx) error {
		ended, err = endStale(t, b.now(), b.stamp())
		return err
	})
	return ended, err
}

// endStale is EndStaleSessions inside a transaction. The times the bus
// writes are UTC in one layout, so the text of two of them compares as the
// times do, to the second.
func endStale(t *sql.Tx, now time.Time, stamp string) (int64, error) {
	before := now.Add(-StaleAfter).UTC().Format(time.RFC3339Nano)
	res, err := t.Exec(`UPDATE sessions SET ended_at = ?
		WHERE ended_at IS NULL AND removed_at IS NULL AND last_seen <> '' AND last_seen < ?
		AND substr(id, 1, ?) <> ?`, stamp, before, len(WindowSessionPrefix), WindowSessionPrefix)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
