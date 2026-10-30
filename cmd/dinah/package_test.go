package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/setup"
)

// testPackage writes a package's ZIP in a build's shape, with its
// manifest where the profile keeps it.
func testPackage(t *testing.T, version string, files map[string]string) string {
	t.Helper()
	sums := map[string]string{}
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for p, c := range files {
		s := sha256.Sum256([]byte(c))
		sums[p] = hex.EncodeToString(s[:])
		w, _ := z.Create(p)
		w.Write([]byte(c))
	}
	m, _ := json.Marshal(map[string]any{"schema": 1, "name": "acme-setup", "version": version, "built": "2026-10-13", "min_tool": "", "roots": []string{".devin/"}, "files": sums})
	w, _ := z.Create(setup.ManifestPath())
	w.Write(m)
	z.Close()
	zp := filepath.Join(t.TempDir(), "hybrid-setup-v"+version+".zip")
	os.WriteFile(zp, buf.Bytes(), 0o644)
	return zp
}

// package install --personal installs a package under the state folder and
// copies its skill to the person's skills folder, which --state-dir keeps
// inside that folder. Document types and flows, which this build does not
// copy, stay in the package's folder and are said to. package status lists
// it, and package remove --personal takes it out again.
func TestPackageInstallPersonal(t *testing.T) {
	state := t.TempDir()
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	zp := testPackage(t, "16", map[string]string{
		".devin/skills/concept-review/SKILL.md": "Read `.devin/rules/style.md`.\n",
		".devin/doctypes/concept/doctype.json":  `{"id":"concept"}`,
		".devin/flows/concept.flow.md":          "Call `workspace:concept.flow.md`.\n",
		".devin/rules/style.md":                 "Style.\n",
	})
	var out bytes.Buffer
	if err := packageCommand([]string{"install", zp, "--personal", "--state-dir", state}, &out); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	skill, err := os.ReadFile(filepath.Join(state, "devin-skills", "concept-review", "SKILL.md"))
	if err != nil || !strings.Contains(string(skill), filepath.ToSlash(filepath.Join(state, "packages", "acme-setup", ".devin", "rules", "style.md"))) {
		t.Fatalf("skill %q %v", skill, err)
	}
	if _, err := os.Stat(filepath.Join(state, "packages", "acme-setup", ".devin", "flows", "concept.flow.md")); err != nil {
		t.Fatal("the flow is not in the package's folder")
	}
	for _, want := range []string{"INSTALLED: acme-setup for you", "KEPT IN THE PACKAGE'S FOLDER: .devin/doctypes/", "KEPT IN THE PACKAGE'S FOLDER: .devin/flows/"} {
		if !strings.Contains(out.String(), want) {
			t.Fatalf("missing %q in\n%s", want, out.String())
		}
	}

	out.Reset()
	if err := packageCommand([]string{"status", "--state-dir", state, "--workspace", t.TempDir()}, &out); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil || len(rows) != 1 || rows[0]["target"] != "me" || rows[0]["version"] != "16" || rows[0]["files"] != float64(4) {
		t.Fatalf("status %s %v", out.String(), err)
	}

	out.Reset()
	if err := packageCommand([]string{"remove", "--personal", "--state-dir", state, "--backup-dir", filepath.Join(t.TempDir(), "b")}, &out); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if _, err := os.Stat(filepath.Join(state, "devin-skills", "concept-review")); err == nil {
		t.Fatal("the copied skill is still there")
	}
	if err := packageCommand([]string{"remove", "--personal", "--state-dir", state}, &out); err == nil || !strings.Contains(err.Error(), "no package is installed") {
		t.Fatalf("removing nothing: %v", err)
	}
}

// package install puts a package in a workspace, --dry-run writes nothing,
// and package remove takes it out, leaving the workspace's own files.
func TestPackageInstallInAWorkspace(t *testing.T) {
	ws := t.TempDir()
	os.WriteFile(filepath.Join(ws, "notes.md"), []byte("mine"), 0o644)
	zp := testPackage(t, "3", map[string]string{".devin/skills/review/SKILL.md": "Review.\n"})
	var out bytes.Buffer
	if err := packageCommand([]string{"install", "--workspace", ws, "--dry-run", zp}, &out); err != nil || !strings.Contains(out.String(), "DRY RUN") {
		t.Fatalf("%v %s", err, out.String())
	}
	if _, err := os.Stat(filepath.Join(ws, ".devin")); err == nil {
		t.Fatal("a dry run wrote files")
	}
	out.Reset()
	if err := packageCommand([]string{"install", zp, "--workspace", ws, "--backup-dir", filepath.Join(t.TempDir(), "b"), "--json"}, &out); err != nil {
		t.Fatal(err)
	}
	var plan map[string]any
	if err := json.Unmarshal(out.Bytes(), &plan); err != nil || plan["to"] != "3" || plan["dry_run"] != false {
		t.Fatalf("%s %v", out.String(), err)
	}
	if m, err := setup.Installed(ws); err != nil || m == nil || m.Version != "3" {
		t.Fatalf("%+v %v", m, err)
	}
	out.Reset()
	if err := packageCommand([]string{"remove", "--workspace", ws, "--backup-dir", filepath.Join(t.TempDir(), "b2")}, &out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(ws, ".devin", "skills", "review")); err == nil {
		t.Fatal("the skill is still in the workspace")
	}
	if b, _ := os.ReadFile(filepath.Join(ws, "notes.md")); string(b) != "mine" {
		t.Fatal("the workspace's own file was touched")
	}
	if err := packageCommand([]string{"install"}, &out); err == nil || !strings.Contains(err.Error(), "usage") {
		t.Fatalf("no ZIP: %v", err)
	}
}
