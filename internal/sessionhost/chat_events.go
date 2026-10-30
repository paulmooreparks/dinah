package sessionhost

import (
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"maps"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
)

//go:embed chat_events.go
var ChatEventsSource []byte

// The events that make up a chat's transcript on the page. They are emitted
// only through EmitChat and EmitChatReplay in this file, which number them in
// the chat's own log first; a test fails if any other code emits them
// (docs/plans/event-streams.md).
const (
	EventUpdate  = "agent:update"
	EventUpdates = "agent:updates"
	EventTurn    = "agent:turn"
	EventSteer   = "agent:steer"
	EventSent    = "agent:sent"
	// EventTeamSent says a message the chat sent in a team's group chat was posted, with the group and the
	// message's number, so the window can fold a run of the team's traffic (docs/plans/teams-ui.md).
	EventTeamSent = "agent:teamsent"
	// EventArriving says messages are waiting for a chat that is still
	// restoring or replaying, so the page can show them where the delivered
	// messages will land (docs/plans/group-reopen.md).
	EventArriving = "agent:arriving"
)

// ChatLogLimits bound the chats' event logs: each chat keeps at most Events
// entries or Bytes of them, whichever is reached first, and all the open
// chats together keep at most TotalBytes. A log that is trimmed costs a page
// that asks for what was dropped a marked loss, never a silent one, so the
// bounds are options in code and not a setting.
type ChatLogLimits struct {
	Events, Bytes, TotalBytes int
}

// DefaultChatLogLimits are the bounds Architecture set on 29 October 2026
// (message 530), from Dinah's saved transcripts: a long chat outgrows any
// log, and a log has only to cover a gap the bridge's shared ring could not.
var DefaultChatLogLimits = ChatLogLimits{Events: 32768, Bytes: 16 << 20, TotalBytes: 64 << 20}

// ChatEvent is one entry of a chat's log: its number, the event it was sent
// as, and the event's data without the number and generation, which the
// emitted event carries beside it. An agent:updates batch is kept as one
// agent:update entry for each update in it.
type ChatEvent struct {
	N    uint64          `json:"n"`
	Name string          `json:"name"`
	Data json.RawMessage `json:"data"`
}

// ChatEventsView is what a page reads to fill a gap: the entries after the
// number it asked from, the log's generation and highest number. Reset says
// the generation is not the one the page has, because the chat was closed
// and opened again or the engine started again, and the page reopens the
// chat. Truncated says the log no longer has some entry after the number
// asked from, and the page marks the loss.
type ChatEventsView struct {
	Gen       string      `json:"gen"`
	Head      uint64      `json:"head"`
	Events    []ChatEvent `json:"events"`
	Truncated bool        `json:"truncated"`
	Reset     bool        `json:"reset"`
}

// chatLog is one chat's numbered events. Its mutex is a leaf: nothing else is
// acquired while it is held, and no event is emitted under it. An entry is
// stored before its event is emitted, so a page that sees a number beyond the
// next one can always read the ones between.
type chatLog struct {
	mu      sync.Mutex
	session string
	gen     string
	head    uint64
	// floor is the highest number not kept, whether dropped by a bound or
	// never kept because it was larger than the per-chat bound. A page whose
	// last number is below it has missed an entry that is gone.
	floor   uint64
	entries []ChatEvent
	bytes   int
	// logs is the registry of the runtime the chat is open in, which keeps
	// the total, or nil before it is registered and after it is removed.
	logs *ChatLogs
}

func newChatLog(session string) *chatLog {
	var raw [12]byte
	_, _ = rand.Read(raw[:])
	return &chatLog{session: session, gen: hex.EncodeToString(raw[:])}
}

// The test hooks see every acquisition and release of a log's mutex, so a
// test can check that no two are ever held together.
var chatLogLocked, chatLogUnlocked func()

func (l *chatLog) lock() {
	l.mu.Lock()
	if chatLogLocked != nil {
		chatLogLocked()
	}
}

func (l *chatLog) unlock() {
	if chatLogUnlocked != nil {
		chatLogUnlocked()
	}
	l.mu.Unlock()
}

func entrySize(name string, data json.RawMessage) int { return len(name) + len(data) + 16 }

// appendLocked numbers and keeps one entry and returns its number. An entry
// larger than the per-chat byte bound is numbered but not kept, so a page that
// missed it is told of the loss. The caller holds l.mu.
func (l *chatLog) appendLocked(name string, data json.RawMessage) uint64 {
	l.head++
	n := l.head
	limits := l.limitsLocked()
	size := entrySize(name, data)
	if size > limits.Bytes {
		l.raiseFloorLocked(n)
		return n
	}
	l.entries = append(l.entries, ChatEvent{N: n, Name: name, Data: data})
	l.bytes += size
	if l.logs != nil {
		l.logs.total.Add(int64(size))
	}
	for len(l.entries) > limits.Events || l.bytes > limits.Bytes {
		l.dropOldestLocked()
	}
	return n
}

