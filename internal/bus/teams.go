package bus

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Teams (docs/plans/teams.md). A team member is a chat that has a lead: the bus records
// each member's lead, a lead is any chat that leads at least one member, and a member can
// lead a team of its own, so teams nest. A team's channel is a group chat its lead runs,
// marked with the lead's session on the group's row. A member's mention of the person is
// rerouted to its lead when the message is stored, which is all the nesting there is to do,
// since a lead's own message that mentions the person may itself be a member's.
//
// Every fact is in this database, which every program on the machine shares.

// StartedPrefix begins the text of the line a team's group keeps when its lead starts a member; the rest says who and
// with what.
const StartedPrefix = "started "

// TeamStartLine posts the line that says a lead started a member, so a start is never silent.
func (b *Bus) TeamStartLine(ctx context.Context, lead, group, detail string) error {
	s, err := b.Session(ctx, lead)
	if err != nil {
		return err
	}
	return b.tx(ctx, func(t *sql.Tx) error {
		key, title, err := findGroup(t, group)
		if err != nil {
			return err
		}
		_, err = b.insertMessage(t, Sender{Session: &s}, MemberKind, resolved{label: "#" + title, room: key, roomTitle: title, toKind: "room", toKey: key}, 0, StartedPrefix+detail, "", "", nil)
		return err
	})
}

// LeadDeletedLine is the text of the line a team's group keeps when its lead is deleted.
const LeadDeletedLine = "lead deleted"

// TeamMember is a chat that has a lead.
type TeamMember struct {
	Session string
	Lead    string
	Since   string
	Retired bool
}

// LeadOf is the chat that leads a member, and whether the member is retired, or "" for a
// chat that has no lead.
func (b *Bus) LeadOf(ctx context.Context, member string) (lead string, retired bool, err error) {
	err = b.db.QueryRowContext(ctx, `SELECT lead, retired FROM team_members WHERE member = ?`, member).Scan(&lead, &retired)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return lead, retired, err
}

