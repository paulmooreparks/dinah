package bus

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// A trusted link is Paul's standing word that the agents of two workspaces
// may act on each other's requests without asking him, the way they act on
// his. It runs both ways, only Paul makes or removes one, and each has its
// own limit on automatic handoffs between the two workspaces, which may be
// higher than MaxHops or off altogether.

// NoHopLimit is the hop limit of a link whose limit Paul turned off.
const NoHopLimit = -1

// DefaultLinkHops is the hop limit a new trusted link is offered with.
const DefaultLinkHops = 10

// Link is a trusted link between two workspaces, by name.
type Link struct {
	A, B     string
	HopLimit int // NoHopLimit when the limit is off
}

// pair orders two workspace keys as the links table keeps them.
func pair(x, y string) (string, string) {
	if x < y {
		return x, y
	}
	return y, x
}

func workspaceKey(t *sql.Tx, name string) (string, error) {
	var key string
	err := t.QueryRow(`SELECT key FROM workspaces WHERE name = ?`, name).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("no workspace on the bus is called %q", name)
	}
	return key, err
}

// Trust makes or changes the trusted link between two workspaces, by name.
// hops is the link's limit on automatic handoffs, or NoHopLimit. A
// workspace may trust itself: its chats then hand off to one another past
// the default limit, which a group of chats in one workspace that work
// together, such as a reviewer and two implementers, needs; without it
// their third exchange waits for the person to speak.
func (b *Bus) Trust(ctx context.Context, a, c string, hops int) error {
	if hops < 1 && hops != NoHopLimit {
		return errors.New("a trusted link allows at least one automatic handoff, or has no limit")
	}
	return b.tx(ctx, func(t *sql.Tx) error {
		ka, err := workspaceKey(t, a)
		if err != nil {
			return err
		}
		kc, err := workspaceKey(t, c)
		if err != nil {
			return err
		}
		x, y := pair(ka, kc)
		_, err = t.Exec(`INSERT INTO links (workspace_a, workspace_b, hop_limit, created_at) VALUES (?, ?, ?, ?)
			ON CONFLICT (workspace_a, workspace_b) DO UPDATE SET hop_limit = excluded.hop_limit`, x, y, hops, b.stamp())
		return err
	})
}

// Untrust removes the trusted link between two workspaces, by name.
func (b *Bus) Untrust(ctx context.Context, a, c string) error {
	return b.tx(ctx, func(t *sql.Tx) error {
		ka, err := workspaceKey(t, a)
		if err != nil {
			return err
		}
		kc, err := workspaceKey(t, c)
		if err != nil {
			return err
		}
		x, y := pair(ka, kc)
		_, err = t.Exec(`DELETE FROM links WHERE workspace_a = ? AND workspace_b = ?`, x, y)
		return err
	})
}

// Links returns every trusted link, by workspace name.
func (b *Bus) Links(ctx context.Context) ([]Link, error) {
	rows, err := b.db.QueryContext(ctx, `SELECT wa.name, wb.name, l.hop_limit FROM links l
		JOIN workspaces wa ON wa.key = l.workspace_a JOIN workspaces wb ON wb.key = l.workspace_b
		ORDER BY wa.name, wb.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Link
	for rows.Next() {
		var l Link
		if err := rows.Scan(&l.A, &l.B, &l.HopLimit); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// hopLimitSQL is the hop limit between a message's sender's workspace and a
// recipient's workspace, given as SQL expressions: the trusted link's when
// there is one, and MaxHops otherwise.
func hopLimitSQL(from, to string) string {
	return fmt.Sprintf(`COALESCE((SELECT l.hop_limit FROM links l WHERE l.workspace_a = MIN(%[1]s, %[2]s) AND l.workspace_b = MAX(%[1]s, %[2]s)), %[3]d)`, from, to, MaxHops)
}

// trustedSQL is true when a trusted link joins the two workspaces.
func trustedSQL(from, to string) string {
	return fmt.Sprintf(`EXISTS (SELECT 1 FROM links l WHERE l.workspace_a = MIN(%[1]s, %[2]s) AND l.workspace_b = MAX(%[1]s, %[2]s))`, from, to)
}
