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
	if p.tasks[root] != nil {
		result.verified = false
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
	}{Workers: p.workersLocked()}
	for _, root := range names {
		view.Roots = append(view.Roots, rootView{p.rootStateLocked(root), p.last[root].key})
	}
	data, _ := json.Marshal(view)
	if string(data) == p.published {
		return
	}
	p.published = string(data)
	p.revision++
	p.notifyLocked()
}
