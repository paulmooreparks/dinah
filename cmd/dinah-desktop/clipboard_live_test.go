//go:build live

package main

import "testing"

// TestLiveClipboardRead reads this computer's clipboard as the Editor's Paste
// does, and says only how much HTML and text it found, never what. It
// changes nothing. Run with: go test -tags live -run TestLiveClipboardRead -v ./cmd/dinah-desktop
func TestLiveClipboardRead(t *testing.T) {
	a := NewApp()
	c, err := a.ClipboardRead()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("HTML: %d bytes, starts with a tag: %v; text: %d characters", len(c.HTML), len(c.HTML) > 0 && c.HTML[0] == '<', len([]rune(c.Text)))
}
