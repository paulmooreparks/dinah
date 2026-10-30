package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDevinSteeringRegistrationPreservesHooksAndIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.v1.json")
	initial := `{"PostToolUse":[{"matcher":"exec","hooks":[{"type":"command","command":"my-check"}]}],"SessionStart":[{"hooks":[{"type":"command","command":"my-start"}]}]}`
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := registerDevinSteeringHooks(path); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(path)
	if !strings.Contains(string(first), "my-check") || !strings.Contains(string(first), "my-start") || strings.Count(string(first), devinSteeringCommand) != 3 {
		t.Fatalf("registered hooks: %s", first)
	}
	if err := registerDevinSteeringHooks(path); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(path)
	if string(first) != string(second) {
		t.Fatal("registration duplicated or changed existing hooks")
	}
	broken := []byte(`{"PostToolUse":42}`)
	os.WriteFile(path, broken, 0o600)
	if err := registerDevinSteeringHooks(path); err == nil {
		t.Fatal("invalid hooks were replaced")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(broken) {
		t.Fatal("failed registration changed the person's hooks")
	}
}
