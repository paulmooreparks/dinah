// Package fileaccess stores the person's host-mediated read policy and audit.
package fileaccess

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const auditRows = `(SELECT id,chat,at,outcome,payload FROM audit UNION ALL SELECT id,chat,at,outcome,payload FROM policy_changes UNION ALL SELECT id,chat,at,outcome,payload FROM denials)`

const auditTimeFormat = "2006-01-02T15:04:05.000000000Z"

const (
	AuditGenerationBytes = 1 << 20
	AuditGenerations     = 4
	MaxDatabaseBytes     = 16 << 20
	MaxAuditRecordBytes  = 16 << 10
	MaxCustomRules       = 256
)

// Store serializes policy changes with audit records in one database.
type Store struct {
	mu  sync.Mutex
	db  *sql.DB
	now func() time.Time
}

type Rule struct {
	Verification  string             `json:"verification,omitempty"`
	NetworkRoots  []NetworkRuleState `json:"networkRoots,omitempty"`
	paths         []string           // Original roots remain available for lexical and canonical checks.
	ID            string             `json:"id"`
	Kind          string             `json:"kind"`
	Pattern       string             `json:"pattern"`
	Reason        string             `json:"reason"`
	Enabled       bool               `json:"enabled"`
	ResolvedPaths []string           `json:"resolvedPaths"`
	Examples      map[string]string  `json:"examples"`
	Actions       []string           `json:"actions"`
}

type Policy struct {
	NetworkWorkers []NetworkWorker `json:"networkWorkers,omitempty"`
	Version        int64           `json:"version"`
	Rules          []Rule          `json:"rules"`
}

type Record struct {
	RuleIDs            []string `json:"ruleIDs,omitempty"`
	DeniedPaths        []string `json:"deniedPaths,omitempty"`
	DeniedPathsOmitted int      `json:"deniedPathsOmitted,omitempty"`
	DeniedBytes        *int     `json:"deniedBytes,omitempty"`
	UnverifiedRuleIDs  []string `json:"unverifiedRuleIDs,omitempty"`
	Operation          bool     `json:"operation,omitempty"`
	Visited            int      `json:"visited,omitempty"`
	Denied             int      `json:"denied,omitempty"`
	Errors             int      `json:"errors,omitempty"`
	Count              int      `json:"count,omitempty"`
	Source             string   `json:"source,omitempty"`
	ID                 int64    `json:"id"`
	At                 string   `json:"at"`
	Chat               string   `json:"chat"`
	ChatTitle          string   `json:"chatTitle"`
	Workspace          string   `json:"workspace"`
	Path               string   `json:"path"`
	Bytes              int      `json:"bytes"`
	Outcome            string   `json:"outcome"`
	RuleID             string   `json:"ruleID"`
	Reason             string   `json:"reason"`
}

type Filter struct {
	Chat    string `json:"chat"`
	Since   string `json:"since"`
	Outcome string `json:"outcome"`
	Before  int64  `json:"before"`
	Limit   int    `json:"limit"`
}

type Page struct {
	Rows           []Record `json:"rows"`
	NextBefore     int64    `json:"nextBefore"`
	HighWater      int64    `json:"highWater"`
	RetentionStart int64    `json:"retentionStart"`
}

