package main

import (
	"regexp"
	"strings"
)

// bodyMarker matches what leads a line of a card's or a comment's Markdown
// before its words: any indent, then any run of block-quote markers, then a
// bullet or a numbered list marker where there is one. A wrapped line's
// continuations hang under the first word after it.
var bodyMarker = regexp.MustCompile(`^ *(?:> ?)*(?:(?:[-*+]|\d{1,9}[.)]) +)?`)

// wrapBodyText lays a card's or a comment's Markdown body out for a window of
// width columns, breaking each source line between words through breakWords,
// the renderer's one word-breaker.
//
// It differs from wrapGuideText on purpose. A guide is text this repository
// writes and guards, so its paragraphs can be re-flowed. A body is whatever
// its author typed, with numbered lists, nested bullets and line breaks the
// guide corpus never carries, so no two source lines are ever joined here:
// each line wraps on its own, and its continuations hang under its first word
// so a wrapped list item still reads as one.
//
// A width of zero or less is the piped case, and the text is then returned
// unchanged, so a redirected `dinah show` carries the body byte for byte.
// A fenced code block, a table row and an indented code line are copied
// verbatim, since re-flowing them would destroy the alignment they carry.
func wrapBodyText(text string, width int) string {
	if width <= 0 {
		return text
	}
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for at := 0; at < len(lines); {
		line := lines[at]
		switch {
		case guideOpensFence(line):
			at = copyFence(lines, at, &out)
		case guideIsTableRow(line):
			at = copyWhile(lines, at, guideIsTableRow, &out)
		default:
			out = append(out, wrapBodyLine(line, width)...)
			at++
		}
	}
	return strings.Join(out, "\n")
}

// wrapBodyLine wraps one source line of a body. Its leading marker is kept
// as written, the words after it are broken to the room the marker leaves,
// and every continuation is indented to the marker's width. A line that fits
// and a line whose leading spaces make it indented code come back unchanged.
//
// The marker's width is its byte length, which is exact because bodyMarker
// matches nothing but ASCII; the words themselves are measured by breakWords,
// the renderer's one measure.
func wrapBodyLine(line string, width int) []string {
	marker := bodyMarker.FindString(line)
	rest := line[len(marker):]
	if strings.TrimSpace(marker) == "" && guideIsIndentedCode(line) {
		return []string{line}
	}
	hang := len(marker)
	if hang >= width/2 {
		// A marker taking half the window leaves too little room to be worth
		// hanging under, so the continuations start at the edge instead.
		marker, rest, hang = "", strings.TrimSpace(line), 0
	}
	wrapped := strings.Split(marker+breakWords(rest, hang, width-hang), "\n")
	if len(wrapped) == 1 {
		// A line that fits is kept as its author spaced it, rather than
		// with its runs of spaces closed up the way breakWords joins words.
		return []string{line}
	}
	return wrapped
}
