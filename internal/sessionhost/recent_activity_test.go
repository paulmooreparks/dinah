package sessionhost

import (
	"path/filepath"
	"sync"
	"testing"
)

func TestRecentActivityWithoutProviderTimestamp(t *testing.T) {
	var mu sync.Mutex
	var runtime Runtime
	ws := t.TempDir()
	runtime.PutConversationLocked("old", NewConversation(Identity{ID: "old", Cwd: ws, UpdatedAt: "2026-10-01T00:00:00Z"}, InitialActivity{}, false, false))
	runtime.PutConversationLocked("active", NewConversation(Identity{ID: "active", Cwd: ws}, InitialActivity{}, false, false))
	host := SessionLifecycleHost{
		ListProviders: func() []string { return nil },
		RecordedActivity: func(id string) string {
			if id == "active" {
				return "2026-11-06T15:00:00+08:00"
			}
			return ""
		},
		FilterAncestors: func(in []Info, _ string) []Info { return in },
	}
	entries, err := NewSessionLifecycle(&mu, &runtime, host).List(filepath.Clean(ws))
	if err != nil || len(entries) != 2 || entries[0].ID != "active" {
		t.Fatalf("order: %+v, %v", entries, err)
	}
	c := runtime.ConversationLocked("active")
	at := entries[0].UpdatedAt
	c.UpdateDisplay(nil, &at)
	for _, stale := range []string{"", "invalid", "2026-10-01T00:00:00Z"} {
		c.UpdateDisplay(nil, &stale)
	}
	if c.UpdatedAt() != at {
		t.Fatal("stale provider metadata replaced activity")
	}
}
