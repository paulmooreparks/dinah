package judge

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"studio/internal/appletcontext"

	_ "modernc.org/sqlite"
)

// DefaultRetention is how long a record is kept unless the person sets another.
const DefaultRetention = 90 * 24 * time.Hour

// Record is one call, as the page and the widget read it.
type Record struct {
	ID      int64     `json:"id"`
	Time    time.Time `json:"time"`
	Kind    string    `json:"kind"`
	Version string    `json:"version"`
	// Input is the JSON the judge was shown, and Answer the text it returned.
	Input  string `json:"input"`
	Answer string `json:"answer"`
	// RuleAnswer is the rule's answer, recorded on every call so the two can be
	// compared. Out is the answer that was used.
	RuleAnswer string `json:"ruleAnswer"`
	Out        string `json:"out"`
	// Used is judge or rule, and Why says why the rule was used, or is empty.
	Used        string  `json:"used"`
	Why         string  `json:"why"`
	Provider    string  `json:"provider"`
	Model       string  `json:"model"`
	Temperature float64 `json:"temperature"`
	Seed        int     `json:"seed"`
	DeadlineMs  int64   `json:"deadlineMs"`
	ElapsedMs   int64   `json:"elapsedMs"`
	CostUnit    string  `json:"costUnit"`
	CostIn      int     `json:"costIn"`
	CostOut     int     `json:"costOut"`
	// EffectKind and EffectID say where the decision was applied, such as a chat
	// and its id.
	EffectKind string `json:"effectKind"`
	EffectID   string `json:"effectId"`
	// Correction is the person's replacement for the decision, and CorrectedAt
	// when they made it; the first is empty and the second is left out of the
	// JSON for a decision that was not corrected.
	Correction  string     `json:"correction"`
	CorrectedAt *time.Time `json:"correctedAt,omitempty"`
}

// Store is the file of records, in SQLite.
type Store struct {
	db  *sql.DB
	now func() time.Time

	mu        sync.Mutex
	retention time.Duration
	pruned    time.Time
}

const schema = `
CREATE TABLE IF NOT EXISTS calls (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	at INTEGER NOT NULL,
	kind TEXT NOT NULL,
	version TEXT NOT NULL,
	input TEXT NOT NULL DEFAULT '',
	answer TEXT NOT NULL DEFAULT '',
	rule_answer TEXT NOT NULL DEFAULT '',
	out TEXT NOT NULL DEFAULT '',
	used TEXT NOT NULL,
	why TEXT NOT NULL DEFAULT '',
	provider TEXT NOT NULL DEFAULT '',
	model TEXT NOT NULL DEFAULT '',
	temperature REAL NOT NULL DEFAULT 0,
	seed INTEGER NOT NULL DEFAULT 0,
	deadline_ms INTEGER NOT NULL DEFAULT 0,
	elapsed_ms INTEGER NOT NULL DEFAULT 0,
	cost_unit TEXT NOT NULL DEFAULT '',
	cost_in INTEGER NOT NULL DEFAULT 0,
	cost_out INTEGER NOT NULL DEFAULT 0,
	effect_kind TEXT NOT NULL DEFAULT '',
	effect_id TEXT NOT NULL DEFAULT '',
	correction TEXT NOT NULL DEFAULT '',
	corrected_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS calls_at ON calls (at);
CREATE INDEX IF NOT EXISTS calls_kind ON calls (kind, at);
CREATE INDEX IF NOT EXISTS calls_effect ON calls (effect_kind, effect_id);
CREATE INDEX IF NOT EXISTS calls_provider ON calls (provider, at);
`

