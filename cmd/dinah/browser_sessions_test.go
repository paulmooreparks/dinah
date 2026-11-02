package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/bus"
)

func TestBrowserSessionsCommandSettingsAndRevocation(t *testing.T) {
	b, err := bus.OpenAt(filepath.Join(t.TempDir(), "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	ctx := context.Background()
	var out bytes.Buffer
	if err := browserSessions(ctx, b, []string{"settings"}, &out); err != nil || !strings.Contains(out.String(), "30 days") || !strings.Contains(out.String(), "90 days") {
		t.Fatalf("default settings: %s %v", &out, err)
	}
	if err := browserSessions(ctx, b, []string{"settings", "--idle-days", "7", "--absolute-days", "60"}, &out); err != nil {
		t.Fatal(err)
	}
	if err := browserSessions(ctx, b, []string{"settings", "--idle-days", "0"}, &out); err == nil {
		t.Fatal("invalid lifetime accepted")
	}
	if got, err := b.WebSessionPolicy(ctx); err != nil || got.IdleDays != 7 || got.AbsoluteDays != 60 {
		t.Fatal("invalid input changed persisted settings")
	}
	code, _ := b.NewWebCode(ctx)
	token, err := b.ExchangeWebCode(ctx, code)
	if err != nil {
		t.Fatal(err)
	}
	if err := browserSessions(ctx, b, []string{"sign-out-everywhere"}, &out); err != nil || b.ValidWebToken(ctx, token) {
		t.Fatal("command did not revoke the browser token")
	}
}
