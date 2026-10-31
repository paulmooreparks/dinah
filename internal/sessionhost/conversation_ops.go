package sessionhost

import (
	_ "embed"
	"encoding/json"
	"time"

	"studio/internal/acp"
)

// Conversation operations acquire the private mutex once per transition.
// History operations instead require the host transcript mutex, as their
// Locked suffix states; they do not acquire the live conversation mutex.

//go:embed conversation_ops.go
var ConversationOpsSource []byte

func (c *Conversation) Open() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.OpenLocked()
}

func (c *Conversation) Working() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.WorkingLocked()
}

func (c *Conversation) Switching() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.SwitchingLocked()
}

func (c *Conversation) InBackground() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.BackgroundLocked()
}

// Announce returns the message numbers among ids that were not yet shown as
// arriving in this chat, and remembers them.
func (c *Conversation) Announce(ids []int64) []int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.AnnounceLocked(ids)
}

// Delivered forgets that the messages were shown as arriving, since a delivery
// has taken them.
func (c *Conversation) Delivered(ids []int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activity.DeliveredLocked(ids)
}

// Receiving reports whether the chat is open but still restoring its options
// or replaying its saved conversation, which is when a waiting message is
// shown as arriving.
func (c *Conversation) Receiving() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.ReceivingLocked()
}

// UsesBackgroundSlot reports whether the chat is open in the background and
// takes one of the slots for chats opened by a message. A chat opened at start
// because it belongs to a group does not.
func (c *Conversation) UsesBackgroundSlot() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.BackgroundLocked() && !c.activity.WarmLocked()
}

// ConnectedWarm marks the chat as opened at start for its group, in the
// background and outside the slots.
func (c *Conversation) ConnectedWarm(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activity.ConnectedBackgroundLocked(now)
	c.activity.SetWarmLocked(true)
}

func (c *Conversation) Parked() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.ParkedLocked()
}

func (c *Conversation) IdleSince() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.IdleSinceLocked()
}

func (c *Conversation) SetSwitching(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activity.SetSwitchingLocked(v)
}

func (c *Conversation) SetSuperseded(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activity.SetSupersededLocked(v)
}

func (c *Conversation) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activity.CloseLocked()
}

func (c *Conversation) StartWork() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activity.StartWorkLocked()
}

func (c *Conversation) ReleaseWork() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activity.ReleaseWorkLocked()
}

func (c *Conversation) QueueCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.QueueCountLocked()
}

func (c *Conversation) Enqueue(p Prompt) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activity.EnqueueLocked(p)
}

func (c *Conversation) ClearQueue() []Prompt {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.ClearQueueLocked()
}

func (c *Conversation) SetInterrupt(p *Prompt) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activity.SetInterruptLocked(p)
}

func (c *Conversation) RemoveQuestion(p Prompt) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.RemoveQuestionLocked(p)
}

func (c *Conversation) ProviderStopped() []Prompt {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.ProviderStoppedLocked()
}

func (c *Conversation) NextTurn(messages *Prompt, late []Prompt, advance bool) (next *Prompt, steered bool, queued []string, completion Completion) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.NextTurnLocked(messages, late, advance)
}

func (c *Conversation) WindowOpened() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.WindowOpenedLocked()
}

func (c *Conversation) ConnectedBackground(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activity.ConnectedBackgroundLocked(now)
}

func (c *Conversation) SetIdleSince(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activity.SetIdleSinceLocked(now)
}

func (c *Conversation) Unpark() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.UnparkLocked()
}

func (c *Conversation) ClosedDue(window bool, now time.Time, after time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.ClosedDueLocked(window, now, after)
}

func (c *Conversation) Ending() *SessionEnd {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.EndingLocked()
}

func (c *Conversation) BeginEnd() *SessionEnd {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.BeginEndLocked()
}

func (c *Conversation) FinishEnd() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activity.FinishEndLocked()
}

func (c *Conversation) BusID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.BusIDLocked()
}

func (c *Conversation) JoinBus(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activity.JoinBusLocked(id)
}

func (c *Conversation) LeaveBus() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.LeaveBusLocked()
}

func (c *Conversation) PublishBusTitle(id, title string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activity.PublishBusTitleLocked(id, title)
}

func (c *Conversation) Selection(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.SelectionLocked(key)
}

func (c *Conversation) StartHook(token string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.StartHookLocked(token)
}

func (c *Conversation) TakeHook() (string, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.TakeHookLocked()
}

func (c *Conversation) Hook() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.HookLocked()
}

