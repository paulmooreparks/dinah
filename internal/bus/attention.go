package bus

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// GroupAttentionSettings apply immediately to the history-derived view.
type GroupAttentionSettings struct {
	Limit      int `json:"limit"`
	Minutes    int `json:"minutes"`
	LimitMin   int `json:"limitMin"`
	LimitMax   int `json:"limitMax"`
	MinutesMin int `json:"minutesMin"`
	MinutesMax int `json:"minutesMax"`
}

type ReachMember struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type EngagedMember struct {
	ReachMember
	EngagedAt string `json:"engagedAt"`
	Expires   string `json:"expires"`
}

// GroupReach is a preview, or the immutable snapshot recorded by a send.
type GroupReach struct {
	Group    string                 `json:"group"`
	Version  int64                  `json:"version"`
	Reach    []ReachMember          `json:"reach"`
	Everyone bool                   `json:"everyone"`
	Reason   string                 `json:"reason"`
	Engaged  []EngagedMember        `json:"engaged"`
	Expires  string                 `json:"expires"`
	Settings GroupAttentionSettings `json:"settings"`
}

func attentionSettings(t querier) (s GroupAttentionSettings, err error) {
	err = t.QueryRow(`SELECT member_limit, minutes FROM group_attention_policy WHERE id = 1`).Scan(&s.Limit, &s.Minutes)
	s.LimitMin, s.LimitMax, s.MinutesMin, s.MinutesMax = 1, 10, 1, 1440
	return
}

func (b *Bus) GroupAttentionSettings(ctx context.Context) (GroupAttentionSettings, error) {
	return attentionSettings(b.db)
}

func (b *Bus) SetGroupAttentionSettings(ctx context.Context, limit, minutes int) error {
	if limit < 1 || limit > 10 || minutes < 1 || minutes > 1440 {
		return errors.New("group attention needs 1 to 10 members and 1 to 1440 minutes")
	}
	return b.tx(ctx, func(t *sql.Tx) error {
		_, err := t.Exec(`UPDATE group_attention_policy SET member_limit = ?, minutes = ? WHERE id = 1`, limit, minutes)
		return err
	})
}

// GroupReach validates explicit mentions by the same path as a person send.
// Its read transaction does not increment the bus change counter.
func (b *Bus) GroupReach(ctx context.Context, group string, mentions []string) (GroupReach, error) {
	person := b.PersonName()
	t, err := b.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return GroupReach{}, err
	}
	defer t.Rollback()
	r, err := groupResolved(t, Sender{}, group)
	if err != nil {
		return GroupReach{}, err
	}
	if err := mentionList(t, Sender{}, &r, mentions, person); err != nil {
		return GroupReach{}, err
	}
	return b.groupReach(t, r, b.now())
}

func attentionMembers(t *sql.Tx, room string) ([]chat, error) {
	return scanChats(t.Query(chatSelect+` JOIN room_members m ON m.member_kind = 'session' AND m.member = s.id
		WHERE m.room = ? AND s.removed_at IS NULL AND NOT EXISTS
		(SELECT 1 FROM team_members tm WHERE tm.member = s.id AND tm.retired = 1) ORDER BY m.rowid`, room))
}

func (b *Bus) groupReach(t *sql.Tx, r resolved, now time.Time) (GroupReach, error) {
	out := GroupReach{Group: r.room, Reach: []ReachMember{}, Engaged: []EngagedMember{}}
	var err error
	if out.Settings, err = attentionSettings(t); err != nil {
		return out, err
	}
	if err := t.QueryRow(`SELECT counter FROM changes WHERE id = 1`).Scan(&out.Version); err != nil {
		return out, err
	}
	members, err := attentionMembers(t, r.room)
	if err != nil {
		return out, err
	}
	out.Engaged, err = engagedMembers(t, r.room, members, out.Settings, now)
	if err != nil {
		return out, err
	}
	var nearest time.Time
	for _, m := range out.Engaged {
		expires, err := time.Parse(time.RFC3339Nano, m.Expires)
		if err != nil {
			return out, err
		}
		if nearest.IsZero() || expires.Before(nearest) {
			nearest = expires
			out.Expires = m.Expires
		}
	}
	if r.mentions != "" {
		out.Reason = "explicit"
		out.Everyone = hasMention(r.mentions, Everyone) || hasMention(r.mentions, WakesAll)
		if out.Everyone {
			for _, m := range members {
				out.Reach = append(out.Reach, ReachMember{m.id, m.display()})
			}
		} else {
			byID := map[string]chat{}
			for _, m := range members {
				byID[m.id] = m
			}
			for _, id := range strings.Split(r.mentions, ",") {
				if m, ok := byID[id]; ok {
					out.Reach = append(out.Reach, ReachMember{id, m.display()})
				}
			}
		}
		return out, nil
	}
	if len(out.Engaged) > 0 {
		out.Reason = "engaged"
		for _, m := range out.Engaged {
			out.Reach = append(out.Reach, m.ReachMember)
		}
		return out, nil
	}
	var lead string
	if err := t.QueryRow(`SELECT lead FROM rooms WHERE name = ?`, r.room).Scan(&lead); err != nil {
		return out, err
	}
	for _, m := range members {
		if m.id == lead {
			out.Reason = "lead"
			out.Reach = append(out.Reach, ReachMember{m.id, m.display()})
			return out, nil
		}
	}
	out.Reason, out.Everyone = "everyone", true
	for _, m := range members {
		out.Reach = append(out.Reach, ReachMember{m.id, m.display()})
	}
	return out, nil
}

