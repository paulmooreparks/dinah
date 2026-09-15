package lsp

import "strings"

// The prose grammar of section 4.3, written once.
//
// A candidate starts at the start of a line or after a character outside the
// boundary set, and the character after it must be outside that set or be the
// end of the line. The boundary rule and a head's own rule are conjunctive,
// and the boundary rule is checked first, which is what refuses
// zz0123456789abzz, pre-dinah-515 and the last group of a UUID, none of which
// a head's own rule catches on its own.
//
// The trim set is derived rather than listed: it is exactly the characters a
// tail segment admits that are not alphanumeric. Listing it by hand is how an
// earlier draft came to carry a slash it can never see, and deriving it is
// what keeps the set correct if the segment class ever widens.

// inBoundarySet reports whether a character is one the boundary rule counts
// as part of a word, so a candidate may neither begin after one nor end
// before one.
func inBoundarySet(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '-'
}

// inSegment reports whether a character is one a tail segment admits.
func inSegment(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_' || c == '.' || c == '-'
}

// trimmable reports whether a character is one a candidate's tail is trimmed
// of, which is exactly a segment character that is not alphanumeric: the dot,
// the dash and the underscore. The derivation is the rule, so the set here is
// computed from inSegment rather than written out beside it.
func trimmable(c byte) bool {
	alphanumeric := c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9'
	return inSegment(c) && !alphanumeric
}

// isHex reports whether a character is a hexadecimal digit of either case,
// which is the class a maximal run is measured over.
func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// isLowerHex reports whether a character is one an identifier is spelled
// with, which is the narrower class the twelve characters themselves have to
// fall in.
func isLowerHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f'
}

// identifierLength is how many characters an entity identifier is spelled
// with. It is bench.IsID's own length, read here as a number rather than by
// calling that predicate, because the scan has to know the length before it
// has a string to hand it.
const identifierLength = 12

// scanProse finds every candidate in the body of a document, tracking fenced
// code blocks line by line.
//
// A reference inside an inline code span is recognised, because that is how
// card bodies and specifications write one. A reference inside a fenced code
// block is not: those are transcripts and examples, and a placeholder in a
// guide names nothing. Indented code blocks are not excluded, knowingly:
// telling one from a list continuation needs a full block parse this server
// does not carry, and guessing from the indent would silence a reference
// written inside an ordinary nested list.
func scanProse(slug string, lines []string, from int) []slot {
	var slots []slot
	fence := ""
	for i := from; i < len(lines); i++ {
		line := lines[i]
		if opener := fenceAt(line); opener != "" {
			if fence == "" {
				fence = opener
			} else if opener[0] == fence[0] && len(opener) >= len(fence) {
				fence = ""
			}
			continue
		}
		if fence != "" {
			continue
		}
		slots = append(slots, scanLine(slug, i, line)...)
	}
	return slots
}

// fenceAt answers the run of backticks or tildes a line opens or closes a
// fenced code block with, and the empty string for a line that is neither.
func fenceAt(line string) string {
	rest := strings.TrimLeft(line, " \t")
	for _, mark := range []byte{'`', '~'} {
		run := 0
		for run < len(rest) && rest[run] == mark {
			run++
		}
		if run >= 3 {
			return rest[:run]
		}
	}
	return ""
}

// scanLine finds the candidates on one line of prose.
func scanLine(slug string, line int, text string) []slot {
	var slots []slot
	for i := 0; i < len(text); i++ {
		if i > 0 && inBoundarySet(text[i-1]) {
			continue
		}
		end, ok := candidateAt(slug, text, i)
		if !ok {
			continue
		}
		if end < len(text) && inBoundarySet(text[end]) {
			continue
		}
		trimmed := end
		for trimmed > i && trimmable(text[trimmed-1]) {
			trimmed--
		}
		if trimmed == i {
			continue
		}
		slots = append(slots, slot{
			Kind:  slotProse,
			Range: Range{Start: positionAt(text, line, i), End: positionAt(text, line, trimmed)},
			Text:  text[i:trimmed],
		})
		i = end - 1
	}
	return slots
}

