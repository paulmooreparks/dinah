package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/claudeagent"
)

// Releases on either side of the tested one.
const (
	earlierAdapter = "0.1.0"
	laterAdapter   = "999.0.0"
)

// fakeRelease makes the state folder look as if a release of the adapter
// were installed.
func fakeRelease(t *testing.T, version string) {
	t.Helper()
	entry := claudeagent.EntryOf(configDir(), version)
	if err := os.MkdirAll(filepath.Dir(entry), 0o700); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(entry, []byte("// stand-in"), 0o600)
	os.WriteFile(filepath.Join(filepath.Dir(filepath.Dir(entry)), "package.json"), []byte(`{"version":"`+version+`","engines":{"node":">=22"}}`), 0o600)
	t.Cleanup(func() { os.RemoveAll(filepath.Join(configDir(), "claude-agent-acp")) })
}

// adapterTools stands in for npm and Node.js: it says which release is npm's
// newest, "installs" a release by writing its stand-in, and counts both.
type adapterTools struct {
	newest    claudeagent.Release
	installed []string
	// fail is the error the next install ends with, and during is run while
	// an install is under way.
	fail   error
	during func()
}

// fakeAdapterTools replaces the adapter's tools for one test. The stand-in
// Node.js is version 22.
func fakeAdapterTools(t *testing.T) *adapterTools {
	t.Helper()
	tools := &adapterTools{newest: claudeagent.Release{Version: laterAdapter, Released: time.Date(2026, 11, 9, 19, 57, 16, 0, time.UTC), Node: 22}}
	before := claudeTools
	t.Cleanup(func() { claudeTools = before })
	claudeTools.latest = func(context.Context, string, string) (claudeagent.Release, error) { return tools.newest, nil }
	claudeTools.install = func(_ context.Context, _, _, version string) error {
		if tools.during != nil {
			tools.during()
		}
		if tools.fail != nil {
			return tools.fail
		}
		tools.installed = append(tools.installed, version)
		fakeRelease(t, version)
		return nil
	}
	claudeTools.checkNode = func(_ context.Context, _ string, minimum int) (string, error) {
		if minimum > 22 {
			return "v22.0.0", errors.New("Claude needs a later Node.js")
		}
		return "v22.0.0", nil
	}
	return tools
}

func adapterTestApp(t *testing.T) (*App, *adapterTools) {
	t.Helper()
	a, _ := providerTestApp(t)
	if err := a.SetAgentExecutable("claude", fakeNode(t, true)); err != nil {
		t.Fatal(err)
	}
	return a, fakeAdapterTools(t)
}

// After an update of the app that moved the tested release, Claude runs on
// the release it had while the app installs the tested one by itself, and the
// earlier release is removed only at a later start, when nothing runs it.
func TestAnUpdateThatMovedTheAdapterInstallsItByItself(t *testing.T) {
	a, tools := adapterTestApp(t)
	fakeRelease(t, earlierAdapter)
	p := claudeEntry(t, a)
	if !p.Available || p.Status != "detected" || p.Claude.AdapterInUse != earlierAdapter || p.Claude.AdapterInstalled || p.Claude.AdapterNewer ||
		!strings.Contains(p.Detail, "Install "+claudeagent.Version+" to bring it up to date") {
		t.Fatalf("with only an earlier release: %+v %+v", p, p.Claude)
	}
	tools.during = func() {
		p := claudeEntry(t, a)
		if !p.Available || !p.Claude.AdapterInstalling || !strings.Contains(p.Detail, "installs "+claudeagent.Version) {
			t.Errorf("during the install: %+v %+v", p, p.Claude)
		}
	}
	a.catchUpClaudeAdapter()
	p = claudeEntry(t, a)
	if len(tools.installed) != 1 || tools.installed[0] != claudeagent.Version || p.Claude.AdapterInUse != claudeagent.Version || !p.Claude.AdapterInstalled || p.Claude.AdapterInstalling {
		t.Fatalf("after the install: %v %+v %+v", tools.installed, p, p.Claude)
	}
	if !claudeagent.InstalledRelease(configDir(), earlierAdapter) {
		t.Fatal("the earlier release was removed while Claude could still be running it")
	}
	a.catchUpClaudeAdapter()
	if claudeagent.InstalledRelease(configDir(), earlierAdapter) || len(tools.installed) != 1 {
		t.Fatalf("at the next start: installs %v, the earlier release still there", tools.installed)
	}
}

// An install that fails leaves Claude on the release it had and says why,
// and the person can try again in Settings.
func TestAFailedAdapterInstallSaysWhyAndCanBeTriedAgain(t *testing.T) {
	a, tools := adapterTestApp(t)
	fakeRelease(t, earlierAdapter)
	tools.fail = errors.New("npm could not install the Claude adapter: exit status 1\nnpm ERR! network")
	a.catchUpClaudeAdapter()
	p := claudeEntry(t, a)
	if !p.Available || p.Claude.AdapterInUse != earlierAdapter || p.Claude.AdapterInstalling ||
		!strings.Contains(p.Detail, "could not be installed: npm could not install the Claude adapter: exit status 1. Install it here to try again.") {
		t.Fatalf("after a failed install: %+v %+v", p, p.Claude)
	}
	tools.fail = nil
	p, err := a.InstallClaudeAdapter()
	if err != nil || p.Claude.AdapterInUse != claudeagent.Version || strings.Contains(p.Detail, "could not be installed") {
		t.Fatalf("trying again: %+v %+v %v", p, p.Claude, err)
	}
}