// OpenStore opens the file of records at path, making it if it is not there.
func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("prepare the judgements file: %w", err)
	}
	s := &Store{db: db, now: time.Now, retention: DefaultRetention}
	if err := s.Prune(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the file.
func (s *Store) Close() error { return s.db.Close() }

// SetRetention sets how long records are kept; the person's setting. Zero or
// less keeps the default.
func (s *Store) SetRetention(d time.Duration) {
	if d <= 0 {
		d = DefaultRetention
	}
	s.mu.Lock()
	s.retention = d
	s.mu.Unlock()
}

// Prune drops the records older than the retention.
func (s *Store) Prune() error {
	s.mu.Lock()
	cutoff := s.now().Add(-s.retention)
	s.pruned = s.now()
	s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM calls WHERE at < ?`, cutoff.UnixMilli())
	return err
}

// Add writes a record and returns its id. A record older than the retention is
// pruned on the hour.
func (s *Store) Add(r Record) (int64, error) {
	s.mu.Lock()
	due := s.now().Sub(s.pruned) > time.Hour
	s.mu.Unlock()
	if due {
		if err := s.Prune(); err != nil {
			return 0, err
		}
	}
	res, err := s.db.Exec(`INSERT INTO calls (at, kind, version, input, answer, rule_answer, out, used, why, provider, model,
		temperature, seed, deadline_ms, elapsed_ms, cost_unit, cost_in, cost_out, effect_kind, effect_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.Time.UnixMilli(), r.Kind, r.Version, appletcontext.Redact(r.Input), appletcontext.Redact(r.Answer), r.RuleAnswer, r.Out, r.Used, r.Why,
		r.Provider, r.Model, r.Temperature, r.Seed, r.DeadlineMs, r.ElapsedMs, r.CostUnit, r.CostIn, r.CostOut, r.EffectKind, r.EffectID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Correct keeps the person's replacement for a decision. It is an error to
// correct a decision that does not exist.
func (s *Store) Correct(id int64, value string) error {
	res, err := s.db.Exec(`UPDATE calls SET correction = ?, corrected_at = ? WHERE id = ?`, value, s.now().UnixMilli(), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("there is no judgement %d", id)
	}
	return nil
}

// Filter narrows a list of records. Zero fields match everything.
type Filter struct {
	From, To time.Time
	Kind     string
	// Used is judge or rule; Why selects one reason.
	Used, Why string
	Model     string
	// Effect selects the decisions applied to one thing.
	EffectKind, EffectID string
	// Before is the cursor: only records with an id below it.
	Before int64
	// Limit is the most records to return; zero means 50 and the most is 500.
	Limit int
}

func (f Filter) where() (string, []any) {
	var cond []string
	var args []any
	add := func(c string, a ...any) { cond = append(cond, c); args = append(args, a...) }
	if !f.From.IsZero() {
		add("at >= ?", f.From.UnixMilli())
	}
	if !f.To.IsZero() {
		add("at < ?", f.To.UnixMilli())
	}
	if f.Kind != "" {
		add("kind = ?", f.Kind)
	}
	if f.Used != "" {
		add("used = ?", f.Used)
	}
	if f.Why != "" {
		add("why = ?", f.Why)
	}
	if f.Model != "" {
		add("model = ?", f.Model)
	}
	if f.EffectKind != "" {
		add("effect_kind = ?", f.EffectKind)
	}
	if f.EffectID != "" {
		add("effect_id = ?", f.EffectID)
	}
	if f.Before > 0 {
		add("id < ?", f.Before)
	}
	if len(cond) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(cond, " AND "), args
}

const columns = `id, at, kind, version, input, answer, rule_answer, out, used, why, provider, model, temperature, seed,
	deadline_ms, elapsed_ms, cost_unit, cost_in, cost_out, effect_kind, effect_id, correction, corrected_at`

// List returns the records that match, newest first, and the cursor for the next
// page: the id to pass as Before, or 0 when there are no more.
func (s *Store) List(f Filter) ([]Record, int64, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	where, args := f.where()
	rows, err := s.db.Query(`SELECT `+columns+` FROM calls`+where+` ORDER BY id DESC LIMIT ?`, append(args, limit+1)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []Record
	for rows.Next() {
		var r Record
		var at, corrected int64
		if err := rows.Scan(&r.ID, &at, &r.Kind, &r.Version, &r.Input, &r.Answer, &r.RuleAnswer, &r.Out, &r.Used, &r.Why, &r.Provider, &r.Model,
			&r.Temperature, &r.Seed, &r.DeadlineMs, &r.ElapsedMs, &r.CostUnit, &r.CostIn, &r.CostOut, &r.EffectKind, &r.EffectID, &r.Correction, &corrected); err != nil {
			return nil, 0, err
		}
		r.Time = time.UnixMilli(at).UTC()
		if corrected != 0 {
			at := time.UnixMilli(corrected).UTC()
			r.CorrectedAt = &at
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var next int64
	if len(out) > limit {
		out = out[:limit]
		next = out[limit-1].ID
	}
	return out, next, nil
}

// Get returns one record.
func (s *Store) Get(id int64) (Record, error) {
	rows, _, err := s.List(Filter{Before: id + 1, Limit: 1})
	if err != nil {
		return Record{}, err
	}
	if len(rows) == 0 || rows[0].ID != id {
		return Record{}, errors.New("there is no such judgement")
	}
	return rows[0], nil
}

// Spent is what the calls of a provider cost in a unit from from until to: the
// units counted in and counted out.
func (s *Store) Spent(provider, unit string, from, to time.Time) (int, error) {
	var n sql.NullInt64
	err := s.db.QueryRow(`SELECT SUM(cost_in + cost_out) FROM calls WHERE provider = ? AND cost_unit = ? AND at >= ? AND at < ?`,
		provider, unit, from.UnixMilli(), to.UnixMilli()).Scan(&n)
	return int(n.Int64), err
}

// Unmeasured counts the calls of a provider in a period whose cost it did not
// report.
func (s *Store) Unmeasured(provider string, from, to time.Time) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM calls WHERE provider = ? AND cost_unit = ? AND at >= ? AND at < ?`,
		provider, UnitUnknown, from.UnixMilli(), to.UnixMilli()).Scan(&n)
	return n, err
}

// CompleteLate writes what a call that was answered by its rule for being late
// cost when its request ended, with the answer it gave, which is kept and was not
// used. It changes only a record whose reason is late.
func (s *Store) CompleteLate(id int64, answer, model string, temperature float64, seed int, unit string, in, out int) error {
	_, err := s.db.Exec(`UPDATE calls SET answer = ?, model = ?, temperature = ?, seed = ?, cost_unit = ?, cost_in = ?, cost_out = ? WHERE id = ? AND why = ?`,
		answer, model, temperature, seed, unit, in, out, id, WhyLate)
	return err
}

// Group is the calls of one kind on one model in one unit, over a period.
type Group struct {
	Kind      string `json:"kind"`
	Model     string `json:"model"`
	CostUnit  string `json:"costUnit"`
	Calls     int    `json:"calls"`
	Fallbacks int    `json:"fallbacks"`
	// Disagreed is how many calls the judge answered differently from the rule.
	Disagreed int   `json:"disagreed"`
	Corrected int   `json:"corrected"`
	AvgMs     int64 `json:"avgMs"`
	MaxMs     int64 `json:"maxMs"`
	CostIn    int   `json:"costIn"`
	CostOut   int   `json:"costOut"`
}

// Summary is what the widget and the graph read from the one file.
type Summary struct {
	From      time.Time `json:"from"`
	To        time.Time `json:"to"`
	Calls     int       `json:"calls"`
	Fallbacks int       `json:"fallbacks"`
	Groups    []Group   `json:"groups"`
	// Latest is when the newest call was made, in any period, and Streak how many
	// of the newest calls in a row fell back to the rule for a reason other than
	// the person's off switch. A judge that has stopped answering shows as a
	// streak that grows.
	Latest *time.Time `json:"latest,omitempty"`
	Streak int        `json:"streak"`
}

// Summarize totals the calls from from to to (to zero means now) by kind, model
// and unit, with the latest call and the current run of fallbacks.
func (s *Store) Summarize(from, to time.Time) (Summary, error) {
	if to.IsZero() {
		to = s.now().Add(time.Millisecond)
	}
	sum := Summary{From: from, To: to}
	rows, err := s.db.Query(`SELECT kind, model, cost_unit, COUNT(*), SUM(used = 'rule'),
			SUM(used = 'judge' AND out <> rule_answer), SUM(correction <> ''), AVG(elapsed_ms), MAX(elapsed_ms), SUM(cost_in), SUM(cost_out)
		FROM calls WHERE at >= ? AND at < ? GROUP BY kind, model, cost_unit ORDER BY kind, model, cost_unit`, from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return sum, err
	}
	for rows.Next() {
		var g Group
		var avg float64
		if err := rows.Scan(&g.Kind, &g.Model, &g.CostUnit, &g.Calls, &g.Fallbacks, &g.Disagreed, &g.Corrected, &avg, &g.MaxMs, &g.CostIn, &g.CostOut); err != nil {
			rows.Close()
			return sum, err
		}
		g.AvgMs = int64(avg + 0.5)
		sum.Calls += g.Calls
		sum.Fallbacks += g.Fallbacks
		sum.Groups = append(sum.Groups, g)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return sum, err
	}
	var latest sql.NullInt64
	if err := s.db.QueryRow(`SELECT MAX(at) FROM calls`).Scan(&latest); err != nil {
		return sum, err
	}
	if latest.Valid {
		at := time.UnixMilli(latest.Int64).UTC()
		sum.Latest = &at
	}
	recent, err := s.db.Query(`SELECT used, why FROM calls ORDER BY id DESC LIMIT 1000`)
	if err != nil {
		return sum, err
	}
	defer recent.Close()
	for recent.Next() {
		var used, why string
		if err := recent.Scan(&used, &why); err != nil {
			return sum, err
		}
		if used == UsedJudge {
			break
		}
		if why != WhyOff {
			sum.Streak++
		}
	}
	return sum, recent.Err()
}

// TypicalCost is the median cost of the calls a provider has recorded in its unit,
// and how many calls it rests on. A call that cost nothing, because the judge
// was never reached, and a call whose cost the provider did not report are left out.
// With no such call the count is 0.
func (s *Store) TypicalCost(provider, unit string) (median, calls int, err error) {
	rows, err := s.db.Query(`SELECT cost_in + cost_out FROM calls WHERE provider = ? AND cost_unit = ? AND cost_in + cost_out > 0 ORDER BY 1`, provider, unit)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	var costs []int
	for rows.Next() {
		var n int
		if err := rows.Scan(&n); err != nil {
			return 0, 0, err
		}
		costs = append(costs, n)
	}
	if err := rows.Err(); err != nil || len(costs) == 0 {
		return 0, 0, err
	}
	mid := len(costs) / 2
	if len(costs)%2 == 1 {
		return costs[mid], len(costs), nil
	}
	return (costs[mid-1] + costs[mid]) / 2, len(costs), nil
}

// TypicalCost is the typical cost of a provider's recorded calls (Store.TypicalCost).
func (j *Judges) TypicalCost(provider, unit string) (median, calls int, err error) {
	if j.store == nil {
		return 0, 0, nil
	}
	return j.store.TypicalCost(provider, unit)
}
