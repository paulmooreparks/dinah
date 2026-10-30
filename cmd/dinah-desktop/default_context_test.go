package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// The options the window saved for an Ollama chat while 64K was every chat's
// default are released once when the settings are read, so reopening the chat does
// not apply the old default as the person's choice. Other lengths, other
// providers' options and a choice made after the release are kept.
func TestTheOldDefaultContextSavedForAChatIsReleasedOnce(t *testing.T) {
	providerTestApp(t)
	opt := func(model, context string) map[string]json.RawMessage {
		o := map[string]json.RawMessage{"model": json.RawMessage(`"` + model + `"`)}
		if context != "" {
			o["context"] = json.RawMessage(`"` + context + `"`)
		}
		return o
	}
	saved := Settings{
		SessionOptions: map[string]map[string]json.RawMessage{
			"ollama:old":    opt("kimi-k3:cloud", "65536"),
			"ollama:large":  opt("kimi-k3:cloud", "131072"),
			"ollama:none":   opt("kimi-k3:cloud", "0"),
			"codex:other":   opt("gpt", "65536"),
			"ollama:nosize": opt("kimi-k3:cloud", ""),
		},
		SessionProviders: map[string]string{"ollama:old": "ollama", "ollama:large": "ollama", "ollama:none": "ollama", "codex:other": "codex", "ollama:nosize": "ollama"},
	}
	data, _ := json.Marshal(saved)
	path := filepath.Join(configDir(), "settings.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := loadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if _, kept := s.SessionOptions["ollama:old"]["context"]; kept || string(s.SessionOptions["ollama:old"]["model"]) != `"kimi-k3:cloud"` {
		t.Fatalf("the old default was not released, or took the model with it: %v", s.SessionOptions["ollama:old"])
	}
	for id, want := range map[string]string{"ollama:large": `"131072"`, "ollama:none": `"0"`, "codex:other": `"65536"`} {
		if got := string(s.SessionOptions[id]["context"]); got != want {
			t.Errorf("%s context is %s, not %s", id, got, want)
		}
	}
	if !s.DefaultContextReleased {
		t.Fatal("the release was not marked, so it would run every time")
	}
	// Once released, a 64K the person chooses is theirs and is kept.
	s.SessionOptions["ollama:old"]["context"] = json.RawMessage(`"65536"`)
	data, _ = json.Marshal(s)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	again, err := loadSettings()
	if err != nil || string(again.SessionOptions["ollama:old"]["context"]) != `"65536"` {
		t.Fatalf("a choice made after the release was dropped: %v %v", again.SessionOptions["ollama:old"], err)
	}
}
