package sessionhost

import (
	_ "embed"
	"time"
)

//go:embed activity.go
var ActivitySource []byte

// InitialActivity is copied before the conversation is published.
type InitialActivity struct{ Open, Working, Switching bool }

// Activity owns admission, queued input and lifecycle state. Every Locked
// operation requires the same session mutex used by Turns and Details. The
// Conversation supplies that private mutex before publishing the owners.
// No operation calls the host or settles prompt waiters.
type Activity struct {
	turns                                *Turns
	details                              *Details
	open, working, switching, superseded bool
	// queued waits for a normal turn end; steer interrupts and takes priority.
	queued []Prompt
	steer  *Prompt
	// carrying are prompts taken out of the queue to be carried to another chat. They
	// stay in what is kept until the carry is settled, so that a crash while they
	// travel loses none; sink is told what is kept each time it changes.
	carrying             []Prompt
	sink                 func([]StoredPrompt)
	inBackground, parked bool
	// warm marks a background chat opened at start for its group, which
	// takes no background slot (docs/plans/group-reopen.md).
	warm bool
	// announced holds the numbers of the waiting messages already shown as
	// arriving in this chat, until a delivery takes them.
	announced              map[int64]bool
	idleSince, closedSince time.Time
	// ending spans provider close and host cleanup before releasing waiters.
	ending                  *SessionEnd
	busID                   string
	busTitle                *string
	lastSelection, hookTurn string
}

func NewActivity(t *Turns, d *Details, initial InitialActivity) *Activity {
	if t == nil || d == nil || t.mu == nil || t.mu != d.mu {
		panic("sessionhost: activity requires one session mutex")
	}
	return &Activity{turns: t, details: d, open: initial.Open, working: initial.Working, switching: initial.Switching}
}
func (a *Activity) ReadyLocked() bool {
	return a.open && !a.switching && !a.superseded && !a.details.RestoringLocked() && !a.details.ReplayingLocked()
}
func (a *Activity) ReceivingLocked() bool {
	return a.open && !a.parked && !a.superseded && (a.details.RestoringLocked() || a.details.ReplayingLocked())
}
func (a *Activity) OpenLocked() bool           { return a.open }
func (a *Activity) WorkingLocked() bool        { return a.working }
func (a *Activity) SwitchingLocked() bool      { return a.switching }
func (a *Activity) SupersededLocked() bool     { return a.superseded }
func (a *Activity) BackgroundLocked() bool     { return a.inBackground }
func (a *Activity) ParkedLocked() bool         { return a.parked }
func (a *Activity) IdleSinceLocked() time.Time { return a.idleSince }
func (a *Activity) SetSwitchingLocked(v bool)  { a.switching = v }
func (a *Activity) SetSupersededLocked(v bool) { a.superseded = v }
func (a *Activity) CloseLocked()               { a.open = false }
func (a *Activity) StartWorkLocked()           { a.working = true }
func (a *Activity) ReleaseWorkLocked()         { a.working = false }
func (a *Activity) QueueCountLocked() int      { return len(a.queued) }
func (a *Activity) HasSteerLocked() bool       { return a.steer != nil }
func (a *Activity) QueueTextsLocked() []string {
	out := make([]string, len(a.queued))
	for i, p := range a.queued {
		out[i] = p.Text()
	}
	return out
}
func (a *Activity) EnqueueLocked(p Prompt) { a.queued = append(a.queued, p); a.keepLocked() }

// ClearQueueLocked empties the queue, which is the person's doing, so that what is
// kept is emptied too.
func (a *Activity) ClearQueueLocked() []Prompt {
	out := a.dropQueueLocked()
	a.keepLocked()
	return out
}
func (a *Activity) dropQueueLocked() []Prompt {
	out := append([]Prompt(nil), a.queued...)
	a.queued = nil
	return out
}
func (a *Activity) SetInterruptLocked(p *Prompt) {
	if p == nil {
		a.steer = nil
	} else {
		copy := *p
		a.steer = &copy
	}
	a.keepLocked()
}

