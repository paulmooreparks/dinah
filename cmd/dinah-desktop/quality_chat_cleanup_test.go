//go:build quality_chat_cleanup

package main

import (
	"strings"
	"testing"
	"time"
)

// This adapter is deliberately unbound until the implementation handover.
// A binding must drive real host operations and observe real persisted state.
var qualityCleanupFactory func(*testing.T) qualityCleanupFixture

type qualityCleanupFixture interface {
	Seed(qualityCleanupSeed)
	Act(string, ...string) []qualityCleanupResult
	Send(string, string) error
	Restart()
	Advance(time.Duration)
	Observe(string) qualityCleanupState
	Events() []qualityCleanupEvent
	SaveUIState(map[string]string)
	ReadUIState() map[string]string
}
type qualityCleanupSeed struct {
	ID                      string
	Chain                   []string
	HistoryOnly             bool
	Parent, Relation        string // transcript ancestry; fork or workspace-move is separate ownership
	Working, Pinned, Loaded bool
	CanUnload               bool
	ProviderDelete          string // supported, unsupported, or failing
	Queued                  int
}
type qualityCleanupResult struct {
	ID, Status, Error, Text string
	OK, ProviderCopyRemains bool
	ProviderDeletionReason  string
}
type qualityCleanupState struct {
	Active, Removed, LocalCopy, ProviderCopy, Tombstone bool
	Open, Loaded, OrdinaryList, Decisions, Broadcast    bool
	Queued, Starts, Loads, Wakes                        int
	RemovedAt, ExpiresAt                                time.Time
	BusIdentity, Content, Settings                      string
}
type qualityCleanupEvent struct{ ID, Kind string }

