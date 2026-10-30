package setup

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/profile"
)

// bundle writes a package's ZIP holding files and a manifest for them, and
// returns its path. edit, when not nil, changes the manifest before it is
// written, which is how the tests build damaged packages.
func bundle(t *testing.T, version string, files map[string]string, edit func(*Manifest)) string {
	t.Helper()
	m := Manifest{Schema: SchemaVersion, Name: "acme-setup", Version: version, Built: "2026-10-07", MinTool: "1.7.0",
		Roots: []string{".devin/", "_templates/", "README-hybrid.md"}, Files: map[string]string{}}
	for p, c := range files {
		m.Files[p] = digest([]byte(c))
	}
	if edit != nil {
		edit(&m)
	}
	zp := filepath.Join(t.TempDir(), "hybrid-setup-v"+version+".zip")
	f, err := os.Create(zp)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	add := func(name string, b []byte) {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write(b)
	}
	for p, c := range files {
		add(p, []byte(c))
	}
	mb, _ := json.Marshal(m)
	add(ManifestPath(), mb)
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return zp
}

func readWS(t *testing.T, ws, rel string) (string, bool) {
	b, err := os.ReadFile(filepath.Join(ws, filepath.FromSlash(rel)))
	if err != nil {
		return "", false
	}
	return string(b), true
}

func writeWS(t *testing.T, ws, rel, text string) {
	t.Helper()
	p := filepath.Join(ws, filepath.FromSlash(rel))
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func apply(t *testing.T, zp, ws string) *Plan {
	t.Helper()
	plan, err := PlanApply(zp, ws, "1.7.0")
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan, filepath.Join(t.TempDir(), "backup")); err != nil {
		t.Fatal(err)
	}
	return plan
}

func kinds(p *Plan) map[string]Action {
	out := map[string]Action{}
	for _, a := range p.Actions {
		out[a.Path] = a
	}
	return out
}

var v7 = map[string]string{
	".devin/skills/concept-hybrid/SKILL.md": "hybrid v7",
	".devin/skills/old-skill/SKILL.md":      "retired in v8",
	".devin/rules/writing.md":               "rules v7",
	"_templates/edit-pipeline/write.md":     "write v7",
	"README-hybrid.md":                      "# Version 7",
}

var v8 = map[string]string{
	".devin/skills/concept-hybrid/SKILL.md": "hybrid v8",
	".devin/rules/writing.md":               "rules v7",
	"_templates/edit-pipeline/write.md":     "write v8",
	"_templates/edit-pipeline/write-d2.md":  "new in v8",
	"README-hybrid.md":                      "# Version 8",
}

// The manifest's place comes from the profile.
func TestTheManifestPathComesFromTheProfile(t *testing.T) {
	if ManifestPath() != profile.Current().PackageManifest || ManifestPath() == "" {
		t.Fatalf("ManifestPath is %q", ManifestPath())
	}
}

func TestAFreshInstallWritesEverythingAndRecordsIt(t *testing.T) {
	ws := t.TempDir()
	writeWS(t, ws, "Orchard/customer.json", `{"code":"Orchard"}`)
	plan := apply(t, bundle(t, "7", v7, nil), ws)
	if c := plan.Counts(); c[Add] != len(v7) || plan.From != "" {
		t.Fatalf("%+v", plan)
	}
	for p, want := range v7 {
		if got, _ := readWS(t, ws, p); got != want {
			t.Errorf("%s = %q", p, got)
		}
	}
	m, err := Installed(ws)
	if err != nil || m == nil || m.Version != "7" {
		t.Fatalf("no installed record: %v %+v", err, m)
	}
	if got, _ := readWS(t, ws, "Orchard/customer.json"); got != `{"code":"Orchard"}` {
		t.Fatal("a customer file changed")
	}
}

