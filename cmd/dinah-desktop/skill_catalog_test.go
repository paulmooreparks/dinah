package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/applet"
)

func TestSkillCatalogScopesIdentityAndRevision(t *testing.T) {
	previous := configOverride
	configOverride = t.TempDir()
	t.Cleanup(func() { configOverride = previous })
	write := func(root, body string) string {
		dir := filepath.Join(root, "same-name")
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		file := filepath.Join(dir, "SKILL.md")
		if err := os.WriteFile(file, []byte("---\ndescription: "+body+"\nargument-hint: '<TITLE>'\n---\n# "+body), 0600); err != nil {
			t.Fatal(err)
		}
		return file
	}
	root := t.TempDir()
	other := t.TempDir()
	file := write(filepath.Join(root, ".devin", "skills"), "Workspace instructions")
	write(filepath.Join(other, ".devin", "skills"), "Other instructions")
	write(filepath.Join(configDir(), "home", "skills"), "Personal instructions")
	service, _ := resourceTestService(t, WorkspaceSource{ID: "source", Kind: "local-folder", Name: "Files", Path: root})
	a := service.app
	a.settings.WorkspaceRecords[0].ExecutionPath = root
	a.settings.WorkspaceRecords = append(a.settings.WorkspaceRecords, WorkspaceRecord{ID: "other", Name: "Other", ExecutionPath: other, Sources: []WorkspaceSource{{ID: "other-source", Kind: "local-folder", Name: "Files", Path: other}}})
	catalog := a.SkillCatalog()
	if len(catalog.Problems) > 0 || len(catalog.Skills) != 3 {
		t.Fatalf("catalog: %+v", catalog)
	}
	ids := map[string]bool{}
	var selected CatalogSkill
	for _, skill := range catalog.Skills {
		if ids[skill.ID] {
			t.Fatal("duplicate ID")
		}
		ids[skill.ID] = true
		if skill.WorkspaceID == "resource-workspace" {
			selected = skill
		}
	}
	scoped := a.skillCatalog("resource-workspace", false)
	if len(scoped.Skills) != 1 || scoped.Skills[0].ID != selected.ID {
		t.Fatalf("scope leak: %+v", scoped)
	}
	bundle, err := a.skillsAppletBundle()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := applet.New(bundle); err != nil {
		t.Fatal(err)
	}
	listed, err := bundle.Handlers["dinah.skills/list@1"](builtinTestContext(root), "{}")
	if err != nil || strings.Contains(listed.OutputJSON, "Personal instructions") || strings.Contains(listed.OutputJSON, "Other instructions") || strings.Contains(listed.OutputJSON, root) {
		t.Fatalf("action scope: %s %v", listed.OutputJSON, err)
	}
	for _, skill := range catalog.Skills {
		input, _ := json.Marshal(map[string]string{"id": skill.ID})
		_, readErr := bundle.Handlers["dinah.skills/read@1"](builtinTestContext(root), string(input))
		if (readErr == nil) != (skill.ID == selected.ID) {
			t.Fatalf("wrong read scope for %s: %v", skill.Origin, readErr)
		}
	}
	a.settings.WorkspaceRecords[0].Name = "Renamed"
	pack, err := a.ReadSkill(selected.ID)
	if err != nil || !strings.Contains(pack.Skill.Origin, "Renamed") {
		t.Fatalf("rename: %+v %v", pack, err)
	}
	if _, err = a.PrepareSkill(selected.ID, selected.Revision, other, "/same-name Test"); err == nil {
		t.Fatal("cross-workspace preparation accepted")
	}
	prepared, err := a.PrepareSkill(selected.ID, selected.Revision, root, "/same-name Test")
	if err != nil || !strings.Contains(prepared, "Workspace instructions") {
		t.Fatalf("prepare: %s %v", prepared, err)
	}
	if err = os.WriteFile(file, []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = a.PrepareSkill(selected.ID, selected.Revision, root, "/same-name Test"); err == nil {
		t.Fatal("stale revision accepted")
	}
	a.settings.WorkspaceRecords[0].Sources[0].ID = "replacement"
	if _, err = a.ReadSkill(selected.ID); err == nil {
		t.Fatal("replaced source redirected retained skill")
	}
}

func TestSkillMetadataYAML(t *testing.T) {
	description, hint, err := skillMetadata("---\ndescription: >-\n  Read the evidence\n  and prepare a review.\nargument-hint: '<TITLE>'\n---\nInstructions")
	if err != nil || description != "Read the evidence and prepare a review." || hint != "<TITLE>" {
		t.Fatalf("%q %q %v", description, hint, err)
	}
	if _, _, err = skillMetadata("---\ndescription: [broken\n---"); err == nil {
		t.Fatal("invalid YAML accepted")
	}
}

func TestSkillCatalogRejectsPackageSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "SKILL.md"), []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	collection := filepath.Join(root, ".devin", "skills")
	if err := os.MkdirAll(collection, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(collection, "escape")); err != nil {
		t.Skip("symlinks unavailable", err)
	}
	service, _ := resourceTestService(t, WorkspaceSource{ID: "source", Kind: "local-folder", Name: "Files", Path: root})
	if got := service.app.skillCatalog("resource-workspace", false); len(got.Skills) != 0 {
		t.Fatalf("symlink was cataloged: %+v", got)
	}
}
