package main

import (
	"strconv"
	"strings"

	"dinah/internal/verb"
)

// compactVersion is the version of the grammar below, carried as the last
// field of the version record that opens every compact payload. A caller
// reads it before assuming the field order this file fixes, and an
// incompatible change to any record increments it.
const compactVersion = "1"

// The compact projection is a second machine form of the answers a driver
// loop reads most: line-oriented UTF-8 rather than JSON, carrying the same
// facts in fewer tokens because it spends none of them on keys, braces and
// quotation marks.
//
// A payload is a sequence of records, one per line, ending on a single
// trailing newline. A record is fields joined by "|", and its first field is
// its kind, which says how many fields follow and what each of them means. A
// payload opens with the version record fmt|compact|1 and nothing else may
// precede it.
//
// The record kinds, with their fields in order after the kind:
//
//	fmt      compact, version
//	rsp      outcome, verb, refusal, detail, basis, warning, warning_detail, message
//	card     id, ref, title, column, column_title, state, severity, priority,
//	         holder, claim_since, expires, block_reason, block_kind, revision,
//	         then one trailing field per workstream identifier
//	wstream  id, ref, slug, title, status, cards
//	instr    global, standing, column
//	move     column, ref, title, direction, reject
//	ctx      key, value
//	msgval   key, value
//	wb       title, slug, path
//	aff      one trailing field per affordance token
//	lst      column
//	off      column, title, awaiting_outside, no_taker, taken_by_pull
//
// A verb's response and a pre-verb refusal both order their records fmt, rsp,
// card, wstream, instr, move, ctx, msgval, wb, aff, and the aff record closes
// every one of them even when it carries no token, so a reader always has a
// line marking where the block ends. An ls answer orders them fmt, lst, card.
// A next answer orders them fmt, then off followed by that offer's card where
// it carries one.
//
// Every field the canonical JSON carries appears here, decoded to the identical
// string. Nothing is dropped, renamed, truncated or summarised. JSON's
// omitempty drops an empty string and a nil pointer, and compact writes the
// same absence as an empty field or as the missing optional record, which is
// the non-distinction the Go structs already make.

// compactEncode renders a value in the compact form, and reports whether the
// form is defined for the value's own type. A caller handed false emits the
// canonical JSON instead, so a shape nobody has defined a compact rendering
// for still answers a machine.
func compactEncode(value any) (string, bool) {
	switch typed := value.(type) {
	case *verb.Response:
		return compactResponse(typed), true
	case refusalReport:
		return compactRefusal(typed), true
	case *verb.Listing:
		return compactListing(typed), true
	case []verb.Offer:
		return compactOffers(typed), true
	}
	return "", false
}

// compactPayload accumulates the records of one answer.
type compactPayload struct {
	lines []string
}

// openCompact starts a payload on its version record, which is the one record
// no caller writes for itself.
func openCompact() *compactPayload {
	payload := &compactPayload{}
	payload.record("fmt", "compact", compactVersion)
	return payload
}

// record appends one record. The kind leads it unescaped, because every kind
// is a literal of this file and none of them carries a byte the grammar
// reserves; every field after it is escaped.
func (p *compactPayload) record(kind string, fields ...string) {
	written := make([]string, 0, len(fields)+1)
	written = append(written, kind)
	for _, field := range fields {
		written = append(written, compactEscape(field))
	}
	p.lines = append(p.lines, strings.Join(written, "|"))
}

// pairs appends one record per entry of a map, ascending by key. The order is
// fixed here for the reason flattenMessageValues fixes it for a rendered
// sentence: one map has to produce one field order however the runtime walks
// it.
func (p *compactPayload) pairs(kind string, values map[string]string) {
	for _, key := range sortedKeys(values) {
		p.record(kind, key, values[key])
	}
}

