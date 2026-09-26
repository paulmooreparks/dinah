package bench

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
)

// QuotedRawLine is one line an import below RawLineFormat wrote for a member
// the renderer could not spell: the member's JSON inside one pair of quotes,
// on the anchor named by Path under the key named by Key.
type QuotedRawLine struct {
	// Path is the anchor carrying the line.
	Path string `json:"path"`
	// Key is the frontmatter key the line stands under.
	Key string `json:"key"`
	// compact is the JSON the line spells, compacted, which is the bare text
	// the migration writes in its place. It is not reported, because the
	// value is what the anchor already carries.
	compact string
}

// RawLineMigration is the account dinah check --migrate-raw-lines answers
// with.
type RawLineMigration struct {
	// From is the format the workbench declared before the run.
	From int `json:"from"`
	// Rewritten names every line the run rewrote, or, on a preview, every
	// line it would rewrite. It is empty on a store at or past
	// RawLineFormat, where nothing is read.
	Rewritten []QuotedRawLine `json:"rewritten,omitempty"`
	// Stamped is true where the anchor was written with RawLineFormat.
	Stamped bool `json:"stamped"`
	// Preview is true where the run carried no confirmation, so it wrote
	// nothing whatever it found.
	Preview bool `json:"preview,omitempty"`
}

// QuotedRawLines names every line on the workbench anchor and on each column
// anchor that the interchange's raw-line fallback wrote quoted, in the order
// the anchors and their keys stand. A line qualifies when it is a key's only
// line, its scalar is wrapped in one pair of quotes, and the text inside the
// quotes parses as a JSON object or array.
//
// Only the keys the import writes through that fallback are read. On the
// workbench anchor that is levels, tiers, fields, field_values and every key
// the interchange does not carry under a name of its own; on a column anchor
// it is field_values, standing_items and every such key. A key the anchor
// reader answers as text, a title say, is left alone whatever it spells,
// because both builds read it as text and rewriting it would change the
// spelling of something no reader parses.
//
// A quoted number, boolean or null is not a line this names. The reader of
// the day parsed those too, so a hand-written `count: "12"` exported as the
// number under an earlier build and exports as the string under this one,
// and that retyping is the correction dinah-593 made on purpose: an author
// who quotes a scalar means text, and the fallback never wrote such a line,
// since a JSON number or boolean is a scalar the renderer spells bare.
func (b *Bench) QuotedRawLines() []QuotedRawLine {
	var lines []QuotedRawLine
	lines = append(lines, quotedRawLinesOf(filepath.Join(b.Root, WorkbenchAnchor), b.FM, rawLineBenchKey)...)
	for _, column := range b.Columns {
		if column.FM == nil {
			continue
		}
		anchor := filepath.Join(b.ColumnDir(column.ID), ColumnAnchor)
		lines = append(lines, quotedRawLinesOf(anchor, column.FM, rawLineColumnKey)...)
	}
	return lines
}

// rawLineBenchKey reports whether the import may write a workbench anchor key
// through the raw-line fallback.
func rawLineBenchKey(key string) bool {
	switch key {
	case LevelsKey, TiersKey, FieldsKey, FieldValuesKey:
		return true
	}
	return !knownBenchKeys[key]
}

// rawLineColumnKey reports whether the import may write a column anchor key
// through the raw-line fallback.
func rawLineColumnKey(key string) bool {
	switch key {
	case FieldValuesKey, StandingItemsKey:
		return true
	}
	return !knownColumnKeys[key]
}

// quotedRawLinesOf names the quoted raw lines of one anchor, reading the keys
// the given predicate admits in the order the header carries them.
func quotedRawLinesOf(anchor string, fm *Frontmatter, admits func(string) bool) []QuotedRawLine {
	var lines []QuotedRawLine
	for _, key := range fm.Keys() {
		if !admits(key) {
			continue
		}
		compact, quotedJSON := quotedRawJSON(fm.Raw(key))
		if !quotedJSON {
			continue
		}
		lines = append(lines, QuotedRawLine{Path: anchor, Key: key, compact: compact})
	}
	return lines
}