// raiseFloorLocked records that n is not kept. The floor only ever rises: an
// entry too large to keep can have a higher number than older entries the
// bounds drop later, and lowering the floor to one of those would tell a page
// that missed the large entry that nothing was lost.
func (l *chatLog) raiseFloorLocked(n uint64) {
	if n > l.floor {
		l.floor = n
	}
}

func (l *chatLog) dropOldestLocked() {
	e := l.entries[0]
	size := entrySize(e.Name, e.Data)
	l.raiseFloorLocked(e.N)
	l.bytes -= size
	if l.logs != nil {
		l.logs.total.Add(-int64(size))
	}
	l.entries[0] = ChatEvent{}
	l.entries = l.entries[1:]
}

func (l *chatLog) limitsLocked() ChatLogLimits {
	if l.logs != nil {
		return l.logs.limits
	}
	return DefaultChatLogLimits
}

// read returns the entries after a number, under the log's mutex, so the
// reply is a prefix of the log as it stood at one moment.
func (l *chatLog) read(gen string, after uint64) ChatEventsView {
	l.lock()
	defer l.unlock()
	view := ChatEventsView{Gen: l.gen, Head: l.head, Events: []ChatEvent{}}
	if gen != l.gen || after > l.head {
		view.Reset = true
		return view
	}
	view.Truncated = after < l.floor
	for _, e := range l.entries {
		if e.N > after {
			view.Events = append(view.Events, e)
		}
	}
	return view
}

func (l *chatLog) position() (string, uint64) {
	l.lock()
	defer l.unlock()
	return l.gen, l.head
}

// ChatLogs is the registry of a runtime's chat logs, which keeps their total
// size and trims the largest when the total is over its bound. Its mutex
// guards only the set of logs and is released before any log's is taken, so
// no two log mutexes are ever held together. The total is a running count
// that may overshoot its bound briefly while a trim is under way.
type ChatLogs struct {
	mu       sync.Mutex
	logs     map[*chatLog]struct{}
	limits   ChatLogLimits
	total    atomic.Int64
	trimming atomic.Bool
	// Logf records each trim, so the bounds can be checked against use.
	Logf func(level, format string, args ...any)
}

// NewChatLogs returns a registry with the given bounds; a zero field takes
// its default.
func NewChatLogs(limits ChatLogLimits) *ChatLogs {
	if limits.Events <= 0 {
		limits.Events = DefaultChatLogLimits.Events
	}
	if limits.Bytes <= 0 {
		limits.Bytes = DefaultChatLogLimits.Bytes
	}
	if limits.TotalBytes <= 0 {
		limits.TotalBytes = DefaultChatLogLimits.TotalBytes
	}
	return &ChatLogs{logs: map[*chatLog]struct{}{}, limits: limits}
}

// Total is the running total of the bytes the registered logs keep.
func (r *ChatLogs) Total() int64 { return r.total.Load() }

func (r *ChatLogs) register(l *chatLog) {
	r.mu.Lock()
	r.logs[l] = struct{}{}
	r.mu.Unlock()
	l.lock()
	if l.logs == nil {
		l.logs = r
		r.total.Add(int64(l.bytes))
		for len(l.entries) > r.limits.Events || l.bytes > r.limits.Bytes {
			l.dropOldestLocked()
		}
	}
	l.unlock()
	// The runtime calls this under the host's lock and calls no hosts, so a
	// total this takes over its bound is trimmed by the next append.
}

func (r *ChatLogs) deregister(l *chatLog) {
	l.lock()
	if l.logs == r {
		r.total.Add(-int64(l.bytes))
		l.logs = nil
	}
	l.unlock()
	r.mu.Lock()
	delete(r.logs, l)
	r.mu.Unlock()
}

// trimIfOver trims the largest logs, one at a time and each under only its
// own mutex, until the total is within its bound. One trim runs at a time; an
// append that finds one running leaves the total over its bound until that
// trim reaches it. The caller holds no log's mutex.
func (r *ChatLogs) trimIfOver() {
	if r == nil || r.total.Load() <= int64(r.limits.TotalBytes) || !r.trimming.CompareAndSwap(false, true) {
		return
	}
	defer r.trimming.Store(false)
	r.mu.Lock()
	logs := make([]*chatLog, 0, len(r.logs))
	for l := range r.logs {
		logs = append(logs, l)
	}
	r.mu.Unlock()
	for r.total.Load() > int64(r.limits.TotalBytes) {
		sizes := make([]int, len(logs))
		for i, l := range logs {
			l.lock()
			sizes[i] = l.bytes
			l.unlock()
		}
		order := make([]int, len(logs))
		for i := range order {
			order[i] = i
		}
		sort.Slice(order, func(a, b int) bool { return sizes[order[a]] > sizes[order[b]] })
		if len(order) == 0 || sizes[order[0]] == 0 {
			return
		}
		// The largest log gives back down to the size of the next largest,
		// or as much as the total needs, whichever is less, so the largest
		// logs are trimmed first and evenly.
		largest := logs[order[0]]
		floor := 0
		if len(order) > 1 {
			floor = sizes[order[1]]
		}
		largest.lock()
		for len(largest.entries) > 0 && largest.bytes > floor && r.total.Load() > int64(r.limits.TotalBytes) {
			largest.dropOldestLocked()
		}
		if floor == largest.bytes && len(largest.entries) > 0 && r.total.Load() > int64(r.limits.TotalBytes) {
			largest.dropOldestLocked()
		}
		entries, bytes, session := len(largest.entries), largest.bytes, largest.session
		largest.unlock()
		if r.Logf != nil {
			r.Logf("info", "trimmed chat %s's event log to %d entries and %d bytes; all chats' logs hold %d bytes", session, entries, bytes, r.total.Load())
		}
	}
}

