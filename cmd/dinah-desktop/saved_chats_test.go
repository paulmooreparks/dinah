package main

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"sort"
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

// A chat switched between programs is one row, its newest stage, in the
// Chats list, the Org Chart and Clean Up Chats alike. The saved conversations
// here are linked only by the parent each one records, as the stages of a chat
// switched before a switch was also recorded in the settings are.
func TestSavedChatsListsAChatSwitchedBetweenProgramsOnce(t *testing.T) {
	a, conn := providerTestApp(t)
	ws := t.TempDir()
	stage := func(id, provider, parent string) {
		t.Helper()
		if err := writeTranscript(&sessionhost.Transcript{SessionID: id, Provider: provider, Workspace: ws, ParentSessionID: parent, Complete: true}); err != nil {
			t.Fatal(err)
		}
		if err := a.updateSettings(func(s *Settings) error {
			if s.SessionProviders == nil {
				s.SessionProviders = map[string]string{}
			}
			if s.SessionTitles == nil {
				s.SessionTitles = map[string]string{}
			}
			s.SessionProviders[id] = provider
			s.SessionTitles[id] = "Invented " + id
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	// One chat, switched twice.
	stage("claude:first", "claude", "")
	stage("ollama:second", "ollama", "claude:first")
	stage("claude:third", "claude", "ollama:second")
	// A chat whose newest stage the person deleted: the chat is gone.
	stage("claude:deleted-first", "claude", "")
	stage("claude:deleted-newest", "claude", "claude:deleted-first")
	// A chat whose switch did not finish: its target is hidden and was never
	// handed the conversation, and the chat goes on where it was.
	stage("claude:kept", "claude", "")
	stage("ollama:unfinished", "ollama", "claude:kept")
	if err := writeTranscript(&sessionhost.Transcript{SessionID: "ollama:unfinished", Provider: "ollama", Workspace: ws, ParentSessionID: "claude:kept", Complete: true, PendingHandoff: true}); err != nil {
		t.Fatal(err)
	}
	// A chat whose newest stage an older build left marked hidden. It was
	// handed the conversation and worked in, so it is where the chat goes on.
	stage("claude:marked-first", "claude", "")
	stage("codex:marked-newest", "codex", "claude:marked-first")
	// A hidden chat that continues nothing stays hidden.
	stage("codex:hidden-alone", "codex", "")
	// A chat that was never switched.
	stage("claude:alone", "claude", "")
	// A chat an older build deleted: its saved conversation is marked deleted,
	// and its bus recipient was left in the settings.
	if err := writeTranscript(&sessionhost.Transcript{SessionID: "ollama:deleted-before", Provider: "ollama", Workspace: ws, Complete: true, Deleted: true}); err != nil {
		t.Fatal(err)
	}
	if err := a.updateSettings(func(s *Settings) error {
		s.RemovedChats = map[string]chatRemoval{"claude:deleted-newest": {Stage: "removed"}}
		s.Hidden = []string{"ollama:unfinished", "codex:marked-newest", "codex:hidden-alone"}
		s.BusSessions = map[string]string{"ollama:deleted-before": "window-left-behind"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := []string{"claude:alone", "claude:kept", "claude:third", "codex:marked-newest"}
	// Opening any stage of a chat opens the session the chat is listed as.
	for stage, listed := range map[string]string{"claude:first": "claude:third", "ollama:second": "claude:third", "claude:kept": "claude:kept", "claude:marked-first": "codex:marked-newest", "claude:alone": "claude:alone"} {
		if opened, err := a.ResolveSessionContinuation(stage); err != nil || opened != listed {
			t.Fatalf("opening %s opens %q (%v), and the chat is listed as %s", stage, opened, err, listed)
		}
	}
	view, err := a.SavedChats()
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, row := range view.Chats {
		got = append(got, row.ID)
	}
	sort.Strings(got)
	if !slices.Equal(got, want) {
		t.Fatalf("the Chats list shows %v, want %v", got, want)
	}
	chart, err := a.OrgChart()
	if err != nil {
		t.Fatal(err)
	}
	got = got[:0]
	for _, chat := range chart.Chats {
		got = append(got, chat.Head)
	}
	sort.Strings(got)
	if !slices.Equal(got, want) {
		t.Fatalf("the Org Chart shows %v, want %v", got, want)
	}
	cleanup, err := a.ListChatCleanup()
	if err != nil {
		t.Fatal(err)
	}
	got = got[:0]
	for _, row := range cleanup.Rows {
		got = append(got, row.SessionID)
	}
	sort.Strings(got)
	if !slices.Equal(got, want) {
		t.Fatalf("Clean Up Chats shows %v, want %v", got, want)
	}
	if len(conn.calls.Snapshot()) != 0 {
		t.Fatal("reading the lists contacted a program")
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
