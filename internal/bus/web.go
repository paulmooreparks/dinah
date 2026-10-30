package bus

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"studio/internal/profile"
)

// CodeLifetime is how long a sign-in code from dinah open stays usable.
const CodeLifetime = 2 * time.Minute

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
		if expires < b.stamp() {
			return errors.New("that sign-in code has expired; run " + profile.Current().CommandProgram + " open again")
		}
		_, err = t.Exec(`INSERT INTO web_sessions (token, created_at) VALUES (?, ?)`, token, b.stamp())
		return err
	})
	return token, err
}

// ValidWebToken reports whether a cookie's token signs a browser in.
func (b *Bus) ValidWebToken(ctx context.Context, token string) bool {
	if token == "" {
		return false
	}
	var one int
	return b.db.QueryRowContext(ctx, `SELECT 1 FROM web_sessions WHERE token = ?`, token).Scan(&one) == nil
}
