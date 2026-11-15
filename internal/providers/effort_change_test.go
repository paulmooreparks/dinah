package providers

import "testing"

// Each provider's EffortChange is only what its documentation establishes
// (docs/review/effort-change-protocol-2026-11-16.md): Codex takes effort as a
// turn/start override, Ollama's think belongs to each request, and Claude and
// Devin through ACP leave the timing during a running prompt unspecified.
func TestEffortChangeIsOnlyWhatIsDocumented(t *testing.T) {
	want := map[string]string{"codex": "next-turn", "ollama": "next-request", "claude": "", "devin": ""}
	for id, row := range Registry {
		switch row.EffortChange {
		case "", "next-turn", "next-request":
		default:
			t.Errorf("%s: EffortChange %q is not one of the documented kinds", id, row.EffortChange)
		}
		if w, ok := want[id]; ok && row.EffortChange != w {
			t.Errorf("%s: EffortChange = %q, want %q", id, row.EffortChange, w)
		}
	}
}
