package main

import (
	"strings"
	"testing"
)

// The start pass shortens the names the person gave that an older build stored
// whole (the bus's own titles are covered in the bus tests).
func TestTheStartPassShortensStoredTitles(t *testing.T) {
	a, _ := providerTestApp(t)
	prompt := "Does Codex require this approval? [Dinah Applet Context] dinah-applet-v1:ABCDEFGHIJKLMNOPQRSTUVWXYZ.ABCDEFGHIJKLMNOPQRSTUVWXYZ " + strings.Repeat("x", 5000)
	if err := a.updateSettings(func(s *Settings) error {
		s.SessionTitles = map[string]string{"codex:long": prompt, "codex:fine": "A good name"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	a.shortenTitles()
	if got := savedTitle(a, "codex:long"); got != "Does Codex require this approval?" {
		t.Fatalf("the saved name is %q", got)
	}
	if got := savedTitle(a, "codex:fine"); got != "A good name" {
		t.Fatalf("a good name was changed to %q", got)
	}
}

// An agent's title that is a whole prompt reaches the chat as its first line.
func TestAnAgentsTitleThatIsAWholePromptIsShortened(t *testing.T) {
	a, _ := providerTestApp(t)
	s := continuationSource(t, a)
	title := "Is this ready? [Dinah Applet Context] dinah-applet-v1:ABCDEFGHIJKLMNOPQRSTUVWXYZ.ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	s.UpdateDisplay(&title, nil)
	if got := s.Title(); got != "Is this ready?" {
		t.Fatalf("the chat's title is %q", got)
	}
}