func (c *Conversation) AdmitSend(p Prompt) (ready, queued bool, texts []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.AdmitSendLocked(p)
}

func (c *Conversation) AdmitLoadingSend(p Prompt) (held bool, texts []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.AdmitLoadingSendLocked(p)
}

func (c *Conversation) TakeWaiting() *Prompt {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.TakeWaitingLocked()
}

func (c *Conversation) AdmitLabelled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.AdmitLabelledLocked()
}

func (c *Conversation) QueueActive(p Prompt) (ready, working bool, texts []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.QueueActiveLocked(p)
}

func (c *Conversation) InterruptActive(p Prompt) (ready, working bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.InterruptActiveLocked(p)
}

func (c *Conversation) ClaimDelivery() (ready, working bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.ClaimDeliveryLocked()
}

func (c *Conversation) Askable() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.AskableLocked()
}

func (c *Conversation) ContinuationBusy(allowSwitching bool) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.ContinuationBusyLocked(allowSwitching)
}

func (c *Conversation) BeginContinuation() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.BeginContinuationLocked()
}

func (c *Conversation) IdleBackground(now time.Time, after time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activity.IdleBackgroundLocked(now, after)
}

func (c *Conversation) BackgroundUpdate(update json.RawMessage) bool {
	return c.details.BackgroundUpdate(update)
}

func (c *Conversation) BackgroundTask(id string) (BackgroundTask, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.details.BackgroundTaskLocked(id)
}

func (c *Conversation) HistoryPrepareReplayLocked() (ConversationMessage, bool) {
	return c.history.PrepareReplayLocked()
}

func (c *Conversation) HistoryFinishReplayLocked(dir string, succeeded bool) {
	c.history.FinishReplayLocked(dir, succeeded)
}

// HistoryRecordToolLocked records a tool call or an update of one as the
// history does for the Claude adapter (history.go, RecordToolLocked).
func (c *Conversation) HistoryRecordToolLocked(replay bool, id, callID string, isCall bool, title, status string, content []string, unsupported bool) {
	c.history.RecordToolLocked(replay, id, callID, isCall, title, status, content, unsupported)
}

func (c *Conversation) HistoryRecordSnapshotToolLocked(replay bool, id, callID string, snapshot bool, title, status string, content []string, unsupported bool, visible func(string) string) {
	c.history.RecordSnapshotToolLocked(replay, id, callID, snapshot, title, status, content, unsupported, visible)
}

func (c *Conversation) HistoryRecordMessageLocked(replay bool, id, role, text string, unsupported, merge bool) {
	c.history.RecordMessageLocked(replay, id, role, text, unsupported, merge)
}

func (c *Conversation) HistoryRecordStatusLocked(replay bool, id, text, kind string) {
	c.history.RecordStatusLocked(replay, id, text, kind)
}

func (c *Conversation) HistoryRecordImageLocked(replay bool, id string, image acp.ContentBlock) {
	c.history.RecordImageLocked(replay, id, image)
}

func (c *Conversation) HistoryRestoreDeliveryLocked(id string, held []json.RawMessage, start int) []json.RawMessage {
	return c.history.RestoreDeliveryLocked(id, held, start)
}

func (c *Conversation) HistorySavedLocked() *Transcript {
	return c.history.SavedLocked()
}

func (c *Conversation) HistoryErrorLocked() error {
	return c.history.ErrorLocked()
}

func (c *Conversation) HistoryReplayLengthLocked() int {
	return c.history.ReplayLengthLocked()
}

func (c *Conversation) HistoryRestoreSavedLocked(tr *Transcript) {
	c.history.RestoreSavedLocked(tr)
}

func (c *Conversation) HistoryReplaceDeletedLocked(tr *Transcript) {
	c.history.ReplaceDeletedLocked(tr)
}

func (c *Conversation) HistoryConsumeHandoffLocked(dir string) {
	c.history.ConsumeHandoffLocked(dir)
}

func (c *Conversation) HistoryRecordSentLocked(dir, id string, d DeliveredMessage, text string) bool {
	return c.history.RecordSentLocked(dir, id, d, text)
}

func (c *Conversation) HistoryFinishPromptLocked(dir string, blocks []acp.ContentBlock, queueID string) {
	c.history.FinishPromptLocked(dir, blocks, queueID)
}

