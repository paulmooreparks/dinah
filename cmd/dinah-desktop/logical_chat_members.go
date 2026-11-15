package main

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

type LogicalChatMember struct {
	SessionID string
	Provider  string
	NativeID  string
	State     string
	AttemptID string
	CreatedAt time.Time
}

type LogicalChatAllocation struct {
	AttemptID string
	Source    string
	Provider  string
	State     string
	Status    string
	CreatedAt time.Time
	Reason    string
}

type LogicalChatRelation struct {
	ChatID      string
	SessionID   string
	RecipientID string
	State       string
	Kind        string
}

type LogicalChatMembers struct {
	ChatID                string
	Generation            string
	Head                  string
	Members               []LogicalChatMember
	UnresolvedAllocations []LogicalChatAllocation
	SeparateRecipients    []LogicalChatRelation
}

// logicalChatMembersFromSettings is the replaceable legacy ownership importer.
// It is pure so a mutation can recheck the same generation under updateSettings.
func logicalChatMembersFromSettings(s Settings, reference string) (LogicalChatMembers, error) {
	if reference == "" {
		return LogicalChatMembers{}, errors.New("choose a chat")
	}
	filtered := s
	filtered.ChatBindings = map[string]string{}
	filtered.ChatTransfers = map[string]chatTransfer{}
	filtered.ChatTransferHistory = nil
	separate := []LogicalChatRelation{}
	distinct := func(source, target, recipient string) bool {
		if target == "" {
			return false
		}
		a, b := s.BusSessions[source], s.BusSessions[target]
		if a == "" {
			a = recipient
		}
		return a != "" && b != "" && a != b
	}
	for source, target := range s.ChatBindings {
		if distinct(source, target, "") {
			separate = append(separate, LogicalChatRelation{ChatID: target, SessionID: target, RecipientID: s.BusSessions[target], State: "separate_recipient", Kind: "separate_recipient"})
			continue
		}
		filtered.ChatBindings[source] = target
	}
	for source, tr := range s.ChatTransfers {
		if distinct(tr.Source, tr.Target, tr.Recipient) {
			separate = append(separate, LogicalChatRelation{ChatID: tr.Target, SessionID: tr.Target, RecipientID: s.BusSessions[tr.Target], State: "separate_recipient", Kind: "separate_recipient"})
			continue
		}
		filtered.ChatTransfers[source] = tr
	}
	for _, tr := range s.ChatTransferHistory {
		if distinct(tr.Source, tr.Target, tr.Recipient) {
			separate = append(separate, LogicalChatRelation{ChatID: tr.Target, SessionID: tr.Target, RecipientID: s.BusSessions[tr.Target], State: "separate_recipient", Kind: "separate_recipient"})
			continue
		}
		filtered.ChatTransferHistory = append(filtered.ChatTransferHistory, tr)
	}
	groups := cleanupChains(filtered, nil, []string{reference})
	var ids []string
	for _, group := range groups {
		for _, id := range group {
			if id == reference {
				ids = group
				break
			}
		}
	}
	members := map[string]bool{}
	for _, id := range ids {
		members[id] = true
	}
	committed := map[string]string{}
	checkCommitted := func(source, target string) error {
		if target == "" {
			return nil
		}
		if prior := committed[source]; prior != "" && prior != target {
			return errors.New("this chat has more than one committed successor; resolve its ownership before deleting it")
		}
		committed[source] = target
		return nil
	}
	for source, target := range filtered.ChatBindings {
		if !members[source] {
			continue
		}
		if tr, ok := filtered.ChatTransfers[source]; ok && tr.Stage != "committed" {
			continue
		}
		if err := checkCommitted(source, target); err != nil {
			return LogicalChatMembers{}, err
		}
	}
	transfers := valuesOfChatTransfers(filtered.ChatTransfers)
	transfers = append(transfers, filtered.ChatTransferHistory...)
	for _, tr := range transfers {
		if members[tr.Source] && tr.Stage == "committed" {
			if err := checkCommitted(tr.Source, tr.Target); err != nil {
				return LogicalChatMembers{}, err
			}
		}
	}
	incoming := map[string]string{}
	for source, target := range committed {
		if prior := incoming[target]; prior != "" && prior != source {
			return LogicalChatMembers{}, errors.New("this chat has conflicting committed predecessors; resolve its ownership before deleting it")
		}
		incoming[target] = source
	}
	// Reject cycles rather than guessing which copy is the person's current chat.
	for source := range committed {
		seen := map[string]bool{}
		for id := source; id != ""; id = committed[id] {
			if seen[id] {
				return LogicalChatMembers{}, errors.New("this chat has a cyclic switch chain; resolve its ownership before deleting it")
			}
			seen[id] = true
		}
	}
	head := ids[0]
	for committed[head] != "" {
		head = committed[head]
	}
	result := LogicalChatMembers{ChatID: ids[0], Head: head, Members: []LogicalChatMember{}, UnresolvedAllocations: []LogicalChatAllocation{}, SeparateRecipients: []LogicalChatRelation{}}
	for _, id := range ids {
		member := LogicalChatMember{SessionID: id, NativeID: id, Provider: s.SessionProviders[id], State: "known_abandoned_candidate"}
		if member.Provider == "" {
			member.Provider = s.ListedChats[id].Provider
		}
		if id == head {
			member.State = "active_head"
		} else if committed[id] != "" {
			member.State = "committed_predecessor"
		}
		for _, tr := range transfers {
			if tr.Target == id {
				member.AttemptID = tr.Revision
				if member.Provider == "" {
					member.Provider = tr.Provider
				}
				if member.CreatedAt.IsZero() || tr.Created.Before(member.CreatedAt) {
					member.CreatedAt = tr.Created
				}
			}
		}
		result.Members = append(result.Members, member)
	}
	for _, tr := range transfers {
		if members[tr.Source] && tr.Target == "" && tr.Stage != "rejected" && tr.Stage != "committed" {
			result.UnresolvedAllocations = append(result.UnresolvedAllocations, LogicalChatAllocation{AttemptID: tr.Revision, Source: tr.Source, Provider: tr.Provider, State: "unknown_allocation_outcome", Status: tr.Stage, CreatedAt: tr.Created, Reason: "The allocation outcome is unknown; no native identity was recorded."})
		}
	}
	// Only relations incident on this chat belong in its ownership snapshot.
	for _, relation := range separate {
		for _, tr := range append(append([]chatTransfer{}, s.ChatTransferHistory...), valuesOfChatTransfers(s.ChatTransfers)...) {
			if tr.Target == relation.SessionID && members[tr.Source] {
				result.SeparateRecipients = append(result.SeparateRecipients, relation)
				break
			}
		}
	}
	sort.Slice(result.UnresolvedAllocations, func(i, j int) bool {
		a, b := result.UnresolvedAllocations[i], result.UnresolvedAllocations[j]
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.Before(b.CreatedAt)
		}
		return a.AttemptID < b.AttemptID
	})
	listed := map[string]listedChat{}
	for _, id := range ids {
		if row, ok := s.ListedChats[id]; ok {
			listed[id] = row
		}
	}
	raw, _ := json.Marshal([]any{s.ChatBindings, s.ChatTransfers, s.ChatTransferHistory, s.BusSessions, result.Members, result.UnresolvedAllocations, listed})
	result.Generation = fmt.Sprintf("%x", sha256.Sum256(raw))
	return result, nil
}

func valuesOfChatTransfers(transfers map[string]chatTransfer) []chatTransfer {
	out := make([]chatTransfer, 0, len(transfers))
	for _, tr := range transfers {
		out = append(out, tr)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Source < out[j].Source })
	return out
}

func (a *App) logicalChatMembers(reference string) (LogicalChatMembers, error) {
	a.mu.Lock()
	snapshot, err := a.settings.clone()
	a.mu.Unlock()
	if err != nil {
		return LogicalChatMembers{}, err
	}
	return logicalChatMembersFromSettings(snapshot, reference)
}
