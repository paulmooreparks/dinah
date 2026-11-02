package fileaccess

import (
	"context"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const NetworkRuleDeadline = 250 * time.Millisecond
const NetworkRuleWorkers = 4
const networkPendingPerRoot = 64

type NetworkWorker struct {
	Root      string   `json:"root"`
	RuleIDs   []string `json:"ruleIDs,omitempty"`
	StartedAt string   `json:"startedAt"`
	Stalled   bool     `json:"stalled"`
}
type NetworkRuleState struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}
type networkResult struct {
	verified, match bool
	reason          string
}
type networkRequest struct {
	ctx   context.Context
	chain []os.FileInfo
	done  chan networkResult
}
type networkJob struct {
	root    string
	started time.Time
	pending []*networkRequest
}
type networkProber struct {
	mu   sync.Mutex
	jobs map[string]*networkJob
	stat func(string) (os.FileInfo, error)
}

var processNetworkProber = newNetworkProber(os.Stat)

func newNetworkProber(stat func(string) (os.FileInfo, error)) *networkProber {
	return &networkProber{jobs: map[string]*networkJob{}, stat: stat}
}

// UNC deny roots are metadata subjects, not agent read paths. Device names
// remain a separate name-only rule and never enter the filesystem probe.
func uncRoot(path string) bool {
	return strings.HasPrefix(strings.ToUpper(path), `\\?\UNC\`) || (strings.HasPrefix(path, `\\`) && !strings.HasPrefix(path, `\\.\`) && !strings.HasPrefix(path, `\\?\`))
}
func (p *networkProber) probe(ctx context.Context, root string, chain []os.FileInfo) networkResult {
	return p.beginProbe(ctx, root, chain)()
}

// Admission is separate from waiting, so every admitted root starts its own
// deadline and lookup before any result is awaited. Only the four workers
// perform filesystem calls; waiting evaluations create no extra goroutines.
func (p *networkProber) beginProbe(ctx context.Context, root string, chain []os.FileInfo) func() networkResult {
	immediate := func(reason string) func() networkResult {
		return func() networkResult { return networkResult{reason: reason} }
	}
	if ctx.Err() != nil {
		return immediate("Network verification deadline reached")
	}
	request := &networkRequest{ctx: ctx, chain: append([]os.FileInfo(nil), chain...), done: make(chan networkResult, 1)}
	p.mu.Lock()
	job := p.jobs[root]
	if job == nil {
		if len(p.jobs) >= NetworkRuleWorkers {
			p.mu.Unlock()
			return immediate("All network verification workers are occupied")
		}
		job = &networkJob{root: root, started: time.Now()}
		p.jobs[root] = job
		job.pending = append(job.pending, request)
		go p.run(job)
	} else {
		if time.Since(job.started) >= NetworkRuleDeadline {
			p.mu.Unlock()
			return immediate("This network lookup is still running past its deadline")
		}
		if len(job.pending) >= networkPendingPerRoot {
			p.mu.Unlock()
			return immediate("This network verification worker has reached its request bound")
		}
		job.pending = append(job.pending, request)
	}
	p.mu.Unlock()
	return func() networkResult {
		select {
		case result := <-request.done:
			return result
		case <-ctx.Done():
			// Another root may have used the caller's wait while this result
			// finished within its own deadline. Keep that completed result.
			select {
			case result := <-request.done:
				return result
			default:
			}
			return networkResult{reason: "Network verification deadline reached"}
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
		if _, exists := waits[root]; exists {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), NetworkRuleDeadline)
		waits[root] = pending{p.beginProbe(ctx, root, chain), cancel}
	}
	results := make(map[string]networkResult, len(waits))
	for root, request := range waits {
		results[root] = request.wait()
		request.cancel()
	}
	return results
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
func (p *networkProber) run(job *networkJob) {
	for {
		p.mu.Lock()
		batch := job.pending
		job.pending = nil
		if len(batch) == 0 {
			delete(p.jobs, job.root)
			p.mu.Unlock()
			return
		}
		job.started = time.Now()
		p.mu.Unlock()
		live := false
		for _, request := range batch {
			live = live || request.ctx.Err() == nil
		}
		if !live {
			continue
		}
		info, err := p.stat(job.root)
		for _, request := range batch {
			if request.ctx.Err() != nil {
				continue
			}
			result := networkResult{reason: "The network location could not be verified"}
			// SameFile may itself need metadata access. Keep every comparison in
			// this bounded worker, without relying on runtime identity caching.
			if err == nil && os.SameFile(info, info) {
				result = networkResult{verified: true}
				for _, candidate := range request.chain {
					if os.SameFile(info, candidate) {
						result.match = true
						break
					}
				}
			}
			if request.ctx.Err() == nil {
				request.done <- result
			}
		}
	}
}
func (p *networkProber) workers() []NetworkWorker {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := []NetworkWorker{}
	for _, job := range p.jobs {
		out = append(out, NetworkWorker{Root: job.root, StartedAt: job.started.UTC().Format(time.RFC3339Nano), Stalled: time.Since(job.started) >= NetworkRuleDeadline})
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
	results := s.networkProber().probeRoots(networkRoots(*policy), nil)
	for i := range policy.Rules {
		rule := &policy.Rules[i]
		if !rule.Enabled {
			continue
		}
		for _, root := range rule.paths {
			if !uncRoot(root) {
				continue
			}
			result := results[root]
			state := NetworkRuleState{Path: root, Status: "verified"}
			if !result.verified {
				state.Status = "unverified"
				state.Reason = result.reason
				rule.Verification = "unverified"
			}
			if rule.Verification == "" {
				rule.Verification = "verified"
			}
			rule.NetworkRoots = append(rule.NetworkRoots, state)
		}
	}
	policy.NetworkWorkers = s.networkProber().workers()
	for i := range policy.NetworkWorkers {
		for _, rule := range policy.Rules {
			for _, root := range rule.paths {
				if root == policy.NetworkWorkers[i].Root {
					policy.NetworkWorkers[i].RuleIDs = append(policy.NetworkWorkers[i].RuleIDs, rule.ID)
					break
				}
			}
		}
	}
}

func (s *Service) NetworkWorkers() []NetworkWorker { return s.networkProber().workers() }