// HistoryThroughLocked is the generation of the chat's log and the number of
// the entry its transcript file is consistent with, and false before the file
// has been written since the chat was opened. The caller holds the history's
// mutex.
func (c *Conversation) HistoryThroughLocked() (string, uint64, bool) {
	n, ok := c.history.ThroughLocked()
	return c.log.gen, n, ok
}

func (c *Conversation) HistoryFinishTurnLocked(dir string) {
	c.history.FinishTurnLocked(dir)
}

func (c *Conversation) HistoryRecordDeliveredLocked(id, prompt string, entries []ConversationMessage) {
	c.history.RecordDeliveredLocked(id, prompt, entries)
}

func (c *Conversation) HistoryBeginHandoffLocked(dir, parent, parentWorkspace, context, summary string, history []ConversationMessage) error {
	return c.history.BeginHandoffLocked(dir, parent, parentWorkspace, context, summary, history)
}

func (c *Conversation) SubagentFamily(id string) map[string]bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.details.SubagentFamilyLocked(id)
}

func (c *Conversation) Subagent(id string) (SubagentInfo, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.details.SubagentLocked(id)
}

func (c *Conversation) SpawnSubagent(rootID, parent, id, name, task, prompt string, canCancel, seen bool, started, ended string) SubagentInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.details.SpawnSubagentLocked(rootID, parent, id, name, task, prompt, canCancel, seen, started, ended)
}

func (c *Conversation) EndSubagent(id, state string) (SubagentInfo, map[string]bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.details.EndSubagentLocked(id, state)
}

func (c *Conversation) DisconnectSubagents(now string) []SubagentInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.details.DisconnectSubagentsLocked(now)
}

func (c *Conversation) SubagentRecord(id string) (SubagentRecord, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.details.SubagentRecordLocked(id)
}

func (c *Conversation) BeginSubmission() *Submission {
	return c.turns.Begin()
}

func (c *Conversation) TrySubmission() *Submission {
	return c.turns.TryBegin()
}

func (c *Conversation) Capturing() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.turns.CapturingLocked()
}

func (c *Conversation) Asked() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.turns.AskedLocked()
}

func (c *Conversation) StartCapture(p *Prompt, reset bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.turns.StartCaptureLocked(p, reset)
}

func (c *Conversation) ResetCapture() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.turns.ResetCaptureLocked()
}

func (c *Conversation) AppendCapture(text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.turns.AppendCaptureLocked(text)
}

func (c *Conversation) Reserve(p Prompt) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.turns.ReserveLocked(p)
}

func (c *Conversation) ClearReservation() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.turns.ClearReservationLocked()
}

func (c *Conversation) MarkDelivered(p Prompt) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.turns.MarkDeliveredLocked(p)
}

func (c *Conversation) PendingCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.turns.PendingCountLocked()
}

func (c *Conversation) Options() []acp.ConfigOption {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.details.OptionsLocked()
}

func (c *Conversation) UpdateOptions(opts []acp.ConfigOption) (bool, map[string]json.RawMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.details.UpdateOptionsLocked(opts)
}

func (c *Conversation) SetOverrides(in map[string]OptionOverride) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.details.SetOverridesLocked(in)
}

func (c *Conversation) ClearOverride(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.details.ClearOverrideLocked(id)
}

func (c *Conversation) SetRestoring(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.details.SetRestoringLocked(v)
}

func (c *Conversation) Replaying() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.details.ReplayingLocked()
}

func (c *Conversation) SetReplaying(v bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.details.SetReplayingLocked(v)
}

func (c *Conversation) EndReplay() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.details.EndReplayLocked()
}

func (c *Conversation) UpdateKind(kind string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.details.UpdateKindLocked(kind)
}

func (c *Conversation) DrainReplay() []NumberedUpdate {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.details.DrainReplayLocked()
}

// QueueReplay numbers replayed updates in the chat's log, in the order given,
// and queues them for the page. Numbering and queuing happen under one hold
// of mu, so the numbers follow the order the queue sends them in.
func (c *Conversation) QueueReplay(updates []json.RawMessage, limit int) bool {
	c.mu.Lock()
	full := c.details.QueueReplayLocked(c.numberReplayLocked(updates), limit)
	c.mu.Unlock()
	c.trimChatLogs()
	return full
}

func (c *Conversation) BeginHeldReplay(start int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.details.BeginHeldReplayLocked(start)
}

func (c *Conversation) HoldReplay(update json.RawMessage) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.details.HoldReplayLocked(update)
}

func (c *Conversation) DrainHeldReplay() ([]json.RawMessage, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.details.DrainHeldReplayLocked()
}
