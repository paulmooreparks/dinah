package setup

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"studio/internal/workspaces"
)

// The tests register document types and flows the way another build would,
// from its own repository: document types are folders copied to a folder of
// their own, and flows are files whose calls to another flow of the same
// package are made calls to the person's copy.
func init() {
	workspaces.RegisterPackageKind("doctypes", workspaces.PackageKind{Label: "Document Types",
		Personal: func(state string, _ bool) (string, error) { return filepath.Join(state, "home", "doctypes"), nil }})
	workspaces.RegisterPackageKind("flows", workspaces.PackageKind{Label: "Flows", FileItems: true,
		Personal: func(state string, _ bool) (string, error) { return filepath.Join(state, "flows"), nil },
		Rewrite:  flowCalls})
}

var flowCall = regexp.MustCompile(`\bworkspace:([A-Za-z0-9_.-]+\.flow\.md)\b`)

// flowCalls makes a call to one of the package's flows a call to the
// person's copy, as a build's installer does.
func flowCalls(text string, items []string) string {
	return flowCall.ReplaceAllStringFunc(text, func(m string) string {
		name := strings.TrimPrefix(m, "workspace:")
		if slices.Contains(items, name) {
			return "mine:" + name
		}
		return m
	})
}

func personalTargets(t *testing.T) PersonalTargets {
	root := t.TempDir()
	return PersonalTargets{
		Packages: filepath.Join(root, "packages"),
		Folders: map[string]string{
			workspaces.Skills: filepath.Join(root, "devin", "skills"),
			"doctypes":        filepath.Join(root, "home", "doctypes"),
			"flows":           filepath.Join(root, "flows"),
		},
	}
}

