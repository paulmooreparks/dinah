package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A skill is edited as a draft of its SKILL.md: the form changes one
// front-matter line, the checks reach the draft's preview, and Copy to My
// Skills and Rename work on the package.
func TestSkillEditing(t *testing.T) {
	previous := configOverride
	configOverride = t.TempDir()
	t.Cleanup(func() { configOverride = previous })
	root := t.TempDir()
	dir := filepath.Join(root, ".devin", "skills", "review-notes")
	os.MkdirAll(filepath.Join(dir, "references"), 0o755)
	os.WriteFile(filepath.Join(dir, "references", "checklist.md"), []byte("# Checklist\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: review-notes\ndescription: Review the notes.\nargument-hint: '<TITLE>'\n---\n\nRead `references/checklist.md` and `references/missing.md`.\n"), 0o644)
	service, _ := resourceTestService(t, WorkspaceSource{ID: "source", Kind: "local-folder", Name: "Files", Path: root})
	a := service.app
	a.settings.WorkspaceRecords[0].ExecutionPath = root

	var skill CatalogSkill
	for _, s := range a.SkillCatalog().Skills {
		if s.Name == "review-notes" {
			skill = s
		}
	}
	if skill.ID == "" {
		t.Fatal("the skill is not in the catalog")
	}
	v, err := a.OpenSkillDraft(skill.ID)
	if err != nil {
		t.Fatal(err)
	}
	fields, err := a.SkillFields(v.ID)
	if err != nil || fields.Folder != "review-notes" || len(fields.Fields) != 3 || fields.Fields[2].Key != "argument-hint" || fields.Fields[2].Value != "<TITLE>" {
		t.Fatalf("%+v %v", fields, err)
	}
	want := map[string]bool{"references/missing.md is not in the package, though its folder references is": false}
	for _, p := range fields.Problems {
		if _, ok := want[p.Message]; ok {
			want[p.Message] = true
		}
	}
	for m, found := range want {
		if !found {
			t.Errorf("no problem %q in %+v", m, fields.Problems)
		}
	}
	preview, err := a.PreviewDraft(v.ID)
	if err != nil || len(preview.Problems) == 0 || !strings.Contains(preview.Problems[len(preview.Problems)-1].Message, "references/missing.md") {
		t.Fatalf("the draft preview has no skill problems: %+v %v", preview.Problems, err)
	}

	updated, err := a.SetSkillField(v.ID, v.Revision, "argument-hint", "<TITLE> [Area]")
	if err != nil || !strings.Contains(updated.Text, "argument-hint: '<TITLE> [Area]'\n") || !strings.Contains(updated.Text, "description: Review the notes.\n") {
		t.Fatalf("%q %v", updated.Text, err)
	}
	if _, err := a.SetSkillField(v.ID, updated.Revision, "name", "other"); err == nil {
		t.Fatal("the name was set as a field")
	}
	if _, err := a.SetSkillField(v.ID, v.Revision, "model", "sonnet"); err == nil {
		t.Fatal("a stale revision was accepted")
	}
	if _, err := a.RenameSkill(skill.ID, "renamed"); err == nil {
		t.Fatal("a skill with unsaved changes was renamed")
	}
	if _, err := a.SaveDraft(v.ID, updated.Revision); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "SKILL.md")); !strings.Contains(string(b), "[Area]") {
		t.Fatal("the save did not reach the file")
	}

	copied, err := a.CopySkillToMine(skill.ID)
	if err != nil || copied.Scope != "mine" || copied.Name != "review-notes" {
		t.Fatalf("%+v %v", copied, err)
	}
	if b, err := os.ReadFile(filepath.Join(configDir(), "home", "skills", "review-notes", "references", "checklist.md")); err != nil || string(b) != "# Checklist\n" {
		t.Fatalf("the supporting file was not copied: %v", err)
	}
	if _, err := a.CopySkillToMine(skill.ID); err == nil {
		t.Fatal("a second copy replaced the first")
	}

	renamed, err := a.RenameSkill(copied.ID, "notes-review")
	if err != nil || renamed.Name != "notes-review" || renamed.Scope != "mine" {
		t.Fatalf("%+v %v", renamed, err)
	}
	b, err := os.ReadFile(filepath.Join(configDir(), "home", "skills", "notes-review", "SKILL.md"))
	if err != nil || !strings.Contains(string(b), "name: notes-review\n") {
		t.Fatalf("%q %v", b, err)
	}
	for _, bad := range []string{"", "../x", ".hidden", "a/b"} {
		if _, err := a.RenameSkill(renamed.ID, bad); err == nil {
			t.Errorf("RenameSkill(%q) was accepted", bad)
		}
	}
}