// keepLocked tells the sink what the queue is now: the steer that waits, then what
// is being carried, then the queue, each with an id of its own, leaving out the
// questions other chats wait on.
func (a *Activity) keepLocked() {
	if a.sink == nil {
		return
	}
	var out []StoredPrompt
	add := func(p *Prompt) {
		if p.qid == "" {
			p.qid = newQueueID()
		}
		if s, ok := p.Stored(); ok {
			out = append(out, s)
		}
	}
	if a.steer != nil {
		add(a.steer)
	}
	for i := range a.carrying {
		add(&a.carrying[i])
	}
	for i := range a.queued {
		add(&a.queued[i])
	}
	a.sink(out)
}

// TakeForCarryLocked takes every queued prompt, to be carried to another chat. They
// are still kept until SettleCarryLocked, or until RestoreQueuedLocked puts them back.
func (a *Activity) TakeForCarryLocked() []Prompt {
	out := a.dropQueueLocked()
	for i := range out {
		if out[i].qid == "" {
			out[i].qid = newQueueID()
		}
	}
	a.carrying = append(a.carrying, out...)
	a.keepLocked()
	return out
}

// SettleCarryLocked says the prompts taken for carrying have been queued in the other
// chat, so that they are no longer kept here.
func (a *Activity) SettleCarryLocked() {
	a.carrying = nil
	a.keepLocked()
}
func (a *Activity) RemoveQuestionLocked(p Prompt) []string {
	for i, q := range a.queued {
		if q.SameReply(p) {
			a.queued = append(a.queued[:i:i], a.queued[i+1:]...)
			break
		}
	}
	return a.QueueTextsLocked()
}
func (a *Activity) AdmitQuestionLocked(p Prompt) (working, occupied bool) {
	working, occupied = a.working, a.turns.AskedLocked() || a.turns.CapturingLocked() || a.turns.ReservedLocked()
	if !working {
		a.working = true
		a.turns.StartCaptureLocked(&p, false)
	}
	return
}
func (a *Activity) ProviderStoppedLocked() []Prompt {
	// The queue of a chat whose provider stopped is dropped from memory but stays kept,
	// to be restored when the chat is opened again.
	out := a.dropQueueLocked()
	a.open, a.working, a.steer = false, false, nil
	a.details.ClearBackgroundLocked()
	return out
}

// NextTurnLocked is the whole final turn transaction. The caller keeps the
// steering gate until Submission.Finish releases it before resolving waiters.
func (a *Activity) NextTurnLocked(messages *Prompt, late []Prompt, advance bool) (next *Prompt, steered bool, queued []string, completion Completion) {
	if messages != nil {
		a.queued = append([]Prompt{*messages}, a.queued...)
	}
	if len(late) > 0 {
		a.queued = append(append([]Prompt(nil), late...), a.queued...)
	}
	a.details.ResetUpdateLocked()
	steer := a.steer
	a.steer = nil
	switch {
	case steer != nil:
		next = steer
	case advance && len(a.queued) > 0:
		p := a.queued[0]
		next = &p
		a.queued = a.queued[1:]
	}
	queued = a.QueueTextsLocked()
	a.keepLocked()
	if next == nil {
		a.working = false
		a.idleSince = time.Now()
	}
	a.turns.StartCaptureLocked(next, false)
	completion = a.turns.DetachReplyLocked()
	return next, steer != nil, queued, completion
}
func (a *Activity) WindowOpenedLocked() bool {
	changed := a.inBackground
	a.inBackground, a.warm = false, false
	a.closedSince = time.Time{}
	return changed
}
func (a *Activity) WarmLocked() bool     { return a.warm }
func (a *Activity) SetWarmLocked(v bool) { a.warm = v }

