package main

import (
	"sort"
	"strconv"
	"strings"
	"unicode"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/screen"
	"dinah/internal/verb"
)

// The two glyph sets the board draws with. Status is carried by the glyph
// and never by colour alone, so the drawing reads the same with colour off.
// Every mark of the plain set is ASCII, for a console or a font that cannot
// show the Unicode ones; no documented interface reports whether it can, so
// the set is chosen and never detected.
var (
	unicodeGlyphs = boardGlyphs{ready: "○", active: "●", blocked: "✕", operator: "◆", rule: '─', ellipsis: tailEllipsis}
	plainGlyphs   = boardGlyphs{ready: "o", active: "*", blocked: "x", operator: "!", rule: '-', ellipsis: "..."}
)

// glyphsSetting is the user setting that switches the board to the plain
// set permanently, and glyphsPlain is the value that does it.
const (
	glyphsSetting = "glyphs"
	glyphsPlain   = "plain"
)

// narrowestDrawWindow is the narrowest window a columns drawing is laid out
// for: one column to draw in and the last column of the window kept clear.
const narrowestDrawWindow = 2

// assumedDrawWindow is the window a columns drawing is laid out for when no
// documented source states one: the tables' assumedWindow, for the same
// reason, so a board redirected with no stated width is the same drawing in
// every terminal and on every machine.
const assumedDrawWindow = assumedWindow

// boardGlyphSet chooses the glyph set: --plain for one run, the user's glyphs
// setting for every run, and the Unicode set otherwise, on the operator's
// ruling of 2026-09-25.
func (s *session) boardGlyphSet(plain bool) boardGlyphs {
	if plain || s.cfg.Get(glyphsSetting) == glyphsPlain {
		return plainGlyphs
	}
	return unicodeGlyphs
}

// boardWindow is the window a columns drawing is laid out for: the width the
// ladder states, unclamped, or assumedDrawWindow where nothing states one. A
// window of one column is drawn as two, the one case where a line reaches
// the window's last column.
func boardWindow(stated int) int {
	switch {
	case stated <= 0:
		return assumedDrawWindow
	case stated < narrowestDrawWindow:
		return narrowestDrawWindow
	}
	return stated
}

// columnsView draws a view in the columns layout for a window of the given
// width, every line in one column fewer than the window: a heading naming
// the view and who it was asked as, the collapsed columns and their counts,
// and each section's board. A view of one section draws its board with no
// section heading.
func (s *session) columnsView(answer *verb.ViewAnswer, b *bench.Bench, glyphs boardGlyphs, window int, all bool) []drawnLine {
	draw := window - 1
	body := answer.View
	acting := ""
	if body.Actor != "" {
		acting = s.r.T("view.acting", "actor", body.Actor)
		if b.Operator != "" && body.Actor == b.Operator {
			acting = s.r.T("view.acting.operator", "actor", body.Actor)
		}
	}
	title := withoutControls(body.Title)
	lines := []drawnLine{{text: boardHeadingLine(title, acting, draw, glyphs.ellipsis)}}
	if collapsed := s.collapsedLine(body, b); collapsed != "" {
		lines = append(lines, plainLines(boardProse(collapsed, 0, draw, glyphs.ellipsis))...)
	}
	lines = append(lines, drawnLine{})
	for i, section := range body.Sections {
		if len(body.Sections) > 1 {
			if i > 0 {
				lines = append(lines, drawnLine{})
			}
			heading := s.r.T("view.section.heading", "title", withoutControls(section.Title), "count", strconv.Itoa(section.Count))
			lines = append(lines, plainLines(boardProse(heading, 0, draw, glyphs.ellipsis))...)
		}
		switch {
		case section.Refused != "":
			for _, refused := range s.refusedSectionLines(section) {
				lines = append(lines, plainLines(boardProse(refused.text, refused.indent, draw, glyphs.ellipsis))...)
			}
		case section.Count == 0:
			lines = append(lines, plainLines(boardProse(s.r.T("view.section.empty"), 2, draw, glyphs.ellipsis))...)
		default:
			columns := s.sectionColumns(section, body, b, glyphs, all)
			lines = append(lines, boardLines(columns, draw, glyphs)...)
		}
	}
	return lines
}

// plainLines turns plain lines into drawn lines, which carry no colour.
func plainLines(lines []string) []drawnLine {
	drawn := make([]drawnLine, 0, len(lines))
	for _, line := range lines {
		drawn = append(drawn, drawnLine{text: line})
	}
	return drawn
}

