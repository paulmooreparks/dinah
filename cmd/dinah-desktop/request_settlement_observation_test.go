//go:build quality_request_settlement && quality_typed_questions && quality_task_host

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"studio/internal/taskhost"
)

// inspect reads committed rows over a separate read-only connection. It does
// not turn a projection or an attempted command into evidence of a commit.
func (x *requestSettlementFixture) inspect(id string) (map[string]any, error) {
	a := x.typed.f.a
	source, err := a.RequestState(id)
	if err != nil {
		return nil, err
	}
	notices, err := a.Notifications()
	if err != nil {
		return nil, err
	}
	entries, err := a.DecisionEntries()
	if err != nil {
		return nil, err
	}
	questions, err := taskhost.QuestionRecords(configDir())
	if err != nil {
		return nil, err
	}
	notificationStoreMu.Lock()
	reports, err := readNotifications()
	notificationStoreMu.Unlock()
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(configDir(), "bus.db"))+"?mode=ro&_pragma=busy_timeout(10000)")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	tables := map[string]any{}
	for _, table := range []string{"messages", "deliveries", "person_requests", "decision_answer_origins", "request_settlements", "request_completion_obligations", "request_settlement_operations", "host_results"} {
		rows, err := tx.Query(`SELECT * FROM ` + table + ` ORDER BY rowid`)
		if err != nil {
			return nil, err
		}
		columns, err := rows.Columns()
		if err != nil {
			rows.Close()
			return nil, err
		}
		values := []map[string]any{}
		for rows.Next() {
			cells := make([]any, len(columns))
			pointers := make([]any, len(columns))
			for i := range cells {
				pointers[i] = &cells[i]
			}
			if err := rows.Scan(pointers...); err != nil {
				rows.Close()
				return nil, err
			}
			value := map[string]any{}
			for i, name := range columns {
				if data, ok := cells[i].([]byte); ok {
					value[name] = string(data)
				} else {
					value[name] = cells[i]
				}
			}
			values = append(values, value)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		tables[table] = values
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	out := map[string]any{"source": source, "notifications": notices, "entries": entries, "pending": a.PendingTaskPermissions(), "questions": questions, "reports": reports, "tables": tables, "observations": x.typed.observations()}
	if strings.HasPrefix(id, "task-q-") {
		e := x.typed.entry(id)
		if e == nil {
			return nil, fmt.Errorf("no actual waiter for %s", id)
		}
		receipt, err := x.typed.receipt(e)
		if err != nil {
			return nil, err
		}
		out["operationReceipt"] = receipt
		released := false
		select {
		case <-e.done:
			released = true
		default:
		}
		out["waiterReleased"] = released
	}
	return out, nil
}

func decodeSettlementIdentity(raw json.RawMessage) (string, error) {
	var in struct {
		ID string `json:"id"`
	}
	err := json.Unmarshal(raw, &in)
	return in.ID, err
}
