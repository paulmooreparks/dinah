package main

import (
	"fmt"
	"strings"
	"testing"
)

// cfHTML builds a clipboard entry in Windows' HTML clipboard format around
// fragment, with its header's byte offsets, as a program writes one.
func cfHTML(fragment string, withFragment bool) []byte {
	const head = "Version:0.9\r\nStartHTML:%010d\r\nEndHTML:%010d\r\n"
	const frag = "StartFragment:%010d\r\nEndFragment:%010d\r\n"
	pre, post := "<html><body>\r\n<!--StartFragment-->", "<!--EndFragment-->\r\n</body></html>"
	headerLen := len(fmt.Sprintf(head, 0, 0))
	if withFragment {
		headerLen += len(fmt.Sprintf(frag, 0, 0))
	}
	startHTML := headerLen
	startFrag := startHTML + len(pre)
	endFrag := startFrag + len(fragment)
	endHTML := endFrag + len(post)
	h := fmt.Sprintf(head, startHTML, endHTML)
	if withFragment {
		h += fmt.Sprintf(frag, startFrag, endFrag)
	}
	return []byte(h + pre + fragment + post)
}

func TestHTMLFragment(t *testing.T) {
	// The fragment is UTF-8, and the offsets count its bytes.
	frag := "<h2>Release Notes</h2><p>Grüße <b>bold</b> → 10 €</p>"
	if got := htmlFragment(cfHTML(frag, true)); got != frag {
		t.Fatalf("fragment %q", got)
	}
	// Without fragment offsets, the whole document between StartHTML and
	// EndHTML.
	if got := htmlFragment(cfHTML(frag, false)); !strings.Contains(got, frag) || !strings.HasPrefix(got, "<html>") {
		t.Fatalf("document %q", got)
	}
	// Not the format, or offsets outside the data.
	if got := htmlFragment([]byte("<p>plain</p>")); got != "" {
		t.Fatalf("not the format: %q", got)
	}
	if got := htmlFragment([]byte("Version:0.9\r\nStartFragment:10\r\nEndFragment:99999\r\n")); got != "" {
		t.Fatalf("offsets past the end: %q", got)
	}
}
