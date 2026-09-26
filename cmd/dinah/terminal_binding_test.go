package main

import (
	"encoding/json"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// TestAKeyBindingIsASetting is the configuration half of
// dinah-623/criteria/41: config set accepts a tui.key binding and its
// tui.label, and the bare config listing shows both; a binding on a key
// dinah tui reads itself, on a key that is not one letter or digit, and of a
// template that is empty, begins with !, names $1 or names $title is refused
// as dinah.invalid-key-binding with the matching defect token; and giving no
// value removes a binding.
func TestAKeyBindingIsASetting(t *testing.T) {
	root := newBench(t)
	if got := runCLI(t, root, "config", "set", "tui.key.o", "oq $card"); got.code != 0 {
		t.Fatalf("the binding was refused: %s", got.errw)
	}
	if got := runCLI(t, root, "config", "set", "tui.label.o", "question"); got.code != 0 {
		t.Fatalf("the label was refused: %s", got.errw)
	}
	listed := runCLI(t, root, "--json", "config")
	var rows []struct{ Key, Value, Source string }
	if err := json.Unmarshal([]byte(listed.out), &rows); err != nil {
		t.Fatalf("the listing does not parse: %v\n%s", err, listed.out)
	}
	found := map[string]string{}
	for _, row := range rows {
		found[row.Key] = row.Value + "|" + row.Source
	}
	if found["tui.key.o"] != "oq $card|"+bench.SourceConfig || found["tui.label.o"] != "question|"+bench.SourceConfig {
		t.Errorf("the listing shows %q and %q", found["tui.key.o"], found["tui.label.o"])
	}
	if got := runCLI(t, root, "config", "get", "tui.key.o"); strings.TrimSpace(got.out) != "oq $card" {
		t.Errorf("config get answers %q", got.out)
	}
	refused := []struct{ key, template, defect string }{
		{"tui.key.a", "claim $card", bench.KeyBindingReservedKey},
		{"tui.key.W", "whoami", bench.KeyBindingReservedKey},
		{"tui.key.ab", "status", bench.KeyBindingInvalidKey},
		{"tui.key..", "status", bench.KeyBindingInvalidKey},
		{"tui.key.o", "", bench.KeyBindingEmptyTemplate},
		{"tui.key.o", "!ls", bench.KeyBindingShellTemplate},
		{"tui.key.o", "comment $1", bench.KeyBindingNumberedPlaceholder},
		{"tui.key.o", "comment $title", bench.KeyBindingUnknownPlaceholder},
		{"tui.label.ab", "a label", bench.KeyBindingInvalidKey},
	}
	for _, c := range refused {
		got := runCLI(t, root, "--json", "config", "set", c.key, c.template)
		if got.code == 0 {
			t.Errorf("config set %s %q was accepted", c.key, c.template)
			continue
		}
		var report refusalReport
		if err := json.Unmarshal([]byte(got.out), &report); err != nil {
			t.Errorf("config set %s %q answered no refusal document: %s", c.key, c.template, got.out)
			continue
		}
		if report.Refusal != contract.InvalidKeyBinding || report.Detail != c.key || report.Context["defect"] != c.defect {
			t.Errorf("config set %s %q was refused %+v, wanted %s with defect %s", c.key, c.template, report, contract.InvalidKeyBinding, c.defect)
		}
	}
	if got := runCLI(t, root, "config", "set", "tui.key.o"); got.code != 0 {
		t.Errorf("removing the binding was refused: %s", got.errw)
	}
	if got := runCLI(t, root, "config", "get", "tui.key.o"); strings.TrimSpace(got.out) != "" {
		t.Errorf("the removed binding still reads %q", got.out)
	}
	t.Logf("%d refusals checked", len(refused))
}
