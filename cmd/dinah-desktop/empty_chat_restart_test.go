package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"studio/internal/acp"
)

// This peer models the ACP provider-owned store. It proves the host's
// creation/list/load boundary, not the persistence of a vendor executable.
type emptyChatPeer struct {
	*testProviderConnection
	file string
}

func (c *emptyChatPeer) Call(ctx context.Context, method string, params, out any) error {
	if method == "session/prompt" {
		return fmt.Errorf("an empty chat must not prompt")
	}
	if method == "session/new" {
		if err := c.testProviderConnection.Call(ctx, method, params, out); err != nil {
			return err
		}
		var p acp.NewSessionParams
		if err := acp.Decode(params, &p); err != nil {
			return err
		}
		b, err := json.Marshal(acp.SessionInfo{SessionID: out.(*acp.NewSessionResult).SessionID, Cwd: p.Cwd})
		if err != nil {
			return err
		}
		return os.WriteFile(c.file, b, 0600)
	}
	if method == "session/list" || method == "session/load" {
		b, err := os.ReadFile(c.file)
		if err != nil {
			return err
		}
		var saved acp.SessionInfo
		if err := json.Unmarshal(b, &saved); err != nil {
			return err
		}
		if method == "session/list" {
			return acp.Result(acp.ListSessionsResult{Sessions: []acp.SessionInfo{saved}}, out)
		}
		var p acp.LoadSessionParams
		if err := acp.Decode(params, &p); err != nil {
			return err
		}
		if p.SessionID != saved.SessionID || p.Cwd != saved.Cwd {
			return fmt.Errorf("wrong restored identity")
		}
	}
	return c.testProviderConnection.Call(ctx, method, params, out)
}

func TestEmptyChatHostRestartBeforeFirstTurn(t *testing.T) {
	for _, provider := range []string{"ollama", "codex", "claude", "devin"} {
		t.Run(provider, func(t *testing.T) {
			a, _ := providerTestApp(t)
			workspace, store := t.TempDir(), filepath.Join(t.TempDir(), "provider.json")
			id := provider + ":empty"
			if provider == "devin" {
				id = "empty-devin"
			}
			attach := func(app *App) *emptyChatPeer {
				c := &emptyChatPeer{testProviderConnection: newTestProviderConnection(), file: store}
				c.newIDs = []string{id}
				init := acp.InitializeResult{}
				init.AgentCapabilities.LoadSession = true
				init.AgentCapabilities.SessionCapabilities.List = &struct{}{}
				app.settings.DefaultProvider = provider
				app.runtime.PutProviderLocked(provider, newTestProvider(c, init))
				t.Cleanup(c.Close)
				return c
			}
			peer := attach(a)
			created, err := a.CreateNamedChat(workspace, provider, "", "Empty chat kept")
			if err != nil {
				t.Fatal(err)
			}
			peer.Close()
			a.queueStore.Close()
			a.closeAgentLogs()
			a.closeChatStates()
			a.closeAppletBridge()
			a.busHost().CloseHandle()
			// NewApp reads the same isolated settings, with a new runtime and peer.
			b := NewApp()
			b.connectGraceOverride = -1
			b.judging.noAuto = true
			t.Cleanup(func() {
				b.turnIO().Close(time.Second)
				b.queueStore.Close()
				b.closeAgentLogs()
				b.closeChatStates()
				b.closeFileAccess()
				b.closeAppletBridge()
				b.busHost().CloseHandle()
				b.closeJudges()
			})
			attach(b)
			listed, err := b.sessionsLifecycle().List(workspace)
			if err != nil || len(listed) != 1 || listed[0].ID != created.ID || b.settings.SessionTitles[created.ID] != "Empty chat kept" {
				t.Fatalf("restored list: %+v, %v", listed, err)
			}
			loaded, err := b.OpenSessionAt(workspace, created.ID)
			if err != nil || loaded.ID != created.ID {
				t.Fatalf("restored chat: %+v, %v", loaded, err)
			}
		})
	}
}
