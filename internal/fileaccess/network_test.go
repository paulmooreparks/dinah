package fileaccess

import (
	"context"
	"fmt"
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
)

func TestNetworkVerificationBoundsAndVisibility(t *testing.T) {
	release := make(chan struct{})
	started := make(chan string, NetworkRuleWorkers)
	var calls atomic.Int32
	p := newNetworkProber(func(root string) (os.FileInfo, error) {
		calls.Add(1)
		started <- root
		<-release
		return nil, os.ErrNotExist
	})
	defer close(release)
	var wg sync.WaitGroup
	for i := 0; i < NetworkRuleWorkers; i++ {
		root := fmt.Sprintf(`\\server\share%d`, i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), NetworkRuleDeadline)
			defer cancel()
			p.probe(ctx, root, nil)
		}()
	}
	for i := 0; i < NetworkRuleWorkers; i++ {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			t.Fatal("worker did not start")
		}
	}
	begin := time.Now()
	result := p.probe(context.Background(), `\\server\fifth`, nil)
	if result.verified || !strings.Contains(result.reason, "deadline") || time.Since(begin) > time.Second {
		t.Fatalf("capacity: %+v", result)
	}
	ctx, cancel := context.WithTimeout(context.Background(), NetworkRuleDeadline)
	defer cancel()
	result = p.probe(ctx, `\\server\share0`, nil)
	if result.verified || time.Since(begin) > time.Second {
		t.Fatalf("deadline: %+v", result)
	}
	wg.Wait()
	if calls.Load() != NetworkRuleWorkers {
		t.Fatalf("shared root started extra lookup: %d", calls.Load())
	}
	svc := &Service{network: p}
	policy := Policy{Rules: []Rule{{ID: "network-key", Enabled: true, paths: []string{`\\server\share0`}}}}
	svc.describeNetwork(&policy)
	if policy.Rules[0].Verification != "unverified" || len(policy.NetworkWorkers) != 4 {
		t.Fatalf("visibility: %+v", policy)
	}
	found := false
	for _, w := range policy.NetworkWorkers {
		if w.Root == `\\server\share0` {
			found = w.Stalled && len(w.RuleIDs) == 1 && w.RuleIDs[0] == "network-key"
		}
	}
	if !found {
		t.Fatalf("stalled worker is not attributed: %+v", policy.NetworkWorkers)
	}
	store, _ := testStore(t)
	svc.Store = store
	home := t.TempDir()
	file := filepath.Join(home, "ordinary")
	if err := os.WriteFile(file, []byte("allowed"), 0600); err != nil {
		t.Fatal(err)
	}
	scope := Scope{Chat: "chat", Workspace: "workspace", Layout: layout.Layout{State: filepath.Join(t.TempDir(), "state")}}
	svc.Defaults = []Rule{{ID: "local", Enabled: true, Kind: "default", ResolvedPaths: []string{file}}}
	if _, err := svc.Read(context.Background(), file, nil, nil, scope); err == nil {
		t.Fatal("occupied network workers disabled local denial")
	}
	svc.Defaults = nil
	if text, err := svc.Read(context.Background(), file, nil, nil, scope); err != nil || text != "allowed" {
		t.Fatalf("local read blocked: %q %v", text, err)
	}
}