// candidateAt consumes one candidate beginning at from, answering where it
// ends. The boundary rule before the candidate has already been checked; the
// rule after it is the caller's, because a candidate that fails it is refused
// outright rather than shortened until it passes.
func candidateAt(slug, text string, from int) (int, bool) {
	if end, ok := workstreamHead(text, from); ok {
		return end, true
	}
	end, ok := cardHead(slug, text, from)
	if !ok {
		end, ok = identifierHead(text, from)
	}
	if !ok {
		return 0, false
	}
	return tailEnd(text, end), true
}

// workstreamHead consumes the literal prefix and the handle after it, which
// is either a slug or an identifier. The head carries no tail, a workstream
// holding no collection a reference reaches by path.
func workstreamHead(text string, from int) (int, bool) {
	const prefix = "workstream/"
	if !strings.HasPrefix(text[from:], prefix) {
		return 0, false
	}
	at := from + len(prefix)
	if end, ok := slugAt(text, at); ok {
		return end, true
	}
	return identifierHead(text, at)
}

// slugAt consumes a handle spelled the way bench.ColumnSlugPattern spells
// one, which is the grammar workstreams share: an ASCII letter, then lower
// alphanumerics, with single dashes between groups of them.
func slugAt(text string, from int) (int, bool) {
	if from >= len(text) || text[from] < 'a' || text[from] > 'z' {
		return 0, false
	}
	at := from + 1
	for at < len(text) && (text[at] >= 'a' && text[at] <= 'z' || text[at] >= '0' && text[at] <= '9') {
		at++
	}
	for at < len(text) && text[at] == '-' {
		group := at + 1
		for group < len(text) && (text[group] >= 'a' && text[group] <= 'z' || text[group] >= '0' && text[group] <= '9') {
			group++
		}
		if group == at+1 {
			break
		}
		at = group
	}
	return at, true
}

// cardHead consumes the workbench's own slug, matched literally and
// case-sensitively, then a dash and a number.
//
// The exact slug is the guard that matters most, and it deliberately departs
// from bench.ResolveCard, which splits at the last dash and keys on the
// number alone. That tolerance is right for a reference somebody typed after
// a rename and a disaster in a scan: utf-8 resolves to card 8 and sha-256 to
// card 256, and the population of English tokens it would annotate grows as
// the workbench does rather than being a fixed set to guard against.
func cardHead(slug, text string, from int) (int, bool) {
	if slug == "" || !strings.HasPrefix(text[from:], slug+"-") {
		return 0, false
	}
	at := from + len(slug) + 1
	if at >= len(text) || text[at] < '1' || text[at] > '9' {
		return 0, false
	}
	at++
	for at < len(text) && text[at] >= '0' && text[at] <= '9' {
		at++
	}
	return at, true
}

// identifierHead consumes a bare identifier: twelve characters of lowercase
// hexadecimal forming a maximal run of hexadecimal of either case.
//
// Maximality is what excludes a longer run, so a forty-character git object
// name yields nothing and a thirty-two-character workbench identifier yields
// nothing, and a substring of either is never taken. It is not the whole
// guard on its own: zz0123456789abzz carries a maximal run of twelve, and the
// boundary rule is what refuses it.
func identifierHead(text string, from int) (int, bool) {
	at := from
	for at < len(text) && isHex(text[at]) {
		at++
	}
	if at-from != identifierLength {
		return 0, false
	}
	for i := from; i < at; i++ {
		if !isLowerHex(text[i]) {
			return 0, false
		}
	}
	return at, true
}

// tailEnd consumes the optional path after a head, greedily.
func tailEnd(text string, from int) int {
	at := from
	for at < len(text) && text[at] == '/' {
		segment := at + 1
		for segment < len(text) && inSegment(text[segment]) {
			segment++
		}
		if segment == at+1 {
			break
		}
		at = segment
	}
	return at
}
