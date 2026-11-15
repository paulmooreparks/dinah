//go:build quality_chat_cleanup && chatcleanup_acceptance

package main

import (
	"strings"
	"testing"
)

// This additional assertion follows the allocation protection in message 8821.
func TestQualityCleanupAllocationRefusal(t *testing.T) {
	f := qualityCleanupNew(t).(*qualityHostCleanup)
	id := "codex:quality-allocating"
	f.Seed(qualityCleanupSeed{ID: id, Queued: 1})
	before := f.Observe(id)
	if err := f.a.transferChat(id, "", "claude", before.BusIdentity, "quality-allocation", "allocating", false); err != nil {
		t.Fatal(err)
	}
	defer f.a.finishTransfer(id)
	rs := f.Act("remove", id)
	if len(rs) != 1 || rs[0].OK || rs[0].Status != "refused" || rs[0].Error == "" {
		t.Fatalf("allocation was not refused: %+v", rs)
	}
	after := f.Observe(id)
	if after.Removed || !after.Active || after.Content != before.Content || after.BusIdentity != before.BusIdentity {
		t.Fatal("allocation refusal changed retained chat")
	}
	qualityNoStarts(t, before, after)
}

// This probe checks delivery while another conversation retains the shared process.
func TestQualityCleanupLoadedDeliveryRefusal(t *testing.T) {
	f := qualityCleanupNew(t).(*qualityHostCleanup)
	id := "fixture:cannot-unload"
	f.Seed(qualityCleanupSeed{ID: id, Loaded: true, CanUnload: false, ProviderDelete: "unsupported"})
	before := f.Observe(id)
	if !before.Loaded || !before.Open {
		t.Fatal("fixture was not loaded")
	}
	rs := f.Act("remove", id)
	if len(rs) != 1 || !rs[0].OK {
		t.Fatal(rs)
	}
	rows, err := f.a.ListChatCleanup()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range rows.Removed {
		if row.SessionID == id {
			found = strings.Contains(strings.ToLower(row.RuntimeNote), "still loaded")
		}
	}
	if !found {
		t.Fatal("removed inventory omitted still-loaded text")
	}
	if !f.Observe(id).Loaded {
		t.Fatal("shared provider stopped before delivery probe")
	}
	if f.Send("direct", id) == nil {
		t.Fatal("binned loaded recipient accepted delivery")
	}
	qualityNoStarts(t, before, f.Observe(id))
	f.Restart()
	after := f.Observe(id)
	qualityBinned(t, after)
	qualityNoStarts(t, before, after)
	if after.Loaded {
		t.Fatal("binned chat reloaded at startup")
	}
}
