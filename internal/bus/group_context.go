package bus

import (
	"context"
	"errors"
)

// RoomMessageContext reads at most five preceding messages in the target's
// group, oldest first. It changes no delivery or read state.
func (b *Bus) RoomMessageContext(ctx context.Context, room string, id int64) ([]Message, error) {
	var targetRoom string
	if err := b.db.QueryRowContext(ctx, `SELECT room FROM messages WHERE id=?`, id).Scan(&targetRoom); err != nil {
		return nil, err
	}
	if room == "" || targetRoom != room {
		return nil, errors.New("the target message is not in this group")
	}
	return b.queryMessages(ctx, messageSelect+` WHERE m.id IN
		(SELECT id FROM messages WHERE room=? AND id<? ORDER BY id DESC LIMIT 5) ORDER BY m.id`, room, id)
}
