package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChatSkillCatalogProviderAndPersonal(t *testing.T) {
	a, c := providerTestApp(t)
	ws := t.TempDir()
	write := func(root, name, body string) string {
		t.Helper()
		path := filepath.Join(root, name, "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	write(personalSkillsDir(), "personal-test", "---\ndescription: Personal\n---\nPersonal instructions")
	path := write(t.TempDir(), "codex-test", "Codex instructions")
	c.skillResult = map[string]any{"data": []any{map[string]any{"cwd": ws, "skills": []any{
		map[string]any{"name": "codex-test", "description": "Codex test", "path": path, "enabled": true},
		map[string]any{"name": "disabled", "path": path, "enabled": false},
	}, "errors": []any{}}}}
	result := a.ChatSkillCatalog(ws, "codex")
	if len(result.Problems) != 0 || len(result.Skills) != 2 {
		t.Fatalf("catalogue: %+v", result)
	}
	var native CatalogSkill
	for _, skill := range result.Skills {
		if skill.Name == "codex-test" {
			native = skill
		}
	}
	if native.ID == "" {
		t.Fatal("Codex skill absent")
	}
	prompt, err := a.PrepareChatSkill(native.ID, native.Revision, ws, "codex", "/codex-test Work")
	if err != nil || !strings.Contains(prompt, "Codex instructions") || !strings.Contains(prompt, path) {
		t.Fatalf("prepare: %q %v", prompt, err)
	}
	if _, err = a.PrepareChatSkill(native.ID, native.Revision, ws, "devin", "/codex-test Work"); err == nil {
		t.Fatal("cross-provider skill accepted")
	}
	if err = os.WriteFile(path, []byte("Changed instructions"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = a.PrepareChatSkill(native.ID, native.Revision, ws, "codex", "/codex-test Work"); err == nil {
		t.Fatal("stale skill accepted")
	}
	c.skillError = errors.New("discovery unavailable")
	result = a.ChatSkillCatalog(ws, "codex")
	if len(result.Skills) != 1 || len(result.Problems) != 1 {
		t.Fatalf("partial result: %+v", result)
	}
	result = a.skillCatalog("", false)
	if len(result.Skills) != 0 {
		t.Fatal("personal skill leaked to scoped applet catalogue")
	}
}
