package fileaccess

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"studio/internal/agents"
	"studio/internal/layout"
)

func TestNetworkHealthyReadsKeepCompletedVerdictAndQuietEvents(t *testing.T) {
	info, err := agents.StatFileIdentity(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	p := newNetworkProber(func(string) (os.FileInfo, error) {
		calls.Add(1)
		time.Sleep(50 * time.Millisecond)
		return info, nil
	})
	const root = `\\server\healthy\root`
	store, _ := testStore(t)
	s := &Service{Store: store, network: p, Defaults: []Rule{{ID: "healthy", Kind: "default", Enabled: true, ResolvedPaths: []string{root}}}}
	if err := s.WarmNetworkRoots(); err != nil {
		t.Fatal(err)
	}
	waitNetworkState(t, p, func() bool { return p.active == 0 && len(p.states) == 1 })
	initial, err := s.Policy(Scope{})
	if err != nil {
		t.Fatal(err)
	}
	initialTime := initial.Rules[len(initial.Rules)-1].NetworkRoots[0].LastVerifiedAt
	beforeRevision := s.NetworkRevision()
	changes, release := s.NetworkChanges()
	defer release()
	dir := t.TempDir()
	path := filepath.Join(dir, "ordinary.txt")
	if err := os.WriteFile(path, []byte("allowed"), 0600); err != nil {
		t.Fatal(err)
	}
	scope := Scope{Chat: "reader", Workspace: "workspace", Roots: []string{dir}, Layout: layout.Layout{State: filepath.Join(t.TempDir(), "state")}}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	errors := make(chan error, 1)
	var observations atomic.Int32
	go func() {
		defer close(done)
		tick := time.NewTicker(time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				p.mu.Lock()
				running := p.tasks[root] != nil
				p.mu.Unlock()
				policy, err := s.Policy(scope)
				if err != nil {
					errors <- err
					return
				}
				state := policy.Rules[len(policy.Rules)-1].NetworkRoots[0]
				if state.Status != "verified" || state.UsingLastKnownIdentity || state.LastVerifiedAt == "" {
					errors <- fmt.Errorf("healthy refresh changed its verdict: %+v", state)
					return
				}
				if running {
					observations.Add(1)
				}
			}
		}
	}()
	t.Cleanup(func() { cancel(); <-done })
	for i := 0; i < 20; i++ {
		before := calls.Load()
		if runtime.GOOS == "windows" {
			text, err := s.Read(context.Background(), path, nil, nil, scope)
			if err != nil || text != "allowed" {
				t.Fatalf("enforcement read %d: %q %v", i, text, err)
			}
		} else if result := p.probe(context.Background(), root, nil); !result.verified {
			t.Fatal("fresh enforcement check failed")
		}
		if calls.Load() <= before {
			t.Fatal("a read skipped fresh verification")
		}
	}
	cancel()
	<-done
	select {
	case err := <-errors:
		t.Fatal(err)
	default:
	}
	if observations.Load() == 0 {
		t.Fatal("guard never observed an in-progress refresh")
	}
	events := 0
drain:
	for {
		select {
		case <-changes:
			events++
		default:
			break drain
		}
	}
	if events > 1 || s.NetworkRevision() != beforeRevision {
		t.Fatalf("healthy reads produced state events: %d", events)
	}
	final, err := s.Policy(scope)
	if err != nil {
		t.Fatal(err)
	}
	if final.Rules[len(final.Rules)-1].NetworkRoots[0].LastVerifiedAt == initialTime {
		t.Fatal("fresh checks did not advance the available timestamp")
	}
}

