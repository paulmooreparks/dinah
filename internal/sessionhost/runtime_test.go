package sessionhost

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/bushost"
)

func TestRuntimeProviderGenerationAndConversationIdentity(t *testing.T) {
	var r Runtime
	old := NewProvider(nil, acp.InitializeResult{}, nil)
	replacement := NewProvider(nil, acp.InitializeResult{}, nil)
	first := NewConversation(Identity{ID: "chat", Provider: "codex"}, InitialActivity{Open: true}, false, false)
	second := NewConversation(Identity{ID: "chat", Provider: "codex"}, InitialActivity{Open: true}, false, false)
	other := NewConversation(Identity{ID: "other", Provider: "claude"}, InitialActivity{Open: true}, false, false)
	r.PutProviderLocked("codex", old)
	r.PutConversationLocked("chat", first)
	r.PutProviderLocked("codex", replacement)
	r.PutConversationLocked("chat", second)
	r.PutConversationLocked("other", other)
	if r.RemoveExpectedLocked("chat", first) {
		t.Fatal("stale conversation removed its replacement")
	}
	if ended := r.DetachProviderLocked("codex", old); len(ended) != 0 {
		t.Fatal("stale watcher detached a replacement generation")
	}
	if r.ProviderLocked("codex") != replacement || r.ConversationLocked("chat") != second {
		t.Fatal("replacement membership changed")
	}
	views := r.ConversationsLocked()
	views[0] = nil
	if r.ConversationCountLocked() != 2 {
		t.Fatal("view mutated registry")
	}
	ended := r.DetachProviderLocked("codex", replacement)
	if len(ended) != 1 || ended[0] != second || r.ConversationLocked("chat") != nil || r.ProviderLocked("codex") != nil || r.ConversationLocked("other") != other {
		t.Fatal("provider and conversation membership were not detached together")
	}
	select {
	case <-replacement.Finished():
		t.Fatal("detach completed teardown early")
	default:
	}
	replacement.Finish()
	replacement.Wait()
	replacement.Finish()
	r.PutConversationLocked("chat", second)
	r.RemoveConversationLocked("chat")
	if r.ConversationLocked("chat") != nil {
		t.Fatal("unconditional removal retained membership")
	}
}

func TestProviderInitializationIsCopiedWithoutChangingNilFields(t *testing.T) {
	initial := acp.InitializeResult{AgentInfo: &acp.Implementation{Name: "before"}, AuthMethods: []acp.AuthMethod{{ID: "original"}}}
	initial.AgentCapabilities.SessionCapabilities.Close = &struct{}{}
	p := NewProvider(nil, initial, nil)
	initial.AgentInfo.Name = "caller mutation"
	initial.AuthMethods[0].ID = "caller mutation"
	view := p.Initialization()
	view.AgentInfo.Name = "view mutation"
	view.AuthMethods[0].ID = "view mutation"
	view.AgentCapabilities.SessionCapabilities.Close = nil
	kept := p.Initialization()
	if kept.AgentInfo.Name != "before" || kept.AuthMethods[0].ID != "original" || kept.AgentCapabilities.SessionCapabilities.Close == nil {
		t.Fatal("initialization shared nested values")
	}
	empty := NewProvider(nil, acp.InitializeResult{}, nil).Initialization()
	if empty.AgentInfo != nil || empty.AuthMethods != nil || empty.AgentCapabilities.SessionCapabilities.List != nil {
		t.Fatal("nil initialization fields became populated")
	}
}

func TestRuntimeAdmissionCompletionAndWindowsShareStateAuthority(t *testing.T) {
	var r Runtime
	var state sync.Mutex
	now := time.Now()
	paths := map[string]string{}
	r.SetWindowsLocked(nil)
	r.ExpireWakeLocked(now)
	for i := 0; i < 30; i++ {
		id := fmt.Sprint(i)
		paths[id] = "folder"
		r.ConsiderWakeLocked(id, "folder", 1, false, now)
	}
	admit := func() {
		state.Lock()
		defer state.Unlock()
		for {
			_, ok := r.NextWakeLocked(paths, now)
			if !ok {
				break
			}
		}
		if n := r.BackgroundUsedLocked(); n > bushost.BackgroundSlots {
			t.Errorf("combined admission bound exceeded: %d", n)
		}
	}
	admit()
	var workers sync.WaitGroup
	for i := 0; i < 30; i++ {
		id := fmt.Sprint(i)
		workers.Add(2)
		go func() {
			defer workers.Done()
			state.Lock()
			r.PutConversationLocked(id, NewConversation(Identity{ID: id}, InitialActivity{Open: true}, false, false))
			r.CompleteWakeLocked(id, nil, now)
			if n := r.BackgroundUsedLocked(); n > bushost.BackgroundSlots {
				t.Errorf("completion freed a slot before counting its session: %d", n)
			}
			state.Unlock()
			admit()
		}()
		go func() { defer workers.Done(); state.Lock(); r.SetWindowsLocked([]string{id}); state.Unlock(); admit() }()
	}
	workers.Wait()
}

func TestConversationConstructionAndLifecycleClockOrder(t *testing.T) {
	c := NewConversation(Identity{ID: "c", Title: "title"}, InitialActivity{Open: true}, false, false)
	if c.turns.mu != &c.mu || c.details.mu != &c.mu || c.activity.turns != c.turns || c.activity.details != c.details {
		t.Fatal("component mutex identities differ")
	}
	calls := 0
	now := time.Now()
	c.BackgroundAt(func() time.Time {
		calls++
		if !c.activity.inBackground {
			t.Fatal("clock preceded background mutation")
		}
		return now
	})
	if !c.ParkAt(func() time.Time {
		calls++
		if !c.activity.parked || !c.activity.open || c.activity.inBackground {
			t.Fatal("clock preceded park transition")
		}
		return now
	}) {
		t.Fatal("idle background did not park")
	}
	if c.ParkAt(func() time.Time { t.Fatal("rejected park called clock"); return now }) {
		t.Fatal("park repeated")
	}
	c.AbandonEndAt(func() time.Time {
		calls++
		if !c.activity.open || !c.activity.parked || c.activity.ending != nil {
			t.Fatal("clock preceded abandon transition")
		}
		return now
	})
	if calls != 3 {
		t.Fatalf("clock calls: %d", calls)
	}
	state := c.State()
	state.Title = "changed"
	title := "later"
	c.UpdateDisplay(&title, nil)
	if c.Title() != "later" || state.ID != "c" {
		t.Fatal("display metadata was not privately owned")
	}
}
