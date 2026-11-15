package main

import (
	"os"
	"path/filepath"
	"studio/internal/sessionhost"
	"testing"
)

func TestCodexDeletionRefusesIndependentDescendant(t *testing.T) {
	s := Settings{SessionTitles: map[string]string{"codex:other": "Separate chat"},
		ChatBindings: map[string]string{"codex:root": "codex:member"}}
	if err := codexDeletionOwnership(s, nil, "codex:root", []string{"codex:member", "codex:native-child"}); err != nil {
		t.Fatal(err)
	}
	if err := codexDeletionOwnership(s, nil, "codex:root", []string{"codex:other"}); err == nil {
		t.Fatal("independent titled descendant was deleted")
	}
	if err := codexDeletionOwnership(s, map[string]bool{"codex:transcript": true}, "codex:root", []string{"codex:transcript"}); err == nil {
		t.Fatal("independent transcript descendant was deleted")
	}
	s.BusSessions = map[string]string{"codex:bus-only": "recipient"}
	if err := codexDeletionOwnership(s, nil, "codex:root", []string{"codex:bus-only"}); err == nil {
		t.Fatal("independent bus descendant was deleted")
	}
	s.ChatBindings["codex:member"] = "codex:root"
	if err := codexDeletionOwnership(s, nil, "codex:root", nil); err == nil {
		t.Fatal("damaged ownership authorized deletion")
	}
}

func TestCodexDeletionRefusesInterimOwnershipEvidence(t *testing.T) {
	for name, s := range map[string]Settings{
		"binding only":       {ChatBindings: map[string]string{"codex:other": "codex:child"}},
		"live transfer only": {ChatTransfers: map[string]chatTransfer{"codex:other": {Source: "codex:other", Target: "codex:child", Stage: "allocated"}}},
		"history only":       {ChatTransferHistory: []chatTransfer{{Source: "codex:other", Target: "codex:child", Stage: "rejected"}}},
		"separate recipient": {ChatBindings: map[string]string{"codex:root": "codex:child"}, BusSessions: map[string]string{"codex:root": "first", "codex:child": "second"}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := codexDeletionOwnership(s, nil, "codex:root", []string{"codex:child"}); err == nil {
				t.Fatal("native cascade was authorized across independent ownership")
			}
		})
	}
}

func TestCodexDeletionAllowsOwnedAbandonedCandidate(t *testing.T) {
	s := Settings{ChatTransfers: map[string]chatTransfer{"codex:root": {Source: "codex:root", Target: "codex:child", Stage: "rejected"}}}
	if err := codexDeletionOwnership(s, nil, "codex:root", []string{"codex:child"}); err != nil {
		t.Fatal(err)
	}
}

func TestCodexDeletionTranscriptEvidenceDoesNotNeedReadableContent(t *testing.T) {
	dir := t.TempDir()
	path := sessionhost.TranscriptPath(dir, "codex:child")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("damaged transcript"), 0600); err != nil {
		t.Fatal(err)
	}
	known, err := codexDeletionTranscriptIDs(dir, []string{"codex:child", "codex:absent"})
	if err != nil || !known["codex:child"] || known["codex:absent"] {
		t.Fatalf("transcript evidence = %v, %v", known, err)
	}
	if err := codexDeletionOwnership(Settings{}, known, "codex:root", []string{"codex:child"}); err == nil {
		t.Fatal("damaged independent transcript authorized native deletion")
	}
	if _, err := codexDeletionTranscriptIDs(string([]byte{0}), []string{"codex:child"}); err == nil {
		t.Fatal("filesystem error was treated as absent ownership")
	}
}
