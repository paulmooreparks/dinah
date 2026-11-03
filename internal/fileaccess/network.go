package fileaccess

import (
	"context"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"studio/internal/agents"
)

const NetworkRuleDeadline = 250 * time.Millisecond
const NetworkRuleWorkers = 4
const networkPendingPerRoot = 64

type NetworkWorker struct {
	Root      string   `json:"root"`
	Roots     []string `json:"roots,omitempty"`
	RuleIDs   []string `json:"ruleIDs,omitempty"`
	StartedAt string   `json:"startedAt"`
	Stalled   bool     `json:"stalled"`
}
type NetworkRuleState struct {
	Path                   string `json:"path"`
	Status                 string `json:"status"`
	Reason                 string `json:"reason,omitempty"`
	UsingLastKnownIdentity bool   `json:"usingLastKnownIdentity,omitempty"`
	LastVerifiedAt         string `json:"lastVerifiedAt,omitempty"`
}
type networkResult struct {
	verified, match, cached bool
	unsupported             bool
	reason, lastVerifiedAt  string
}
type networkRequest struct {
	ctx   context.Context
	chain []os.FileInfo
	done  chan networkResult
}
type networkTask struct {
	root     string
	requests []*networkRequest
}
type networkJob struct {
	share   string
	started time.Time
	current *networkTask
	tasks   []*networkTask
	active  bool
}
type verifiedIdentity struct {
	key agents.FileIdentity
	at  string
}
type networkProber struct {
	mu          sync.Mutex
	jobs        map[string]*networkJob
	tasks       map[string]*networkTask
	queue       []*networkJob
	active      int
	last        map[string]verifiedIdentity
	configured  map[string]bool
	suppliers   map[*Service]map[string]bool
	revision    uint64
	stat        func(string) (os.FileInfo, error)
	subscribers map[chan struct{}]bool
	unavailable map[string]string
	states      map[string]networkResult
	published   string
}

var processNetworkProber = newNetworkProber(agents.StatFileIdentity)

