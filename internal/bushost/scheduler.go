package bushost

import (
	_ "embed"
	"maps"
	"slices"
	"time"
)

//go:embed scheduler.go
var SchedulerSource []byte

const BackgroundSlots = 3
const ConnectPatience = 2 * time.Minute

// The chats of groups are opened in the background when the window starts, so
// a mention finds them ready (docs/plans/group-reopen.md). At most WarmChats
// are opened, WarmConcurrency at a time, and they do not use the
// BackgroundSlots, which are for chats opened by a message. Only groups with
// a message in the last WarmWithin are considered.
const (
	WarmChats       = 4
	WarmConcurrency = 2
	WarmWithin      = 7 * 24 * time.Hour
)

// Scheduler contains only scheduling state. Every read and mutation requires
// caller serialization, including the caller's live-session snapshot and any
// completion updates to that snapshot. It never calls the host. Returned maps
// and decisions are copies. The desktop uses App.mu for this transaction.
type Scheduler struct {
	windows    map[string]bool
	known      bool
	queue      []string
	connecting map[string]request
	tried      map[string]int64
	failed     map[string]failure
	warmQueue  []Decision
	warmed     bool
}
type request struct {
	at         time.Time
	background bool
	warm       bool
}
type failure struct {
	at  time.Time
	err error
}
type Decision struct {
	Chat, Path string
	Background bool
}
type Completion struct{ Asked, Background, Window, Warm bool }

func (s *Scheduler) SetWindows(ids []string) {
	s.windows = make(map[string]bool, len(ids))
	for _, id := range ids {
		s.windows[id] = true
	}
	s.known = true
}
func (s *Scheduler) Windows() (bool, map[string]bool) { return s.known, maps.Clone(s.windows) }
func (s *Scheduler) Known() bool                      { return s.known }
func (s *Scheduler) HasWindow(id string) bool         { return s.windows[id] }
func (s *Scheduler) FailedSince(id string, since time.Time) error {
	f, ok := s.failed[id]
	if !ok || f.at.Before(since) {
		return nil
	}
	return f.err
}

// BackgroundPending counts the connects that use the background slots, which
// a warm connect does not.
func (s *Scheduler) BackgroundPending() int {
	n := 0
	for _, r := range s.connecting {
		if r.background && !r.warm {
			n++
		}
	}
	return n
}

// WarmPending counts the warm connects in progress.
func (s *Scheduler) WarmPending() int {
	n := 0
	for _, r := range s.connecting {
		if r.warm {
			n++
		}
	}
	return n
}

// Warmed reports whether the warm pass has been made, which happens once
// while the window runs.
func (s *Scheduler) Warmed() bool { return s.warmed }

// QueueWarm queues the chats to open at start, in order, at most WarmChats of
// them, and ends the pass: a later call does nothing. A chat already
// connecting or queued is left out.
func (s *Scheduler) QueueWarm(chats []Decision) {
	if s.warmed {
		return
	}
	s.warmed = true
	for _, d := range chats {
		if len(s.warmQueue) >= WarmChats {
			break
		}
		if _, asked := s.connecting[d.Chat]; asked || slices.Contains(s.queue, d.Chat) {
			continue
		}
		// A chat with a window is warmed too, since the layout restores the
		// window with the saved copy only; it is not a background chat.
		d.Background = !s.windows[d.Chat]
		s.warmQueue = append(s.warmQueue, d)
	}
}

// NextWarm returns the next warm chat to connect while fewer than
// WarmConcurrency are connecting. open says whether a chat is already open,
// in which case it is dropped.
func (s *Scheduler) NextWarm(open func(chat string) bool, now time.Time) (Decision, bool) {
	for len(s.warmQueue) > 0 && s.WarmPending() < WarmConcurrency {
		d := s.warmQueue[0]
		s.warmQueue = s.warmQueue[1:]
		if _, asked := s.connecting[d.Chat]; asked || open(d.Chat) {
			continue
		}
		s.connecting[d.Chat] = request{at: now, background: d.Background, warm: true}
		return d, true
	}
	return Decision{}, false
}
func (s *Scheduler) Expire(now time.Time) {
	if s.connecting == nil {
		s.connecting = map[string]request{}
		s.tried = map[string]int64{}
	}
	for id, r := range s.connecting {
		if now.Sub(r.at) >= ConnectPatience {
			delete(s.connecting, id)
		}
	}
}
func (s *Scheduler) Consider(chat, path string, newest int64, hidden, open bool, now time.Time) (Decision, bool) {
	if open {
		return Decision{}, false
	}
	_, asked := s.connecting[chat]
	skip := asked || hidden || s.tried[chat] >= newest
	for _, id := range s.queue {
		if id == chat {
			skip = true
			break
		}
	}
	if skip {
		return Decision{}, false
	}
	s.tried[chat] = newest
	if s.windows[chat] {
		s.connecting[chat] = request{at: now}
		return Decision{Chat: chat, Path: path}, true
	}
	s.queue = append(s.queue, chat)
	return Decision{}, false
}
func (s *Scheduler) Next(paths map[string]string, liveBackground int, now time.Time) (Decision, bool) {
	for len(s.queue) > 0 && s.BackgroundPending()+liveBackground < BackgroundSlots {
		chat := s.queue[0]
		s.queue = s.queue[1:]
		path, waiting := paths[chat]
		if !waiting {
			continue
		}
		background := !s.windows[chat]
		s.connecting[chat] = request{at: now, background: background}
		return Decision{Chat: chat, Path: path, Background: background}, true
	}
	return Decision{}, false
}
func (s *Scheduler) Complete(chat string, err error, now time.Time) Completion {
	if err != nil {
		if s.failed == nil {
			s.failed = map[string]failure{}
		}
		s.failed[chat] = failure{now, err}
	} else {
		delete(s.failed, chat)
	}
	r, asked := s.connecting[chat]
	delete(s.connecting, chat)
	for i, c := range s.queue {
		if c == chat && err == nil {
			s.queue = append(s.queue[:i:i], s.queue[i+1:]...)
			break
		}
	}
	return Completion{asked, r.background, s.windows[chat], r.warm}
}
