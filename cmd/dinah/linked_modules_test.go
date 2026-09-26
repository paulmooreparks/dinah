package main

import (
	"os/exec"
	"sort"
	"strings"
	"testing"
)

// TestTheMainBinaryLinksNoCharmCode is dinah-603/criteria/37. The modules go
// list -deps reports for ./cmd/dinah with no build tags, for windows, linux
// and darwin, are exactly golang.org/x/sys, golang.org/x/term and
// golang.org/x/text, so the main dinah binary links none of the terminal
// UI's libraries; with the tui tag, which builds dinah-tui, they include
// charm.land/bubbletea/v2.
func TestTheMainBinaryLinksNoCharmCode(t *testing.T) {
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go is not on PATH, so the linked modules cannot be listed")
	}
	want := "golang.org/x/sys golang.org/x/term golang.org/x/text"
	for _, goos := range []string{"windows", "linux", "darwin"} {
		plain := moduleNames(linkedModules(t, gobin, ".", goos, ""))
		t.Logf("%s, no tags: %d modules: %s", goos, len(plain), strings.Join(plain, " "))
		if got := strings.Join(plain, " "); got != want {
			t.Errorf("for %s with no build tags dinah links %s, and it must link exactly %s", goos, got, want)
		}
		tagged := moduleNames(linkedModules(t, gobin, ".", goos, "tui"))
		t.Logf("%s, tags tui: %d modules", goos, len(tagged))
		if !contains(tagged, "charm.land/bubbletea/v2") {
			t.Errorf("for %s with the tui tag dinah-tui does not link charm.land/bubbletea/v2: %s", goos, strings.Join(tagged, " "))
		}
	}
}

// moduleNames answers the module paths of a set, sorted.
func moduleNames(modules map[string][2]string) []string {
	var names []string
	for path := range modules {
		names = append(names, path)
	}
	sort.Strings(names)
	return names
}
