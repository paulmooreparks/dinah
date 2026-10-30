package main

import (
	"strings"
	"testing"
)

func TestATitleMadeFromTheAppletContextIsReplacedByTheFirstMessage(t *testing.T) {
	a, _ := providerTestApp(t)
	s := newSessionState(&sessionSeed{ID: "titled-from-context", Provider: "devin", Cwd: t.TempDir()}, sessionInitial{Open: true})
	a.runtime.PutConversationLocked(s.ID(), s)
	a.transcriptCapture().Initialize(s, false)
	a.transcriptCapture().RecordMessage(s, "you", "Find documentation about queue monitoring in the wiki.\n\n"+appletContextHeading+"pass dinah-applet-v1:ABC", false, false)

	got := a.sessionTitle(s.ID(), `functions.applet_discover:0{"context": "dinah-applet-v1:VPU5X65GEDK56VVOJXHCNVCM...`)
	if got != "Find documentation about queue monitoring in the wiki." {
		t.Fatalf("got %q", got)
	}
	if got := a.sessionTitle(s.ID(), "Retrieve and Add Page Comments"); got != "Retrieve and Add Page Comments" {
		t.Fatalf("an ordinary title changed: %q", got)
	}
	if got := a.sessionTitle("no-such-session", "dinah-applet-v1:X"); got != "" {
		t.Fatalf("a context title with no saved message should be dropped, got %q", got)
	}
}

func TestTitleOfShortensAtAWord(t *testing.T) {
	long := strings.Repeat("release notes draft ", 10)
	got := titleOf("\n  " + long)
	if !strings.HasSuffix(got, "…") || len([]rune(got)) > titleLimit+1 || strings.Contains(got, "  ") {
		t.Fatalf("got %q", got)
	}
}
