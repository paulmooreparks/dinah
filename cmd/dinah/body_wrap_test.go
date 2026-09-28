package main

import (
	"strings"
	"testing"
)

// A card body wraps each source line to the window, never joining two of
// them, and its list items hang under their first word.
func TestWrapBodyTextWrapsEachLine(t *testing.T) {
	body := strings.Join([]string{
		"**Status bar.** The held card and its lease countdown, or a count of what needs you.",
		"",
		"1. first numbered item that runs well past the edge of the window",
		"2. second",
		"  - nested bullet that also runs well past the edge of the window",
		"> quoted words that run well past the edge of the window here",
	}, "\n")
	got := wrapBodyText(body, 30)
	want := strings.Join([]string{
		"**Status bar.** The held card",
		"and its lease countdown, or a",
		"count of what needs you.",
		"",
		"1. first numbered item that",
		"   runs well past the edge of",
		"   the window",
		"2. second",
		"  - nested bullet that also",
		"    runs well past the edge of",
		"    the window",
		"> quoted words that run well",
		"  past the edge of the window",
		"  here",
	}, "\n")
	if got != want {
		t.Fatalf("wrapped body:\n%s\nwant:\n%s", got, want)
	}
	for _, line := range strings.Split(got, "\n") {
		if len([]rune(line)) > 30 {
			t.Errorf("line %q is wider than the window", line)
		}
	}
}

// Code and tables keep their alignment, and an unknown window leaves the body
// byte for byte as its author wrote it.
func TestWrapBodyTextKeepsCodeAndPipedText(t *testing.T) {
	body := strings.Join([]string{
		"```",
		"a very long line of code that must never be broken anywhere at all",
		"```",
		"| a table row that is much wider than the window it is drawn in |",
		"    indented code that is also much wider than the window it is in",
	}, "\n")
	if got := wrapBodyText(body, 30); got != body {
		t.Errorf("code and tables were re-flowed:\n%s", got)
	}
	long := "words " + strings.Repeat("and more words ", 20)
	if got := wrapBodyText(long, 0); got != long {
		t.Errorf("an unknown window changed the body")
	}
}

// A marker taking half the window or more is not hung under: the line wraps
// from the window's edge instead.
func TestWrapBodyTextDropsAWideHang(t *testing.T) {
	got := wrapBodyText("            - words that run past the edge", 20)
	want := "- words that run\npast the edge"
	if got != want {
		t.Errorf("wide marker wrapped as:\n%s\nwant:\n%s", got, want)
	}
}
