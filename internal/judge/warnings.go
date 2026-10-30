package judge

import (
	"database/sql"
	"errors"
	"time"
)

// A warning is a call the rule answered because the judge could not: it failed, was late, answered out of form or failed
// the kind's check. A call the rule answered because the person turned judging off, or no judge was set up, or the
// budget refused it, or the provider is paused, is not a warning of its own. The warnings that count are those of the
// current streak, the newest calls since the judge last answered, that the person has not dismissed. Dismissing is kept in
// the file, so it survives a restart; a call that falls back after a dismissal is a new warning.

// Warning is one such call.
type Warning struct {
	ID   int64     `json:"id"`
	At   time.Time `json:"at"`
	Kind string    `json:"kind"`
	Why  string    `json:"why"`
	// Reason is what a person reads: the provider's own words for an error, shortened, else a sentence for the why.
	Reason string `json:"reason"`
}

// maxWarnings is how many warnings are listed.
const maxWarnings = 50

func warningReason(why, answer string) string {
	switch why {
	case WhyError:
		if r := ShortReason(answer); r != "" {
			return r
		}
		return "The judge failed"
	case WhyLate:
		return "The judge did not answer in time"
	case WhyForm:
		return "The judge's answer was not in the form asked for"
	case WhyCheck:
		return "The judge's answer failed the check"
	}
	return why
}

// Warnings are the undismissed warnings of the current streak, newest first.
func (s *Store) Warnings() ([]Warning, error) {
	var through int64
	err := s.db.QueryRow(`SELECT through FROM warning_marker WHERE id = 1`).Scan(&through)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT c.id, c.at, c.kind, c.used, c.why, c.answer, d.call_id IS NOT NULL
		FROM calls c LEFT JOIN warning_dismissals d ON d.call_id = c.id ORDER BY c.id DESC LIMIT 1000`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Warning{}
	for rows.Next() {
		var w Warning
		var at int64
		var used, answer string
		var dismissed bool
		if err := rows.Scan(&w.ID, &at, &w.Kind, &used, &w.Why, &answer, &dismissed); err != nil {
			return nil, err
		}
		if used == UsedJudge {
			break
		}
		if w.ID <= through || dismissed || len(out) >= maxWarnings {
			continue
		}
		switch w.Why {
		case WhyError, WhyLate, WhyForm, WhyCheck:
		default:
			continue
		}
		w.At = time.UnixMilli(at).UTC()
		w.Reason = warningReason(w.Why, answer)
		out = append(out, w)
	}
	return out, rows.Err()
}

// DismissWarning dismisses one warning and says whether the call was there to dismiss.
func (s *Store) DismissWarning(id int64) (bool, error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM calls WHERE id = ?`, id).Scan(&n); err != nil || n == 0 {
		return false, err
	}
	_, err := s.db.Exec(`INSERT OR IGNORE INTO warning_dismissals (call_id, at) VALUES (?, ?)`, id, s.now().UnixMilli())
	return true, err
}

// DismissWarnings dismisses every warning there is now, and says how many that was.
func (s *Store) DismissWarnings() (int, error) {
	ws, err := s.Warnings()
	if err != nil || len(ws) == 0 {
		return 0, err
	}
	var newest int64
	if err := s.db.QueryRow(`SELECT COALESCE(MAX(id), 0) FROM calls`).Scan(&newest); err != nil {
		return 0, err
	}
	_, err = s.db.Exec(`INSERT INTO warning_marker (id, through) VALUES (1, ?) ON CONFLICT(id) DO UPDATE SET through = MAX(through, excluded.through)`, newest)
	return len(ws), err
}
