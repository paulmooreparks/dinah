package textdiff

import "strings"

// Conflict is a region that both sides changed differently, given as the
// base's text there and each side's text, with the base line it starts at
// (counting from 1).
type Conflict struct {
	Line   int    `json:"line"`
	Base   string `json:"base"`
	Mine   string `json:"mine"`
	Theirs string `json:"theirs"`
}

// hunk is one change from the base: base lines [i1, i2) become lines.
type hunk struct {
	i1, i2 int
	lines  []string
	mine   bool
}

func hunks(base, other []string, mine bool) []hunk {
	ta, tb := Tokens(base, other)
	var out []hunk
	for _, op := range Opcodes(ta, tb) {
		if op.Tag != 'e' {
			out = append(out, hunk{op.I1, op.I2, other[op.J1:op.J2], mine})
		}
	}
	return out
}

// Merge3 merges two texts that were both changed from base, line by line:
// a change that only one side made is taken, a change both sides made the
// same way is taken once, and changes that touch the same lines, or lines
// next to each other, differently are conflicts. The merged text takes mine
// where there is a conflict, so a caller that finds conflicts can refuse the
// merge or show them; the text is complete only when there are none.
// Neighbouring lines count as touching, so two edits to one paragraph are
// never combined without the person seeing both.
func Merge3(base, mine, theirs string) (string, []Conflict) {
	b := Lines(base)
	all := append(hunks(b, Lines(mine), true), hunks(b, Lines(theirs), false)...)
	// Order by start, an insertion before a change at the same line.
	for i := 1; i < len(all); i++ {
		for j := i; j > 0 && (all[j].i1 < all[j-1].i1 || all[j].i1 == all[j-1].i1 && all[j].i2 < all[j-1].i2); j-- {
			all[j], all[j-1] = all[j-1], all[j]
		}
	}
	var out strings.Builder
	var conflicts []Conflict
	pos := 0
	for g := 0; g < len(all); {
		// A group is a run of hunks that touch: each starts no later than
		// where the group so far ends.
		lo, hi, end := all[g].i1, all[g].i2, g+1
		for end < len(all) && all[end].i1 <= hi {
			if all[end].i2 > hi {
				hi = all[end].i2
			}
			end++
		}
		group := all[g:end]
		g = end
		for _, line := range b[pos:lo] {
			out.WriteString(line)
		}
		pos = hi
		side := func(mine bool) (string, bool) {
			var s strings.Builder
			at, any := lo, false
			for _, h := range group {
				if h.mine != mine {
					continue
				}
				any = true
				for _, line := range b[at:h.i1] {
					s.WriteString(line)
				}
				for _, line := range h.lines {
					s.WriteString(line)
				}
				at = h.i2
			}
			for _, line := range b[at:hi] {
				s.WriteString(line)
			}
			return s.String(), any
		}
		m, mineChanged := side(true)
		t, theirsChanged := side(false)
		switch {
		case !theirsChanged || m == t:
			out.WriteString(m)
		case !mineChanged:
			out.WriteString(t)
		default:
			out.WriteString(m)
			conflicts = append(conflicts, Conflict{Line: lo + 1, Base: strings.Join(b[lo:hi], ""), Mine: m, Theirs: t})
		}
	}
	for _, line := range b[pos:] {
		out.WriteString(line)
	}
	return out.String(), conflicts
}
