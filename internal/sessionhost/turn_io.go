package sessionhost

import (
	"context"
	"crypto/rand"
	_ "embed"
	"errors"
	"strings"
	"sync"
	"time"

	"studio/internal/acp"
	"studio/internal/bus"
)

//go:embed turn_io.go
var TurnIOSource []byte

// TurnIO orchestrates protocol and host effects over opaque conversations.
// It holds no registry of its own. Host callbacks must not re-enter a held
// submission gate; receipt confirmation remains independent of that gate.
type TurnIO struct {
	host    TurnHost
	running sync.WaitGroup
}

// start runs a turn on its own goroutine, which Wait counts.
func (t *TurnIO) start(s *Conversation, next *Prompt) {
	t.running.Add(1)
	go func() {
		defer t.running.Done()
		t.Run(s, next)
	}()
}

// Wait returns when every turn started so far has ended, or when the limit passes, and says whether they all ended.
// An app that is ending, or a test that is, waits here so that no turn writes into a folder after it is gone.
func (t *TurnIO) Wait(limit time.Duration) bool {
	done := make(chan struct{})
	go func() { t.running.Wait(); close(done) }()
	select {
	case <-done:
		return true
	case <-time.After(limit):
		return false
	}
}

func NewTurnIO(host TurnHost) *TurnIO { return &TurnIO{host: host} }

type TurnHost struct {
	LookupConversation  func(string) *Conversation
	SessionConnection   func(*Conversation) Connection
	HoldingTurns        func() error
	ResolveContinuation func(string) (string, error)
	MakePrompt          func(*Conversation, string, *DocRef, []Attachment) (Prompt, error)
	PromptFrom          func(*Conversation, string, []Attachment) (Prompt, error)
	DraftDocRef         func(*DocRef) *DocRef
	WithDoc             func(string, *DocRef) string
	Remember            func(string, string, string, string, *DocRef)
	Unpark              func(*Conversation)
	Journal             func(string, string)
	RecordPrompt        func(*Conversation, *Prompt)
	LogPrompt           func(string, string, string)
	RecordDelivered     func(*Conversation, string, []ConversationMessage)
	FinishTranscript    func(*Conversation, error, string)
	// CommitTurn appends the turn's event to the chat's log and writes the saved
	// copy in one critical section (transcript_capture.go).
	CommitTurn                 func(*Conversation, map[string]any) *LoggedEvent
	DisconnectSubagents        func(*Conversation)
	CancelQuestions            func(string)
	Emit                       func(string, any)
	PrepareProviderApplets     func(string, string) error
	AppletPromptContext        func(string) (string, error)
	PendingContinuationContext func(*Conversation) (string, error)
	ConsumeContinuationContext func(*Conversation)
	Call                       func(Connection, string, any, any) error
	BusWaiting                 func(*Conversation) (string, []ConversationMessage)
	BusAfterTurn               func(*Conversation) *Prompt
	HooksReach                 func(string) (bool, error)
	QueueHookSteer             func(string, string) (int64, error)
	WithdrawHookSteer          func(int64) (bool, error)
	BeginHookTurn              func(string, string) error
	EndHookTurn                func(string, string) error
	SaveSteering               func(*Conversation, string, *DocRef, []Attachment, Prompt) (int64, error)
	SteeringState              func(string, int64, string)
	LogError                   func(string, string, ...any)
	VisibleConversationText    func(string) string
	// ContextGate says a prompt that is about to start a turn is held, having
	// put it back in the chat's queue, because the chat's window is full.
	ContextGate func(*Conversation, *Prompt) bool
}

const busChatLabel = "Messages from the bus"

