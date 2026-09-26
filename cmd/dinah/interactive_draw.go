//go:build tui

package main

import (
	"bytes"
	"image/color"
	"sort"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"dinah/internal/bench"
	"dinah/internal/screen"
	"dinah/internal/verb"
)

// The ANSI colour numbers the state colours are drawn in: blue, red and
// yellow, which are colours 4, 1 and 3 of the eight every ANSI terminal has.
var interactiveColours = map[screen.Colour]color.Color{
	screen.Blue:   lipgloss.Color("4"),
	screen.Red:    lipgloss.Color("1"),
	screen.Yellow: lipgloss.Color("3"),
}

// interactiveFrame is the tea.View every frame is drawn as: on the
// alternate screen, with bracketed paste on, and with no mouse and no
// keyboard enhancement.
//
// Dinah turns bracketed paste on before each program starts and off after it
// ends, and its reader decodes the marks. Every frame also asks for the mode
// on, because View documents DisableBracketedPasteMode as disabling the mode
// for the view that carries it: a frame asking for it off is a frame during
// which a paste is not marked, whoever set the mode last. Bubble Tea's
// renderer starts ticking before Init returns, so a tick can draw its own
// zero View, which asks for the mode on, ahead of the first frame of the
// head's; a frame asking for it off then wrote the disable, and a head
// restarted after a lend, whose Init reads the workbench again, left paste
// unmarked for the rest of the session (dinah-623/criteria/52).
func interactiveFrame(content string) tea.View {
	view := tea.NewView(content)
	view.AltScreen = true
	view.MouseMode = tea.MouseModeNone
	view.KeyboardEnhancements = tea.KeyboardEnhancements{}
	return view
}

// View draws the screen. A panic here is kept, the program is asked to quit
// through crashMsg, and an empty frame is answered, carrying the same modes
// every frame carries so that nothing about the terminal changes on the way
// out.
func (m *interactiveModel) View() (view tea.View) {
	defer func() {
		if r := recover(); r != nil {
			m.recordCrash(r)
			if m.program != nil {
				go m.program.Send(crashMsg{})
			}
			view = interactiveFrame("")
		}
	}()
	if interactiveSeam != nil && interactiveSeam.view != nil {
		interactiveSeam.view()
	}
	content := m.content()
	if interactiveSeam != nil && interactiveSeam.frame != nil {
		interactiveSeam.frame(content)
	}
	return interactiveFrame(content)
}

// content is the whole screen's text.
func (m *interactiveModel) content() string {
	if m.width <= 0 || m.height <= 0 || m.answer == nil {
		return ""
	}
	draw := m.draw()
	if m.tooSmall() {
		size := strconv.Itoa(interactiveMinimumWidth) + "x" + strconv.Itoa(interactiveMinimumHeight)
		notice := withoutControls(m.s.r.T("interactive.too-small", "size", size))
		return interactiveCut(notice, max(draw, 1), m.glyphs.ellipsis)
	}
	rows := []string{
		m.render(interactiveHeading(m.headingLeft(), m.acting(), draw, m.glyphs.ellipsis)),
		m.render(interactiveLaneBar(m.barLanes(), m.focus, draw, m.glyphs)),
		interactiveRule(m.glyphs, draw),
	}
	for _, line := range m.body(m.bodyHeight()) {
		rows = append(rows, m.render(line))
	}
	rows = append(rows, m.messageRows()...)
	rows = append(rows, m.footerRows()...)
	return strings.Join(rows, "\n")
}

// headingLeft is the heading's left part: the workbench and the view, and the
// filter where one is set.
func (m *interactiveModel) headingLeft() string {
	workbench := withoutControls(m.l.Bench.Title)
	view := withoutControls(m.answer.View.Title)
	if m.filter != "" {
		return m.s.r.T("interactive.heading.filtered", "workbench", workbench, "view", view, "query", withoutControls(m.filter))
	}
	return m.s.r.T("interactive.heading", "workbench", workbench, "view", view)
}

// acting is the heading's right part: who is acting, and on which model where
// the caller declared one.
func (m *interactiveModel) acting() string {
	actor := withoutControls(m.req.Actor)
	if actor == "" {
		return m.s.r.T("interactive.heading.no-actor")
	}
	operator := m.l.Bench.Operator != "" && m.req.Actor == m.l.Bench.Operator
	if m.req.Provider != "" && m.req.Model != "" {
		declared := bench.TierModel{Provider: m.req.Provider, Model: m.req.Model, Server: m.req.Server}
		model := withoutControls(declared.Render())
		if operator {
			return m.s.r.T("interactive.heading.acting.operator", "actor", actor, "model", model)
		}
		return m.s.r.T("interactive.heading.acting", "actor", actor, "model", model)
	}
	if operator {
		return m.s.r.T("view.acting.operator", "actor", actor)
	}
	return m.s.r.T("view.acting", "actor", actor)
}

