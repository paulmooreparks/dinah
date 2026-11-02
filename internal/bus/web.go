package bus

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"studio/internal/profile"
)

// CodeLifetime is how long a sign-in code from dinah open stays usable.
const CodeLifetime = 2 * time.Minute

// tokenDigest is the database key; the bearer value remains only in the cookie.
func tokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomHex(n int) string {
	buf := make([]byte, n)
	rand.Read(buf)
	return hex.EncodeToString(buf)
}

// NewWebCode makes a single-use code that signs a browser in to the page.
func (b *Bus) NewWebCode(ctx context.Context) (string, error) {
	code := randomHex(16)
	err := b.tx(ctx, func(t *sql.Tx) error {
		if _, err := t.Exec(`DELETE FROM web_codes WHERE expires_at < ?`, b.stamp()); err != nil {
			return err
		}
		_, err := t.Exec(`INSERT INTO web_codes (code, expires_at) VALUES (?, ?)`,
			code, b.now().Add(CodeLifetime).UTC().Format(time.RFC3339Nano))
		return err
	})
	return code, err
}

// ExchangeWebCode spends a sign-in code and returns a token for the page's
// cookie.
func (b *Bus) ExchangeWebCode(ctx context.Context, code string) (string, error) {
	token := randomHex(32)
	err := b.tx(ctx, func(t *sql.Tx) error {
		var expires string
		err := t.QueryRow(`SELECT expires_at FROM web_codes WHERE code = ?`, code).Scan(&expires)
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("that sign-in code is unknown or already used; run " + profile.Current().CommandProgram + " open again")
		}
		if err != nil {
			return err
		}
		if _, err := t.Exec(`DELETE FROM web_codes WHERE code = ?`, code); err != nil {
			return err
		}
		expiresAt, parseErr := time.Parse(time.RFC3339Nano, expires)
		if parseErr != nil || !b.now().Before(expiresAt) {
			return errors.New("that sign-in code has expired; run " + profile.Current().CommandProgram + " open again")
		}
		stamp := b.stamp()
		_, err = t.Exec(`INSERT INTO web_sessions (token, token_version, created_at, last_used_at) VALUES (?, 1, ?, ?)`, tokenDigest(token), stamp, stamp)
		return err
	})
	return token, err
}

// ValidWebToken reports whether a cookie's token signs a browser in.
func (b *Bus) ValidWebToken(ctx context.Context, token string) bool {
	return b.validateWebToken(ctx, token, true)
}

// CheckWebToken checks continuing authority without extending idle lifetime.
// Passive event streams must not keep a forgotten browser signed in forever.
func (b *Bus) CheckWebToken(ctx context.Context, token string) bool {
	return b.validateWebToken(ctx, token, false)
}

func (b *Bus) validateWebToken(ctx context.Context, token string, touch bool) bool {
	if token == "" {
		return false
	}
	token = tokenDigest(token)
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return false
	}
	defer tx.Rollback()
	var created, used string
	var p WebSessionPolicy
	if err := tx.QueryRowContext(ctx, `SELECT s.created_at, s.last_used_at, p.idle_days, p.absolute_days
 FROM web_sessions s CROSS JOIN web_session_policy p WHERE s.token = ? AND s.token_version = 1 AND p.id = 1`, token).Scan(&created, &used, &p.IdleDays, &p.AbsoluteDays); err != nil {
		return false
	}
	if used == "" {
		used = created
	}
	issued, issueErr := time.Parse(time.RFC3339Nano, created)
	last, useErr := time.Parse(time.RFC3339Nano, used)
	now := b.now().UTC()
	// A backward wall-clock correction must not invalidate recent activity.
	if last.After(now) {
		last = now
	}
	valid := issueErr == nil && useErr == nil && p.valid() &&
		now.Before(issued.Add(time.Duration(p.AbsoluteDays)*24*time.Hour)) && now.Before(last.Add(time.Duration(p.IdleDays)*24*time.Hour))
	if !valid {
		_, err = tx.ExecContext(ctx, `DELETE FROM web_sessions WHERE token = ?`, token)
	} else if touch && now.Sub(last) >= time.Hour {
		_, err = tx.ExecContext(ctx, `UPDATE web_sessions SET last_used_at = ? WHERE token = ?`, now.Format(time.RFC3339Nano), token)
	}
	// Authentication metadata does not advance the bus change counter.
	return err == nil && tx.Commit() == nil && valid
}

// WebSessionPolicy contains persisted whole-day lifetime settings.
type WebSessionPolicy struct {
	IdleDays     int `json:"idleDays"`
	AbsoluteDays int `json:"absoluteDays"`
}

func (p WebSessionPolicy) valid() bool {
	return p.IdleDays >= 1 && p.IdleDays <= 3650 && p.AbsoluteDays >= 1 && p.AbsoluteDays <= 3650
}

func (b *Bus) WebSessionPolicy(ctx context.Context) (WebSessionPolicy, error) {
	var p WebSessionPolicy
	err := b.db.QueryRowContext(ctx, `SELECT idle_days, absolute_days FROM web_session_policy WHERE id = 1`).Scan(&p.IdleDays, &p.AbsoluteDays)
	if err == nil && !p.valid() {
		err = errors.New("browser session lifetime settings must be between 1 and 3650 days")
	}
	return p, err
}

func (b *Bus) SetWebSessionPolicy(ctx context.Context, p WebSessionPolicy) error {
	if !p.valid() {
		return errors.New("browser session lifetime settings must be between 1 and 3650 days")
	}
	_, err := b.db.ExecContext(ctx, `UPDATE web_session_policy SET idle_days = ?, absolute_days = ? WHERE id = 1`, p.IdleDays, p.AbsoluteDays)
	return err
}

func (b *Bus) RevokeWebToken(ctx context.Context, token string) error {
	_, err := b.db.ExecContext(ctx, `DELETE FROM web_sessions WHERE token = ?`, tokenDigest(token))
	return err
}

// RevokeAllWebTokens also retires outstanding sign-in links.
func (b *Bus) RevokeAllWebTokens(ctx context.Context) error {
	tx, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM web_sessions`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM web_codes`); err != nil {
		return err
	}
	return tx.Commit()
}
