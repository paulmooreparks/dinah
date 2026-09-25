package answer

import (
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// Sealed is a composed answer that a head can read and encode but cannot
// rewrite. Its payload and response are unexported, so outside this package
// no expression reaches the affordance list the answer publishes, and the
// compiler refuses every attempt to write it whatever form the write takes.
// Affordances returns a copy, and a head that needs a name changed asks this
// package to change it.
//
// The HTTP head holds its answers as Sealed. The MCP head still calls Run and
// Refusal, which hand back the *verb.Response itself.
type Sealed struct {
	// payload is the value Encode publishes, with its affordances already in
	// the surface vocabulary.
	payload any
	// response is payload when payload is a *verb.Response, and nil when it
	// is a read's own answer.
	response *verb.Response
}

// RunSealed answers one command as Run does and seals the answer.
func RunSealed(command string, l *verb.Library, r *verb.Request) Sealed {
	payload, response := Run(command, l, r)
	return Sealed{payload: payload, response: response}
}

// RefusalSealed composes a refusal the head raised itself as Refusal does and
// seals the answer.
func RefusalSealed(r *verb.Request, refusal *contract.Refusal) Sealed {
	response := Refusal(r, refusal)
	return Sealed{payload: response, response: response}
}

// FromErrorSealed composes the answer to an error the head met as FromError
// does and seals the answer.
func FromErrorSealed(l *verb.Library, r *verb.Request, err error) Sealed {
	response := FromError(l, r, err)
	return Sealed{payload: response, response: response}
}

// Encode is Encode applied to the sealed payload, so a sealed answer and the
// payload it seals encode to one byte sequence.
func (s Sealed) Encode() ([]byte, error) {
	return Encode(s.payload)
}

// Outcome is the response's outcome, and empty when the answer is a read's
// own answer, which a runner produces only on success.
func (s Sealed) Outcome() string {
	if s.response == nil {
		return ""
	}
	return s.response.Outcome
}

// Refusal is the refusal name the response carries, and empty when it carries
// none or the answer is a read's own answer.
func (s Sealed) Refusal() string {
	if s.response == nil {
		return ""
	}
	return s.response.Refusal
}

// Detail is the token the response names in its detail member, and empty
// when it names none or the answer is a read's own answer.
func (s Sealed) Detail() string {
	if s.response == nil {
		return ""
	}
	return s.response.Detail
}

// CardRef is the reference of the card the response carries, and empty when
// it carries none or the answer is a read's own answer.
func (s Sealed) CardRef() string {
	if s.response == nil || s.response.Card == nil {
		return ""
	}
	return s.response.Card.Ref
}

// Revision is the revision of the card the answer carries, which is the only
// entity revision the library reports: the response's card on a response, the
// detail's card on a read that answered with one, and empty otherwise.
func (s Sealed) Revision() string {
	if s.response != nil {
		if s.response.Card != nil {
			return s.response.Card.Revision
		}
		return ""
	}
	wrapped, ok := s.payload.(map[string]any)
	if !ok {
		return ""
	}
	if detail, ok := wrapped["detail"].(*verb.Detail); ok && detail != nil {
		return detail.Card.Revision
	}
	return ""
}

// Affordances returns a copy of the affordance list the answer publishes, so
// writing to what it returns leaves the answer unchanged.
func (s Sealed) Affordances() []string {
	if s.response != nil {
		return append([]string(nil), s.response.Affordances...)
	}
	wrapped, _ := s.payload.(map[string]any)
	listed, _ := wrapped["affordances"].([]string)
	return append([]string(nil), listed...)
}
