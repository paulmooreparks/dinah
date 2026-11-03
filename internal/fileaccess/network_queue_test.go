package fileaccess

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"studio/internal/agents"
	"studio/internal/layout"
)

func waitNetworkState(t *testing.T, p *networkProber, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		p.mu.Lock()
		ok := ready()
		p.mu.Unlock()
		if ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("network verification did not reach the expected state")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestNetworkQueuedSharesKeepLateIdentityDuringStalledRefresh(t *testing.T) {
	roots := make([]string, 5)
	infos := make(map[string]os.FileInfo)
	files := make([]string, len(roots))
	chain := make([]os.FileInfo, 0, len(roots))
	for i := range roots {
		roots[i] = fmt.Sprintf(`\\server\share%d\denied`, i)
		dir := t.TempDir()
		info, err := agents.StatFileIdentity(dir)
		if err != nil {
			t.Fatal(err)
		}
		infos[roots[i]] = info
		chain = append(chain, info)
		files[i] = filepath.Join(dir, "synthetic-key")
		if err := os.WriteFile(files[i], []byte("synthetic secret"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	first, later := make(chan struct{}), make(chan struct{})
	var releaseFirst, releaseLater sync.Once
	t.Cleanup(func() { releaseFirst.Do(func() { close(first) }); releaseLater.Do(func() { close(later) }) })
	started := make(chan string, 16)
	var refresh atomic.Bool
	var active, maximum atomic.Int32
	p := newNetworkProber(func(root string) (os.FileInfo, error) {
		n := active.Add(1)
		defer active.Add(-1)
		for old := maximum.Load(); n > old && !maximum.CompareAndSwap(old, n); old = maximum.Load() {
		}
		started <- root
		if refresh.Load() {
			<-later
			return nil, os.ErrPermission
		}
		<-first
		return infos[root], nil
	})
	p.syncRoots(roots)
	for i := 0; i < NetworkRuleWorkers; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("eager verification did not fill the bounded workers")
		}
	}
	for root, result := range p.probeRoots(roots, chain) {
		if result.verified || result.cached || result.match {
			t.Fatalf("cold queued identity claimed verification for %s: %+v", root, result)
		}
	}
	releaseFirst.Do(func() { close(first) })
	waitNetworkState(t, p, func() bool { return len(p.last) == len(roots) && p.active == 0 })
	refresh.Store(true)
	for root, result := range p.probeRoots(roots, chain) {
		if result.verified || !result.cached || !result.match || result.lastVerifiedAt == "" {
			t.Fatalf("late identity lost during stalled refresh for %s: %+v", root, result)
		}
	}
	if maximum.Load() > NetworkRuleWorkers {
		t.Fatalf("too many metadata workers: %d", maximum.Load())
	}
	if runtime.GOOS == "windows" {
		store, _ := testStore(t)
		svc := &Service{Store: store, network: p}
		for i, root := range roots {
			svc.Defaults = append(svc.Defaults, Rule{ID: fmt.Sprintf("share-%d", i), Kind: "default", Enabled: true, ResolvedPaths: []string{root}})
		}
		scope := Scope{Chat: "chat", Workspace: "workspace", Layout: layout.Layout{State: filepath.Join(t.TempDir(), "state")}}
		for i, file := range files {
			if text, err := svc.Read(context.Background(), file, nil, nil, scope); err == nil || text != "" {
				t.Fatalf("share %d drive alias returned denied bytes: %q, %v", i, text, err)
			}
		}
		policy, err := svc.Policy(scope)
		if err != nil {
			t.Fatal(err)
		}
		for _, rule := range policy.Rules {
			if rule.Kind == "default" && (len(rule.NetworkRoots) != 1 || rule.NetworkRoots[0].UsingLastKnownIdentity || rule.NetworkRoots[0].Status != "verified") {
				t.Fatalf("running refresh replaced the completed verdict: %+v", rule)
			}
		}
	}
	releaseLater.Do(func() { close(later) })
	waitNetworkState(t, p, func() bool { return p.active == 0 })
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, root := range roots {
		state := p.rootStateLocked(root)
		if state.Status != "unverified" || !state.UsingLastKnownIdentity || state.LastVerifiedAt == "" {
			t.Fatalf("completed failed refresh lost the cached denial: %+v", state)
		}
	}
}

func TestNetworkTwentyRootsUseOneShareBatch(t *testing.T) {
	info, err := agents.StatFileIdentity(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	roots := make([]string, 20)
	for i := range roots {
		roots[i] = fmt.Sprintf(`\\server\share\root%d`, i)
	}
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	var calls atomic.Int32
	p := newNetworkProber(func(string) (os.FileInfo, error) {
		calls.Add(1)
		<-release
		return info, nil
	})
	p.syncRoots(roots)
	waitNetworkState(t, p, func() bool { return calls.Load() == 1 })
	workers := p.workers()
	if len(workers) != 1 || len(workers[0].Roots) != len(roots) || workers[0].Root != `\\server\share` {
		t.Fatalf("share roots did not form one bounded batch: %+v", workers)
	}
	once.Do(func() { close(release) })
	waitNetworkState(t, p, func() bool { return len(p.last) == len(roots) && p.active == 0 })
	if calls.Load() != int32(len(roots)) {
		t.Fatalf("each descendant needs its own metadata lookup: %d", calls.Load())
	}
}

func TestNetworkPolicyLoadAndRootChangeStartVerificationBeforeRead(t *testing.T) {
	store, _ := testStore(t)
	started := make(chan string, 4)
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	p := newNetworkProber(func(root string) (os.FileInfo, error) {
		started <- root
		<-release
		return nil, os.ErrPermission
	})
	svc := &Service{Store: store, network: p, Defaults: []Rule{{ID: "first", Kind: "default", Enabled: true, ResolvedPaths: []string{`\\server\first`}}}}
	for _, want := range []string{`\\server\first`, `\\server\second`} {
		if err := svc.WarmNetworkRoots(); err != nil {
			t.Fatal(err)
		}
		select {
		case got := <-started:
			if got != want {
				t.Fatalf("started %q, want %q", got, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("policy load did not start %s before any read", want)
		}
		svc.Defaults = append(svc.Defaults, Rule{ID: "second", Kind: "default", Enabled: true, ResolvedPaths: []string{`\\server\second`}})
	}
	once.Do(func() { close(release) })
	waitNetworkState(t, p, func() bool { return p.active == 0 })
}
