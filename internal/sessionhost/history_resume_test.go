package sessionhost

import "testing"

// A copy that cannot be resumed says why, which is what the reopen's log line
// tells the person who reads it.
func TestResumeSaysWhyACopyCannotBeUsed(t *testing.T) {
	dir, ws := t.TempDir(), t.TempDir()
	write := func(change func(*Transcript)) {
		tr := &Transcript{SessionID: "claude:x", Provider: "claude", Workspace: ws, Complete: true, Messages: []ConversationMessage{{Role: "you", Text: "Hi"}}}
		if change != nil {
			change(tr)
		}
		if err := WriteTranscript(dir, tr); err != nil {
			t.Fatal(err)
		}
	}
	cases := []struct {
		name   string
		change func(*Transcript)
		want   string
	}{
		{"whole", nil, ""},
		{"incomplete", func(tr *Transcript) { tr.Complete = false }, "the saved copy is not complete"},
		{"capture error", func(tr *Transcript) { tr.CaptureError = "x" }, "the saved copy has a capture error"},
		{"deleted", func(tr *Transcript) { tr.Deleted = true }, "the saved copy is marked deleted"},
		{"provider", func(tr *Transcript) { tr.Provider = "codex" }, "the saved copy is another provider's"},
		{"workspace", func(tr *Transcript) { tr.Workspace = t.TempDir() }, "the saved copy is another workspace's"},
	}
	for _, tc := range cases {
		write(tc.change)
		var h History
		if got := h.ResumeLocked(dir, "claude:x", "claude", ws); got != tc.want {
			t.Errorf("%s: %q, not %q", tc.name, got, tc.want)
		}
	}
	var h History
	if got := h.ResumeLocked(t.TempDir(), "claude:none", "claude", ws); got != "there is no saved copy" {
		t.Errorf("no copy: %q", got)
	}
}
