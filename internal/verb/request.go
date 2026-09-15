package verb

import (
	"dinah/internal/bench"
	"dinah/internal/contract"
)

// Acting composes the actor block every event this package writes carries: the
// owner the act is attributed to, and whatever the caller declared about what
// performed it.
//
// One function composes it, and every write site in this package calls that
// function. The alternative is what makes this worth a function of its own:
// bench.Event is built at about forty sites across this package and
// internal/bench, and forty hand repairs is how one event family ends up
// carrying the name alone while the others carry the provenance. The sibling
// composer is bench.NamedActor, which serves the writes that have no request
// to read the declared facts from.
//
// A member the caller did not declare is left absent rather than stamped with
// a literal unknown, because a workbench is free to run a provider named
// unknown and a magic value would collide with it.
func (r *Request) Acting() bench.Actor {
	return bench.Actor{
		Name:     r.Actor,
		Harness:  r.Harness,
		Provider: r.Provider,
		Model:    r.Model,
		Server:   r.Server,
	}
}

// DeclaredAgent is what the caller declared about what is performing the act,
// in the shape the resolver and whoami read it as.
func (r *Request) DeclaredAgent() bench.Agent {
	return bench.Agent{
		Harness:  r.Harness,
		Provider: r.Provider,
		Model:    r.Model,
		Server:   r.Server,
	}
}

// malformedHarness refuses an act that writes a journal line under a declared
// harness name outside the one-segment grammar, so a harness that misspells
// its own name learns about it on its first call rather than in a journal
// nobody reads.
//
// It reaches a write and never a read. A mistyped variable that stopped show
// and ls would take the whole tool away from whoever has to repair it, and a
// read stamps nothing a malformed name could damage. dinah whoami is where a
// person meets the value instead, reported and marked malformed.
func (l *Library) malformedHarness(req *Request, card *bench.Card) *Response {
	if req.Harness == "" || bench.HarnessName(req.Harness) {
		return nil
	}
	return l.refuse(req, card, contract.MalformedHarness, req.Harness)
}
