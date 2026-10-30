package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"studio/internal/claudeagent"
)

func claudeEntry(t *testing.T, a *App) AgentProviderInfo {
	t.Helper()
	for _, p := range a.AgentProviders() {
		if p.ID == "claude" {
			return p
		}
	}
	t.Fatal("Claude is not among the agents")
	return AgentProviderInfo{}
}

// fakeNode is a stand-in Node.js the person chose, with npm beside it; it is
// never run.
func fakeNode(t *testing.T, withNPM bool) string {
	t.Helper()
	dir := t.TempDir()
	node, npm := filepath.Join(dir, "node"), filepath.Join(dir, "npm")
	if runtime.GOOS == "windows" {
		node, npm = node+".exe", npm+".cmd"
	}
	os.WriteFile(node, []byte("stand-in"), 0o700)
	if withNPM {
		os.WriteFile(npm, []byte("stand-in"), 0o700)
	}
	return node
}

func fakeAdapter(t *testing.T) {
	t.Helper()
	entry := claudeagent.Entry(configDir())
	os.MkdirAll(filepath.Dir(entry), 0o700)
	os.WriteFile(entry, []byte("// stand-in"), 0o600)
	os.WriteFile(filepath.Join(filepath.Dir(filepath.Dir(entry)), "package.json"), []byte(`{"version":"`+claudeagent.Version+`"}`), 0o600)
	t.Cleanup(func() { os.RemoveAll(filepath.Join(configDir(), "claude-agent-acp")) })
}

// Settings > Agents says what Claude still needs: Node.js, then the adapter,
// which npm beside Node.js installs; then it is ready, on the person's own
// sign-in unless they stored an API key.
func TestClaudeSetupSaysWhatIsMissing(t *testing.T) {
	a, _ := providerTestApp(t)
	if err := a.SetAgentExecutable("claude", fakeNode(t, false)); err != nil {
		t.Fatal(err)
	}
	p := claudeEntry(t, a)
	if p.Available || p.Status != "adapter-missing" || p.Claude.CanInstall || !strings.Contains(p.Detail, "npm") {
		t.Fatalf("Node.js without npm showed %+v %+v", p, p.Claude)
	}
	if err := a.SetAgentExecutable("claude", fakeNode(t, true)); err != nil {
		t.Fatal(err)
	}
	p = claudeEntry(t, a)
	if p.Available || !p.Claude.CanInstall || p.Claude.AdapterVersion != claudeagent.Version || !strings.Contains(p.Detail, "Install the Claude adapter") {
		t.Fatalf("Node.js without the adapter showed %+v %+v", p, p.Claude)
	}
	fakeAdapter(t)
	p = claudeEntry(t, a)
	if !p.Available || p.Status != "detected" || p.Claude.APIKey || !strings.Contains(p.Detail, "your own Claude Code sign-in") {
		t.Fatalf("a ready Claude showed %+v %+v", p, p.Claude)
	}
	if p, err := a.SetClaudeAPIKey("  sk-test  "); err != nil || !p.Claude.APIKey || !strings.Contains(p.Detail, "billed to the API key") {
		t.Fatalf("storing a key showed %+v %v", p, err)
	}
	if key, _ := claudeKeys.get(); key != "sk-test" {
		t.Fatalf("the key was stored as %q", key)
	}
	if p, err := a.SetClaudeAPIKey(""); err != nil || p.Claude.APIKey {
		t.Fatalf("removing the key showed %+v %v", p, err)
	}
	if err := a.AgentSignIn("claude"); err == nil || !strings.Contains(err.Error(), "/login") {
		t.Fatalf("Sign In for Claude said %v", err)
	}
}

func TestClaudeSessionsAreRoutedToClaude(t *testing.T) {
	if providerForSession("claude:03840e85") != "claude" || providerForSession("03840e85") != "devin" {
		t.Fatal("a Claude session was not told from a Devin one")
	}
	if !supportedPreferenceProvider("claude") || prepareProviderApplets("claude", t.TempDir()) != nil {
		t.Fatal("Claude is not a supported provider")
	}
}
