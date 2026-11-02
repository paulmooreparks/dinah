package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/applet"
	"studio/internal/fileaccess"
)

func TestPathlessSkillReadsRejectPathsTraversalAndSymlinksAndAudit(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	skillDir := filepath.Join(root, ".devin", "skills", "safe")
	if err := os.MkdirAll(skillDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\ndescription: Synthetic skill\n---\nInstructions"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "SKILL.md"), []byte("---\ndescription: Outside secret\n---\nOutside instructions"), 0600); err != nil {
		t.Fatal(err)
	}
	service, _ := resourceTestService(t, WorkspaceSource{ID: "source", Kind: "local-folder", Name: "Files", Path: root})
	a := service.app
	a.settings.WorkspaceRecords[0].ExecutionPath = root
	bundle, err := a.skillsAppletBundle()
	if err != nil {
		t.Fatal(err)
	}
	registry, err := applet.New(bundle)
	if err != nil {
		t.Fatal(err)
	}
	ctx := builtinTestContext(root)
	catalog := a.skillCatalog("resource-workspace", false)
	if len(catalog.Skills) != 1 {
		t.Fatalf("catalogue: %+v", catalog)
	}
	selected := catalog.Skills[0].ID
	for _, id := range []string{filepath.Join(outside, "SKILL.md"), "../safe/SKILL.md", skillResourceID(ResourceRef{WorkspaceID: "resource-workspace", SourceID: "source", Path: "../../SKILL.md"})} {
		input, _ := json.Marshal(map[string]string{"id": id})
		if _, err = registry.Execute(ctx, "dinah.skills/read@1", string(input)); err == nil {
			t.Fatal("uncatalogued selector accepted")
		}
	}
	if _, err = registry.Execute(ctx, "dinah.skills/list@1", `{}`); err != nil {
		t.Fatal(err)
	}
	input, _ := json.Marshal(map[string]string{"id": selected})
	result, err := registry.Execute(ctx, "dinah.skills/read@1", string(input))
	if err != nil || !strings.Contains(result.OutputJSON, "Instructions") {
		t.Fatalf("catalogue read: %s %v", result.OutputJSON, err)
	}
	log, err := a.FileAccessLog(fileaccess.Filter{})
	if err != nil || len(log.Rows) != 2 || log.Rows[0].Source != "dinah.skills/read@1" || log.Rows[1].Source != "dinah.skills/list@1" || log.Rows[0].Bytes == 0 {
		t.Fatalf("pathless audit: %+v %v", log, err)
	}
	link := filepath.Join(root, ".devin", "skills", "linked")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	for _, entry := range a.skillCatalog("resource-workspace", false).Skills {
		if strings.Contains(entry.Description, "Outside") {
			t.Fatal("symlinked entry was catalogued")
		}
	}
	parent := filepath.Dir(skillDir)
	if err := os.Rename(parent, parent+"-original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, parent); err != nil {
		t.Fatal(err)
	}
	if result, err = registry.Execute(ctx, "dinah.skills/read@1", string(input)); err == nil || strings.Contains(result.OutputJSON, "Outside instructions") {
		t.Fatal("symlinked catalogue root redirected a read")
	}
}

func TestPathlessHistoryRejectsSelectorsAndSymlinkAndAudits(t *testing.T) {
	a, _ := providerTestApp(t)
	workspace := t.TempDir()
	a.remember(workspace, "", "send", "synthetic prompt", nil)
	bundle, err := a.chatAppletBundle()
	if err != nil {
		t.Fatal(err)
	}
	registry, err := applet.New(bundle)
	if err != nil {
		t.Fatal(err)
	}
	ctx := builtinTestContext(workspace)
	for _, input := range []string{`{"path":"C:/secret"}`, `{"sessionId":"../escape"}`, `{"limit":"../escape"}`} {
		if _, err = registry.Execute(ctx, chatHistoryAction, input); err == nil {
			t.Fatal("history accepted a path selector")
		}
	}
	result, err := registry.Execute(ctx, chatHistoryAction, `{}`)
	if err != nil || !strings.Contains(result.OutputJSON, "synthetic prompt") {
		t.Fatalf("history: %s %v", result.OutputJSON, err)
	}
	log, err := a.FileAccessLog(fileaccess.Filter{})
	if err != nil || len(log.Rows) != 1 || log.Rows[0].Source != chatHistoryAction {
		t.Fatal("history read not audited", err)
	}
	target := filepath.Join(t.TempDir(), "elsewhere.jsonl")
	entry, _ := json.Marshal(HistoryEntry{Workspace: workspace, Text: "outside synthetic history"})
	if err := os.WriteFile(target, append(entry, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(historyPath()); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, historyPath()); err != nil {
		t.Fatal(err)
	}
	if result, err = registry.Execute(ctx, chatHistoryAction, `{}`); err == nil || strings.Contains(result.OutputJSON, "outside synthetic history") {
		t.Fatal("history followed a symbolic link")
	}
}