// Send gives a session a prompt, with the document the person picked and
// the attachments they added, if any. It returns at once; the turn's
// progress arrives as events and its end as agent:turn. While a turn runs,
// the message is queued instead, as Queue does. Each message goes into the
// prompt history as the person typed it.
func (t *TurnIO) Send(id, text string, doc *DocRef, atts []Attachment) error {
	text = strings.TrimSpace(text)
	if text == "" && len(atts) == 0 {
		return nil
	}
	// No new turn starts while the window waits to restart (update.go);
	// messages already queued still run.
	if err := t.host.HoldingTurns(); err != nil {
		return err
	}
	if current, err := t.host.ResolveContinuation(id); err != nil {
		return err
	} else if current != id {
		return errors.New("this chat continues with another provider; open its current conversation before sending")
	}
	s := t.host.LookupConversation(id)
	if s == nil {
		return errors.New("that session is not open")
	}

	state := s.State()
	if !state.Ready {
		return t.holdWhileLoading(s, text, doc, atts)
	}
	if state.Working {
		return t.Queue(id, text, doc, atts)
	}

	p, err := t.host.MakePrompt(s, text, doc, atts)
	if err != nil {
		return err
	}
	ready, queued, _ := s.AdmitSend(p)
	if !ready {
		return errors.New("wait for the provider change to finish")
	}
	if queued {
		t.host.Remember(s.Cwd(), s.ID(), "queue", text, doc)
		EmitQueue(t.host.Emit, s)
		return nil
	}

	t.host.Remember(s.Cwd(), s.ID(), "send", text, doc)
	EmitSession(t.host.Emit, s)
	t.start(s, &p)
	return nil
}

// holdWhileLoading takes a message sent to a chat that is still restoring or
// replaying its history. The message waits in the chat's queue, which the page
// shows, and StartWaiting sends it when the load ends. A chat that is not
// loading, such as one being switched to another provider, refuses it.
func (t *TurnIO) holdWhileLoading(s *Conversation, text string, doc *DocRef, atts []Attachment) error {
	state := s.State()
	if !state.Open || state.Switching || state.Superseded || (!state.Restoring && !state.Replaying) {
		return errors.New("wait for the provider change to finish")
	}
	p, err := t.host.MakePrompt(s, text, doc, atts)
	if err != nil {
		return err
	}
	if held, _ := s.AdmitLoadingSend(p); !held {
		return errors.New("wait for the provider change to finish")
	}
	t.host.Remember(s.Cwd(), s.ID(), "queue", text, doc)
	EmitQueue(t.host.Emit, s)
	return nil
}

// StartWaiting starts the turn for the first message held while the chat
// loaded, when the load has ended; the rest follow as queued messages do.
func (t *TurnIO) StartWaiting(s *Conversation) {
	p := s.TakeWaiting()
	if p == nil {
		return
	}
	EmitSession(t.host.Emit, s)
	t.start(s, p)
}

// SendPrompt sends a prompt that has already been made, such as one that was
// held in another chat, to a chat, as a message sent to it would go: it starts a
// turn, or waits behind the turn that is running, or waits for the chat to finish
// loading. Everything the prompt carries goes with it, its pictures and files
// included, since they were made for a chat of the same provider.
func (t *TurnIO) SendPrompt(id string, p Prompt) error {
	s := t.host.LookupConversation(id)
	if s == nil {
		return errors.New("that session is not open")
	}
	state := s.State()
	if !state.Ready {
		if held, _ := s.AdmitLoadingSend(p); held {
			EmitQueue(t.host.Emit, s)
			return nil
		}
		return errors.New("wait for the provider change to finish")
	}
	ready, queued, _ := s.AdmitSend(p)
	if !ready {
		return errors.New("wait for the provider change to finish")
	}
	if queued {
		EmitQueue(t.host.Emit, s)
		return nil
	}
	EmitSession(t.host.Emit, s)
	t.start(s, &p)
	return nil
}

// sendAs sends a prompt the app makes for the person, such as a skill's
// instructions, to a session that is not working. The chat shows it with its
// label, and it does not go into the person's prompt history. It returns at
// once, as Send does.
func (t *TurnIO) SendAs(s *Conversation, label, text string) error {
	if err := t.host.HoldingTurns(); err != nil {
		return err
	}
	p, err := t.host.PromptFrom(s, text, nil)
	if err != nil {
		return err
	}
	p = p.WithLabel(label)
	if !s.AdmitLabelled() {
		return errors.New("the session is busy with another turn")
	}

	EmitSession(t.host.Emit, s)
	EmitChat(t.host.Emit, s, EventSent, map[string]any{"sessionId": s.ID(), "text": text, "kind": "skill", "label": label})
	t.start(s, &p)
	return nil
}