// barLanes is every lane as the lane bar draws it.
func (m *interactiveModel) barLanes() []interactiveLane {
	lanes := make([]interactiveLane, 0, len(m.lanes))
	for _, lane := range m.lanes {
		count := strconv.Itoa(len(lane.cards))
		if lane.column == "" && m.answer.View.Layout != bench.ViewLayoutColumns {
			label := m.s.r.T("view.section.heading", "title", lane.title, "count", count)
			lanes = append(lanes, interactiveLane{label: label})
			continue
		}
		rendered := m.s.r.T("view.columns.count", "count", count)
		lanes = append(lanes, interactiveLane{label: lane.title, operator: lane.operator, count: rendered})
	}
	return lanes
}

// body is the rows between the rule and the message area: card mode's lines,
// or the list pane, or the move menu over it, beside the detail pane at a
// window wide enough for it.
func (m *interactiveModel) body(height int) []interactiveLine {
	draw := m.draw()
	switch {
	case m.mode == modeOutput:
		return m.outputBody(height)
	case m.mode == modeItems:
		return m.itemBody(height)
	case m.mode == modeCard || (m.cardOpen && m.mode != modeMenu):
		return m.cardBody(height)
	}
	width := draw
	var detail []string
	if m.width >= interactiveWideWidth {
		width = interactiveListWidth(draw) - 1
		detail = m.detail
		if detail == nil {
			detail = []string{}
		}
	}
	var list []interactiveLine
	switch {
	case m.mode == modeMenu:
		list = m.menuLines(width, height)
	default:
		list = m.listLines(width, height)
	}
	return interactivePanes(list, detail, draw, height, m.separator(), m.glyphs.ellipsis)
}

// separator is the glyph between the two panes.
func (m *interactiveModel) separator() string {
	if m.glyphs == plainGlyphs {
		return "|"
	}
	return "│"
}

// marker is the glyph on the selected row.
func (m *interactiveModel) marker() string {
	if m.glyphs == plainGlyphs {
		return ">"
	}
	return "›"
}

// cardBody is card mode's rows: the lines of the card the viewport has
// scrolled to, each cut to the window.
func (m *interactiveModel) cardBody(height int) []interactiveLine {
	offset := min(m.viewport.YOffset(), max(len(m.cardLines)-1, 0))
	var rows []interactiveLine
	for i := offset; i < len(m.cardLines) && len(rows) < height; i++ {
		text := interactiveCut(m.cardLines[i], m.draw(), m.glyphs.ellipsis)
		rows = append(rows, interactiveLine{drawnLine: drawnLine{text: text}})
	}
	return rows
}

// listLines is the list pane's rows for the focused lane, scrolled so the
// selected row is shown.
func (m *interactiveModel) listLines(width, height int) []interactiveLine {
	lane, ok := m.focusedLane()
	if !ok {
		empty := interactiveCut(m.s.r.T("view.section.empty"), width, m.glyphs.ellipsis)
		return []interactiveLine{{drawnLine: drawnLine{text: empty}}}
	}
	if lane.refused != nil {
		var rows []interactiveLine
		for _, refused := range m.s.refusedSectionLines(*lane.refused) {
			text := interactiveCut(withoutControls(refused.text), width, m.glyphs.ellipsis)
			rows = append(rows, interactiveLine{drawnLine: drawnLine{text: text}})
		}
		return rows
	}
	if len(lane.cards) == 0 {
		empty := interactiveCut(m.s.r.T("view.section.empty"), width, m.glyphs.ellipsis)
		return []interactiveLine{{drawnLine: drawnLine{text: empty}}}
	}
	first := 0
	if m.selected >= height {
		first = m.selected - height + 1
	}
	var rows []interactiveLine
	for i := first; i < len(lane.cards) && len(rows) < height; i++ {
		card := boardCardOf(lane.cards[i], m.glyphs)
		rows = append(rows, interactiveListRow(card, i == m.selected, width, m.glyphs, m.marker()))
	}
	return rows
}

// menuLines is the open menu's rows: its title, then one numbered row per
// value, the move menu's rows marked as on the route or the reject where
// they are.
func (m *interactiveModel) menuLines(width, height int) []interactiveLine {
	menu := m.menu
	if menu == nil {
		return nil
	}
	var rows []string
	for i, row := range menu.rows {
		number := strconv.Itoa(i + 1)
		name := withoutControls(row.label)
		switch {
		case i >= 9:
			rows = append(rows, name)
		case row.route:
			rows = append(rows, m.s.r.T("interactive.menu.row.route", "number", number, "title", name))
		case row.reject:
			rows = append(rows, m.s.r.T("interactive.menu.row.reject", "number", number, "title", name))
		default:
			rows = append(rows, m.s.r.T("interactive.menu.row", "number", number, "title", name))
		}
	}
	return interactiveMenu(withoutControls(menu.title), rows, menu.highlight, width, height, m.glyphs, m.marker())
}

