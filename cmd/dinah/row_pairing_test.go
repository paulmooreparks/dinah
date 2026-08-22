package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/guide"
	"dinah/internal/msg"
	"dinah/internal/verb"
)

// sweptRecord is what the fixture put in. Together with the catalog it is
// everything an expectation is built from. No field of it is a filesystem
// path, and the two columns that draw one carry a matcher instead, so an
// expect function has nothing to open and cannot consult the store to settle a
// mismatch.
type sweptRecord struct {
	// actor is the owner every act the fixture performed carries unless the
	// act names another.
	actor string
	// cards are the cards the fixture filed, in creation order, each with the
	// title it was filed under and the standing it ended in.
	cards []sweptCardRecord
	// claims, blocks, moves, and releases are the acts the fixture performed,
	// each carrying the reference, the actor, and the reason it passed.
	claims   []sweptActRecord
	blocks   []sweptActRecord
	moves    []sweptActRecord
	releases []sweptActRecord
	// joins are the memberships the fixture wrote, which land on the card's own
	// journal and so draw a row of its history alongside the acts above.
	joins []sweptActRecord
	// comments and links are what the fixture wrote onto its first card.
	comments    []sweptCommentRecord
	links       []sweptLinkRecord
	attachments []sweptAttachmentRecord
	// renames are the attachment renames the fixture ran, each carrying the
	// reference of the attachment that was renamed and the new filename it was
	// carried under.
	renames []sweptActRecord
	// states are the states the healthy tree holds, in the order the workbench
	// anchor lists them, which is the order every listing walks.
	states []sweptStateRecord
	// stripped are the states the tree a slug migration repairs holds, which is
	// what that repair's own listing draws one row per.
	stripped []sweptStateRecord
	// editor is the editor the fixture wrote into the user config, which is the
	// rung the editor row of dinah config then answers at.
	editor string
	// settings are the config keys dinah config lists, with what the fixture
	// caused each of them to answer.
	settings []sweptSettingRecord
	// benches are the workbenches populateBase created, with the titles
	// sweptRetitle wrote and the slugs they were given.
	benches []sweptBenchRecord
	// workbench is the healthy tree's own fields, as init wrote them.
	workbench sweptWorkbenchRecord
	// workstreams are the workstreams the healthy tree holds, in creation
	// order, and strippedWorkstreams are the ones the tree a slug migration
	// repairs holds, which that repair's third report draws one row per.
	workstreams         []sweptWorkstreamRecord
	strippedWorkstreams []sweptWorkstreamRecord
}

// sweptWorkstreamRecord is one workstream the fixture created: the title it was
// filed under, the slug that title derives, the status it ended in, and the
// cards the fixture joined to it. The identifier is not recorded, because
// NewID mints it and the fixture never reads it back, so the row that draws it
// carries a matcher instead.
type sweptWorkstreamRecord struct {
	title  string
	slug   string
	status string
	cards  []string
}

// sweptCardRecord is one card the fixture filed, carrying what the fixture
// handed dinah add and what the acts it ran afterwards left the card standing
// in. state is an index into sweptRecord.states rather than an identifier,
// since the identifiers of the states init mints are not known to the fixture.
type sweptCardRecord struct {
	ref      string
	title    string
	state    int
	standing string
	holder   string
	reason   string
}

// sweptActRecord is one act the fixture ran against a card: which card, who
// ran it, and the reason or destination it passed.
type sweptActRecord struct {
	card  string
	actor string
	// reason is what a block carried, empty on every other act.
	reason string
	// from and to are the states a move carried, as indexes into
	// sweptRecord.states. to is minus one on every other act.
	from int
	to   int
}

// sweptCommentRecord is one comment the fixture wrote, with the owner it wrote
// it as. The stamp is not recorded, because the fixture cannot know it: that
// column is opaque and carries a predicate instead.
type sweptCommentRecord struct {
	card  string
	actor string
	body  string
}

// sweptLinkRecord is one link the fixture planted in a card's frontmatter.
type sweptLinkRecord struct {
	card string
	kind string
	to   string
}

// sweptAttachmentRecord is one attachment the fixture wrote onto its first
// card. The position is recorded because dinah show draws it; the description
// is empty on a fixture that did not pass --description.
type sweptAttachmentRecord struct {
	card        string
	actor       string
	filename    string
	description string
}

// sweptStateRecord is one state of a tree the fixture built. slug is empty on a
// state the fixture wrote by hand, which is what the missing-slug placeholder
// is drawn for, and id is empty on a state init minted, whose identifier the
// fixture never sees.
type sweptStateRecord struct {
	id            string
	title         string
	slug          string
	kind          string
	operatorOwned bool
}

// ref is what a person types to reach a state, which is its slug where it has
// one and its identifier where it does not. It reproduces bench.State.Ref
// against the fixture's own record rather than against a state read back out
// of a workbench.
func (s sweptStateRecord) ref() string {
	if s.slug != "" {
		return s.slug
	}
	return s.id
}

// sweptSettingRecord is one row of dinah config as the fixture caused it: the
// key, the rung of the ladder that answers for it, and the value that rung
// carries. A setting whose value is the language the invocation asked for
// carries fromTag rather than text, since the sweep passes --lang on every
// invocation and that row therefore draws whichever language it is read in.
// The workbench row draws a resolved path and carries pathIs, for the reason
// sweptBenchRecord gives.
type sweptSettingRecord struct {
	key     string
	source  string
	value   string
	fromTag bool
	pathIs  func(field string) bool
}

// sweptBenchRecord is one workbench the fixture created. pathIs tests a drawn
// field against the path resolvedDir returned for this workbench, and it is
// the only thing that path is reachable through: a closure hands nothing back,
// so an expectation cannot join .dinah onto it, open a card's own file, and
// lift a title out of the store.
type sweptBenchRecord struct {
	title  string
	slug   string
	pathIs func(field string) bool
}

// sweptWorkbenchRecord is what init wrote into the healthy tree's own anchor,
// which is what dinah workbench lists one row per field of.
type sweptWorkbenchRecord struct {
	title    string
	slug     string
	operator string
}

// sweptCell is one expected field. text is what the column draws. match is set
// in its place on the two columns that draw a resolved path, where the fixture
// holds the value as a closure so that no path string reaches an expectation.
// A cell carrying a matcher is compared by calling it, and it rides its own
// position through the permutation self-check.
type sweptCell struct {
	text  string
	match func(field string) bool
}

// sweptExpectation is what a block's rows carry, built from the fixture's own
// record and from the catalog rather than from the composition that fills a
// tableRow.
type sweptExpectation struct {
	// rows are the expected fields, one slice per record, in column order.
	// Every slice is as long as the block's declared keys. A column the
	// record's own row never reaches, and a column the row draws empty, both
	// carry the empty text.
	rows [][]sweptCell
	// source names what the expected side walked and under which condition,
	// which is what a failure reports when the two counts disagree.
	source string
	// opaque names, by column index, the columns whose text the fixture cannot
	// know, each with the predicate its field has to satisfy. Rows are paired
	// on the columns that are not opaque, and an opaque position of an expected
	// row carries the empty string, which nothing compares.
	opaque map[int]func(field string) bool
	// opaqueReason says why each opaque column is opaque, under the same column
	// index. An opaque column with no reason fails the block.
	opaqueReason map[int]string
}

// sweptTexts builds one expected record out of literal fields, which is what
// every column but the two drawing a resolved path carries.
func sweptTexts(fields ...string) []sweptCell {
	cells := make([]sweptCell, 0, len(fields))
	for _, field := range fields {
		cells = append(cells, sweptCell{text: field})
	}
	return cells
}