func hasMention(list, id string) bool { return strings.Contains(list, ","+id+",") }

func engagedMembers(t *sql.Tx, room string, members []chat, settings GroupAttentionSettings, now time.Time) ([]EngagedMember, error) {
	eligible := map[string]chat{}
	for _, m := range members {
		eligible[m.id] = m
	}
	// Membership changes are history too. Engagement before a leave/rejoin
	// boundary cannot return merely because that member joined again.
	boundaries := map[string]int64{}
	changes, err := t.Query(`SELECT to_key, MAX(id) FROM messages WHERE room = ? AND kind = ? AND to_kind = 'session' GROUP BY to_key`, room, MemberKind)
	if err != nil {
		return nil, err
	}
	for changes.Next() {
		var id string
		var message int64
		if err := changes.Scan(&id, &message); err != nil {
			changes.Close()
			return nil, err
		}
		boundaries[id] = message
	}
	err = changes.Err()
	changes.Close()
	if err != nil {
		return nil, err
	}
	window := time.Duration(settings.Minutes) * time.Minute
	// A one-second SQL margin avoids rounding at SQLite's date precision.
	// The parsed Go timestamps make the exact expiry decision below.
	rows, err := t.Query(`SELECT m.id, m.created_at, m.from_session, m.mentions FROM messages m
		WHERE m.room = ? AND m.kind = 'text' AND julianday(m.created_at) >= julianday(?) AND (
		(m.from_session = '' AND m.from_name = ? AND m.attention_reason IN ('', 'explicit')) OR
		(m.from_session <> '' AND (m.mentions LIKE ? OR m.relay_lead <> '' OR EXISTS
		(SELECT 1 FROM messages p WHERE p.id = m.reply_to AND p.room = m.room AND p.kind = 'text' AND p.from_session = '' AND p.from_name = ?))))
		ORDER BY m.id DESC`, room, now.Add(-window-time.Second).UTC().Format(time.RFC3339Nano), Human, "%,"+Human+",%", Human)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []EngagedMember{}
	seen := map[string]bool{}
	for rows.Next() {
		var id int64
		var at, sender, mentions string
		if err := rows.Scan(&id, &at, &sender, &mentions); err != nil {
			return nil, err
		}
		stamp, err := time.Parse(time.RFC3339Nano, at)
		if err != nil {
			return nil, fmt.Errorf("group engagement message %d has an invalid time: %w", id, err)
		}
		expires := stamp.Add(window)
		if !now.Before(expires) {
			continue
		}
		ids := strings.Split(mentions, ",")
		if sender != "" {
			ids = []string{sender}
		}
		for _, member := range ids {
			m, ok := eligible[member]
			if !ok || seen[member] || id <= boundaries[member] {
				continue
			}
			seen[member] = true
			out = append(out, EngagedMember{ReachMember{m.id, m.display()}, stamp.UTC().Format(time.RFC3339Nano), expires.UTC().Format(time.RFC3339Nano)})
			if len(out) == settings.Limit {
				return out, nil
			}
		}
	}
	return out, rows.Err()
}

func (b *Bus) routePersonAttention(t *sql.Tx, r *resolved) (GroupReach, string, error) {
	reach, err := b.groupReach(t, *r, b.now())
	if err != nil {
		return reach, "", err
	}
	if r.mentions == "" {
		if reach.Everyone {
			r.mentions = "," + Everyone + "," + WakesAll + ","
		} else {
			ids := []string{}
			for _, member := range reach.Reach {
				ids = append(ids, member.ID)
			}
			r.mentions = "," + strings.Join(ids, ",") + ","
		}
	}
	reach.Version++ // The enclosing send commits one bus change.
	encoded, err := json.Marshal(reach)
	return reach, string(encoded), err
}
