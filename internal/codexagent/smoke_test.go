package codexagent

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"studio/internal/acp"
)

// This opt-in compatibility check makes no inference requests and uses an
// empty Codex home. It never reads or writes the person's Codex settings.
func TestInstalledProtocolSmoke(t *testing.T) {
	if os.Getenv("DINAH_CODEX_SMOKE") != "1" {
		t.Skip("set DINAH_CODEX_SMOKE=1 for the installed binary check")
	}
	for _, mode := range []string{"writes", "approve"} {
		t.Run(mode, func(t *testing.T) { installedProtocolSmoke(t, mode) })
	}
}

func installedProtocolSmoke(t *testing.T, mode string) {
	path, err := exec.LookPath("codex")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CODEX_HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("CODEX_API_KEY", "")
	c, err := Start(path, []string{"app-server", "-c", "cli_auth_credentials_store=\"file\"",
		"-c", `mcp_servers.dinah-applets.command="unstarted-test-helper"`,
		"-c", `mcp_servers.dinah-applets.default_tools_approval_mode="` + mode + `"`,
		"-c", `mcp_servers.dinah-applets.tool_timeout_sec=30`}, t.TempDir(), &testHandler{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var init acp.InitializeResult
	if err = c.Call(ctx, "initialize", acp.InitializeParams{ProtocolVersion: 1}, &init); err != nil {
		t.Fatal(err)
	}
	var config struct {
		Config map[string]any `json:"config"`
	}
	if err = c.Conn.Call(ctx, "config/read", map[string]any{"cwd": t.TempDir(), "includeLayers": false}, &config); err != nil {
		t.Fatal(err)
	}
	servers, ok := config.Config["mcp_servers"].(map[string]any)
	if !ok {
		t.Fatal("installed Codex omitted MCP configuration")
	}
	managed, ok := servers["dinah-applets"].(map[string]any)
	if !ok || managed["default_tools_approval_mode"] != mode || managed["tool_timeout_sec"] != float64(30) {
		t.Fatalf("installed Codex did not retain managed MCP policy: %#v", managed)
	}
	options, err := c.Catalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(options) == 0 {
		t.Fatal("empty catalog")
	}
	t.Logf("Installed Codex initialized; configuration read and catalog returned %d controls", len(options))
}
