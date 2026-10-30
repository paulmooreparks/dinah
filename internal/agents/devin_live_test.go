//go:build live

package agents

import (
	"testing"
)

// The Devin command line installed on this machine is found and answers the
// protocol's initialize, which costs nothing.
func TestFindDevinLive(t *testing.T) {
	path, found := FindDevin(nil)
	for _, c := range found {
		t.Logf("%s: version %q, problem %q", c.Path, c.Version, c.Problem)
	}
	t.Logf("Devin Desktop: %q", DevinDesktop())
	if path == "" {
		t.Fatal(DevinMissing(found, DevinDesktop()))
	}
}