var personalFiles = map[string]string{
	".devin/skills/concept-review/SKILL.md": "---\nname: concept-review\n---\nRead `.devin/rules/concept-conventions.md` and `.devin/skills/concept-hybrid/SKILL.md`.\nSkip `.devin/` when searching. See customers/.devin/rules/x.md and `_templates/rubrics/r.md`.\n",
	".devin/skills/concept-hybrid/SKILL.md": "---\nname: concept-hybrid\n---\nUse `.devin/doctypes/concept/template.md`.\n",
	".devin/doctypes/concept/doctype.json":  `{"id":"concept","name":"Concept","template":"template.md","editing":{"flow":"change-a-passage.flow.md","values":{"rubric":"_templates/rubrics/r.md"}}}`,
	".devin/doctypes/concept/template.md":   "# {{title}}\n",
	".devin/flows/concept.flow.md":          "Run `workspace:change-a-passage.flow.md` and `workspace:someone-else.flow.md`; read `.devin/flows/concept.flow.md`.\n",
	".devin/flows/change-a-passage.flow.md": "Read `.devin/rules/writers-brief.md`.\n",
	".devin/rules/concept-conventions.md":   "Rules.\n",
	".devin/rules/writers-brief.md":         "Brief.\n",
	"_templates/rubrics/r.md":               "Rubric.\n",
	"README-hybrid.md":                      "Read me.\n",
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Targets has a folder for every registered kind with a personal folder,
// inside the state folder when it is isolated.
func TestTargetsFollowTheRegisteredKinds(t *testing.T) {
	state := t.TempDir()
	tg, err := Targets(state, true)
	if err != nil {
		t.Fatal(err)
	}
	if tg.Packages != filepath.Join(state, "packages") || tg.Folders[workspaces.Skills] != filepath.Join(state, "devin-skills") ||
		tg.Folders["doctypes"] != filepath.Join(state, "home", "doctypes") || tg.Folders["flows"] != filepath.Join(state, "flows") || len(tg.Folders) != 3 {
		t.Fatalf("%+v", tg)
	}
}

// A personal install unpacks the package into its home and copies the
// skills, document types and flows to the person's folders, with the
// package's references made absolute and calls between its flows made to
// the copies.
func TestAPersonalInstallCopiesWithAbsoluteReferences(t *testing.T) {
	pt := personalTargets(t)
	zp := bundle(t, "16", personalFiles, nil)
	p, err := PlanPersonal(zp, pt, "1.19.0")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(pt.Packages); err == nil {
		t.Fatal("planning wrote the packages folder")
	}
	if strings.Join(p.Skills, ",") != "concept-hybrid,concept-review" || p.CopyCounts()[Add] != 6 || len(p.Conflicts) != 0 {
		t.Fatalf("%+v", p.Copies)
	}
	if strings.Join(p.HomeOnly, ",") != ".devin/rules/" {
		t.Fatalf("the folders kept in the home are %v", p.HomeOnly)
	}
	if err := ApplyPersonal(p, filepath.Join(t.TempDir(), "backup")); err != nil {
		t.Fatal(err)
	}
	skills, doctypes, flows := pt.Folders[workspaces.Skills], pt.Folders["doctypes"], pt.Folders["flows"]
	home := filepath.ToSlash(filepath.Join(pt.Packages, "acme-setup"))
	if read(t, filepath.Join(pt.Packages, "acme-setup", ".devin", "rules", "writers-brief.md")) != "Brief.\n" {
		t.Fatal("the home was not unpacked")
	}
	review := read(t, filepath.Join(skills, "concept-review", "SKILL.md"))
	for _, want := range []string{
		"`" + home + "/.devin/rules/concept-conventions.md`",
		"`" + filepath.ToSlash(skills) + "/concept-hybrid/SKILL.md`",
		"Skip `.devin/` when",
		"customers/.devin/rules/x.md",
		"`" + home + "/_templates/rubrics/r.md`",
	} {
		if !strings.Contains(review, want) {
			t.Fatalf("missing %q in\n%s", want, review)
		}
	}
	doctype := read(t, filepath.Join(doctypes, "concept", "doctype.json"))
	if !strings.Contains(doctype, `"rubric":"`+home+`/_templates/rubrics/r.md"`) || !strings.Contains(doctype, `"template":"template.md"`) {
		t.Fatal(doctype)
	}
	flow := read(t, filepath.Join(flows, "concept.flow.md"))
	if !strings.Contains(flow, "`mine:change-a-passage.flow.md`") || !strings.Contains(flow, "`workspace:someone-else.flow.md`") ||
		!strings.Contains(flow, filepath.ToSlash(flows)+"/concept.flow.md") {
		t.Fatal(flow)
	}
	if m, err := InstalledPersonal(pt, "acme-setup"); err != nil || m == nil || m.Version != "16" {
		t.Fatalf("%v %v", m, err)
	}
	if names := InstalledPersonalNames(pt); len(names) != 1 || names[0] != "acme-setup" {
		t.Fatalf("installed %v", names)
	}
}

// A package with a kind this build does not know is installed, not refused:
// its files stay in the package's home, the plan says so, nothing of the
// kind is copied to the person's folders, and references to them lead into
// the home. Refusing it would gain nothing, since those files touch nothing
// of the person's, and a build that registers the kind copies them at the
// next update.
func TestAKindThisBuildDoesNotKnowStaysInTheHome(t *testing.T) {
	pt := personalTargets(t)
	files := map[string]string{
		".devin/skills/report/SKILL.md":       "Use `.devin/widgets/chart/widget.json`.\n",
		".devin/widgets/chart/widget.json":    `{"id":"chart"}`,
		".devin/widgets/chart/template.md":    "# Chart\n",
		".devin/rules/style.md":               "Style.\n",
		".devin/setup-notes-not-a-folder.txt": "A file directly in .devin is no kind.\n",
	}
	p, err := PlanPersonal(bundle(t, "1", files, nil), pt, "1.19.0")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(p.HomeOnly, ",") != ".devin/rules/,.devin/widgets/" {
		t.Fatalf("the plan does not say what stays in the home: %v", p.HomeOnly)
	}
	if p.CopyCounts()[Add] != 1 {
		t.Fatalf("only the skill is copied: %+v", p.Copies)
	}
	if err := ApplyPersonal(p, filepath.Join(t.TempDir(), "b")); err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(pt.Packages, "acme-setup")
	if read(t, filepath.Join(home, ".devin", "widgets", "chart", "widget.json")) != `{"id":"chart"}` {
		t.Fatal("the unknown kind's files are not in the home")
	}
	skill := read(t, filepath.Join(pt.Folders[workspaces.Skills], "report", "SKILL.md"))
	if !strings.Contains(skill, filepath.ToSlash(home)+"/.devin/widgets/chart/widget.json") {
		t.Fatalf("the reference does not lead into the home: %s", skill)
	}
	for _, dir := range pt.Folders {
		if _, err := os.Stat(filepath.Join(dir, "chart")); err == nil {
			t.Fatalf("the unknown kind was copied into %s", dir)
		}
	}
}

// An update replaces the copies, backs up a copy the person changed, and
// removes the copies of files the new version dropped.
func TestAPersonalUpdateReplacesAndRetiresCopies(t *testing.T) {
	pt := personalTargets(t)
	first, err := PlanPersonal(bundle(t, "16", personalFiles, nil), pt, "1.19.0")
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyPersonal(first, filepath.Join(t.TempDir(), "b1")); err != nil {
		t.Fatal(err)
	}
	skills := pt.Folders[workspaces.Skills]
	changed := filepath.Join(skills, "concept-review", "SKILL.md")
	os.WriteFile(changed, []byte("my own words"), 0o644)
	next := map[string]string{}
	for k, v := range personalFiles {
		next[k] = v
	}
	delete(next, ".devin/skills/concept-hybrid/SKILL.md")
	next[".devin/skills/concept-review/SKILL.md"] = "---\nname: concept-review\n---\nNew.\n"
	p, err := PlanPersonal(bundle(t, "17", next, nil), pt, "1.19.0")
	if err != nil {
		t.Fatal(err)
	}
	actions := map[string]CopyAction{}
	for _, a := range p.Copies {
		actions[a.Path] = a
	}
	if a := actions[changed]; a.Kind != Update || !a.Modified {
		t.Fatalf("%+v", a)
	}
	if PersonalObstacle(p) != "files of the package were changed by hand" {
		t.Fatalf("the obstacle is %q", PersonalObstacle(p))
	}
	gone := filepath.Join(skills, "concept-hybrid", "SKILL.md")
	if actions[gone].Kind != Remove {
		t.Fatalf("%+v", p.Copies)
	}
	backup := filepath.Join(t.TempDir(), "b2")
	if err := ApplyPersonal(p, backup); err != nil {
		t.Fatal(err)
	}
	if read(t, changed) != "---\nname: concept-review\n---\nNew.\n" {
		t.Fatal("the copy was not replaced")
	}
	if read(t, filepath.Join(backup, "personal", backupName(changed))) != "my own words" {
		t.Fatal("the person's change was not backed up")
	}
	if _, err := os.Stat(filepath.Join(skills, "concept-hybrid")); err == nil {
		t.Fatal("the retired skill's folder is still there")
	}
}

// Removing a personal install removes its home and every copy, backs up a
// copy the person changed, and leaves what it did not install.
func TestRemovingAPersonalInstall(t *testing.T) {
	pt := personalTargets(t)
	p, err := PlanPersonal(bundle(t, "16", personalFiles, nil), pt, "1.19.0")
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyPersonal(p, filepath.Join(t.TempDir(), "b1")); err != nil {
		t.Fatal(err)
	}
	skills := pt.Folders[workspaces.Skills]
	mine := filepath.Join(skills, "my-own", "SKILL.md")
	os.MkdirAll(filepath.Dir(mine), 0o755)
	os.WriteFile(mine, []byte("mine"), 0o644)
	changed := filepath.Join(skills, "concept-review", "SKILL.md")
	os.WriteFile(changed, []byte("my words"), 0o644)
	r, err := PlanRemovePersonal(pt, "acme-setup")
	if err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "b2")
	if err := RemovePersonal(r, backup); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{filepath.Join(skills, "concept-review"), filepath.Join(pt.Folders["doctypes"], "concept"), filepath.Join(pt.Folders["flows"], "concept.flow.md"), filepath.Join(pt.Packages, "acme-setup")} {
		if _, err := os.Stat(gone); err == nil {
			t.Fatalf("%s is still there", gone)
		}
	}
	if read(t, mine) != "mine" || read(t, filepath.Join(backup, "personal", backupName(changed))) != "my words" {
		t.Fatal("the person's own skill was removed, or their change was not backed up")
	}
	if _, err := PlanRemovePersonal(pt, "acme-setup"); err == nil {
		t.Fatal("removing what is not installed was not refused")
	}
}

