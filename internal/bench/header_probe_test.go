package bench

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestACardHeaderReadsPastItsFirstProbe holds readCardHeader's two reads to
// the one answer the single 64 KiB read gave. A header that closes inside the
// first probe, one that closes past it, one whose probe ends in the middle of
// a line that only begins with the fence, and one that never closes inside
// the limit each answer what they answered before the probe.
//
// Arming: deciding on a last line the probe cut short reads "----x" cut to
// "---" as the closing fence, and the third case answers a header missing the
// key that follows it.
func TestACardHeaderReadsPastItsFirstProbe(t *testing.T) {
	dir := t.TempDir()
	pad := func(n int) string { return strings.Repeat("x", n) }
	// The third anchor places "----x" so that the probe ends after its
	// first three dashes.
	lead := "---\ntitle: Cut\nfiller: "
	cutAt := cardHeaderProbe - 3
	filler := pad(cutAt - len(lead) - 1)
	cases := []struct {
		name  string
		text  string
		title string
		err   bool
	}{
		{"closes inside the probe", "---\ntitle: Short\n---\nBody.\n", "Short", false},
		{"closes past the probe", "---\ntitle: Long\nfiller: " + pad(2*cardHeaderProbe) + "\n---\nBody.\n", "Long", false},
		{"the probe cuts a line that begins with the fence", lead + filler + "\n----x\nafter: kept\n---\nBody.\n", "Cut", false},
		{"never closes inside the limit", "---\ntitle: Open\nfiller: " + pad(cardHeaderLimit) + "\n", "", true},
	}
	for _, c := range cases {
		anchor := filepath.Join(dir, strings.ReplaceAll(c.name, " ", "-")+".md")
		write(t, anchor, c.text)
		fm, err := readCardHeader(Disk{}, anchor)
		if (err != nil) != c.err {
			t.Errorf("%s: answered error %v", c.name, err)
			continue
		}
		if err != nil {
			continue
		}
		if fm.Value("title") != c.title {
			t.Errorf("%s: read the title %q, wanted %q", c.name, fm.Value("title"), c.title)
		}
		if c.name == "the probe cuts a line that begins with the fence" && fm.Value("after") != "kept" {
			t.Errorf("%s: the header stopped at the cut line and lost the key after it", c.name)
		}
	}
	if len(cases) != 4 {
		t.Fatalf("ran %d cases, wanted four", len(cases))
	}
}
