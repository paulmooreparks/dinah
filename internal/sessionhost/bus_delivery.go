package sessionhost

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"studio/internal/bus"
)

//go:embed bus_delivery.go
var BusDeliverySource []byte

const BusChatLabel = "Messages from the bus"

// BusAskTarget is an already authorized address copied from the host directory.
type BusAskTarget struct{ SessionID, Cwd, Title string }

// askPoll retains the readiness polling interval and selection behavior.
var askPoll = 100 * time.Millisecond

// deliverBusRound preserves the session snapshot and per-session event order.
func (a *BusChats) DeliverRound(ctx context.Context, b *bus.Bus) {
	a.mu.Lock()
	open := make([]*Conversation, 0, a.runtime.ConversationCountLocked())
	for _, s := range a.runtime.ConversationsLocked() {
		open = append(open, s)
	}
	a.mu.Unlock()
	for _, s := range open {
		a.host.TurnIO().NoticeSteers(b.SteerDelivered, s)
		a.Ring(ctx, b, s)
	}
}

// ringChat steers a capable busy chat with the messages waiting that ask it to
// answer, or starts an idle chat with any messages waiting, notices included,
// within the hop limit (bus.Doorbell). The chat is marked working before
// the messages are taken from the bus, so nothing the person sends in the
// meantime can start a second turn, and messages taken are never dropped.
func (a *BusChats) Ring(ctx context.Context, b *bus.Bus, s *Conversation) {
	state := s.State()
	id := state.BusID
	gone := !state.Open && id != ""

	if gone {
		a.LeaveBus(s)
		return
	}
	// While the window waits to restart, messages wait on the bus for the
	// reopened chat (update.go).
	if id == "" || a.host.HoldingTurns() != nil {
		return
	}
	if n, _, err := b.Doorbell(ctx, id); err != nil || n == 0 {
		return
	}
	// A chat still restoring or replaying cannot take them yet; the person
	// sees them arrive meanwhile.
	if s.Receiving() {
		a.announceArriving(ctx, b, s, id)
		return
	}
	submission := s.BeginSubmission()
	defer submission.Release()

	ready, working := s.ClaimDelivery()
	if !ready {
		return
	}

	if working && !a.host.CanSteer(s.ID(), nil) {
		return
	}
	// A notice never interrupts a turn. It waits, with any others that come,
	// and the turn that follows this one takes them together (AfterTurn).
	if working {
		if asking, err := b.Asking(ctx, id); err != nil || asking == 0 {
			return
		}
	}
	msgs, _, err := b.Deliver(ctx, id, bus.DoorbellTurn)
	deliveredNumbers(s, msgs)
	var p Prompt
	if err == nil && len(msgs) > 0 {
		p, err = a.Prompt(s, msgs, working)
	}
	if err != nil || len(msgs) == 0 {
		if !working {
			s.ReleaseWork()
		}

		if err != nil {
			a.host.Log("error", "delivering to chat %s: %v", s.ID(), err)
		}
		return
	}
	if working {
		if err := a.host.TurnIO().SubmitSteer(s, submission, p, p.Text(), nil, p.Pictures()); err != nil {
			a.host.Log("error", "steering messages to chat %s: %v", s.ID(), err)
			return
		}
		return
	}
	EmitSession(a.host.Emit, s)
	EmitChat(a.host.Emit, s, EventSent, SentEvent(s.ID(), p.Text(), "bus", BusChatLabel, p.Delivered()))
	go a.host.TurnIO().Run(s, &p)
}

// Arriving limits: how many waiting messages one event describes, and how
// much of each message's text it carries.
const (
	ArrivingMessages = 5
	ArrivingText     = 400
)

// announceArriving tells the page, once for each message, that messages are
// waiting for a chat that is still restoring or replaying. The event goes
// through the chat's event log, so a window opened during the replay shows
// them too. It changes nothing on the bus.
func (a *BusChats) announceArriving(ctx context.Context, b *bus.Bus, s *Conversation, id string) {
	msgs, more, err := b.Arriving(ctx, id, ArrivingMessages)
	if err != nil {
		a.host.Log("error", "looking for the messages arriving at chat %s: %v", s.ID(), err)
		return
	}
	ids := make([]int64, len(msgs))
	for i, m := range msgs {
		ids[i] = m.ID
	}
	fresh := map[int64]bool{}
	for _, n := range s.Announce(ids) {
		fresh[n] = true
	}
	if len(fresh) == 0 {
		return
	}
	var out []map[string]any
	for _, m := range msgs {
		if !fresh[m.ID] {
			continue
		}
		text := []rune(m.Text)
		if len(text) > ArrivingText {
			text = append(text[:ArrivingText:ArrivingText], '…')
		}
		from := m.From
		if from == "" {
			from = m.FromName
		}
		entry := map[string]any{"id": m.ID, "kind": m.Kind, "from": from, "text": string(text), "sentAt": m.CreatedAt}
		if m.FromWorkspace != "" {
			entry["workspace"] = m.FromWorkspace
		}
		if m.Room != "" {
			entry["group"] = m.RoomTitle
		}
		out = append(out, entry)
	}
	EmitChat(a.host.Emit, s, EventArriving, map[string]any{"sessionId": s.ID(), "messages": out, "more": more})
}

// delivered forgets that the messages were shown as arriving, since a
// delivery has taken them.
func deliveredNumbers(s *Conversation, msgs []bus.Message) {
	ids := make([]int64, len(msgs))
	for i, m := range msgs {
		ids[i] = m.ID
	}
	s.Delivered(ids)
}