// itemBody is item mode's rows: its title, then one row per item carrying an
// offered act, scrolled as the list pane scrolls so the highlighted row stays
// shown.
func (m *interactiveModel) itemBody(height int) []interactiveLine {
	ref, _, _ := m.target()
	title := withoutControls(m.s.r.T("interactive.items.title", "ref", ref))
	entries := make([][]string, 0, len(m.itemRows))
	for i, item := range m.itemRows {
		entries = append(entries, []string{strconv.Itoa(i + 1), withoutControls(item.Ref), withoutControls(item.State), withoutControls(item.Text)})
	}
	return interactiveMenu(title, interactiveColumns(entries), m.itemHighlight, m.draw(), height, m.glyphs, m.marker())
}

// outputBody is output mode's rows: its title, then the transcript from the
// line it is scrolled to, each cut to the window.
func (m *interactiveModel) outputBody(height int) []interactiveLine {
	rows := []interactiveLine{{drawnLine: drawnLine{text: interactiveCut(m.outputTitle, m.draw(), m.glyphs.ellipsis)}}}
	for i := m.outputOffset; i < len(m.output) && len(rows) < height; i++ {
		text := interactiveCut(m.output[i], m.draw(), m.glyphs.ellipsis)
		rows = append(rows, interactiveLine{drawnLine: drawnLine{text: text}})
	}
	return rows
}

// messageRows is the message area, exactly messageHeight rows: the open
// prompt, or an act's answer, or the live status.
func (m *interactiveModel) messageRows() []string {
	height := m.messageHeight()
	draw := m.draw()
	var rows []string
	switch {
	case m.mode == modePrompt && m.prompt == promptText:
		label := m.promptLabel()
		rows = append(rows, interactiveCut(label, draw, m.glyphs.ellipsis))
		rows = append(rows, strings.Split(m.area.View(), "\n")...)
	case m.mode == modePrompt && m.prompt == promptStep:
		shown := append([]string{}, m.cleaned(m.message)...)
		shown = append(shown, m.promptLabel())
		rows = append(rows, interactiveMessage(shown, height-1, draw, m.glyphs.ellipsis)...)
		rows = append(rows, m.input.View())
	case m.mode == modePrompt:
		rows = append(rows, interactiveMessage(m.cleaned(m.message), height-1, draw, m.glyphs.ellipsis)...)
		rows = append(rows, m.input.View())
	default:
		rows = interactiveMessage(m.cleaned(m.messageLines()), height, draw, m.glyphs.ellipsis)
	}
	for len(rows) < height {
		rows = append(rows, "")
	}
	return rows[:height]
}

// footerText is the help model's drawing of the bindings the current mode
// allows.
func (m *interactiveModel) footerText() string {
	keys := m.keys()
	switch m.mode {
	case modeMenu:
		return m.help.ShortHelpView(keys.menuHelp())
	case modePrompt:
		return m.help.ShortHelpView(keys.promptHelp(m.prompt))
	case modeItems:
		return m.help.ShortHelpView(keys.itemHelp(m))
	case modeOutput:
		return m.help.ShortHelpView(keys.outputHelp())
	}
	if m.fullHelp {
		return m.help.FullHelpView(keys.fullHelp(m))
	}
	return m.help.ShortHelpView(keys.shortHelp(m, false))
}

// promptLabel is the label of the step prompt open, the comment's label
// where the comment key opened it.
func (m *interactiveModel) promptLabel() string {
	if step, ok := m.currentStep(); ok {
		return m.stepLabel(step.label)
	}
	ref, _, _ := m.target()
	return withoutControls(m.s.r.T("interactive.prompt.comment", "ref", ref))
}

// bindingHelp is the footer's entries for the user's own bindings the footer
// lists: each key with its label, or its template where it has none, as the
// user wrote it with every control character replaced. The label is never
// looked up in a catalog.
func (m *interactiveModel) bindingHelp() []key.Binding {
	var bindings []key.Binding
	for _, binding := range m.bindings {
		if !m.bindingListed(binding) {
			continue
		}
		label := binding.Label
		if label == "" {
			label = binding.Template
		}
		entry := key.NewBinding(key.WithKeys(binding.Key), key.WithHelp(binding.Key, withoutControls(label)))
		bindings = append(bindings, entry)
	}
	return bindings
}

// footerRows is the footer, exactly footerHeight rows, each cut to the
// window.
func (m *interactiveModel) footerRows() []string {
	height := m.footerHeight()
	var rows []string
	for _, line := range strings.Split(m.footerText(), "\n") {
		if len(rows) == height {
			break
		}
		rows = append(rows, interactiveCut(line, m.draw(), m.glyphs.ellipsis))
	}
	for len(rows) < height {
		rows = append(rows, "")
	}
	return rows
}

