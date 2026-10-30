package sessionhost

// A chat's state is worked out in one place (docs/plans/chat-states.md). It has a life, which says whether its
// session runs, and, only while it runs, an execution state with the reason for it as a full phrase. Whether the
// chat has a window is a separate fact. Every surface that shows a chat takes these fields as they are and works
// out neither the state nor the urgency of several chats.

// The execution states, most urgent first, and the lives.
const (
	ExecStuck   = "stuck"
	ExecWaiting = "waiting"
	ExecWorking = "working"
	ExecIdle    = "idle"

	LifeOpen    = "open"
	LifeClosed  = "closed"
	LifeRetired = "retired"
)

// ChatExec is what an open chat is doing, and why in words, so that the mark is never the only fact.
type ChatExec struct {
	State string `json:"state"`
	Why   string `json:"why"`
}

// ChatState is a chat's life, whether it has a window, and, when its session runs, its execution state.
type ChatState struct {
	Life   string    `json:"life"`
	Window bool      `json:"window"`
	Exec   *ChatExec `json:"exec,omitempty"`
	// Elsewhere says, in words, that the chat's session runs but not in this window, so its activity is not known
	// here: it has no Exec, and is shown as a lifecycle word, never as Idle.
	Elsewhere string `json:"elsewhere,omitempty"`
}

// StateCounts is how many of a set of chats are in each execution state, and the state the set shows as a whole:
// Stuck, then Waiting, then Working, else Idle, which includes a set whose chats are all closed. Closed counts the
// chats that do not run, closed or retired, which have no execution state. Unknown counts the chats that run elsewhere.
type StateCounts struct {
	Stuck   int `json:"stuck"`
	Waiting int `json:"waiting"`
	Working int `json:"working"`
	Idle    int `json:"idle"`
	Closed  int `json:"closed"`
	// Unknown counts the chats that run elsewhere, whose activity is not known; they are counted as nothing else
	// and Top ignores them: when no chat has a known state and any is unknown, Top is empty.
	Unknown int    `json:"unknown"`
	Top     string `json:"top,omitempty"`
}

// CountStates counts chats' states and chooses the state they show as a whole.
func CountStates(states []ChatState) StateCounts {
	var c StateCounts
	for _, s := range states {
		c.Add(s)
	}
	c.Finish()
	return c
}

// Add counts one chat. A rollup that adds counts of its own parts calls Merge instead.
func (c *StateCounts) Add(s ChatState) {
	if s.Exec == nil {
		if s.Elsewhere != "" {
			c.Unknown++
		} else {
			c.Closed++
		}
		return
	}
	switch s.Exec.State {
	case ExecStuck:
		c.Stuck++
	case ExecWaiting:
		c.Waiting++
	case ExecWorking:
		c.Working++
	default:
		c.Idle++
	}
}

// Merge adds another set's counts (not its Top).
func (c *StateCounts) Merge(o StateCounts) {
	c.Stuck += o.Stuck
	c.Waiting += o.Waiting
	c.Working += o.Working
	c.Idle += o.Idle
	c.Closed += o.Closed
	c.Unknown += o.Unknown
}

// Finish chooses the state the counts show as a whole.
func (c *StateCounts) Finish() {
	switch {
	case c.Stuck > 0:
		c.Top = ExecStuck
	case c.Waiting > 0:
		c.Top = ExecWaiting
	case c.Working > 0:
		c.Top = ExecWorking
	case c.Idle > 0:
		c.Top = ExecIdle
	case c.Unknown > 0:
		// Nothing is known of any chat that runs, and some run elsewhere: no state is shown as a whole.
		c.Top = ""
	default:
		c.Top = ExecIdle
	}
}

func copyState(s *ChatState) *ChatState {
	if s == nil {
		return nil
	}
	out := *s
	if s.Exec != nil {
		e := *s.Exec
		out.Exec = &e
	}
	return &out
}

func copyCounts(c *StateCounts) *StateCounts {
	if c == nil {
		return nil
	}
	out := *c
	return &out
}

// SameState says two states are the same chat state (either may be nil).
func SameState(a, b *ChatState) bool { return sameState(a, b) }

func sameState(a, b *ChatState) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Life != b.Life || a.Window != b.Window || a.Elsewhere != b.Elsewhere || (a.Exec == nil) != (b.Exec == nil) {
		return false
	}
	return a.Exec == nil || *a.Exec == *b.Exec
}

func sameCounts(a, b *StateCounts) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
