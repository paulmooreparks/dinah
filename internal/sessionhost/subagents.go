package sessionhost

import (
	_ "embed"
	"encoding/json"
	"sort"
	"time"
)

//go:embed subagents.go
var SubagentsSource []byte

// SubagentUpdatesKept, SubagentBytesKept and SubagentBuffersKept bound what
// Dinah keeps of the subagents' updates: a subagent's latest 4,000 updates or
// 4 MiB of them, whichever is less, with the consecutive pieces of one
// message or thought joined into one update, and the updates of a chat's 64
// latest subagents. An older subagent keeps its place in the tree.
const (
	SubagentUpdatesKept = 4000
	SubagentBytesKept   = 4 << 20
	SubagentBuffersKept = 64
)

// The states of a subagent. Running is what spawning means; the others are
// terminal. Disconnected means the outcome is unknown.
const (
	subagentRunning      = "running"
	subagentDisconnected = "disconnected"
)

func terminalSubagentState(state string) bool {
	switch state {
	case "completed", "failed", "cancelled", subagentDisconnected:
		return true
	}
	return false
}

// subagent is one subagent of a chat, with its kept updates.
type subagent struct {
	info   SubagentInfo
	prompt string
	order  int64
	buf    []subagentEntry
	bytes  int
	// seq counts the updates the subagent has had, so that the page can
	// tell which live updates its copy of the kept ones already holds.
	seq int64
}

// subagentEntry is one kept update. A message or thought arrives in many
// small pieces; consecutive pieces of text are kept as one, in chunk, with
// the kind of update they are.
type subagentEntry struct {
	raw   json.RawMessage
	kind  string
	chunk []byte
}

func (e subagentEntry) size() int { return len(e.raw) + len(e.chunk) }

func (e subagentEntry) update() json.RawMessage {
	if e.kind == "" {
		return e.raw
	}
	out, _ := json.Marshal(map[string]any{"sessionUpdate": e.kind, "content": map[string]string{"type": "text", "text": string(e.chunk)}})
	return out
}

// chunkText is the text of a piece of a message or thought, and whether the
// update is one.
func chunkText(kind string, update json.RawMessage) (string, bool) {
	if kind != "agent_message_chunk" && kind != "agent_thought_chunk" {
		return "", false
	}
	var c struct {
		Content struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if json.Unmarshal(update, &c) != nil || c.Content.Type != "text" {
		return "", false
	}
	return c.Content.Text, true
}

// trimSubagentBuffersLocked lets go of the updates of the chat's oldest
// finished subagents beyond the latest SubagentBuffersKept. The caller holds
// s.mu.
func (s *Details) trimSubagentBuffersLocked() {
	var kept []*subagent
	for _, sa := range s.subagents {
		if sa.buf != nil {
			kept = append(kept, sa)
		}
	}
	if len(kept) <= SubagentBuffersKept {
		return
	}
	sort.Slice(kept, func(i, j int) bool { return kept[i].order < kept[j].order })
	for _, sa := range kept[:len(kept)-SubagentBuffersKept] {
		if sa.info.State == subagentRunning {
			continue
		}
		sa.buf, sa.bytes, sa.info.Trimmed = nil, 0, true
	}
}

// SubagentFamilyLocked is a subagent and all its descendants. The caller
// holds s.mu.
func (s *Details) SubagentFamilyLocked(id string) map[string]bool {
	out := map[string]bool{id: true}
	for grew := true; grew; {
		grew = false
		for cid, sa := range s.subagents {
			if !out[cid] && out[sa.info.ParentID] {
				out[cid], grew = true, true
			}
		}
	}
	return out
}

// subagentList is the chat's subagents in the order they began, without
// their prompts. The caller holds s.mu.
func (s *Details) SubagentListLocked() []SubagentInfo {
	all := make([]*subagent, 0, len(s.subagents))
	for _, sa := range s.subagents {
		all = append(all, sa)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].order < all[j].order })
	out := make([]SubagentInfo, len(all))
	for i, sa := range all {
		out[i] = sa.info
	}
	return out
}

// runningSubagents counts the chat's subagents still running. The caller
// holds s.mu.
func (s *Details) RunningSubagentsLocked() int {
	n := 0
	for _, sa := range s.subagents {
		if sa.info.State == subagentRunning {
			n++
		}
	}
	return n
}

