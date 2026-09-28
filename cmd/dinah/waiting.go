package main

import (
	"sync/atomic"

	"dinah/internal/durable"
	"dinah/internal/verb"
)

// waitingSink is where a wait notice goes in this process: standard error for
// an ordinary command, and the head's own channel once a head has started.
// main sets durable.Waiting once, to forwardWaiting, and every head installs
// its sink here when it starts, before it serves anything. The indirection
// keeps durable.Waiting written once per process while the sink it reaches
// can change from the command line's to a head's.
var waitingSink atomic.Pointer[func(durable.Wait)]

// installWaiting makes sink the one a wait notice reaches.
func installWaiting(sink func(durable.Wait)) {
	waitingSink.Store(&sink)
}

// forwardWaiting hands a wait notice to the installed sink.
func forwardWaiting(wait durable.Wait) {
	sink := waitingSink.Load()
	if sink == nil {
		return
	}
	(*sink)(wait)
}

// noticeWaiting prints a wait notice to standard error, prefixed dinah:, which
// is how an ordinary command surfaces a wait.
func (s *session) noticeWaiting(wait durable.Wait) {
	s.errLine("dinah: " + verb.WaitingNotice(s.r, []string{s.workbenchRoot}, wait))
}
