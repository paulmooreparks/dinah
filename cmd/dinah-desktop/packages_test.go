package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"studio/internal/setup"
	"studio/internal/workspaces"
)

// makePackage writes a package's ZIP named for its version, holding files
// and their manifest.
func makePackage(t *testing.T, name, version string, files map[string]string) string {
	t.Helper()
	m := setup.Manifest{Schema: setup.SchemaVersion, Name: name, Version: version, Built: "2026-10-27", Roots: []string{workspaces.PackageFolder + "/"}, Files: map[string]string{}}
	zp := filepath.Join(t.TempDir(), name+"-v"+version+".zip")
	f, err := os.Create(zp)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	for p, c := range files {
		sum := sha256.Sum256([]byte(c))
		m.Files[p] = hex.EncodeToString(sum[:])
		w, _ := z.Create(p)
		w.Write([]byte(c))
	}
	mb, _ := json.Marshal(m)
	w, _ := z.Create(setup.ManifestPath())
	w.Write(mb)
	z.Close()
	f.Close()
	return zp
}

// A package is planned without writing, then installed into a registered
// workspace, with a file changed by hand reported and kept in the backup,
// and removed again.
func TestInstallPackages(t *testing.T) {
	isolateWorkspaceSettings(t)
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	a := closeBusWhenDone(t, NewApp())
	ws := t.TempDir()
	if _, err := a.SetWorkspace(ws); err != nil {
		t.Fatal(err)
	}
	first := makePackage(t, "team-setup", "1", map[string]string{".devin/skills/review/SKILL.md": "one", ".devin/rules/layout.md": "rules"})
	plan, err := a.PlanPackages(ws, first)
	if err != nil || plan.Added != 2 || plan.From != "" || plan.To != "1" {
		t.Fatalf("plan %+v %v", plan, err)
	}
	if _, err := os.Stat(filepath.Join(ws, ".devin", "skills", "review", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("planning wrote a file")
	}
	if _, err := a.InstallPackages(ws, first, plan.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := a.InstallPackages(ws, first, plan.Token); err == nil {
		t.Fatal("a plan made before the install was installed again")
	}
	os.WriteFile(filepath.Join(ws, ".devin", "rules", "layout.md"), []byte("mine"), 0o644)
	second := makePackage(t, "team-setup", "2", map[string]string{".devin/skills/review/SKILL.md": "two", ".devin/rules/layout.md": "rules 2"})
	plan, err = a.PlanPackages(ws, second)
	if err != nil || plan.From != "1" || plan.Updated != 2 || len(plan.Changed) != 1 || plan.Changed[0] != ".devin/rules/layout.md" {
		t.Fatalf("second plan %+v %v", plan, err)
	}
	done, err := a.InstallPackages(ws, second, plan.Token)
	if err != nil || done.Backup == "" {
		t.Fatalf("install %+v %v", done, err)
	}
	if b, _ := os.ReadFile(filepath.Join(done.Backup, ".devin", "rules", "layout.md")); string(b) != "mine" {
		t.Fatalf("the hand-changed file was not kept: %q", b)
	}
	if _, err := a.PlanPackages(t.TempDir(), second); err == nil {
		t.Fatal("a folder that is no registered workspace was planned")
	}
	if installed, _ := a.InstalledPackages(); len(installed) != 1 || installed[0].Target != setup.ForWorkspace || installed[0].Version != "2" {
		t.Fatalf("installed %+v", installed)
	}

	removal, err := a.PlanRemovePackages(setup.ForWorkspace, ws, "team-setup")
	if err != nil || removal.Removed != 2 {
		t.Fatalf("removal %+v %v", removal, err)
	}
	if _, err := a.RemovePackages(setup.ForWorkspace, ws, "team-setup", "stale"); err == nil {
		t.Fatal("a removal was made from a plan the person did not see")
	}
	if _, err := a.RemovePackages(setup.ForWorkspace, ws, "team-setup", removal.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ws, ".devin", "skills", "review")); err == nil {
		t.Fatal("the package's skill is still in the workspace")
	}
}

// A package installed for the person puts its skills in the person's skills
// folder, which under a test's state folder is inside it, says which of
// them the workspace in front has too, and what stays in the package's
// folder; it is removed again by name.
func TestInstallPersonalPackages(t *testing.T) {
	isolateWorkspaceSettings(t)
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	a := closeBusWhenDone(t, NewApp())
	ws := t.TempDir()
	if _, err := a.SetWorkspace(ws); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(ws, ".devin", "skills", "review"), 0o755)
	zp := makePackage(t, "team-setup", "3", map[string]string{".devin/skills/review/SKILL.md": "Read `.devin/flows/a.flow.md`.\n", ".devin/flows/a.flow.md": "A flow.\n"})
	plan, err := a.PlanPersonalPackages(ws, zp)
	if err != nil || plan.Copied != 1 || len(plan.Duplicates) != 1 || plan.Duplicates[0] != "review" || strings.Join(plan.HomeOnly, ",") != ".devin/flows/" {
		t.Fatalf("plan %+v %v", plan, err)
	}
	done, err := a.InstallPersonalPackages(ws, zp, plan.Token)
	if err != nil {
		t.Fatal(err)
	}
	skill, err := os.ReadFile(filepath.Join(configDir(), "devin-skills", "review", "SKILL.md"))
	if err != nil || !strings.Contains(string(skill), filepath.ToSlash(done.Home)+"/.devin/flows/a.flow.md") {
		t.Fatalf("skill %q %v", skill, err)
	}
	removal, err := a.PlanRemovePackages(setup.ForMe, ws, "")
	if err != nil || removal.Name != "team-setup" {
		t.Fatalf("removal %+v %v", removal, err)
	}
	if _, err := a.RemovePackages(setup.ForMe, ws, "", removal.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(configDir(), "devin-skills", "review")); err == nil {
		t.Fatal("the copied skill is still there")
	}
	if _, err := a.PlanRemovePackages(setup.ForMe, ws, ""); err == nil {
		t.Fatal("removing with nothing installed was not refused")
	}
}