// sweptMatches reports whether one expected cell answers for one drawn field,
// by calling the matcher the fixture carried or by comparing the text exactly.
func sweptMatches(cell sweptCell, field string) bool {
	if cell.match != nil {
		return cell.match(field)
	}
	return cell.text == field
}

// sweptComparedColumns are the positions a pairing is decided on, which is
// every declared column the expectation did not declare opaque.
func sweptComparedColumns(want sweptExpectation, columns int) []int {
	compared := make([]int, 0, columns)
	for i := 0; i < columns; i++ {
		if want.opaque[i] != nil {
			continue
		}
		compared = append(compared, i)
	}
	return compared
}

// sweptRecordPairs reports whether every drawn record pairs with an expected
// record of its own, one for one, on the columns that are not opaque, and
// returns the first drawn record left over when they do not.
//
// The pairing is a matching rather than a walk down the two lists in step,
// because row order is out of scope and two records can legitimately agree on
// every column a third one also agrees on. A greedy walk answers those cases
// by whichever record it met first, so the augmenting search below is what
// makes a false failure impossible.
func sweptRecordPairs(want [][]sweptCell, got [][]string, compared []int) (bool, []string) {
	if len(want) != len(got) {
		return false, nil
	}
	answers := make([][]bool, len(got))
	for i, drawn := range got {
		answers[i] = make([]bool, len(want))
		for j, expected := range want {
			answers[i][j] = sweptRecordAnswers(expected, drawn, compared)
		}
	}
	taken := make([]int, len(want))
	for j := range taken {
		taken[j] = -1
	}
	for i := range got {
		seen := make([]bool, len(want))
		if !sweptAugment(answers, i, seen, taken) {
			return false, got[i]
		}
	}
	return true, nil
}

// sweptRecordAnswers reports whether one expected record answers for one drawn
// record on every column that is not opaque.
func sweptRecordAnswers(want []sweptCell, got []string, compared []int) bool {
	for _, at := range compared {
		if at >= len(want) || at >= len(got) {
			return false
		}
		if !sweptMatches(want[at], got[at]) {
			return false
		}
	}
	return true
}

// sweptAugment is one augmenting step of the matching: it finds an expected
// record for the drawn record at from, displacing an earlier pairing where
// another expected record is free for it.
func sweptAugment(answers [][]bool, from int, seen []bool, taken []int) bool {
	for j := range taken {
		if !answers[from][j] || seen[j] {
			continue
		}
		seen[j] = true
		if taken[j] < 0 || sweptAugment(answers, taken[j], seen, taken) {
			taken[j] = from
			return true
		}
	}
	return false
}

// sweptPermutations returns every permutation of a block's column positions,
// the identity first.
func sweptPermutations(columns int) [][]int {
	if columns == 0 {
		return [][]int{{}}
	}
	order := make([]int, columns)
	for i := range order {
		order[i] = i
	}
	var built [][]int
	var walk func(at int)
	walk = func(at int) {
		if at == columns {
			held := make([]int, columns)
			copy(held, order)
			built = append(built, held)
			return
		}
		for i := at; i < columns; i++ {
			order[at], order[i] = order[i], order[at]
			walk(at + 1)
			order[at], order[i] = order[i], order[at]
		}
	}
	walk(0)
	return built
}

// sweptIsIdentity reports whether a permutation leaves every column where it
// was, which is the one arrangement the self-check requires to pair.
func sweptIsIdentity(order []int) bool {
	for i, at := range order {
		if i != at {
			return false
		}
	}
	return true
}

// sweptPermuted applies a permutation to every expected record's field vector,
// leaving the opacity map and the compared positions where they are. Position
// i of the permuted record carries what position order[i] carried, so a
// permutation that moves a value under a neighbouring label is exactly what the
// self-check asks the corpus to reject.
func sweptPermuted(rows [][]sweptCell, order []int) [][]sweptCell {
	permuted := make([][]sweptCell, 0, len(rows))
	for _, row := range rows {
		moved := make([]sweptCell, len(order))
		for i, at := range order {
			if at < len(row) {
				moved[i] = row[at]
			}
		}
		permuted = append(permuted, moved)
	}
	return permuted
}

// sweptPadded returns a drawn record padded on the right to the block's
// declared column count.
//
// The table form closes a row as soon as a line stops before the next column's
// edge, so a row whose trailing columns hold nothing comes back short while an
// empty interior column comes back as an explicit empty string. Both sides are
// then full-length vectors, which is what a permutation is defined on.
func sweptPadded(record []string, columns int) []string {
	if len(record) >= columns {
		return record
	}
	padded := make([]string, columns)
	copy(padded, record)
	return padded
}

// sweptOrphanReport is what the pairing assertion says when a drawn record
// pairs with no expected record, which is the one failure the two form
// controls require of it. A control requiring any failure at all would be
// armed by whichever assertion fired first, and the permutation self-check
// fires on the same shifted bytes, so the control names the report it is about.
const sweptOrphanReport = "a drawn record pairs with no expected record"

// sweptCountReport is what the pairing assertion says when the two sides carry
// a different number of records, which is the report the two count controls
// are about.
const sweptCountReport = "the expected side walked"

// sweptReported reports whether an assertion said what a control asked it to
// say, rather than merely having said something.
func sweptReported(reported *sweptFailures, said string) bool {
	for _, report := range reported.reported {
		if strings.Contains(report, said) {
			return true
		}
	}
	return false
}

// sweptReporter is what the pairing assertion reports through. It is
// satisfied by *testing.T, which is what the sweep hands it, and by the
// recorder below, which is what lets a control require that the assertion
// fires without requiring the suite to go red.
type sweptReporter interface {
	Helper()
	Errorf(format string, args ...any)
}

// sweptFailures is the recorder a control reads the assertion's answer out of.
type sweptFailures struct {
	reported []string
}

// Helper does nothing, since a recorder keeps no stack of its own.
func (f *sweptFailures) Helper() {}

// Errorf keeps what an assertion reported, in the order it reported it.
func (f *sweptFailures) Errorf(format string, args ...any) {
	f.reported = append(f.reported, fmt.Sprintf(format, args...))
}

// assertRecordsPair is the seventh assertion, and the one this card exists for:
// the value drawn under a label is the value belonging to that label.
//
// It reports whether a comparison ran and paired at least one record, which is
// what the coverage counter records against the triple of entry, language and
// pass.
func assertRecordsPair(t sweptReporter, block sweptBlock, tag string, want sweptExpectation, got [][]string) bool {
	t.Helper()
	fail := func(format string, args ...any) {
		t.Helper()
		t.Errorf("%s (%s), locale %s, pass %s: "+format, append([]any{block.site, block.label, tag, sweptPass}, args...)...)
	}
	for at := range want.opaque {
		if want.opaqueReason[at] == "" {
			fail("column %d is declared opaque and gives no reason", at)
			return false
		}
	}
	columns := len(block.keys)
	padded := make([][]string, 0, len(got))
	for _, record := range got {
		padded = append(padded, sweptPadded(record, columns))
	}
	if len(padded) != len(want.rows) {
		fail("the block drew %d records and "+sweptCountReport+" %d, which is %s",
			len(padded), len(want.rows), want.source)
		return false
	}
	for _, record := range padded {
		for at, predicate := range want.opaque {
			if at >= len(record) {
				continue
			}
			if !predicate(record[at]) {
				fail("the opaque column %s draws a field its own predicate rejects: %q", block.keys[at], record[at])
				return false
			}
		}
	}
	compared := sweptComparedColumns(want, columns)
	paired, orphan := sweptRecordPairs(want.rows, padded, compared)
	if !paired {
		fail(sweptOrphanReport+": %q", orphan)
		return false
	}
	if len(padded) == 0 {
		return false
	}
	assertNoPermutationSurvives(t, block, tag, want, padded, compared)
	return true
}

