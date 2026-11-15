package main

import (
	"encoding/json"
	"strings"
	"testing"

	"studio/internal/sessionhost"
)

func TestLogicalQuerySeamsPreserveLocalEvidenceAndHolds(t *testing.T) {
	s := Settings{SessionTitles: map[string]string{"old": "Earlier", "head": "Current"},
		SessionProviders: map[string]string{"external-history-only": "codex"},
		ChatBindings:     map[string]string{"old": "head"},
		RemovedChats:     map[string]chatRemoval{"old": {Stage: "a-future-phase"}, "gone": {Stage: "deleted"}},
		logicalEvidence:  map[string]sessionhost.TranscriptHead{"head": {SessionID: "head", Workspace: "invented-workspace"}, "transcript-only": {SessionID: "transcript-only"}, "deleted-transcript": {SessionID: "deleted-transcript", Deleted: true}}}
	refs := strings.Join(logicalChatReferencesFromSettings(s), ",")
	if refs != "head,old,transcript-only" {
		t.Fatal("wrong local inventory", refs)
	}
	metadata, err := logicalChatMetadataFromSettings(s, "old")
	if err != nil || metadata.Title != "Current" || metadata.Workspace != "invented-workspace" || metadata.Lifecycle != "reserved" {
		t.Fatalf("metadata %+v %v", metadata, err)
	}
	copy, err := s.clone()
	if err != nil {
		t.Fatal(err)
	}
	copy.logicalEvidence["head"] = sessionhost.TranscriptHead{Workspace: "changed-copy"}
	if s.logicalEvidence["head"].Workspace != "invented-workspace" {
		t.Fatal("query clone shares mutable evidence")
	}
	data, err := json.Marshal(s)
	if err != nil || strings.Contains(string(data), "invented-workspace") {
		t.Fatal("query evidence was serialized", err)
	}
}

func TestOrgChartIncludesTranscriptOnlyChatWithoutHistoryDiscovery(t *testing.T) {
	starts := orgChartForbidProviderStarts(t)
	a, connection := providerTestApp(t)
	ws := t.TempDir()
	id := "codex:transcript-only"
	cleanupFixtureChat(t, a, id, "codex", ws, "")
	if err := a.updateSettings(func(s *Settings) error { delete(s.SessionTitles, id); delete(s.SessionProviders, id); return nil }); err != nil {
		t.Fatal(err)
	}
	view, err := a.OrgChart()
	if err != nil || len(view.Chats) != 1 || view.Chats[0].ChatID != id || view.Chats[0].Workspace != ws {
		t.Fatalf("transcript inventory %+v %v", view, err)
	}
	if len(connection.calls) != 0 || starts.Load() != 0 {
		t.Fatal("local transcript discovery started a provider")
	}
}