// quotedRawJSON reads a key's raw lines and reports whether they are one line
// whose quoted scalar spells a JSON object or array, answering that JSON
// compacted. The quotes are read by the rule quoted states and stripped by
// unquote, which undoes the escape the writer of the day applied, so the
// text this parses is the text that writer was handed.
func quotedRawJSON(lines []string) (string, bool) {
	if len(lines) != 1 {
		return "", false
	}
	_, text, cut := strings.Cut(lines[0], ":")
	if !cut {
		return "", false
	}
	text = strings.TrimSpace(text)
	if !quoted(text) {
		return "", false
	}
	inner := unquote(text)
	shape := jsonShape(json.RawMessage(inner))
	if shape != '{' && shape != '[' {
		return "", false
	}
	// Compact validates as it copies, so text that is not JSON is refused
	// here, and what it writes is the one-line spelling setRawJSON writes.
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(inner)); err != nil {
		return "", false
	}
	return compact.String(), true
}

// checkRawLines reports every quoted raw line on a store below RawLineFormat.
// A store at or past it is not read, because there the quoted spelling is
// text by declaration and the reader's answer is the right one.
func (b *Bench) checkRawLines() []Finding {
	if b.Format >= RawLineFormat {
		return nil
	}
	var findings []Finding
	for _, line := range b.QuotedRawLines() {
		findings = append(findings, Finding{Path: line.Path, Key: FindingRawLineQuoted, Detail: line.Key})
	}
	return findings
}

// MigrateRawLines rewrites every quoted raw line on a workbench declaring a
// format below RawLineFormat to the bare compact spelling setRawJSON writes,
// and then stamps RawLineFormat on the anchor. It does nothing to a workbench
// already at or above the format, which is the floor the stamp keeps: a
// number this build has not imagined yet is never written down to this one.
// Without apply it classifies and writes nothing, which is why it needs no
// rehearsal flag.
//
// The rewrite is exact for every line the fallback produced, because the
// bare compact spelling reads as the same JSON under the reader of the day
// and under this one, so an export before the rewrite under that build and
// an export after it under this build carry the member as one value. The
// lines are rewritten before the format is stamped, so a run that stops part
// way leaves the store below the format, where check still names what is
// left.
//
// Any lower number is stamped, a number below DesignationFormat included, on
// the terms MigrateAppliesWhen states: this stamp converts no designation and
// does not refuse for want of it, so a store below DesignationFormat runs
// `dinah check --migrate-designations` first.
func (b *Bench) MigrateRawLines(apply bool) (*RawLineMigration, error) {
	report := &RawLineMigration{From: b.Format, Preview: !apply}
	if b.Format >= RawLineFormat {
		return report, nil
	}
	report.Rewritten = b.QuotedRawLines()
	if !apply {
		return report, nil
	}
	byAnchor := map[string][]QuotedRawLine{}
	var anchors []string
	for _, line := range report.Rewritten {
		if _, seen := byAnchor[line.Path]; !seen {
			anchors = append(anchors, line.Path)
		}
		byAnchor[line.Path] = append(byAnchor[line.Path], line)
	}
	for _, anchor := range anchors {
		if err := rewriteRawLines(b.source(), anchor, byAnchor[anchor]); err != nil {
			return report, err
		}
	}
	if err := b.stampFormat(RawLineFormat); err != nil {
		return report, err
	}
	report.Stamped = true
	return report, nil
}

// rewriteRawLines rewrites the named lines of one anchor in place, reading the
// file rather than the header the bench holds so the body and every other key
// are written back as they stand.
func rewriteRawLines(src Source, anchor string, lines []QuotedRawLine) error {
	text, err := readText(src, anchor)
	if err != nil {
		return err
	}
	fm, body := ParseAnchor(text)
	for _, line := range lines {
		setRawJSON(fm, line.Key, json.RawMessage(line.compact))
	}
	return WriteText(anchor, fm.Render(body))
}