// assertNoPermutationSurvives establishes the guard's own discriminating power
// on every run. An expectation stated correctly still proves nothing when a
// block's own columns cannot be told apart, so every non-identity permutation
// of the block's columns applied to the expected records has to fail.
//
// Only the expected record's field vector is permuted. The opacity map, its
// predicates, and the choice of which positions are compared belong to the
// column position and never move, since a guard permuting them would compare
// the same projection on both sides.
func assertNoPermutationSurvives(t sweptReporter, block sweptBlock, tag string, want sweptExpectation, got [][]string, compared []int) {
	t.Helper()
	for _, order := range sweptPermutations(len(block.keys)) {
		if sweptIsIdentity(order) {
			continue
		}
		paired, _ := sweptRecordPairs(sweptPermuted(want.rows, order), got, compared)
		if !paired {
			continue
		}
		t.Errorf("%s (%s), locale %s, pass %s: the permutation %v survives, so the corpus cannot tell the columns %s apart and the pairing this block asserts is vacuous",
			block.site, block.label, tag, sweptPass, order, strings.Join(sweptMovedColumns(block, order), " and "))
		return
	}
}

// sweptMovedColumns names the columns a surviving permutation moved, which is
// what a fixture has to be strengthened to tell apart.
func sweptMovedColumns(block sweptBlock, order []int) []string {
	var moved []string
	for i, at := range order {
		if i == at {
			continue
		}
		moved = append(moved, block.keys[i])
	}
	return moved
}

// sweptHarvestOf runs the command that draws a block and cuts the block's own
// lines out of what it wrote.
//
// A block declaring blanksAreLost keeps the harvest it always had, which
// collects the indented lines following a heading over any number of
// unindented lines between them. A comment's body sits unindented between two
// comment headers, so that block cannot stop at the first line that is not
// indented and cannot keep the separators either.
func sweptHarvestOf(t *testing.T, block sweptBlock, w *sweptWorkbenches, tag string) []string {
	t.Helper()
	out := block.render(t, w, tag)
	opener := ""
	if block.opensAt != "" {
		opener = msg.For(tag).T(block.opensAt)
	}
	if block.blanksAreLost {
		return indentedLinesAfter(out, opener)
	}
	return sweptHarvest(out, opener, sweptSections(block, tag))
}

// sweptStandDown records the one stand-down the guard permits, which is the
// table form on a pass sweptWindows does not mark full.
//
// What the clamp denies there is a table read rather than a comparison:
// continuation clamps a continuation line to the window less minTailColumns,
// which puts it left of the column it continues, so sweptRowLines already
// returns no rows to compare. The stacked form carries no such limit at any
// width and is compared on every pass, the forty-column pass included.
func sweptStandDown(block sweptBlock, tag string, full bool) {
	if full {
		return
	}
	sweptCoverage[sweptTriple(block, tag, sweptPass)] = sweptStoodDown
}

// assertTableRecordsPair pairs the records a block drew as a table against the
// expectation its entry carries, and records the comparison against the triple
// of entry, language and pass.
func assertTableRecordsPair(t *testing.T, block sweptBlock, w *sweptWorkbenches, tag string, lines []string, rows [][]string) {
	t.Helper()
	if block.expect == nil {
		return
	}
	if !sweptDrawnColumns(t, block, tag, rows) {
		return
	}
	if assertRecordsPair(t, block, tag, block.expect(t, w.record, tag), rows) {
		sweptCoverage[sweptTriple(block, tag, sweptPass)] = sweptCompared
	}
	assertDroppingARecordFails(t, block, w, tag, rows)
	assertTruncatingAtASectionFails(t, block, w, tag, lines, false)
}

// assertStackedRecordsPair reads a stacked block back into records keyed by
// label and pairs them against the same expectation by the same rule, so a
// value drawn beside the wrong label fails there too.
func assertStackedRecordsPair(t *testing.T, block sweptBlock, w *sweptWorkbenches, tag string, lines []string) {
	t.Helper()
	if block.expect == nil {
		return
	}
	records := readSweptStackedRecords(t, block, tag, lines)
	if records == nil {
		return
	}
	if !sweptDrawnColumns(t, block, tag, records) {
		return
	}
	if assertRecordsPair(t, block, tag, block.expect(t, w.record, tag), records) {
		sweptCoverage[sweptTriple(block, tag, sweptPass)] = sweptCompared
	}
	assertDroppingARecordFails(t, block, w, tag, records)
	assertTruncatingAtASectionFails(t, block, w, tag, lines, true)
}

// sweptHarvest reads a block to its own boundaries rather than to the first
// separator the output draws.
//
// It starts after the line opener names, or at the top of the output when the
// entry names none, and collects every line up to the first unindented
// non-blank line that is none of the section headings the entry declares. It
// crosses a blank line and keeps it, because the stacked reader splits records
// on it, and it crosses a section heading and keeps it, because a section
// belongs to the block and marks a record boundary in both forms.
//
// The blank lines around the block's own edges are not part of it. A leading
// one stands between the opener and the first record, and a trailing one
// stands between the last record and whatever follows, so both are dropped and
// what is returned begins and ends on a line the block drew.
func sweptHarvest(out, opener string, sections []string) []string {
	var lines []string
	started := opener == ""
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if !started {
			started = line == opener
			continue
		}
		blank := strings.TrimSpace(line) == ""
		belongs := blank || strings.HasPrefix(line, "  ") || sweptIsSection(line, sections)
		if belongs {
			if len(lines) == 0 && blank {
				continue
			}
			lines = append(lines, line)
			continue
		}
		if len(lines) > 0 {
			break
		}
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// sweptIsSection reports whether a line is one of the section headings an
// entry declares, which the harvest crosses and both readers take as a record
// boundary.
func sweptIsSection(line string, sections []string) bool {
	for _, heading := range sections {
		if line == heading {
			return true
		}
	}
	return false
}

// sweptSections renders the section headings a block declares in one language.
func sweptSections(block sweptBlock, tag string) []string {
	headings := make([]string, 0, len(block.sections))
	for _, key := range block.sections {
		headings = append(headings, msg.For(tag).T(key))
	}
	return headings
}

// sweptHelpSections is the catalog keys of the section rows the command list
// draws, derived from the head's own group list so that the entry cannot
// declare a heading the block does not draw and a fifth group moves the
// sections and the expected records together.
func sweptHelpSections() []string {
	keys := make([]string, 0, len(groups))
	for _, group := range groups {
		keys = append(keys, "help.group."+group)
	}
	return keys
}

// sweptRecordLines drops the separators from a harvest, leaving the lines the
// block drew rows on. The table readers below read positions off those lines
// and cannot be shown a blank line or a section heading, neither of which
// begins at a column.
func sweptRecordLines(block sweptBlock, tag string, lines []string) []string {
	sections := sweptSections(block, tag)
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" || sweptIsSection(line, sections) {
			continue
		}
		kept = append(kept, line)
	}
	return kept
}

// sweptFirstRecordLine returns the first line of a harvest that carries a
// record, which is what the form of the block is read off. A block drawing
// section rows opens on a heading, and a heading says nothing about whether
// the rows beneath it were laid out as a table or as a stack.
func sweptFirstRecordLine(block sweptBlock, tag string, lines []string) string {
	kept := sweptRecordLines(block, tag, lines)
	if len(kept) == 0 {
		return ""
	}
	return kept[0]
}

