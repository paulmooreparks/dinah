package mcp

import (
	"strings"
	"sync"
	"time"

	"dinah/internal/verb"
)

// HoldTTL is how long the head may withhold a layer it has already served
// before serving it again unasked. It bounds the window in which an agent that
// has lost the text recovers only by acting on the withheld marker.
//
// Fifteen minutes is longer than any single act on this surface takes and far
// shorter than the hours a stdio session runs, which is the asymmetry that
// makes the bound cheap.
const HoldTTL = 15 * time.Minute

// HoldActCeiling is how many further tool calls the head may answer for one
// owner on one connection before serving a withheld layer again unasked. The
// call after the ceiling serves in full, so a ceiling of twenty means twenty
// further calls still withhold and the twenty-first does not.
//
// Twenty calls is longer than one card's whole six-call sequence and shorter
// than three cards' worth.
const HoldActCeiling = 20

// chainRecord is one layer's serve: when it went out, and how many tool calls
// the head had answered for that owner at the time.
type chainRecord struct {
	// served is the wall-clock instant the layer last went out in full.
	served time.Time
	// calls is the owner's tool-call counter at that instant.
	calls int
}

// chainMemory is what one connection remembers of the instruction chain it has
// already sent. It holds a record per (owner, layer text) pair and a tool-call
// counter per owner, and it answers the one question the rule turns on: which
// layers may this act withhold from this owner right now.
//
// The head records what it sent, and what the design needs to know is what the
// agent still holds. Those are the same thing only for as long as the agent's
// context carries the text, and MCP publishes no capability by which a server
// learns what a client still holds. So this type expires its own records on its
// own clock and its own counter rather than resting anything on how a client
// manages its context.
//
// The mutex is defensive rather than needed today, because Serve scans one line
// at a time in a single goroutine. It is cheap insurance against a later
// concurrent head.
type chainMemory struct {
	mu sync.Mutex
	// now is the clock, injectable so a test drives expiry without sleeping.
	now func() time.Time
	// sent holds one record per chain key, which verb.chainKey writes as the
	// owner, a NUL, and the revision of the text that was served.
	sent map[string]chainRecord
	// calls counts every tool call this connection has answered for an owner.
	// Every call counts and not only the acts that consult the chain, because
	// the exposure being bounded is the agent's context churn and every call
	// contributes to that.
	calls map[string]int
}

// newChainMemory builds the memory one connection keeps for its own life.
func newChainMemory() *chainMemory {
	return &chainMemory{
		now:   time.Now,
		sent:  map[string]chainRecord{},
		calls: map[string]int{},
	}
}

// open counts one further tool call for an owner and answers the chain keys
// that owner may still have withheld from it. Expiry is resolved here, before
// the set is built, so the library sees a set that is already true at the
// moment of the call and holds no clock of its own.
//
// A call naming no owner counts for nobody and withholds nothing. The library
// refuses such an act anyway, and a nil set serves the whole chain.
func (m *chainMemory) open(actor string) map[string]bool {
	if m == nil || actor == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls[actor]++
	now := m.now()
	prefix := actor + "\x00"
	held := map[string]bool{}
	for key, record := range m.sent {
		if !strings.HasPrefix(key, prefix) {
			continue
		}
		if now.Sub(record.served) >= HoldTTL || m.calls[actor]-record.calls > HoldActCeiling {
			delete(m.sent, key)
			continue
		}
		held[key] = true
	}
	if len(held) == 0 {
		return nil
	}
	return held
}

// record writes down the layers an act has just served in full to an owner. It
// runs after the tool has answered and before the answer is encoded, which is
// safe because a failed encode ends the process.
func (m *chainMemory) record(actor string, keys []string) {
	if m == nil || actor == "" || len(keys) == 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	at := m.now()
	for _, key := range keys {
		m.sent[key] = chainRecord{served: at, calls: m.calls[actor]}
	}
}

// chainServed reads the served-layer report off whatever shape a tool answered
// with. A coordination act answers a *verb.Response, the instructions tool
// answers a *verb.Served under the member the surface publishes it as, and
// every other shape served no chain and records nothing.
func chainServed(payload any) []string {
	switch answer := payload.(type) {
	case *verb.Response:
		return answer.ChainServed
	case *verb.Served:
		return answer.ChainServed
	case map[string]any:
		if served, ok := answer["served"].(*verb.Served); ok {
			return served.ChainServed
		}
	}
	return nil
}
