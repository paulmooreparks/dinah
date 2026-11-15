//go:build org_chart_acceptance

package bus

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
)

var teamLeadAcceptanceHooks sync.Map
var teamLeadAcceptancePaths sync.Map

func (b *Bus) teamLeadAcceptanceCheckpoint() func() error {
	if hook, ok := teamLeadAcceptanceHooks.Load(b); ok {
		return hook.(func() error)
	}
	return nil
}

// SetTeamLeadAcceptanceCheckpoint exists only in the isolated receiving build.
func (b *Bus) SetTeamLeadAcceptanceCheckpoint(hook func() error) {
	if hook == nil {
		teamLeadAcceptanceHooks.Delete(b)
		teamLeadAcceptancePaths.Delete(b)
	} else {
		var sequence int
		var name, path string
		if err := b.db.QueryRow(`PRAGMA database_list`).Scan(&sequence, &name, &path); err != nil {
			panic(err)
		}
		teamLeadAcceptancePaths.Store(b, path)
		teamLeadAcceptanceHooks.Store(b, hook)
	}
}

// AcceptanceCommittedRelations reads the real store through an independent
// read-only connection while the App's writer is held before commit.
func (b *Bus) AcceptanceCommittedRelations(ctx context.Context) (AcceptanceRelations, error) {
	stored, ok := teamLeadAcceptancePaths.Load(b)
	if !ok {
		return AcceptanceRelations{}, fmt.Errorf("committed-read checkpoint is not installed")
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(stored.(string))+"?mode=ro&_txlock=deferred&_pragma=busy_timeout(10000)")
	if err != nil {
		return AcceptanceRelations{}, err
	}
	defer db.Close()
	return (&Bus{db: db}).AcceptanceRelations(ctx)
}

// AcceptanceSeedLegacyTeam represents records made before nesting was refused.
// It does not replace admission for any mutation exercised by receiving tests.
func (b *Bus) AcceptanceSeedLegacyTeam(ctx context.Context, member, lead string) error {
	return b.tx(ctx, func(tx *sql.Tx) error { _, err := b.assignLead(tx, member, lead); return err })
}

// AcceptanceRelations retains channel budgets and allowances beside the real
// relation query. The fixture serializes mutations; checkpoints use a separate
// connection while the writer remains uncommitted.
type AcceptanceRelations struct {
	TeamRelations
	Allowances     map[string][]TeamAllowance
	SessionRecords []map[string]any
	RoomRecords    map[string]map[string]any
}

func (b *Bus) AcceptanceRelations(ctx context.Context) (AcceptanceRelations, error) {
	relations, err := b.TeamRelations(ctx)
	out := AcceptanceRelations{TeamRelations: relations, Allowances: map[string][]TeamAllowance{}}
	if err != nil {
		return out, err
	}
	for i := range out.Rooms {
		room := &out.Rooms[i]
		if err := b.db.QueryRowContext(ctx, `SELECT turn_budget, turns_used FROM rooms WHERE name = ?`, room.Name).Scan(&room.Budget, &room.Used); err != nil {
			return out, err
		}
		allowances := []TeamAllowance{}
		if room.Lead != "" {
			rows, err := b.TeamAllowances(ctx, room.Lead)
			if err != nil {
				return out, err
			}
			allowances = append(allowances, rows...)
		}
		out.Allowances[room.Name] = allowances
	}
	var readErr error
	out.SessionRecords, readErr = b.acceptanceTableRecords(ctx, "sessions")
	if readErr != nil {
		return out, readErr
	}
	records, readErr := b.acceptanceTableRecords(ctx, "rooms")
	if readErr != nil {
		return out, readErr
	}
	out.RoomRecords = map[string]map[string]any{}
	for _, record := range records {
		out.RoomRecords[record["name"].(string)] = record
	}
	return out, nil
}

func (b *Bus) acceptanceTableRecords(ctx context.Context, table string) ([]map[string]any, error) {
	// Only the two literal fixture-owned table names reach this helper.
	query := "SELECT * FROM sessions ORDER BY id"
	if table == "rooms" {
		query = "SELECT * FROM rooms ORDER BY name"
	}
	rows, err := b.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	out := []map[string]any{}
	for rows.Next() {
		values, targets := make([]any, len(columns)), make([]any, len(columns))
		for i := range targets {
			targets[i] = &values[i]
		}
		if err := rows.Scan(targets...); err != nil {
			return nil, err
		}
		record := map[string]any{}
		for i, name := range columns {
			record[name] = values[i]
		}
		out = append(out, record)
	}
	return out, rows.Err()
}
