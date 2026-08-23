package main

import (
	"regexp"
	"strings"
)

// guideHeading matches an ATX heading: one to six number signs, one space,
// then something other than a space. The guides carry no setext headings and
// no heading that spans two source lines.
var guideHeading = regexp.MustCompile(`^#{1,6} \S`)

// guideBullet matches a bullet list item at column zero. A numbered list is
// deliberately not matched: none exists in the corpus, and one added later
// wraps as an ordinary paragraph rather than misbehaving.
var guideBullet = regexp.MustCompile(`^(-|\*) \S`)

// guideQuote matches a block-quote line, including the bare marker that ends
// a quoted paragraph without ending the quote.
var guideQuote = regexp.MustCompile(`^>( |$)`)

// guideIndent is how many leading spaces make a line an indented code block.
// Every list continuation in the corpus is indented by two, well clear of it.
const guideIndent = 4

// guideQuoteMarker leads every line of a wrapped block quote.
const guideQuoteMarker = "> "

// wrapGuideText lays a guide's Markdown body out for a window of width
// columns, breaking headings, list items, block quotes and paragraphs between
// words through breakWords, the renderer's one word-breaker.
//
// A width of zero or less means no documented source told the head how wide
// the window is, which is the piped case, and the text is then returned
// unchanged. That keeps a redirected `dinah guide <topic>` byte-identical to
// the guide's own source, which is what the guide guards already read.
//
// Four things are never re-flowed: a fenced code block, an indented code
// block, a table row, and a single word wider than the room it has. The first
// three carry alignment that re-flowing would destroy, and the fourth has
// nowhere to wrap into. Whichever of them a reader meets wider than the
// window, the terminal's own character-cell wrap governs it, exactly as it
// governs the whole unwrapped guide today.
func wrapGuideText(text string, width int) string {
	if width <= 0 {
		return text
	}
	lines := strings.Split(text, "\n")
	var out []string
	for at := 0; at < len(lines); {
		line := lines[at]
		switch {
		case guideOpensFence(line):
			at = copyFence(lines, at, &out)
		case guideIsIndentedCode(line):
			at = copyWhile(lines, at, guideIsIndentedCode, &out)
		case guideIsTableRow(line):
			at = copyWhile(lines, at, guideIsTableRow, &out)
		case guideHeading.MatchString(line):
			marker := line[:strings.Index(line, " ")+1]
			out = append(out, wrappedLines(line, len(marker), width)...)
			at++
		case guideQuote.MatchString(line):
			at = wrapQuote(lines, at, width, &out)
		case guideBullet.MatchString(line):
			at = wrapBullet(lines, at, width, &out)
		case strings.TrimSpace(line) == "":
			out = append(out, "")
			at++
		default:
			at = wrapParagraph(lines, at, width, &out)
		}
	}
	return strings.Join(out, "\n")
}

// guideOpensFence reports whether a line is a code fence, opening or closing.
// The rule is the simplified one two of this package's guards already apply:
// three or more backticks once the line is trimmed, with no matching of the
// closing run's own length. Every fence in the corpus opens and closes with a
// bare three-backtick marker, and this card changes none of them.
func guideOpensFence(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "```")
}

// guideIsIndentedCode reports whether a line carries the leading spaces that
// make it a code line. A blank line carries none and so ends the run.
func guideIsIndentedCode(line string) bool {
	return strings.HasPrefix(line, strings.Repeat(" ", guideIndent))
}

// guideIsTableRow reports whether a line is a row of a pipe table, which the
// heading, the separator and every data row all are.
func guideIsTableRow(line string) bool {
	return strings.HasPrefix(strings.TrimLeft(line, " "), "|")
}

// copyFence copies a fenced block verbatim, both markers included, and
// returns the index of the line after it. A fence the text never closes
// carries to the end, which leaves an unterminated block unwrapped rather
// than re-flowing the rest of the guide as prose.
func copyFence(lines []string, at int, out *[]string) int {
	*out = append(*out, lines[at])
	for at++; at < len(lines); at++ {
		*out = append(*out, lines[at])
		if guideOpensFence(lines[at]) {
			return at + 1
		}
	}
	return at
}

// copyWhile copies the run of lines from at that satisfies belongs, verbatim,
// and returns the index of the first line that does not.
func copyWhile(lines []string, at int, belongs func(string) bool, out *[]string) int {
	for at < len(lines) && belongs(lines[at]) {
		*out = append(*out, lines[at])
		at++
	}
	return at
}