func TestNetworkVerificationDoesNotCacheSuccess(t *testing.T) {
	a, err := agents.StatFileIdentity(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b, err := agents.StatFileIdentity(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var calls atomic.Int32
	p := newNetworkProber(func(string) (os.FileInfo, error) {
		if calls.Add(1) == 1 {
			return a, nil
		}
		return b, nil
	})
	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		result := p.probe(ctx, "root", []os.FileInfo{a})
		cancel()
		if !result.verified || result.match != (i == 0) {
			t.Fatalf("lookup %d: %+v", i, result)
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("cached identity: %d", calls.Load())
	}
}

func TestWindowsUNCDefaultRefusesDriveAlias(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows administrative-share alias")
	}
	home := t.TempDir()
	if len(home) < 3 || home[1] != ':' {
		t.Fatal("fixture needs a drive-letter temporary folder")
	}
	unc := `\\localhost\` + home[:1] + "$" + home[2:]
	ctx, cancel := context.WithTimeout(context.Background(), NetworkRuleDeadline)
	defer cancel()
	probe := newNetworkProber(agents.StatFileIdentity).probe(ctx, unc, nil)
	if !probe.verified {
		t.Skip("local administrative share unavailable: " + probe.reason)
	}
	file := filepath.Join(home, "synthetic-key")
	if err := os.WriteFile(file, []byte("synthetic secret"), 0600); err != nil {
		t.Fatal(err)
	}
	store, _ := testStore(t)
	svc := &Service{Store: store, Defaults: DefaultRules("windows", t.TempDir(), "", "", map[string]string{"GNUPGHOME": unc})}
	scope := Scope{Chat: "chat", Workspace: "workspace", Layout: layout.Layout{State: filepath.Join(t.TempDir(), "state")}}
	if text, err := svc.Read(context.Background(), file, nil, nil, scope); err == nil || text != "" {
		t.Fatalf("UNC default bypassed: %q %v", text, err)
	}
	log, err := store.Log(Filter{})
	if err != nil || len(log.Rows) != 1 || log.Rows[0].Outcome != "denied" {
		t.Fatalf("denial audit: %+v %v", log, err)
	}
}

func TestWindowsUnverifiedRuleIsAuditedInsideWorkspace(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows network deny rules")
	}
	release := make(chan struct{})
	defer close(release)
	p := newNetworkProber(func(string) (os.FileInfo, error) { <-release; return nil, os.ErrNotExist })
	store, _ := testStore(t)
	home := t.TempDir()
	file := filepath.Join(home, "ordinary")
	if err := os.WriteFile(file, []byte("allowed"), 0600); err != nil {
		t.Fatal(err)
	}
	svc := &Service{Store: store, network: p, Defaults: []Rule{{ID: "network-secret", Kind: "default", Enabled: true, ResolvedPaths: []string{`\\server\unavailable`}}}}
	scope := Scope{Chat: "chat", Workspace: "workspace", Roots: []string{home}, Layout: layout.Layout{State: filepath.Join(t.TempDir(), "state")}}
	text, err := svc.Read(context.Background(), file, nil, nil, scope)
	if err != nil || text != "allowed" {
		t.Fatalf("local content: %q %v", text, err)
	}
	log, err := store.Log(Filter{})
	if err != nil || len(log.Rows) != 1 || len(log.Rows[0].UnverifiedRuleIDs) != 1 || log.Rows[0].UnverifiedRuleIDs[0] != "network-secret" {
		t.Fatalf("missing unverified audit: %+v %v", log, err)
	}
}

func TestCollapsedAuditKeepsUnverifiedRules(t *testing.T) {
	store, _ := testStore(t)
	for _, ids := range [][]string{{"first"}, {"second"}, nil} {
		if _, err := store.Append(Record{Chat: "chat", Source: "read", Path: "synthetic", Outcome: "error", UnverifiedRuleIDs: ids}); err != nil {
			t.Fatal(err)
		}
	}
	page, err := store.Log(Filter{})
	if err != nil || len(page.Rows) != 1 || page.Rows[0].Count != 3 || len(page.Rows[0].UnverifiedRuleIDs) != 2 {
		t.Fatalf("collapsed verification lost: %+v %v", page, err)
	}
}

func TestWindowsSlowNetworkRootsDoNotStarveLaterAlias(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows network deny rules")
	}
	first, second := t.TempDir(), t.TempDir()
	one, err := agents.StatFileIdentity(first)
	if err != nil {
		t.Fatal(err)
	}
	two, err := agents.StatFileIdentity(second)
	if err != nil {
		t.Fatal(err)
	}
	store, _ := testStore(t)
	svc := &Service{Store: store, network: newNetworkProber(func(root string) (os.FileInfo, error) {
		time.Sleep(150 * time.Millisecond)
		if root == `\\server\first` {
			return one, nil
		}
		return two, nil
	}), Defaults: []Rule{
		{ID: "first", Kind: "default", Enabled: true, ResolvedPaths: []string{`\\server\first`}},
		{ID: "second", Kind: "default", Enabled: true, ResolvedPaths: []string{`\\server\second`}},
	}}
	file := filepath.Join(second, "synthetic-key")
	if err := os.WriteFile(file, []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	scope := Scope{Chat: "chat", Workspace: "workspace", Layout: layout.Layout{State: filepath.Join(t.TempDir(), "state")}}
	for i := 0; i < 3; i++ {
		if text, err := svc.Read(context.Background(), file, nil, nil, scope); err == nil || text != "" {
			t.Fatalf("second slow root bypassed on read %d: %q %v", i, text, err)
		}
	}
	policy, err := svc.Policy(scope)
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range policy.Rules {
		if rule.ID == "first" || rule.ID == "second" {
			if rule.Verification != "verified" {
				t.Fatalf("slow root starved in settings: %+v", rule)
			}
		}
	}
}

func TestNetworkAccessDeniedIsImmediatelyUnverified(t *testing.T) {
	p := newNetworkProber(func(string) (os.FileInfo, error) {
		return nil, &os.PathError{Op: "stat", Path: "synthetic", Err: os.ErrPermission}
	})
	begin := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result := p.probe(ctx, `\\server\denied`, nil)
	if result.verified || result.match || result.reason == "" || ctx.Err() != nil || time.Since(begin) >= NetworkRuleDeadline {
		t.Fatalf("access denial did not return immediately as unverified: %+v", result)
	}
	svc := &Service{network: p}
	policy := Policy{Rules: []Rule{{ID: "denied-share", Enabled: true, paths: []string{`\\server\denied`}}}}
	svc.describeNetwork(&policy)
	if policy.Rules[0].Verification != "unverified" || len(policy.Rules[0].NetworkRoots) != 1 || policy.Rules[0].NetworkRoots[0].Status != "unverified" {
		t.Fatalf("access denial hidden in policy: %+v", policy)
	}
}

func TestCompletedNetworkRootKeepsItsResultAfterOtherWait(t *testing.T) {
	info, err := agents.StatFileIdentity(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	p := newNetworkProber(func(string) (os.FileInfo, error) { return info, nil })
	ctx, cancel := context.WithTimeout(context.Background(), NetworkRuleDeadline)
	defer cancel()
	wait := p.beginProbe(ctx, "completed", []os.FileInfo{info})
	<-ctx.Done()
	result := wait()
	if !result.verified || !result.match {
		t.Fatalf("completed identity discarded after another wait: %+v", result)
	}
}