// askWait steers one question, the prompt p, into a capable active turn or
// queues it for its own turn, then captures the answer. Expiry removes
// queued input and prevents an undelivered steer from being replayed after
// its deadline. p has the question's label and the delivered entry of the
// bus message that brought it (chat_ask.go).
func (t *TurnIO) AskWait(ctx context.Context, s *Conversation, p Prompt) (string, End) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if err := t.host.HoldingTurns(); err != nil {
		return "", End{StopReason: "error", Err: err}
	}
	text := p.Text()
	var err error
	p = s.WithReply(ctx, p)
	submission := s.BeginSubmission()
	defer submission.Release()

	ready, working, occupied := s.AdmitReadyQuestion(p)
	if !ready {
		submission.Release()
		return "", End{StopReason: "error", Err: errors.New("the chat is not ready for a question: it is closed, or changing its provider, or reading its history")}
	}

	if working && !occupied && t.CanSteer(s.ID(), nil) {
		s.Reserve(p)
		err = t.SubmitSteer(s, submission, p, text, nil, nil)
		if err != nil {
			s.ClearReservation()
		}
		submission.Release()
		if err != nil {
			return "", End{StopReason: "error", Err: err}
		}
	} else {
		s.EnqueueQuestion(p, working)
		if working {
			submission.Release()
			EmitQueue(t.host.Emit, s)
		} else {
			submission.Release()
			EmitSession(t.host.Emit, s)
			EmitChat(t.host.Emit, s, EventSent, SentEvent(s.ID(), text, SentKind(&p, "asked"), p.Label(), p.Delivered()))
			t.start(s, &p)
		}
	}
	words, end, completed := p.Wait(ctx)
	if completed {
		return words, End{StopReason: end.StopReason, Err: end.Err}
	} else {
		s.RemoveQuestion(p)
		EmitQueue(t.host.Emit, s)
		return "", End{StopReason: "error", Err: ctx.Err()}
	}
}

// Queue keeps a message until the session's turn ends by itself, then sends
// it as the next prompt. With no turn running it is sent at once.
func (t *TurnIO) Queue(id, text string, doc *DocRef, atts []Attachment) error {
	text = strings.TrimSpace(text)
	if text == "" && len(atts) == 0 {
		return nil
	}
	if err := t.host.HoldingTurns(); err != nil {
		return err
	}
	s := t.host.LookupConversation(id)
	if s == nil {
		return errors.New("that session is not open")
	}

	state := s.State()
	if !state.Ready {
		return t.holdWhileLoading(s, text, doc, atts)
	}
	working := state.Working

	if !working {
		return t.Send(id, text, doc, atts)
	}
	p, err := t.host.MakePrompt(s, text, doc, atts)
	if err != nil {
		return err
	}
	ready, stillWorking, _ := s.QueueActive(p)
	if !ready {
		return errors.New("wait for the provider change to finish")
	}
	if !stillWorking {
		return t.Send(id, text, doc, atts)
	}

	t.host.Remember(s.Cwd(), s.ID(), "queue", text, doc)
	EmitQueue(t.host.Emit, s)
	return nil
}

// Unqueue drops a session's queued messages.
func (t *TurnIO) Unqueue(id string) {
	if s := t.host.LookupConversation(id); s != nil {
		dropped := s.ClearQueue()
		DropPrompts(dropped, "the person cleared the chat's queued messages")
		for _, p := range dropped {
			t.host.SteeringState(id, p.LedgerID(), "dismissed")
		}
		EmitQueue(t.host.Emit, s)
		return
	}
	EmitQueueCleared(t.host.Emit, id)
}

// Steer adds guidance without cancelling active work. Unsupported delivery
// paths keep the input for the next turn; interruption is StopAndSteer's job.
func (t *TurnIO) Steer(id, text string, doc *DocRef, atts []Attachment) error {
	return t.SteerActive(id, text, doc, atts)
}

