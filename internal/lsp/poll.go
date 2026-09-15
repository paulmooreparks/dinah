package lsp

import (
	"sort"
	"time"

	"dinah/internal/bench"
	"dinah/internal/verb"
)

// poll is the loop that gives every answer its live accuracy. It rides the
// library's own change checkpoint, which rests on file content and on nothing
// else: no modification time is read, no filesystem watcher is registered,
// and no platform notification behaviour is depended on.
//
// The loop sleeps after a walk completes rather than firing on a fixed
// schedule, so two walks are never in flight at once and a slow workbench
// degrades refresh latency instead of saturating a core. The sleep is the
// larger of the interval and the duration of the walk just finished, which
// caps the walk at half the process's wall time whatever the workbench's
// size.
func (s *Server) poll() {
	for {
		started := s.opts.Now()
		changed := s.tick()
		s.mu.Lock()
		s.walked = s.opts.Since(started)
		interval, walked := s.interval, s.walked
		entered, left := s.crossed()
		s.mu.Unlock()

		if entered {
			_ = s.log(messageTypeWarning, keyLogSlowWalkEntered, "duration", walked.String(), "interval", interval.String())
		}
		if left {
			_ = s.log(messageTypeWarning, keyLogSlowWalkLeft, "duration", walked.String(), "interval", interval.String())
		}
		if changed {
			s.publish()
		}
		select {
		case <-s.stop:
			return
		default:
		}
		s.opts.Sleep(sleepFor(interval, walked))
		select {
		case <-s.stop:
			return
		default:
		}
	}
}

// crossed reads the slow-walk state against the walk that has just finished
// and reports which edge, if either, it crossed. The caller holds the lock.
//
// The state is a comparison of the last walk against the interval in force
// when it finished, not a memory of how slow the workbench once was. So
// raising the interval above the last walk's duration leaves the state, and
// that is the intended reading: an operator who raised the interval to stop
// the overrun wants to be told the overrun has stopped.
func (s *Server) crossed() (entered, left bool) {
	slow := s.walked > s.interval
	switch {
	case slow && !s.slow:
		s.slow = true
		return true, false
	case !slow && s.slow:
		s.slow = false
		return false, true
	}
	return false, false
}

// tick runs one change checkpoint and, when it reports a change, rebuilds
// everything the server holds. It answers whether an open document's model
// actually moved.
func (s *Server) tick() bool {
	if s.opts.Walk != nil {
		return s.opts.Walk()
	}
	s.mu.Lock()
	if s.library == nil {
		// Nothing resolved yet, so each tick retries discovery. A folder
		// that will have a workbench in a minute is a folder this server
		// starts serving without a restart.
		root := s.searched(s.scopeStart())
		s.openAt(root)
		resolved := s.bench != nil
		s.mu.Unlock()
		if !resolved {
			return false
		}
		s.mu.Lock()
	}
	set, err := s.library.Changes(&verb.Request{Verb: "changes", Since: s.cursor})
	if err != nil {
		s.mu.Unlock()
		return false
	}
	s.cursor = set.Cursor
	if !set.Changed {
		s.mu.Unlock()
		return false
	}
	root := s.bench.Root
	moved := s.refresh(root, set)
	s.mu.Unlock()
	return len(moved) > 0
}

// refresh is what a tick does when the checkpoint reports a change: re-open
// the workbench, discard the memo entirely, drop what the answer says has
// gone, and recompute every open document's model. The caller holds the lock.
//
// The memo is discarded whole rather than by the entries the answer names.
// That selective rule is wrong for the case the widened checkpoint exists to
// fix: a hand-edited column moves the column term and names nothing in
// events, cards or gone, so a selective drop would keep the stale column
// label for good. A whole discard is also cheaper than it looks, the re-open
// above having already re-read every anchor.
func (s *Server) refresh(root string, set *verb.ChangeSet) []string {
	if opened, err := bench.Open(root); err == nil {
		s.bench = opened
		s.library = verb.New(opened, "")
		s.scopeRoot = scopeRootOf(opened)
	}
	s.memo = map[memoKey]memoed{}
	for _, gone := range set.Gone {
		s.forget(gone.ID)
	}
	return s.recompute()
}

// forget drops what was retained for an entity the checkpoint reports as
// gone, which is the positive evidence section 3.3 requires before a retained
// annotation is given up. The caller holds the lock.
func (s *Server) forget(id string) {
	if id == "" {
		return
	}
	for key, held := range s.retained {
		if held.Target != nil && held.Target.ID == id {
			delete(s.retained, key)
			continue
		}
		if held.Target != nil && held.Target.Card != nil && *held.Target.Card == id {
			delete(s.retained, key)
		}
	}
}

// scopeStart is the directory a retry of discovery searches from, which is
// the process working directory once the explicit rungs and the client's own
// have all been tried and failed. The caller holds the lock.
func (s *Server) scopeStart() string {
	return s.opts.Wd
}

// publish tells the client the models moved: the protocol's own refresh
// request, sent only to a client that declared it supports one, and the
// namespaced notification the VS Code client redraws its chip from.
func (s *Server) publish() {
	s.mu.Lock()
	refresh := s.refreshSupport
	uris := make([]string, 0, len(s.docs))
	for uri := range s.docs {
		uris = append(uris, uri)
	}
	s.mu.Unlock()
	sort.Strings(uris)

	if refresh {
		_ = s.conn.request(methodInlayHintRefresh, nil)
	}
	_ = s.conn.notify(methodAnnotationsChanged, annotationsChangedParams{URIs: uris})
}

// sleepFor is the wait one turn of the loop takes: the larger of the
// configured interval and the walk just finished. It is written out so a
// reader sees that the second operand is an observation of what happened
// rather than a policy about how long a value stays good.
func sleepFor(interval, walked time.Duration) time.Duration {
	if walked > interval {
		return walked
	}
	return interval
}
