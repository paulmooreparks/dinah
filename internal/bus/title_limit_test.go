package bus

import (
	"context"
	"strings"
	"testing"
)

const injected = "[Dinah Applet Context]\nDinah supplies the internal dinah-applets MCP server. Pass dinah-applet-v1:ABCDEFGHIJKLMNOPQRSTUVWXYZ.ABCDEFGHIJKLMNOPQRSTUVWXYZ for every call.\n[Dinah Chat Communication]\nChat communication is asynchronous."

// A title is the first line of what was written, without anything Dinah
// injected into the prompt, and at most TitleLimit characters.
func TestCleanTitle(t *testing.T) {
	long := strings.Repeat("word ", 60)
	cases := []struct{ name, in, want string }{
		{"short", "  Release  notes ", "Release notes"},
		{"a whole prompt with injected blocks", "Does Codex require this approval?\n\nMore of the question.\n" + injected, "Does Codex require this approval?"},
		{"injected text on the same line after whitespace collapsed", "Does Codex require this approval? " + strings.ReplaceAll(injected, "\n", " "), "Does Codex require this approval?"},
		{"only injected text", injected, ""},
		{"a bare context", "dinah-applet-v1:ABCDEFGHIJKLMNOPQRSTUVWXYZ.ABCDEFGHIJKLMNOPQRSTUVWXYZ", ""},
		{"leading blank lines", "\n\n  First real line\nSecond", "First real line"},
		{"long", long, strings.TrimRight(strings.TrimSpace(long)[:TitleLimit-1], " ") + "…"},
	}
	for _, c := range cases {
		got := CleanTitle(c.in)
		if got != c.want {
			t.Errorf("%s: %q, not %q", c.name, got, c.want)
		}
		if len([]rune(got)) > TitleLimit || strings.Contains(got, "dinah-applet-v1:") || strings.Contains(got, "\n") {
			t.Errorf("%s: %q is not a short clean line", c.name, got)
		}
	}
}

// No writer can store a longer title, and the pass shortens one stored before
// the limit.
func TestTheBusStoresOnlyShortTitlesAndShortensOldOnes(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	studio := workspace(t, root, "studio")
	prompt := "Work with the team on the design " + strings.ReplaceAll(injected, "\n", " ")
	s, err := b.RegisterSession(ctx, "window-a", studio, prompt)
	if err != nil || s.Title != "Work with the team on the design" {
		t.Fatalf("a registered title: %q %v", s.Title, err)
	}
	if err := b.SetTitle(ctx, "window-a", injected+"\nSecond"); err != nil {
		t.Fatal(err)
	}
	if got, _ := b.Session(ctx, "window-a"); got.Title != "" {
		t.Fatalf("a title that was only injected text was stored as %q", got.Title)
	}
	known, err := b.KnowSession(ctx, "window-b", studio, "Closed chat\n"+injected)
	if err != nil || known.Title != "Closed chat" {
		t.Fatalf("a known title: %q %v", known.Title, err)
	}
	// An oversized title as an older build stored it.
	if _, err := b.db.Exec(`UPDATE sessions SET title = ? WHERE id = ?`, "Does Codex require this approval? "+strings.Repeat("x", 190000)+" "+strings.ReplaceAll(injected, "\n", " "), "window-b"); err != nil {
		t.Fatal(err)
	}
	n, err := b.ShortenTitles(ctx)
	if err != nil || n != 1 {
		t.Fatalf("the pass changed %d titles: %v", n, err)
	}
	got, _ := b.Session(ctx, "window-b")
	if len([]rune(got.Title)) > TitleLimit || !strings.HasPrefix(got.Title, "Does Codex require this approval?") {
		t.Fatalf("the stored title is now %q", got.Title)
	}
	if n, _ := b.ShortenTitles(ctx); n != 0 {
		t.Fatalf("a second pass changed %d titles", n)
	}
}