// StopAndSteer interrupts the session's turn and sends the message as the
// next prompt in the same session after cancellation settles. Ordinary
// steering uses provider-specific delivery without cancellation. ACP
// documents session/cancel, after which the session goes on
// (https://agentclientprotocol.com/protocol/prompt-turn).
func (t *TurnIO) StopAndSteer(id, text string, doc *DocRef, atts []Attachment) error {
	text = strings.TrimSpace(text)
	if text == "" && len(atts) == 0 {
		return nil
	}
	if err := t.host.HoldingTurns(); err != nil {
		return err
	}
	s := t.host.LookupConversation(id)
	if s == nil {
		return errors.New("that session is not open")
	}

	state := s.State()
	if !state.Ready {
		return errors.New("wait for the provider change to finish")
	}
	working := state.Working

	if !working {
		return t.Send(id, text, doc, atts)
	}
	p, err := t.host.PromptFrom(s, steerPrompt(t.host.WithDoc(text, t.host.DraftDocRef(FreshSelection(s, doc)))), atts)
	if err != nil {
		return err
	}
	ready, stillWorking := s.InterruptActive(p)
	if !ready {
		return errors.New("wait for the provider change to finish")
	}
	if !stillWorking {
		return t.Send(id, text, doc, atts)
	}

	t.host.Remember(s.Cwd(), s.ID(), "steer", text, doc)
	return t.CancelConversation(s)
}

// Cancel stops a session's current turn. The session stays open, and a
// stopped turn sends nothing queued after it; the queue waits.
func (t *TurnIO) Cancel(id string) error {
	s := t.host.LookupConversation(id)
	if s == nil {
		return nil
	}
	return t.CancelConversation(s)
}

func (t *TurnIO) CancelConversation(s *Conversation) error {
	conn := t.host.SessionConnection(s)
	if conn == nil {
		return nil
	}
	t.host.CancelQuestions(s.ID())
	return conn.Notify("session/cancel", acp.CancelParams{SessionID: s.ID()})
}

// steerprompt is what the agent is told when the person steers it.
func steerPrompt(text string) string {
	return "I stopped you to steer the work: " + text + "\n\nCarry on with the task you were doing, with this in mind. If a step was cut off when I stopped you, check its result before you go on."
}

