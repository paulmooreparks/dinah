package sessionhost

import (
	_ "embed"
	"encoding/json"

	"studio/internal/acp"
)

//go:embed conversation_state.go
var ConversationStateSource []byte

// State copies the observations used together by host decisions.
type State struct {
	Info
	Ready, Superseded, Switching, Replaying, Restoring, HoldingReplay bool
	Asked, Reserved, HasSteer                                         bool
	BackgroundCount, RunningSubagents                                 int
	BusID                                                             string
}

func (c *Conversation) State() State {
	c.mu.Lock()
	defer c.mu.Unlock()
	return State{Info: c.infoLocked(), Ready: c.activity.ReadyLocked(), Superseded: c.activity.SupersededLocked(), Switching: c.activity.SwitchingLocked(), Replaying: c.details.ReplayingLocked(), Restoring: c.details.RestoringLocked(), HoldingReplay: c.details.HoldingReplayLocked(), Asked: c.turns.AskedLocked(), Reserved: c.turns.ReservedLocked(), HasSteer: c.activity.HasSteerLocked(), BackgroundCount: c.details.BackgroundCountLocked(), RunningSubagents: c.details.RunningSubagentsLocked(), BusID: c.activity.BusIDLocked()}
}
func (c *Conversation) StartTurn(p *Prompt) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.details.ResetUpdateLocked()
	c.turns.StartCaptureLocked(p, true)
}
func (c *Conversation) AdmitReadyQuestion(p Prompt) (ready, working, occupied bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.activity.ReadyLocked() {
		return false, false, false
	}
	working, occupied = c.activity.AdmitQuestionLocked(p)
	return true, working, occupied
}
func (c *Conversation) EnqueueQuestion(p Prompt, working bool) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !working {
		return nil
	}
	c.activity.EnqueueLocked(p)
	return c.activity.QueueTextsLocked()
}
func (c *Conversation) BeginSwitch() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.activity.SwitchingLocked() {
		return false
	}
	c.activity.SetSwitchingLocked(true)
	return true
}

// QueueIfReplaying numbers and queues an update while the chat replays its
// history, under one hold of mu (QueueReplay).
func (c *Conversation) QueueIfReplaying(update json.RawMessage, limit int) (replay, full bool) {
	c.mu.Lock()
	if !c.details.ReplayingLocked() {
		c.mu.Unlock()
		return false, false
	}
	full = c.details.QueueReplayLocked(c.numberReplayLocked([]json.RawMessage{update}), limit)
	c.mu.Unlock()
	c.trimChatLogs()
	return true, full
}
func (c *Conversation) OptionForChange(id string) (loading bool, opt *acp.ConfigOption) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.details.RestoringLocked() || c.details.ReplayingLocked() {
		return true, nil
	}
	for _, o := range c.details.OptionsLocked() {
		if o.ID == id {
			v := o
			opt = &v
		}
	}
	return false, opt
}
func (c *Conversation) BusTitleState(title string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.BusIDLocked(), c.activity.SameBusTitleLocked(title)
}
func (c *Conversation) KeepSubagentEvent(id, kind string, update json.RawMessage) (int64, bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	seq, found := c.details.KeepSubagentUpdateLocked(id, kind, update)
	return seq, found, c.details.ReplayingLocked()
}