// TeamMembers are the chats a lead leads, in the order they joined, retired ones too.
func (b *Bus) TeamMembers(ctx context.Context, lead string) ([]TeamMember, error) {
	rows, err := b.db.QueryContext(ctx, `SELECT member, lead, since, retired FROM team_members WHERE lead = ? ORDER BY rowid`, lead)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TeamMember
	for rows.Next() {
		var m TeamMember
		if err := rows.Scan(&m.Session, &m.Lead, &m.Since, &m.Retired); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// IsLead says a chat leads at least one member.
func (b *Bus) IsLead(ctx context.Context, session string) (bool, error) {
	var n int
	err := b.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM team_members WHERE lead = ?`, session).Scan(&n)
	return n > 0, err
}

// TeamGroup is the stable key of a lead's team channel, or "" when it has none.
func (b *Bus) TeamGroup(ctx context.Context, lead string) (string, error) {
	var key string
	err := b.db.QueryRowContext(ctx, `SELECT name FROM rooms WHERE lead = ?`, lead).Scan(&key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return key, err
}

func teamLeadOf(t querier, member string) (lead string, retired bool, err error) {
	err = t.QueryRow(`SELECT lead, retired FROM team_members WHERE member = ?`, member).Scan(&lead, &retired)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return lead, retired, err
}

func knownSession(t querier, id string) (title string, err error) {
	err = t.QueryRow(`SELECT title FROM sessions WHERE id = ?`, id).Scan(&title)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("chat %s: %w", id, ErrNotFound)
	}
	return title, err
}

// chain lists a chat's leads, nearest first, as far as the relation goes.
func chainOf(t querier, session string) ([]string, error) {
	var out []string
	seen := map[string]bool{session: true}
	for cur := session; ; {
		lead, _, err := teamLeadOf(t, cur)
		if err != nil {
			return nil, err
		}
		if lead == "" || seen[lead] {
			return out, nil
		}
		seen[lead] = true
		out = append(out, lead)
		cur = lead
	}
}

// teamGroup finds the lead's team channel, making it when it has none: a group chat
// called "<lead>'s team" with the lead as its member.
func (b *Bus) teamGroup(t *sql.Tx, lead string) (string, error) {
	var key string
	err := t.QueryRow(`SELECT name FROM rooms WHERE lead = ?`, lead).Scan(&key)
	if err == nil {
		return key, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	title, err := knownSession(t, lead)
	if err != nil {
		return "", err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "A lead"
	}
	title += "'s team"
	key = groupKey(t, title)
	if _, err := t.Exec(`INSERT INTO rooms (name, created_at, title, lead) VALUES (?, ?, ?, ?)`, key, b.stamp(), title, lead); err != nil {
		return "", err
	}
	if _, err := t.Exec(`INSERT OR IGNORE INTO room_members (room, member_kind, member) VALUES (?, 'human', ?), (?, 'session', ?)`, key, Human, key, lead); err != nil {
		return "", err
	}
	return key, nil
}

func (b *Bus) joinTeamGroup(t *sql.Tx, key, member string) error {
	_, err := t.Exec(`INSERT OR IGNORE INTO room_members (room, member_kind, member) VALUES (?, 'session', ?)`, key, member)
	return err
}

// SetLead makes a chat a member of a lead's team, or moves it there from another, and puts
// it in the team's group, which it makes when the lead has none; it leaves the group of the
// lead it had. It returns the team group's key. A chat cannot lead itself, and a relation that
// would make a chat its own lead through a chain is refused with the chain.
func (b *Bus) SetLead(ctx context.Context, member, lead string) (string, error) {
	var group string
	err := b.tx(ctx, func(t *sql.Tx) error {
		var err error
		group, err = b.setLead(t, member, lead)
		return err
	})
	return group, err
}

func (b *Bus) setLead(t *sql.Tx, member, lead string) (string, error) {
	if member == lead {
		return "", errors.New("a chat cannot lead itself")
	}
	memberTitle, err := knownSession(t, member)
	if err != nil {
		return "", err
	}
	leadTitle, err := knownSession(t, lead)
	if err != nil {
		return "", err
	}
	// The lead's own chain must not reach the member.
	chain, err := chainOf(t, lead)
	if err != nil {
		return "", err
	}
	for _, c := range chain {
		if c == member {
			names := []string{leadTitle}
			for _, id := range chain {
				title, _ := knownSession(t, id)
				names = append(names, title)
				if id == member {
					break
				}
			}
			return "", fmt.Errorf("%s cannot be a member of %s's team, because %s already leads it (%s)", memberTitle, leadTitle, leadTitle, strings.Join(names, " is led by "))
		}
	}
	previous, _, err := teamLeadOf(t, member)
	if err != nil {
		return "", err
	}
	if previous == lead {
		return b.teamGroup(t, lead)
	}
	if previous != "" {
		if err := b.leaveTeamGroup(t, member, previous); err != nil {
			return "", err
		}
		if _, err := t.Exec(`DELETE FROM team_members WHERE member = ?`, member); err != nil {
			return "", err
		}
	}
	group, err := b.teamGroup(t, lead)
	if err != nil {
		return "", err
	}
	if _, err := t.Exec(`INSERT INTO team_members (member, lead, since) VALUES (?, ?, ?)`, member, lead, b.stamp()); err != nil {
		return "", err
	}
	return group, b.joinTeamGroup(t, group, member)
}

// leaveTeamGroup takes a member out of the group of the lead it had.
func (b *Bus) leaveTeamGroup(t *sql.Tx, member, lead string) error {
	_, err := t.Exec(`DELETE FROM room_members WHERE member_kind = 'session' AND member = ? AND room IN (SELECT name FROM rooms WHERE lead = ?)`, member, lead)
	return err
}

// ClearLead turns a team member back into an ordinary chat: it has no lead and is out of the
// team's group, and stays in any other group it is in. Its messages to the person reach him
// directly again.
func (b *Bus) ClearLead(ctx context.Context, member string) error {
	return b.tx(ctx, func(t *sql.Tx) error {
		lead, _, err := teamLeadOf(t, member)
		if err != nil {
			return err
		}
		if lead == "" {
			return nil
		}
		if err := b.leaveTeamGroup(t, member, lead); err != nil {
			return err
		}
		_, err = t.Exec(`DELETE FROM team_members WHERE member = ?`, member)
		return err
	})
}

// SetRetired marks a member retired, or brings it back. A retired member stays in the team's
// group and the strip, but is left out of the group's wake and notice set (rooms.go), so a
// message there does not reopen its ended session.
func (b *Bus) SetRetired(ctx context.Context, member string, retired bool) error {
	return b.tx(ctx, func(t *sql.Tx) error {
		res, err := t.Exec(`UPDATE team_members SET retired = ? WHERE member = ?`, retired, member)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return fmt.Errorf("chat %s has no lead: %w", member, ErrNotFound)
		}
		return nil
	})
}

// LeftOut is a chat that making a group a team did not make a member, and why.
type LeftOut struct {
	Session string
	Title   string
	Why     string
}

// MakeGroupTeam makes a group chat a team: the lead's team channel is the group, and every other
// chat in it becomes a member, except one that already has a different lead, which keeps it, since
// a chat has only one. It returns the members made and the chats left out.
func (b *Bus) MakeGroupTeam(ctx context.Context, group, lead string) (made []string, left []LeftOut, err error) {
	err = b.tx(ctx, func(t *sql.Tx) error {
		key, _, err := findGroup(t, group)
		if err != nil {
			return err
		}
		var isMember int
		if err := t.QueryRow(`SELECT COUNT(*) FROM room_members WHERE room = ? AND member_kind = 'session' AND member = ?`, key, lead).Scan(&isMember); err != nil {
			return err
		}
		if isMember == 0 {
			return errors.New("the chat that leads a team must be a member of the group")
		}
		var current string
		if err := t.QueryRow(`SELECT lead FROM rooms WHERE name = ?`, key).Scan(&current); err != nil {
			return err
		}
		if current != "" && current != lead {
			title, _ := knownSession(t, current)
			return fmt.Errorf("this group is already the team channel of %s", title)
		}
		var other string
		err = t.QueryRow(`SELECT name FROM rooms WHERE lead = ? AND name <> ?`, lead, key).Scan(&other)
		if err == nil {
			title, _ := knownSession(t, lead)
			return fmt.Errorf("%s already leads a team with its own group; a chat leads one team", title)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err := t.Exec(`UPDATE rooms SET lead = ? WHERE name = ?`, lead, key); err != nil {
			return err
		}
		rows, err := t.Query(`SELECT member FROM room_members WHERE room = ? AND member_kind = 'session' AND member <> ? ORDER BY rowid`, key, lead)
		if err != nil {
			return err
		}
		var members []string
		for rows.Next() {
			var m string
			if err := rows.Scan(&m); err != nil {
				rows.Close()
				return err
			}
			members = append(members, m)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, m := range members {
			title, _ := knownSession(t, m)
			prior, _, err := teamLeadOf(t, m)
			if err != nil {
				return err
			}
			if prior != "" && prior != lead {
				priorTitle, _ := knownSession(t, prior)
				left = append(left, LeftOut{Session: m, Title: title, Why: "it already has a lead, " + priorTitle + ", and a chat has only one"})
				continue
			}
			chain, err := chainOf(t, lead)
			if err != nil {
				return err
			}
			if slicesContains(chain, m) {
				left = append(left, LeftOut{Session: m, Title: title, Why: "it already leads " + "the lead's own chain, so it cannot be a member"})
				continue
			}
			if prior == lead {
				made = append(made, m)
				continue
			}
			if _, err := t.Exec(`INSERT INTO team_members (member, lead, since) VALUES (?, ?, ?)`, m, lead, b.stamp()); err != nil {
				return err
			}
			made = append(made, m)
		}
		return nil
	})
	return made, left, err
}

func slicesContains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// LeadRemoved is called when a chat is deleted. The members it led become ordinary chats that reach
// the person directly, out of its team's group, which keeps a line saying so; and the chat's own row
// as a member is removed.
func (b *Bus) LeadRemoved(ctx context.Context, session string) (orphans []string, err error) {
	err = b.tx(ctx, func(t *sql.Tx) error {
		rows, err := t.Query(`SELECT member FROM team_members WHERE lead = ? ORDER BY rowid`, session)
		if err != nil {
			return err
		}
		for rows.Next() {
			var m string
			if err := rows.Scan(&m); err != nil {
				rows.Close()
				return err
			}
			orphans = append(orphans, m)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if _, err := t.Exec(`DELETE FROM team_members WHERE lead = ? OR member = ?`, session, session); err != nil {
			return err
		}
		// The team's group stays, as an ordinary group, with its history and a line that says its
		// members now reach the person directly.
		var key, title string
		if err := t.QueryRow(`SELECT name, CASE WHEN title <> '' THEN title ELSE name END FROM rooms WHERE lead = ?`, session).Scan(&key, &title); err == nil && len(orphans) > 0 {
			if _, err := b.insertMessage(t, Sender{}, MemberKind, resolved{label: title, room: key, roomTitle: title, toKind: "room", toKey: key}, 0, LeadDeletedLine, "", "", nil); err != nil {
				return err
			}
		}
		_, err = t.Exec(`UPDATE rooms SET lead = '' WHERE lead = ?`, session)
		return err
	})
	return orphans, err
}

func slicesWithout(list []string, s string) []string {
	var out []string
	for _, x := range list {
		if x != s {
			out = append(out, x)
		}
	}
	return out
}

// retiredMembers are the retired team members, who are left out of a group's wake and notice set.
func retiredMembers(t querier) (map[string]bool, error) {
	rows, err := t.Query(`SELECT member FROM team_members WHERE retired = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return nil, err
		}
		out[m] = true
	}
	return out, rows.Err()
}