// run carries a session's turns until nothing more is waiting: the first
// prompt, then a steering message if the person steered, then each queued
// message once a turn ends by itself.
func (t *TurnIO) Run(s *Conversation, next *Prompt) {
	var submission *Submission
	defer func() { submission.Release() }()
	// A chat that closed itself in the background is back on the bus for
	// any turn it takes, such as another chat's question (bus_wake.go).
	t.host.Unpark(s)
	for next != nil {
		// A chat whose known context window is full holds the send until the
		// person's course has run (context_hold.go).
		if t.host.ContextGate != nil && t.host.ContextGate(s, next) {
			return
		}
		conn := t.host.SessionConnection(s)
		var res acp.PromptResult
		began := time.Now()
		current := next
		if current.SteerID() != 0 {
			t.host.SteeringState(s.ID(), current.LedgerID(), "pending")
			EmitChat(t.host.Emit, s, EventSteer, map[string]any{"sessionId": s.ID(), "steer": current.SteerID(), "state": "sent"})
		}
		// The person's message goes into the session's journal as it is sent,
		// and the agent's first message of the turn begins a new entry. A
		// prompt of the bus's alone is no message of the person's, and its
		// entries keep their own times, so the page's times for the person's
		// messages still line up with the journal.
		if len(next.Delivered()) > 0 {
			t.host.Journal(s.ID(), deliveredRole)
		} else {
			t.host.Journal(s.ID(), "you")
		}

		s.StartTurn(current)

		t.host.RecordPrompt(s, next)
		t.host.LogPrompt(s.ID(), next.Label(), t.promptText(next))
		// The person speaking takes what the bus holds for the chat along
		// with their words (bus_chats.go).
		var waiting string
		if next.Label() == "" && !next.Asked() {
			var delivered []ConversationMessage
			if waiting, delivered = t.host.BusWaiting(s); waiting != "" {
				// The agent is given the messages with the person's words; the
				// saved copy keeps them after those words as the senders'
				// entries, with the text the agent was given, so that a replay
				// of the whole prompt is still recognised (delivered.go).
				t.host.RecordDelivered(s, waiting, delivered)
				EmitChat(t.host.Emit, s, EventSent, SentEvent(s.ID(), waiting, "bus", busChatLabel, delivered))
			}
		}
		err := t.host.PrepareProviderApplets(normalizeProvider(s.Provider()), s.Cwd())
		if conn == nil {
			err = errors.New("the agent stopped before the turn could start")
		}
		var appletContext string
		if err == nil {
			appletContext, err = t.host.AppletPromptContext(s.ID())
		}
		if err == nil {
			blocks := append([]acp.ContentBlock{}, next.Blocks()...)
			if waiting != "" {
				blocks = append(blocks, acp.ContentBlock{Type: "text", Text: waiting})
			}
			var handoff string
			handoff, err = t.host.PendingContinuationContext(s)
			if err == nil {
				if handoff != "" {
					blocks = append(blocks, acp.ContentBlock{Type: "text", Text: handoff})
				}
				blocks = append(blocks, acp.ContentBlock{Type: "text", Text: appletContext})
				// The marker that lets the agent's hooks find this chat, to
				// hand over a steer mid-turn (steers.go).
				if marker := SteerMarker(s); marker != "" {
					token := rand.Text()
					busID := s.StartHook(token)
					err = t.host.BeginHookTurn(busID, token)
					marker = bus.SteerTurnMarker(busID, token)
					blocks = append(blocks, acp.ContentBlock{Type: "text", Text: marker})
				}
				if err == nil {
					err = t.host.Call(conn, "session/prompt", acp.PromptParams{SessionID: s.ID(), Prompt: blocks}, &res)
				}
				if err == nil {
					t.host.ConsumeContinuationContext(s)
				}
			}
		}
		submission = s.BeginSubmission()
		if errors.Is(err, acp.ErrSteeringUnconfirmed) {
			t.UnconfirmNativeSteers(s, submission)
		}
		if current.LedgerID() != 0 {
			state := "sent"
			if err != nil {
				state = "failed"
			}
			t.host.SteeringState(s.ID(), current.LedgerID(), state)
		}
		hookTurn, busID := s.TakeHook()
		if hookTurn != "" {
			closeErr := t.host.EndHookTurn(busID, hookTurn)
			if closeErr != nil {
				err = errors.Join(err, closeErr)
			}
		}
		took := time.Since(began)
		// Steers no hook took in this turn go next, ahead of the queue.
		late := t.SettleSteersWithSubmission(s, submission)
		t.host.FinishTranscript(s, err, res.StopReason)
		// The RFD has every subagent end before its parent's prompt
		// returns; one the agent has not reported ended now has an unknown
		// outcome (subagents.go).
		t.host.DisconnectSubagents(s)
		var messages *Prompt
		if err == nil && res.StopReason != "cancelled" {
			messages = t.host.BusAfterTurn(s)
		}

		var steered bool

		var completion Completion
		next, steered, _, completion = s.NextTurn(messages, late, err == nil && res.StopReason != "cancelled")

		submission.Finish(completion, *current, late, End{StopReason: res.StopReason, Err: err})
		// The turn's length is measured here, from the prompt to its answer;
		// the protocol reports no duration of its own.
		turn := map[string]any{"sessionId": s.ID(), "stopReason": res.StopReason, "seconds": int(took.Round(time.Second) / time.Second)}
		switch {
		case err != nil:
			turn["stopReason"], turn["error"] = "error", err.Error()
		case steered:
			turn["stopReason"] = "steered"
		}
		// The turn's event is appended to the log where the saved copy is
		// written with how the turn ended, and emitted after.
		if t.host.CommitTurn != nil {
			t.host.CommitTurn(s, turn).Emit(t.host.Emit)
		} else {
			EmitChat(t.host.Emit, s, EventTurn, turn)
		}
		if next != nil {
			kind := "queued"
			if steered {
				kind = "steer"
			} else if next.Asked() || next.Label() == busChatLabel {
				kind = SentKind(next, "asked")
			} else if next.Late() {
				kind = "late-steer"
			}
			EmitChat(t.host.Emit, s, EventSent, SentEvent(s.ID(), next.Text(), kind, next.Label(), next.Delivered()))
			EmitQueue(t.host.Emit, s)
		}
	}
	EmitSession(t.host.Emit, s)
}