func qualityCleanupNew(t *testing.T) qualityCleanupFixture {
	t.Helper()
	if qualityCleanupFactory == nil {
		t.Fatal("cleanup candidate binding is required; acceptance has not run")
	}
	return qualityCleanupFactory(t)
}
func qualityCleanupResultFor(t *testing.T, results []qualityCleanupResult, id, status string) qualityCleanupResult {
	t.Helper()
	for _, r := range results {
		if r.ID == id {
			if !r.OK || r.Status != status {
				t.Fatalf("%s: status=%s ok=%v error=%s", id, r.Status, r.OK, r.Error)
			}
			return r
		}
	}
	t.Fatalf("missing result for %s", id)
	return qualityCleanupResult{}
}
func qualityNoStarts(t *testing.T, before, after qualityCleanupState) {
	t.Helper()
	if after.Starts != before.Starts || after.Loads != before.Loads || after.Wakes != before.Wakes {
		t.Fatal("operation started, loaded or woke a provider")
	}
}
func qualityBinned(t *testing.T, s qualityCleanupState) {
	t.Helper()
	if !s.Removed || s.Active || s.Open || s.OrdinaryList || s.Decisions || s.Broadcast || !s.LocalCopy {
		t.Fatal("binned chat remains active or lost its retained copy")
	}
}
func TestQualityCleanupDormancy(t *testing.T) {
	f := qualityCleanupNew(t)
	id := "codex:quality-dormant"
	f.Seed(qualityCleanupSeed{ID: id, CanUnload: true, ProviderDelete: "supported", Queued: 2})
	before := f.Observe(id)
	qualityCleanupResultFor(t, f.Act("remove", id), id, "removed")
	binned := f.Observe(id)
	qualityBinned(t, binned)
	qualityNoStarts(t, before, binned)
	if binned.Queued != 2 || binned.Content != before.Content || binned.Settings != before.Settings || binned.BusIdentity != before.BusIdentity {
		t.Fatal("staging changed retained work or bus identity")
	}
	if binned.ExpiresAt.Sub(binned.RemovedAt) != 30*24*time.Hour {
		t.Fatal("retention is not thirty days")
	}
	for _, route := range []string{"direct", "group", "broadcast"} {
		if e := f.Send(route, id); route != "broadcast" && (e == nil || !strings.Contains(strings.ToLower(e.Error()), "removed")) {
			t.Fatalf("%s did not refuse the removed recipient", route)
		}
		now := f.Observe(id)
		qualityNoStarts(t, binned, now)
		if now.Queued != binned.Queued {
			t.Fatal("removed recipient gained queued work")
		}
	}
	f.Restart()
	now := f.Observe(id)
	qualityBinned(t, now)
	qualityNoStarts(t, binned, now)
	qualityCleanupResultFor(t, f.Act("restore", id), id, "restored")
	restored := f.Observe(id)
	qualityNoStarts(t, now, restored)
	if !restored.Active || restored.Removed || restored.Open || restored.Loaded || restored.Queued != 2 || restored.Content != before.Content || restored.Settings != before.Settings || restored.BusIdentity != before.BusIdentity {
		t.Fatal("restore did not preserve closed dormant identity and content")
	}
}
func TestQualityCleanupProviderDeletion(t *testing.T) {
	for _, mode := range []string{"supported", "unsupported", "failing"} {
		for _, action := range []string{"delete", "empty", "expire"} {
			t.Run(mode+"/"+action, func(t *testing.T) {
				f := qualityCleanupNew(t)
				id := "fixture:quality-final"
				f.Seed(qualityCleanupSeed{ID: id, ProviderDelete: mode, CanUnload: true, Queued: 1})
				qualityCleanupResultFor(t, f.Act("remove", id), id, "removed")
				for _, e := range f.Events() {
					if e.Kind == "provider-delete" || e.Kind == "local-delete" {
						t.Fatal("staging invoked permanent deletion")
					}
				}
				if action == "expire" {
					f.Advance(30 * 24 * time.Hour)
				}
				r := qualityCleanupResultFor(t, f.Act(action, id), id, "deleted")
				s := f.Observe(id)
				if s.LocalCopy || s.Removed || s.Active || s.Queued != 0 || !s.Tombstone || s.Content != "" || s.Settings != "" {
					t.Fatal("final deletion retained private state or lost tombstone")
				}
				remains := mode != "supported"
				if r.ProviderCopyRemains != remains || s.ProviderCopy != remains {
					t.Fatal("provider outcome was misreported")
				}
				if remains && r.ProviderDeletionReason == "" {
					t.Fatal("remaining provider copy has no reason")
				}
				attempted, erased := -1, -1
				for i, e := range f.Events() {
					if e.ID != id {
						continue
					}
					if e.Kind == "provider-delete" {
						attempted = i
					}
					if e.Kind == "local-delete" {
						erased = i
					}
				}
				if erased < 0 {
					t.Fatal("local erasure was not observed")
				}
				if mode == "unsupported" {
					if attempted >= 0 {
						t.Fatal("unsupported deletion invoked")
					}
				} else if attempted < 0 || attempted >= erased {
					t.Fatal("provider deletion did not precede local erasure")
				}
				f.Restart()
				s = f.Observe(id)
				if s.Active || s.LocalCopy || !s.Tombstone {
					t.Fatal("deleted provider history returned after restart")
				}
			})
		}
	}
}
func TestQualityCleanupSwitchedChain(t *testing.T) {
	f := qualityCleanupNew(t)
	ids := []string{"codex:old", "claude:middle", "ollama:current", "claude:history-target"}
	for i, id := range ids {
		mode := "supported"
		if i == 1 {
			mode = "unsupported"
		}
		f.Seed(qualityCleanupSeed{ID: id, Chain: ids, HistoryOnly: i == 3, CanUnload: true, ProviderDelete: mode, Queued: 1})
	}
	qualityCleanupResultFor(t, f.Act("remove", ids[1]), ids[1], "removed")
	for _, id := range ids {
		qualityBinned(t, f.Observe(id))
	}
	f.Restart()
	for _, id := range ids {
		qualityBinned(t, f.Observe(id))
	}
	qualityCleanupResultFor(t, f.Act("restore", ids[0]), ids[0], "restored")
	for _, id := range ids {
		s := f.Observe(id)
		if !s.Active || s.Removed || s.Open || s.Loaded || !s.LocalCopy || s.Wakes != 0 {
			t.Fatal("chain restore split or woke the chat")
		}
	}
	qualityCleanupResultFor(t, f.Act("remove", ids[2]), ids[2], "removed")
	qualityCleanupResultFor(t, f.Act("delete", ids[0]), ids[0], "deleted")
	for _, id := range ids {
		s := f.Observe(id)
		if s.LocalCopy || s.ProviderCopy != (id == ids[1]) || !s.Tombstone {
			t.Fatal("chain finalization left a member")
		}
	}
}
func TestQualityCleanupLoadedWithoutUnload(t *testing.T) {
	f := qualityCleanupNew(t)
	id := "fixture:cannot-unload"
	f.Seed(qualityCleanupSeed{ID: id, Loaded: true, CanUnload: false, ProviderDelete: "unsupported"})
	before := f.Observe(id)
	r := qualityCleanupResultFor(t, f.Act("remove", id), id, "removed")
	qualityBinned(t, f.Observe(id))
	if !strings.Contains(strings.ToLower(r.Text), "still loaded") {
		t.Fatal("loaded provider copy was not disclosed")
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
// A pinned chat was refused until 16 November 2026. The person then had to
// find the pin, which shows only while its workspace is open, before a chat
// could be deleted. Deleting now takes the pin off and restoring puts it back.
func TestQualityCleanupTakesAPinnedChat(t *testing.T) {
	f := qualityCleanupNew(t)
	id := "fixture:pinned"
	f.Seed(qualityCleanupSeed{ID: id, Pinned: true, CanUnload: true, ProviderDelete: "supported"})
	qualityCleanupResultFor(t, f.Act("remove", id), id, "removed")
	qualityBinned(t, f.Observe(id))
}
func TestQualityCleanupRefusals(t *testing.T) {
	for _, reason := range []string{"working"} {
		t.Run(reason, func(t *testing.T) {
			f := qualityCleanupNew(t)
			id := "fixture:" + reason
			f.Seed(qualityCleanupSeed{ID: id, Working: reason == "working", Pinned: reason == "pinned"})
			before := f.Observe(id)
			rs := f.Act("remove", id)
			if len(rs) != 1 || rs[0].ID != id || rs[0].OK || rs[0].Status != "refused" || rs[0].Error == "" {
				t.Fatal("protected chat was not refused")
			}
			s := f.Observe(id)
			if s.Removed || !s.Active || s.Content != before.Content {
				t.Fatal("refusal mutated the chat")
			}
		})
	}
}
func TestQualityCleanupExpiryBoundary(t *testing.T) {
	f := qualityCleanupNew(t)
	id := "fixture:expiry"
	f.Seed(qualityCleanupSeed{ID: id, ProviderDelete: "supported", CanUnload: true})
	qualityCleanupResultFor(t, f.Act("remove", id), id, "removed")
	f.Advance(30*24*time.Hour - time.Nanosecond)
	f.Act("expire")
	qualityBinned(t, f.Observe(id))
	f.Advance(time.Nanosecond)
	f.Act("expire")
	s := f.Observe(id)
	if s.LocalCopy || s.Active || !s.Tombstone {
		t.Fatal("thirty-day expiry did not finalize")
	}
	rs := f.Act("restore", id)
	if len(rs) != 1 || rs[0].OK || rs[0].Error == "" {
		t.Fatal("expired chat was restored")
	}
	f.Restart()
	s = f.Observe(id)
	if s.LocalCopy || s.Active {
		t.Fatal("expired chat reappeared")
	}
}

func TestQualityCleanupMixedBatch(t *testing.T) {
	f := qualityCleanupNew(t)
	ids := []string{"fixture:first", "fixture:working", "fixture:last"}
	for i, id := range ids {
		f.Seed(qualityCleanupSeed{ID: id, Working: i == 1, CanUnload: true, ProviderDelete: "supported"})
	}
	rs := f.Act("remove", ids...)
	if len(rs) != len(ids) {
		t.Fatal("batch lost an outcome")
	}
	for i, id := range ids {
		if rs[i].ID != id {
			t.Fatal("batch reordered outcomes")
		}
		if i == 1 {
			if rs[i].OK || rs[i].Status != "refused" || rs[i].Error == "" {
				t.Fatal("working batch member was not refused")
			}
			continue
		}
		qualityCleanupResultFor(t, rs, id, "removed")
		qualityBinned(t, f.Observe(id))
	}
	before := f.Observe(ids[0])
	qualityCleanupResultFor(t, f.Act("remove", ids[0]), ids[0], "removed")
	after := f.Observe(ids[0])
	if after.RemovedAt != before.RemovedAt || after.ExpiresAt != before.ExpiresAt {
		t.Fatal("repeat removal reset retention")
	}
	qualityNoStarts(t, before, after)
}

func TestQualityCleanupProtectedChain(t *testing.T) {
	for _, reason := range []string{"working"} {
		t.Run(reason, func(t *testing.T) {
			f := qualityCleanupNew(t)
			ids := []string{"codex:chain-head", "claude:chain-history"}
			f.Seed(qualityCleanupSeed{ID: ids[0], Chain: ids})
			f.Seed(qualityCleanupSeed{ID: ids[1], Chain: ids, HistoryOnly: true, Working: reason == "working", Pinned: reason == "pinned"})
			rs := f.Act("remove", ids[0])
			if len(rs) != 1 || rs[0].OK || rs[0].Status != "refused" || rs[0].Error == "" {
				t.Fatal("protected history member did not refuse the chain")
			}
			for _, id := range ids {
				if s := f.Observe(id); s.Removed || !s.Active {
					t.Fatal("refusal split the chain")
				}
			}
		})
	}
}

func TestQualityCleanupDraftsAndStaleSave(t *testing.T) {
	for _, action := range []string{"delete", "expire"} {
		t.Run(action, func(t *testing.T) {
			f := qualityCleanupNew(t)
			ids := []string{"codex:draft-earlier", "claude:draft-current"}
			stale := map[string]string{"quality-unrelated-preference": "retained"}
			for _, id := range ids {
				f.Seed(qualityCleanupSeed{ID: id, Chain: ids, CanUnload: true, ProviderDelete: "supported"})
				stale["dinah-draft:chat:"+id] = "invented composer draft for " + id
			}
			f.SaveUIState(stale)
			checkPresent := func() {
				t.Helper()
				got := f.ReadUIState()
				for k, v := range stale {
					if got[k] != v {
						t.Fatalf("retained UI value changed: %s", k)
					}
				}
			}
			checkPresent()
			qualityCleanupResultFor(t, f.Act("remove", ids[1]), ids[1], "removed")
			checkPresent()
			f.Restart()
			checkPresent()
			qualityCleanupResultFor(t, f.Act("restore", ids[0]), ids[0], "restored")
			checkPresent()
			qualityCleanupResultFor(t, f.Act("remove", ids[1]), ids[1], "removed")
			if action == "expire" {
				f.Advance(30 * 24 * time.Hour)
			}
			f.Act(action, ids[1])
			checkAbsent := func() {
				t.Helper()
				got := f.ReadUIState()
				for _, id := range ids {
					if _, ok := got["dinah-draft:chat:"+id]; ok {
						t.Fatal("finalized chain draft remains or was revived")
					}
				}
				if got["quality-unrelated-preference"] != "retained" {
					t.Fatal("unrelated UI state was lost")
				}
			}
			checkAbsent()
			f.SaveUIState(stale)
			checkAbsent()
			f.Restart()
			checkAbsent()
			f.SaveUIState(stale)
			checkAbsent()
		})
	}
}

// These methods bind the existing stores and authority services, not a model.
type qualityCleanupOwnershipFixture interface {
	SeedLinkedRecords([]string)
	LinkedRecords() map[string]string
	RemoteWrites() int
	OpenWithLeases(string) map[string]string
	UseLease(string) error
	StartPendingLeaseWork(string)
	PendingLeaseWork(string) (cancelled, completed bool)
}

func TestQualityCleanupOwnershipAndLeases(t *testing.T) {
	f := qualityCleanupNew(t)
	o, ok := f.(qualityCleanupOwnershipFixture)
	if !ok {
		t.Fatal("ownership and lease binding is required")
	}
	ids := []string{"codex:linked-old", "claude:linked-current"}
	for _, id := range ids {
		f.Seed(qualityCleanupSeed{ID: id, Chain: ids, CanUnload: true, ProviderDelete: "supported"})
	}
	o.SeedLinkedRecords(ids)
	original := o.LinkedRecords()
	for _, key := range []string{"document", "document-record", "document-draft", "document-draft-record"} {
		if original[key] == "" {
			t.Fatalf("missing linked fixture %s", key)
		}
	}
	checkRecords := func() {
		t.Helper()
		got := o.LinkedRecords()
		if len(got) != len(original) {
			t.Fatal("cleanup changed linked record inventory")
		}
		for k, v := range original {
			if got[k] != v {
				t.Fatalf("cleanup changed linked record %s", k)
			}
		}
		if o.RemoteWrites() != 0 {
			t.Fatal("cleanup caused a remote write")
		}
	}
	leases := o.OpenWithLeases(ids[1])
	if leases["drawing"] == "" {
		t.Fatal("no real drawing lease was issued")
	}
	for _, token := range leases {
		if token == "" || o.UseLease(token) != nil {
			t.Fatal("lease was not valid before removal")
		}
		o.StartPendingLeaseWork(token)
	}
	qualityCleanupResultFor(t, f.Act("remove", ids[0]), ids[0], "removed")
	checkRecords()
	checkRevoked := func() {
		t.Helper()
		for _, token := range leases {
			if o.UseLease(token) == nil {
				t.Fatal("old lease still grants authority")
			}
		}
	}
	checkRevoked()
	for _, token := range leases {
		cancelled, completed := o.PendingLeaseWork(token)
		if !cancelled || completed {
			t.Fatal("pending authorized work was not stopped")
		}
	}
	qualityCleanupResultFor(t, f.Act("restore", ids[1]), ids[1], "restored")
	checkRecords()
	checkRevoked()
	fresh := o.OpenWithLeases(ids[1])
	if fresh["drawing"] == "" {
		t.Fatal("reopen did not issue fresh drawing authority")
	}
	for kind, token := range fresh {
		if token == "" || token == leases[kind] || o.UseLease(token) != nil {
			t.Fatal("reopen did not create valid fresh authority")
		}
	}
	checkRevoked()
	qualityCleanupResultFor(t, f.Act("remove", ids[1]), ids[1], "removed")
	for _, token := range fresh {
		if o.UseLease(token) == nil {
			t.Fatal("second removal retained fresh authority")
		}
	}
	qualityCleanupResultFor(t, f.Act("delete", ids[0]), ids[0], "deleted")
	checkRecords()
	checkRevoked()
	f.Restart()
	checkRecords()
}

func TestQualityCleanupSeparateRecipients(t *testing.T) {
	for _, relation := range []string{"fork", "workspace-move"} {
		t.Run(relation, func(t *testing.T) {
			f := qualityCleanupNew(t)
			ids := []string{"codex:owned-old", "claude:owned-current"}
			for _, id := range ids {
				f.Seed(qualityCleanupSeed{ID: id, Chain: ids, CanUnload: true, ProviderDelete: "supported"})
			}
			separate := "codex:separate"
			f.Seed(qualityCleanupSeed{ID: separate, Parent: ids[0], Relation: relation, CanUnload: true, ProviderDelete: "supported", Queued: 1})
			before := f.Observe(separate)
			check := func() {
				t.Helper()
				after := f.Observe(separate)
				if !after.Active || after.Removed || after.Tombstone || after.LocalCopy != before.LocalCopy || after.ProviderCopy != before.ProviderCopy || after.Content != before.Content || after.Settings != before.Settings || after.BusIdentity != before.BusIdentity || after.Queued != before.Queued {
					t.Fatal("cleanup included a separate recipient through ancestry")
				}
				qualityNoStarts(t, before, after)
			}
			qualityCleanupResultFor(t, f.Act("remove", ids[1]), ids[1], "removed")
			check()
			qualityCleanupResultFor(t, f.Act("restore", ids[0]), ids[0], "restored")
			check()
			qualityCleanupResultFor(t, f.Act("remove", ids[1]), ids[1], "removed")
			qualityCleanupResultFor(t, f.Act("delete", ids[0]), ids[0], "deleted")
			check()
			for _, e := range f.Events() {
				if e.ID == separate && (e.Kind == "provider-delete" || e.Kind == "local-delete") {
					t.Fatal("separate recipient received deletion")
				}
			}
		})
	}
}
