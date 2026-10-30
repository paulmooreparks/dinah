package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/agents"
	"studio/internal/bus"
	"studio/internal/codexagent"
	"studio/internal/layout"
	"studio/internal/ollamaagent"
)

type providerCall struct {
	method string
	params any
}

type testProviderConnection struct {
	done    chan struct{}
	once    sync.Once
	calls   []providerCall
	options []acp.ConfigOption
	// closeErr is the answer to session/close.
	closeErr error
	// resumeErr is the answer to session/resume.
	resumeErr error
	// onLoad runs when session/load is called, to replay messages.
	onLoad func()
	// imageInput is the answer to the image-input request; empty is an error.
	imageInput string
	// window is what the context requests say; setAside and choice record what
	// they were asked, and each set-aside makes the window not full.
	window     ollamaagent.ContextState
	asideTurns int
	asides     int
	choice     string
	// offered is what the turns request says, and nothing is offered, as an
	// adapter that is not asked would say, when it is nil. asideRequests are the
	// set-aside requests as they were made, and refuseChosen makes a request that
	// lists turns fail, as the adapter does with a list it will not allow.
	offered       *ollamaagent.AsideTurns
	asideRequests []any
	refuseChosen  bool
	// newIDs are the IDs session/new gives, in turn; the last is kept. newErr makes
	// session/new fail, and onNew runs when it is called, to change things while a
	// provider is starting.
	newIDs []string
	newErr error
	onNew  func()
}

func newTestProviderConnection() *testProviderConnection {
	return &testProviderConnection{done: make(chan struct{}), options: []acp.ConfigOption{{ID: "model", Category: "model", Type: "select", CurrentValue: json.RawMessage(`"initial"`),
		Options: json.RawMessage(`[{"value":"initial","name":"Initial"},{"value":"codex-default","name":"Codex"},{"value":"devin-default","name":"Devin"},{"value":"changed","name":"Changed"}]`)}}}
}

func (c *testProviderConnection) Call(_ context.Context, method string, params any, result any) error {
	c.calls = append(c.calls, providerCall{method, params})
	switch method {
	case "session/new":
		if c.newErr != nil {
			return c.newErr
		}
		if c.onNew != nil {
			c.onNew()
		}
		// A new session starts with a window that is not full.
		c.window.Full, c.window.Used = false, 0
		id := "codex:created"
		if len(c.newIDs) > 0 {
			id = c.newIDs[0]
			if len(c.newIDs) > 1 {
				c.newIDs = c.newIDs[1:]
			}
		}
		*result.(*acp.NewSessionResult) = acp.NewSessionResult{SessionID: id, ConfigOptions: c.options}
	case "session/load":
		if c.onLoad != nil {
			c.onLoad()
		}
		*result.(*acp.LoadSessionResult) = acp.LoadSessionResult{ConfigOptions: c.options}
	case ollamaagent.ContextMethod:
		*result.(*ollamaagent.ContextState) = c.window
	case ollamaagent.TurnsMethod:
		if c.offered == nil {
			return fmt.Errorf("unexpected provider call %s", method)
		}
		*result.(*ollamaagent.AsideTurns) = *c.offered
	case ollamaagent.SetAsideMethod:
		c.asideRequests = append(c.asideRequests, params)
		if m, ok := params.(map[string]any); ok && c.refuseChosen && m["turns"] != nil {
			return fmt.Errorf("those turns cannot be set aside")
		}
		c.asides++
		c.window.Full, c.window.Used = false, 100
		*result.(*ollamaagent.SetAsideResult) = ollamaagent.SetAsideResult{Turns: c.asideTurns}
	case ollamaagent.ChoiceMethod:
		c.choice = params.(map[string]string)["choice"]
		c.window.Choice = c.choice
	case ollamaagent.ImageInputMethod:
		if c.imageInput == "" {
			return fmt.Errorf("unexpected provider call %s", method)
		}
		*result.(*ollamaagent.ImageInputResult) = ollamaagent.ImageInputResult{ImageInput: c.imageInput}
	case "session/resume":
		if c.resumeErr != nil {
			return c.resumeErr
		}
		*result.(*acp.LoadSessionResult) = acp.LoadSessionResult{ConfigOptions: c.options}
	case "session/set_config_option":
		p := params.(acp.SetConfigOptionParams)
		options := append([]acp.ConfigOption(nil), c.options...)
		options[0].CurrentValue, _ = json.Marshal(p.Value)
		c.options = options
		*result.(*acp.ConfigOptionsResult) = acp.ConfigOptionsResult{ConfigOptions: options}
	case "session/prompt":
		*result.(*acp.PromptResult) = acp.PromptResult{StopReason: "end_turn"}
	case "session/close":
		return c.closeErr
	default:
		return fmt.Errorf("unexpected provider call %s", method)
	}
	return nil
}
func (c *testProviderConnection) Notify(method string, params any) error {
	c.calls = append(c.calls, providerCall{method, params})
	return nil
}
func (c *testProviderConnection) Done() <-chan struct{}                     { return c.done }
func (c *testProviderConnection) Err() error                                { return nil }
func (c *testProviderConnection) Close()                                    { c.once.Do(func() { close(c.done) }) }
func (c *testProviderConnection) RegisterTraceSecret(string)                {}
func (c *testProviderConnection) RegisterTraceRedactor(func(string) string) {}