// AnnounceLocked returns those of the numbers not yet announced as arriving
// and remembers them. A number is forgotten when a delivery takes it.
func (a *Activity) AnnounceLocked(ids []int64) []int64 {
	var fresh []int64
	for _, id := range ids {
		if a.announced[id] {
			continue
		}
		if a.announced == nil {
			a.announced = map[int64]bool{}
		}
		a.announced[id] = true
		fresh = append(fresh, id)
	}
	return fresh
}
func (a *Activity) DeliveredLocked(ids []int64) {
	for _, id := range ids {
		delete(a.announced, id)
	}
}
func (a *Activity) ConnectedBackgroundLocked(now time.Time) {
	a.inBackground = true
	if !a.working {
		a.idleSince = now
	}
}
func (a *Activity) SetBackgroundLocked()             { a.inBackground = true }
func (a *Activity) MarkClosedLocked(now time.Time)   { a.closedSince = now }
func (a *Activity) SetIdleSinceLocked(now time.Time) { a.idleSince = now }
func (a *Activity) ParkLocked() bool {
	if !a.inBackground || a.working {
		return false
	}
	a.inBackground, a.parked, a.warm = false, true, false
	return true
}
func (a *Activity) UnparkLocked() bool {
	if !a.parked {
		return false
	}
	a.parked = false
	a.closedSince = time.Time{}
	return true
}
func (a *Activity) ClosedDueLocked(window bool, now time.Time, after time.Duration) bool {
	switch {
	case !a.ReadyLocked() || a.ending != nil:
	case window || a.inBackground || a.working || a.details.BackgroundCountLocked() > 0:
		a.closedSince = time.Time{}
	case a.closedSince.IsZero():
		a.closedSince = now
	case now.Sub(a.closedSince) >= after:
		return true
	}
	return false
}

// SessionEnd keeps the completion channel private. Wait and Complete retain
// the caller's existing positions relative to session cleanup and events.
type SessionEnd struct{ done chan struct{} }

func (e *SessionEnd) Wait() {
	if e != nil {
		<-e.done
	}
}
func (e *SessionEnd) Complete()               { close(e.done) }
func (a *Activity) EndingLocked() *SessionEnd { return a.ending }
func (a *Activity) BeginEndLocked() *SessionEnd {
	if !a.open || a.working || a.inBackground || a.ending != nil || a.closedSince.IsZero() || a.details.BackgroundCountLocked() > 0 {
		return nil
	}
	e := &SessionEnd{done: make(chan struct{})}
	a.ending = e
	a.open, a.parked = false, false
	return e
}
func (a *Activity) FinishEndLocked() { a.ending = nil; a.closedSince = time.Time{} }
func (a *Activity) AbandonEndLocked() {
	a.open, a.parked, a.ending = true, true, nil
}
func (a *Activity) BusIDLocked() string     { return a.busID }
func (a *Activity) JoinBusLocked(id string) { a.busID, a.busTitle = id, nil }
func (a *Activity) LeaveBusLocked() string  { id := a.busID; a.busID, a.busTitle = "", nil; return id }
func (a *Activity) SameBusTitleLocked(title string) bool {
	return a.busTitle != nil && *a.busTitle == title
}
func (a *Activity) PublishBusTitleLocked(id, title string) {
	if a.busID == id {
		a.busTitle = &title
	}
}
func (a *Activity) SelectionLocked(key string) bool {
	same := key != "" && key == a.lastSelection
	a.lastSelection = key
	return same
}
func (a *Activity) StartHookLocked(token string) string { a.hookTurn = token; return a.busID }
func (a *Activity) TakeHookLocked() (string, string) {
	hook := a.hookTurn
	a.hookTurn = ""
	return hook, a.busID
}
func (a *Activity) HookLocked() string { return a.hookTurn }

// AdmitSendLocked rechecks readiness after prompt construction, then either
// queues behind the active turn or reserves the idle conversation.
func (a *Activity) AdmitSendLocked(p Prompt) (ready, queued bool, texts []string) {
	if !a.ReadyLocked() {
		return false, false, nil
	}
	if a.working {
		a.EnqueueLocked(p)
		return true, true, a.QueueTextsLocked()
	}
	a.working = true
	return true, false, nil
}

