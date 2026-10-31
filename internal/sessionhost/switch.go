package sessionhost

import (
	"encoding/json"
	"errors"
	"strings"

	"studio/internal/acp"
)

// A chat's switch is a continuation that the person confirmed: to another provider,
// into another workspace, or a compaction (docs/plans/chat-workspaces-and-switching.md).
// While it runs the chat's information says which stage it is in, in the numbered
// state, so a window opened late or a resync shows the same. When it fails the
// chat is as it was and the information says why, in plain words, until the person
// dismisses it.

// The kinds of switch.
const (
	SwitchProvider   = "provider"
	SwitchWorkspace  = "workspace"
	SwitchCompaction = "compaction"
)

// The stages of a switch, in the order they come.
const (
	StageSummary  = "summary"
	StageStarting = "starting"
	StageHandover = "handover"
	StageOpening  = "opening"
)

// SwitchState is a switch as the chat's information shows it. While a switch runs,
// Stage and Label say where it is; when one failed, Stage is empty and Failed says
// why. To is the name of what the chat is moving to, a provider or a workspace.
type SwitchState struct {
	Kind   string `json:"kind"`
	To     string `json:"to"`
	Stage  string `json:"stage"`
	Label  string `json:"label"`
	Failed string `json:"failed,omitempty"`
}

// SetSwitch records the state of the chat's switch, or with nil that it has none.
func (c *Conversation) SetSwitch(s *SwitchState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if s == nil {
		c.switchState = nil
		return
	}
	copied := *s
	c.switchState = &copied
}

// SwitchState is the chat's switch, or nil.
func (c *Conversation) SwitchState() *SwitchState {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.copySwitchLocked()
}

func (c *Conversation) copyLeadLocked() *LeadRef {
	if c.lead == nil {
		return nil
	}
	copied := *c.lead
	return &copied
}

func (c *Conversation) copySwitchLocked() *SwitchState {
	if c.switchState == nil {
		return nil
	}
	copied := *c.switchState
	return &copied
}

// DismissSwitchFailure clears the failure of a switch that failed. It leaves a
// switch that is running alone.
func (c *Conversation) DismissSwitchFailure() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.switchState != nil && c.switchState.Stage == "" {
		c.switchState = nil
	}
}

// BlockedError says why a continuation cannot start now: a turn is running, or
// something the chat is waiting on must finish. The preview shows it as a fact and
// not as an error, so the confirmation can offer to stop a running turn.
type BlockedError struct{ Reason string }

func (e *BlockedError) Error() string { return e.Reason }

// SwitchEnd is one end of a switch: the provider, its display name, the model, and
// the workspace, by its folder and its name.
type SwitchEnd struct {
	Provider      string `json:"provider"`
	ProviderName  string `json:"providerName"`
	Model         string `json:"model"`
	Workspace     string `json:"workspace"`
	WorkspaceName string `json:"workspaceName"`
}

// HandoffFacts says what the new chat will be given: how many messages the
// conversation has, how many are kept whole, how many are summarised in a few
// lines, the size of the handoff against the provider's budget, and whether a judge
// wrote the summary or the rule did.
type HandoffFacts struct {
	Messages   int  `json:"messages"`
	Whole      int  `json:"whole"`
	Summarised int  `json:"summarised"`
	Chars      int  `json:"chars"`
	Budget     int  `json:"budget"`
	Judged     bool `json:"judged"`
}

// OptionFact is one of the current chat's settings, by its name and its value in
// words.
type OptionFact struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Value string `json:"value"`
}

// RunningFacts is what must finish before a switch can start. CanStop says a running
// turn can be stopped from the confirmation.
type RunningFacts struct {
	Turn      bool `json:"turn"`
	Queued    int  `json:"queued"`
	Approvals bool `json:"approvals"`
	Applets   int  `json:"applets"`
	CanStop   bool `json:"canStop"`
}

// SwitchFacts is what the confirmation of a switch shows, built by the engine. For a
// provider switch every setting of the chat is not carried, since the new provider
// has its own. For a workspace move the settings that do not grant authority are
// carried, and those that do, such as a permission mode, are not: authority given
// in one folder never follows a chat silently into another.
type SwitchFacts struct {
	Kind       string        `json:"kind"`
	From       SwitchEnd     `json:"from"`
	To         SwitchEnd     `json:"to"`
	Handoff    *HandoffFacts `json:"handoff,omitempty"`
	Carried    []OptionFact  `json:"carried"`
	NotCarried []OptionFact  `json:"notCarried"`
	Running    RunningFacts  `json:"running"`
}

// authorityCategory is the category the protocol gives a setting that is a
// permission mode, which grants the agent authority.
const authorityCategory = "mode"

// optionFacts reads a chat's settings as facts.
func optionFacts(options []acp.ConfigOption) []OptionFact {
	out := make([]OptionFact, 0, len(options))
	for _, o := range options {
		out = append(out, OptionFact{ID: o.ID, Name: o.Name, Value: optionValueName(o)})
	}
	return out
}

// optionValueName is a setting's current value as its option list names it, or the
// value itself.
func optionValueName(o acp.ConfigOption) string {
	var value any
	if json.Unmarshal(o.CurrentValue, &value) != nil {
		return strings.TrimSpace(string(o.CurrentValue))
	}
	var choices []struct {
		Value any    `json:"value"`
		Name  string `json:"name"`
	}
	if json.Unmarshal(o.Options, &choices) == nil {
		for _, c := range choices {
			if c.Value == value && c.Name != "" {
				return c.Name
			}
		}
	}
	switch v := value.(type) {
	case string:
		return v
	case nil:
		return ""
	}
	return strings.TrimSpace(string(o.CurrentValue))
}

// splitOptions separates the settings a workspace move carries from those it does
// not, by the category the protocol gives each.
func splitOptions(options []acp.ConfigOption) (carried, notCarried []acp.ConfigOption) {
	for _, o := range options {
		if o.Category == authorityCategory {
			notCarried = append(notCarried, o)
		} else {
			carried = append(carried, o)
		}
	}
	return carried, notCarried
}

var errSameWorkspace = errors.New("the chat is already in that workspace")
