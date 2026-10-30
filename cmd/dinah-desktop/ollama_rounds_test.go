package main

import (
	"testing"
)

// Every install starts with no limit on the rounds of an Ollama turn. The
// person's number is saved in one write and reported in the state, and 0 is
// none; a negative number is refused and changes nothing.
func TestChoosingHowManyRoundsAnOllamaTurnMayTake(t *testing.T) {
	a, _ := providerTestApp(t)
	if got := a.State().OllamaRoundLimit.Limit; got != 0 {
		t.Fatalf("a new install has a limit of %d", got)
	}
	if a.State().OllamaRoundLimit.Chosen {
		t.Fatal("the default is shown as the person's choice")
	}
	if err := a.SetOllamaRoundLimit(25); err != nil {
		t.Fatal(err)
	}
	if got := a.State().OllamaRoundLimit; !got.Chosen || got.Limit != 25 {
		t.Fatalf("the state says %+v", got)
	}
	if got := a.State().OllamaRoundLimit.Limit; got != 25 {
		t.Fatalf("the state says %d", got)
	}
	if reloaded, err := loadSettings(); err != nil || reloaded.OllamaRounds != 25 {
		t.Fatalf("what was saved: %d %v", reloaded.OllamaRounds, err)
	}
	if err := a.SetOllamaRoundLimit(-1); err == nil {
		t.Fatal("a negative limit was accepted")
	}
	if a.State().OllamaRoundLimit.Limit != 25 {
		t.Fatal("a refused choice changed the setting")
	}
	if err := a.SetOllamaRoundLimit(0); err != nil || a.State().OllamaRoundLimit.Limit != 0 {
		t.Fatalf("no limit: %v", err)
	}
	// A hand-edited settings file with a negative number is no limit.
	a.mu.Lock()
	a.settings.OllamaRounds = -4
	a.mu.Unlock()
	if ollamaRounds(a.settings) != 0 {
		t.Fatal("a negative number in the settings is a limit")
	}
}