func (s *Details) SubagentLocked(id string) (SubagentInfo, bool) {
	sa := s.subagents[id]
	if sa == nil {
		return SubagentInfo{}, false
	}
	return sa.info, true
}
func (s *Details) SpawnSubagentLocked(rootID, parent, id, name, task, prompt string, canCancel, seen bool, started, ended string) SubagentInfo {
	sa := s.subagents[id]
	if sa == nil {
		s.subagentSerial++
		sa = &subagent{order: s.subagentSerial, info: SubagentInfo{ID: id, ParentID: parent, RootID: rootID, State: subagentRunning}}
		switch {
		case !s.replaying:
			sa.info.Started, sa.info.ThisRun = time.Now().UTC().Format(time.RFC3339Nano), true
		case seen:
			sa.info.Started, sa.info.Ended, sa.info.ThisRun = started, ended, true
		default:
			sa.info.Replayed = true
		}
		if s.subagents == nil {
			s.subagents = map[string]*subagent{}
		}
		s.subagents[id] = sa
	}
	// A second announcement of a subagent that has ended does not make it
	// run again.
	sa.info.Name, sa.info.Task, sa.info.CanCancel, sa.prompt = name, task, canCancel, prompt
	info := sa.info
	return info
}

func (s *Details) EndSubagentLocked(id, state string) (SubagentInfo, map[string]bool, bool) {
	if !terminalSubagentState(state) {
		return SubagentInfo{}, nil, false
	}
	sa := s.subagents[id]
	if sa == nil || (sa.info.State != subagentRunning && !sa.info.Local) {
		return SubagentInfo{}, nil, false
	}
	sa.info.State, sa.info.Local = state, false
	// A replay says how a subagent ended but not when.
	if sa.info.Ended == "" && !sa.info.Replayed && !s.replaying {
		sa.info.Ended = time.Now().UTC().Format(time.RFC3339Nano)
	}
	info := sa.info
	ids := s.SubagentFamilyLocked(id)
	return info, ids, true
}

func (s *Details) DisconnectSubagentsLocked(now string) []SubagentInfo {
	var changed []SubagentInfo
	for _, sa := range s.subagents {
		if sa.info.State != subagentRunning {
			continue
		}
		sa.info.State, sa.info.Local = subagentDisconnected, true
		if !sa.info.Replayed {
			sa.info.Ended = now
		}
		changed = append(changed, sa.info)
	}
	return changed
}
func (s *Details) KeepSubagentUpdateLocked(id, kind string, update json.RawMessage) (int64, bool) {
	sa := s.subagents[id]
	if sa == nil {
		return 0, false
	}
	sa.seq++
	seq := sa.seq
	first := len(sa.buf) == 0
	text, joinable := chunkText(kind, update)
	if last := len(sa.buf) - 1; joinable && last >= 0 && sa.buf[last].kind == kind {
		sa.buf[last].chunk = append(sa.buf[last].chunk, text...)
		sa.bytes += len(text)
	} else {
		e := subagentEntry{raw: append(json.RawMessage(nil), update...)}
		if joinable {
			e = subagentEntry{kind: kind, chunk: []byte(text)}
		}
		sa.buf = append(sa.buf, e)
		sa.bytes += e.size()
	}
	for len(sa.buf) > 1 && (len(sa.buf) > SubagentUpdatesKept || sa.bytes > SubagentBytesKept) {
		sa.bytes -= sa.buf[0].size()
		sa.buf[0] = subagentEntry{}
		sa.buf = sa.buf[1:]
		sa.info.Trimmed = true
	}
	if first {
		s.trimSubagentBuffersLocked()
	}
	return seq, true
}

func (s *Details) SubagentRecordLocked(id string) (SubagentRecord, bool) {
	sa := s.subagents[id]
	if sa == nil {
		return SubagentRecord{}, false
	}
	rec := SubagentRecord{Subagent: sa.info, Seq: sa.seq, Updates: make([]json.RawMessage, len(sa.buf))}
	rec.Subagent.Prompt = sa.prompt
	for i, e := range sa.buf {
		rec.Updates[i] = append(json.RawMessage(nil), e.update()...)
	}
	return rec, true
}
