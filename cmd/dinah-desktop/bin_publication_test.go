package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Hold only timer scheduling; explicit production publication still runs. This
// exposes a late pass deterministically without sleeping through its delay.
func holdInventoryPublicationTimers(t *testing.T, a *App) {
	t.Helper()
	a.chatStates.run.Lock()
	a.chatStates.mu.Lock()
	if a.chatStates.timer != nil {
		a.chatStates.timer.Stop()
	}
	a.chatStates.timer = time.NewTimer(time.Hour)
	a.chatStates.timer.Stop()
	a.chatStates.mu.Unlock()
	a.chatStates.run.Unlock()
	a.teamRun.Lock()
	a.teamMu.Lock()
	if a.teamTimer != nil {
		a.teamTimer.Stop()
	}
	a.teamTimer = time.NewTimer(time.Hour)
	a.teamTimer.Stop()
	a.teamMu.Unlock()
	a.teamRun.Unlock()
}

func TestBinningMemberReturnsItsRemainingChatsPublishedState(t *testing.T) {
	a, lead, member, _, _, _ := teamApp(t)
	a.sendTeamStates()
	a.sendChatStates()
	if lead.Info().Team == nil {
		t.Fatal("fixture has no published team counts")
	}
	holdInventoryPublicationTimers(t, a)
	if err := a.RemoveChat(member.ID()); err != nil {
		t.Fatal(err)
	}
	before := lead.InfoVersioned()
	// The scheduled passes must have nothing left that changes SavedChats'
	// guarded runtime identities once binning has reported success.
	a.sendTeamStates()
	a.sendChatStates()
	after := lead.InfoVersioned()
	if inventoryRuntimeHash(map[string]SessionInfo{lead.ID(): before}) != inventoryRuntimeHash(map[string]SessionInfo{lead.ID(): after}) {
		left, _ := json.Marshal(before)
		right, _ := json.Marshal(after)
		t.Fatalf("binning returned before dependent state publication: before=%s after=%s", left, right)
	}
	if before.Team == nil || before.Team.Idle != 0 {
		t.Fatal("the binned member was still counted as idle", before.Team)
	}
	listing, err := a.ListChatCleanup()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range listing.Removed {
		if row.SessionID == member.ID() {
			found = true
		}
	}
	if !found {
		t.Fatal("binned chat missing from recycle bin")
	}
}

func TestSavedChatsAcceptsTheScheduledPublicationAfterBinning(t *testing.T) {
	a, _, member, _, _, _ := teamApp(t)
	a.sendTeamStates()
	a.sendChatStates()
	holdInventoryPublicationTimers(t, a)
	if err := a.RemoveChat(member.ID()); err != nil {
		t.Fatal(err)
	}
	_, err := a.savedChatsAtBoundary(func() { a.sendTeamStates(); a.sendChatStates() })
	if err != nil {
		t.Fatal("the pending removal publication invalidated the next inventory read", err)
	}
}

func TestSavedChatsStillRefusesARealIdentityChangeDuringItsRead(t *testing.T) {
	a, _, _ := orgActionsFixture(t)
	_, err := a.savedChatsAtBoundary(func() {
		if updateErr := a.updateSettings(func(s *Settings) error { s.SessionTitles["codex:a"] = "Changed during this read"; return nil }); updateErr != nil {
			t.Fatal(updateErr)
		}
	})
	if err == nil || !strings.Contains(err.Error(), "Saved chat identities changed") {
		t.Fatal("the consistency guard no longer refuses changed identities", err)
	}
}
