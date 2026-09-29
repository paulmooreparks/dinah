package verb

import (
	"dinah/internal/bench"
	"dinah/internal/contract"
)

// MigrateStorage runs dinah check --migrate-storage: it carries the workbench
// from the old layout to the card-unit layout, or rehearses doing so.
//
// A rehearsal is refused to nobody and runs whether or not the layout is
// switched on, because it writes nothing. A writing run is the operator's
// alone, is refused while the layout is switched off, and is refused while
// any card, live or archived, is claimed, unless the operator forces it past
// the claims, which the run's own line on the workbench journal then names.
func (l *Library) MigrateStorage(req *Request) (*bench.StorageMigration, error) {
	if !req.Rehearse {
		if refused := l.malformedHarness(req, nil); refused != nil {
			return nil, contract.Refuse(contract.MalformedHarness, req.Harness)
		}
		if l.Bench.Operator == "" {
			return nil, contract.Refuse(contract.NoOperator, "")
		}
		if req.Actor != l.Bench.Operator {
			return nil, contract.Refuse(contract.NotOperator, req.Actor)
		}
		if !bench.CardUnitOn() {
			return nil, contract.Refuse(contract.MigrationAwaitsCapabilities, l.Bench.Root)
		}
	}
	if req.ForceClaims && req.Actor != l.Bench.Operator {
		return nil, contract.Refuse(contract.NotOperator, req.Actor)
	}
	// A store already at the card-unit format with no migration in progress
	// has nothing to migrate, so the claims a writing run is refused over are
	// not asked about: the answer is that it is already migrated, and telling
	// the operator to release claims first would be advice for a run with
	// nothing to do.
	migrated := l.Bench.Format >= bench.CardUnitFormat && l.Bench.Migrating == ""
	var passed []string
	if !req.Rehearse && !migrated {
		claimed, err := l.Bench.ClaimedCardsBothHalves()
		if err != nil {
			return nil, err
		}
		if len(claimed) > 0 && !req.ForceClaims {
			return nil, bench.StorageRefusalWorkbenchInUse(claimed)
		}
		if req.ForceClaims {
			passed = make([]string, 0, len(claimed))
			for _, card := range claimed {
				passed = append(passed, card.Ref)
			}
		}
	}
	return l.Bench.MigrateStorage(bench.StorageMigrationRun{
		Template: bench.Event{TS: bench.Stamp(l.Now()), Actor: req.Acting()},
		Backup:   req.Backup,
		Rehearse: req.Rehearse,
		Accept:   req.AcceptDifference,
		Claims:   passed,
	})
}