// EmitChat numbers an event of a chat's transcript in the chat's log, then
// emits a copy of its data with the log's generation and the number put in
// (gen and n). It is the one way agent:update, agent:turn, agent:steer and
// agent:sent are emitted. The entry is kept, and the log's mutex released,
// before the event is emitted. An event for a session no open chat has is
// emitted as it is, without a number.
func EmitChat(emit func(string, any), s *Conversation, name string, data map[string]any) {
	AppendChat(s, name, data).Emit(emit)
}

// LoggedEvent is an event of a chat's transcript that has been numbered and
// kept in the chat's log and not yet emitted. AppendChat makes one, so that
// the append can be made inside the critical section that records the same
// event in the saved history, and Emit sends it after that section is left,
// as events have always been sent (docs/plans/running-chat-catch-up.md).
type LoggedEvent struct {
	name string
	data map[string]any
	gen  string
	n    uint64
	logs *ChatLogs
}

// AppendChat numbers an event in the chat's log and keeps it, without
// emitting it. The log's mutex is a leaf, so this may be called with any other
// lock held. For a session no open chat has it numbers nothing, and its Emit
// sends the data as it is.
func AppendChat(s *Conversation, name string, data map[string]any) *LoggedEvent {
	if s == nil {
		return &LoggedEvent{name: name, data: data}
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil
	}
	l := s.log
	l.lock()
	n := l.appendLocked(name, raw)
	gen, logs := l.gen, l.logs
	l.unlock()
	return &LoggedEvent{name: name, data: data, gen: gen, n: n, logs: logs}
}

// Emit trims the logs if they are over their bound and emits the event with
// the log's generation and its number put in (gen and n). It does nothing for
// an event that could not be encoded, and is safe on a nil event.
func (e *LoggedEvent) Emit(emit func(string, any)) {
	if e == nil {
		return
	}
	if e.gen == "" {
		emit(e.name, e.data)
		return
	}
	e.logs.trimIfOver()
	out := maps.Clone(e.data)
	out["gen"], out["n"] = e.gen, e.n
	emit(e.name, out)
}

// NumberedUpdate is an update queued during a replay, with the number its
// entry took when it was queued.
type NumberedUpdate struct {
	N      uint64
	Update json.RawMessage
}

// EmitChatReplay emits a batch of replayed updates as one agent:updates event
// with each update's number. The updates were numbered in the chat's log when
// they were queued, in the order the agent sent them, so this only emits.
func EmitChatReplay(emit func(string, any), s *Conversation, batch []NumberedUpdate) {
	if len(batch) == 0 {
		return
	}
	updates := make([]json.RawMessage, len(batch))
	ns := make([]uint64, len(batch))
	for i, u := range batch {
		updates[i], ns[i] = u.Update, u.N
	}
	gen, _ := s.log.position()
	emit(EventUpdates, map[string]any{"sessionId": s.ID(), "updates": updates, "gen": gen, "ns": ns})
}

// numberReplayLocked keeps one agent:update entry for each replayed update,
// in order, and returns them with their numbers. The caller holds c.mu, under
// which the log's mutex may be taken because it is a leaf.
func (c *Conversation) numberReplayLocked(updates []json.RawMessage) []NumberedUpdate {
	out := make([]NumberedUpdate, len(updates))
	l := c.log
	l.lock()
	for i, u := range updates {
		data, _ := json.Marshal(map[string]any{"sessionId": c.identity.ID, "update": u})
		out[i] = NumberedUpdate{N: l.appendLocked(EventUpdate, data), Update: slices.Clone(u)}
	}
	l.unlock()
	return out
}

// ChatEvents reads the chat's log after a number (ChatEventsView).
func (c *Conversation) ChatEvents(gen string, after uint64) ChatEventsView {
	return c.log.read(gen, after)
}

// StreamPosition is the chat log's generation and highest number.
func (c *Conversation) StreamPosition() (string, uint64) { return c.log.position() }

// trimChatLogs trims the logs of the runtime this chat is open in, if their
// total is over its bound. The caller holds no log's mutex.
func (c *Conversation) trimChatLogs() {
	c.log.lock()
	logs := c.log.logs
	c.log.unlock()
	logs.trimIfOver()
}
