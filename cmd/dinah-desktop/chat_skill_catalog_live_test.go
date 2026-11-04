package main

import (
	"context"
	"os"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/codexagent"
)

// This opt-in check performs discovery only; it never creates a model turn.
func TestLiveChatSkillDiscovery(t *testing.T) {
	executable := os.Getenv("DINAH_SKILL_TEST_CODEX")
	if executable == "" {
		t.Skip("set DINAH_SKILL_TEST_CODEX for discovery-only verification")
	}
	workspace := os.Getenv("DINAH_SKILL_TEST_WORKSPACE")
	if workspace == "" {
		t.Fatal("set DINAH_SKILL_TEST_WORKSPACE")
	}
	a, _ := providerTestApp(t)
	conn, err := codexagent.Start(executable, []string{"app-server"}, workspace, a.newAgentHandler(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var init acp.InitializeResult
	if err = conn.Call(ctx, "initialize", acp.InitializeParams{ProtocolVersion: 1, ClientInfo: &acp.Implementation{Name: "dinah", Version: "skills-test"}}, &init); err != nil {
		t.Fatal(err)
	}
	a.runtime.PutProviderLocked("codex", newTestProvider(conn, init))
	result := a.ChatSkillCatalog(workspace, "codex")
	count := 0
	for _, skill := range result.Skills {
		if skill.Origin == "Codex" {
			count++
		}
	}
	if count == 0 {
		t.Fatalf("no Codex skills: %v", result.Problems)
	}
	t.Logf("Discovered %d usable Codex skills; %d discovery problems", count, len(result.Problems))
	for _, problem := range result.Problems {
		t.Log(problem)
	}
}
