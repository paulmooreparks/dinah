package main

import (
	"strings"
	"testing"

	"dinah/internal/msg"
	"dinah/internal/verb"
)

// TestTheTUICommandIsDeclaredOnEverySurface is dinah-603/criteria/1 beyond
// the help block itself, and the help half of dinah-603/criteria/47: dinah
// help tui prints the summary, the note that the command runs the separate
// program dinah-tui, both parameters and the three check rows, and dinah view
// gains no parameter. It builds with no tags, so it holds the main dinah
// binary, which answers help for a command whose head it does not carry.
func TestTheTUICommandIsDeclaredOnEverySurface(t *testing.T) {
	r := msg.For(msg.Base)
	got := runCLI(t, t.TempDir(), "help", "tui")
	if got.code != 0 {
		t.Fatalf("help tui: %d %s", got.code, got.errw)
	}
	for _, key := range []string{"cmd.tui.summary", "cmd.tui.note", "param.tui.view.summary", "param.tui.plain.summary", "check.tui.1", "check.tui.2"} {
		if !strings.Contains(got.out, r.T(key)) {
			t.Errorf("dinah help tui does not print %s: %q", key, r.T(key))
		}
	}
	if !strings.Contains(got.out, "dinah.tui-unavailable") || !strings.Contains(got.out, "standard input and output are a terminal") {
		t.Errorf("dinah help tui does not print the third check row:\n%s", got.out)
	}
	var names []string
	for _, param := range verb.Params("view") {
		names = append(names, param.Name)
	}
	if strings.Join(names, " ") != "view card explain all plain watch" {
		t.Errorf("dinah view declares %v, which is not the six parameters it had", names)
	}
}