// card appends a card record, and appends nothing where the answer carries no
// card, since the absence of the record is how compact writes a nil pointer.
func (p *compactPayload) card(card *verb.CardView) {
	if card == nil {
		return
	}
	fields := []string{
		card.ID,
		card.Ref,
		card.Title,
		card.Column,
		card.ColumnTitle,
		card.State,
		card.Severity,
		card.Priority,
		card.Holder,
		card.ClaimSince,
		card.Expires,
		card.BlockReason,
		card.BlockKind,
		card.Revision,
	}
	p.record("card", append(fields, card.Workstreams...)...)
}

// text joins the records into the payload a caller writes, ending on the one
// trailing newline emitCanonical's own output ends on.
func (p *compactPayload) text() string {
	return strings.Join(p.lines, "\n") + "\n"
}

// compactEscape makes a value safe to write as one field. A backslash doubles
// first, so that the escapes introduced after it are unambiguous, then the
// field separator and the two line endings take a backslash of their own. No
// field can then carry a raw newline, which is what lets a reader split a
// whole payload into records before it reads a field from any of them.
func compactEscape(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, "|", `\|`)
	escaped = strings.ReplaceAll(escaped, "\n", `\n`)
	return strings.ReplaceAll(escaped, "\r", `\r`)
}

// compactFlag writes a boolean: one byte where the flag is set, and an empty
// field where it is not, which is the same absence JSON's omitempty writes for
// a false.
func compactFlag(set bool) string {
	if set {
		return "1"
	}
	return ""
}

// compactResponse renders the shared envelope every mutating command answers
// through, which is what carries claim, move, release, block and unblock, the
// eleven other acts on a card that reach emit, and the two acts on a
// workstream that reach emitWorkstream.
func compactResponse(response *verb.Response) string {
	payload := openCompact()
	payload.record("rsp",
		response.Outcome,
		response.Verb,
		response.Refusal,
		response.Detail,
		response.Basis,
		response.Warning,
		response.WarningDetail,
		response.Message,
	)
	payload.card(response.Card)
	if workstream := response.Workstream; workstream != nil {
		payload.record("wstream",
			workstream.ID,
			workstream.Ref,
			workstream.Slug,
			workstream.Title,
			workstream.Status,
			strconv.Itoa(workstream.Cards),
		)
	}
	if instructions := response.Instructions; instructions != nil {
		payload.record("instr", instructions.Global, instructions.Standing, instructions.Column)
	}
	for _, move := range response.LegalMoves {
		payload.record("move", move.Column, move.Ref, move.Title, move.Direction, compactFlag(move.Reject))
	}
	payload.pairs("ctx", response.Context)
	payload.pairs("msgval", response.MessageValues)
	payload.record("aff", response.Affordances...)
	return payload.text()
}

// compactRefusal renders a refusal raised before any verb ran. It writes the
// rsp record the verbs write, with the members no pre-verb refusal has left
// empty, so a caller reading compact reads one shape whichever layer said no,
// which is the property refusalReport already claims for the canonical form.
func compactRefusal(report refusalReport) string {
	payload := openCompact()
	payload.record("rsp", report.Outcome, "", report.Refusal, report.Detail, "", "", "", "")
	payload.pairs("ctx", report.Context)
	for _, candidate := range report.Workbenches {
		payload.record("wb", candidate.Title, candidate.Slug, candidate.Path)
	}
	payload.record("aff")
	return payload.text()
}

// compactListing renders the answer ls gives: the column listed, then the cards
// in the order CORE-QUEUE-3 fixes.
func compactListing(listing *verb.Listing) string {
	payload := openCompact()
	payload.record("lst", listing.Column)
	for i := range listing.Cards {
		payload.card(&listing.Cards[i])
	}
	return payload.text()
}

// compactOffers renders the answer next gives: one offer per column in slice
// order, each followed by its card where it offers one and by nothing where it
// does not.
func compactOffers(offers []verb.Offer) string {
	payload := openCompact()
	for _, offer := range offers {
		payload.record("off",
			offer.Column,
			offer.Title,
			compactFlag(offer.AwaitingOutside),
			compactFlag(offer.NoTaker),
			compactFlag(offer.TakenByPull),
		)
		payload.card(offer.Card)
	}
	return payload.text()
}
