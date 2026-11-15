package main

import (
	"sort"
	"time"

	"studio/internal/sessionhost"
)

// cleanupChains joins every recorded switch edge, including failed targets.
// Display identity follows committed bindings; disposal includes the component.
func cleanupChains(settings Settings, heads map[string]*sessionhost.TranscriptHead, known []string) [][]string {
	parent := map[string]string{}
	var root func(string) string
	root = func(id string) string {
		if _, ok := parent[id]; !ok {
			parent[id] = id
		}
		if parent[id] != id {
			parent[id] = root(parent[id])
		}
		return parent[id]
	}
	link := func(a, b string) {
		if a != "" && b != "" {
			ra, rb := root(a), root(b)
			if ra != rb {
				parent[rb] = ra
			}
		}
	}
	for _, id := range known {
		if id != "" {
			root(id)
		}
	}
	for id, h := range heads {
		root(id)
		link(id, h.ParentSessionID)
	}
	for source, target := range settings.ChatBindings {
		link(source, target)
	}
	dates := map[string]time.Time{}
	transfers := make([]chatTransfer, 0, len(settings.ChatTransfers)+len(settings.ChatTransferHistory))
	for _, tr := range settings.ChatTransfers {
		transfers = append(transfers, tr)
	}
	transfers = append(transfers, settings.ChatTransferHistory...)
	for _, tr := range transfers {
		if tr.Source != "" {
			root(tr.Source)
		}
		link(tr.Source, tr.Target)
		if tr.Target != "" && !tr.Created.IsZero() {
			if at := dates[tr.Target]; at.IsZero() || tr.Created.Before(at) {
				dates[tr.Target] = tr.Created
			}
		}
	}
	groups := map[string][]string{}
	for id := range parent {
		groups[root(id)] = append(groups[root(id)], id)
	}
	// A deterministic topological order keeps ancestors before descendants.
	// Corrupt cycles are retained and ordered by recorded date and identity.
	edges := map[string]map[string]bool{}
	add := func(a, b string) {
		if a != "" && b != "" && a != b {
			if edges[a] == nil {
				edges[a] = map[string]bool{}
			}
			edges[a][b] = true
		}
	}
	for a, b := range settings.ChatBindings {
		add(a, b)
	}
	for _, tr := range transfers {
		add(tr.Source, tr.Target)
	}
	for id, h := range heads {
		add(h.ParentSessionID, id)
	}
	less := func(a, b string) bool {
		if !dates[a].Equal(dates[b]) {
			return dates[a].Before(dates[b])
		}
		return a < b
	}
	out := make([][]string, 0, len(groups))
	for _, members := range groups {
		remaining := map[string]bool{}
		for _, id := range members {
			remaining[id] = true
		}
		ordered := make([]string, 0, len(members))
		for len(remaining) > 0 {
			ready := []string{}
			for id := range remaining {
				blocked := false
				for source := range remaining {
					if edges[source][id] {
						blocked = true
						break
					}
				}
				if !blocked {
					ready = append(ready, id)
				}
			}
			if len(ready) == 0 {
				for id := range remaining {
					ready = append(ready, id)
				}
			}
			sort.Slice(ready, func(i, j int) bool { return less(ready[i], ready[j]) })
			id := ready[0]
			ordered = append(ordered, id)
			delete(remaining, id)
		}
		out = append(out, ordered)
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out
}

func cleanupDisplayIdentity(settings Settings, ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	members := map[string]bool{}
	for _, id := range ids {
		members[id] = true
	}
	// Only a committed switch can select its target as the visible head.
	for i := len(ids) - 1; i >= 0; i-- {
		id := ids[i]
		for source, target := range settings.ChatBindings {
			if target != id || !members[source] {
				continue
			}
			if tr, ok := settings.ChatTransfers[source]; ok && tr.Stage != "committed" {
				continue
			}
			if next := settings.ChatBindings[id]; next == "" || !members[next] {
				return id
			}
		}
	}
	return ids[0]
}
