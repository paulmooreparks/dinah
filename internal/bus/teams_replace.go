package bus

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func TeamLeadAssignmentReason(title string) string {
	return fmt.Sprintf("This chat leads %s. Reassign its members, or make one of them the lead, first.", title)
}

type TeamLeadRefusal struct{ Reason string }

func (e *TeamLeadRefusal) Error() string { return e.Reason }

// checkRecipientRemoval refuses to remove a chat that leads a team. Only the
// members the person can still see and reassign count: a member that is in
// the Recycle Bin is in no team as any reading of the teams shows them
// (LeadOf, Team), and a lead whose every member had been deleted first was
// refused with nothing left to reassign. The member's row is kept, so
// restoring both chats gives the team back.
func checkRecipientRemoval(t querier, id string) error {
	if yes, err := leadsAvailableMember(t, id); err != nil {
		return err
	} else if yes {
		reason, err := currentTeamLeadReason(t, id)
		if err != nil {
			return err
		}
		return &TeamLeadRefusal{Reason: reason}
	}
	return nil
}

func (b *Bus) CheckRecipientRemoval(id string) error { return checkRecipientRemoval(b.db, id) }

func currentTeamLeadReason(t querier, id string) (string, error) {
	var title string
	err := t.QueryRow(`SELECT CASE WHEN title <> '' THEN title ELSE name END FROM rooms WHERE lead = ?`, id).Scan(&title)
	if errors.Is(err, sql.ErrNoRows) {
		name, err := knownSession(t, id)
		return TeamLeadAssignmentReason(name + "'s team"), err
	}
	return TeamLeadAssignmentReason(title), err
}

func leadsTeam(t querier, id string) (bool, error) {
	var n int
	err := t.QueryRow(`SELECT COUNT(*) FROM team_members WHERE lead = ?`, id).Scan(&n)
	return n > 0, err
}

func leadsAvailableMember(t querier, id string) (bool, error) {
	var n int
	err := t.QueryRow(`SELECT COUNT(*) FROM team_members tm WHERE tm.lead = ? AND EXISTS (SELECT 1 FROM sessions s WHERE s.id = tm.member AND s.removed_at IS NULL)`, id).Scan(&n)
	return n > 0, err
}