func Open(path string) (*Store, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("file-access database needs an absolute path")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	uriPath := filepath.ToSlash(path)
	if runtime.GOOS == "windows" {
		uriPath = "/" + uriPath
	}
	uri := url.URL{Scheme: "file", Path: uriPath}
	db, err := sql.Open("sqlite", uri.String()+"?_txlock=immediate")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	fail := func(err error) (*Store, error) { db.Close(); return nil, err }
	var schema int
	if err := db.QueryRow("PRAGMA user_version").Scan(&schema); err != nil || schema > 3 || schema < 0 {
		return fail(errors.New("unsupported file-access database schema"))
	}
	for _, q := range []string{"PRAGMA busy_timeout=5000", "PRAGMA journal_mode=DELETE", "PRAGMA auto_vacuum=FULL", "PRAGMA synchronous=FULL"} {
		if _, err := db.Exec(q); err != nil {
			return fail(err)
		}
	}
	var pageSize, vacuum, maxPages, synchronous int
	var journal string
	if err := db.QueryRow("PRAGMA page_size").Scan(&pageSize); err != nil || pageSize <= 0 {
		return fail(errors.New("cannot determine audit database page size"))
	}
	if err := db.QueryRow(fmt.Sprintf("PRAGMA max_page_count=%d", MaxDatabaseBytes/pageSize)).Scan(&maxPages); err != nil || maxPages*pageSize > MaxDatabaseBytes {
		return fail(errors.New("audit database size bound unavailable"))
	}
	if err := db.QueryRow("PRAGMA synchronous").Scan(&synchronous); err != nil || synchronous != 2 {
		return fail(errors.New("audit database requires FULL synchronous commits"))
	}
	if err := db.QueryRow("PRAGMA auto_vacuum").Scan(&vacuum); err != nil || vacuum != 1 {
		return fail(errors.New("audit database requires FULL auto-vacuum"))
	}
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journal); err != nil || journal != "delete" {
		return fail(errors.New("audit database requires rollback journaling"))
	}
	tx, err := db.Begin()
	if err != nil {
		return fail(err)
	}
	defer tx.Rollback()
	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS policy (id INTEGER PRIMARY KEY CHECK(id=1), version INTEGER NOT NULL, high_water INTEGER NOT NULL);
 INSERT OR IGNORE INTO policy VALUES(1,1,0);
 CREATE TABLE IF NOT EXISTS custom_rules (id TEXT PRIMARY KEY, path TEXT NOT NULL UNIQUE);
 CREATE TABLE IF NOT EXISTS defaults (id TEXT PRIMARY KEY, enabled INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS acknowledgements (chat TEXT PRIMARY KEY, through_id INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS audit (id INTEGER PRIMARY KEY, generation INTEGER NOT NULL, bytes INTEGER NOT NULL, chat TEXT NOT NULL, at TEXT NOT NULL, outcome TEXT NOT NULL, payload TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS policy_changes (id INTEGER PRIMARY KEY, generation INTEGER NOT NULL, bytes INTEGER NOT NULL, chat TEXT NOT NULL, at TEXT NOT NULL, outcome TEXT NOT NULL, payload TEXT NOT NULL);
 INSERT OR IGNORE INTO policy_changes SELECT * FROM audit WHERE outcome='policy-change';
 DELETE FROM audit WHERE outcome='policy-change';
 CREATE TABLE IF NOT EXISTS denials (id INTEGER PRIMARY KEY, generation INTEGER NOT NULL, bytes INTEGER NOT NULL, chat TEXT NOT NULL, at TEXT NOT NULL, outcome TEXT NOT NULL, payload TEXT NOT NULL, count INTEGER NOT NULL, rule_id TEXT NOT NULL, source TEXT NOT NULL, sealed INTEGER NOT NULL DEFAULT 0);
 DROP INDEX IF EXISTS denial_open_group;
 CREATE UNIQUE INDEX IF NOT EXISTS denial_open_group ON denials(chat,rule_id,source,outcome) WHERE sealed=0;
 CREATE INDEX IF NOT EXISTS audit_chat ON audit(chat,id);
 CREATE INDEX IF NOT EXISTS audit_at ON audit(at,id);`)
	if err != nil {
		return fail(err)
	}
	if _, err := tx.Exec("PRAGMA user_version=3"); err != nil {
		return fail(err)
	}
	if err := tx.Commit(); err != nil {
		return fail(err)
	}
	if err := os.Chmod(path, 0600); err != nil {
		return fail(err)
	}
	return &Store{db: db, now: time.Now}, nil
}

func (s *Store) Close() error { s.mu.Lock(); defer s.mu.Unlock(); return s.db.Close() }

// Snapshot returns stored custom paths and optional-default switches. The host
// supplies immutable protected rules and platform-specific default resolutions.
func (s *Store) Snapshot() (int64, map[string]string, map[string]bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return 0, nil, nil, err
	}
	defer tx.Rollback()
	var version int64
	if err := tx.QueryRow(`SELECT version FROM policy WHERE id=1`).Scan(&version); err != nil {
		return 0, nil, nil, err
	}
	custom, defaults := map[string]string{}, map[string]bool{}
	rows, err := tx.Query(`SELECT id,path FROM custom_rules ORDER BY id`)
	if err != nil {
		return 0, nil, nil, err
	}
	for rows.Next() {
		var id, path string
		if err = rows.Scan(&id, &path); err != nil {
			break
		}
		custom[id] = path
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return 0, nil, nil, err
	}
	rows, err = tx.Query(`SELECT id,enabled FROM defaults`)
	if err != nil {
		return 0, nil, nil, err
	}
	for rows.Next() {
		var id string
		var enabled bool
		if err = rows.Scan(&id, &enabled); err != nil {
			break
		}
		defaults[id] = enabled
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return 0, nil, nil, err
	}
	if err = tx.Commit(); err != nil {
		return 0, nil, nil, err
	}
	return version, custom, defaults, nil
}

// Change is internal to the policy service. It accepts only validated custom
// rules or known optional-default identities, never a page-supplied SQL name.
func (s *Store) Change(version int64, operation, id, path string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var current int64
	if err = tx.QueryRow(`SELECT version FROM policy WHERE id=1`).Scan(&current); err != nil {
		return err
	}
	if current != version {
		return errors.New("the file-access policy changed; reload it before changing a rule")
	}
	record := Record{Outcome: "policy-change", RuleID: id, Path: path}
	switch operation {
	case "add":
		var count int
		if err = tx.QueryRow(`SELECT COUNT(*) FROM custom_rules`).Scan(&count); err != nil {
			return err
		}
		if count >= MaxCustomRules {
			return errors.New("the custom rule limit has been reached")
		}
		_, err = tx.Exec(`INSERT INTO custom_rules(id,path) VALUES(?,?)`, id, path)
		record.Reason = "Custom denial added"
	case "remove":
		if err = tx.QueryRow(`SELECT path FROM custom_rules WHERE id=?`, id).Scan(&record.Path); err != nil {
			return errors.New("only an existing custom rule can be removed")
		}
		_, err = tx.Exec(`DELETE FROM custom_rules WHERE id=?`, id)
		record.Reason = "Custom denial removed"
	case "default":
		var old bool
		e := tx.QueryRow(`SELECT enabled FROM defaults WHERE id=?`, id).Scan(&old)
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		if errors.Is(e, sql.ErrNoRows) {
			old = true
		}
		if old == enabled {
			return nil
		}
		_, err = tx.Exec(`INSERT INTO defaults(id,enabled) VALUES(?,?) ON CONFLICT(id) DO UPDATE SET enabled=excluded.enabled`, id, enabled)
		record.Reason = "Default denial disabled"
		if enabled {
			record.Reason = "Default denial enabled"
		}
	default:
		return errors.New("unknown file-access policy change")
	}
	if err != nil {
		return err
	}
	if _, err = s.append(tx, record); err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE policy SET version=version+1 WHERE id=1`); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Append(record Record) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return Record{}, err
	}
	defer tx.Rollback()
	record, err = s.append(tx, record)
	if err != nil {
		return Record{}, err
	}
	if err = tx.Commit(); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s *Store) append(tx *sql.Tx, record Record) (Record, error) {
	switch record.Outcome {
	case "allowed", "denied", "error", "policy-change":
	default:
		return Record{}, errors.New("invalid file-access audit outcome")
	}
	if record.Bytes < 0 || (!record.Operation && record.Outcome != "allowed" && record.Bytes != 0) {
		return Record{}, errors.New("invalid returned audit byte count")
	}
	var high int64
	if err := tx.QueryRow(`SELECT high_water FROM policy WHERE id=1`).Scan(&high); err != nil {
		return Record{}, err
	}
	record.ID, record.At = high+1, s.now().UTC().Format(auditTimeFormat)
	grouped := record.Outcome == "error" || (!record.Operation && record.Outcome == "denied")
	if grouped {
		var previous int64
		var count int
		var priorPayload string
		err := tx.QueryRow(`SELECT id,count,payload FROM denials WHERE chat=? AND rule_id=? AND source=? AND outcome=? AND sealed=0`, record.Chat, record.RuleID, record.Source, record.Outcome).Scan(&previous, &count, &priorPayload)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return Record{}, err
		}
		if err == nil {
			var prior Record
			if err := json.Unmarshal([]byte(priorPayload), &prior); err != nil {
				return Record{}, err
			}
			ids := map[string]bool{}
			for _, id := range prior.UnverifiedRuleIDs {
				ids[id] = true
			}
			for _, id := range record.UnverifiedRuleIDs {
				ids[id] = true
			}
			record.UnverifiedRuleIDs = nil
			for id := range ids {
				record.UnverifiedRuleIDs = append(record.UnverifiedRuleIDs, id)
			}
			sort.Strings(record.UnverifiedRuleIDs)
		}
		record.Count = count + 1
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return Record{}, err
	}
	if len(payload) > MaxAuditRecordBytes {
		return Record{}, errors.New("file-access audit metadata exceeds 16 KiB")
	}
	table := "audit"
	if record.Outcome == "policy-change" {
		table = "policy_changes"
	}
	if grouped {
		table = "denials"
		if _, err = tx.Exec(`DELETE FROM denials WHERE chat=? AND rule_id=? AND source=? AND outcome=? AND sealed=0`, record.Chat, record.RuleID, record.Source, record.Outcome); err != nil {
			return Record{}, err
		}
	}
	var generation, used int64
	if err = tx.QueryRow(`SELECT COALESCE(MAX(generation),1) FROM ` + table).Scan(&generation); err != nil {
		return Record{}, err
	}
	if err = tx.QueryRow(`SELECT COALESCE(SUM(bytes),0) FROM `+table+` WHERE generation=?`, generation).Scan(&used); err != nil {
		return Record{}, err
	}
	generationBudget := AuditGenerationBytes
	if table != "audit" {
		generationBudget = AuditGenerationBytes / 4
	}
	if used+int64(len(payload)) > int64(generationBudget) {
		generation++
	}
	if grouped {
		_, err = tx.Exec(`INSERT INTO denials(id,generation,bytes,chat,at,outcome,payload,count,rule_id,source) VALUES(?,?,?,?,?,?,?,?,?,?)`, record.ID, generation, len(payload), record.Chat, record.At, record.Outcome, string(payload), record.Count, record.RuleID, record.Source)
	} else {
		_, err = tx.Exec(`INSERT INTO `+table+`(id,generation,bytes,chat,at,outcome,payload) VALUES(?,?,?,?,?,?,?)`, record.ID, generation, len(payload), record.Chat, record.At, record.Outcome, string(payload))
	}
	if err != nil {
		return Record{}, err
	}
	if _, err = tx.Exec(`DELETE FROM `+table+` WHERE generation<=?`, generation-AuditGenerations); err != nil {
		return Record{}, err
	}
	if _, err = tx.Exec(`DELETE FROM acknowledgements WHERE NOT EXISTS(SELECT 1 FROM ` + auditRows + ` a WHERE a.chat=acknowledgements.chat)`); err != nil {
		return Record{}, err
	}
	if _, err = tx.Exec(`UPDATE policy SET high_water=? WHERE id=1`, record.ID); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s *Store) Log(filter Filter) (Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := Page{Rows: []Record{}}
	if filter.Limit == 0 {
		filter.Limit = 50
	}
	if len(filter.Chat) > 512 {
		return out, errors.New("audit chat filter exceeds 512 characters")
	}
	if filter.Limit < 1 || filter.Limit > 200 || filter.Before < 0 {
		return out, errors.New("audit page limit must be 1 to 200 and cursor nonnegative")
	}
	if filter.Since != "" {
		t, err := time.Parse(time.RFC3339, filter.Since)
		if err != nil {
			return out, errors.New("audit start time must be RFC 3339")
		}
		filter.Since = t.UTC().Format(auditTimeFormat)
	}
	switch filter.Outcome {
	case "", "allowed", "denied", "error", "policy-change":
	default:
		return out, errors.New("invalid audit outcome filter")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	if err := tx.QueryRow(`SELECT high_water FROM policy WHERE id=1`).Scan(&out.HighWater); err != nil {
		return out, err
	}
	if err := tx.QueryRow(`SELECT COALESCE(MIN(id),0) FROM `+auditRows+` WHERE (?='' OR outcome=?)`, filter.Outcome, filter.Outcome).Scan(&out.RetentionStart); err != nil {
		return out, err
	}
	if filter.Before > 0 && out.RetentionStart > 0 && filter.Before <= out.RetentionStart {
		return out, errors.New("that audit page is no longer retained; reload from the retention boundary")
	}
	// All stored timestamps use the same fixed-width UTC representation below.
	rows, err := tx.Query(`SELECT payload FROM `+auditRows+` WHERE (?='' OR chat=?) AND (?='' OR at>=?) AND (?='' OR outcome=?) AND (?=0 OR id<?) ORDER BY id DESC LIMIT ?`, filter.Chat, filter.Chat, filter.Since, filter.Since, filter.Outcome, filter.Outcome, filter.Before, filter.Before, filter.Limit+1)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			return out, err
		}
		var record Record
		if err = json.Unmarshal([]byte(raw), &record); err != nil {
			return out, err
		}
		if len(out.Rows) == filter.Limit {
			out.NextBefore = out.Rows[len(out.Rows)-1].ID
			break
		}
		out.Rows = append(out.Rows, record)
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	rows.Close()
	// A row that has been displayed is immutable. Later refusals form another
	// group, so acknowledging the displayed cursor cannot hide a new refusal.
	for _, row := range out.Rows {
		if row.Outcome == "denied" || row.Outcome == "error" {
			if _, err = tx.Exec(`UPDATE denials SET sealed=1 WHERE id=?`, row.ID); err != nil {
				return out, err
			}
		}
	}
	return out, tx.Commit()
}