func providerTestApp(t *testing.T) (*App, *testProviderConnection) {
	t.Helper()
	previous := configOverride
	configOverride = t.TempDir()
	t.Cleanup(func() { configOverride = previous })
	a := NewApp()
	a.connectGraceOverride = -1
	a.settings.DefaultProvider = "codex"
	c := newTestProviderConnection()
	a.runtime.PutProviderLocked("codex", newTestProvider(c, acp.InitializeResult{AgentCapabilities: acp.AgentCapabilities{LoadSession: true}}))
	t.Cleanup(a.closeAppletBridge)
	t.Cleanup(func() { a.busHost().CloseHandle() })
	a.judging.noAuto = true
	t.Cleanup(a.closeJudges)
	// The queue files are written by a goroutine, so they are finished before the
	// test's folder is removed.
	t.Cleanup(a.queueStore.Close)
	// The logs are closed too, so that a chat still running writes none after the
	// test's folder is being removed.
	t.Cleanup(a.closeAgentLogs)
	// Turns the test started are waited for before anything above is closed, so none writes into the
	// folder after the test returns (they run first, as the last cleanup registered).
	t.Cleanup(func() { a.turnIO().Wait(3 * time.Second) })
	return a, c
}

func TestStaleProviderWatcherKeepsReplacementAndCompletesAfterEvents(t *testing.T) {
	a, oldConn := providerTestApp(t)
	old := a.runtime.ProviderLocked("codex")
	newConn := newTestProviderConnection()
	replacement := newTestProvider(newConn, old.Initialization())
	s := newSessionState(&sessionSeed{ID: "replacement", Provider: "codex"}, sessionInitial{Open: true})
	a.runtime.PutProviderLocked("codex", replacement)
	a.runtime.PutConversationLocked(s.ID(), s)
	stopped := 0
	a.sink = func(name string, _ any) {
		if name == "agent:state" {
			stopped++
		}
	}
	go a.lifecycle().Watch("codex", old)
	oldConn.Close()
	old.Wait()
	if a.runtime.ProviderLocked("codex") != replacement || a.session(s.ID()) != s || !s.Open() {
		t.Fatal("stale watcher detached the replacement generation")
	}
	if stopped != 1 {
		t.Fatalf("completion preceded the old generation's stopped event: %d", stopped)
	}
	go a.lifecycle().Watch("codex", replacement)
	newConn.Close()
	replacement.Wait()
	if a.runtime.ProviderLocked("codex") != nil || a.session(s.ID()) != nil || s.Open() || stopped != 2 {
		t.Fatal("completion preceded replacement cleanup and events")
	}
}

func TestProviderJournalsDoNotCollide(t *testing.T) {
	a, _ := providerTestApp(t)
	for _, id := range []string{"codex:thread", "codex_thread", "thread"} {
		a.journal(id, id)
	}
	for _, id := range []string{"codex:thread", "codex_thread", "thread"} {
		entries := a.SessionTimes(id)
		if len(entries) != 1 || entries[0].Kind != id {
			t.Fatalf("journal for %s contains another provider's messages: %v", id, entries)
		}
	}
	if journalPath("thread") != filepath.Join(configDir(), "journal", "thread.jsonl") {
		t.Fatal("legacy Devin journal location changed")
	}
}