// The app installs nothing by itself for a person who never set Claude up.
func TestNothingIsInstalledForAPersonWhoNeverSetClaudeUp(t *testing.T) {
	a, tools := adapterTestApp(t)
	a.catchUpClaudeAdapter()
	if len(tools.installed) != 0 {
		t.Fatalf("installed %v", tools.installed)
	}
	if p := claudeEntry(t, a); p.Available || p.Status != "adapter-missing" {
		t.Fatalf("with no adapter: %+v", p)
	}
}

// The person may run npm's newest release in place of the tested one, and
// return to the tested one. Only npm's newest release, and only a later one,
// can be chosen.
func TestThePersonChoosesNpmsNewestAdapterAndReturns(t *testing.T) {
	a, tools := adapterTestApp(t)
	fakeRelease(t, claudeagent.Version)
	r, err := a.NewestClaudeAdapter()
	if err != nil || r.Version != laterAdapter || !r.Newer || r.Tested != claudeagent.Version || r.Released != "2026-11-09T19:57:16Z" {
		t.Fatalf("npm's newest: %+v %v", r, err)
	}
	for _, refused := range []string{"", "latest", "0.102.1-preview.7", "../" + laterAdapter, claudeagent.Version, earlierAdapter, "998.0.0"} {
		if _, err := a.UseClaudeAdapter(refused); err == nil {
			t.Errorf("%q was taken", refused)
		}
	}
	// A release that needs a later Node.js than the person has is refused.
	tools.newest.Node = 30
	if _, err := a.UseClaudeAdapter(laterAdapter); err == nil {
		t.Fatal("a release for a later Node.js was installed")
	}
	if len(tools.installed) != 0 || a.settings.ClaudeAdapter != "" {
		t.Fatalf("after the refusals: installed %v, chose %q", tools.installed, a.settings.ClaudeAdapter)
	}
	tools.newest.Node = 22
	p, err := a.UseClaudeAdapter(laterAdapter)
	if err != nil || !p.Available || p.Claude.AdapterInUse != laterAdapter || !p.Claude.AdapterNewer || !p.Claude.AdapterInstalled ||
		!strings.Contains(p.Detail, "You chose that release, which is newer than "+claudeagent.Version) {
		t.Fatalf("choosing the newest: %+v %+v %v", p, p.Claude, err)
	}
	if a.settings.ClaudeAdapter != laterAdapter {
		t.Fatalf("the choice was saved as %q", a.settings.ClaudeAdapter)
	}
	// A start of the app keeps the chosen release and the tested one.
	a.catchUpClaudeAdapter()
	if !claudeagent.InstalledRelease(configDir(), laterAdapter) || !claudeagent.Installed(configDir()) {
		t.Fatal("a start of the app removed a release in use")
	}
	p, err = a.UseTestedClaudeAdapter()
	if err != nil || p.Claude.AdapterInUse != claudeagent.Version || p.Claude.AdapterNewer || a.settings.ClaudeAdapter != "" {
		t.Fatalf("returning to the tested release: %+v %+v %v", p, p.Claude, err)
	}
	if claudeagent.InstalledRelease(configDir(), laterAdapter) {
		t.Fatal("the release the person left is still installed")
	}
}

// A choice the tested release has caught up with is forgotten at the app's
// start, so the tested release wins from then on.
func TestAChoiceTheTestedReleaseCaughtUpWithIsForgotten(t *testing.T) {
	a, _ := adapterTestApp(t)
	fakeRelease(t, claudeagent.Version)
	if err := a.updateSettings(func(s *Settings) error { s.ClaudeAdapter = claudeagent.Version; return nil }); err != nil {
		t.Fatal(err)
	}
	a.catchUpClaudeAdapter()
	if a.settings.ClaudeAdapter != "" {
		t.Fatalf("the choice is still %q", a.settings.ClaudeAdapter)
	}
}

// The connection says which release of the adapter answered, which is where
// a problem with a release the person chose is seen.
func TestClaudesConnectionSaysItsAdapter(t *testing.T) {
	if got := claudeAdapter.readyDetail(acp.InitializeResult{AgentInfo: &acp.Implementation{Version: "0.102.0"}}); got != "Claude is connected, on the adapter 0.102.0" {
		t.Fatalf("with a release: %q", got)
	}
	for _, init := range []acp.InitializeResult{{}, {AgentInfo: &acp.Implementation{Version: "<b>x</b>"}}} {
		if got := claudeAdapter.readyDetail(init); got != "Claude is connected" {
			t.Fatalf("without a release: %q", got)
		}
	}
}
