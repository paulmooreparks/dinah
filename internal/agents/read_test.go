package agents

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"studio/internal/layout"
	"testing"
	"unicode/utf8"
)

func TestSliceLinesRejectsInvalidRanges(t *testing.T) {
	for _, tc := range []struct{ line, limit int }{{-1, 1}, {1, -5}, {0, 1}, {1, 0}, {4, 1}} {
		if _, err := SliceLines("a\nb", &tc.line, &tc.limit); err == nil {
			t.Fatalf("accepted %+v", tc)
		}
	}
	line, limit := 2, int(^uint(0)>>1)
	if got, err := SliceLines("a\nb", &line, &limit); err != nil || got != "b" {
		t.Fatalf("large positive limit: %q %v", got, err)
	}
}
func FuzzSliceLines(f *testing.F) {
	for _, v := range []struct {
		s    string
		l, n int
	}{{"a\nb", 1, 1}, {"", -1, -5}, {"a", 99, 1}, {"a", 1, int(^uint(0) >> 1)}} {
		f.Add(v.s, v.l, v.n)
	}
	f.Fuzz(func(t *testing.T, s string, line, limit int) {
		got, err := SliceLines(s, &line, &limit)
		if (line < 1 || limit < 1) && err == nil {
			t.Fatal("invalid range accepted")
		}
		if err == nil && len(got) > len(s) {
			t.Fatal("slice grew")
		}
	})
}
func TestProviderReadPolicyAndBounds(t *testing.T) {
	home := t.TempDir()
	state := filepath.Join(home, "state")
	ws := filepath.Join(home, "workspace")
	for _, p := range []string{state, ws} {
		if err := os.MkdirAll(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	l := layout.Layout{State: state}
	write := func(p string, b []byte) {
		t.Helper()
		if err := os.WriteFile(p, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	outside := filepath.Join(home, "sibling.txt")
	write(outside, []byte("sibling"))
	protected := filepath.Join(state, "tokens.txt")
	write(protected, []byte("synthetic-token"))
	ctx := context.Background()
	if got, err := ReadTextFile(ctx, l, outside, nil, nil, nil); err != nil || got != "sibling" {
		t.Fatalf("outside workspace must remain allowed: %q %v", got, err)
	}
	if _, err := ReadTextFile(ctx, l, protected, nil, nil, nil); err == nil {
		t.Fatal("protected state read")
	}
	managed := l.ManagedRoot("12345678-1234-1234-1234-123456789abc")
	if err := os.MkdirAll(managed, 0700); err != nil {
		t.Fatal(err)
	}
	managedFile := filepath.Join(managed, "source.txt")
	write(managedFile, []byte("managed"))
	if got, err := ReadTextFile(ctx, l, managedFile, []string{managed}, nil, nil); err != nil || got != "managed" {
		t.Fatalf("managed execution exception: %q %v", got, err)
	}
	alias := filepath.Join(ws, "alias.txt")
	t.Run("protected symlink", func(t *testing.T) {
		if err := os.Symlink(protected, alias); err != nil {
			t.Skipf("symlink creation unavailable: %v", err)
		}
		if _, err := ReadTextFile(ctx, l, alias, nil, nil, nil); err == nil {
			t.Fatal("protected symlink read")
		}
	})
	t.Run("permitted symlink", func(t *testing.T) {
		link := filepath.Join(ws, "sibling-link.txt")
		if err := os.Symlink(outside, link); err != nil {
			t.Skipf("symlink creation unavailable: %v", err)
		}
		if got, err := ReadTextFile(ctx, l, link, nil, nil, nil); err != nil || got != "sibling" {
			t.Fatalf("permitted alias: %q %v", got, err)
		}
	})
	bad := filepath.Join(ws, "invalid.txt")
	write(bad, []byte{'x', 0xff, 'y'})
	got, err := ReadTextFile(ctx, l, bad, nil, nil, nil)
	if err != nil || !utf8.ValidString(got) || !strings.Contains(got, "\uFFFD") || !strings.HasPrefix(got, UTF8ReplacementNote) {
		t.Fatalf("invalid UTF8 not marked: %q %v", got, err)
	}
	big := filepath.Join(ws, "large.txt")
	write(big, []byte("short\n"+strings.Repeat("x", MaxProviderText)))
	if _, err := ReadTextFile(ctx, l, big, nil, nil, nil); err == nil || !strings.Contains(err.Error(), "line and limit") {
		t.Fatalf("large response: %v", err)
	}
	line, limit := 1, 1
	if got, err := ReadTextFile(ctx, l, big, nil, &line, &limit); err != nil || got != "short\n" {
		t.Fatalf("bounded range of large file: %q %v", got, err)
	}
	line = 999
	if _, err := ReadTextFile(ctx, l, outside, nil, &line, &limit); err == nil {
		t.Fatal("past-end line accepted")
	}
}