func TestAgentExecutableOverridePersistsAndNeverFallsBack(t *testing.T) {
	a, _ := providerTestApp(t)
	root := t.TempDir()
	auto := filepath.Join(root, "codex.exe")
	chosen := filepath.Join(root, "chosen codex.exe")
	for _, path := range []string{auto, chosen} {
		if err := os.WriteFile(path, []byte("test fixture, never execute"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", root)
	get := func(app *App) AgentProviderInfo {
		for _, provider := range app.AgentProviders() {
			if provider.ID == "codex" {
				return provider
			}
		}
		t.Fatal("Codex missing from providers")
		return AgentProviderInfo{}
	}
	if err := a.SetAgentExecutable("codex", chosen); err != nil {
		t.Fatal(err)
	}
	reopened := NewApp()
	if p := get(reopened); !layout.SamePath(p.Path, chosen) || p.ExecutableOverride != chosen || !p.Available {
		t.Fatalf("override was not retained across restart: %+v", p)
	}
	if err := os.Remove(chosen); err != nil {
		t.Fatal(err)
	}
	if p := get(reopened); p.Path != "" || p.Available || p.Status != "invalid-path" || p.ExecutableOverride != chosen {
		t.Fatalf("missing override silently fell back: %+v", p)
	}
	if err := reopened.SetAgentExecutable("codex", ""); err != nil {
		t.Fatal(err)
	}
	if p := get(NewApp()); !layout.SamePath(p.Path, auto) || p.ExecutableOverride != "" {
		t.Fatalf("automatic discovery was not restored: %+v", p)
	}
	for _, path := range []string{"relative.exe", root, filepath.Join(root, "missing.exe")} {
		if err := a.SetAgentExecutable("codex", path); err == nil {
			t.Fatalf("accepted invalid executable %q", path)
		}
	}
	if err := a.SetAgentExecutable("unknown", auto); err == nil {
		t.Fatal("accepted unknown provider")
	}
}

func checkManagedAppletServer(t *testing.T, a *App, servers []any) {
	t.Helper()
	if len(servers) != 2 {
		t.Fatalf("expected the managed applet server and the chat's bus server, got %v", servers)
	}
	server, ok := servers[0].(map[string]any)
	if !ok || server["name"] != "dinah-applets" {
		t.Fatalf("incorrect managed applet server: %v", servers)
	}
	command, _ := server["command"].(string)
	args, _ := server["args"].([]string)
	if !filepath.IsAbs(command) || len(args) != 3 || args[0] != "--applet-mcp" || args[1] != "--state-dir" || args[2] != configDir() {
		t.Fatalf("incorrect helper command: %q %v", command, args)
	}
	checkBusServer(t, a, servers[1])
}

// checkBusServer checks a chat's bus server: this program as a helper,
// serving a session the bus knows (bus_chats.go).
func checkBusServer(t *testing.T, a *App, s any) {
	t.Helper()
	server, ok := s.(map[string]any)
	if !ok || server["name"] != "dinah-bus" {
		t.Fatalf("incorrect bus server: %v", s)
	}
	command, _ := server["command"].(string)
	args, _ := server["args"].([]string)
	if !filepath.IsAbs(command) || len(args) != 5 || args[0] != "--bus-mcp" || args[1] != "--state-dir" || args[2] != configDir() || args[3] != "--session" {
		t.Fatalf("incorrect bus helper command: %q %v", command, args)
	}
	err := a.busHost().With(func(b *bus.Bus) error {
		_, err := b.Session(context.Background(), args[4])
		return err
	})
	if err != nil {
		t.Fatalf("the chat's bus session is not on the bus: %v", err)
	}
}

func TestCodexSessionRoutingAndManagedApplets(t *testing.T) {
	a, c := providerTestApp(t)
	ws := t.TempDir()
	a.settings.Defaults = map[string]json.RawMessage{"model": json.RawMessage(`"devin-default"`)}
	a.settings.ProviderDefaults = map[string]map[string]json.RawMessage{"codex": {"model": json.RawMessage(`"codex-default"`)}}
	info, err := a.NewSessionAt(ws)
	if err != nil {
		t.Fatal(err)
	}
	if info.Provider != "codex" || info.ID != "codex:created" || a.runtime.ProviderLocked("devin") != nil {
		t.Fatalf("wrong session provider: %+v", info)
	}
	checkManagedAppletServer(t, a, c.calls[0].params.(codexagent.SessionParams).McpServers)
	if got := string(a.SessionOptions(info.ID)[0].CurrentValue); got != `"codex-default"` {
		t.Fatalf("Codex received wrong default: %s", got)
	}
	if len(a.settings.Catalog) != 0 || len(a.settings.ProviderCatalogs["codex"]) == 0 {
		t.Fatal("Codex catalog was stored in Devin settings")
	}
	if _, err := a.SetSessionOption(info.ID, "model", json.RawMessage(`"changed"`)); err != nil {
		t.Fatal(err)
	}
	s := a.session(info.ID)
	a.turnIO().Run(s, promptPointer("Hello", []acp.ContentBlock{{Type: "text", Text: "Hello"}}))
	if err := a.Cancel(info.ID); err != nil {
		t.Fatal(err)
	}
	foundPrompt, foundCancel := false, false
	for _, call := range c.calls {
		switch call.method {
		case "session/prompt":
			p := call.params.(acp.PromptParams)
			foundPrompt = p.SessionID == info.ID && len(p.Prompt) == 2 && strings.Contains(p.Prompt[1].Text, appletContextPrefix)
		case "session/cancel":
			foundCancel = call.params.(acp.CancelParams).SessionID == info.ID
		}
	}
	if !foundPrompt || !foundCancel {
		t.Fatalf("provider routing missing prompt or cancellation: %v", c.calls)
	}
	if _, err := os.Stat(filepath.Join(ws, ".devin")); !os.IsNotExist(err) {
		t.Fatalf("Codex session created Devin configuration: %v", err)
	}
	a.runtime.RemoveConversationLocked(info.ID)
	if _, err := a.OpenSessionAt(ws, info.ID); err != nil {
		t.Fatal(err)
	}
	foundLoad := false
	for _, call := range c.calls {
		if call.method == "session/load" {
			p := call.params.(codexagent.SessionParams)
			checkManagedAppletServer(t, a, p.McpServers)
			foundLoad = p.SessionID == info.ID && p.Cwd == ws
		}
	}
	if !foundLoad || a.session(info.ID).Provider() != "codex" {
		t.Fatal("saved Codex session did not reopen through Codex")
	}
	if _, err := os.Stat(filepath.Join(ws, ".devin")); !os.IsNotExist(err) {
		t.Fatalf("reopening Codex created Devin configuration: %v", err)
	}
}

func TestProviderDefaultsAreIndependent(t *testing.T) {
	a, conn := providerTestApp(t)
	a.settings.Catalog = conn.options
	a.settings.ProviderCatalogs = map[string][]acp.ConfigOption{"codex": conn.options}
	a.settings.Defaults = map[string]json.RawMessage{"model": json.RawMessage(`"devin-default"`)}
	if _, err := a.SetDefault("model", json.RawMessage(`"codex-default"`)); err != nil {
		t.Fatal(err)
	}
	if string(a.settings.Defaults["model"]) != `"devin-default"` || string(a.settings.ProviderDefaults["codex"]["model"]) != `"codex-default"` {
		t.Fatal("Codex changed Devin's defaults")
	}
	if err := a.SetDefaultProvider("devin"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SetDefault("model", json.RawMessage(`"changed"`)); err != nil {
		t.Fatal(err)
	}
	if string(a.settings.ProviderDefaults["codex"]["model"]) != `"codex-default"` {
		t.Fatal("Devin changed Codex defaults")
	}
}

func TestProviderDisconnectPreservesOtherSessionsAndChannels(t *testing.T) {
	a, c := providerTestApp(t)
	ws := t.TempDir()
	if _, err := a.NewSessionAt(ws); err != nil {
		t.Fatal(err)
	}
	devin := newSessionState(&sessionSeed{ID: "devin-existing", Cwd: ws}, sessionInitial{Open: true})
	a.runtime.PutConversationLocked(devin.ID(), devin)
	other := newTestProviderConnection()
	channel, _, err := a.newAppletChannel(other, ws)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.bindAppletChannel(channel, devin.ID()); err != nil {
		t.Fatal(err)
	}
	p := a.runtime.ProviderLocked("codex")
	go a.lifecycle().Watch("codex", p)
	c.Close()
	select {
	case <-p.Finished():
	case <-time.After(3 * time.Second):
		t.Fatal("provider cleanup did not finish")
	}
	if a.session("codex:created") != nil || a.session(devin.ID()) != devin || !devin.Info().Open {
		t.Fatal("disconnect cleanup affected another provider")
	}
	if _, err := a.appletPromptContext("codex:created"); err == nil {
		t.Fatal("disconnected provider retained its applet channel")
	}
	if _, err := a.appletPromptContext(devin.ID()); err != nil {
		t.Fatalf("another provider lost its applet channel: %v", err)
	}
}

func TestDiscoverAgentPathSkipsWorkspaceAndRelativeDirectories(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	ws, safe := filepath.Join(root, "workspace"), filepath.Join(root, "installed")
	for _, dir := range []string{root, filepath.Join(root, "relative-dir"), ws, safe} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "test-agent.exe"), []byte("fixture"), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", strings.Join([]string{".", "relative-dir", ws, safe}, string(os.PathListSeparator)))
	if got := agents.FindExecutable("test-agent", []string{ws}); !layout.SamePath(got, filepath.Join(safe, "test-agent.exe")) {
		t.Fatalf("discovery selected unsafe executable: %q", got)
	}
	t.Setenv("PATH", strings.Join([]string{".", "relative-dir", ws}, string(os.PathListSeparator)))
	if got := agents.FindExecutable("test-agent", []string{ws}); got != "" {
		t.Fatalf("discovery should find no safe executable, got %q", got)
	}
	t.Run("symlink", func(t *testing.T) {
		alias := filepath.Join(root, "alias")
		if err := os.Symlink(ws, alias); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		t.Setenv("PATH", alias)
		if got := agents.FindExecutable("test-agent", []string{ws}); got != "" {
			t.Fatalf("discovery followed workspace symlink: %q", got)
		}
	})
}
