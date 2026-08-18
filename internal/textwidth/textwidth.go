// Package textwidth measures how many terminal columns a string draws in.
//
// One measure serves the whole product. The CLI head pads its columns with it,
// and the guard that reads the message catalogs for a hand-laid-out row asks
// it how wide a run of spaces is. Two measures that can disagree are worse
// than one, because the guard then passes on output the binary misaligns.
//
// Every rule here comes from published Unicode data and none of it comes from
// asking a terminal anything at runtime. A whole emoji sequence is taken as
// one unit and given the width of the one glyph it draws as, a regional
// indicator pair is a flag and draws as one, and everything else is measured
// per rune from General_Category and from the East Asian Width property of
// UAX #11.
package textwidth

import (
	"unicode"

	"golang.org/x/text/width"
)

// The three runes the emoji rules name directly.
const (
	// zeroWidthJoiner joins two emoji elements into one sequence, per ED-17
	// of UTS #51.
	zeroWidthJoiner = '‍'
	// emojiSelector is VARIATION SELECTOR-16, the request for the emoji
	// glyph, which ED-15a makes part of an emoji presentation sequence.
	//
	// Its sibling U+FE0E, VARIATION SELECTOR-15, is deliberately absent from
	// these rules. That one is the request for the narrow text glyph, so a
	// base carrying it is a text presentation sequence per ED-15b and is not
	// an emoji element at all. Giving the two selectors a matched branch
	// inverts the meaning of one of them and measures every text
	// presentation sequence a column too wide, which is the direction that
	// glues the next field on. Letting U+FE0E fall through to the per-rune
	// path is what gives the right answer, since it is a format character
	// and counts nothing while its base carries the pair.
	emojiSelector = '️'
	// keycap is COMBINING ENCLOSING KEYCAP, which closes a keycap sequence.
	keycap = '⃣'
)

// Columns reports how many terminal columns text draws in. It takes one unit
// at a time, where a unit is a whole emoji sequence as UTS #51 defines one, a
// regional indicator pair, or a single rune.
func Columns(text string) int {
	runes := []rune(text)
	total := 0
	for i := 0; i < len(runes); {
		if i+1 < len(runes) && isRegionalIndicator(runes[i]) && isRegionalIndicator(runes[i+1]) {
			total += 2
			i += 2
			continue
		}
		if unit := emojiUnit(runes, i); unit > i {
			total += 2
			i = unit
			continue
		}
		if drawsNothing(runes[i]) {
			i++
			continue
		}
		total += runeColumns(runes[i])
		i++
	}
	return total
}

// runeColumns reports the columns one rune draws in, outside any emoji
// sequence. An East Asian Wide or Fullwidth rune draws in two and everything
// else draws in one, the Ambiguous class included: UAX #11 resolves Ambiguous
// by the typographic context the text sits in, no documented API reports which
// context a terminal is drawing in, and one column is the value that is right
// outside East Asian legacy contexts.
func runeColumns(r rune) int {
	switch width.LookupRune(r).Kind() {
	case width.EastAsianWide, width.EastAsianFullwidth:
		return 2
	default:
		return 1
	}
}

// drawsNothing reports whether a rune occupies no column of its own: a
// nonspacing mark, an enclosing mark, a format character, or a control
// character. A spacing combining mark is absent from that list on purpose,
// since it does take a column, and half of the Devanagari vowel signs are
// spacing marks.
func drawsNothing(r rune) bool {
	return unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf, unicode.Cc)
}

// isRegionalIndicator reports whether a rune is one of the twenty-six letters
// a flag is spelled with.
func isRegionalIndicator(r rune) bool {
	return r >= 0x1F1E6 && r <= 0x1F1FF
}

// emojiUnit reports the index after the emoji sequence starting at i, which is
// ED-17 of UTS #51: one emoji element, then any number of elements joined to
// it with a zero width joiner. It returns i when no element starts there.
func emojiUnit(runes []rune, i int) int {
	end := emojiElement(runes, i)
	if end == i {
		return i
	}
	for end < len(runes) && runes[end] == zeroWidthJoiner {
		joined := emojiElement(runes, end+1)
		if joined == end+1 {
			return end
		}
		end = joined
	}
	return end
}

// emojiElement reports the index after the emoji element starting at i, which
// is ED-14 through ED-16 of UTS #51: an emoji character, an emoji presentation
// sequence, or an emoji modifier sequence. It returns i when the runes carry
// no element there.
func emojiElement(runes []rune, i int) int {
	if i >= len(runes) || !unicode.Is(emoji, runes[i]) {
		return i
	}
	if i+1 < len(runes) {
		next := runes[i+1]
		if unicode.Is(emojiModifier, next) && unicode.Is(emojiModifierBase, runes[i]) {
			return i + 2
		}
		if next == emojiSelector {
			if i+2 < len(runes) && runes[i+2] == keycap {
				return i + 3
			}
			return i + 2
		}
		if next == keycap {
			return i + 2
		}
	}
	if unicode.Is(emojiPresentation, runes[i]) {
		return i + 1
	}
	return i
}