// render applies colour, reverse video and bold to a laid-out line with Lip
// Gloss, and applies nothing where colour is off.
func (m *interactiveModel) render(line interactiveLine) string {
	if !m.colour {
		return line.text
	}
	cuts := map[int]bool{0: true, len(line.text): true}
	for _, span := range line.spans {
		cuts[span.start], cuts[span.end] = true, true
	}
	for _, r := range append(append([]byteRange{}, line.reverse...), line.bold...) {
		cuts[r.start], cuts[r.end] = true, true
	}
	var edges []int
	for at := range cuts {
		if at >= 0 && at <= len(line.text) {
			edges = append(edges, at)
		}
	}
	sort.Ints(edges)
	var b strings.Builder
	for i := 0; i+1 < len(edges); i++ {
		start, end := edges[i], edges[i+1]
		b.WriteString(m.styleAt(line, start).Render(line.text[start:end]))
	}
	return b.String()
}

// styleAt is the style of the byte at offset in a line.
func (m *interactiveModel) styleAt(line interactiveLine, offset int) lipgloss.Style {
	style := lipgloss.NewStyle()
	for _, span := range line.spans {
		if offset >= span.start && offset < span.end {
			if colour, ok := interactiveColours[span.colour]; ok {
				style = style.Foreground(colour)
			}
		}
	}
	for _, r := range line.reverse {
		if offset >= r.start && offset < r.end {
			style = style.Reverse(true)
		}
	}
	for _, r := range line.bold {
		if offset >= r.start && offset < r.end {
			style = style.Bold(true)
		}
	}
	return style
}

// detailText is what renderDetail prints for a card, laid out at width, as
// lines. It runs renderDetail on a copy of the session whose stdout is a
// buffer, as drawnText does for a view, so the detail pane and card mode
// place exactly the lines dinah show prints.
func (s *session) detailText(detail *verb.Detail, width int) []string {
	var buffer bytes.Buffer
	frame := *s
	frame.out = &buffer
	frame.width = width
	frame.rawWidth = width
	frame.renderDetail(detail)
	return splitLines(strings.TrimSuffix(buffer.String(), "\n"))
}

// interactiveHelp builds the footer's help model: the watch's separator
// between entries, the glyph set's ellipsis, and every style replaced by a
// plain one, so nothing in the footer carries a colour of its own.
func interactiveHelp(s *session, glyphs boardGlyphs) help.Model {
	model := help.New()
	separator := s.r.T("view.watch.separator")
	model.ShortSeparator = separator
	model.FullSeparator = separator
	model.Ellipsis = glyphs.ellipsis
	plain := lipgloss.NewStyle()
	model.Styles = help.Styles{
		Ellipsis:       plain,
		ShortKey:       plain,
		ShortDesc:      plain,
		ShortSeparator: plain,
		FullKey:        plain,
		FullDesc:       plain,
		FullSeparator:  plain,
	}
	return model
}

// interactiveInput builds the jump and filter prompts' textinput: no
// character limit, no suggestions, the key map of interactiveInputKeys, and
// plain styles with a cursor that does not blink.
func interactiveInput() textinput.Model {
	input := textinput.New()
	input.CharLimit = 0
	input.ShowSuggestions = false
	input.KeyMap = interactiveInputKeys()
	plain := lipgloss.NewStyle()
	state := textinput.StyleState{Text: plain, Placeholder: plain, Suggestion: plain, Prompt: plain}
	input.SetStyles(textinput.Styles{
		Focused: state,
		Blurred: state,
		Cursor:  textinput.CursorStyle{Shape: tea.CursorBlock},
	})
	return input
}

// interactiveArea builds the comment prompt's textarea: no prompt, no line
// numbers, no character limit, the key map of interactiveAreaKeys, and plain
// styles with a cursor that does not blink.
func interactiveArea() textarea.Model {
	area := textarea.New()
	area.Prompt = ""
	area.ShowLineNumbers = false
	area.CharLimit = 0
	area.KeyMap = interactiveAreaKeys()
	plain := lipgloss.NewStyle()
	state := textarea.StyleState{
		Base:             plain,
		Text:             plain,
		LineNumber:       plain,
		CursorLineNumber: plain,
		CursorLine:       plain,
		EndOfBuffer:      plain,
		Placeholder:      plain,
		Prompt:           plain,
		Selection:        plain.Reverse(true),
	}
	area.SetStyles(textarea.Styles{
		Focused: state,
		Blurred: state,
		Cursor:  textarea.CursorStyle{Shape: tea.CursorBlock},
	})
	return area
}
