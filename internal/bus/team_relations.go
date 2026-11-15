package bus

import (
	"context"
	"database/sql"
)

// TeamRelations contains the group identities, group membership and reporting
// rows captured in one read transaction. Notice counts are outside this view.
type TeamRelations struct {
	Rooms   []Room
	Members []TeamMember
}

func (b *Bus) TeamRelations(ctx context.Context) (TeamRelations, error) {
	tx, err := b.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return TeamRelations{}, err
	}
	defer tx.Rollback()
	out := TeamRelations{Rooms: []Room{}, Members: []TeamMember{}}
	rows, err := tx.QueryContext(ctx, `SELECT name, title, lead FROM rooms ORDER BY name`)
	if err != nil {
		return out, err
	}
	positions := map[string]int{}
	for rows.Next() {
		var room Room
		if err := rows.Scan(&room.Name, &room.Title, &room.Lead); err != nil {
			rows.Close()
			return out, err
		}
		room.Title = groupTitle(room.Name, room.Title)
		room.Members = []RoomMember{}
		positions[room.Name] = len(out.Rooms)
		out.Rooms = append(out.Rooms, room)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT rm.room, rm.member_kind, rm.member FROM room_members rm
		LEFT JOIN sessions s ON rm.member_kind = 'session' AND s.id = rm.member
		WHERE rm.member_kind = 'human' OR (s.id IS NOT NULL AND s.removed_at IS NULL) ORDER BY rm.rowid`)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var room string
		var member RoomMember
		if err := rows.Scan(&room, &member.Kind, &member.Key); err != nil {
			rows.Close()
			return out, err
		}
		if i, ok := positions[room]; ok {
			out.Rooms[i].Members = append(out.Rooms[i].Members, member)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT tm.member, tm.lead, tm.since, tm.retired FROM team_members tm
		JOIN sessions m ON m.id = tm.member AND m.removed_at IS NULL
		JOIN sessions l ON l.id = tm.lead AND l.removed_at IS NULL ORDER BY tm.rowid`)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var member TeamMember
		if err := rows.Scan(&member.Session, &member.Lead, &member.Since, &member.Retired); err != nil {
			rows.Close()
			return out, err
		}
		out.Members = append(out.Members, member)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}
	return out, tx.Commit()
}
