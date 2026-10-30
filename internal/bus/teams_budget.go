package bus

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"
)

// A team's budget (docs/plans/teams.md, Paul's answers) is optional and is not normalized across providers: it is a set of
// allowances, each in one provider's own unit, that the team's members on that provider share, nested teams included. Usage
// is what the provider reported for each turn of a member's chat. A turn for which a provider reported nothing is recorded
// as unknown and counted as nothing in a sum, never estimated; a sum says how many turns it could not count, so a figure
// that is not complete is never shown as if it were.

// TeamAllowance is a lead's allowance on one provider, in the provider's unit.
type TeamAllowance struct {
	Provider  string
	Unit      string
	Allowance int
}

// SetTeamAllowances replaces a lead's allowances; an empty list is no budget.
func (b *Bus) SetTeamAllowances(ctx context.Context, lead string, allowances []TeamAllowance) error {
	for _, a := range allowances {
		if strings.TrimSpace(a.Provider) == "" || strings.TrimSpace(a.Unit) == "" || a.Allowance < 0 {
			return errors.New("an allowance needs a provider, its unit and a count of 0 or more")
		}
	}
	return b.tx(ctx, func(t *sql.Tx) error {
		if _, err := t.Exec(`DELETE FROM team_budgets WHERE lead = ?`, lead); err != nil {
			return err
		}
		for _, a := range allowances {
			if _, err := t.Exec(`INSERT OR REPLACE INTO team_budgets (lead, provider, unit, allowance) VALUES (?, ?, ?, ?)`, lead, a.Provider, a.Unit, a.Allowance); err != nil {
				return err
			}
		}
		return nil
	})
}

// TeamAllowances are a lead's allowances, in the order of their providers.
func (b *Bus) TeamAllowances(ctx context.Context, lead string) ([]TeamAllowance, error) {
	rows, err := b.db.QueryContext(ctx, `SELECT provider, unit, allowance FROM team_budgets WHERE lead = ? ORDER BY provider`, lead)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TeamAllowance
	for rows.Next() {
		var a TeamAllowance
		if err := rows.Scan(&a.Provider, &a.Unit, &a.Allowance); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// RecordTurnUsage records what a member's turn used on its provider, in the provider's unit, or that the provider reported
// nothing for it (known false).
func (b *Bus) RecordTurnUsage(ctx context.Context, session, provider, unit string, amount int, known bool) error {
	return b.tx(ctx, func(t *sql.Tx) error {
		_, err := t.Exec(`INSERT INTO team_usage (session, provider, unit, at, amount, known) VALUES (?, ?, ?, ?, ?, ?)`,
			session, provider, unit, b.now().UnixMilli(), amount, known)
		return err
	})
}

// Usage is what a team's members used on one provider in a period.
type Usage struct {
	Provider string
	Unit     string
	// Used is the sum of the turns that were counted, and Unknown how many turns could not be, so that Used is complete
	// only when Unknown is 0.
	Used    int
	Unknown int
	Turns   int
}

// TeamUsage sums the usage of a lead's members, nested teams included, from from to to, by provider.
func (b *Bus) TeamUsage(ctx context.Context, lead string, from, to time.Time) ([]Usage, error) {
	members, err := b.teamTree(ctx, lead)
	if err != nil {
		return nil, err
	}
	byProvider := map[string]*Usage{}
	for _, m := range members {
		rows, err := b.db.QueryContext(ctx, `SELECT provider, unit, COALESCE(SUM(CASE WHEN known THEN amount ELSE 0 END), 0), COALESCE(SUM(CASE WHEN known THEN 0 ELSE 1 END), 0), COUNT(*)
			FROM team_usage WHERE session = ? AND at >= ? AND at < ? GROUP BY provider, unit`, m, from.UnixMilli(), to.UnixMilli())
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var u Usage
			if err := rows.Scan(&u.Provider, &u.Unit, &u.Used, &u.Unknown, &u.Turns); err != nil {
				rows.Close()
				return nil, err
			}
			key := u.Provider + "\x00" + u.Unit
			if cur := byProvider[key]; cur != nil {
				cur.Used, cur.Unknown, cur.Turns = cur.Used+u.Used, cur.Unknown+u.Unknown, cur.Turns+u.Turns
			} else {
				byProvider[key] = &u
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	var out []Usage
	for _, u := range byProvider {
		out = append(out, *u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Provider < out[j].Provider })
	return out, nil
}

// teamTree is every member under a lead, nested ones included, retired ones too.
func (b *Bus) teamTree(ctx context.Context, lead string) ([]string, error) {
	var out []string
	seen := map[string]bool{lead: true}
	queue := []string{lead}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		members, err := b.TeamMembers(ctx, cur)
		if err != nil {
			return nil, err
		}
		for _, m := range members {
			if seen[m.Session] {
				continue
			}
			seen[m.Session] = true
			out = append(out, m.Session)
			queue = append(queue, m.Session)
		}
	}
	return out, nil
}

// MemberUsage is what one chat's own turns used from from to to, by provider, with the turns that could not be counted.
func (b *Bus) MemberUsage(ctx context.Context, session string, from, to time.Time) ([]Usage, error) {
	rows, err := b.db.QueryContext(ctx, `SELECT provider, unit, COALESCE(SUM(CASE WHEN known THEN amount ELSE 0 END), 0), COALESCE(SUM(CASE WHEN known THEN 0 ELSE 1 END), 0), COUNT(*)
		FROM team_usage WHERE session = ? AND at >= ? AND at < ? GROUP BY provider, unit ORDER BY provider`, session, from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Usage
	for rows.Next() {
		var u Usage
		if err := rows.Scan(&u.Provider, &u.Unit, &u.Used, &u.Unknown, &u.Turns); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}