// testSource is the package source the update test registers once for the
// whole test binary, offering whatever testSourceFiles holds.
var (
	testSourceOnce  sync.Once
	testSourceMu    sync.Mutex
	testSourceFiles map[string]string
)

func useTestSource(t *testing.T, files map[string]string) {
	testSourceOnce.Do(func() {
		setup.RegisterSource("desktop-test-page", setup.Source{Label: "the test page", Find: func(context.Context) ([]setup.Candidate, error) {
			testSourceMu.Lock()
			defer testSourceMu.Unlock()
			var out []setup.Candidate
			for version, zp := range testSourceFiles {
				out = append(out, setup.Candidate{Version: version, Fetch: func(_ context.Context, dir string) (string, error) {
					b, err := os.ReadFile(zp)
					if err != nil {
						return "", err
					}
					to := filepath.Join(dir, filepath.Base(zp))
					os.MkdirAll(dir, 0o755)
					return to, os.WriteFile(to, b, 0o644)
				}})
			}
			return out, nil
		}})
	})
	testSourceMu.Lock()
	testSourceFiles = files
	testSourceMu.Unlock()
	t.Cleanup(func() {
		testSourceMu.Lock()
		testSourceFiles = nil
		testSourceMu.Unlock()
	})
}

// A newer version on a source is offered for the person's install, never
// installed without asking; a file beside it that is not a package is
// passed over; and a copy the person changed is said in the offer, which is
// made once.
func TestPackagesUpdateFromASource(t *testing.T) {
	isolateWorkspaceSettings(t)
	a := closeBusWhenDone(t, NewApp())
	var events []string
	a.sink = func(name string, data any) {
		if strings.HasPrefix(name, "packages:") {
			b, _ := json.Marshal(data)
			events = append(events, name+" "+string(b))
		}
	}
	tg, err := personalPackageTargets()
	if err != nil {
		t.Fatal(err)
	}
	skill := func(text string) map[string]string {
		return map[string]string{".devin/skills/concept-review/SKILL.md": text, ".devin/rules/style.md": "Style.\n"}
	}
	p, err := setup.PlanPersonal(makePackage(t, "team-setup", "16", skill("Old.\n")), tg, "1.20.0")
	if err != nil {
		t.Fatal(err)
	}
	if err := setup.ApplyPersonal(p, filepath.Join(t.TempDir(), "b")); err != nil {
		t.Fatal(err)
	}
	notAPackage := filepath.Join(t.TempDir(), "concept-skill-v11.zip")
	os.WriteFile(notAPackage, []byte("not a package"), 0o644)
	useTestSource(t, map[string]string{"17": makePackage(t, "team-setup", "17", skill("New.\n")), "11": notAPackage, "15": makePackage(t, "team-setup", "15", skill("Older.\n"))})
	if err := a.checkPackageUpdates(); err != nil {
		t.Fatal(err)
	}
	if m, _ := setup.InstalledPersonal(tg, "team-setup"); m == nil || m.Version != "16" {
		t.Fatalf("an update was installed without asking: %+v", m)
	}
	if len(events) != 1 || !strings.HasPrefix(events[0], "packages:update-available") || !strings.Contains(events[0], `"to":"17"`) || strings.Contains(events[0], `"reason"`) {
		t.Fatalf("events %v", events)
	}
	// The person installs it from the offer.
	p, err = setup.PlanPersonal(filepath.Join(packageUpdatesDir(), "team-setup-v17.zip"), tg, "1.20.0")
	if err != nil {
		t.Fatal(err)
	}
	if err := setup.ApplyPersonal(p, filepath.Join(t.TempDir(), "b17")); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(tg.Folders[workspaces.Skills], "concept-review", "SKILL.md")); string(b) != "New.\n" {
		t.Fatalf("skill %q", b)
	}

	// A hand-changed copy makes the next offer say so, and it is made once.
	os.WriteFile(filepath.Join(tg.Folders[workspaces.Skills], "concept-review", "SKILL.md"), []byte("My words.\n"), 0o644)
	useTestSource(t, map[string]string{"18": makePackage(t, "team-setup", "18", skill("Newer.\n"))})
	events = nil
	for i := 0; i < 2; i++ {
		if err := a.checkPackageUpdates(); err != nil {
			t.Fatal(err)
		}
	}
	if m, _ := setup.InstalledPersonal(tg, "team-setup"); m.Version != "17" {
		t.Fatal("a hand-changed install was updated without asking")
	}
	if len(events) != 1 || !strings.HasPrefix(events[0], "packages:update-available") || !strings.Contains(events[0], "changed by hand") {
		t.Fatalf("events %v", events)
	}
	status, err := a.FindPackageUpdates()
	if err != nil || len(status.Updates) != 1 || !strings.Contains(status.Updates[0].Reason, "changed by hand") {
		t.Fatalf("status %+v %v", status, err)
	}
}
