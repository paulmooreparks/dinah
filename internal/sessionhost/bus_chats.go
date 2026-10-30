package sessionhost

import (
	"context"
	_ "embed"
	"sync"
	"time"

	"studio/internal/applet/broker"
	"studio/internal/bus"
	"studio/internal/bushost"
)

//go:embed bus_chats.go
var BusChatsSource []byte

// A message that asks a chat to answer, such as a mention in a group chat,
// a message sent to the chat itself, or a reply it waits for, reaches the
// chat even when the chat is not connected to its agent
// (docs/plans/group-chats.md, Closed chats). The watcher looks for such
// messages at every round (bus.Waking) and connects the chat they wait for;
// once the chat is open, the watcher delivers them as it delivers any
// message (ringChat). Because the messages wait on the bus, a restart pokes
// the chats they wait for at the watcher's first round. Another chat's
// question is such a message, sent to the chat itself; once the chat is
// open, the invocation that asked it delivers it (chat_ask.go).
//
// Dinah connects a chat through the page, by the path that Reconnect Now
// takes (app.js, reconnectChat), so a chat whose window was restored with
// the layout and waits at the prompt to reconnect keeps its saved copy in
// sight while its agent replays, and reports a failure in that window as
// the prompt does. A chat with no window is opened the same way without
// one, and is then open in the background. The page says which chats have
// a window (ChatWindows), and nothing is connected until it has said so
// once, so a window being restored at startup is never mistaken for a
// closed chat.
//
// At most backgroundSlots chats are open in the background at once, or
// being opened there; the rest queue in the order their messages came and
// open as slots free, so a message to a large group cannot start a crowd of
// agents. A chat open in the background closes itself backgroundIdle after
// its last turn ends, unless the person has opened its window by then,
// which makes it an ordinary open chat. Neither limit touches a chat's
// membership of a group: its messages wait for it as they wait for any
// closed member. A chat that fails to connect is not asked again until a
// newer message wakes it, and its messages keep waiting.

const BackgroundIdle = 30 * time.Minute

// BusChats coordinates session bus activity using the existing runtime authority.
// It owns no bus handle or watcher and must not be copied after use.
type BusChats struct {
	mu      *sync.Mutex
	runtime *Runtime
	host    BusChatsHost
	// retries holds the results that could not be queued yet, and when each is tried again (bus_results.go).
	retries resultRetries
}

type BusChatsHost struct {
	ResultBroker     func() (*broker.Broker, error)
	SavedBusIdentity func(string) string
	TurnIO           func() *TurnIO
	AgentPerson      func() string
	CanSteer         func(string, []Attachment) bool
	BusPictures      func(*Conversation, []bus.Message) ([]Attachment, string)
	PreparePrompt    func(*Conversation, string, []Attachment) (Prompt, error)
	AskPrompt        func(string, string) string
	EnsureBusChat    func(BusAskTarget) (string, error)
	BusService       func() *bushost.Service
	EndClosed        func()
	HoldingTurns     func() error
	ChatOfBus        func(string) string
	// HiddenLocked is called under the shared state mutex and must not re-enter it.
	HiddenLocked        func(string) bool
	Clock               func() time.Time
	AllocateBusIdentity func(string) string
	SaveBusIdentity     func(string, string)
	ShownTitle          func(*Conversation) string
	Emit                func(string, any)
	Log                 func(string, string, ...any)
	// OpenChat opens a chat in the engine without the page, as the page does
	// when it is asked to connect one. ConnectGrace is how long the page is
	// given to do so before the engine opens the chat itself; zero leaves the
	// chat to the page alone.
	// ConnectFailed is called when a chat could not be connected for the messages waiting for it.
	ConnectFailed func(chat string, err error)
	OpenChat      func(chat, cwd string) error
	ConnectGrace  time.Duration
}

func NewBusChats(mu *sync.Mutex, runtime *Runtime, host BusChatsHost) *BusChats {
	return &BusChats{mu: mu, runtime: runtime, host: host}
}

func (a *BusChats) session(id string) *Conversation {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.runtime.ConversationLocked(id)
}

func (a *BusChats) ConnectFailedSince(chat string, since time.Time) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.runtime.FailedSinceLocked(chat, since)
}

