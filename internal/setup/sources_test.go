package setup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// unregisterSource takes a test's source away again, so that each test
// starts with the sources a build has.
func unregisterSource(id string) {
	sourcesMu.Lock()
	defer sourcesMu.Unlock()
	delete(sources, id)
}

// fakeSource offers the packages in files, a ZIP's path for each version,
// and counts what it fetches.
func fakeSource(t *testing.T, id string, files map[string]string, enabled *bool) *int {
	fetched := 0
	RegisterSource(id, Source{Label: "the test page",
		Enabled: func() bool { return enabled == nil || *enabled },
		Find: func(context.Context) ([]Candidate, error) {
			var out []Candidate
			for version, zip := range files {
				out = append(out, Candidate{Version: version, Fetch: func(_ context.Context, dir string) (string, error) {
					fetched++
					b, err := os.ReadFile(zip)
					if err != nil {
						return "", err
					}
					to := filepath.Join(dir, filepath.Base(zip))
					os.MkdirAll(dir, 0o755)
					return to, os.WriteFile(to, b, 0o644)
				}})
			}
			return out, nil
		}})
	t.Cleanup(func() { unregisterSource(id) })
	return &fetched
}

// With no source registered, as in Dinah's own build, nothing is looked at
// and nothing is offered.
func TestNoSourcesFindNothing(t *testing.T) {
	if ids := SourceIDs(); len(ids) != 0 {
		t.Fatalf("this build registers sources: %v", ids)
	}
	status, err := FindUpdates(context.Background(), []InstalledPackage{{Target: ForMe, Name: "a", Version: "1"}}, t.TempDir(), true)
	if err != nil || len(status.Updates) != 0 || len(status.Available) != 0 || status.Off {
		t.Fatalf("%+v %v", status, err)
	}
}

// A newer version on a source is found for the person's install and for a
// workspace's, an older one is passed over, a file that is not a package is
// passed over and removed, and a package not installed is offered only when
// asked for.
func TestFindUpdatesComparesWithWhatIsInstalled(t *testing.T) {
	pt := personalTargets(t)
	first, err := PlanPersonal(bundle(t, "16", map[string]string{".devin/skills/concept-review/SKILL.md": "Old.\n"}, nil), pt, "1.20.0")
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyPersonal(first, filepath.Join(t.TempDir(), "b")); err != nil {
		t.Fatal(err)
	}
	ws := t.TempDir()
	apply(t, bundle(t, "15", map[string]string{".devin/skills/concept-review/SKILL.md": "Older.\n"}, nil), ws)

	notAPackage := filepath.Join(t.TempDir(), "concept-skill-v99.zip")
	os.WriteFile(notAPackage, []byte("not a package"), 0o644)
	other := bundle(t, "3", map[string]string{".devin/skills/other/SKILL.md": "Other.\n"}, func(m *Manifest) { m.Name = "other-setup" })
	fetched := fakeSource(t, "test-page", map[string]string{
		"17": bundle(t, "17", map[string]string{".devin/skills/concept-review/SKILL.md": "New.\n"}, nil),
		"14": bundle(t, "14", map[string]string{".devin/skills/concept-review/SKILL.md": "Oldest.\n"}, nil),
		"99": notAPackage,
		"3":  other,
	}, nil)

	installed := InstalledPackages(pt, ws)
	if len(installed) != 2 {
		t.Fatalf("installed %+v", installed)
	}
	dir := filepath.Join(t.TempDir(), "package-updates")
	status, err := FindUpdates(context.Background(), installed, dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Updates) != 2 || len(status.Available) != 0 {
		t.Fatalf("%+v", status)
	}
	for _, u := range status.Updates {
		if u.To != "17" || (u.Target == ForMe && u.From != "16") || (u.Target == ForWorkspace && (u.From != "15" || u.Workspace != ws)) {
			t.Fatalf("%+v", u)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.Base(notAPackage))); err == nil {
		t.Fatal("a file that is not a package was kept")
	}
	// Version 3 of another package is older than both installed versions,
	// so it was not fetched until packages not installed are asked for.
	before := *fetched
	status, err = FindUpdates(context.Background(), installed, dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Available) != 1 || status.Available[0].Name != "other-setup" || *fetched <= before {
		t.Fatalf("%+v", status.Available)
	}
}

// A source the person turned off is not asked, and the status says every
// source is off; an error from a source is returned with what the others
// found.
func TestTurnedOffAndFailingSources(t *testing.T) {
	off := false
	fetched := fakeSource(t, "off-page", map[string]string{"2": bundle(t, "2", map[string]string{".devin/rules/a.md": "a"}, nil)}, &off)
	status, err := FindUpdates(context.Background(), []InstalledPackage{{Target: ForMe, Name: "acme-setup", Version: "1"}}, t.TempDir(), true)
	if err != nil || !status.Off || *fetched != 0 || len(status.Updates) != 0 {
		t.Fatalf("%+v %v %d", status, err, *fetched)
	}
	RegisterSource("broken-page", Source{Find: func(context.Context) ([]Candidate, error) { return nil, errors.New("the site is not connected") }})
	t.Cleanup(func() { unregisterSource("broken-page") })
	status, err = FindUpdates(context.Background(), nil, t.TempDir(), true)
	if err == nil || !strings.Contains(err.Error(), "not connected") || status.Off {
		t.Fatalf("%+v %v", status, err)
	}
}

// Registering a source twice, or with no way to find packages, is a
// mistake in the build and panics.
func TestRegisteringASourceBadlyPanics(t *testing.T) {
	RegisterSource("twice", Source{Find: func(context.Context) ([]Candidate, error) { return nil, nil }})
	t.Cleanup(func() { unregisterSource("twice") })
	for name, register := range map[string]func(){
		"twice": func() {
			RegisterSource("twice", Source{Find: func(context.Context) ([]Candidate, error) { return nil, nil }})
		},
		"no find": func() { RegisterSource("no-find", Source{}) },
		"bad id": func() {
			RegisterSource("Bad ID", Source{Find: func(context.Context) ([]Candidate, error) { return nil, nil }})
		},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s did not panic", name)
				}
			}()
			register()
		}()
	}
}

func TestVersionNewer(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{{"17", "16", true}, {"16", "16", false}, {"1.10.0", "1.9.9", true}, {"2", "1.9", true}, {"1.0", "1", false}, {"v3", "2", true}, {"dev", "1", false}, {"1", "dev", false}} {
		if got := versionNewer(c.a, c.b); got != c.want {
			t.Errorf("versionNewer(%q, %q) = %v", c.a, c.b, got)
		}
	}
}