// readSweptStackedRecords reads a stacked block back into one record per block
// of lines, each record a full-length field vector in column order.
//
// A stacked record is not padded, and the table form's rule does not carry
// over to it. stackLines draws no line for a field holding no text wherever
// that field sits, so the reading is keyed by label: each drawn value takes its
// own column's position, and a column whose label the record never drew takes
// the empty string.
//
// One record is separated from the next at the separators the harvest carries,
// which are a blank line and a blank line followed by a section heading. A
// block declaring blanksAreLost is harvested without them, so it is split on a
// label that fails to advance instead, which is the rule assertStackedBlock
// already reads that block by. Where the separators are carried, a label drawn
// twice inside one record is what a merged pair of records looks like from
// inside, and it fails rather than pairing half a record against a whole one.
func readSweptStackedRecords(t *testing.T, block sweptBlock, tag string, lines []string) [][]string {
	t.Helper()
	fail := func(format string, args ...any) [][]string {
		t.Helper()
		t.Errorf("%s (%s), locale %s, pass %s: "+format, append([]any{block.site, block.label, tag, sweptPass}, args...)...)
		return nil
	}
	labels := sweptLabels(block.keys, tag)
	for i, label := range labels {
		for j := i + 1; j < len(labels); j++ {
			if labels[j] == label {
				return fail("the columns %s and %s both draw the label %q, so a value under either one reads as a value under the other",
					block.keys[i], block.keys[j], label)
			}
		}
	}
	widest := 0
	for _, label := range labels {
		if drawn := displayWidth(label); drawn > widest {
			widest = drawn
		}
	}
	values := sweptIndent + widest + sweptGutter
	sections := sweptSections(block, tag)
	var records [][]string
	record := make([]string, len(labels))
	drawn := make([]bool, len(labels))
	open := false
	closeRecord := func() {
		if !open {
			return
		}
		records = append(records, record)
		record = make([]string, len(labels))
		drawn = make([]bool, len(labels))
		open = false
	}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" || sweptIsSection(line, sections) {
			closeRecord()
			continue
		}
		at := stackedLabel(line, labels)
		if at < 0 {
			return fail("a line carries no label the block declares a heading for:\n%q", line)
		}
		if drawn[at] && block.blanksAreLost {
			closeRecord()
		}
		if drawn[at] {
			return fail("one record draws the label %q twice, so two records were read as one:\n%q", labels[at], line)
		}
		record[at] = sweptField(line, values, -1)
		drawn[at] = true
		open = true
	}
	closeRecord()
	return records
}

// sweptDrawnColumns asserts that the block drew every column its entry
// declares, since layOut removes any column no row of a block fills and a
// block that starts to needs somebody to decide what the pairing question
// means there rather than a projection that quietly narrows it.
//
// A column removed that way leaves no label anywhere in the stacked form and
// no field anywhere in the table form, so the drawn records are what it is read
// off. It reports whether every declared column was drawn.
func sweptDrawnColumns(t sweptReporter, block sweptBlock, tag string, records [][]string) bool {
	t.Helper()
	if len(records) == 0 {
		return true
	}
	for at, key := range block.keys {
		filled := false
		for _, record := range records {
			if at < len(record) && record[at] != "" {
				filled = true
				break
			}
		}
		if filled {
			continue
		}
		t.Errorf("%s (%s), locale %s, pass %s: no record fills the column %s, so the block drew fewer columns than the %d its entry declares",
			block.site, block.label, tag, sweptPass, key, len(block.keys))
		return false
	}
	return true
}

// The four states a covered triple of entry, language and pass can be left in.
// Every triple has to record a comparison that paired at least one record, or
// the one stand-down, and a triple recording neither fails naming all three of
// its parts.
const (
	sweptCompared  = "compared"
	sweptStoodDown = "stood down"
)

// sweptCoverage is what the run compared, keyed by the triple of covered
// entry, language tag and running pass. A counter totalling per language and a
// counter totalling per pass are both satisfied by a guard that compared one
// entry in each, which proves close to nothing.
var sweptCoverage = map[string]string{}

// sweptTriple names one covered entry in one language on one pass, which is
// the key the coverage counter is kept against and what a failure reports.
func sweptTriple(block sweptBlock, tag, pass string) string {
	return block.site + " (" + block.label + "), locale " + tag + ", pass " + pass
}

// assertEveryTripleWasCompared asserts the counter: every covered entry, in
// every tag msg.Tags returns, on every pass sweptWindows declares, recorded
// either a comparison or the one stand-down.
func assertEveryTripleWasCompared(t *testing.T) {
	t.Helper()
	for _, block := range sweptBlocks() {
		if len(block.keys) < 2 {
			continue
		}
		for _, tag := range msg.Tags() {
			for pass := range sweptWindows {
				triple := sweptTriple(block, tag, strconv.Itoa(pass))
				if sweptCoverage[triple] != "" {
					continue
				}
				t.Errorf("%s: neither a comparison nor a stand-down was recorded, so the pairing this entry declares was not asserted there", triple)
			}
		}
	}
}

