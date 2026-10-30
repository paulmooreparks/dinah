package main

import (
	"testing"

	"studio/internal/acp"
)

// TestSignInMethod checks that the app picks a method it can carry out,
// one the agent does itself, and none when the agent offers only methods
// that need the client to collect a secret or run a terminal.
func TestSignInMethod(t *testing.T) {
	devin := []acp.AuthMethod{{ID: "devin-browser", Name: "Log in with browser", Description: "Sign in via your browser"}}
	if m := signInMethod(devin); m == nil || m.ID != "devin-browser" {
		t.Fatalf("Devin's browser method was not chosen: %+v", m)
	}
	mixed := []acp.AuthMethod{{ID: "key", Type: "env_var"}, {ID: "sso", Type: "agent"}}
	if m := signInMethod(mixed); m == nil || m.ID != "sso" {
		t.Fatalf("the agent method was not chosen: %+v", m)
	}
	if m := signInMethod([]acp.AuthMethod{{ID: "key", Type: "env_var"}, {ID: "tty", Type: "terminal"}}); m != nil {
		t.Fatalf("a method the app cannot carry out was chosen: %+v", m)
	}
	if m := signInMethod(nil); m != nil {
		t.Fatalf("a method was chosen from none: %+v", m)
	}
}