func TestAnUpdateReplacesRetiresAndKeepsThePersonsChanges(t *testing.T) {
	ws := t.TempDir()
	apply(t, bundle(t, "7", v7, nil), ws)
	// The person changed one shipped file and added a file of their own.
	writeWS(t, ws, "_templates/edit-pipeline/write.md", "write v7, with my change")
	writeWS(t, ws, ".devin/skills/my-skill/SKILL.md", "mine")
	writeWS(t, ws, "Orchard/customer.json", `{"code":"Orchard"}`)

	zp := bundle(t, "8", v8, nil)
	plan, err := PlanApply(zp, ws, "1.7.0")
	if err != nil {
		t.Fatal(err)
	}
	k := kinds(plan)
	for p, want := range map[string]Action{
		".devin/skills/concept-hybrid/SKILL.md": {Kind: Update},
		".devin/rules/writing.md":               {Kind: Unchanged},
		"_templates/edit-pipeline/write.md":     {Kind: Update, Modified: true},
		"_templates/edit-pipeline/write-d2.md":  {Kind: Add},
		".devin/skills/old-skill/SKILL.md":      {Kind: Remove},
	} {
		if got := k[p]; got.Kind != want.Kind || got.Modified != want.Modified {
			t.Errorf("%s: %+v, want %+v", p, got, want)
		}
	}
	if _, listed := k[".devin/skills/my-skill/SKILL.md"]; listed {
		t.Error("the plan touches a file no package shipped")
	}
	if plan.From != "7" || plan.To != "8" {
		t.Errorf("%s -> %s", plan.From, plan.To)
	}

	backup := filepath.Join(t.TempDir(), "backup")
	if err := Apply(plan, backup); err != nil {
		t.Fatal(err)
	}
	if _, ok := readWS(t, ws, ".devin/skills/old-skill/SKILL.md"); ok {
		t.Error("the retired skill is still installed")
	}
	if isDir(filepath.Join(ws, ".devin", "skills", "old-skill")) {
		t.Error("the retired skill's empty folder is left behind")
	}
	if got, _ := readWS(t, ws, ".devin/skills/my-skill/SKILL.md"); got != "mine" {
		t.Error("the person's own file was touched")
	}
	if got, _ := readWS(t, ws, "_templates/edit-pipeline/write.md"); got != "write v8" {
		t.Error("the changed file was not updated")
	}
	if got, _ := readWS(t, backup, "_templates/edit-pipeline/write.md"); got != "write v7, with my change" {
		t.Error("the person's change is not in the backup")
	}
	if got, _ := readWS(t, backup, ".devin/skills/old-skill/SKILL.md"); got != "retired in v8" {
		t.Error("the retired file is not in the backup")
	}
	if _, ok := readWS(t, backup, ".devin/rules/writing.md"); ok {
		t.Error("an unchanged file was backed up")
	}
	if m, _ := Installed(ws); m.Version != "8" {
		t.Error("the installed record was not updated")
	}

	// Applying the same package again changes nothing.
	again, err := PlanApply(zp, ws, "1.7.0")
	if err != nil {
		t.Fatal(err)
	}
	if c := again.Counts(); c[Unchanged] != len(v8) || len(again.Actions) != len(v8) {
		t.Errorf("a second apply is not a no-op: %+v", again.Actions)
	}
}

// A workspace whose files were extracted by hand has no record, so nothing
// can be retired and every differing file counts as possibly changed and is
// backed up.
func TestAnUpdateOverAnUnrecordedInstallBacksUpEverythingItReplaces(t *testing.T) {
	ws := t.TempDir()
	for p, c := range v7 {
		writeWS(t, ws, p, c)
	}
	plan, err := PlanApply(bundle(t, "8", v8, nil), ws, "1.7.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Warnings) == 0 || !strings.Contains(plan.Warnings[0], "no record of an installed package") || !strings.Contains(plan.Warnings[0], ".devin/") {
		t.Errorf("no warning: %v", plan.Warnings)
	}
	k := kinds(plan)
	if a := k["_templates/edit-pipeline/write.md"]; a.Kind != Update || !a.Unknown {
		t.Errorf("%+v", a)
	}
	if _, ok := k[".devin/skills/old-skill/SKILL.md"]; ok {
		t.Error("a file was retired without a record of what was installed")
	}
	backup := filepath.Join(t.TempDir(), "b")
	if err := Apply(plan, backup); err != nil {
		t.Fatal(err)
	}
	if got, _ := readWS(t, backup, "_templates/edit-pipeline/write.md"); got != "write v7" {
		t.Error("not backed up")
	}
}

