package httphead

import (
	"strconv"
	"strings"
)

// negotiate chooses the representation a response is served in, from the
// types a route offers and the request's Accept header. Each offered type
// takes the q value of the most specific media range matching it, an exact
// type ranking above type/* and type/* above */*; q=0 excludes; the highest
// q wins and a tie goes to the type offered first. An absent Accept takes the
// first offered type. It reports false when the header admits none of them.
//
// Media range parameters other than q are not compared, so a range naming the
// vendor type with a profile matches the vendor type whatever the profile.
func negotiate(accept string, offered []string) (string, bool) {
	if strings.TrimSpace(accept) == "" {
		return offered[0], true
	}
	ranges := parseAccept(accept)
	best, bestQ := "", 0.0
	for _, candidate := range offered {
		q := qualityOf(bareType(candidate), ranges)
		if q > bestQ {
			best, bestQ = candidate, q
		}
	}
	return best, bestQ > 0
}

// mediaRange is one member of an Accept header.
type mediaRange struct {
	kind, subtype string
	q             float64
}

// parseAccept reads an Accept header's media ranges, skipping any member
// that is not type/subtype or whose q is not a number from 0 to 1.
func parseAccept(accept string) []mediaRange {
	var ranges []mediaRange
	for _, member := range strings.Split(accept, ",") {
		fields := strings.Split(member, ";")
		kind, subtype, ok := strings.Cut(strings.ToLower(strings.TrimSpace(fields[0])), "/")
		if !ok || kind == "" || subtype == "" || (kind == "*" && subtype != "*") {
			continue
		}
		q, valid := 1.0, true
		for _, parameter := range fields[1:] {
			name, value, _ := strings.Cut(strings.TrimSpace(parameter), "=")
			if strings.EqualFold(strings.TrimSpace(name), "q") {
				parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
				if err != nil || parsed < 0 || parsed > 1 {
					valid = false
				}
				q = parsed
			}
		}
		if valid {
			ranges = append(ranges, mediaRange{kind: kind, subtype: subtype, q: q})
		}
	}
	return ranges
}

// qualityOf is the q the most specific range matching a type gives it, and
// zero when no range matches.
func qualityOf(mediaType string, ranges []mediaRange) float64 {
	kind, subtype, _ := strings.Cut(mediaType, "/")
	specificity, q := -1, 0.0
	for _, r := range ranges {
		level := -1
		switch {
		case r.kind == kind && r.subtype == subtype:
			level = 2
		case r.kind == kind && r.subtype == "*":
			level = 1
		case r.kind == "*" && r.subtype == "*":
			level = 0
		}
		if level > specificity {
			specificity, q = level, r.q
		}
	}
	return q
}

// bareType is a media type without its parameters, lowercased.
func bareType(mediaType string) string {
	bare, _, _ := strings.Cut(mediaType, ";")
	return strings.ToLower(strings.TrimSpace(bare))
}
