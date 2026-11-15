//go:build quality_chat_cleanup && chatcleanup_acceptance

package main

import (
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