// busWaiting takes the messages waiting for a chat as the person speaks to
// it, which also starts the count of automatic handoffs again, as the
// person's prompt does in a Claude Code session. It returns them as the text
// to add to the person's prompt, or "" when none wait, and the entries the
// chat's saved copy keeps for them (delivered.go).
func (a *BusChats) Waiting(s *Conversation) (string, []ConversationMessage) {
	id := s.BusID()
	if id == "" {
		return "", nil
	}
	var msgs []bus.Message
	err := a.host.BusService().With(func(b *bus.Bus) error {
		var err error
		msgs, _, err = b.Deliver(context.Background(), id, "UserPromptSubmit")
		deliveredNumbers(s, msgs)
		return err
	})
	if err != nil {
		a.host.Log("error", "delivering to chat %s: %v", s.ID(), err)
		return "", nil
	}
	if len(msgs) == 0 {
		return "", nil
	}
	return bus.FormatForAgent(msgs, a.host.AgentPerson()), DeliveredEntries(msgs)
}

// busAfterTurn collects waiting messages without interrupting the active
// provider call. A result can therefore run before the next queued prompt.
// Messages arriving after this check are picked up by the normal watcher.
func (a *BusChats) AfterTurn(s *Conversation) *Prompt {
	if a.host.HoldingTurns() != nil {
		return nil
	}
	id := s.BusID()
	if id == "" {
		return nil
	}
	var next *Prompt
	err := a.host.BusService().With(func(b *bus.Bus) error {
		n, _, err := b.Doorbell(context.Background(), id)
		if err != nil || n == 0 {
			return err
		}
		msgs, _, err := b.Deliver(context.Background(), id, bus.DoorbellTurn)
		deliveredNumbers(s, msgs)
		if err != nil || len(msgs) == 0 {
			return err
		}
		p, err := a.Prompt(s, msgs, false)
		if err != nil {
			return err
		}
		next = &p
		return nil
	})
	if err != nil {
		a.host.Log("error", "reading messages after chat %s's turn: %v", s.ID(), err)
	}
	return next
}

// busprompt is the prompt that delivers messages to a chat's agent. The
// pictures the person attached go in it as image blocks where the chat's
// agent takes them (busPictures), and the prompt keeps them as its pictures.
// A prompt steered into a running turn takes them only when the turn can
// take pictures as steering (CanSteer); otherwise they are left listed, as
// every attachment is, for the agent to read with read_attachment.
func (a *BusChats) Prompt(s *Conversation, msgs []bus.Message, steering bool) (Prompt, error) {
	text := bus.FormatForAgent(msgs, a.host.AgentPerson())
	pics, note := a.host.BusPictures(s, msgs)
	if len(pics) > 0 && steering && !a.host.CanSteer(s.ID(), pics) {
		pics, note = nil, ""
	}
	if note != "" {
		text += "\n\n" + note
	}
	p, err := a.host.PreparePrompt(s, text, pics)
	if err != nil {
		return p, err
	}
	p = p.WithDelivery(BusChatLabel, DeliveredEntries(msgs), pics)
	return p, nil
}

// askOverBus asks the chat target the question from the chat caller, with
// the label the asked chat's log shows, and returns the answer and how the
// answering turn ended.
func (a *BusChats) Ask(ctx context.Context, caller *Conversation, target BusAskTarget, label, question string) (string, End) {
	fail := func(err error) (string, End) { return "", End{StopReason: "error", Err: err} }
	from := caller.BusID()
	if from == "" {
		return fail(errors.New("this chat is not on the bus, so it cannot ask another chat; the person can reopen it and ask again"))
	}
	to := ""
	if s := a.session(target.SessionID); s != nil {
		to = s.BusID()
	}
	if to == "" {
		var err error
		if to, err = a.host.EnsureBusChat(target); err != nil {
			return fail(fmt.Errorf("putting the chat on the bus: %w", err))
		}
	}
	posted := a.host.Clock()
	id, err := a.host.BusService().PostQuestion(ctx, from, to, question)
	if err != nil {
		return fail(err)
	}
	defer a.host.BusService().DropQuestion(id)
	s, err := a.AwaitAskable(ctx, target.SessionID, posted)
	if err != nil {
		return fail(err)
	}
	var msg bus.Message
	err = a.host.BusService().With(func(b *bus.Bus) error {
		var err error
		msg, err = b.TakeQuestion(ctx, to, id)
		return err
	})
	if err != nil {
		return fail(err)
	}
	p, err := a.host.PreparePrompt(s, a.host.AskPrompt(label, question), nil)
	if err != nil {
		return fail(err)
	}
	p = p.WithDelivery(label, DeliveredEntries([]bus.Message{msg}), nil)
	return a.host.TurnIO().AskWait(ctx, s, p)
}

// awaitAskable waits until the chat is connected and ready for a question,
// and returns it. It ends early when ctx ends, or when the chat fails to
// open after the question was posted, with the reason.
func (a *BusChats) AwaitAskable(ctx context.Context, chat string, posted time.Time) (*Conversation, error) {
	for {
		if s := a.session(chat); s != nil {
			ready := s.Askable()
			if ready {
				return s, nil
			}
		}
		if err := a.ConnectFailedSince(chat, posted); err != nil {
			return nil, fmt.Errorf("the chat could not be opened: %w", err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(askPoll):
		}
	}
}

// deliveredEntries are the entries for messages delivered in one prompt.
func DeliveredEntries(msgs []bus.Message) []ConversationMessage {
	out := make([]ConversationMessage, 0, len(msgs))
	for _, m := range msgs {
		d := bus.Describe(m)
		out = append(out, ConversationMessage{Role: "delivered", Text: d.Text,
			Delivered: &DeliveredMessage{ID: d.ID, Kind: d.Kind, From: d.From, FromSession: d.FromSession, Workspace: d.Workspace, Time: d.Time, Status: d.Status, Room: m.Room, RoomTitle: m.RoomTitle}})
	}
	return out
}
