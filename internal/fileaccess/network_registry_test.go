package fileaccess

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"studio/internal/agents"
	"studio/internal/layout"
	"studio/internal/workspaces"
)

func TestNetworkRegistrySurvivesScopeChanges(t *testing.T) {
	dir := t.TempDir()
	info, err := agents.StatFileIdentity(dir)
	if err != nil {
		t.Fatal(err)
	}
	const root = `\\server\share\B`
	const source = `\\server\sources\B`
	records := []workspaces.Record{{ID: "A", ExecutionPath: t.TempDir()}, {ID: "B", ExecutionPath: root, Sources: []workspaces.Source{{Kind: workspaces.LocalFolder, Path: source}, {Kind: workspaces.LocalFolder, Path: `Z:\B`}}}}
	roots := RegisteredWorkspaceRoots(records)
	if len(roots) != 4 {
		t.Fatalf("registry omitted execution or source roots: %v", roots)
	}
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	var stall atomic.Bool
	p := newNetworkProber(func(string) (os.FileInfo, error) {
		if stall.Load() {
			<-release
		}
		return info, nil
	})
	store, _ := testStore(t)
	s := &Service{Store: store, network: p, Defaults: []Rule{{ID: "other-workspaces", Kind: "default", Enabled: true}}}
	initial := append([]workspaces.Record(nil), records...)
	initial[1].Sources = nil
	if err := s.SetWorkspaceRoots(RegisteredWorkspaceRoots(initial)); err != nil {
		t.Fatal(err)
	}
	waitNetworkState(t, p, func() bool { return len(p.last) == 1 && p.active == 0 })
	if err := s.SetWorkspaceRoots(roots); err != nil {
		t.Fatal(err)
	}
	waitNetworkState(t, p, func() bool { return len(p.last) == 2 && p.active == 0 })
	scopeB := WorkspaceScope(layout.Layout{}, records, root)
	if _, err := s.Policy(scopeB); err != nil {
		t.Fatal(err)
	}
	if err := s.WarmNetworkRoots(); err != nil {
		t.Fatal(err)
	}
	waitNetworkState(t, p, func() bool { return len(p.last) == 2 && p.active == 0 })
	stall.Store(true)
	scopeA := WorkspaceScope(layout.Layout{State: filepath.Join(t.TempDir(), "state")}, records, records[0].ExecutionPath)
	policy, err := s.Policy(scopeA)
	if err != nil {
		t.Fatal(err)
	}
	key, _ := agents.SnapshotIdentity(info)
	if result := p.fallback(root, []os.FileInfo{info}, "stalled"); !result.cached || !result.match {
		t.Fatalf("other scope erased denial: %+v, key %+v", result, key)
	}
	if runtime.GOOS == "windows" {
		file := filepath.Join(dir, "denied.txt")
		if err := os.WriteFile(file, []byte("secret"), 0600); err != nil {
			t.Fatal(err)
		}
		if text, err := s.Read(context.Background(), file, nil, nil, scopeA); err == nil || text != "" {
			t.Fatalf("drive alias leaked: %q %v", text, err)
		}
	}
	if len(policy.Rules) == 0 {
		t.Fatal("missing policy")
	}
	// Removing B and its sources removes their cached authority, even if a
	// metadata call that started earlier succeeds afterwards.
	if err := s.SetWorkspaceRoots(RegisteredWorkspaceRoots(records[:1])); err != nil {
		t.Fatal(err)
	}
	once.Do(func() { close(release) })
	waitNetworkState(t, p, func() bool { return p.active == 0 && len(p.last) == 0 })
}

