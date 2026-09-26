package screen

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestTheScreenLayerLinksNoCharmCode is dinah-603/criteria/38. Among the
// dependencies go list -deps reports for this package and for
// internal/screen/keyboard, for windows, linux and darwin, no module path
// begins charm.land/ or github.com/charmbracelet/, so the key decoders and
// the terminfo reader the terminal UI shares with dinah view --watch link
// none of the terminal UI's libraries.
func TestTheScreenLayerLinksNoCharmCode(t *testing.T) {
	gobin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go is not on PATH, so the dependencies cannot be listed")
	}
	for _, goos := range []string{"windows", "linux", "darwin"} {
		list := exec.Command(gobin, "list", "-deps", "-f", "{{.ImportPath}} {{with .Module}}{{.Path}}{{end}}", ".", "./keyboard")
		list.Env = append(os.Environ(), "GOOS="+goos, "GOARCH=amd64")
		out, err := list.Output()
		if err != nil {
			t.Fatalf("go list for %s: %v", goos, err)
		}
		packages := strings.Split(strings.TrimSpace(string(out)), "\n")
		for _, line := range packages {
			fields := strings.Fields(line)
			if len(fields) == 2 && (strings.HasPrefix(fields[1], "charm.land/") || strings.HasPrefix(fields[1], "github.com/charmbracelet/")) {
				t.Errorf("for %s the screen layer depends on %s, from %s", goos, fields[0], fields[1])
			}
		}
		t.Logf("%s: %d packages read", goos, len(packages))
	}
}
