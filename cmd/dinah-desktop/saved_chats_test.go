package main

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/acp"
	"studio/internal/sessionhost"
)

func TestSavedChatsUsesLocalCanonicalInventoryWithoutProviderContact(t *testing.T) {
	starts := orgChartForbidProviderStarts(t)
	a, conn := providerTestApp(t)
	ws, other := t.TempDir(), t.TempDir()
	seedOrgChartFixture(t, a, ws)
	cleanupFixtureChat(t, a, "codex:other", "codex", other, "")
	cleanupFixtureChat(t, a, "codex:hidden", "codex", ws, "")
	if err := a.updateSettings(func(s *Settings) error {
		s.Hidden = []string{"codex:hidden"}
		// A recorded provider alone is neither an import nor ownership evidence.
		s.SessionProviders["codex:external"] = "codex"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	live := newSessionState(&sessionSeed{ID: "codex:runtime", Provider: "codex", Cwd: ws, Title: "Already open"}, sessionInitial{Open: true})
	a.runtime.PutConversationLocked(live.ID(), live)
	before := len(conn.calls.Snapshot())
	view, err := a.SavedChats()
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]SavedChatInfo{}
	for _, row := range view.Chats {
		rows[row.ChatID] = row
		if row.Resolution != "resolved" || row.Reason != "" {
			t.Fatalf("local metadata not resolved: %+v", row)
		}
	}
	if len(rows) != 4 || rows["codex:old"].ID != "claude:head" || rows["codex:old"].Title != "Current title" || rows["codex:fork"].ID != "codex:fork" || rows["codex:other"].Cwd != other || !rows["codex:runtime"].Open {
		t.Fatalf("wrong canonical inventory: %+v", view)
	}
	if starts.Load() != 0 || len(conn.calls.Snapshot()) != before || a.runtime.ConversationLocked("claude:head") != nil {
		t.Fatal("passive inventory started, contacted or loaded a provider")
	}
	again, err := a.SavedChats()
	if err != nil || again.Generation != view.Generation {
		t.Fatalf("unchanged saved inventory changed generation: %v", err)
	}
	if err := a.updateSettings(func(s *Settings) error { s.SessionTitles["claude:head"] = "Renamed locally"; return nil }); err != nil {
		t.Fatal(err)
	}
	again, err = a.SavedChats()
	if err != nil || again.Generation == view.Generation {
		t.Fatalf("local rename did not change generation: %v", err)
	}
}

func TestSavedChatsExcludesWholeReservedChainAndKeepsRepairRows(t *testing.T) {
	a, conn := providerTestApp(t)
	ws := t.TempDir()
	seedOrgChartFixture(t, a, ws)
	if err := a.updateSettings(func(s *Settings) error {
		s.RemovedChats["codex:old"] = chatRemoval{Stage: "unknown-future-phase"}
		s.SessionTitles["missing"] = "Missing metadata"
		s.SessionTitles["ambiguous"] = "Ambiguous ownership"
		s.ChatBindings["ambiguous"] = "candidate-a"
		s.ChatTransferHistory = []chatTransfer{{Source: "ambiguous", Target: "candidate-b", Stage: "committed"}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	view, err := a.SavedChats()
	if err != nil {
		t.Fatal(err)
	}
	repairs := 0
	for _, row := range view.Chats {
		if row.ChatID == "codex:old" || row.ID == "claude:head" || row.ID == "codex:bin" {
			t.Fatal("reserved chain entered ordinary inventory")
		}
		if row.ChatID == "codex:fork" {
			continue
		}
		if row.Resolution != "repair" || row.Reason == "" || row.Cwd != "" {
			t.Fatalf("repair identity disappeared or gained invented metadata: %+v", row)
		}
		repairs++
	}
	if repairs == 0 || len(conn.calls.Snapshot()) != 0 {
		t.Fatal("repair inventory missing or queried native state")
	}
}

func TestDeclaredProvidersDoesNotProbeAvailability(t *testing.T) {
	a, conn := providerTestApp(t)
	previous := providerModules
	providerModules = map[string]providerModule{}
	t.Cleanup(func() { providerModules = previous })
	probes := 0
	for id, module := range previous {
		module.Info = func(*App, *AgentProviderInfo, []string) { probes++ }
		providerModules[id] = module
	}
	rows := a.DeclaredProviders()
	if len(rows) != len(product.Providers) || len(rows) == 0 {
		t.Fatal("product declarations missing")
	}
	for _, row := range rows {
		if row.Status != "unknown" || row.Name == "" || row.Default != (row.ID == "codex") {
			t.Fatalf("invented availability or lost default: %+v", row)
		}
		data, _ := json.Marshal(row)
		if strings.Contains(string(data), `"available"`) || strings.Contains(string(data), `"installed"`) {
			t.Fatal("passive row claims checked availability")
		}
	}
	if probes != 0 || len(conn.calls.Snapshot()) != 0 {
		t.Fatal("provider declaration performed a native or installation probe")
	}
	// The explicit scan remains a positive control for the replaced Info ports.
	_ = a.AgentProviders()
	if probes != len(product.Providers) {
		t.Fatal("explicit availability scan did not reach the counted ports")
	}
}

func TestCodexPreparationInspectsOnlyCodex(t *testing.T) {
	a, conn := providerTestApp(t)
	previous := providerModules
	providerModules = map[string]providerModule{}
	t.Cleanup(func() { providerModules = previous })
	probes := map[string]int{}
	for id, module := range previous {
		module.Info = func(_ *App, info *AgentProviderInfo, _ []string) {
			probes[info.ID]++
			info.Path = filepath.Join(t.TempDir(), "invented-program.exe")
		}
		providerModules[id] = module
	}
	start, err := codexAdapter.prepare(a)
	if err != nil || start.launch == nil || probes["codex"] != 1 || len(probes) != 1 || len(conn.calls.Snapshot()) != 0 {
		t.Fatalf("Codex preparation scanned another program or contacted a provider: %+v %v", probes, err)
	}
	module := providerModules["codex"]
	module.Info = func(_ *App, info *AgentProviderInfo, _ []string) { probes[info.ID]++ }
	providerModules["codex"] = module
	if _, err := codexAdapter.prepare(a); err == nil {
		t.Fatal("missing Codex executable did not retain preparation refusal")
	}
}

func TestSavedChatsKeepsTranscriptOnlyChatAndNativeListingExplicit(t *testing.T) {
	a, conn := providerTestApp(t)
	id, ws := "codex:local-transcript", t.TempDir()
	if err := writeTranscript(&sessionhost.Transcript{SessionID: id, Provider: "codex", Workspace: ws, Complete: true}); err != nil {
		t.Fatal(err)
	}
	view, err := a.SavedChats()
	if err != nil || len(view.Chats) != 1 || view.Chats[0].ID != id || view.Chats[0].Title != "" || len(conn.calls.Snapshot()) != 0 {
		t.Fatalf("transcript-only local record disappeared or contacted its provider: %+v %v", view, err)
	}
	initial := acp.InitializeResult{}
	initial.AgentCapabilities.SessionCapabilities.List = &struct{}{}
	setTestInitialization(a, "codex", initial)
	if _, err := a.SessionsAt(ws); err != nil {
		t.Fatal(err)
	}
	listed := false
	for _, call := range conn.calls.Snapshot() {
		listed = listed || call.method == "session/list"
	}
	if !listed {
		t.Fatal("explicit native listing did not reach the connection positive control")
	}
}

func TestSavedChatsRefusesFailedStartupWithoutProviderContact(t *testing.T) {
	a, conn := providerTestApp(t)
	a.startupErr = errors.New("invented unreadable settings")
	view, err := a.SavedChats()
	if !errors.Is(err, a.startupErr) || len(view.Chats) != 0 || len(conn.calls.Snapshot()) != 0 {
		t.Fatalf("failed startup returned a usable inventory or contacted a provider: %+v %v", view, err)
	}
}