// A workspace install is removed file by file, its manifest last, leaving
// the workspace's own files.
func TestRemovingAWorkspaceInstall(t *testing.T) {
	ws := t.TempDir()
	plan, err := PlanApply(bundle(t, "16", personalFiles, nil), ws, "1.19.0")
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(plan, filepath.Join(t.TempDir(), "b1")); err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(filepath.Join(ws, "Orchard"), 0o755)
	os.WriteFile(filepath.Join(ws, "Orchard", "customer.json"), []byte("{}"), 0o644)
	r, err := PlanRemove(ws)
	if err != nil || len(r.Actions) != len(personalFiles) {
		t.Fatalf("%v %d", err, len(r.Actions))
	}
	if err := Uninstall(r, filepath.Join(t.TempDir(), "b2")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ws, ".devin")); err == nil {
		t.Fatal(".devin is still there")
	}
	if _, err := os.Stat(filepath.Join(ws, filepath.FromSlash(ManifestPath()))); err == nil {
		t.Fatal("the manifest is still there")
	}
	if read(t, filepath.Join(ws, "Orchard", "customer.json")) != "{}" {
		t.Fatal("the customer's file was touched")
	}
}

// A skill folder or flow that is in the way and was not written by the
// install is never overwritten.
func TestAPersonalInstallRefusesWhatItDidNotWrite(t *testing.T) {
	pt := personalTargets(t)
	skills, flows := pt.Folders[workspaces.Skills], pt.Folders["flows"]
	os.MkdirAll(filepath.Join(skills, "concept-review"), 0o755)
	os.WriteFile(filepath.Join(skills, "concept-review", "notes.md"), []byte("mine"), 0o644)
	os.MkdirAll(flows, 0o755)
	os.WriteFile(filepath.Join(flows, "concept.flow.md"), []byte("my flow"), 0o644)
	p, err := PlanPersonal(bundle(t, "16", personalFiles, nil), pt, "1.19.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Conflicts) != 2 {
		t.Fatalf("%v", p.Conflicts)
	}
	if o := PersonalObstacle(p); !strings.Contains(o, "concept-review") || !strings.Contains(o, "in the way") {
		t.Fatalf("the obstacle is %q", o)
	}
	if err := ApplyPersonal(p, filepath.Join(t.TempDir(), "b")); err == nil {
		t.Fatal("a plan with conflicts was applied")
	}
	if read(t, filepath.Join(flows, "concept.flow.md")) != "my flow" {
		t.Fatal("the person's flow was overwritten")
	}
}

// A copy is in a kind's folder when it is in it or under it; a name that
// only begins with two dots is inside, and the parent is not.
func TestRootOf(t *testing.T) {
	pt := personalTargets(t)
	root := pt.Folders[workspaces.Skills]
	cases := map[string]string{
		filepath.Join(root, "a", "SKILL.md"):   root,
		filepath.Join(root, "..notes"):         root,
		filepath.Join(root, "..", "elsewhere"): "",
		filepath.Dir(root):                     "",
	}
	for target, want := range cases {
		if got := rootOf(target, pt); got != want {
			t.Errorf("rootOf(%q) = %q, want %q", target, got, want)
		}
	}
}
