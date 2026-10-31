package bus

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// ResolveRoom is a read-only window lookup. Stable keys take precedence over
// current titles, which take precedence over recorded historical titles.
// Historical aliases never change where sends are allowed to go.
// A missing room is not evidence that it was deleted.
func (b *Bus) ResolveRoom(ctx context.Context, address string) (state string, room *Room, err error) {
	state, key, err := b.resolveRoomKey(ctx, address)
	if err != nil || key == "" {
		return state, nil, err
	}
	return b.resolvedRoom(ctx, key)
}

func (b *Bus) resolveRoomKey(ctx context.Context, address string) (state, key string, err error) {
	a := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(address), "#"))
	state = "not-found"
	t, err := b.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return "", "", err
	}
	defer t.Rollback()
	err = func() error {
		rows, err := t.QueryContext(ctx, `SELECT name, title FROM rooms`)
		if err != nil {
			return err
		}
		current := map[string]bool{}
		for rows.Next() {
			var k, title string
			if err := rows.Scan(&k, &title); err != nil {
				rows.Close()
				return err
			}
			if k == strings.ToLower(a) {
				key = k
			}
			if fold(title) == fold(a) && a != "" {
				current[k] = true
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if key != "" {
			return nil
		}
		if len(current) == 0 {
			rows, err = t.QueryContext(ctx, `SELECT r.name, h.old_title FROM room_renames h JOIN rooms r ON r.name = h.room`)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var k, title string
				if err := rows.Scan(&k, &title); err != nil {
					return err
				}
				if fold(title) == fold(a) && a != "" {
					current[k] = true
				}
			}
			if err := rows.Err(); err != nil {
				return err
			}
		}
		if len(current) > 1 {
			state = "ambiguous"
		} else {
			for k := range current {
				key = k
			}
		}
		return nil
	}()
	if err == nil {
		err = t.Commit()
	}
	return state, key, err
}

func (b *Bus) resolvedRoom(ctx context.Context, key string) (string, *Room, error) {
	// Fetch by the chosen identity so a subsequent rename cannot rebind the
	// window to another room that takes its former title.
	var title string
	err := b.db.QueryRowContext(ctx, `SELECT title FROM rooms WHERE name = ?`, key).Scan(&title)
	if errors.Is(err, sql.ErrNoRows) {
		return "not-found", nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	r, err := b.room(ctx, key, title)
	if errors.Is(err, ErrNotFound) || errors.Is(err, sql.ErrNoRows) {
		return "not-found", nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	return "resolved", &r, nil
}