func (a *BusChats) ChatWindows(ids []string) {
	a.mu.Lock()
	a.runtime.SetWindowsLocked(ids)
	a.mu.Unlock()
	for _, id := range ids {
		s := a.session(id)
		if s == nil {
			continue
		}
		a.Unpark(s)
		changed := s.WindowOpened()
		if changed {
			EmitSession(a.host.Emit, s)
		}
	}
}

func (a *BusChats) HasWindow(chat string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.runtime.HasWindowLocked(chat)
}

// ConnectWaking consumes a watcher-borrowed bus synchronously and retains no handle.
func (a *BusChats) ConnectWaking(ctx context.Context, b *bus.Bus) {
	a.CloseIdleBackground()
	a.host.EndClosed()
	a.mu.Lock()
	known := a.runtime.WindowsKnownLocked()
	a.mu.Unlock()
	// While the window waits to restart, no chat is opened for a message;
	// the messages wait for the reopened window (update.go).
	if !known || a.host.HoldingTurns() != nil {
		return
	}
	a.queueWarm(ctx, b)
	wakes, err := b.Waking(ctx)
	if err != nil {
		a.host.Log("error", "looking for chats to wake: %v", err)
		return
	}
	now := a.host.Clock()
	var reconnect []bus.Wake
	paths := map[string]string{}
	a.mu.Lock()
	a.runtime.ExpireWakeLocked(now)
	a.mu.Unlock()
	for _, w := range wakes {
		chat := a.host.ChatOfBus(w.Session)
		if chat == "" {
			continue
		}
		paths[chat] = w.WorkspacePath
		a.mu.Lock()
		decision, connect := a.runtime.ConsiderWakeLocked(chat, w.WorkspacePath, w.Newest, a.host.HiddenLocked(chat), now)
		if connect {
			reconnect = append(reconnect, bus.Wake{Session: decision.Chat, WorkspacePath: decision.Path})
		}
		a.mu.Unlock()
	}
	for _, w := range reconnect {
		a.AskToConnect(w.Session, w.WorkspacePath, false)
	}
	a.OpenQueued(paths)
	a.openWarm()
}

// queueWarm makes the one pass, after the window has started, that queues the
// chats of groups to open in the background ahead of any message: those of
// the groups with a message in the last bushost.WarmWithin, most active group
// first. Hidden chats, chats with windows (the layout restores those) and
// chats already open are left out, and the scheduler keeps at most
// bushost.WarmChats of the rest (docs/plans/group-reopen.md). A pass that
// cannot read the groups is made again at the next look.
func (a *BusChats) queueWarm(ctx context.Context, b *bus.Bus) {
	a.mu.Lock()
	warmed := a.runtime.WarmedLocked()
	a.mu.Unlock()
	if warmed {
		return
	}
	chats, err := b.GroupChats(ctx, a.host.Clock().Add(-bushost.WarmWithin))
	if err != nil {
		a.host.Log("error", "looking for group chats to open at start: %v", err)
		return
	}
	var queue []bushost.Decision
	seen := map[string]bool{}
	for _, g := range chats {
		chat := a.host.ChatOfBus(g.Session)
		if chat == "" || seen[chat] {
			continue
		}
		seen[chat] = true
		a.mu.Lock()
		// A chat with a window is warmed too unless it is already open: the
		// layout restores the window with the saved copy only, and nothing
		// else opens the chat until the person types.
		skip := a.host.HiddenLocked(chat) || (a.runtime.HasWindowLocked(chat) && a.runtime.ConversationLocked(chat) != nil)
		a.mu.Unlock()
		if skip {
			continue
		}
		queue = append(queue, bushost.Decision{Chat: chat, Path: g.WorkspacePath, Background: true})
	}
	a.mu.Lock()
	a.runtime.QueueWarmLocked(queue)
	a.mu.Unlock()
}

// openWarm starts the connects of the queued group chats that the limit on
// connecting at a time allows.
func (a *BusChats) openWarm() {
	for {
		a.mu.Lock()
		decision, ok := a.runtime.NextWarmLocked(a.host.Clock())
		a.mu.Unlock()
		if !ok {
			return
		}
		a.AskToConnect(decision.Chat, decision.Path, true)
	}
}

func (a *BusChats) OpenQueued(paths map[string]string) {
	now := a.host.Clock()
	for {
		a.mu.Lock()
		decision, ok := a.runtime.NextWakeLocked(paths, now)
		a.mu.Unlock()
		if !ok {
			return
		}
		a.AskToConnect(decision.Chat, decision.Path, decision.Background)
	}
}