// leaveTeamOfGroup ends a chat's membership of the team whose group it left.
func leaveTeamOfGroup(t *sql.Tx, group, member string) error {
	var lead string
	err := t.QueryRow(`SELECT lead FROM rooms WHERE name = ?`, group).Scan(&lead)
	if errors.Is(err, sql.ErrNoRows) || lead == "" {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := t.Exec(`DELETE FROM team_members WHERE member = ? AND lead = ?`, member, lead); err != nil {
		return err
	}
	return clearEmptyTeam(t, lead)
}

// DropTeamMembersOutsideTheirGroup ends the team membership of each chat that
// is not in its team's group and that gone accepts. A member joins the group
// when it joins the team, and a chat that left the group by itself used to
// keep its team row. The caller says which of those chats are gone for it,
// because a chat can be out of the group for other reasons and still be a
// member. A team whose lead has no group is not touched. It returns how many
// memberships it ended.
func (b *Bus) DropTeamMembersOutsideTheirGroup(ctx context.Context, gone func(member string) bool) (int, error) {
	dropped := 0
	err := b.tx(ctx, func(t *sql.Tx) error {
		rows, err := t.Query(`SELECT tm.member, tm.lead FROM team_members tm JOIN rooms r ON r.lead = tm.lead
			WHERE NOT EXISTS (SELECT 1 FROM room_members rm WHERE rm.room = r.name AND rm.member_kind = 'session' AND rm.member = tm.member)`)
		if err != nil {
			return err
		}
		type pair struct{ member, lead string }
		pairs := []pair{}
		for rows.Next() {
			var p pair
			if err := rows.Scan(&p.member, &p.lead); err != nil {
				rows.Close()
				return err
			}
			pairs = append(pairs, p)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, p := range pairs {
			if !gone(p.member) {
				continue
			}
			if _, err := t.Exec(`DELETE FROM team_members WHERE member = ? AND lead = ?`, p.member, p.lead); err != nil {
				return err
			}
			if err := clearEmptyTeam(t, p.lead); err != nil {
				return err
			}
			dropped++
		}
		return nil
	})
	return dropped, err
}

func clearEmptyTeam(t *sql.Tx, lead string) error {
	_, err := t.Exec(`UPDATE rooms SET lead = '' WHERE lead = ? AND NOT EXISTS (SELECT 1 FROM team_members WHERE lead = ?)`, lead, lead)
	return err
}

func checkReplaceTeamLead(t querier, group, oldLead, newLead string) (string, error) {
	key, _, err := findGroup(t, group)
	if err != nil {
		return "", err
	}
	var current string
	if err := t.QueryRow(`SELECT lead FROM rooms WHERE name = ?`, key).Scan(&current); err != nil {
		return "", err
	}
	if current == "" || oldLead == "" || current != oldLead || oldLead == newLead {
		return "", errors.New("the team's lead changed; refresh the chart")
	}
	for _, id := range []string{oldLead, newLead} {
		var active int
		if err := t.QueryRow(`SELECT COUNT(*) FROM sessions WHERE id = ? AND removed_at IS NULL`, id).Scan(&active); err != nil {
			return "", err
		}
		if active != 1 {
			return "", errors.New("both team leads must be available chats")
		}
	}
	parent, retired, err := teamLeadOf(t, newLead)
	if err != nil {
		return "", err
	}
	if parent != oldLead || retired {
		return "", errors.New("choose a current, non-retired member of this team")
	}
	var members int
	if err := t.QueryRow(`SELECT COUNT(*) FROM room_members WHERE room = ? AND member_kind = 'session' AND member IN (?, ?)`, key, oldLead, newLead).Scan(&members); err != nil {
		return "", err
	}
	if members != 2 {
		return "", errors.New("the team's channel membership needs repair before replacing its lead")
	}
	if yes, err := leadsTeam(t, newLead); err != nil {
		return "", err
	} else if yes {
		return "", errors.New("this member already leads a team; resolve its own team before making it this team's lead")
	}
	if parent, _, err := teamLeadOf(t, oldLead); err != nil {
		return "", err
	} else if parent != "" {
		return "", errors.New("the current lead also belongs to another team; resolve that reporting line first")
	}
	var conflicts int
	if err := t.QueryRow(`SELECT COUNT(*) FROM rooms WHERE lead = ? AND name <> ?`, newLead, key).Scan(&conflicts); err != nil {
		return "", err
	}
	if conflicts > 0 {
		return "", errors.New("this member already owns a team channel; resolve it before replacing the lead")
	}
	if err := t.QueryRow(`SELECT COUNT(*) FROM team_budgets WHERE lead = ?`, newLead).Scan(&conflicts); err != nil {
		return "", err
	}
	if conflicts > 0 {
		return "", errors.New("this member has its own team allowances; resolve them before replacing the lead")
	}
	return key, nil
}

// CheckReplaceTeamLead exposes the same preconditions the transaction checks.
func (b *Bus) CheckReplaceTeamLead(group, oldLead, newLead string) error {
	_, err := checkReplaceTeamLead(b.db, group, oldLead, newLead)
	return err
}

// ReplaceTeamLead retains the team channel and changes all reporting rows in
// one transaction. The expected old lead prevents a stale request succeeding.
func (b *Bus) ReplaceTeamLead(ctx context.Context, group, oldLead, newLead string) error {
	return b.replaceTeamLead(ctx, group, oldLead, newLead, b.teamLeadAcceptanceCheckpoint())
}

// The private checkpoint lets tests hold every partial write before commit.
// Public mutations never supply a callback.
func (b *Bus) replaceTeamLead(ctx context.Context, group, oldLead, newLead string, checkpoint func() error) error {
	return b.tx(ctx, func(t *sql.Tx) error {
		key, err := checkReplaceTeamLead(t, group, oldLead, newLead)
		if err != nil {
			return err
		}
		writes := []struct {
			query string
			args  []any
		}{
			{`UPDATE team_members SET lead = ? WHERE lead = ? AND member <> ?`, []any{newLead, oldLead, newLead}},
			// The old lead was never started as a member, so it has no remit; the new lead's was its own.
			{`UPDATE team_members SET member = ?, lead = ?, since = ?, retired = 0, remit = '' WHERE member = ? AND lead = ?`, []any{oldLead, newLead, b.stamp(), newLead, oldLead}},
			{`UPDATE rooms SET lead = ? WHERE name = ?`, []any{newLead, key}},
			{`UPDATE team_budgets SET lead = ? WHERE lead = ?`, []any{newLead, oldLead}},
		}
		for _, write := range writes {
			if _, err := t.Exec(write.query, write.args...); err != nil {
				return err
			}
			if checkpoint != nil {
				if err := checkpoint(); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
