package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"studio/internal/bridge"
)

// The page opens a saved chat without holding a call open while the agent
// loads it (docs/design/chat-open-rules.md). A page call is one HTTP request,
// and a browser limits how many requests to one host it runs at a time, so
// calls that wait for an agent can leave the page unable to make any other
// call. On 21 November 2026 five opens that were never answered did that to
// the whole window.
//
// BeginChatOpen starts one of the opening methods below and returns at once.
// The result is sent as the chat:open-result event, on the stream every
// other event uses, and the page's waiting promise is settled with it, so
// the page's code after an open is what it was. The methods themselves are
// unchanged and still wait for the agent, which is what their other callers
// want: the task host, the service chat port, the bus wake and the tests.
//
// The page chooses the token, since the event can arrive before the answer
// to BeginChatOpen does. A page whose event stream missed the result asks
// again with the same token and is given the state or the outcome
// (dinah:resync, internal/bridge). A page that was reloaded has new tokens,
// and its open of a chat that is still loading joins that load
// (SessionLifecycle.openExact).

// chatOpenEvent is the event that gives the page the outcome of an open.
const chatOpenEvent = "chat:open-result"

// keptChatOpens is how many finished opens are kept for a page that asks
// again. A page asks again only for an open it still waits for, straight
// after its stream reconnects, so a few hundred is far more than it needs.
const keptChatOpens = 256

// ChatOpenOutcome is how an open ended: the chat's information, as the
// opening method returns it, or its error, with the error's detail for the
// page when it has one.
type ChatOpenOutcome struct {
	Open   string          `json:"open"`
	Method string          `json:"method"`
	Info   *SessionInfo    `json:"info,omitempty"`
	Error  string          `json:"error,omitempty"`
	Detail json.RawMessage `json:"detail,omitempty"`
	// Advice is one sentence the provider's module adds for a failed open,
	// which the page shows below the error (agent_threads.go).
	Advice string `json:"advice,omitempty"`
}

// ChatOpenState is what BeginChatOpen returns: "opening" while the open runs,
// or "done" with the outcome for an open that has already ended.
type ChatOpenState struct {
	State   string           `json:"state"`
	Outcome *ChatOpenOutcome `json:"outcome,omitempty"`
}

type chatOpens struct {
	mu sync.Mutex
	// byToken has every open that runs, with a nil outcome, and the last
	// keptChatOpens that ended. ended lists the latter, oldest first.
	byToken map[string]*ChatOpenOutcome
	ended   []string
}

// begin records a new open and reports false for a token already known,
// with its outcome when it has ended.
func (o *chatOpens) begin(token string) (*ChatOpenOutcome, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if outcome, known := o.byToken[token]; known {
		return outcome, false
	}
	if o.byToken == nil {
		o.byToken = map[string]*ChatOpenOutcome{}
	}
	o.byToken[token] = nil
	return nil, true
}

func (o *chatOpens) finish(outcome ChatOpenOutcome) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.byToken[outcome.Open] = &outcome
	o.ended = append(o.ended, outcome.Open)
	for len(o.ended) > keptChatOpens {
		delete(o.byToken, o.ended[0])
		o.ended = o.ended[1:]
	}
}

// BeginChatOpen starts the opening method the page asks for, with the
// arguments the page would have called it with, and returns without waiting
// for it. The outcome
// follows as a chat:open-result event with the same token. A token that is
// already known starts nothing: the answer is "opening", or "done" with the
// outcome, which is how a page that may have missed the event asks again.
func (a *App) BeginChatOpen(token, method string, args []json.RawMessage) (ChatOpenState, error) {
	if token == "" || len(token) > 128 {
		return ChatOpenState{}, errors.New("the open has no usable token")
	}
	call, err := a.chatOpenCall(method, args)
	if err != nil {
		return ChatOpenState{}, err
	}
	outcome, fresh := a.pageOpens.begin(token)
	if !fresh {
		if outcome != nil {
			return ChatOpenState{State: "done", Outcome: outcome}, nil
		}
		return ChatOpenState{State: "opening"}, nil
	}
	go func() {
		// The open is unfinished work for a window asked to close, under the
		// name of the method the page called before (exit_guard.go).
		if done := a.observeExitCall(method); done != nil {
			defer done()
		}
		outcome := ChatOpenOutcome{Open: token, Method: method}
		func() {
			// A fault in a page call is contained by the HTTP server, and one
			// here is contained in the same way, so the page is told.
			defer func() {
				if recover() != nil {
					logf("chat", "error", "the open %s started by the page ended in a fault", method)
					outcome.Info, outcome.Error = nil, "The window could not finish opening this chat."
				}
			}()
			info, err := call()
			if err != nil {
				outcome.Error = err.Error()
				outcome.Advice = a.openFailureAdvice(chatOpenTarget(method, args))
				var detailed bridge.DetailedError
				if errors.As(err, &detailed) {
					if raw, encErr := json.Marshal(detailed.ErrorDetail()); encErr == nil {
						outcome.Detail = raw
					}
				}
				return
			}
			outcome.Info = &info
		}()
		a.pageOpens.finish(outcome)
		a.emit(chatOpenEvent, outcome)
	}()
	return ChatOpenState{State: "opening"}, nil
}

// chatOpenCall is the opening method asked for, bound to its arguments. The
// methods listed are the ones that open or reopen a saved chat and wait for
// its agent.
func (a *App) chatOpenCall(method string, args []json.RawMessage) (func() (SessionInfo, error), error) {
	// decode reads the arguments as the bridge reads a call's: one that is
	// missing or null keeps its zero value, and one too many is an error.
	decode := func(into ...any) error {
		if len(args) > len(into) {
			return fmt.Errorf("%s takes %d arguments, and %d were given", method, len(into), len(args))
		}
		for i, raw := range args {
			if len(raw) == 0 || string(raw) == "null" {
				continue
			}
			if err := json.Unmarshal(raw, into[i]); err != nil {
				return fmt.Errorf("%s's argument %d: %w", method, i+1, err)
			}
		}
		return nil
	}
	var first, second, option string
	var value json.RawMessage
	var always bool
	switch method {
	case "OpenSessionAt":
		return func() (SessionInfo, error) { return a.OpenSessionAt(first, second) }, decode(&first, &second)
	case "LoadFullHistory":
		return func() (SessionInfo, error) { return a.LoadFullHistory(first, second) }, decode(&first, &second)
	case "OpenWaitingSession":
		return func() (SessionInfo, error) { return a.OpenWaitingSession(first, second) }, decode(&first, &second)
	case "RetryWaitingSession":
		return func() (SessionInfo, error) { return a.RetryWaitingSession(first) }, decode(&first)
	case "OpenSessionWith":
		return func() (SessionInfo, error) { return a.OpenSessionWith(first, second, option, value, always) }, decode(&first, &second, &option, &value, &always)
	case "ReopenSessionWith":
		return func() (SessionInfo, error) { return a.ReopenSessionWith(first, second, option, value) }, decode(&first, &second, &option, &value)
	}
	return nil, fmt.Errorf("%s is not a way to open a chat", method)
}