func TestNetworkIdentityReplacementAndFailureEachNotifyOnce(t *testing.T) {
	one, err := agents.StatFileIdentity(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	two, err := agents.StatFileIdentity(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var phase atomic.Int32
	p := newNetworkProber(func(string) (os.FileInfo, error) {
		time.Sleep(50 * time.Millisecond)
		switch phase.Load() {
		case 0:
			return one, nil
		case 1:
			return two, nil
		default:
			return nil, os.ErrNotExist
		}
	})
	const root = `\\server\changing\root`
	if result := p.probe(context.Background(), root, nil); !result.verified {
		t.Fatal("initial check failed")
	}
	s := &Service{network: p}
	changes, release := s.NetworkChanges()
	defer release()
	assertOne := func(before uint64) {
		t.Helper()
		if s.NetworkRevision() != before+1 {
			t.Fatal("transition did not change exactly one semantic revision")
		}
		select {
		case <-changes:
		default:
			t.Fatal("transition did not notify")
		}
		select {
		case <-changes:
			t.Fatal("transition notified more than once")
		default:
		}
	}
	before := s.NetworkRevision()
	phase.Store(1)
	if result := p.probe(context.Background(), root, []os.FileInfo{two}); !result.verified || !result.match {
		t.Fatal("replacement check did not use new identity")
	}
	assertOne(before)
	policy := Policy{Rules: []Rule{{ID: "changing", Enabled: true, paths: []string{root}}}}
	s.describeNetwork(&policy)
	if policy.Rules[0].NetworkRoots[0].Status != "verified" {
		t.Fatal("replacement changed verified status")
	}
	before = s.NetworkRevision()
	phase.Store(2)
	if result := p.probe(context.Background(), root, nil); result.verified || !result.cached {
		t.Fatal("missing share did not retain fallback")
	}
	assertOne(before)
	policy = Policy{Rules: []Rule{{ID: "changing", Enabled: true, paths: []string{root}}}}
	s.describeNetwork(&policy)
	if state := policy.Rules[0].NetworkRoots[0]; state.Status != "unverified" || !state.UsingLastKnownIdentity {
		t.Fatal("missing share kept a verified verdict")
	}
	before = s.NetworkRevision()
	p.probe(context.Background(), root, nil)
	if s.NetworkRevision() != before {
		t.Fatal("repeated failure changed semantic revision")
	}
	select {
	case <-changes:
		t.Fatal("unchanged failure notified again")
	default:
	}
}

func TestNetworkPolicyRefreshDoesNotRestartSlowVerification(t *testing.T) {
	info, err := agents.StatFileIdentity(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	p := newNetworkProber(func(string) (os.FileInfo, error) {
		calls.Add(1)
		time.Sleep(300 * time.Millisecond)
		return info, nil
	})
	store, _ := testStore(t)
	s := &Service{Store: store, network: p, Defaults: []Rule{{ID: "slow", Kind: "default", Enabled: true, ResolvedPaths: []string{`\\server\slow\root`}}}}
	changes, release := s.NetworkChanges()
	defer release()
	initial, err := s.Policy(Scope{})
	if err != nil {
		t.Fatal(err)
	}
	if state := initial.Rules[len(initial.Rules)-1].NetworkRoots[0]; state.Status != "unverified" || state.LastVerifiedAt != "" {
		t.Fatalf("new root fabricated verification: %+v", state)
	}
	seen := map[string]bool{}
	sawStalled, sawVerified := false, false
	events := 0
	end := time.NewTimer(900 * time.Millisecond)
	defer end.Stop()
loop:
	for {
		select {
		case <-changes:
			// This is the page's real event -> Policy refresh path.
			policy, err := s.Policy(Scope{})
			if err != nil {
				t.Fatal(err)
			}
			encoded, _ := json.Marshal(policy)
			if seen[string(encoded)] {
				t.Fatal("unchanged policy emitted another notification")
			}
			seen[string(encoded)] = true
			events++
			for _, worker := range policy.NetworkWorkers {
				sawStalled = sawStalled || worker.Stalled
			}
			state := policy.Rules[len(policy.Rules)-1].NetworkRoots[0]
			sawVerified = sawVerified || state.Status == "verified" && state.LastVerifiedAt != ""
		case <-end.C:
			break loop
		}
	}
	if calls.Load() != 1 || events > 3 || !sawStalled || !sawVerified {
		t.Fatalf("display drove probes or lost a transition: calls=%d events=%d stalled=%v verified=%v", calls.Load(), events, sawStalled, sawVerified)
	}
	before := s.NetworkRevision()
	for i := 0; i < 10; i++ {
		if _, err := s.Policy(Scope{}); err != nil {
			t.Fatal(err)
		}
	}
	p.mu.Lock()
	p.publishLocked()
	p.publishLocked()
	p.mu.Unlock()
	if s.NetworkRevision() != before || calls.Load() != 1 {
		t.Fatal("unchanged snapshot changed revision or scheduled work")
	}
	select {
	case <-changes:
		t.Fatal("unchanged snapshot notified")
	default:
	}
}

func TestNetworkNeverCheckedSnapshotIsUnverified(t *testing.T) {
	var calls atomic.Int32
	p := newNetworkProber(func(string) (os.FileInfo, error) { calls.Add(1); return nil, os.ErrNotExist })
	s := &Service{network: p}
	policy := Policy{Rules: []Rule{{ID: "never-checked", Enabled: true, paths: []string{`\\server\new\root`}}}}
	s.describeNetwork(&policy)
	state := policy.Rules[0].NetworkRoots[0]
	if state.Status != "unverified" || state.LastVerifiedAt != "" || state.UsingLastKnownIdentity || state.Reason == "" || policy.Rules[0].Verification != "unverified" || calls.Load() != 0 {
		t.Fatalf("unprobed snapshot: %+v, calls=%d", policy, calls.Load())
	}
}

func TestWindowsUnsupportedRootStillAuditsUnverified(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows network enforcement")
	}
	dir := t.TempDir()
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	p := newNetworkProber(func(string) (os.FileInfo, error) { return unsupportedIdentityInfo{info}, nil })
	store, _ := testStore(t)
	s := &Service{Store: store, network: p, Defaults: []Rule{{ID: "unsupported", Kind: "default", Enabled: true, ResolvedPaths: []string{`\\server\unsupported\root`}}}}
	path := filepath.Join(dir, "ordinary.txt")
	if err := os.WriteFile(path, []byte("allowed"), 0600); err != nil {
		t.Fatal(err)
	}
	scope := Scope{Chat: "chat", Workspace: "workspace", Roots: []string{dir}, Layout: layout.Layout{State: filepath.Join(t.TempDir(), "state")}}
	text, err := s.Read(context.Background(), path, nil, nil, scope)
	if err != nil || text != "allowed" {
		t.Fatalf("ordinary read: %q %v", text, err)
	}
	page, err := store.Log(Filter{})
	if err != nil || len(page.Rows) != 1 || len(page.Rows[0].UnverifiedRuleIDs) != 1 || page.Rows[0].UnverifiedRuleIDs[0] != "unsupported" {
		t.Fatalf("unsupported verdict lost audit: %+v %v", page, err)
	}
	policy, err := s.Policy(scope)
	if err != nil {
		t.Fatal(err)
	}
	state := policy.Rules[len(policy.Rules)-1].NetworkRoots[0]
	if state.Status != "unsupported" || state.LastVerifiedAt != "" || strings.Contains(state.Reason, "FileIdInfo") || policy.Rules[len(policy.Rules)-1].Verification != "unverified" {
		t.Fatalf("unsupported policy: %+v", policy)
	}
}

func TestNetworkUnsupportedRefreshKeepsLastKnownIdentity(t *testing.T) {
	info, err := agents.StatFileIdentity(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var unsupported atomic.Bool
	p := newNetworkProber(func(string) (os.FileInfo, error) {
		if unsupported.Load() {
			return unsupportedIdentityInfo{info}, nil
		}
		return info, nil
	})
	const root = `\\server\share\root`
	if result := p.probe(context.Background(), root, []os.FileInfo{info}); !result.verified || !result.match {
		t.Fatal("initial identity was not verified")
	}
	unsupported.Store(true)
	result := p.probe(context.Background(), root, []os.FileInfo{info})
	if result.verified || !result.unsupported || !result.cached || !result.match || result.lastVerifiedAt == "" {
		t.Fatalf("unsupported refresh discarded identity: %+v", result)
	}
	policy := Policy{Rules: []Rule{{ID: "cached", Enabled: true, paths: []string{root}}}}
	(&Service{network: p}).describeNetwork(&policy)
	state := policy.Rules[0].NetworkRoots[0]
	if state.Status != "unsupported" || !state.UsingLastKnownIdentity || state.LastVerifiedAt == "" || policy.Rules[0].Verification != "unverified" {
		t.Fatalf("fallback status missing: %+v", policy)
	}
}