// sweptStampIsATimestamp is the predicate the two opaque stamp columns carry.
// A stamp is drawn in one format in one time zone, which the fixture cannot
// predict and which no other column of either block draws: an owner, an act's
// own token and a card title are all rejected by it.
func sweptStampIsATimestamp(field string) bool {
	if len(field) != len("2006-01-02T15:04:05Z") {
		return false
	}
	for at, r := range field {
		switch at {
		case 4, 7:
			if r != '-' {
				return false
			}
		case 10:
			if r != 'T' {
				return false
			}
		case 13, 16:
			if r != ':' {
				return false
			}
		case 19:
			if r != 'Z' {
				return false
			}
		default:
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

// sweptStampColumn is the opacity one stamp column carries, which is the same
// declaration on both blocks that draw one.
func sweptStampColumn(at int, why string) (map[int]func(field string) bool, map[int]string) {
	return map[int]func(field string) bool{at: sweptStampIsATimestamp}, map[int]string{at: why}
}

// sweptToken renders a machine token for a person the way the head's own token
// method does, which is the catalog's text where it carries one and the
// canonical spelling where it does not.
func sweptToken(tag, name string) string {
	key := "token." + name
	if !msg.For(tag).Has(key) {
		return name
	}
	return msg.For(tag).T(key)
}

// sweptSlugCell renders a slug column's value the way the head's own slugCell
// does: the slug where the entity has one, and the placeholder naming the
// repair where it has none.
func sweptSlugCell(tag, slug string) string {
	if slug == "" {
		return msg.For(tag).T("slug.missing")
	}
	return slug
}

// sweptCardsIn returns the cards the record holds in one state, in the order
// the queue fixes, which is ascending creation ordinal once every card of a
// state arrived in the same second.
func sweptCardsIn(r *sweptRecord, state int) []sweptCardRecord {
	var kept []sweptCardRecord
	for _, card := range r.cards {
		if card.state == state {
			kept = append(kept, card)
		}
	}
	return kept
}

// expectMoves is the legal moves under a served instruction: every state but
// the one the card stands in, under the head's own direction rule, which is
// forward for a state further down the flow and backward for one behind it.
func expectMoves(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	card := r.cards[0]
	var rows [][]sweptCell
	for at, state := range r.states {
		if at == card.state {
			continue
		}
		direction := verb.Backward
		if at > card.state {
			direction = verb.Forward
		}
		rows = append(rows, sweptTexts(state.ref(), state.title, sweptToken(tag, direction)))
	}
	return sweptExpectation{rows: rows, source: "the record's states, other than the one the served card stands in"}
}

// expectHolding is the cards you hold: the record's claims still held by the
// acting owner, which a release and a block each give back.
func expectHolding(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	var rows [][]sweptCell
	for _, card := range r.cards {
		if card.holder != r.actor {
			continue
		}
		rows = append(rows, sweptTexts(card.ref, card.title))
	}
	return sweptExpectation{rows: rows, source: "the record's cards still held by the acting owner"}
}

// expectBlocked is the cards that are blocked, with the reason the fixture
// gave each block.
func expectBlocked(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	var rows [][]sweptCell
	for _, card := range r.cards {
		if card.standing != contract.SubstateBlocked {
			continue
		}
		rows = append(rows, sweptTexts(card.ref, card.reason))
	}
	return sweptExpectation{rows: rows, source: "the record's blocked cards"}
}

// expectStates is dinah states: one row per state the record holds, with the
// occupancy counted off the record's own cards.
func expectStates(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	var rows [][]sweptCell
	for at, state := range r.states {
		owner := msg.For(tag).T("states.moved-by.agent")
		if state.operatorOwned {
			owner = msg.For(tag).T("states.moved-by.operator")
		}
		count := strconv.Itoa(len(sweptCardsIn(r, at)))
		slug := sweptSlugCell(tag, state.slug)
		kind := sweptToken(tag, state.kind)
		rows = append(rows, sweptTexts(slug, state.title, kind, count, owner))
	}
	return sweptExpectation{rows: rows, source: "the record's states"}
}

// expectListing is dinah ls with no state named, which lists the cards of the
// whole workbench.
func expectListing(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	var rows [][]sweptCell
	for _, card := range r.cards {
		rows = append(rows, sweptTexts(card.ref, sweptToken(tag, card.standing), card.title))
	}
	return sweptExpectation{rows: rows, source: "the record's cards, since the entry runs ls with no state"}
}

// expectMatches is dinah query with no query, which selects every card of the
// workbench and carries the state's title where the listing has no state
// column at all.
func expectMatches(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	var rows [][]sweptCell
	for _, card := range r.cards {
		state := r.states[card.state]
		rows = append(rows, sweptTexts(card.ref, state.title, sweptToken(tag, card.standing), card.title))
	}
	return sweptExpectation{rows: rows, source: "the record's cards, since the entry runs query with no argument"}
}

// expectSettings is dinah config: one row per key the tool knows, with the
// value and the rung the fixture caused it to answer at.
func expectSettings(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	var rows [][]sweptCell
	for _, setting := range r.settings {
		value := sweptCell{text: setting.value}
		if setting.fromTag {
			value = sweptCell{text: tag}
		}
		if setting.pathIs != nil {
			value = sweptCell{match: setting.pathIs}
		}
		row := []sweptCell{{text: setting.key}, value, {text: sweptToken(tag, setting.source)}}
		rows = append(rows, row)
	}
	return sweptExpectation{rows: rows, source: "the record's settings, which are the keys config lists"}
}

// expectWorkbenches is the workbench listing, which one call site draws on
// stdout and the ambiguous-workbench refusal draws on stderr.
func expectWorkbenches(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	var rows [][]sweptCell
	for _, room := range r.benches {
		row := []sweptCell{{text: room.title}, {text: sweptSlugCell(tag, room.slug)}, {match: room.pathIs}}
		rows = append(rows, row)
	}
	return sweptExpectation{rows: rows, source: "the record's workbenches"}
}

// expectOffers is dinah next: one row per state, carrying the ready card of
// lowest creation ordinal where the state has one and the catalog's own line
// where it has none.
func expectOffers(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	var rows [][]sweptCell
	for at, state := range r.states {
		offered := sweptCardRecord{}
		for _, card := range sweptCardsIn(r, at) {
			if card.standing != contract.SubstateReady {
				continue
			}
			offered = card
			break
		}
		if offered.ref == "" {
			rows = append(rows, sweptTexts(state.title, msg.For(tag).T("next.none"), ""))
			continue
		}
		rows = append(rows, sweptTexts(state.title, offered.ref, offered.title))
	}
	return sweptExpectation{rows: rows, source: "the record's states, each under the head's own ready-card condition"}
}

// expectLinks is a card's links, which the fixture planted in its frontmatter.
func expectLinks(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	var rows [][]sweptCell
	for _, link := range r.links {
		rows = append(rows, sweptTexts(link.kind, link.to))
	}
	return sweptExpectation{rows: rows, source: "the record's links"}
}

// expectComments is a card's comments, with the stamp column opaque because
// the test that provoked it cannot predict it.
func expectComments(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	var rows [][]sweptCell
	for _, comment := range r.comments {
		rows = append(rows, sweptTexts("", comment.actor))
	}
	opaque, why := sweptStampColumn(0, "a comment's stamp is the moment the fixture ran, which the fixture cannot know before it runs")
	return sweptExpectation{
		rows:         rows,
		source:       "the record's comments",
		opaque:       opaque,
		opaqueReason: why,
	}
}

// expectAttachments is a card's attachments, in the position order the
// fixture wrote them. The position is the row's first column and the
// description is what --description carried, empty when the fixture passed none.
// A rename the fixture ran against one of these attachments rewrites its
// filename to the new one, since the rendered block carries the post-rename
// filename rather than the original.
func expectAttachments(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	filenames := make([]string, len(r.attachments))
	for i, attachment := range r.attachments {
		filenames[i] = attachment.filename
	}
	for _, act := range r.renames {
		_, posStr, _ := strings.Cut(act.card, "/attachments/")
		pos, err := strconv.Atoi(posStr)
		if err != nil || pos < 1 || pos > len(filenames) {
			continue
		}
		filenames[pos-1] = act.reason
	}
	var rows [][]sweptCell
	for i, attachment := range r.attachments {
		rows = append(rows, sweptTexts(strconv.Itoa(i+1), filenames[i], attachment.description))
	}
	return sweptExpectation{rows: rows, source: "the record's attachments, with renames applied"}
}

// expectHistory is dinah log against the held card: the card's own creation
// and every act the record carries against it, with the stamp column opaque
// and the detail column drawn under the head's own rule for each act.
func expectHistory(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	card := r.cards[0]
	rows := [][]sweptCell{sweptTexts("", sweptToken(tag, contract.EventCreated), r.actor, card.title)}
	for _, act := range r.claims {
		if act.card != card.ref {
			continue
		}
		rows = append(rows, sweptTexts("", sweptToken(tag, contract.EventClaimed), act.actor, ""))
	}
	for _, act := range r.blocks {
		if act.card != card.ref {
			continue
		}
		rows = append(rows, sweptTexts("", sweptToken(tag, contract.EventBlocked), act.actor, act.reason))
	}
	for _, act := range r.moves {
		if act.card != card.ref {
			continue
		}
		carried := msg.For(tag).T("log.moved", "from", r.states[act.from].title, "to", r.states[act.to].title)
		rows = append(rows, sweptTexts("", sweptToken(tag, contract.EventMoved), act.actor, carried))
	}
	for _, act := range r.releases {
		if act.card != card.ref {
			continue
		}
		rows = append(rows, sweptTexts("", sweptToken(tag, contract.EventReleased), act.actor, ""))
	}
	for _, comment := range r.comments {
		if comment.card != card.ref {
			continue
		}
		rows = append(rows, sweptTexts("", sweptToken(tag, contract.EventCommented), comment.actor, ""))
	}
	for _, attachment := range r.attachments {
		if attachment.card != card.ref {
			continue
		}
		rows = append(rows, sweptTexts("", sweptToken(tag, contract.EventAttached), attachment.actor, attachment.filename))
	}
	for _, act := range r.renames {
		cardRef, _, _ := strings.Cut(act.card, "/attachments/")
		if cardRef != card.ref {
			continue
		}
		// The act's reason carries the new filename; the previous one is
		// read off the record by the position the reference names.
		var previous string
		_, posStr, _ := strings.Cut(act.card, "/attachments/")
		if pos, err := strconv.Atoi(posStr); err == nil {
			if pos-1 >= 0 && pos-1 < len(r.attachments) {
				previous = r.attachments[pos-1].filename
			}
		}
		carried := msg.For(tag).T("log.attachment-renamed", "from", previous, "to", act.reason)
		rows = append(rows, sweptTexts("", sweptToken(tag, contract.EventAttachmentRenamed), act.actor, carried))
	}
	for _, act := range r.joins {
		if act.card != card.ref {
			continue
		}
		rows = append(rows, sweptTexts("", sweptToken(tag, contract.EventWorkstreamJoined), act.actor, ""))
	}
	opaque, why := sweptStampColumn(0, "an act's stamp is the moment the fixture ran, which the fixture cannot know before it runs")
	return sweptExpectation{
		rows:         rows,
		source:       "the record's acts against the held card",
		opaque:       opaque,
		opaqueReason: why,
	}
}

// expectAssignedSlugs is the slugs a migration assigned, which it derives from
// each state's own title through the same call the head makes.
func expectAssignedSlugs(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	var rows [][]sweptCell
	for _, state := range r.stripped {
		rows = append(rows, sweptTexts(bench.SlugifyDashed(state.title), state.title))
	}
	return sweptExpectation{rows: rows, source: "the states the stripped tree holds"}
}

// expectCatalogs is the catalog-coverage block, which walks the tags the
// binary ships and reads each one's coverage out of the catalog rather than
// out of the row that drew it.
func expectCatalogs(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	var rows [][]sweptCell
	for _, shipped := range msg.Tags() {
		translated, _, total := msg.Coverage(shipped)
		rows = append(rows, sweptTexts(shipped, strconv.Itoa(translated)+"/"+strconv.Itoa(total)))
	}
	return sweptExpectation{rows: rows, source: "msg.Tags()"}
}

// expectCommands is the command list of bare dinah, which walks the head's own
// group list on the outside and its command list on the inside under the group
// condition helpBlock applies, so it holds one record for every command
// carrying a group and one fewer than commands declares elements.
func expectCommands(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	var rows [][]sweptCell
	for _, group := range groups {
		for _, c := range commands {
			if c.group != group {
				continue
			}
			rows = append(rows, sweptTexts(verb.Usage(c.name), msg.For(tag).T("cmd."+c.name+".summary")))
		}
	}
	return sweptExpectation{rows: rows, source: "groups and commands under helpBlock's own group condition"}
}

// expectFlags is the global flag list, which walks the head's own flag
// declaration.
func expectFlags(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	var rows [][]sweptCell
	for _, flag := range globalFlags {
		rows = append(rows, sweptTexts(flag.usage, msg.For(tag).T("flag."+flag.name+".summary")))
	}
	return sweptExpectation{rows: rows, source: "globalFlags"}
}

// sweptHelpCommand is the command dinah help <command> is asked about, which
// the entry that renders the refusal list runs and the expectation walks the
// checks of.
const sweptHelpCommand = "add"

// expectRefusals is the refusal list of dinah help <command>, which walks the
// profile's own checks for the command asked about, in the profile's order.
func expectRefusals(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	var rows [][]sweptCell
	for i, check := range verb.Checks(sweptHelpCommand) {
		rows = append(rows, sweptTexts(strconv.Itoa(i+1), msg.For(tag).T(check.Key), check.Refusal))
	}
	return sweptExpectation{rows: rows, source: "verb.Checks(" + sweptHelpCommand + ")"}
}

// expectGuides is the guide topics, which walks the guide documents the binary
// carries.
func expectGuides(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	var rows [][]sweptCell
	for _, topic := range guide.Topics() {
		rows = append(rows, sweptTexts(topic, guide.Title(topic)))
	}
	return sweptExpectation{rows: rows, source: "the guide documents the binary carries"}
}

// expectWorkbenchFields is the workbench's own fields, which walks the field
// names the tool knows and reads each one off what init wrote.
func expectWorkbenchFields(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	var rows [][]sweptCell
	for _, name := range bench.WorkbenchFields {
		value := ""
		switch name {
		case "title":
			value = r.workbench.title
		case "slug":
			value = sweptSlugCell(tag, r.workbench.slug)
		case "operator":
			value = r.workbench.operator
		}
		rows = append(rows, sweptTexts(name, value))
	}
	return sweptExpectation{rows: rows, source: "the fields workbench lists"}
}

// expectWorkstreams is dinah workstream, one row per workstream the fixture
// created, with the member count derived from the joins the fixture ran rather
// than read back off the workstream.
func expectWorkstreams(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	var rows [][]sweptCell
	for _, workstream := range r.workstreams {
		rows = append(rows, sweptTexts(
			sweptSlugCell(tag, workstream.slug),
			workstream.title,
			workstream.status,
			strconv.Itoa(len(workstream.cards)),
		))
	}
	return sweptExpectation{rows: rows, source: "the record's workstreams"}
}

// expectWorkstreamFields is one workstream's own fields, which walks the rows
// the detail draws and reads each one off the record.
//
// The identifier row carries a matcher rather than text, for the reason
// sweptWorkstreamRecord gives, and it is a cell matcher rather than an opaque
// column because the value that cannot be known is one row of the value column
// rather than the column itself.
func expectWorkstreamFields(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	workstream := sweptWorkstreamNamed(t, r, sweptWorkstream)
	rows := [][]sweptCell{
		sweptTexts("slug", sweptSlugCell(tag, workstream.slug)),
		{{text: "id"}, {match: bench.IsID}},
		sweptTexts("title", workstream.title),
		sweptTexts("status", workstream.status),
		sweptTexts("cards", strconv.Itoa(len(workstream.cards))),
	}
	return sweptExpectation{rows: rows, source: "the fields one workstream's detail lists"}
}

// expectWorkstreamMembers is the cards belonging to one workstream, read out of
// the joins the fixture ran and carrying each card's own state title.
func expectWorkstreamMembers(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	workstream := sweptWorkstreamNamed(t, r, sweptWorkstream)
	var rows [][]sweptCell
	for _, ref := range workstream.cards {
		card := r.cards[sweptCardAt(r, ref)]
		rows = append(rows, sweptTexts(card.ref, card.title, r.states[card.state].title))
	}
	return sweptExpectation{rows: rows, source: "the cards the record joined to " + sweptWorkstream}
}

// expectAssignedWorkstreamSlugs is the third report of one slug migration,
// which derives a slug from each workstream's own title through the same call
// the head makes.
func expectAssignedWorkstreamSlugs(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	var rows [][]sweptCell
	for _, workstream := range r.strippedWorkstreams {
		rows = append(rows, sweptTexts(bench.SlugifyDashed(workstream.title), workstream.title))
	}
	return sweptExpectation{rows: rows, source: "the workstreams the stripped tree holds"}
}

// sweptWorkstreamNamed is the record of the workstream an entry reads, found by
// the slug that entry names.
func sweptWorkstreamNamed(t *testing.T, r *sweptRecord, slug string) sweptWorkstreamRecord {
	t.Helper()
	for _, workstream := range r.workstreams {
		if workstream.slug == slug {
			return workstream
		}
	}
	t.Fatalf("the record holds no workstream under the slug %q, so the entry that reads one has nothing to compare", slug)
	return sweptWorkstreamRecord{}
}

// The suite carries four controls, and each one perturbs something and
// requires the pairing assertion or its record count to report the
// perturbation. A test that passes proves nothing on its own, since it also
// passes when the thing it guards is absent.
//
// sweptControlRows are what the two form controls build their table out of.
// The values are chosen so that no column of the control can be mistaken for
// another: a shift of one position along moves a reference under a standing
// and a title under a reference, and neither pairs.
func sweptControlRows() [][]string {
	return [][]string{
		{"demo-1", "one", "a card of some length"},
		{"demo-2", "two", "a second card of some length"},
	}
}

// sweptControlBlock is the entry the two form controls are read through. It
// declares the three columns of dinah ls, which is a block of three columns
// whose headings are short enough for the control to hold its table shape at a
// full pass and long enough to stack at the narrow one.
func sweptControlBlock() sweptBlock {
	return sweptBlock{
		site:  "row_sweep_test.go",
		label: "the control block the pairing assertion arms itself with",
		keys:  []string{"column.ls.card", "column.ls.standing", "column.ls.title"},
	}
}

// sweptShifted moves every field of every record one position along, which is
// the defect this card exists to catch: the labels stay where they are and the
// values slide under their neighbours.
func sweptShifted(rows [][]string) [][]string {
	shifted := make([][]string, 0, len(rows))
	for _, row := range rows {
		moved := make([]string, len(row))
		for i := range row {
			moved[(i+1)%len(row)] = row[i]
		}
		shifted = append(shifted, moved)
	}
	return shifted
}

// sweptControlExpectation is the truth the two form controls compare a shifted
// table against, which is the rows the control composed before it shifted them.
func sweptControlExpectation() sweptExpectation {
	var rows [][]sweptCell
	for _, row := range sweptControlRows() {
		rows = append(rows, sweptTexts(row...))
	}
	return sweptExpectation{rows: rows, source: "the rows the control composed"}
}

// sweptControlLines draws the control's shifted rows through the head's own
// tableLines, so the control reads bytes the head laid out rather than bytes it
// composed itself.
func sweptControlLines(tag string) []string {
	s := &session{r: msg.For(tag), width: sweptWindow}
	built := table{indent: sweptIndent, columns: s.columns("ls", "card", "standing", "title")}
	for _, row := range sweptShifted(sweptControlRows()) {
		built.rows = append(built.rows, tableRow{fields: row})
	}
	return s.tableLines(built)
}

// assertThePairingCheckCanFail arms the table form's pairing assertion, in
// every shipped language, on each pass that reads a table back.
//
// It builds a table through the head's own tableLines with the fields shifted
// one position along, reads it back the way the sweep reads the corpus, and
// requires the pairing assertion to report it. Removing the pairing assertion
// from the table path makes this control fail.
func assertThePairingCheckCanFail(t *testing.T) {
	t.Helper()
	block := sweptControlBlock()
	for _, tag := range msg.Tags() {
		lines := sweptControlLines(tag)
		if drawsTheStackedForm(block, tag, lines[0]) {
			t.Errorf("locale %s: the control block stacked at a window of %d, so it arms the table form's pairing assertion with nothing:\n%q", tag, sweptWindow, lines[0])
			continue
		}
		columns := assertHeadingRow(t, block, tag, lines)
		if columns == nil {
			continue
		}
		rows := readSweptRows(t, block, tag, lines[2:], columns)
		reported := &sweptFailures{}
		assertRecordsPair(reported, block, tag, sweptControlExpectation(), rows)
		if !sweptReported(reported, sweptOrphanReport) {
			t.Errorf("locale %s: a table whose fields are shifted one position along paired against the rows they were composed from, so the table form's pairing assertion asserts nothing", tag)
		}
	}
}

// assertTheStackedPairingCheckCanFail arms the stacked form's pairing
// assertion, in every shipped language, on the narrow pass.
//
// It is the same control read through the other reader. Removing the pairing
// assertion from the stacked path makes it fail.
func assertTheStackedPairingCheckCanFail(t *testing.T) {
	t.Helper()
	block := sweptControlBlock()
	for _, tag := range msg.Tags() {
		lines := sweptControlLines(tag)
		if !drawsTheStackedForm(block, tag, lines[0]) {
			t.Errorf("locale %s: the control block held its table shape at a window of %d, so it arms the stacked form's pairing assertion with nothing:\n%q", tag, sweptWindow, lines[0])
			continue
		}
		records := readSweptStackedRecords(t, block, tag, lines)
		if records == nil {
			continue
		}
		reported := &sweptFailures{}
		assertRecordsPair(reported, block, tag, sweptControlExpectation(), records)
		if !sweptReported(reported, sweptOrphanReport) {
			t.Errorf("locale %s: a stack whose fields are shifted one position along paired against the rows they were composed from, so the stacked form's pairing assertion asserts nothing", tag)
		}
	}
}

// assertDroppingARecordFails is the first of the two controls that arm the
// record count, and it runs against the corpus rather than against a table the
// control composed.
//
// Dropping the last record a covered entry drew has to make that entry's
// comparison fail. Neither count control perturbs the expectation, and the
// reason belongs beside them: an expectation narrowed by a condition tighter
// than the head's own draws fewer records than the head drew while the harvest
// still carries every record the block drew, so the counts disagree and the
// build goes red against the whole harvest. These two prove that the harvest
// side cannot shrink unnoticed, and a harvest that cannot shrink is what makes
// an over-tight expectation visible.
func assertDroppingARecordFails(t *testing.T, block sweptBlock, w *sweptWorkbenches, tag string, records [][]string) {
	t.Helper()
	if len(records) == 0 {
		return
	}
	reported := &sweptFailures{}
	assertRecordsPair(reported, block, tag, block.expect(t, w.record, tag), records[:len(records)-1])
	if sweptReported(reported, sweptCountReport) {
		return
	}
	t.Errorf("%s (%s), locale %s, pass %s: a harvest one record short of what the block drew still paired, so the record count asserts nothing here",
		block.site, block.label, tag, sweptPass)
}

// assertTruncatingAtASectionFails is the second count control, and it states
// the first-group-only result as a check the suite runs.
//
// An entry declaring section rows whose harvest stops at its first section
// boundary carries the records of one group alone, which is what the harvest
// returned before this card and what an expectation written to that harvest
// would have agreed with. Truncating there has to make the entry's comparison
// fail.
func assertTruncatingAtASectionFails(t *testing.T, block sweptBlock, w *sweptWorkbenches, tag string, lines []string, stacked bool) {
	t.Helper()
	if len(block.sections) == 0 {
		return
	}
	truncated := sweptTruncatedAtASection(block, tag, lines)
	if len(truncated) == len(lines) {
		t.Errorf("%s (%s), locale %s, pass %s: the harvest carries no second section, so this control truncates nothing",
			block.site, block.label, tag, sweptPass)
		return
	}
	records := sweptControlRecords(t, block, tag, truncated, stacked)
	if records == nil {
		return
	}
	reported := &sweptFailures{}
	assertRecordsPair(reported, block, tag, block.expect(t, w.record, tag), records)
	if sweptReported(reported, sweptCountReport) {
		return
	}
	t.Errorf("%s (%s), locale %s, pass %s: a harvest stopping at the block's first section boundary still paired, so the record count asserts nothing here",
		block.site, block.label, tag, sweptPass)
}

// sweptTruncatedAtASection cuts a harvest at the block's first section
// boundary, which is the second section heading it draws: the first one opens
// the block rather than ending anything.
func sweptTruncatedAtASection(block sweptBlock, tag string, lines []string) []string {
	sections := sweptSections(block, tag)
	seen := 0
	for at, line := range lines {
		if !sweptIsSection(line, sections) {
			continue
		}
		seen++
		if seen == 2 {
			return lines[:at]
		}
	}
	return lines
}

// sweptControlRecords reads a perturbed harvest back into records through
// whichever reader the block's own form is read by, so a control compares what
// the sweep would have compared.
func sweptControlRecords(t *testing.T, block sweptBlock, tag string, lines []string, stacked bool) [][]string {
	t.Helper()
	if stacked {
		return readSweptStackedRecords(t, block, tag, lines)
	}
	recordLines := sweptRecordLines(block, tag, lines)
	if block.noHeadingRow {
		columns := deriveHeadinglessColumns(t, block, tag, recordLines)
		if columns == nil {
			return nil
		}
		return readSweptRows(t, block, tag, recordLines, columns)
	}
	columns := assertHeadingRow(t, block, tag, recordLines)
	if columns == nil {
		return nil
	}
	return readSweptRows(t, block, tag, recordLines[2:], columns)
}

// sweptTreeNode is one expected row of a tree block before its guide prefix is
// composed: the four fields the row carries, and the rows nested under it. The
// nesting is what decides the prefix, so the expectation is written as a shape
// and flattened rather than written flat.
type sweptTreeNode struct {
	// ref is what the Reference column carries before the guide, which is the
	// group's value on a group row and the entity's own reference elsewhere.
	ref string
	// entity is the Entity column: the axis on a group row and the kind on an
	// entity row. Neither is translated, so both are the same in every locale.
	entity string
	// title is the Title column, empty on every group row.
	title string
	// count is the Count column, empty on a card of the grouped producer.
	count string
	// children are the rows drawn under this one.
	children []sweptTreeNode
}

// sweptTreeRows flattens an expected tree, composing each row's guide prefix
// out of its ancestors. A level whose node was the last of its siblings
// contributes four spaces and every other contributes a trunk, and the node's
// own piece is the corner where it is last and the tee where it is not. The
// four pieces are the spec's own.
func sweptTreeRows(nodes []sweptTreeNode, ancestors string) [][]sweptCell {
	var rows [][]sweptCell
	for at, node := range nodes {
		piece, below := "|-- ", "|   "
		if at == len(nodes)-1 {
			piece, below = "`-- ", "    "
		}
		rows = append(rows, sweptTexts(ancestors+piece+node.ref, node.entity, node.title, node.count))
		rows = append(rows, sweptTreeRows(node.children, ancestors+below)...)
	}
	return rows
}

// sweptSubstates are the three substates the grouped producer draws under a
// state, in the order it draws them, which is the order the contract declares
// them in rather than an order this fixture chose.
var sweptSubstates = []string{contract.SubstateReady, contract.SubstateActive, contract.SubstateBlocked}

// expectTree is dinah tree with no arguments: a group per declared state in
// flow order, the three substates under each one whether or not a card stands
// there, and the cards themselves at the third level.
//
// A group carries its value under Reference, its axis under Entity, nothing
// under Title, and its own card count under Count. A card carries no count at
// all, since the grouped producer counts cards rather than what a card holds.
//
// The cards of a group come in the order the fixture filed them. That is the
// arrival order the producer draws here because no card of any group in this
// fixture moved after another card of the same group was filed, and a fixture
// that breaks that reddens this expectation rather than passing quietly.
func expectTree(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	var states []sweptTreeNode
	for at, state := range r.states {
		held := sweptCardsIn(r, at)
		node := sweptTreeNode{ref: state.ref(), entity: bench.KindState, count: strconv.Itoa(len(held))}
		for _, substate := range sweptSubstates {
			var cards []sweptTreeNode
			for _, card := range held {
				if card.standing != substate {
					continue
				}
				cards = append(cards, sweptTreeNode{ref: card.ref, entity: bench.KindCard, title: card.title})
			}
			group := sweptTreeNode{
				ref:      substate,
				entity:   verb.FieldSubstate,
				count:    strconv.Itoa(len(cards)),
				children: cards,
			}
			node.children = append(node.children, group)
		}
		states = append(states, node)
	}
	return sweptExpectation{
		rows:   sweptTreeRows(states, ""),
		source: "the record's states and cards, nested the way the default chain groups them",
	}
}

// expectContents is dinah contents from the workbench root: the states in flow
// order, then the cards in arrival order, then whatever each card holds.
//
// Every row carries a count here, including a card holding nothing, because
// the containment producer counts what an entity contains rather than the
// cards below a group.
func expectContents(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	var nodes []sweptTreeNode
	for _, state := range r.states {
		nodes = append(nodes, sweptTreeNode{
			ref:    state.ref(),
			entity: bench.KindState,
			title:  state.title,
			count:  "0",
		})
	}
	for _, card := range sweptArrivalOrder(r) {
		var comments []sweptTreeNode
		for at, comment := range sweptCommentsOn(r, card.ref) {
			comments = append(comments, sweptTreeNode{
				ref:    card.ref + "/comments/" + strconv.Itoa(at+1),
				entity: bench.KindComment,
				title:  comment.body,
				count:  "0",
			})
		}
		var attachments []sweptTreeNode
		for at, attachment := range sweptAttachmentsOn(r, card.ref) {
			title := attachment.filename
			if attachment.description != "" {
				title = attachment.description
			}
			attachments = append(attachments, sweptTreeNode{
				ref:    card.ref + "/attachments/" + strconv.Itoa(at+1),
				entity: bench.KindAttachment,
				title:  title,
				count:  "0",
			})
		}
		nodes = append(nodes, sweptTreeNode{
			ref:      card.ref,
			entity:   bench.KindCard,
			title:    card.title,
			count:    strconv.Itoa(len(comments) + len(attachments)),
			children: append(comments, attachments...),
		})
	}
	return sweptExpectation{
		rows:   sweptTreeRows(nodes, ""),
		source: "the record's states and cards, walked the way the containment grammar mounts them",
	}
}

// sweptArrivalOrder is the record's cards in the order the containment walk
// draws them, which is the order they arrived in the state they now stand in.
//
// A card the fixture never moved arrived when it was filed, and every move the
// fixture ran happened after every card was filed, so the unmoved cards come
// first in filing order and the moved ones follow in the order of their last
// move. That reproduces the arrival rule against the record rather than
// against a card read back out of a workbench.
func sweptArrivalOrder(r *sweptRecord) []sweptCardRecord {
	moved := map[string]int{}
	for at, act := range r.moves {
		moved[act.card] = at
	}
	var stayed, travelled []sweptCardRecord
	for _, card := range r.cards {
		if _, ok := moved[card.ref]; ok {
			travelled = append(travelled, card)
			continue
		}
		stayed = append(stayed, card)
	}
	sort.SliceStable(travelled, func(i, j int) bool {
		return moved[travelled[i].ref] < moved[travelled[j].ref]
	})
	return append(stayed, travelled...)
}

// sweptCommentsOn is the comments the record wrote onto one card, in the order
// it wrote them, which is the order a positional reference counts in.
func sweptCommentsOn(r *sweptRecord, ref string) []sweptCommentRecord {
	var kept []sweptCommentRecord
	for _, comment := range r.comments {
		if comment.card == ref {
			kept = append(kept, comment)
		}
	}
	return kept
}

// sweptAttachmentsOn is the attachments the record wrote onto one card, in
// the order the fixture wrote them, which is the order a positional reference
// counts in.
func sweptAttachmentsOn(r *sweptRecord, ref string) []sweptAttachmentRecord {
	var kept []sweptAttachmentRecord
	for _, attachment := range r.attachments {
		if attachment.card == ref {
			kept = append(kept, attachment)
		}
	}
	return kept
}

// sweptArgumentsCommand is the command dinah help <command> is asked about for
// the arguments block. attach is the choice because every one of its arguments
// declares a vocabulary of nothing, so the expected meaning is the catalog
// sentence alone and the expectation reads no workbench.
const sweptArgumentsCommand = "attach"

// expectArguments is the arguments table of dinah help <command>, which walks
// the command's own parameter list: the token on the left and the sentence the
// parameter names on the right, both from the one place each is declared.
func expectArguments(t *testing.T, r *sweptRecord, tag string) sweptExpectation {
	t.Helper()
	var rows [][]sweptCell
	for _, param := range verb.Params(sweptArgumentsCommand) {
		meaning := msg.For(tag).T(param.SummaryKey(sweptArgumentsCommand))
		rows = append(rows, sweptTexts(param.Token(), meaning))
	}
	return sweptExpectation{rows: rows, source: "verb.Params(" + sweptArgumentsCommand + ")"}
}