func TestATooOldProgramIsRefused(t *testing.T) {
	want := "needs " + profile.Current().PackageTool + " 1.7.0"
	if _, err := PlanApply(bundle(t, "8", v8, nil), t.TempDir(), "1.6.0"); err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("%v", err)
	}
	plan, err := PlanApply(bundle(t, "8", v8, nil), t.TempDir(), "0.0.0-dev+abc")
	if err != nil || len(plan.Warnings) == 0 {
		t.Fatalf("a development build should warn, not refuse: %v %v", err, plan)
	}
	if ok, known := versionAtLeast("1.10.0", "1.7.0"); !ok || !known {
		t.Error("1.10.0 < 1.7.0")
	}
}

func TestABackupInsideTheWorkspaceIsRefused(t *testing.T) {
	ws := t.TempDir()
	plan, err := PlanApply(bundle(t, "7", v7, nil), ws, "1.7.0")
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan, filepath.Join(ws, "backup")); err == nil {
		t.Fatal("a backup inside the workspace was accepted")
	}
	if _, ok := readWS(t, ws, "README-hybrid.md"); ok {
		t.Fatal("files were written before the refusal")
	}
}

func TestADamagedPackageIsRefused(t *testing.T) {
	for name, edit := range map[string]func(*Manifest){
		"wrong hash":       func(m *Manifest) { m.Files["README-hybrid.md"] = digest([]byte("other")) },
		"unlisted file":    func(m *Manifest) { delete(m.Files, "README-hybrid.md") },
		"missing file":     func(m *Manifest) { m.Files[".devin/ghost.md"] = digest(nil) },
		"outside roots":    func(m *Manifest) { m.Roots = []string{".devin/"} },
		"escaping path":    func(m *Manifest) { m.Files["../evil.md"] = digest(nil) },
		"newer schema":     func(m *Manifest) { m.Schema = SchemaVersion + 1 },
		"customer root":    func(m *Manifest) { m.Roots = append(m.Roots, "../Orchard/") },
		"absolute windows": func(m *Manifest) { m.Files[`C:/x.md`] = digest(nil) },
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := PlanApply(bundle(t, "7", v7, edit), t.TempDir(), "1.7.0"); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}

func TestAZipWithoutAManifestIsRefused(t *testing.T) {
	zp := filepath.Join(t.TempDir(), "old.zip")
	f, _ := os.Create(zp)
	z := zip.NewWriter(f)
	w, _ := z.Create("README-hybrid.md")
	w.Write([]byte("# Version 7"))
	z.Close()
	f.Close()
	if _, err := PlanApply(zp, t.TempDir(), "1.7.0"); err == nil || !strings.Contains(err.Error(), "not a package") || !strings.Contains(err.Error(), ManifestPath()) {
		t.Fatalf("%v", err)
	}
}

func TestADifferentPackageIsRefused(t *testing.T) {
	ws := t.TempDir()
	apply(t, bundle(t, "7", v7, nil), ws)
	if _, err := PlanApply(bundle(t, "1", v8, func(m *Manifest) { m.Name = "acme-onepager-setup" }), ws, "1.7.0"); err == nil {
		t.Fatal("a different package was applied over the installed one")
	}
}

// A package that brings its own record of personal copies is refused, since
// that record says which files a later update removes.
func TestAPackageWithACopiesRecordIsRefused(t *testing.T) {
	zp := bundle(t, "2", map[string]string{".devin/rules/a.md": "rule", CopiesPath: `{"copies":{"C:/Windows/win.ini":"x"}}`}, nil)
	if _, _, err := readBundle(zp); err == nil || !strings.Contains(err.Error(), CopiesPath) {
		t.Fatalf("a package with %s was read: %v", CopiesPath, err)
	}
}

// The manifest keeps its JSON shape, field for field, so that its
// packages install unchanged.
func TestTheManifestKeepsItsJSONShape(t *testing.T) {
	data := []byte(`{"schema":1,"name":"acme-setup","version":"16","built":"2026-10-13","min_tool":"1.20.0","roots":[".devin/"],"files":{".devin/rules/a.md":"` + digest([]byte("a")) + `"}}`)
	m, err := parseManifest(data)
	if err != nil || m.Name != "acme-setup" || m.MinTool != "1.20.0" || m.Built != "2026-10-13" {
		t.Fatalf("%+v %v", m, err)
	}
	again, _ := json.Marshal(m)
	var a, b map[string]any
	json.Unmarshal(data, &a)
	json.Unmarshal(again, &b)
	if len(a) != len(b) {
		t.Fatalf("the shape changed: %s", again)
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			t.Fatalf("the field %s is lost: %s", k, again)
		}
	}
}