// wrappedLines wraps text through the renderer's word-breaker and returns the
// lines it produced, so a caller appends them to the output rather than
// carrying an embedded newline into it.
//
// The room handed to breakWords is the window less the indent, rather than
// the window itself. breakWords counts a continuation line from the first
// word it writes and not from the indent it wrote before it, so a room of the
// whole window lets every line after the first draw indent columns past the
// window's edge. breakTail and this file's own block quote deduct too, but
// for a different reason rather than for this one: their prefix occupies
// every line including the first, so their deduction is exact and costs
// nothing. A hanging indent occupies every line except the first, so one room
// cannot be right for both, and the narrower room is the only one that keeps
// every line inside the window. The cost is that the first line stops indent
// columns short of the window; the alternative is a guide whose every wrapped
// list item reaches past the edge it was wrapped for.
func wrappedLines(text string, indent, width int) []string {
	return strings.Split(breakWords(text, indent, width-indent), "\n")
}

// wrapQuote wraps the block quote that opens at at and returns the index of
// the first line beyond it. Each quoted paragraph is joined and wrapped on
// its own, and a bare quote marker separates two of them the way a blank line
// separates two ordinary paragraphs.
func wrapQuote(lines []string, at int, width int, out *[]string) int {
	var gathered []string
	flush := func() {
		if len(gathered) == 0 {
			return
		}
		joined := strings.Join(gathered, " ")
		for _, line := range wrappedLines(joined, 0, width-len(guideQuoteMarker)) {
			*out = append(*out, guideQuoteMarker+line)
		}
		gathered = nil
	}
	for at < len(lines) && guideQuote.MatchString(lines[at]) {
		body := strings.TrimPrefix(lines[at], ">")
		body = strings.TrimPrefix(body, " ")
		if strings.TrimSpace(body) == "" {
			flush()
			*out = append(*out, ">")
			at++
			continue
		}
		gathered = append(gathered, body)
		at++
	}
	flush()
	return at
}

// wrapBullet wraps the list item that opens at at and returns the index of
// the first line beyond it. The item's own source line breaks carry no
// meaning, so the marker line and every continuation are joined before the
// wrap: a source that hard-wrapped its item with a two-space continuation and
// a source that wrote the same item as one long line come out identical.
func wrapBullet(lines []string, at int, width int, out *[]string) int {
	marker := lines[at][:2]
	gathered := []string{lines[at]}
	for at++; at < len(lines); at++ {
		if !guideContinues(lines[at], len(marker)) {
			break
		}
		gathered = append(gathered, strings.TrimSpace(lines[at]))
	}
	joined := strings.Join(gathered, " ")
	*out = append(*out, wrappedLines(joined, len(marker), width)...)
	return at
}

// guideContinues reports whether a line is the continuation of a list item
// whose marker is indent columns wide: non-blank, indented by exactly that
// much, and not itself the start of a new item at column zero.
func guideContinues(line string, indent int) bool {
	if strings.TrimSpace(line) == "" {
		return false
	}
	if guideBullet.MatchString(line) {
		return false
	}
	return len(line)-len(strings.TrimLeft(line, " ")) == indent
}

// wrapParagraph wraps the paragraph that opens at at and returns the index of
// the first line beyond it. A paragraph runs until a blank line or the start
// of any other block kind, and its own source line breaks are discarded on
// the join, exactly as Markdown itself discards them.
func wrapParagraph(lines []string, at int, width int, out *[]string) int {
	var gathered []string
	for at < len(lines) && !guideBreaksParagraph(lines[at]) {
		gathered = append(gathered, strings.TrimSpace(lines[at]))
		at++
	}
	joined := strings.Join(gathered, " ")
	*out = append(*out, wrappedLines(joined, 0, width)...)
	return at
}

// guideBreaksParagraph reports whether a line ends the paragraph being
// gathered, which every blank line and the opening line of every other block
// kind does.
func guideBreaksParagraph(line string) bool {
	if strings.TrimSpace(line) == "" {
		return true
	}
	if guideOpensFence(line) || guideIsIndentedCode(line) || guideIsTableRow(line) {
		return true
	}
	return guideHeading.MatchString(line) || guideQuote.MatchString(line) || guideBullet.MatchString(line)
}