func TestNetworkBackgroundCompletionNotifies(t *testing.T) {
	info, err := agents.StatFileIdentity(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	p := newNetworkProber(func(string) (os.FileInfo, error) { <-release; return info, nil })
	s := &Service{network: p}
	changes, unsubscribe := s.NetworkChanges()
	defer unsubscribe()
	p.syncRoots([]string{`\\server\share\root`})
	if result := p.probe(context.Background(), `\\server\share\root`, nil); result.verified {
		t.Fatal("stalled lookup verified")
	}
	before := s.NetworkRevision()
	once.Do(func() { close(release) })
	select {
	case <-changes:
	case <-time.After(time.Second):
		t.Fatal("background completion did not notify")
	}
	waitNetworkState(t, p, func() bool { return p.active == 0 && len(p.last) == 1 })
	if s.NetworkRevision() <= before {
		t.Fatal("completion did not change revision")
	}
}

func TestNetworkSuppliersRetainEachOthersIdentities(t *testing.T) {
	roots := []string{`\\server\shareA\root`, `\\server\shareB\root`}
	infos := map[string]os.FileInfo{}
	files := map[string]string{}
	for _, root := range roots {
		dir := t.TempDir()
		info, err := agents.StatFileIdentity(dir)
		if err != nil {
			t.Fatal(err)
		}
		infos[root] = info
		files[root] = filepath.Join(dir, "denied.txt")
		if err := os.WriteFile(files[root], []byte("secret"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	var stall atomic.Bool
	p := newNetworkProber(func(root string) (os.FileInfo, error) {
		if stall.Load() {
			<-release
		}
		return infos[root], nil
	})
	services := make([]*Service, 2)
	for i, root := range roots {
		store, _ := testStore(t)
		services[i] = &Service{Store: store, network: p, Defaults: []Rule{{ID: "denied", Kind: "default", Enabled: true, ResolvedPaths: []string{root}}}}
		if err := services[i].WarmNetworkRoots(); err != nil {
			t.Fatal(err)
		}
	}
	waitNetworkState(t, p, func() bool { return len(p.last) == 2 && p.active == 0 })
	stall.Store(true)
	for i, root := range roots {
		scope := Scope{Layout: layout.Layout{State: filepath.Join(t.TempDir(), "state")}}
		if _, err := services[i].Policy(scope); err != nil {
			t.Fatal(err)
		}
		if result := p.fallback(root, []os.FileInfo{infos[root]}, "stalled"); !result.cached || !result.match {
			t.Fatalf("supplier erased another's identity: %+v", result)
		}
		if runtime.GOOS == "windows" {
			if text, err := services[i].Read(context.Background(), files[root], nil, nil, scope); err == nil || text != "" {
				t.Fatalf("supplier drive alias leaked: %q %v", text, err)
			}
		}
	}
	// Both suppliers can own the same root. Withdrawing one must preserve it.
	services[1].Defaults = append(services[1].Defaults, services[0].Defaults[0])
	if err := services[1].WarmNetworkRoots(); err != nil {
		t.Fatal(err)
	}
	services[0].ReleaseNetworkRoots()
	p.mu.Lock()
	remaining := len(p.last)
	p.mu.Unlock()
	if remaining != 2 {
		t.Fatal("withdrawing one supplier erased shared ownership")
	}
	services[1].ReleaseNetworkRoots()
	once.Do(func() { close(release) })
	waitNetworkState(t, p, func() bool { return p.active == 0 && len(p.last) == 0 })
}

type unsupportedIdentityInfo struct{ os.FileInfo }

func (unsupportedIdentityInfo) IdentityUnavailableReason() string {
	return agents.UnsupportedFileIdentityReason
}

func TestNetworkUnsupportedIdentityReachesPolicy(t *testing.T) {
	info, err := os.Stat(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := newNetworkProber(func(string) (os.FileInfo, error) { return unsupportedIdentityInfo{info}, nil })
	store, _ := testStore(t)
	s := &Service{Store: store, network: p, Defaults: []Rule{{ID: "unsupported", Kind: "default", Enabled: true, ResolvedPaths: []string{`\\server\share\root`}}}}
	policy, err := s.Policy(Scope{})
	if err != nil {
		t.Fatal(err)
	}
	state := policy.Rules[len(policy.Rules)-1].NetworkRoots[0]
	if state.Status != "unverified" || state.UsingLastKnownIdentity || !strings.Contains(state.Reason, "does not support FileIdInfo") || !strings.Contains(state.Reason, "mapped drive letter") {
		t.Fatalf("Settings lacks lasting unsupported warning: %+v", state)
	}
	waitNetworkState(t, p, func() bool { return p.active == 0 })
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.last) != 0 {
		t.Fatal("unsupported identity cached as strong")
	}
}
