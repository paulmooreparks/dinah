package judge

import (
	"fmt"
	"time"
)

// An outcome is what became of a decision that the person was free to take
// or leave, such as a suggested reply: taken, dropped, or passed over for
// something else. It is kept beside the call's record and is not a
// correction, which says the judge was wrong; a suggestion the person did
// not take was only not what they chose to say.
//
// The outcomes of a kind, counted over a time, are how a change to what the
// judge is given is measured: the share that was taken before and after.

const outcomeSchema = `
CREATE TABLE IF NOT EXISTS outcomes (
	call_id INTEGER PRIMARY KEY,
	outcome TEXT NOT NULL,
	at INTEGER NOT NULL
);
`

// SetOutcome keeps what became of a decision. The first outcome stands: a
// suggestion that was taken is not then counted as passed over when the
// person sends it.
func (s *Store) SetOutcome(id int64, outcome string) error {
	if id <= 0 || outcome == "" {
		return fmt.Errorf("an outcome needs a judgement and a word for what became of it")
	}
	_, err := s.db.Exec(`INSERT INTO outcomes (call_id, outcome, at) SELECT id, ?, ? FROM calls WHERE id = ?
		ON CONFLICT(call_id) DO NOTHING`, outcome, s.now().UnixMilli(), id)
	return err
}

// Outcome is what became of a decision, or empty when nothing is recorded.
func (s *Store) Outcome(id int64) (string, error) {
	var outcome string
	err := s.db.QueryRow(`SELECT COALESCE((SELECT outcome FROM outcomes WHERE call_id = ?), '')`, id).Scan(&outcome)
	return outcome, err
}

// Outcomes counts what became of a kind's decisions from from until to, by
// outcome. A decision with no outcome yet is counted under the empty word.
func (s *Store) Outcomes(kind string, from, to time.Time) (map[string]int, error) {
	rows, err := s.db.Query(`SELECT COALESCE(o.outcome, ''), COUNT(*) FROM calls c LEFT JOIN outcomes o ON o.call_id = c.id
		WHERE c.kind = ? AND c.used = ? AND c.at >= ? AND c.at < ? GROUP BY 1`, kind, UsedJudge, from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var word string
		var n int
		if err := rows.Scan(&word, &n); err != nil {
			return nil, err
		}
		out[word] = n
	}
	return out, rows.Err()
}
