// Package profile extracts the normative statements from a core profile
// document and reports the mechanical defects the document's own notation
// section forbids.
//
// The extraction is line-oriented on purpose. A statement occupies one line
// beginning with its identifier in square brackets at column one, so nothing
// here parses Markdown and a document that grows new prose sections cannot
// break the extractor.
package profile

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Statement is one identified normative statement of a profile document.
type Statement struct {
	// ID is the statement's identifier, such as CORE-CLAIM-3.
	ID string
	// Keyword is the single RFC 2119 keyword the statement carries.
	Keyword string
	// Text is the statement itself, without the identifier.
	Text string
	// Line is the one-based line number the statement was read from.
	Line int
	// Class is the identifier's leading family: CORE, DOC or ACTOR.
	Class string
}

// statementLine matches an identified statement line. Capture group one is
// the identifier and capture group three is the statement text.
var statementLine = regexp.MustCompile(`^\[([A-Z][A-Z0-9]*(-[A-Z0-9]+)*)\] (.+)$`)

// keyword matches an RFC 2119 keyword. The alternatives are ordered longest
// first so that a negated keyword counts once rather than twice, which is the
// rule the profile's notation section states.
var keyword = regexp.MustCompile(`\b(MUST NOT|SHOULD NOT|MUST|SHOULD|MAY)\b`)

// fence recognizes the start and end of a fenced block. Keywords inside one
// are vocabulary listings rather than requirements.
var fence = regexp.MustCompile("^```")

// Document is the result of reading a profile document.
type Document struct {
	// Statements are the identified statements, in document order.
	Statements []Statement
	// StrayKeywords are the lines carrying a keyword outside both a
	// statement and a fenced block, which the notation section forbids.
	StrayKeywords []Defect
	// Duplicates are identifiers published more than once.
	Duplicates []Defect
	// KeywordCounts are statements not carrying exactly one keyword.
	KeywordCounts []Defect
	// Fences are fenced blocks left open at the end of the document. An
	// unterminated fence disarms keyword detection for everything after
	// it, so the document reports it rather than reading clean.
	Fences []Defect
}

// Defect is one mechanical fault, reported with enough context to fix it.
type Defect struct {
	Line int
	Text string
	Why  string
}

func (d Defect) String() string {
	return fmt.Sprintf("line %d: %s: %s", d.Line, d.Why, d.Text)
}

// Extract reads a profile document and returns its statements together with
// the mechanical defects the notation section forbids. A read error is
// returned; a defect in the document is not, because the caller decides how
// loudly to complain about one.
func Extract(r io.Reader) (*Document, error) {
	doc := &Document{}
	seen := map[string]int{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	inFence := false
	fenceOpenedAt := 0
	fenceOpenedBy := ""
	line := 1
	for ; sc.Scan(); line++ {
		text := strings.TrimRight(sc.Text(), "\r")

		if fence.MatchString(text) {
			inFence = !inFence
			if inFence {
				fenceOpenedAt, fenceOpenedBy = line, text
			}
			continue
		}

		m := statementLine.FindStringSubmatch(text)
		if m == nil {
			if !inFence && keyword.MatchString(text) {
				doc.StrayKeywords = append(doc.StrayKeywords, Defect{
					Line: line,
					Text: text,
					Why:  "keyword outside an identified statement",
				})
			}
			continue
		}

		id, body := m[1], m[3]
		found := keyword.FindAllString(body, -1)
		st := Statement{ID: id, Text: body, Line: line, Class: class(id)}
		if len(found) == 1 {
			st.Keyword = found[0]
		} else {
			doc.KeywordCounts = append(doc.KeywordCounts, Defect{
				Line: line,
				Text: text,
				Why:  fmt.Sprintf("statement carries %d keywords, wanted 1", len(found)),
			})
		}
		if prior, ok := seen[id]; ok {
			doc.Duplicates = append(doc.Duplicates, Defect{
				Line: line,
				Text: id,
				Why:  fmt.Sprintf("identifier already published at line %d", prior),
			})
		}
		seen[id] = line
		doc.Statements = append(doc.Statements, st)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if inFence {
		doc.Fences = append(doc.Fences, Defect{
			Line: fenceOpenedAt,
			Text: fenceOpenedBy,
			Why:  fmt.Sprintf("fence opened at line %d is never closed, so keyword detection is off from there to the end of the document", fenceOpenedAt),
		})
	}
	return doc, nil
}

// class returns the leading family of an identifier, which the profile uses
// to say what a statement is asserted against.
func class(id string) string {
	if i := strings.Index(id, "-"); i > 0 {
		return id[:i]
	}
	return id
}

// whitespace matches any run of spacing, including the line breaks a wrapped
// document puts inside a two-word phrase.
var whitespace = regexp.MustCompile(`\s+`)

// Excluded reports every whole-word occurrence of a term in text, matched
// without regard to case. The profile uses it to keep one trade's vocabulary,
// and the product vocabulary of other tools, out of the places it names.
//
// Every run of whitespace in the text and in the term collapses to one space
// before matching, so a banned phrase whose two words land either side of a
// line wrap is still found. Matching the raw text would have let the width of
// the paragraph decide whether the check ran.
func Excluded(text string, terms []string) []string {
	flat := whitespace.ReplaceAllString(text, " ")
	var hits []string
	for _, term := range terms {
		want := whitespace.ReplaceAllString(strings.TrimSpace(term), " ")
		re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(want) + `\b`)
		if re.MatchString(flat) {
			hits = append(hits, term)
		}
	}
	return hits
}

// BoundaryRow is one row of the boundary table of section 10.
type BoundaryRow struct {
	// Item is the concept the row rules on.
	Item string
	// Ruling is `in` or `out`.
	Ruling string
	// Reason says what the core gains or loses by the ruling.
	Reason string
	// Reopen is the condition that would bring an excluded concept back,
	// and is empty on a row ruled in.
	Reopen string
	// Statements are the identifiers carrying an included concept, and are
	// empty on a row ruled out.
	Statements []string
	// Line is the one-based line the row was read from.
	Line int
}

// boundaryRow matches a five-celled table row whose second cell rules the
// concept in or out. The index table of section 11 carries four cells, so it
// cannot be read as a boundary row by accident.
var boundaryRow = regexp.MustCompile(`^\| ([^|]+) \| (in|out) \| ([^|]*) \|([^|]*)\|([^|]*)\|$`)

// BoundaryTable reads the boundary table out of a profile document. It works
// over the whole document rather than a located section, so a caller needs no
// Markdown parser here either.
func BoundaryTable(text string) []BoundaryRow {
	var rows []BoundaryRow
	for i, line := range strings.Split(text, "\n") {
		m := boundaryRow.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if m == nil {
			continue
		}
		row := BoundaryRow{
			Item:   strings.TrimSpace(m[1]),
			Ruling: m[2],
			Reason: strings.TrimSpace(m[3]),
			Reopen: strings.TrimSpace(m[4]),
			Line:   i + 1,
		}
		for _, id := range strings.Split(m[5], ",") {
			if id = strings.TrimSpace(id); id != "" {
				row.Statements = append(row.Statements, id)
			}
		}
		rows = append(rows, row)
	}
	return rows
}