// AdmitLoadingSendLocked holds a prompt sent to a chat that is still restoring
// or replaying, in the queue the person sees, until the load ends
// (TakeWaitingLocked). A chat being switched to another provider does not hold
// it.
func (a *Activity) AdmitLoadingSendLocked(p Prompt) (held bool, texts []string) {
	if !a.ReceivingLocked() || a.switching {
		return false, nil
	}
	a.EnqueueLocked(p)
	return true, a.QueueTextsLocked()
}

// HoldLocked puts a prompt that was about to start a turn back at the front of
// the queue and ends the reservation for it (context_hold.go).
func (a *Activity) HoldLocked(p Prompt) {
	a.queued = append([]Prompt{p}, a.queued...)
	a.working = false
	a.keepLocked()
}

// TakeFirstQueuedLocked removes the first queued prompt and returns it.
func (a *Activity) TakeFirstQueuedLocked() *Prompt {
	if len(a.queued) == 0 {
		return nil
	}
	p := a.queued[0]
	a.queued = a.queued[1:]
	a.keepLocked()
	return &p
}

// RestoreQueuedLocked puts prompts taken out of the queue back in front of what is
// queued now, in their order.
func (a *Activity) RestoreQueuedLocked(ps []Prompt) {
	a.queued = append(append([]Prompt(nil), ps...), a.queued...)
	// What was being carried is back in the queue, or in it for the first time.
	a.carrying = nil
	a.keepLocked()
}

// TakeWaitingLocked takes the first prompt held while the chat loaded, once it
// is ready and idle, and reserves the chat for the turn it starts.
func (a *Activity) TakeWaitingLocked() *Prompt {
	if !a.ReadyLocked() || a.working || len(a.queued) == 0 {
		return nil
	}
	p := a.queued[0]
	a.queued = a.queued[1:]
	a.working = true
	a.keepLocked()
	return &p
}

func (a *Activity) AdmitLabelledLocked() bool {
	if a.working || !a.open {
		return false
	}
	a.working = true
	return true
}

// QueueActiveLocked and InterruptActiveLocked report an idle conversation
// separately, since those public callers reconstruct their prompt via Send.
func (a *Activity) QueueActiveLocked(p Prompt) (ready, working bool, texts []string) {
	if !a.ReadyLocked() {
		return false, a.working, nil
	}
	if !a.working {
		return true, false, nil
	}
	a.EnqueueLocked(p)
	return true, true, a.QueueTextsLocked()
}
func (a *Activity) InterruptActiveLocked(p Prompt) (ready, working bool) {
	if !a.ReadyLocked() {
		return false, a.working
	}
	if !a.working {
		return true, false
	}
	a.SetInterruptLocked(&p)
	return true, true
}
func (a *Activity) ClaimDeliveryLocked() (ready, working bool) {
	working = a.working
	if !a.ReadyLocked() {
		return false, working
	}
	if !working {
		a.working = true
	}
	return true, working
}

func (a *Activity) AskableLocked() bool {
	return a.open && !a.parked && a.busID != "" && !a.switching && !a.superseded && !a.details.RestoringLocked() && !a.details.ReplayingLocked()
}
func (a *Activity) ContinuationBusyLocked(allowSwitching bool) bool {
	return !a.open || a.working || a.details.ReplayingLocked() || len(a.queued) > 0 || a.steer != nil || a.turns.CapturingLocked() || (a.switching && !allowSwitching)
}
func (a *Activity) BeginContinuationLocked() bool {
	if a.ContinuationBusyLocked(false) {
		return false
	}
	a.switching = true
	return true
}
func (a *Activity) IdleBackgroundLocked(now time.Time, after time.Duration) bool {
	return a.inBackground && !a.working && !a.idleSince.IsZero() && now.Sub(a.idleSince) >= after
}
