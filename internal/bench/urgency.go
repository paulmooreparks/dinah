package bench

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	"dinah/internal/contract"
)

// UrgencyKey is the top-level frontmatter key the weights of the urgency
// order are declared under. It is read from the workbench's own
// workbench.md and from nowhere else.
const UrgencyKey = contract.UrgencyKey

// The eight terms of the urgency order, by the key the dinah.urgency block
// spells each one with. UrgencyTerms holds them in the fixed order every
// explanation and every Why cell reads them in.
const (
	UrgencyWaitsOnYou   = "waits-on-you"
	UrgencyYourQuestion = "your-question"
	UrgencyPriority     = "priority"
	UrgencySeverity     = "severity"
	UrgencyBlocked      = "blocked"
	UrgencyBlocksOthers = "blocks-others"
	UrgencyAge          = "age"
	UrgencyStaleClaim   = "stale-claim"
)

// UrgencyTerms is every term, in the order the score lists them.
var UrgencyTerms = []string{
	UrgencyWaitsOnYou, UrgencyYourQuestion, UrgencyPriority, UrgencySeverity,
	UrgencyBlocked, UrgencyBlocksOthers, UrgencyAge, UrgencyStaleClaim,
}

// The members of the two mapping terms, your-question and age.
const (
	urgencyPerItem = "per-item"
	urgencyPerDay  = "per-day"
	urgencyCap     = "cap"
)

// The two defects that make a dinah.urgency block unusable, as machine
// tokens. The first one found is the one reported.
const (
	UrgencyNotAMapping   = "not-a-mapping"
	UrgencyMalformedTerm = "malformed-term"
)

// urgencyLimit is the largest magnitude a weight or a cap may carry, in whole
// points. A weight is held in tenths, so the limit in tenths is ten times it.
const urgencyLimit = 1000

// urgencyLiteral matches the literal form a number takes in the block: an
// optional minus sign, a whole part with no leading zero, and an optional
// fraction. It admits no exponent, so 1e1 is refused on its form even though
// its value, 10, is a whole number of tenths. The value rules are applied to
// the fraction afterwards.
var urgencyLiteral = regexp.MustCompile(`^(-?)(0|[1-9][0-9]*)(?:\.([0-9]+))?$`)

// Urgency is the weights of the urgency order, every weight held as an
// integer count of tenths and every cap as a whole count. A workbench that
// declares no block, or one whose block declares nothing, carries the
// shipped defaults.
type Urgency struct {
	WaitsOnYou   int64
	PerItem      int64 // tenths per counted item of the caller's
	ItemCap      int64 // the most items counted
	Priority     []int64
	Severity     []int64
	Blocked      int64
	BlocksOthers int64
	PerDay       int64 // tenths per whole day in the current column
	DayCap       int64 // the most days counted
	StaleClaim   int64
	// PriorityDeclared and SeverityDeclared say the block itself declared
	// the list, which is what the level-count finding reads: a list the
	// block leaves out is the shipped default and was never declared.
	PriorityDeclared bool
	SeverityDeclared bool
	// Unknown are the member paths the block carries that this build does
	// not read, in the order read, such as age.perday.
	Unknown []string
}

// UrgencyDefect is what made a dinah.urgency block unusable, or the zero value
// where it is usable. Term is the path of the member at fault, such as
// age.cap, and Read is the JSON the block reader answered for that member,
// so a trailing comment or a quoted number shows up in what the reader sees.
// Term and Read are empty on a block that is not a mapping.
type UrgencyDefect struct {
	Defect string
	Term   string
	Read   string
}

// DefaultUrgency is the weights Dinah ships, in tenths.
func DefaultUrgency() Urgency {
	return Urgency{
		WaitsOnYou:   100,
		PerItem:      40,
		ItemCap:      3,
		Priority:     []int64{0, 20, 40, 60},
		Severity:     []int64{0, 10, 20, 40},
		Blocked:      30,
		BlocksOthers: 20,
		PerDay:       5,
		DayCap:       5,
		StaleClaim:   30,
	}
}

// Urgency answers the workbench's weights and the defect that made its block
// unusable, if any. A defect never falls back to the defaults: a caller that
// ranks refuses rather than ranking on weights nobody declared.
func (b *Bench) Urgency() (Urgency, UrgencyDefect) {
	weights := b.urgency
	weights.Priority = append([]int64(nil), b.urgency.Priority...)
	weights.Severity = append([]int64(nil), b.urgency.Severity...)
	weights.Unknown = append([]string(nil), b.urgency.Unknown...)
	return weights, b.urgencyDefect
}