// Leads are the chats that lead at least one member, in the order of their first member.
func (b *Bus) Leads(ctx context.Context) ([]string, error) {
	rows, err := b.db.QueryContext(ctx, `SELECT lead FROM team_members GROUP BY lead ORDER BY MIN(rowid)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var l string
		if err := rows.Scan(&l); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// Assignment is the newest message a lead sent that mentions a member: the brief or the work it was given.
type Assignment struct {
	Message int64
	Room    string
	Text    string
}

// LastAssignment is the newest message the lead sent that mentions the member, and false when it sent none.
func (b *Bus) LastAssignment(ctx context.Context, lead, member string) (Assignment, bool, error) {
	var a Assignment
	err := b.db.QueryRowContext(ctx, `SELECT id, room, text FROM messages WHERE from_session = ? AND kind = 'text' AND mentions LIKE '%,' || ? || ',%'
		ORDER BY id DESC LIMIT 1`, lead, member).Scan(&a.Message, &a.Room, &a.Text)
	if errors.Is(err, sql.ErrNoRows) {
		return Assignment{}, false, nil
	}
	return a, err == nil, err
}

// PendingRelayFrom says a member has a relay the lead has not yet answered: pending, and no later message of
// the lead's in the team's group.
func (b *Bus) PendingRelayFrom(ctx context.Context, member string) (int64, bool, error) {
	var id int64
	err := b.db.QueryRowContext(ctx, `SELECT r.message_id FROM relays r WHERE r.member = ? AND r.state = 'pending'
		AND NOT EXISTS (SELECT 1 FROM messages m WHERE m.from_session = r.lead AND m.id > r.message_id)
		ORDER BY r.message_id DESC LIMIT 1`, member).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return id, err == nil, err
}

// TeamSends are the messages chats sent in team channels after a number, in order: the lead's and the members' own successful
// sends, which a window marks in a chat's history so that a run of team traffic can be folded.
func (b *Bus) TeamSends(ctx context.Context, after int64) ([]Message, error) {
	rows, err := b.db.QueryContext(ctx, `SELECT m.id FROM messages m JOIN rooms r ON r.name = m.room
		WHERE r.lead <> '' AND m.from_session <> '' AND m.kind = 'text' AND m.id > ? ORDER BY m.id`, after)
	if err != nil {
		return nil, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]Message, 0, len(ids))
	for _, id := range ids {
		m, err := b.Message(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}
