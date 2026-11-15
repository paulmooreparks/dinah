package main

import (
	"context"
	"errors"
	"testing"

	"studio/internal/acp"
)

type listedTestConnection struct {
	*testProviderConnection
	rows      []acp.SessionInfo
	deletes   int
	deleteErr error
}

func (c *listedTestConnection) Call(ctx context.Context, method string, params, out any) error {
	switch method {
	case "session/list":
		return acp.Result(acp.ListSessionsResult{Sessions: c.rows}, out)
	case "session/delete":
		c.deletes++
		if params.(acp.DeleteSessionParams).SessionID != c.rows[0].SessionID {
			return errors.New("wrong native identity")
		}
		return c.deleteErr
	default:
		return c.testProviderConnection.Call(ctx, method, params, out)
	}
}

func TestNativeListedChatCanBeBinnedRestoredAndDeleted(t *testing.T) {
	for _, provider := range []string{"devin", "claude"} {
		for _, outcome := range []string{"supported", "unsupported", "failed"} {
			t.Run(provider+"/"+outcome, func(t *testing.T) {
				a, base := providerTestApp(t)
				ws := t.TempDir()
				id := provider + ":invented-native-only"
				conn := &listedTestConnection{testProviderConnection: base, rows: []acp.SessionInfo{{SessionID: id, Cwd: ws, Title: "Recorded provider title"}}}
				var init acp.InitializeResult
				if err := acp.Result(map[string]any{"agentCapabilities": map[string]any{"sessionCapabilities": map[string]any{"list": map[string]any{}, "delete": map[string]any{}}}}, &init); err != nil {
					t.Fatal(err)
				}
				if outcome == "unsupported" {
					init.AgentCapabilities.SessionCapabilities.Delete = nil
				}
				if outcome == "failed" {
					conn.deleteErr = errors.New("invented deletion failure")
				}
				a.settings.DefaultProvider = provider
				a.runtime.PutProviderLocked(provider, newTestProvider(conn, init))
				// A second open chat retains the shared provider through binning.
				control := newSessionState(&sessionSeed{ID: provider + ":control", Provider: provider, Cwd: ws}, sessionInitial{Open: true})
				a.runtime.PutConversationLocked(control.ID(), control)
				panel, err := a.SessionsAt(ws)
				if err != nil || len(panel) != 2 {
					t.Fatalf("panel %+v %v", panel, err)
				}
				if a.settings.SessionTitles[id] != "" || a.settings.SessionProviders[id] != "" {
					t.Fatal("fixture is no longer native-only")
				}
				saved, err := loadSettings()
				if err != nil || saved.ListedChats[id].Title != "Recorded provider title" {
					t.Fatalf("native observation did not survive settings reload: %+v %v", saved.ListedChats, err)
				}
				listing, err := a.ListChatCleanup()
				if err != nil {
					t.Fatal(err)
				}
				found := false
				for _, row := range listing.Rows {
					if row.SessionID == id {
						found = row.Eligible && row.Title == "Recorded provider title" && row.Provider == provider && row.Workspace == ws && !row.MessageCountKnown
					}
				}
				if !found {
					t.Fatalf("native cleanup row absent or wrong: %+v", listing.Rows)
				}
				if err := a.RemoveChat(id); err != nil {
					t.Fatal(err)
				}
				if conn.deletes != 0 {
					t.Fatal("binning deleted native history")
				}
				conn.rows[0].Title = "Late native title"
				panel, err = a.SessionsAt(ws)
				if err != nil || len(panel) != 1 {
					t.Fatalf("bin leaked into panel %+v %v", panel, err)
				}
				if a.settings.ListedChats[id].Title != "Recorded provider title" {
					t.Fatal("refresh changed a reserved chat")
				}
				conn.rows[0].Title = "Recorded provider title"
				listing, err = a.ListChatCleanup()
				if err != nil || len(listing.Removed) != 1 || listing.Removed[0].Title != "Recorded provider title" {
					t.Fatalf("bin %+v %v", listing, err)
				}
				if result := a.RestoreRemovedChat(id); !result.OK {
					t.Fatalf("restore %+v", result)
				}
				panel, err = a.SessionsAt(ws)
				if err != nil || len(panel) != 2 {
					t.Fatalf("restore panel %+v %v", panel, err)
				}
				if err := a.RemoveChat(id); err != nil {
					t.Fatal(err)
				}
				result := a.DeleteRemovedChat(id)
				wantCalls := 1
				if outcome == "unsupported" {
					wantCalls = 0
				}
				if !result.OK || result.ProviderCopyRemains != (outcome != "supported") || conn.deletes != wantCalls {
					t.Fatalf("final %+v calls %d", result, conn.deletes)
				}
				if _, exists := a.settings.ListedChats[id]; exists {
					t.Fatal("final deletion retained cached title")
				}
				panel, err = a.SessionsAt(ws)
				if err != nil || len(panel) != 1 {
					t.Fatalf("tombstone leaked %+v %v", panel, err)
				}
			})
		}
	}
}

func TestNativeListingKeepsOverridesAndRefusesConflictingScope(t *testing.T) {
	a, _ := providerTestApp(t)
	ws := t.TempDir()
	rows := []acp.SessionInfo{{SessionID: "native", Cwd: ws, Title: "Provider title"}}
	if err := a.recordListedSessions("devin", ws, rows); err != nil {
		t.Fatal(err)
	}
	a.settings.SessionTitles = map[string]string{"native": "Person's title"}
	if err := a.recordListedSessions("claude", ws, rows); err == nil {
		t.Fatal("cross-program identity accepted")
	}
	listing, err := a.ListChatCleanup()
	if err != nil || len(listing.Rows) != 1 || listing.Rows[0].Title != "Person's title" {
		t.Fatalf("override %+v %v", listing, err)
	}
	if a.settings.ListedChats["native"].Title != "Provider title" {
		t.Fatal("override replaced provider metadata")
	}
	rows[0].Title = "dinah-applet-v1:invented-private-context"
	if err := a.recordListedSessions("devin", ws, rows); err != nil {
		t.Fatal(err)
	}
	if a.settings.ListedChats["native"].Title != "Provider title" {
		t.Fatal("unsafe title replaced retained title")
	}
}