// collapsedLine names each collapsed column that holds at least one card of
// the view, in flow order, with its count summed across the sections, or
// answers the empty string where none holds any.
func (s *session) collapsedLine(body verb.ViewBody, b *bench.Bench) string {
	counts := map[string]int{}
	for _, section := range body.Sections {
		for _, card := range section.Cards {
			counts[card.Column]++
		}
	}
	var entries []string
	for _, id := range body.Collapsed {
		if counts[id] == 0 {
			continue
		}
		title := id
		if column := b.Column(id); column != nil {
			title = column.Title
		}
		entries = append(entries, s.r.T("view.columns.collapsed.entry", "title", withoutControls(title), "count", strconv.Itoa(counts[id])))
	}
	if len(entries) == 0 {
		return ""
	}
	return s.r.T("view.columns.collapsed", "columns", strings.Join(entries, s.r.T("view.columns.collapsed.separator")))
}

// sectionColumns chooses the columns one section draws: the flow's columns
// in flow order that are not collapsed and hold at least one of the
// section's cards, then one trailing column for each column identifier a
// card stands in that the flow does not list, in byte order of the
// identifier, titled by the stored title of its first card. Inside a column
// the cards keep the section's order, which is the view's order.
func (s *session) sectionColumns(section verb.ViewSectionAnswer, body verb.ViewBody, b *bench.Bench, glyphs boardGlyphs, all bool) []boardColumn {
	collapsed := map[string]bool{}
	for _, id := range body.Collapsed {
		collapsed[id] = true
	}
	byColumn := map[string][]verb.CardView{}
	for _, card := range section.Cards {
		byColumn[card.Column] = append(byColumn[card.Column], card)
	}
	var columns []boardColumn
	for _, column := range b.Columns {
		cards := byColumn[column.ID]
		if collapsed[column.ID] || len(cards) == 0 {
			continue
		}
		columns = append(columns, s.boardColumnOf(column.Title, column.OperatorOwned, cards, glyphs, all))
	}
	var unlisted []string
	for id := range byColumn {
		if b.Column(id) == nil {
			unlisted = append(unlisted, id)
		}
	}
	sort.Strings(unlisted)
	for _, id := range unlisted {
		cards := byColumn[id]
		title := cards[0].ColumnTitle
		if title == "" {
			title = id
		}
		columns = append(columns, s.boardColumnOf(title, false, cards, glyphs, all))
	}
	return columns
}

// boardColumnOf composes one column: its heading's parts, and its cards
// capped at boardCap unless all is set, with the line counting the rest.
func (s *session) boardColumnOf(title string, operator bool, cards []verb.CardView, glyphs boardGlyphs, all bool) boardColumn {
	column := boardColumn{
		title:    withoutControls(title),
		operator: operator,
		count:    s.r.T("view.columns.count", "count", strconv.Itoa(len(cards))),
	}
	shown := cards
	if !all && len(cards) > boardCap {
		shown = cards[:boardCap]
		column.more = s.r.T("view.columns.more", "count", strconv.Itoa(len(cards)-boardCap))
	}
	for _, card := range shown {
		column.cards = append(column.cards, boardCardOf(card, glyphs))
	}
	return column
}

// boardCap is how many cards a column of the board shows before the rest are
// counted on one line, unless --all lifts it.
const boardCap = 5

// boardCardOf composes one card: the glyph and colour of its state, its
// number, the operator mark where an item on it waits on the operator, the
// holder of an active card or the kind of a blocked card's block, and its
// priority. Severity is never drawn on the board.
func boardCardOf(card verb.CardView, glyphs boardGlyphs) boardCard {
	drawn := boardCard{
		glyph:    glyphs.ready,
		number:   cardNumber(card.Ref),
		mark:     card.OperatorPending > 0,
		priority: withoutControls(card.Priority),
		title:    withoutControls(card.Title),
	}
	switch card.State {
	case contract.StateActive:
		drawn.glyph, drawn.colour = glyphs.active, screen.Blue
		drawn.holder = withoutControls(card.Holder)
	case contract.StateBlocked:
		drawn.glyph, drawn.colour = glyphs.blocked, screen.Red
		drawn.holder = withoutControls(card.BlockKind)
	}
	return drawn
}

// cardNumber is a card's reference without the workbench's slug, which
// dinah show resolves on its own, as the references guide teaches. A
// reference carrying no hyphen is drawn whole.
func cardNumber(ref string) string {
	at := strings.LastIndex(ref, "-")
	if at < 0 {
		return ref
	}
	return ref[at+1:]
}

// withoutControls replaces every control character, general category Cc,
// with a space. The measure counts a control character as drawing nothing,
// so one left in would misalign the grid, and under --watch an escape in a
// title would reach a terminal the tool is driving.
func withoutControls(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Cc, r) {
			return ' '
		}
		return r
	}, text)
}
