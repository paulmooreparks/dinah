package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// TestTheStartupRefusalsMCPRaisesLeadWithTheirName asserts AC-25 across all
// three refusals dinah mcp can raise before it serves, in one test, because
// the criterion is about the three agreeing rather than about any one of them.
//
// A client reads the refusal name off this stream by splitting the line on
// whitespace and taking the first field, so a name carrying a trailing colon
// matches nothing the client is looking for. Two of the three lines are built
// by hand and are space-separated; the third was rendered through
// Refusal.Error, which formats as "%s: %s". A test holding one line at a time
// cannot see a disagreement between three, so this one holds all three.
func TestTheStartupRefusalsMCPRaisesLeadWithTheirName(t *testing.T) {
	container := newBench(t)
	workbench := soleBenchDir(t, container)

	elsewhere := t.TempDir()
	empty := filepath.Join(elsewhere, "empty")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	missing := filepath.Join(elsewhere, "missing")

	cases := []struct {
		name string
		argv []string
		want string
	}{
		{
			name: "a root no directory sits at",
			argv: []string{"mcp", "--root", missing},
			want: contract.UnknownRoot,
		},
		{
			name: "a named workbench holding no anchor",
			argv: []string{"--workbench", empty, "mcp", "--root", empty},
			want: contract.NoWorkbench,
		},
		{
			name: "a named workbench outside the root",
			argv: []string{"--workbench", workbench, "mcp", "--root", empty},
			want: contract.OutsideRoot,
		},
	}
	for _, held := range cases {
		t.Run(held.name, func(t *testing.T) {
			got := runCLI(t, empty, held.argv...)
			if got.code != contract.ExitCode(contract.OutcomeRefused) {
				t.Fatalf("wanted the refused exit code %d, got %d: %s", contract.ExitCode(contract.OutcomeRefused), got.code, got.errw)
			}
			fields := strings.Fields(got.errw)
			if len(fields) == 0 {
				t.Fatalf("the error stream carried nothing")
			}
			if fields[0] != held.want {
				t.Errorf("the first whitespace-delimited token on stderr: wanted %q, got %q from %q", held.want, fields[0], strings.TrimSpace(got.errw))
			}
			if len(fields) < 2 {
				t.Errorf("the line names no detail after the refusal name: %q", strings.TrimSpace(got.errw))
			}
		})
	}
}
