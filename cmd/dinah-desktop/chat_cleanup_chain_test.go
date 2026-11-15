package main

import (
	"reflect"
	"studio/internal/sessionhost"
	"testing"
	"time"
)

func TestCleanupChainIncludesRejectedAndHistoryOnlyTargets(t *testing.T) {
	s := Settings{
		ChatBindings:  map[string]string{"codex-old": "claude-current"},
		ChatTransfers: map[string]chatTransfer{"codex-old": {Source: "codex-old", Target: "claude-current", Stage: "committed"}},
		ChatTransferHistory: []chatTransfer{
			{Source: "claude-current", Target: "devin-rejected", Stage: "rejected", Created: time.Unix(10, 0)},
			{Source: "claude-current", Target: "ollama-uncommitted", Stage: "allocated", Created: time.Unix(20, 0)},
		},
	}
	heads := map[string]*sessionhost.TranscriptHead{"claude-current": {ParentSessionID: "codex-old"}, "older": {}}
	groups := cleanupChains(s, heads, []string{"separate"})
	want := [][]string{{"codex-old", "claude-current", "devin-rejected", "ollama-uncommitted"}, {"older"}, {"separate"}}
	if !reflect.DeepEqual(groups, want) {
		t.Fatalf("chain membership/order %#v", groups)
	}
	if id := cleanupDisplayIdentity(s, groups[0]); id != "claude-current" {
		t.Fatalf("uncommitted target became display head %q", id)
	}
}

func TestCleanupChainRetainsCyclesAndAmbiguousSuccessors(t *testing.T) {
	s := Settings{ChatBindings: map[string]string{"a": "b", "b": "a"}, ChatTransferHistory: []chatTransfer{{Source: "a", Target: "c"}}}
	groups := cleanupChains(s, nil, nil)
	if len(groups) != 1 || len(groups[0]) != 3 {
		t.Fatalf("lost malformed chain members %#v", groups)
	}
	if got := cleanupChains(s, nil, nil); !reflect.DeepEqual(got, groups) {
		t.Fatal("unstable cycle order")
	}
}