// ReadUrgency reads the dinah.urgency key of whatever frontmatter it is
// handed. It works on the JSON value blockValue answers and changes nothing
// about how that reader reads a line, so the spellings the block admits
// follow from it: a weight written bare, a list written inline, a mapping
// term written nested, and a comment on a line of its own.
//
// A key the frontmatter does not carry, and a key with nothing readable
// beneath it, which blockValue answers as the empty string, both declare
// nothing, and every term takes its default. Any other value that is not a
// mapping is not-a-mapping. A member the block does not know is recorded in
// Unknown and otherwise ignored.
func ReadUrgency(fm *Frontmatter) (Urgency, UrgencyDefect) {
	weights := DefaultUrgency()
	if fm == nil || !fm.Has(UrgencyKey) {
		return weights, UrgencyDefect{}
	}
	raw := blockValue(fm, UrgencyKey)
	if sameJSON(raw, mustMarshal("")) {
		return weights, UrgencyDefect{}
	}
	members, mapping := firstMembers(raw)
	if !mapping {
		return weights, UrgencyDefect{Defect: UrgencyNotAMapping}
	}
	var defect UrgencyDefect
	fail := func(term string, value json.RawMessage) {
		if defect.Defect != "" {
			return
		}
		defect = UrgencyDefect{Defect: UrgencyMalformedTerm, Term: term, Read: strings.TrimSpace(string(value))}
	}
	for _, member := range members {
		switch member.name {
		case UrgencyWaitsOnYou, UrgencyBlocked, UrgencyBlocksOthers, UrgencyStaleClaim:
			tenths, ok := urgencyWeight(member.value)
			if !ok {
				fail(member.name, member.value)
				continue
			}
			*scalarWeight(&weights, member.name) = tenths
		case UrgencyPriority, UrgencySeverity:
			list, ok := urgencyList(member.value)
			if !ok {
				fail(member.name, member.value)
				continue
			}
			if member.name == UrgencyPriority {
				weights.Priority, weights.PriorityDeclared = list, true
				continue
			}
			weights.Severity, weights.SeverityDeclared = list, true
		case UrgencyYourQuestion:
			readMappingTerm(member, urgencyPerItem, &weights.PerItem, &weights.ItemCap, &weights.Unknown, fail)
		case UrgencyAge:
			readMappingTerm(member, urgencyPerDay, &weights.PerDay, &weights.DayCap, &weights.Unknown, fail)
		default:
			weights.Unknown = append(weights.Unknown, member.name)
		}
	}
	if defect.Defect != "" {
		return DefaultUrgency(), defect
	}
	return weights, UrgencyDefect{}
}

// scalarWeight is the field one of the four single-weight terms is held in.
func scalarWeight(weights *Urgency, term string) *int64 {
	switch term {
	case UrgencyWaitsOnYou:
		return &weights.WaitsOnYou
	case UrgencyBlocked:
		return &weights.Blocked
	case UrgencyBlocksOthers:
		return &weights.BlocksOthers
	}
	return &weights.StaleClaim
}

// readMappingTerm reads one of the two mapping terms, whose members are a
// weight named rate and a whole-number cap. A member the block leaves out
// keeps its default, and a member it does not know is recorded as unknown
// under the term's path.
func readMappingTerm(member jsonMember, rate string, weight, limit *int64, unknown *[]string, fail func(string, json.RawMessage)) {
	members, mapping := firstMembers(member.value)
	if !mapping {
		fail(member.name, member.value)
		return
	}
	for _, inner := range members {
		path := member.name + "." + inner.name
		switch inner.name {
		case rate:
			tenths, ok := urgencyWeight(inner.value)
			if !ok {
				fail(path, inner.value)
				continue
			}
			*weight = tenths
		case urgencyCap:
			count, ok := urgencyWhole(inner.value)
			if !ok {
				fail(path, inner.value)
				continue
			}
			*limit = count
		default:
			*unknown = append(*unknown, path)
		}
	}
}

// urgencyWeight reads one weight: a JSON number written as a decimal literal
// with no exponent, whose value is a whole number of tenths and whose
// magnitude is at most urgencyLimit. It answers the value in tenths.
//
// The value rule is read off the literal rather than off a parsed float, so
// no floating point takes part. 0.50 is admitted, since its value is five
// tenths, and 0.25 is refused, since it is not a whole number of tenths.
func urgencyWeight(raw json.RawMessage) (int64, bool) {
	m := urgencyLiteral.FindStringSubmatch(strings.TrimSpace(string(raw)))
	if m == nil {
		return 0, false
	}
	whole, err := strconv.ParseInt(m[2], 10, 64)
	if err != nil || whole > urgencyLimit {
		return 0, false
	}
	fraction := m[3]
	tenth := int64(0)
	if fraction != "" {
		tenth = int64(fraction[0] - '0')
		if strings.Trim(fraction[1:], "0") != "" {
			return 0, false
		}
	}
	tenths := whole*10 + tenth
	if tenths > urgencyLimit*10 {
		return 0, false
	}
	if m[1] == "-" {
		tenths = -tenths
	}
	return tenths, true
}

// urgencyWhole reads a cap: a weight with no tenths, from 0 to urgencyLimit.
// It answers the whole count rather than tenths.
func urgencyWhole(raw json.RawMessage) (int64, bool) {
	tenths, ok := urgencyWeight(raw)
	if !ok || tenths < 0 || tenths%10 != 0 {
		return 0, false
	}
	return tenths / 10, true
}

// urgencyList reads a level term's list: a JSON array of at least one entry,
// every entry a weight.
func urgencyList(raw json.RawMessage) ([]int64, bool) {
	entries, ok := jsonEntries(raw)
	if !ok || len(entries) == 0 {
		return nil, false
	}
	list := make([]int64, 0, len(entries))
	for _, entry := range entries {
		tenths, ok := urgencyWeight(entry)
		if !ok {
			return nil, false
		}
		list = append(list, tenths)
	}
	return list, true
}

// FormatTenths writes an integer count of tenths with exactly one decimal
// digit and a leading minus sign when negative, so 185 is 18.5, 160 is 16.0
// and -15 is -1.5. Every figure the urgency order prints goes through it.
func FormatTenths(tenths int64) string {
	sign := ""
	if tenths < 0 {
		sign = "-"
		tenths = -tenths
	}
	return sign + strconv.FormatInt(tenths/10, 10) + "." + strconv.FormatInt(tenths%10, 10)
}
