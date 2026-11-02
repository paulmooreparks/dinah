package bus

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestWebSessionLifetimeAndRevocation(t *testing.T) {
	ctx := context.Background()
	for _, mode := range []string{"idle", "sliding", "absolute", "passive", "single-revoke", "global-revoke", "legacy", "invalid-time", "policy"} {
		t.Run(mode, func(t *testing.T) {
			b, dir := open(t)
			now := time.Date(2026, 1, 1, 0, 0, 0, 123, time.UTC)
			issued := now
			b.now = func() time.Time { return now }
			issue := func() string {
				t.Helper()
				code, err := b.NewWebCode(ctx)
				if err != nil {
					t.Fatal(err)
				}
				token, err := b.ExchangeWebCode(ctx, code)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := b.ExchangeWebCode(ctx, code); err == nil {
					t.Fatal("sign-in code replay succeeded")
				}
				return token
			}
			token, other := issue(), issue()
			counter, _ := b.ChangeCounter(ctx)
			if !b.ValidWebToken(ctx, token) {
				t.Fatal("new session invalid")
			}
			if after, _ := b.ChangeCounter(ctx); after != counter {
				t.Fatal("token activity changed the message counter")
			}
			switch mode {
			case "idle", "legacy":
				if mode == "legacy" {
					if _, err := b.db.Exec(`UPDATE web_sessions SET last_used_at = '' WHERE token = ?`, token); err != nil {
						t.Fatal(err)
					}
				}
				now = issued.Add(30 * 24 * time.Hour)
				if b.ValidWebToken(ctx, token) {
					t.Fatal("idle token survived its boundary")
				}
			case "sliding", "absolute":
				for _, day := range []int{20, 40, 60, 80} {
					now = issued.Add(time.Duration(day) * 24 * time.Hour)
					if !b.ValidWebToken(ctx, token) {
						t.Fatal("legitimate session expired inside both windows")
					}
				}
				if mode == "absolute" {
					now = issued.Add(90 * 24 * time.Hour)
					if b.ValidWebToken(ctx, token) {
						t.Fatal("activity extended the absolute deadline")
					}
				}
			case "passive":
				now = issued.Add(29 * 24 * time.Hour)
				if !b.CheckWebToken(ctx, token) {
					t.Fatal("passive check invalid inside the window")
				}
				now = issued.Add(30 * 24 * time.Hour)
				if b.CheckWebToken(ctx, token) {
					t.Fatal("passive stream extended idle lifetime")
				}
			case "single-revoke":
				if err := b.RevokeWebToken(ctx, token); err != nil {
					t.Fatal(err)
				}
				if b.ValidWebToken(ctx, token) || !b.ValidWebToken(ctx, other) {
					t.Fatal("single-browser revocation changed the wrong scope")
				}
			case "global-revoke":
				code, _ := b.NewWebCode(ctx)
				if err := b.RevokeAllWebTokens(ctx); err != nil {
					t.Fatal(err)
				}
				if b.ValidWebToken(ctx, token) || b.ValidWebToken(ctx, other) {
					t.Fatal("global revocation left a token valid")
				}
				if _, err := b.ExchangeWebCode(ctx, code); err == nil {
					t.Fatal("outstanding link survived global sign-out")
				}
			case "invalid-time":
				if _, err := b.db.Exec(`UPDATE web_sessions SET last_used_at = 'invalid' WHERE token = ?`, token); err != nil {
					t.Fatal(err)
				}
				if b.ValidWebToken(ctx, token) {
					t.Fatal("invalid timestamp authenticated")
				}
			case "policy":
				p := WebSessionPolicy{IdleDays: 2, AbsoluteDays: 4}
				if err := b.SetWebSessionPolicy(ctx, p); err != nil {
					t.Fatal(err)
				}
				if err := b.SetWebSessionPolicy(ctx, WebSessionPolicy{IdleDays: 0, AbsoluteDays: 4}); err == nil {
					t.Fatal("invalid policy accepted")
				}
				reopened, err := OpenAt(filepath.Join(dir, "bus.db"))
				if err != nil {
					t.Fatal(err)
				}
				defer reopened.Close()
				if got, err := reopened.WebSessionPolicy(ctx); err != nil || got != p {
					t.Fatalf("policy was not persisted: %+v %v", got, err)
				}
				now = issued.Add(2 * 24 * time.Hour)
				if b.ValidWebToken(ctx, token) {
					t.Fatal("changed policy did not apply to an existing token")
				}
			}
		})
	}
}

func TestLegacyWebTokenMigrationDoesNotRenewIssueTime(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "bus.db")
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	issued := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := db.Exec(`CREATE TABLE web_sessions (token TEXT PRIMARY KEY, created_at TEXT NOT NULL)`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO web_sessions VALUES (?, ?)`, "legacy", issued.Format(time.RFC3339Nano)); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()
	b, err := OpenAt(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	now := issued.Add(29 * 24 * time.Hour)
	b.now = func() time.Time { return now }
	if !b.CheckWebToken(ctx, "legacy") {
		t.Fatal("migration rejected a token inside both windows")
	}
	now = issued.Add(30 * 24 * time.Hour)
	if b.CheckWebToken(ctx, "legacy") {
		t.Fatal("migration renewed the old issue time")
	}
}

func TestWebRevocationCannotBeUndoneByConcurrentUse(t *testing.T) {
	b, _ := open(t)
	ctx := context.Background()
	code, _ := b.NewWebCode(ctx)
	token, err := b.ExchangeWebCode(ctx, code)
	if err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	start := make(chan struct{})
	for range 5 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			for range 10 {
				b.ValidWebToken(ctx, token)
			}
		}()
	}
	close(start)
	if err := b.RevokeAllWebTokens(ctx); err != nil {
		t.Fatal(err)
	}
	workers.Wait()
	if b.CheckWebToken(ctx, token) {
		t.Fatal("a last-use update resurrected a revoked token")
	}
}
