package verb

import (
	"dinah/internal/contract"
)

// ComposeRefusal builds a refused response from a request and a Refusal,
// without a library to read affordances from. It is what an MCP head that has
// no default workbench calls when its own per-call checks refuse, and what
// FromError delegates the refusal branch to so the two cannot compose
// different shapes for one refusal.
//
// Affordances come from the same default set Library.affordances returns when
// there is no card to read them from, since a caller reaching this site has
// no entity either: the refuse happened before any workbench answered the
// call. The basis the request carried travels through unchanged, so a retry
// the caller computes from the response is the same retry the request named.
//
// The response is what the contract names a refusal: outcome refused, the
// refusal's own name and detail, and the named values the shape declared in
// Extra. There is no card, since the call never reached one.
func ComposeRefusal(req *Request, refusal *contract.Refusal) *Response {
	return &Response{
		Outcome:     contract.OutcomeRefused,
		Verb:        req.Verb,
		Refusal:     refusal.Name,
		Detail:      refusal.Detail,
		Affordances: defaultAffordances(),
		Basis:       req.Basis,
		Context:     refusal.Extra,
	}
}

// defaultAffordances is what a refused response carries when no library
// answered the call. It is the same set Library.affordances returns for a
// nil card, lifted out of the method so a caller with no library reads one
// thing rather than reading a method that wants one.
func defaultAffordances() []string {
	return []string{"status", "columns", "ls", "next"}
}
