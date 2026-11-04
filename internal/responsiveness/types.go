// Package responsiveness retains bounded, content-free timing observations.
package responsiveness

import "time"

const (
	Period       = time.Second
	Capacity     = 600
	StorageLimit = 512 << 10
	Measured     = "measured"
	Unavailable  = "unavailable"
	NotSupported = "not_supported"
)

type Number struct {
	State string   `json:"state"`
	Value *float64 `json:"value"`
}

func number(v float64) Number     { return Number{Measured, &v} }
func missing(state string) Number { return Number{State: state} }

type Clock struct {
	UTC    time.Time `json:"utc"`
	MonoMS float64   `json:"monoMs"`
}
type Span struct {
	Anchor     Clock   `json:"anchor"`
	StartMS    float64 `json:"startMonoMs"`
	DurationMS float64 `json:"durationMs"`
	Complete   bool    `json:"complete"`
}
type State struct {
	Enabled    bool   `json:"enabled"`
	Generation uint64 `json:"generation"`
	Anchor     Clock  `json:"anchor"`
	State      string `json:"state"`
}
type Stats struct {
	Count     Number `json:"count"`
	Total     Number `json:"totalMs"`
	Max       Number `json:"maxMs"`
	Offset    Number `json:"maxCompletedOffsetMs"`
	Saturated bool   `json:"saturated"`
}
type CategoryStats struct {
	App    Stats `json:"appEmit"`
	Bridge Stats `json:"bridgeEnqueue"`
}
type Categories struct {
	Text  CategoryStats `json:"agent_text"`
	Tool  CategoryStats `json:"agent_tool"`
	Chat  CategoryStats `json:"chat_state"`
	Team  CategoryStats `json:"team_state"`
	Bus   CategoryStats `json:"bus_change"`
	Other CategoryStats `json:"other"`
}
type Queues struct {
	Subscribers Number `json:"subscriberPeak"`
	Depth       Number `json:"depthPeak"`
	Drops       Number `json:"overflowDisconnects"`
}
type RuntimeInterval struct {
	Scope   string `json:"scope"`
	Time    Span   `json:"time"`
	GCCount Number `json:"gcPauseCount"`
	GCLower Number `json:"gcPauseMaxBucketLowerMs"`
	GCUpper Number `json:"gcPauseMaxBucketUpperMs"`
	GCOpen  bool   `json:"gcPauseUpperOpen"`
	Mutex   Number `json:"mutexWaitMs"`
}
type Slot struct {
	Time       Span            `json:"time"`
	Categories Categories      `json:"categories"`
	Queues     Queues          `json:"queues"`
	Runtime    RuntimeInterval `json:"runtime"`
}
type Coverage struct {
	Retained    int    `json:"retainedSlots"`
	Overwritten uint64 `json:"overwrittenSlots"`
	Missed      uint64 `json:"missedPeriods"`
	Disabled    Number `json:"disabledObservations"`
}
type Snapshot struct {
	Version   int      `json:"schemaVersion"`
	Recording State    `json:"recording"`
	Captured  Clock    `json:"capturedAt"`
	Coverage  Coverage `json:"coverage"`
	Slots     []Slot   `json:"slots"`
}

func DisabledState() State { return State{Anchor: Clock{UTC: time.Now().UTC()}, State: "off"} }
func DisabledSnapshot() Snapshot {
	state := DisabledState()
	return Snapshot{Version: 1, Recording: state, Captured: state.Anchor, Coverage: Coverage{Disabled: missing(Unavailable)}, Slots: []Slot{}}
}

type Category uint8

const (
	Text Category = iota
	Tool
	Chat
	Team
	Bus
	Other
	categoryCount
)

type Kind uint8

const (
	App Kind = iota
	Bridge
)

// Unknown names, including generic update envelopes without a typed event
// name, stay Other. We never inspect or retain event content to guess a class.
func Classify(name string) Category {
	switch name {
	case "agent:message_chunk", "agent:text":
		return Text
	case "agent:tool_call", "agent:tool_call_update":
		return Tool
	case "agent:chatstate", "agent:session", "agent:state", "agent:queue", "agent:turn":
		return Chat
	case "team:state":
		return Team
	case "bus:changed":
		return Bus
	default:
		return Other
	}
}