func newNetworkProber(stat func(string) (os.FileInfo, error)) *networkProber {
	return &networkProber{jobs: map[string]*networkJob{}, tasks: map[string]*networkTask{}, last: map[string]verifiedIdentity{}, states: map[string]networkResult{}, stat: stat}
}
func uncRoot(path string) bool {
	return strings.HasPrefix(strings.ToUpper(path), `\\?\UNC\`) || (strings.HasPrefix(path, `\\`) && !strings.HasPrefix(path, `\\.\`) && !strings.HasPrefix(path, `\\?\`))
}
func networkShare(root string) string {
	path := strings.ReplaceAll(root, "/", `\`)
	if strings.HasPrefix(strings.ToUpper(path), `\\?\UNC\`) {
		path = `\\` + path[8:]
	}
	parts := strings.Split(strings.TrimPrefix(path, `\\`), `\`)
	if len(parts) >= 2 {
		return strings.ToLower(`\\` + parts[0] + `\` + parts[1])
	}
	return root
}
func identityMatches(key agents.FileIdentity, chain []os.FileInfo) bool {
	for _, info := range chain {
		if candidate, ok := agents.SnapshotIdentity(info); ok && candidate == key {
			return true
		}
	}
	return false
}
func (p *networkProber) fallback(root string, chain []os.FileInfo, reason string) networkResult {
	p.mu.Lock()
	last, ok := p.last[root]
	detail := p.unavailable[root]
	if detail != "" {
		reason = detail
	}
	p.mu.Unlock()
	out := networkResult{reason: reason, unsupported: detail != ""}
	if ok {
		out.cached = true
		out.match = identityMatches(last.key, chain)
		out.lastVerifiedAt = last.at
		out.reason += "; using the last verified identity"
	}
	return out
}
func (p *networkProber) scheduleLocked(root string) *networkTask {
	if task := p.tasks[root]; task != nil {
		return task
	}
	share := networkShare(root)
	job := p.jobs[share]
	if job == nil {
		job = &networkJob{share: share}
		p.jobs[share] = job
		p.queue = append(p.queue, job)
	}
	task := &networkTask{root: root}
	p.tasks[root] = task
	job.tasks = append(job.tasks, task)
	p.startLocked()
	return task
}
func (p *networkProber) startLocked() {
	for p.active < NetworkRuleWorkers && len(p.queue) > 0 {
		job := p.queue[0]
		p.queue = p.queue[1:]
		job.active = true
		job.started = time.Now()
		p.active++
		go p.run(job)
	}
}

// syncRoots starts new configured roots eagerly and removes obsolete cached
// authority. An already running system call cannot be cancelled by this change.
func (p *networkProber) syncRoots(roots []string) {
	p.syncRootsFor(nil, roots)
}

func (p *networkProber) syncRootsFor(supplier *Service, roots []string) {
	supplied := map[string]bool{}
	for _, root := range roots {
		supplied[root] = true
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.suppliers == nil {
		p.suppliers = map[*Service]map[string]bool{}
	}
	if len(supplied) == 0 {
		delete(p.suppliers, supplier)
	} else {
		p.suppliers[supplier] = supplied
	}
	wanted := map[string]bool{}
	for _, set := range p.suppliers {
		for root := range set {
			wanted[root] = true
		}
	}
	prior := p.configured
	p.configured = wanted
	for root := range p.states {
		if !wanted[root] {
			delete(p.states, root)
		}
	}
	for root := range p.unavailable {
		if !wanted[root] {
			delete(p.unavailable, root)
		}
	}
	for root := range p.last {
		if !wanted[root] {
			delete(p.last, root)
		}
	}
	for root := range wanted {
		if !prior[root] {
			p.scheduleLocked(root)
		}
	}
	p.publishLocked()
}
func (p *networkProber) probe(ctx context.Context, root string, chain []os.FileInfo) networkResult {
	ctx, cancel := context.WithTimeout(ctx, NetworkRuleDeadline)
	defer cancel()
	return p.beginProbe(ctx, root, chain)()
}
func (p *networkProber) beginProbe(ctx context.Context, root string, chain []os.FileInfo) func() networkResult {
	immediate := func(reason string) func() networkResult {
		return func() networkResult { return p.fallback(root, chain, reason) }
	}
	if ctx.Err() != nil {
		return immediate("Network verification deadline reached")
	}
	request := &networkRequest{ctx: ctx, chain: append([]os.FileInfo(nil), chain...), done: make(chan networkResult, 1)}
	p.mu.Lock()
	task := p.scheduleLocked(root)
	live := task.requests[:0]
	for _, r := range task.requests {
		if r.ctx.Err() == nil {
			live = append(live, r)
		}
	}
	task.requests = live
	if len(task.requests) >= networkPendingPerRoot {
		p.mu.Unlock()
		return immediate("This network verification has reached its waiting-request bound")
	}
	task.requests = append(task.requests, request)
	p.mu.Unlock()
	return func() networkResult {
		select {
		case result := <-request.done:
			return result
		case <-ctx.Done():
			select {
			case result := <-request.done:
				return result
			default:
			}
			return p.fallback(root, chain, "Network verification deadline reached or lookup is queued")
		}
	}
}
func (p *networkProber) probeRoots(roots []string, chain []os.FileInfo) map[string]networkResult {
	type pending struct {
		wait   func() networkResult
		cancel context.CancelFunc
	}
	waits := map[string]pending{}
	for _, root := range roots {
		if _, ok := waits[root]; ok {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), NetworkRuleDeadline)
		waits[root] = pending{p.beginProbe(ctx, root, chain), cancel}
	}
	out := make(map[string]networkResult, len(waits))
	for root, r := range waits {
		out[root] = r.wait()
		r.cancel()
	}
	return out
}
func (p *networkProber) run(job *networkJob) {
	for {
		p.mu.Lock()
		task := job.tasks[0]
		job.tasks = job.tasks[1:]
		job.current = task
		if job.started.IsZero() {
			job.started = time.Now()
		}
		p.publishLocked()
		stalled := time.AfterFunc(NetworkRuleDeadline, func() {
			p.mu.Lock()
			defer p.mu.Unlock()
			if job.current == task {
				p.publishLocked()
			}
		})
		p.mu.Unlock()
		info, err := p.stat(task.root)
		stalled.Stop()
		key, known := agents.SnapshotIdentity(info)
		at := time.Now().UTC().Format(time.RFC3339Nano)
		p.mu.Lock()
		// Successful late metadata is retained even when every waiting caller has
		// timed out. Cancellation of the caller does not cancel the system call.
		if err == nil && known && (p.configured == nil || p.configured[task.root]) {
			p.last[task.root] = verifiedIdentity{key, at}
			delete(p.unavailable, task.root)
		} else if reason := agents.SnapshotIdentityReason(info); reason != "" && (p.configured == nil || p.configured[task.root]) {
			if p.unavailable == nil {
				p.unavailable = map[string]string{}
			}
			p.unavailable[task.root] = reason
		}
		result := networkResult{reason: "The network location could not be verified"}
		if detail := p.unavailable[task.root]; detail != "" {
			result.reason, result.unsupported = detail, true
		}
		if err == nil && known {
			result = networkResult{verified: true, lastVerifiedAt: at}
		} else if last, ok := p.last[task.root]; ok {
			result.cached = true
			result.lastVerifiedAt = last.at
			result.reason += "; using the last verified identity"
		}
		if p.configured == nil || p.configured[task.root] {
			p.states[task.root] = result
		}
		for _, r := range task.requests {
			if r.ctx.Err() != nil {
				continue
			}
			answer := result
			if result.verified {
				answer.match = identityMatches(key, r.chain)
			} else if last, ok := p.last[task.root]; ok {
				answer.match = identityMatches(last.key, r.chain)
			}
			r.done <- answer
		}
		delete(p.tasks, task.root)
		job.current = nil
		finished := len(job.tasks) == 0
		if finished {
			delete(p.jobs, job.share)
			p.active--
			p.startLocked()
		} else {
			job.started = time.Now()
		}
		p.publishLocked()
		p.mu.Unlock()
		if finished {
			return
		}
	}
}

func (p *networkProber) notifyLocked() {
	for subscriber := range p.subscribers {
		select {
		case subscriber <- struct{}{}:
		default:
		}
	}
}

// NetworkChanges coalesces changes to the displayed network snapshot.
// Reading that snapshot never starts a verification.
func (s *Service) NetworkChanges() (<-chan struct{}, func()) {
	p := s.networkProber()
	ch := make(chan struct{}, 1)
	p.mu.Lock()
	if p.subscribers == nil {
		p.subscribers = map[chan struct{}]bool{}
	}
	p.subscribers[ch] = true
	p.mu.Unlock()
	return ch, func() {
		p.mu.Lock()
		delete(p.subscribers, ch)
		p.mu.Unlock()
	}
}
func networkRoots(policy Policy) []string {
	roots := []string{}
	for _, rule := range policy.Rules {
		if !rule.Enabled || rule.Kind == "protected" {
			continue
		}
		paths := rule.paths
		if paths == nil {
			paths = rule.ResolvedPaths
		}
		for _, root := range paths {
			if uncRoot(root) {
				roots = append(roots, root)
			}
		}
	}
	return roots
}
func (p *networkProber) workers() []NetworkWorker {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.workersLocked()
}

func (p *networkProber) workersLocked() []NetworkWorker {
	out := []NetworkWorker{}
	for _, job := range p.jobs {
		if !job.active {
			continue
		}
		roots := []string{}
		if job.current != nil {
			roots = append(roots, job.current.root)
		}
		for _, task := range job.tasks {
			roots = append(roots, task.root)
		}
		out = append(out, NetworkWorker{Root: job.share, Roots: roots, StartedAt: job.started.UTC().Format(time.RFC3339Nano), Stalled: !job.started.IsZero() && time.Since(job.started) >= NetworkRuleDeadline})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Root < out[j].Root })
	return out
}
func (s *Service) networkProber() *networkProber {
	if s.network != nil {
		return s.network
	}
	return processNetworkProber
}
func (s *Service) describeNetwork(policy *Policy) {
	p := s.networkProber()
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range policy.Rules {
		rule := &policy.Rules[i]
		if !rule.Enabled {
			continue
		}
		for _, root := range rule.paths {
			if !uncRoot(root) {
				continue
			}
			state := p.rootStateLocked(root)
			if state.Status != "verified" {
				rule.Verification = "unverified"
			}
			if rule.Verification == "" {
				rule.Verification = "verified"
			}
			rule.NetworkRoots = append(rule.NetworkRoots, state)
		}
	}
	policy.NetworkWorkers = p.workersLocked()
	for i := range policy.NetworkWorkers {
		worker := &policy.NetworkWorkers[i]
		for _, rule := range policy.Rules {
			found := false
			for _, root := range rule.paths {
				for _, active := range worker.Roots {
					found = found || root == active
				}
			}
			if found {
				worker.RuleIDs = append(worker.RuleIDs, rule.ID)
			}
		}
	}
}
func (s *Service) NetworkWorkers() []NetworkWorker { return s.networkProber().workers() }
func (s *Service) NetworkRevision() uint64 {
	p := s.networkProber()
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.revision
}

// WarmNetworkRoots is called when the host loads policy, before any agent read.
func (s *Service) WarmNetworkRoots() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.rawPolicy(Scope{})
	return err
}
