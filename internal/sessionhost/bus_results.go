package sessionhost

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"studio/internal/applet/broker"
	"studio/internal/bus"
)

//go:embed bus_results.go
var BusResultsSource []byte

const ChatAskAction = "dinah.chat/ask@2"

func ChatResultKey(workspace, id string) string {
	return "chat-result:" + bus.WorkspaceKey(workspace) + ":" + id
}

// A result that cannot be queued stays saved, and is tried again after a delay that doubles from resultRetryFirst
// to resultRetryLongest, and again at once when any chat joins the bus. Only its first failure in a run is logged:
// 27 results logged every retry wrote some 16,000 errors in three days (9 November 2026).
const (
	resultRetryFirst   = 5 * time.Second
	resultRetryLongest = 10 * time.Minute
)

type resultRetry struct {
	delay time.Duration
	next  time.Time
}

type resultRetries struct {
	mu sync.Mutex
	by map[string]resultRetry
}

// due says whether a result may be tried now.
func (r *resultRetries) due(key string, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, waiting := r.by[key]
	return !waiting || !now.Before(t.next)
}

// failed records another failure and returns the delay before the next try.
func (r *resultRetries) failed(key string, now time.Time) time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.by == nil {
		r.by = map[string]resultRetry{}
	}
	t := r.by[key]
	switch {
	case t.delay == 0:
		t.delay = resultRetryFirst
	case t.delay < resultRetryLongest:
		t.delay = min(t.delay*2, resultRetryLongest)
	}
	t.next = now.Add(t.delay)
	r.by[key] = t
	return t.delay
}

func (r *resultRetries) succeeded(key string) {
	r.mu.Lock()
	delete(r.by, key)
	r.mu.Unlock()
}

// reset makes every waiting result due at once, for when something that could let one through has happened.
func (r *resultRetries) reset() {
	r.mu.Lock()
	for k, t := range r.by {
		t.next = time.Time{}
		r.by[k] = t
	}
	r.mu.Unlock()
}

// reconcileChatResults moves saved completions to the bus. The index avoids
// rereading unrelated records; reported avoids rereading results this process
// already queued. The bus key also deduplicates across process restarts.
func (a *BusChats) ReconcileResults(ctx context.Context, b *bus.Bus, reported map[string]bool) {
	host, err := a.host.ResultBroker()
	if err != nil {
		a.host.Log("error", "reading completed chat requests: %v", err)
		return
	}
	results, err := host.Attention(func(s broker.Summary) bool {
		return s.Action == ChatAskAction && s.OwnerKind == "chat" && broker.TerminalStatus(s.Status) && !reported[ChatResultKey(s.Workspace, s.ID)]
	})
	if err != nil {
		a.host.Log("error", "recovering chat request results: %v", err)
		return
	}
	now := time.Now()
	if a.host.Clock != nil {
		now = a.host.Clock()
	}
	for _, v := range results {
		key := ChatResultKey(v.Workspace, v.ID)
		if !a.retries.due(key, now) {
			continue
		}
		if err := a.EnqueueResult(ctx, b, v); err != nil {
			if errors.Is(err, errCallerRemoved) {
				// Nothing can take it; it stays saved until it is pruned.
				a.host.Log("info", "chat request %s's result is not queued: %v", v.ID, err)
				reported[key] = true
				continue
			}
			if delay := a.retries.failed(key, now); delay == resultRetryFirst {
				a.host.Log("error", "queueing chat request %s's result: %v; it is tried again, and further failures are not logged", v.ID, err)
			}
			continue
		}
		a.retries.succeeded(key)
		reported[key] = true
	}
}

// errCallerRemoved is the failure for a result whose caller was removed from
// Dinah, which nothing can take.
var errCallerRemoved = errors.New("the chat that asked was removed")

// EnqueueResult addresses only the caller's bus identity: its own, or the
// one its continuation took over (SavedBusIdentity). A closed caller keeps
// its result until it reopens; another chat in that workspace must never
// consume it.
//
// A result already on the bus is done, whoever it went to. Every saved
// result is checked again after a restart, and the caller's identity may
// since have passed to another chat; looking the identity up first made 27
// results delivered in October fail on every retry (9 November 2026).
func (a *BusChats) EnqueueResult(ctx context.Context, b *bus.Bus, v broker.Invocation) error {
	if v.Action != ChatAskAction || v.OwnerKind != "chat" || !broker.TerminalStatus(v.Status) {
		return errors.New("only a completed asynchronous chat request has a result to deliver")
	}
	if queued, err := b.ResultQueued(ctx, ChatResultKey(v.Workspace, v.ID)); err != nil {
		return err
	} else if queued {
		return nil
	}
	id := a.host.SavedBusIdentity(v.OwnerID)
	if id == "" {
		if a.host.Removed != nil && a.host.Removed(v.OwnerID) {
			return errCallerRemoved
		}
		return errors.New("the caller has no bus identity; its result remains saved for retry")
	}
	report := bus.HostReport{InvocationID: v.ID, Request: json.RawMessage(v.InputJSON), Status: v.Status, Output: json.RawMessage(v.OutputJSON), Error: v.Error}
	// The report's identity is its JSON (bus.EnqueueResult), so a result
	// saved before results had text must make the report it made then.
	// Its text and the answering chat's title are therefore taken only
	// from a result that has text; one saved earlier has neither, and is
	// shown as its output.
	if v.Text != "" {
		var out struct {
			Title string `json:"title"`
		}
		_ = json.Unmarshal([]byte(v.OutputJSON), &out)
		report.Text, report.From = v.Text, out.Title
	}
	text, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("reading the saved chat result: %w", err)
	}
	_, err = b.EnqueueResult(ctx, ChatResultKey(v.Workspace, v.ID), id, string(text))
	return err
}
