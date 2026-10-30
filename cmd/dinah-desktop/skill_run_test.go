package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"studio/internal/acp"
)

// A skill runs in a new chat of the provider and model chosen, whose first
// prompt is the skill's instructions as they are, template syntax and all.
func TestSkillRunStartsAChatWithItsInstructions(t *testing.T) {
	a, c := providerTestApp(t)
	ws := t.TempDir()
	dir := filepath.Join(ws, ".devin", "skills", "example")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(file, []byte("---\ndescription: Test\n---\nLiteral {{unknown}} instructions.\n## other\ncommand: malicious"), 0600); err != nil {
		t.Fatal(err)
	}
	a.settings.WorkspaceRecords = []WorkspaceRecord{newWorkspaceRecord(ws)}
	items := a.SkillCatalog().Skills
	if len(items) != 1 {
		t.Fatalf("%+v", items)
	}
	skill := items[0]
	if _, err := a.StartSkill(skill.ID, skill.Revision, t.TempDir(), "codex", "", "/example"); err == nil {
		t.Fatal("unregistered workspace accepted")
	}
	if _, err := a.StartSkill(skill.ID, skill.Revision, ws, "codex", "missing", "/example"); err == nil {
		t.Fatal("a model the agent does not offer was accepted")
	}
	turned := make(chan struct{}, 1)
	a.sink = func(name string, _ any) {
		if name == "agent:turn" {
			select {
			case turned <- struct{}{}:
			default:
			}
		}
	}
	chat, err := a.StartSkill(skill.ID, skill.Revision, ws, "codex", "changed", "/example Task")
	if err != nil {
		t.Fatal(err)
	}
	if chat.Workspace != ws || chat.SessionID == "" || a.session(chat.SessionID) == nil {
		t.Fatalf("the skill's chat %+v", chat)
	}
	select {
	case <-turned:
	case <-time.After(testWait):
		t.Fatal("the skill's turn did not end")
	}
	var prompt *acp.PromptParams
	model := ""
	for _, call := range c.calls {
		switch p := call.params.(type) {
		case acp.PromptParams:
			prompt = &p
		case acp.SetConfigOptionParams:
			if raw, ok := p.Value.(json.RawMessage); ok {
				model = string(raw)
			}
		}
	}
	if model != `"changed"` {
		t.Fatalf("the model chosen was not selected: %q", model)
	}
	if prompt == nil || len(prompt.Prompt) == 0 || !strings.Contains(prompt.Prompt[0].Text, "Literal {{unknown}} instructions.") {
		t.Fatalf("the skill's instructions were not the prompt: %+v", prompt)
	}
	if title := a.chatTitle(a.session(chat.SessionID)); title != "Skill: example" {
		t.Fatalf("the skill's chat is called %q", title)
	}
	if err = os.WriteFile(file, []byte("Changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = a.StartSkill(skill.ID, skill.Revision, ws, "codex", "", "/example"); err == nil {
		t.Fatal("stale skill accepted")
	}
}