func (a *BusChats) AskToConnect(chat, cwd string, background bool) {
	if s := a.session(chat); s != nil {
		parked := s.Parked()
		if parked {
			a.Unpark(s)
			a.Connected(chat, nil)
			return
		}
	}
	a.host.Emit("chat:connect", map[string]any{"sessionId": chat, "cwd": cwd, "background": background})
	// A window the page restored may not act on the request, and nothing asks
	// again until a newer message arrives, so a chat the page has not opened
	// within the grace is opened here (docs/plans/group-reopen.md).
	if a.host.OpenChat != nil && a.host.ConnectGrace > 0 {
		time.AfterFunc(a.host.ConnectGrace, func() { a.openIfStillClosed(chat, cwd) })
	}
}

// openIfStillClosed opens a chat the page was asked to connect and did not.
// A chat that is open, or being opened, by then is left to the page.
func (a *BusChats) openIfStillClosed(chat, cwd string) {
	if a.session(chat) != nil {
		return
	}
	a.host.Log("info", "the window did not open chat %s for its waiting messages, so it is opened without the window", chat)
	if err := a.host.OpenChat(chat, cwd); err != nil {
		a.host.Log("error", "chat %s could not be opened for its waiting messages: %v", chat, err)
	}
}

func (a *BusChats) Connected(chat string, err error) {
	now := a.host.Clock()
	a.mu.Lock()
	changed, asked := a.runtime.CompleteWakeLocked(chat, err, now)
	a.mu.Unlock()
	if !asked {
		return
	}
	if err != nil {
		a.host.Log("error", "chat %s could not be connected for the messages waiting for it, which keep waiting: %v", chat, err)
		if a.host.ConnectFailed != nil {
			a.host.ConnectFailed(chat, err)
		}
		return
	}
	if changed != nil {
		EmitSession(a.host.Emit, changed)
	}
}

func (a *BusChats) CloseIdleBackground() {
	now := a.host.Clock()
	a.mu.Lock()
	var idle []*Conversation
	for _, s := range a.runtime.ConversationsLocked() {
		if s.IdleBackground(now, BackgroundIdle) {
			idle = append(idle, s)
		}
	}
	a.mu.Unlock()
	for _, s := range idle {
		a.Park(s)
	}
}

func (a *BusChats) Park(s *Conversation) {
	if !s.ParkAt(a.host.Clock) {
		return
	}

	a.LeaveBus(s)
	EmitSession(a.host.Emit, s)
}

func (a *BusChats) Unpark(s *Conversation) {
	if !s.Unpark() {
		return
	}

	id := a.host.AllocateBusIdentity(s.ID())
	err := a.host.BusService().With(func(b *bus.Bus) error {
		_, err := b.RegisterSession(context.Background(), id, s.Cwd(), a.host.ShownTitle(s))
		return err
	})
	if err != nil {
		a.host.Log("error", "putting chat %s back on the bus: %v", s.ID(), err)
	} else {
		a.JoinBus(s, id)
	}
	if !a.HasWindow(s.ID()) {
		s.BackgroundAt(a.host.Clock)
	}
	EmitSession(a.host.Emit, s)
}

func (a *BusChats) JoinBus(s *Conversation, id string) {
	s.JoinBus(id)
	a.host.SaveBusIdentity(s.ID(), id)
	// A chat that now has a bus identity may be the caller of a result that was waiting for one.
	a.retries.reset()
	a.PublishBusTitle(s)
}

func (a *BusChats) LeaveBus(s *Conversation) {
	id := s.LeaveBus()
	a.EndBusSession(id)
}

func (a *BusChats) PublishBusTitle(s *Conversation) {
	title := a.host.ShownTitle(s)

	id, same := s.BusTitleState(title)

	if id == "" || same {
		return
	}
	err := a.host.BusService().With(func(b *bus.Bus) error { return b.SetTitle(context.Background(), id, title) })
	if err != nil {
		a.host.Log("error", "giving chat %s its title on the bus: %v", s.ID(), err)
		return
	}
	s.PublishBusTitle(id, title)
}

func (a *BusChats) EndBusSession(id string) {
	if id == "" {
		return
	}
	_ = a.host.BusService().With(func(b *bus.Bus) error { return b.EndSession(context.Background(), id) })
}
