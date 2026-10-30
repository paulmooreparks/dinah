package sessionhost

import (
	_ "embed"
	"encoding/json"
	"slices"

	"studio/internal/acp"
)

//go:embed updates.go
var UpdatesSource []byte

// OptionOverride holds the original and temporarily substituted values.
type OptionOverride struct{ From, To json.RawMessage }

func optionValue(v json.RawMessage) (any, bool) {
	var x any
	if json.Unmarshal(v, &x) != nil {
		return nil, false
	}
	switch x.(type) {
	case string, bool:
		return x, true
	}
	return nil, false
}
func SameOptionValue(x, y json.RawMessage) bool {
	a, okA := optionValue(x)
	b, okB := optionValue(y)
	return okA && okB && a == b
}
func copyOptions(in []acp.ConfigOption) []acp.ConfigOption {
	out := slices.Clone(in)
	for i := range out {
		out[i].CurrentValue = slices.Clone(out[i].CurrentValue)
		out[i].Options = slices.Clone(out[i].Options)
	}
	return out
}
func (s *Details) OptionsLocked() []acp.ConfigOption { return copyOptions(s.options) }
func (s *Details) UpdateOptionsLocked(opts []acp.ConfigOption) (bool, map[string]json.RawMessage) {
	s.options = copyOptions(opts)
	kept := map[string]json.RawMessage{}
	for _, option := range opts {
		if sub, ok := s.substituted[option.ID]; ok {
			if SameOptionValue(option.CurrentValue, sub.To) {
				kept[option.ID] = slices.Clone(sub.From)
			} else {
				delete(s.substituted, option.ID)
			}
		}
	}
	return s.restoringOptions, kept
}
func (s *Details) SetOverridesLocked(in map[string]OptionOverride) {
	s.substituted = make(map[string]OptionOverride, len(in))
	for k, v := range in {
		s.substituted[k] = OptionOverride{slices.Clone(v.From), slices.Clone(v.To)}
	}
}
func (s *Details) ClearOverrideLocked(id string) { delete(s.substituted, id) }
func (s *Details) RestoringLocked() bool         { return s.restoringOptions }
func (s *Details) SetRestoringLocked(v bool)     { s.restoringOptions = v }
func (s *Details) ReplayingLocked() bool         { return s.replaying }
func (s *Details) SetReplayingLocked(v bool)     { s.replaying = v }
func (s *Details) EndReplayLocked()              { s.replaying, s.lastUpdate = false, "" }
func (s *Details) ResetUpdateLocked()            { s.lastUpdate = "" }
func (s *Details) UpdateKindLocked(kind string) bool {
	starts := kind == "agent_message_chunk" && s.lastUpdate != "agent_message_chunk" && !s.replaying
	s.lastUpdate = kind
	return starts
}
func copyUpdates(in []json.RawMessage) []json.RawMessage {
	out := slices.Clone(in)
	for i := range out {
		out[i] = slices.Clone(out[i])
	}
	return out
}

// DrainReplayLocked hands over the queued replayed updates, which the caller
// then owns.
func (s *Details) DrainReplayLocked() []NumberedUpdate {
	out := s.replayed
	s.replayed = nil
	return out
}

// QueueReplayLocked queues replayed updates the chat's log has numbered
// (numberReplayLocked), which the queue then owns.
func (s *Details) QueueReplayLocked(updates []NumberedUpdate, limit int) bool {
	s.replayed = append(s.replayed, updates...)
	return len(s.replayed) >= limit
}
func (s *Details) HoldingReplayLocked() bool       { return s.replayHolding }
func (s *Details) BeginHeldReplayLocked(start int) { s.replayHolding, s.replayHeldStart = true, start }
func (s *Details) HoldReplayLocked(update json.RawMessage) bool {
	if !s.replaying || !s.replayHolding {
		return false
	}
	s.replayHeld = append(s.replayHeld, slices.Clone(update))
	return true
}
func (s *Details) DrainHeldReplayLocked() ([]json.RawMessage, int) {
	out, start := copyUpdates(s.replayHeld), s.replayHeldStart
	s.replayHeld, s.replayHolding = nil, false
	return out, start
}
