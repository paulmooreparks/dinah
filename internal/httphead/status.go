package httphead

import (
	"net/http"

	"dinah/internal/contract"
)

// refusalStatus maps the refusal names that do not answer 409 to the status
// they do answer. Every name it does not list answers 409, because most
// refusals say the workbench is in a state that forbids the act, and a name
// added later lands there without a row. The rows are the refusals about who
// is asking (403), a reference naming nothing (404), and a malformed request
// (400), beside the head's own transport refusals.
var refusalStatus = map[string]int{
	contract.Malformed:            http.StatusBadRequest,
	contract.NoReason:             http.StatusBadRequest,
	contract.Usage:                http.StatusBadRequest,
	contract.MalformedHarness:     http.StatusBadRequest,
	contract.MalformedDepth:       http.StatusBadRequest,
	contract.MalformedMemberName:  http.StatusBadRequest,
	contract.UnknownField:         http.StatusBadRequest,
	contract.UnknownValue:         http.StatusBadRequest,
	contract.UnknownAxis:          http.StatusBadRequest,
	contract.RepeatedAxis:         http.StatusBadRequest,
	contract.UnknownDepth:         http.StatusBadRequest,
	contract.ChainTooLong:         http.StatusBadRequest,
	contract.MultipleWords:        http.StatusBadRequest,
	contract.EmptySearch:          http.StatusBadRequest,
	contract.NotOperator:          http.StatusForbidden,
	contract.NotHolder:            http.StatusForbidden,
	contract.NotRequester:         http.StatusForbidden,
	contract.NoOwner:              http.StatusForbidden,
	contract.BelowTier:            http.StatusForbidden,
	contract.ForeignOrigin:        http.StatusForbidden,
	contract.OriginRequired:       http.StatusForbidden,
	contract.UnknownCard:          http.StatusNotFound,
	contract.UnknownColumn:        http.StatusNotFound,
	contract.UnknownWorkstream:    http.StatusNotFound,
	contract.UnknownView:          http.StatusNotFound,
	contract.UnknownResource:      http.StatusNotFound,
	contract.MethodNotAllowed:     http.StatusMethodNotAllowed,
	contract.NotAcceptable:        http.StatusNotAcceptable,
	contract.BodyTooLarge:         http.StatusRequestEntityTooLarge,
	contract.UnsupportedMediaType: http.StatusUnsupportedMediaType,
	contract.ForeignHost:          http.StatusMisdirectedRequest,
	contract.BasisRequired:        http.StatusPreconditionRequired,
	contract.NotImplemented:       http.StatusNotImplemented,
}

// statusFor is the one place a status is chosen for an answer the library or
// package answer composed. An ok answer is 200 here, and the route replaces
// it with its own success status.
func statusFor(outcome, refusal string) int {
	switch outcome {
	case contract.OutcomeOK:
		return http.StatusOK
	case contract.OutcomeStale:
		return http.StatusPreconditionFailed
	case contract.OutcomeUnreachable:
		return http.StatusServiceUnavailable
	}
	if status, listed := refusalStatus[refusal]; listed {
		return status
	}
	return http.StatusConflict
}
