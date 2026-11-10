package claudeagent

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// installRelease makes the state folder look as if a release were installed,
// with the Node.js range given in its package.json.
func installRelease(t *testing.T, state, version, node string) {
	t.Helper()
	entry := EntryOf(state, version)
	if err := os.MkdirAll(filepath.Dir(entry), 0o700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(entry, []byte("// stand-in"), 0o600)
	os.WriteFile(filepath.Join(filepath.Dir(filepath.Dir(entry)), "package.json"), []byte(`{"version":"`+version+`","engines":{"node":"`+node+`"}}`), 0o600)
}

// earlier and later are releases on either side of the pinned one.
const (
	earlier = "0.1.0"
	later   = "999.0.0"
)

// A release's number is three numbers. A preview's number, a range and a
// path are not one, so none of them can become a folder or reach npm.
func TestOnlyAReleaseNumberIsARelease(t *testing.T) {
	for _, v := range []string{"0.101.0", "1.0.0", "10.20.30"} {
		if !ValidRelease(v) {
			t.Errorf("%q was refused", v)
		}
	}
	for _, v := range []string{"", "latest", "0.102.1-preview.7", "1.0", "1.0.0.0", "01.0.0", "^1.0.0", "1.0.0 ", "../1.0.0", "1.0.0/..", "v1.0.0"} {
		if ValidRelease(v) {
			t.Errorf("%q was taken as a release", v)
		}
	}
}

func TestNewerComparesNumbersAndNotText(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"0.102.0", "0.101.0", true}, {"0.101.0", "0.102.0", false}, {"0.101.0", "0.101.0", false},
		{"0.100.0", "0.99.0", true}, {"1.0.0", "0.999.999", true}, {"0.101.10", "0.101.9", true},
		{"0.102.1-preview.7", "0.101.0", false}, {"0.102.0", "", false}, {"", "0.101.0", false},
	} {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v", c.a, c.b, got)
		}
	}
}

// Claude runs the release the person chose while it is later than the pinned
// one, the pinned one otherwise, and the latest earlier release while the
// pinned one is not installed.
func TestTheReleaseInUse(t *testing.T) {
	state := t.TempDir()
	if got := InUse(state, ""); got != "" {
		t.Fatalf("with nothing installed: %q", got)
	}
	installRelease(t, state, "0.0.9", ">=22")
	installRelease(t, state, earlier, ">=22")
	if got := InUse(state, ""); got != earlier {
		t.Fatalf("with only earlier releases: %q", got)
	}
	if got := Releases(state); !reflect.DeepEqual(got, []string{earlier, "0.0.9"}) {
		t.Fatalf("the installed releases: %v", got)
	}
	// A later release nobody chose is not run in the pinned one's place.
	installRelease(t, state, later, ">=22")
	if got := InUse(state, ""); got != earlier {
		t.Fatalf("with a later release that was not chosen: %q", got)
	}
	if got := InUse(state, later); got != later {
		t.Fatalf("with the later release chosen: %q", got)
	}
	installRelease(t, state, Version, ">=22")
	if got := InUse(state, ""); got != Version {
		t.Fatalf("with the pinned release installed: %q", got)
	}
	if got := InUse(state, later); got != later {
		t.Fatalf("with the later release chosen and the pinned one installed: %q", got)
	}
	// A choice the pin has caught up with, or passed, counts for nothing, and
	// so does one that is not installed or is not a release.
	for _, chosen := range []string{Version, earlier, "999.0.1", "0.102.1-preview.7", "../" + later} {
		if got := InUse(state, chosen); got != Version {
			t.Errorf("with %q chosen: %q", chosen, got)
		}
	}
}

// A folder that only looks like a release, because its package says another
// number or its program is missing, is not an installed release.
func TestAHalfInstalledReleaseIsNotInstalled(t *testing.T) {
	state := t.TempDir()
	installRelease(t, state, earlier, ">=22")
	os.WriteFile(filepath.Join(filepath.Dir(filepath.Dir(EntryOf(state, earlier))), "package.json"), []byte(`{"version":"0.0.1"}`), 0o600)
	installRelease(t, state, "0.2.0", ">=22")
	os.Remove(EntryOf(state, "0.2.0"))
	if got := Releases(state); len(got) != 0 {
		t.Fatalf("the installed releases: %v", got)
	}
}

func TestPruneKeepsTheNamedReleasesAndWhatIsNotARelease(t *testing.T) {
	state := t.TempDir()
	for _, v := range []string{earlier, Version, later} {
		installRelease(t, state, v, ">=22")
	}
	other := filepath.Join(state, "claude-agent-acp", "installing-123")
	os.MkdirAll(other, 0o700)
	// What a removal that could not finish left behind goes too.
	left := filepath.Join(state, "claude-agent-acp", removingPrefix+"0.0.1-1")
	os.MkdirAll(filepath.Join(left, "node_modules"), 0o700)
	Prune(state, Version, later, "")
	if _, err := os.Stat(left); !os.IsNotExist(err) {
		t.Fatalf("what a removal left behind: %v", err)
	}
	if got := Releases(state); !reflect.DeepEqual(got, []string{later, Version}) {
		t.Fatalf("after pruning: %v", got)
	}
	if _, err := os.Stat(DirOf(state, earlier)); !os.IsNotExist(err) {
		t.Fatalf("the earlier release's folder: %v", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("a folder that is not a release's was removed: %v", err)
	}
	if err := Remove(state, later); err != nil || InstalledRelease(state, later) {
		t.Fatalf("removing a release: %v", err)
	}
	if err := Remove(state, ".."); err == nil {
		t.Fatal("a path was removed as a release")
	}
}

// The oldest Node.js a release supports is read from its own package, and is
// the pinned release's when the package gives it in another form.
func TestTheNodeAReleaseNeeds(t *testing.T) {
	state := t.TempDir()
	installRelease(t, state, later, ">=24")
	installRelease(t, state, earlier, "^20 || >=22")
	installRelease(t, state, Version, ">=99")
	for version, want := range map[string]int{later: 24, earlier: MinimumNode, Version: MinimumNode, "0.3.0": MinimumNode} {
		if got := NodeNeeded(state, version); got != want {
			t.Errorf("Node.js for %s: %d, want %d", version, got, want)
		}
	}
}

// npm's answer gives the release its latest tag is on, when that release was
// published, and the Node.js it needs.
func TestLatestIsReadFromNpmsAnswer(t *testing.T) {
	r, err := parseLatest([]byte(`{"version":"0.102.0","time":{"created":"2026-03-26T11:29:04.549Z","0.101.0":"2026-11-05T21:11:45.924Z","0.102.0":"2026-11-09T19:57:16.031Z","0.102.1-preview.7":"2026-11-10T08:22:27.162Z"},"engines":{"node":">=22"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 11, 9, 19, 57, 16, 31000000, time.UTC); r.Version != "0.102.0" || !r.Released.Equal(want) || r.Node != 22 {
		t.Fatalf("read %+v", r)
	}
	// A release with no date is still a release; its date is unknown.
	if r, err := parseLatest([]byte(`{"version":"0.102.0"}`)); err != nil || !r.Released.IsZero() || r.Node != 0 {
		t.Fatalf("with no date: %+v %v", r, err)
	}
	for _, bad := range []string{``, `[]`, `{"version":"0.102.1-preview.7"}`, `{"version":"../x"}`, `{}`} {
		if r, err := parseLatest([]byte(bad)); err == nil {
			t.Errorf("%q was read as %+v", bad, r)
		}
	}
}
