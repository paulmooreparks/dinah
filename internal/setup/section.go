package setup

import (
	"bytes"
	"fmt"
	"strings"
)

// markers returns the begin and end lines of a marked section, without line
// endings.
func markers(comment, key string) (begin, end string) {
	if comment == "hash" {
		return "# dinah-setup:begin " + key, "# dinah-setup:end " + key
	}
	return "<!-- dinah-setup:begin " + key + " -->", "<!-- dinah-setup:end " + key + " -->"
}

// textLine is one line of a text file, located by the offsets of its first
// byte and of the byte after its line break.
type textLine struct {
	start, end int
	// content is the line with its line ending removed.
	content string
}

// splitLines reads a file into lines, each keeping the offsets of its bytes.
// A last line with no line break still counts.
func splitLines(data []byte) []textLine {
	var lines []textLine
	for at := 0; at < len(data); {
		next := bytes.IndexByte(data[at:], '\n')
		end := len(data)
		if next >= 0 {
			end = at + next + 1
		}
		content := strings.TrimSuffix(strings.TrimSuffix(string(data[at:end]), "\n"), "\r")
		lines = append(lines, textLine{start: at, end: end, content: content})
		at = end
	}
	return lines
}

// isMarker reports whether a line is a marker line, which it is when it equals
// the marker once trailing spaces, tabs and a trailing CR are removed.
func isMarker(line, marker string) bool {
	return strings.TrimRight(line, " \t\r") == marker
}

// section is where a marked section stands in a file.
type section struct {
	// found says the file carries the section's markers.
	found bool
	// begin and end index the marker lines.
	begin, end int
	// body is the text strictly between the markers, LF-terminated.
	body string
}

// locateSection finds a section's markers and refuses markers that are
// unbalanced, repeated or out of order.
func locateSection(lines []textLine, comment, key string) (section, error) {
	beginMarker, endMarker := markers(comment, key)
	var begins, ends []int
	for i, line := range lines {
		if isMarker(line.content, beginMarker) {
			begins = append(begins, i)
		}
		if isMarker(line.content, endMarker) {
			ends = append(ends, i)
		}
	}
	if len(begins) == 0 && len(ends) == 0 {
		return section{}, nil
	}
	if len(begins) != 1 || len(ends) != 1 {
		return section{}, fmt.Errorf("carries the markers of %s unbalanced or repeated", key)
	}
	if begins[0] > ends[0] {
		return section{}, fmt.Errorf("carries the markers of %s in the wrong order", key)
	}
	s := section{found: true, begin: begins[0], end: ends[0]}
	var body strings.Builder
	for _, line := range lines[s.begin+1 : s.end] {
		body.WriteString(line.content)
		body.WriteByte('\n')
	}
	s.body = body.String()
	return s, nil
}

// sectionBody is a rendered template with a final line break ensured and its
// line endings written LF, which is the form a digest and a comparison read.
func sectionBody(rendered string) string {
	body := strings.ReplaceAll(rendered, "\r\n", "\n")
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return body
}

// sectionText is a whole section, markers included, in a file's own
// line-ending convention.
func sectionText(comment, key, body, lineBreak string) string {
	begin, end := markers(comment, key)
	text := begin + "\n" + body + end + "\n"
	return withLineBreak(text, lineBreak)
}

// appendSection adds a section at the end of a file. A file with no final line
// break gains one, and a file that is not empty gains one empty line before
// the section.
func appendSection(data []byte, comment, key, body string) []byte {
	lineBreak := lineBreakOf(data)
	out := append([]byte(nil), data...)
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, lineBreak...)
	}
	if len(out) > 0 {
		out = append(out, lineBreak...)
	}
	return append(out, sectionText(comment, key, body, lineBreak)...)
}

// replaceSectionBody replaces the lines strictly between a section's markers.
func replaceSectionBody(data []byte, lines []textLine, s section, body string) []byte {
	start := lines[s.begin].end
	end := lines[s.end].start
	return splice(data, start, end, withLineBreak(body, lineBreakOf(data)))
}

// removeSection removes a section's lines, markers included, with the one
// empty line in front of it that setup added, when that line is still there.
func removeSection(data []byte, lines []textLine, s section) []byte {
	start := lines[s.begin].start
	if s.begin > 0 && lines[s.begin-1].content == "" {
		start = lines[s.begin-1].start
	}
	return splice(data, start, lines[s.end].end, "")
}
