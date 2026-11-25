package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTermscanReportsOnlyHashAndListLine(t *testing.T) {
	dir := t.TempDir()
	list := filepath.Join(dir, "terms.txt")
	if err := os.WriteFile(list, []byte("# comment\n\nsecretword\nfoo[0-9]+\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	terms, err := readTerms(list)
	if err != nil {
		t.Fatal(err)
	}
	in := "0123456789abcdef\tA clean message\x00" +
		"fedcba9876543210\tMentions SecretWord here\x00" +
		"patch\t+ a line with foo42 in it\x00"
	var out strings.Builder
	hits, records := scan(in, terms, &out)
	if records != 3 || hits != 2 {
		t.Fatalf("records %d hits %d, want 3 and 2", records, hits)
	}
	got := out.String()
	if !strings.Contains(got, "fedcba98 list line 3") || !strings.Contains(got, "patch list line 4") {
		t.Fatalf("unexpected report:\n%s", got)
	}
	if strings.Contains(strings.ToLower(got), "secretword") || strings.Contains(got, "foo42") {
		t.Fatalf("the report must not repeat the matched text:\n%s", got)
	}
}

func TestTermscanRefusesABadPattern(t *testing.T) {
	list := filepath.Join(t.TempDir(), "terms.txt")
	if err := os.WriteFile(list, []byte("ok\n(unclosed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readTerms(list); err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("want an error naming line 2, got %v", err)
	}
}