func (s *Store) Acknowledge(chat string, through int64) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if chat == "" || len(chat) > 512 || through < 0 {
		return 0, errors.New("a chat and nonnegative audit cursor are required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var high int64
	if err = tx.QueryRow(`SELECT high_water FROM policy WHERE id=1`).Scan(&high); err != nil {
		return 0, err
	}
	if through > high {
		return 0, errors.New("audit cursor is newer than the displayed log")
	}
	var retained int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM `+auditRows+` WHERE chat=?`, chat).Scan(&retained); err != nil {
		return 0, err
	}
	if retained == 0 {
		return 0, nil
	}
	if _, err = tx.Exec(`INSERT INTO acknowledgements(chat,through_id) VALUES(?,?) ON CONFLICT(chat) DO UPDATE SET through_id=MAX(through_id,excluded.through_id)`, chat, through); err != nil {
		return 0, err
	}
	if _, err = tx.Exec(`UPDATE denials SET sealed=1 WHERE chat=? AND id<=?`, chat, through); err != nil {
		return 0, err
	}
	var count int
	if err = tx.QueryRow(`SELECT COALESCE(SUM(n),0) FROM (SELECT COALESCE(json_extract(payload,'$.count'),1) n FROM audit WHERE chat=? AND outcome='denied' AND id>COALESCE((SELECT through_id FROM acknowledgements WHERE chat=?),0) UNION ALL SELECT count n FROM denials WHERE chat=? AND outcome='denied' AND id>COALESCE((SELECT through_id FROM acknowledgements WHERE chat=?),0))`, chat, chat, chat, chat).Scan(&count); err != nil {
		return 0, err
	}
	return count, tx.Commit()
}

func (s *Store) RefusedCounts() (map[string]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	counts := map[string]int{}
	rows, err := s.db.Query(`SELECT a.chat,SUM(a.n) FROM (SELECT chat,id,COALESCE(json_extract(payload,'$.count'),1) n FROM audit WHERE outcome='denied' UNION ALL SELECT chat,id,count n FROM denials WHERE outcome='denied') a LEFT JOIN acknowledgements k ON k.chat=a.chat WHERE a.id>COALESCE(k.through_id,0) GROUP BY a.chat`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var chat string
		var n int
		if err = rows.Scan(&chat, &n); err != nil {
			return nil, err
		}
		counts[chat] = n
	}
	return counts, rows.Err()
}

func (s *Store) Metadata() (version, highWater int64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	err = s.db.QueryRow(`SELECT version,high_water FROM policy WHERE id=1`).Scan(&version, &highWater)
	return
}
