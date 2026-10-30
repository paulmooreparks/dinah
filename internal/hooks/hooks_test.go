package hooks

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/bus"
)

func TestSessionStartJoinsTheBus(t *testing.T) {
	// The test may itself run under an agent in Dinah's window.
	t.Setenv(bus.HostedVar, "")
	b, err := bus.OpenAt(filepath.Join(t.TempDir(), "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	var out bytes.Buffer
	in := `{"session_id":"s1","cwd":"` + filepath.ToSlash(t.TempDir()) + `","hook_event_name":"SessionStart"}`
	if err := Run(context.Background(), b, strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Session(context.Background(), "s1"); err != nil {
		t.Fatalf("the session did not join: %v", err)
	}
	if !strings.Contains(out.String(), "additionalContext") {
		t.Fatalf("the hook told the agent nothing: %s", out.String())
	}
}

// An agent in Dinah's window runs the person's hooks too, but the window has
// already put the chat on the bus, so the hook adds nobody.
func TestAHookInDinahsWindowStandsDown(t *testing.T) {
	t.Setenv(bus.HostedVar, "1")
	b, err := bus.OpenAt(filepath.Join(t.TempDir(), "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	var out bytes.Buffer
	in := `{"session_id":"s1","cwd":"` + filepath.ToSlash(t.TempDir()) + `","hook_event_name":"SessionStart"}`
	if err := Run(context.Background(), b, strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Session(context.Background(), "s1"); err == nil {
		t.Fatal("a chat in Dinah's window joined the bus a second time through the hook")
	}
	if out.Len() != 0 {
		t.Fatalf("the hook said something: %s", out.String())
	}
}
