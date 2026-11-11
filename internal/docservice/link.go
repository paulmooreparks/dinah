package docservice

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// Command is something the service asks of a document's window process:
// come forward, change the title, show one of the system's dialogs. A
// command that has an answer is answered with its number.
type Command struct {
	Seq  int64           `json:"seq"`
	Kind string          `json:"kind"`
	Data json.RawMessage `json:"data,omitempty"`
}

// link carries commands to one window process. The process asks for what
// comes after the last command it has, and waits a while when there is
// nothing, so the service always has a way to reach a window and knows by
// the asking that the window is still there.
type link struct {
	mu      sync.Mutex
	next    int64
	queue   []Command
	wake    chan struct{}
	replies map[int64]chan json.RawMessage
	// asking counts the requests waiting for a command now, and asked is
	// when the last one came or went.
	asking int
	asked  time.Time
	seen   bool
	closed bool
	// pid is the window's process, as it said when it asked.
	pid int
}

func newLink() *link {
	return &link{wake: make(chan struct{}), replies: map[int64]chan json.RawMessage{}, asked: time.Now()}
}

// send queues a command and returns its number.
func (l *link) send(kind string, data any) int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.publish(kind, data)
}

// publish queues a command for a caller that has the lock.
func (l *link) publish(kind string, data any) int64 {
	var raw json.RawMessage
	if data != nil {
		raw, _ = json.Marshal(data)
	}
	l.next++
	l.queue = append(l.queue, Command{Seq: l.next, Kind: kind, Data: raw})
	close(l.wake)
	l.wake = make(chan struct{})
	return l.next
}

// ask queues a command and waits for the window's answer to it. The place
// for the answer is made before the window can see the command, so an
// answer that comes at once is not lost.
func (l *link) ask(ctx context.Context, kind string, data any) (json.RawMessage, error) {
	answer := make(chan json.RawMessage, 1)
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return nil, errors.New("the window has closed")
	}
	seq := l.publish(kind, data)
	l.replies[seq] = answer
	l.mu.Unlock()
	defer func() {
		l.mu.Lock()
		delete(l.replies, seq)
		l.mu.Unlock()
	}()
	select {
	case raw, ok := <-answer:
		if !ok {
			return nil, errors.New("the window has closed")
		}
		return raw, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// reply gives the answer to a command that waits for one.
func (l *link) reply(seq int64, raw json.RawMessage) {
	l.mu.Lock()
	answer := l.replies[seq]
	delete(l.replies, seq)
	l.mu.Unlock()
	if answer != nil {
		answer <- raw
	}
}

// after returns the commands numbered above seq, waiting up to wait for
// one. Commands up to seq are the window's and are forgotten.
func (l *link) after(ctx context.Context, seq int64, wait time.Duration, pid int) []Command {
	l.mu.Lock()
	if pid > 0 {
		l.pid = pid
	}
	l.asking++
	l.asked, l.seen = time.Now(), true
	defer func() {
		l.mu.Lock()
		l.asking--
		l.asked = time.Now()
		l.mu.Unlock()
	}()
	deadline := time.NewTimer(wait)
	defer deadline.Stop()
	for {
		kept := l.queue[:0]
		for _, c := range l.queue {
			if c.Seq > seq {
				kept = append(kept, c)
			}
		}
		l.queue = kept
		if len(l.queue) > 0 || l.closed {
			out := append([]Command{}, l.queue...)
			l.mu.Unlock()
			return out
		}
		wake := l.wake
		l.mu.Unlock()
		select {
		case <-wake:
		case <-deadline.C:
			return nil
		case <-ctx.Done():
			return nil
		}
		l.mu.Lock()
	}
}

// process is the window's process, or 0 before it has asked.
func (l *link) process() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.pid
}

// gone says the window process has ended: nothing is asking for commands,
// and nothing has for longer than a window stays silent. A window that has
// not asked yet is given the time a process takes to start.
func (l *link) gone(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.asking > 0 {
		return false
	}
	if !l.seen {
		return now.Sub(l.asked) > startGrace
	}
	return now.Sub(l.asked) > windowGrace
}

// close ends the link: whoever waits for an answer is told the window has
// closed.
func (l *link) close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return
	}
	l.closed = true
	for seq, answer := range l.replies {
		close(answer)
		delete(l.replies, seq)
	}
	close(l.wake)
	l.wake = make(chan struct{})
}
