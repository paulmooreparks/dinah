//go:build startup_page_acceptance

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/claudeagent"
	"studio/internal/sessionhost"
)

var startupCounts = struct {
	sync.Mutex
	Values map[string]int
	Trace  []startupPageContact
}{Values: map[string]int{}}

type startupPageContact struct {
	Provider string    `json:"provider"`
	Kind     string    `json:"kind"`
	At       time.Time `json:"at"`
}

func startupCount(provider, kind string) {
	startupCounts.Lock()
	startupCounts.Values[provider+"/"+kind]++
	startupCounts.Trace = append(startupCounts.Trace, startupPageContact{Provider: provider, Kind: kind, At: time.Now().UTC()})
	startupCounts.Unlock()
}

// StartupPageTrace returns the unfiltered boundary observations without starting work.
func (a *App) StartupPageTrace() []startupPageContact {
	startupCounts.Lock()
	defer startupCounts.Unlock()
	return append([]startupPageContact{}, startupCounts.Trace...)
}

// StartupPageCounters is a diagnostic read compiled only into this tagged test binary.
func (a *App) StartupPageCounters() map[string]int {
	startupCounts.Lock()
	out := map[string]int{}
	for k, v := range startupCounts.Values {
		out[k] = v
	}
	startupCounts.Unlock()
	a.mu.Lock()
	out["host/loaded-chats"] = len(a.runtime.ConversationsLocked())
	out["host/live-providers"] = len(a.runtime.ProviderIDsLocked())
	a.mu.Unlock()
	return out
}

type startupCountedConnection struct {
	agentConnection
	provider string
}

func (c *startupCountedConnection) Call(ctx context.Context, method string, params, out any) error {
	startupCount(c.provider, method)
	return c.agentConnection.Call(ctx, method, params, out)
}

// TestStartupPageCostHost serves the actual page and App bridge without a native window.
// The test owns every program, server, workspace and credential boundary it reaches.
func TestStartupPageCostHost(t *testing.T) {
	entry := os.Getenv("DINAH_STARTUP_AUDIT_ENTRY")
	if entry == "" {
		t.Skip("only the explicit isolated audit runner starts this host")
	}
	if os.Getenv("DINAH_HOME") == "" || os.Getenv("DINAH_STARTUP_PEER_DIR") == "" {
		t.Fatal("isolated home and peer folder required")
	}
	oldConfig := configOverride
	configOverride = os.Getenv("DINAH_HOME")
	defer func() { configOverride = oldConfig }()
	pageMethods["StartupPageCounters"] = pageQuery
	defer delete(pageMethods, "StartupPageCounters")
	pageMethods["StartupPageTrace"] = pageQuery
	defer delete(pageMethods, "StartupPageTrace")
	oldKeys := claudeKeys
	claudeKeys.get = func() (string, error) {
		startupCount("claude", "credential-read")
		return "", errors.New("no fixture credential")
	}
	claudeKeys.set = func(string) error { return errors.New("credential writes forbidden") }
	claudeKeys.remove = func() error { return errors.New("credential writes forbidden") }
	defer func() { claudeKeys = oldKeys }()
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startupCount("ollama", r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/version":
			_ = json.NewEncoder(w).Encode(map[string]string{"version": "fixture-1"})
		case "/api/tags", "/api/ps":
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []any{}})
		default:
			http.Error(w, "fixture refuses unexpected request", 400)
		}
	}))
	defer ollama.Close()
	t.Setenv("OLLAMA_HOST", ollama.URL)
	for id, module := range providerModules {
		info := module.Info
		if info != nil {
			module.Info = func(a *App, p *AgentProviderInfo, ws []string) { startupCount(id, "discovery"); info(a, p, ws) }
			providerModules[id] = module
			defer func() { m := providerModules[id]; m.Info = info; providerModules[id] = m }()
		}
		if module.Adapter == nil {
			continue
		}
		adapter := module.Adapter
		original := adapter.prepare
		adapter.prepare = func(a *App) (providerStart, error) {
			startupCount(id, "prepare")
			start, err := original(a)
			if err != nil {
				return start, err
			}
			launch := start.launch
			start.launch = func(h acp.Handler) (agentConnection, error) {
				startupCount(id, "launch")
				conn, err := launch(h)
				if err != nil {
					return conn, err
				}
				// Codex exposes additional concrete adapter methods. Its native peer
				// records protocol requests without hiding that adapter behind a wrapper.
				if id == "codex" {
					return conn, nil
				}
				return &startupCountedConnection{agentConnection: conn, provider: id}, nil
			}
			return start, nil
		}
		defer func() { adapter.prepare = original }()
	}
	// Real discovery still runs, but explicit executable paths prevent automatic native discovery.
	// The Claude package is invented and executed by the peer, never by a real Node installation.
	adapterEntry := claudeagent.Entry(configDir())
	if err := os.MkdirAll(filepath.Dir(adapterEntry), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(adapterEntry, []byte("// invented adapter fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(filepath.Dir(filepath.Dir(adapterEntry)), "package.json")
	data, _ := json.Marshal(map[string]any{"version": claudeagent.Version, "engines": map[string]string{"node": ">=22"}})
	if err := os.WriteFile(manifest, data, 0600); err != nil {
		t.Fatal(err)
	}
	a := NewApp()
	if a.startupErr != nil {
		t.Fatal(a.startupErr)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a.ctx = ctx
	a.nativeAPI = &fakeNativeWindow{}
	controlID, controlProvider := os.Getenv("DINAH_STARTUP_CONTROL_ID"), os.Getenv("DINAH_STARTUP_CONTROL_PROVIDER")
	if controlID == "" {
		controlID, controlProvider = "fixture-open", "devin"
	}
	if controlProvider == "ollama" {
		dir := filepath.Join(configDir(), "ollama", "sessions")
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(map[string]any{"id": controlID, "cwd": a.settings.Workspace, "title": "Positive control", "model": "", "contextRev": 4, "messages": []any{}})
		if err := os.WriteFile(filepath.Join(dir, controlID[len("ollama:"):]+".json"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := sessionhost.WriteTranscript(configDir(), &sessionhost.Transcript{SessionID: controlID, Provider: controlProvider, Workspace: a.settings.Workspace, Complete: true}); err != nil {
		t.Fatal(err)
	}
	frontend, err := fs.Sub(assets, "frontend")
	if err != nil {
		t.Fatal(err)
	}
	shell, err := newShellServer(a, frontend, "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go shell.serve()
	defer shell.close()
	defer func() {
		for _, id := range product.Providers {
			a.lifecycle().Stop(id)
		}
	}()
	if err := os.WriteFile(entry, []byte(shell.entry()), 0600); err != nil {
		t.Fatal(err)
	}
	line := make(chan bool, 1)
	go func() { scan := bufio.NewScanner(os.Stdin); scan.Scan(); line <- true }()
	select {
	case <-line:
	case <-time.After(150 * time.Second):
		t.Fatal("audit runner did not close its fixture")
	}
}
