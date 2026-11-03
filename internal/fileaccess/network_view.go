package fileaccess

import (
	"encoding/json"
	"sort"

	"studio/internal/agents"
)

// rootStateLocked describes the last completed check and any current work.
// Only enforcement requests schedule fresh checks after initial admission.
func (p *networkProber) rootStateLocked(root string) NetworkRuleState {
	result, checked := p.states[root]
	if !checked {
		result.reason = "This folder has not been checked yet"
	}
	if !checked && p.tasks[root] != nil {
		result.reason = "This folder's check is waiting or still running"
	}
	if detail := p.unavailable[root]; detail != "" {
		result.unsupported, result.reason = true, detail
	}
	state := NetworkRuleState{Path: root, Status: "unverified", Reason: result.reason}
	if result.verified {
		state.Status, state.Reason = "verified", ""
	} else if result.unsupported {
		state.Status = "unsupported"
	}
	if last, ok := p.last[root]; ok {
		state.LastVerifiedAt = last.at
		state.UsingLastKnownIdentity = state.Status != "verified"
	}
	return state
}

func (p *networkProber) publishLocked() {
	type rootView struct {
		State    NetworkRuleState
		Identity agents.FileIdentity
	}
	roots := map[string]bool{}
	for root := range p.configured {
		roots[root] = true
	}
	for root := range p.states {
		roots[root] = true
	}
	for root := range p.tasks {
		roots[root] = true
	}
	var names []string
	for root := range roots {
		names = append(names, root)
	}
	sort.Strings(names)
	view := struct {
		Roots   []rootView
		Workers []NetworkWorker
	}{}
	for _, worker := range p.workersLocked() {
		if worker.Stalled {
			view.Workers = append(view.Workers, worker)
		}
	}
	for _, root := range names {
		state := p.rootStateLocked(root)
		// The latest successful time remains available on a display read, but
		// advancing it alone must not reload the page after every agent read.
		state.LastVerifiedAt = ""
		view.Roots = append(view.Roots, rootView{state, p.last[root].key})
	}
	data, _ := json.Marshal(view)
	if string(data) == p.published {
		return
	}
	p.published = string(data)
	p.revision++
	p.notifyLocked()
}
