package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"

	"studio/internal/sessionhost"
)

// chatWithEarlierStages gives the members that removing a chat takes: the
// chat's own members (logicalChatMembersFromSettings) and, before them, the
// earlier stages that only its saved conversations link.
//
// A chat switched to another program, compacted or continued elsewhere goes
// on in a new session, and the saved conversation of that session records the
// one it continues. Switches made before bindings were recorded, and stages
// that kept a bus recipient of their own, are linked in no other way. Such a
// stage is not a chat in any list (continuedConversations), so removing the
// chat is the only way a person can remove it. A stage left behind would be
// listed as a chat again as soon as the saved conversation that continues it
// has gone to the Recycle Bin.
//
// Two kinds of earlier stage stay where they are. A stage that another chat
// still goes on from belongs to that chat as well, and the last of them to be
// removed takes it. A stage that already has a removal record is governed by
// that record; when that removal is finished for good, the stages older than
// it are still taken.
//
// The chat's identity and head are unchanged. Only removal uses this; the
// lists and the ownership of a chat still follow the bindings alone.
func chatWithEarlierStages(s Settings, headers map[string]*sessionhost.TranscriptHead, reference string) (LogicalChatMembers, error) {
	result, err := logicalChatMembersFromSettings(s, reference)
	if err != nil {
		return result, err
	}
	hidden := map[string]bool{}
	for _, id := range s.Hidden {
		hidden[id] = true
	}
	children := map[string][]string{}
	for id, header := range headers {
		if header != nil && header.ParentSessionID != "" && header.ParentSessionID != id {
			children[header.ParentSessionID] = append(children[header.ParentSessionID], id)
		}
	}
	// A saved conversation goes on as a chat of its own unless it was deleted,
	// has a removal record, or is the hidden target of a switch that did not
	// finish.
	goesOn := func(id string) bool {
		header := headers[id]
		if header == nil || header.Deleted || hidden[id] {
			return false
		}
		_, reserved := s.RemovedChats[id]
		return !reserved
	}
	taken, passed := map[string]bool{}, map[string]bool{}
	queue := []string{}
	for _, member := range result.Members {
		taken[member.SessionID] = true
		queue = append(queue, member.SessionID)
	}
	stages := []LogicalChatMembers{}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		header := headers[id]
		if header == nil || header.ParentSessionID == "" || header.ParentSessionID == id || taken[header.ParentSessionID] {
			continue
		}
		// A parent with no saved conversation here has nothing left to take.
		if headers[header.ParentSessionID] == nil {
			continue
		}
		stage, err := logicalChatMembersFromSettings(s, header.ParentSessionID)
		if err != nil {
			return LogicalChatMembers{}, err
		}
		own := map[string]bool{}
		for _, member := range stage.Members {
			own[member.SessionID] = true
		}
		stays, gone := false, false
		for _, member := range stage.Members {
			if record, reserved := s.RemovedChats[member.SessionID]; reserved {
				if record.Stage == "removed" || record.Stage == "deleted" {
					gone = true
				} else {
					stays = true
				}
			}
			for _, child := range children[member.SessionID] {
				if !taken[child] && !own[child] && goesOn(child) {
					stays = true
				}
			}
		}
		if stays {
			continue
		}
		if gone {
			// A stage that was removed by itself for good is not taken again,
			// and what is older than it still belongs to this chat.
			for _, member := range stage.Members {
				if !passed[member.SessionID] {
					passed[member.SessionID] = true
					queue = append(queue, member.SessionID)
				}
			}
			continue
		}
		for _, member := range stage.Members {
			taken[member.SessionID] = true
			queue = append(queue, member.SessionID)
		}
		stages = append(stages, stage)
	}
	if len(stages) == 0 {
		return result, nil
	}
	// The stages were found nearest first. Members stay oldest first.
	members := []LogicalChatMember{}
	generations := []string{result.Generation}
	ids := []string{}
	for i := len(stages) - 1; i >= 0; i-- {
		stage := stages[i]
		for _, member := range stage.Members {
			if member.State == "active_head" {
				member.State = "committed_predecessor"
			}
			members = append(members, member)
			ids = append(ids, member.SessionID)
		}
		generations = append(generations, stage.Generation)
		result.UnresolvedAllocations = append(result.UnresolvedAllocations, stage.UnresolvedAllocations...)
		result.SeparateRecipients = append(result.SeparateRecipients, stage.SeparateRecipients...)
	}
	result.Members = append(members, result.Members...)
	sort.SliceStable(result.UnresolvedAllocations, func(i, j int) bool {
		a, b := result.UnresolvedAllocations[i], result.UnresolvedAllocations[j]
		if !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.Before(b.CreatedAt)
		}
		return a.AttemptID < b.AttemptID
	})
	raw, _ := json.Marshal([]any{generations, ids})
	result.Generation = fmt.Sprintf("%x", sha256.Sum256(raw))
	return result, nil
}

func (a *App) chatRemovalMembers(reference string) (LogicalChatMembers, error) {
	a.mu.Lock()
	snapshot, err := a.settings.clone()
	a.mu.Unlock()
	if err != nil {
		return LogicalChatMembers{}, err
	}
	_, headers := sessionhost.ConversationLinks(configDir())
	return chatWithEarlierStages(snapshot, headers, reference)
}
