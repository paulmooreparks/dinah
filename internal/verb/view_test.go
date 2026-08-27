package verb

import (
	"encoding/json"
	"testing"

	"dinah/internal/bench"
)

// TestViewCarriesSeverityAndPriorityVerbatim asserts that Library.view sets
// CardView.Severity and CardView.Priority directly from the card, with
// omitempty dropping the key entirely when the card carries no value for
// that axis (dinah-194 AC-1, AC-2). It covers a card carrying both axes, one
// axis, and neither, since those are the combinations the acceptance
// criteria name explicitly.
func TestViewCarriesSeverityAndPriorityVerbatim(t *testing.T) {
	h := newHarness(t)
	cases := []struct {
		name     string
		severity string
		priority string
		wantKeys []string
		omitKeys []string
	}{
		{
			name:     "both axes",
			severity: "major",
			priority: "now",
			wantKeys: []string{"severity", "priority"},
		},
		{
			name:     "severity only",
			severity: "minor",
			wantKeys: []string{"severity"},
			omitKeys: []string{"priority"},
		},
		{
			name:     "priority only",
			priority: "soon",
			wantKeys: []string{"priority"},
			omitKeys: []string{"severity"},
		},
		{
			name:     "neither axis",
			omitKeys: []string{"severity", "priority"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			card := &bench.Card{
				ID:       "a00000000abc",
				Number:   1,
				Title:    "A card",
				Column:   intake,
				State:    "ready",
				Severity: tc.severity,
				Priority: tc.priority,
				Revision: "deadbeefcafe",
			}
			view := h.library.view(card)
			if view.Severity != tc.severity {
				t.Errorf("Severity = %q, want %q", view.Severity, tc.severity)
			}
			if view.Priority != tc.priority {
				t.Errorf("Priority = %q, want %q", view.Priority, tc.priority)
			}
			data, err := json.Marshal(view)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var raw map[string]any
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			for _, key := range tc.wantKeys {
				if _, ok := raw[key]; !ok {
					t.Errorf("%s: expected key %q present in %s", tc.name, key, data)
				}
			}
			for _, key := range tc.omitKeys {
				if _, ok := raw[key]; ok {
					t.Errorf("%s: expected key %q absent, got %s", tc.name, key, data)
				}
			}
		})
	}
}
