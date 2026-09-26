//go:build tui

package main

import (
	"os"
	"slices"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"dinah/internal/bench"
	"dinah/internal/completion"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// interactiveOutputLimit is the most lines a line's transcript keeps.
const interactiveOutputLimit = 10000

// lineTranscript is where both streams of a line session write: one
// transcript keeping every line in the order it was written, each cleaned of
// control characters, at most interactiveOutputLimit of them.
type lineTranscript struct {
	lines   []string
	partial strings.Builder
	dropped int
}

// Write appends what a stream wrote, splitting it into lines.
func (t *lineTranscript) Write(p []byte) (int, error) {
	for _, b := range p {
		if b == '\n' {
			t.keep(t.partial.String())
			t.partial.Reset()
			continue
		}
		t.partial.WriteByte(b)
	}
	return len(p), nil
}

// keep keeps one finished line, or counts it where the limit is reached.
func (t *lineTranscript) keep(line string) {
	if len(t.lines) >= interactiveOutputLimit {
		t.dropped++
		return
	}
	t.lines = append(t.lines, withoutControls(strings.TrimSuffix(line, "\r")))
}

// finish keeps the last line where it carried no line break.
func (t *lineTranscript) finish() {
	if t.partial.Len() > 0 {
		t.keep(t.partial.String())
		t.partial.Reset()
	}
}

// lineResult is what one line left: its transcript, its exit status, the
// command it named and the line as the title of output mode shows it.
type lineResult struct {
	// words are the words the line was given, before any alias expanded.
	words      []string
	transcript *lineTranscript
	code       int
	command    string
	title      string
	// pinned names a pinned setting the line wrote with config set, which
	// the screen keeps the start value of.
	pinned string
}

// lendRequest is a command that lends the terminal, parsed and ready to
// dispatch once the cycle that asked for it has ended.
type lendRequest struct {
	line   *session
	parsed *arguments
	result *lineResult
}

// submitLine answers Enter at the command line by the rules of the
// specification's section 3.2: an empty text closes the prompt, a text
// beginning with @ is a jump, a text the line splitter refuses is refused
// with the prompt left open, and a line whose first word is a positional
// naming neither a command nor an alias is a jump. Every other line is a
// command.
func (m *interactiveModel) submitLine(text string) tea.Cmd {
	if text == "" {
		m.closePrompt()
		return nil
	}
	if rest, jump := strings.CutPrefix(text, "@"); jump {
		m.closePrompt()
		if target := strings.TrimSpace(rest); target != "" {
			m.jumpTo(target)
		}
		return nil
	}
	words, err := verb.SplitLine(text)
	if err != nil {
		composer := *m.s
		composer.command = ""
		m.message = m.cleaned(m.errorLinesFor(err, ""))
		return nil
	}
	if len(words) > 0 && words[0] == "dinah" {
		words = words[1:]
	}
	if m.isJump(words) {
		m.closePrompt()
		m.jumpTo(text)
		return nil
	}
	m.closePrompt()
	return m.runLine(words, text)
}

// isJump reports whether a split line is a jump: its first word is a
// positional, no word stands before it, and it names neither a command nor
// an alias in the configuration the line would read.
func (m *interactiveModel) isJump(words []string) bool {
	valued := map[string]bool{}
	known := map[string]bool{}
	for _, flag := range valuedFlags {
		valued[flag] = true
		known[flag] = true
	}
	for _, flag := range markerFlags {
		known[flag] = true
	}
	first := -1
	walkFlags(words, valued, known,
		func(_ string, index int) {
			if first < 0 {
				first = index
			}
		},
		func(string, string, bool, []string) {},
		func(string) bool { return false },
	)
	if first != 0 {
		return false
	}
	name := words[0]
	if commandNames()[name] {
		return false
	}
	cfg := m.s.lineSession(nil, nil, m.draw(), nil).cfg
	_, isAlias := cfg.AliasNamed(name)
	return !isAlias
}

// runLine runs a command given as words, the steps of the specification's
// section 3.3: it builds the line session, expands an alias, parses, refuses
// a session flag the line may not give, looks the command up, refuses an
// absent command or a flag that waits, and dispatches the line through the
// CLI's own dispatch in this process. typed is the text a person typed, empty
// for a binding or a read key. Every command the head runs by words passes
// through here.
func (m *interactiveModel) runLine(words []string, typed string) tea.Cmd {
	transcript := &lineTranscript{}
	title := typed
	if title == "" {
		title = strings.Join(words, " ")
	}
	result := &lineResult{words: words, transcript: transcript, title: withoutControls(title)}
	line := m.s.lineSession(transcript, transcript, m.draw(), words)
	parsed, stop := m.prepareLine(line, words, result)
	if stop {
		transcript.finish()
		m.afterLine(result)
		return nil
	}
	if parsed == nil {
		transcript.finish()
		m.afterLine(result)
		return nil
	}
	if c, ok := lookup(at(parsed.positional, 0)); ok && c.lendsTerminal && !parsed.has("help") && !parsed.has("version") {
		m.lend = &lendRequest{line: line, parsed: parsed, result: result}
		m.quitting = true
		return tea.Quit
	}
	result.code = line.dispatch(parsed)
	transcript.finish()
	result.pinned = pinnedWrite(parsed, result.code)
	m.afterLine(result)
	return nil
}

// prepareLine runs the steps of a line before its dispatch, writing any
// refusal to the line's own transcript exactly as run writes it, and answers
// the parsed line, or true where a step stopped it.
func (m *interactiveModel) prepareLine(line *session, words []string, result *lineResult) (*arguments, bool) {
	expanded, expansionErr := expandAlias(words, line.cfg)
	valued := map[string]bool{}
	for _, flag := range valuedFlags {
		valued[flag] = true
	}
	parsed, parseErr := parseArgs(expanded, valued)
	if parseErr != nil && expansionErr == nil {
		result.code = line.reportError(parseErr)
		return nil, true
	}
	formatFlag := parsed.value("format")
	if refusal, ok := expansionErr.(*contract.Refusal); ok && refusal.Name == contract.AliasMissing {
		if missing := refusal.Extra["argument"]; missing != "" && strings.Contains(formatFlag, missing) {
			formatFlag = ""
		}
	}
	format, formatRefusal := resolveFormat(parsed.has("json"), formatFlag, os.Getenv("DINAH_FORMAT"))
	if formatRefusal != nil {
		result.code = line.reportError(formatRefusal)
		return nil, true
	}
	line.format = format
	line.quiet = parsed.has("quiet")
	if expansionErr != nil {
		result.code = line.reportError(expansionErr)
		return nil, true
	}
	for _, name := range sortedNames(sessionFlagNames) {
		if !terminalSessionFlags[name] && parsed.has(name) {
			result.code = line.fail(contract.Usage, "--"+name)
			return nil, true
		}
	}
	name := at(parsed.positional, 0)
	if name == "" {
		return parsed, false
	}
	c, known := lookup(name)
	if !known {
		if parsed.has("help") || parsed.has("version") {
			return parsed, false
		}
		result.code = line.fail(contract.UnknownVerb, name)
		return nil, true
	}
	result.command = c.name
	line.command = c.name
	if c.terminal == terminalAbsent {
		result.code = line.reportError(contract.RefuseWith(contract.NotInTUI, c.name, map[string]string{"reason": c.name}))
		return nil, true
	}
	for _, flag := range sortedFlagNames(terminalRefusedFlags[c.name]) {
		if parsed.has(flag) {
			token := terminalRefusedFlags[c.name][flag]
			result.code = line.reportError(contract.RefuseWith(contract.NotInTUI, "--"+flag, map[string]string{"reason": token}))
			return nil, true
		}
	}
	line.command = ""
	if interactiveSeam != nil && interactiveSeam.lineDispatch != nil && interactiveSeam.lineDispatch(c.name) {
		return nil, true
	}
	return parsed, false
}

// sortedFlagNames are a flag table's names in byte order, so a line giving
// two refused flags is refused naming the same one every time.
func sortedFlagNames(flags map[string]string) []string {
	names := make([]string, 0, len(flags))
	for name := range flags {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// pinnedWrite names the pinned setting a line wrote with config set, empty
// where it wrote none.
func pinnedWrite(parsed *arguments, code int) string {
	words := parsed.rest()
	if code != 0 || at(parsed.positional, 0) != "config" || at(words, 0) != "set" {
		return ""
	}
	key := at(words, 1)
	if slices.Contains(terminalPinnedSettings, key) {
		return key
	}
	return ""
}

// afterLine shows what a line left and reads the workbench again: the pinned
// workbench is opened afresh, the key bindings are read again, and the view
// is reread, so a line that changed the flow is drawn at once. A transcript
// with nothing in it names the command and its status in the message area, a
// short one is shown there, and any other opens output mode.
func (m *interactiveModel) afterLine(result *lineResult) {
	if reopened, err := m.s.open(); err == nil {
		m.l = reopened
	}
	m.loadBindings(false)
	m.message = nil
	m.reread()
	// A notice the read left, such as item mode closing because the line
	// settled its last item, is kept beneath what the line showed.
	notice := m.message
	defer func() { m.message = append(m.message, notice...) }()
	lines := result.transcript.lines
	if result.pinned != "" {
		lines = append(lines, withoutControls(m.s.r.T("interactive.line.pinned", "key", result.pinned)))
	}
	if result.transcript.dropped > 0 {
		lines = append(lines, withoutControls(m.s.r.T("interactive.output.cut", "count", strconv.Itoa(result.transcript.dropped))))
	}
	command := result.command
	if command == "" {
		command = result.title
	}
	switch {
	case len(lines) == 0 && result.code == 0:
		m.message = []string{withoutControls(m.s.r.T("interactive.line.done", "command", command))}
	case len(lines) == 0:
		m.message = []string{withoutControls(m.s.r.T("interactive.line.exit", "command", command, "code", strconv.Itoa(result.code)))}
	case m.fitsMessage(lines):
		m.message = lines
	default:
		m.openOutput(lines, result.title)
	}
	if interactiveSeam != nil && interactiveSeam.lineDone != nil {
		interactiveSeam.lineDone(result)
	}
}

// fitsMessage reports whether a transcript fits the message area: at most
// interactiveMessageLimit lines, none wider than the draw width.
func (m *interactiveModel) fitsMessage(lines []string) bool {
	if len(lines) > interactiveMessageLimit {
		return false
	}
	for _, line := range lines {
		if displayWidth(line) > m.draw() {
			return false
		}
	}
	return true
}

// openOutput opens output mode over a transcript.
func (m *interactiveModel) openOutput(lines []string, title string) {
	m.output = lines
	m.outputTitle = withoutControls(m.s.r.T("interactive.output.title", "line", title))
	m.outputOffset = 0
	back := m.mode
	if back == modePrompt || back == modeMenu || back == modeOutput {
		back = m.returnMode()
	}
	m.outputBack = back
	m.mode = modeOutput
}

// lendTerminal runs the command a cycle ended for, on the terminal the cycle
// gave back, and leaves its transcript for the next cycle to show. The line
// session is handed the head's own terminal streams, so the editor gets the
// console as it does from a shell.
func (m *interactiveModel) lendTerminal() {
	lend := m.lend
	m.lend = nil
	m.quitting = false
	line := lend.line
	line.rawOut, line.rawErr, line.in = m.s.rawOut, m.s.rawErr, m.s.in
	lend.result.code = line.dispatch(lend.parsed)
	lend.result.transcript.finish()
	m.lent = lend.result
}

// pasteLine takes a marked paste into the command line: one line break at
// its very end is removed, and a paste still holding a line break is
// discarded whole, the prompt keeping its text, because the command line runs
// one line at a time. A paste is never carried out by its own line break.
func (m *interactiveModel) pasteLine(content string) tea.Cmd {
	for _, ending := range []string{"\r\n", "\n", "\r"} {
		if trimmed, ok := strings.CutSuffix(content, ending); ok {
			content = trimmed
			break
		}
	}
	if strings.ContainsAny(content, "\r\n") {
		m.message = []string{withoutControls(m.s.r.T("interactive.line.paste"))}
		return nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(tea.PasteMsg{Content: content})
	return cmd
}

// complete answers Tab at the command line through the shell completion
// engine, over the head's pinned workbench and as the head's identity. One
// candidate replaces the current word, several insert their longest common
// prefix, and a second Tab in a row lists them in the message area. Tab
// with the cursor anywhere but the end of the text does nothing.
func (m *interactiveModel) complete(tabbed bool) {
	text := m.input.Value()
	if m.input.Position() != len([]rune(text)) {
		return
	}
	words, err := verb.SplitLine(text)
	if err != nil {
		return
	}
	current := ""
	start := len(text)
	if len(words) > 0 && !strings.HasSuffix(text, " ") && !strings.HasSuffix(text, "\t") {
		current = words[len(words)-1]
		words = words[:len(words)-1]
		start = strings.LastIndexAny(text, " \t") + 1
	}
	if len(words) > 0 && words[0] == "dinah" {
		words = words[1:]
	}
	call := lineCompletion(m.s, words, current)
	call.s.width = m.draw()
	mode, candidates, err := call.answer()
	if err != nil || mode == completion.ModeFiles || mode == completion.ModeDirs || len(candidates) == 0 {
		return
	}
	if len(candidates) == 1 {
		replaced := text[:start] + candidates[0].Word
		if mode != completion.ModeNospace {
			replaced += " "
		}
		m.input.SetValue(replaced)
		m.input.CursorEnd()
		return
	}
	common := candidates[0].Word
	for _, candidate := range candidates[1:] {
		common = commonPrefix(common, candidate.Word)
	}
	if len(common) > len(current) {
		m.input.SetValue(text[:start] + common)
		m.input.CursorEnd()
		m.tabbed = true
		return
	}
	m.tabbed = true
	if !tabbed {
		return
	}
	m.message = m.candidateLines(candidates)
}

// candidateLines are the candidates the message area lists after a second
// Tab: as many as its rows hold, the last row naming how many did not fit.
func (m *interactiveModel) candidateLines(candidates []completion.Candidate) []string {
	room := interactiveMessageLimit - 1
	var lines []string
	for i, candidate := range candidates {
		if i == room-1 && len(candidates) > room {
			more := strconv.Itoa(len(candidates) - i)
			lines = append(lines, withoutControls(m.s.r.T("interactive.line.candidates", "count", more)))
			break
		}
		line := candidate.Word
		if candidate.Description != "" {
			line += "  " + candidate.Description
		}
		lines = append(lines, withoutControls(line))
	}
	return lines
}

// commonPrefix is the longest prefix two words share, byte for byte.
func commonPrefix(a, b string) string {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return a[:n]
}

// loadBindings reads the user's key bindings from the configuration a line
// session reads. At start it reports once, in the message area, every stored
// binding the head will not run: one whose template is invalid, and one on a
// key the head reads itself.
func (m *interactiveModel) loadBindings(start bool) {
	cfg := m.s.lineSession(nil, nil, m.draw(), nil).cfg
	var usable []bench.KeyBinding
	var unused []string
	for _, binding := range cfg.KeyBindings() {
		defect := binding.Defect
		if _, reserved := terminalReservedKeys[binding.Key]; reserved && defect == "" {
			defect = bench.KeyBindingReservedKey
		}
		if defect != "" {
			reason := m.s.r.T("interactive.binding.defect." + defect)
			unused = append(unused, withoutControls(m.s.r.T("interactive.binding.unused", "key", binding.Key, "reason", reason)))
			continue
		}
		usable = append(usable, binding)
	}
	m.bindings = usable
	if start && len(unused) > 0 {
		m.message = unused
	}
}

// bindingValues are the values a binding's placeholders take: the target
// card's reference, the column of the target card or of the focused lane,
// written as the column's slug where it has one and as its identifier
// otherwise, and the view the head draws. A value is empty where there is
// nothing to substitute.
func (m *interactiveModel) bindingValues() map[string]string {
	values := map[string]string{"view": m.req.View}
	ref, _, ok := m.target()
	if ok {
		values["card"] = ref
	}
	column := ""
	if card, found := m.cardInLanes(ref); ok && found {
		column = card.Column
	} else if lane, found := m.focusedLane(); found {
		column = lane.column
	}
	if declared := m.l.Bench.Column(column); declared != nil {
		values["column"] = declared.Ref()
	}
	if interactiveSeam != nil && interactiveSeam.bindingValue != nil {
		for name, value := range values {
			values[name] = interactiveSeam.bindingValue(name, value)
		}
	}
	return values
}

// substitute builds a binding's words from its template's tokens: each
// placeholder is replaced by its value in one left-to-right pass over its
// token, and a replaced value is never scanned again, so no value can become
// two arguments. It answers the placeholder whose value is empty, or the
// value that begins with - or carries a $, which the parser or the alias
// expansion would read as something other than an argument.
func substitute(tokens []string, values map[string]string) (words []string, missing, refused string) {
	for _, token := range tokens {
		var b strings.Builder
		for i := 0; i < len(token); i++ {
			if token[i] != '$' {
				b.WriteByte(token[i])
				continue
			}
			name := bench.PlaceholderAt(token, i)
			value := values[name]
			if value == "" {
				return nil, name, ""
			}
			if strings.HasPrefix(value, "-") || strings.Contains(value, "$") {
				return nil, "", value
			}
			b.WriteString(value)
			i += len(name)
		}
		words = append(words, b.String())
	}
	return words, "", ""
}

// bindingNamed answers the user's binding on a key.
func (m *interactiveModel) bindingNamed(pressed string) (bench.KeyBinding, bool) {
	for _, binding := range m.bindings {
		if binding.Key == pressed {
			return binding, true
		}
	}
	return bench.KeyBinding{}, false
}

// bindingKey runs the user's binding on a key pressed in browse or card
// mode. A binding needing a value nothing supplies shows which, and runs
// nothing; a value that would not stay one argument is refused as
// dinah.usage; and a binding whose command the offer judges refused writes
// nothing.
func (m *interactiveModel) bindingKey(pressed tea.KeyPressMsg) tea.Cmd {
	binding, ok := m.bindingNamed(pressed.Text)
	if !ok {
		return nil
	}
	words, missing, refused := substitute(binding.Tokens, m.bindingValues())
	if missing != "" {
		what := m.s.r.T("interactive.binding.what." + missing)
		m.message = []string{withoutControls(m.s.r.T("interactive.binding.needs", "key", binding.Key, "what", what))}
		return nil
	}
	if refused != "" {
		m.message = m.cleaned(m.errorLinesFor(contract.Refuse(contract.Usage, refused), ""))
		return nil
	}
	if judged, accepted := m.offerJudges(words); judged && !accepted {
		return nil
	}
	return m.runLine(words, "")
}

// bindingListed reports whether the footer lists a binding: never while one
// of its placeholders has no value, and, for a binding whose command is a
// single card verb on the target card, only where the offer accepts it.
func (m *interactiveModel) bindingListed(binding bench.KeyBinding) bool {
	words, missing, refused := substitute(binding.Tokens, m.bindingValues())
	if missing != "" {
		return false
	}
	if refused != "" {
		return true
	}
	judged, accepted := m.offerJudges(words)
	return !judged || accepted
}

// offerJudges answers whether the offer can judge a line before it runs, and
// whether it accepts it. It can where the line, after alias expansion, is a
// single command setting actsOnCard whose card argument is the target card
// and whose offer member does not depend on an argument the offer was not
// asked about. Every other line is judged when it runs.
func (m *interactiveModel) offerJudges(words []string) (bool, bool) {
	ref, _, ok := m.target()
	if !ok {
		return false, false
	}
	cfg := m.s.lineSession(nil, nil, m.draw(), nil).cfg
	expanded, err := expandAlias(words, cfg)
	if err != nil {
		return false, false
	}
	valued := map[string]bool{}
	for _, flag := range valuedFlags {
		valued[flag] = true
	}
	parsed, err := parseArgs(expanded, valued)
	if err != nil {
		return false, false
	}
	c, known := lookup(at(parsed.positional, 0))
	if !known || !c.actsOnCard {
		return false, false
	}
	if at(parsed.rest(), 0) != ref {
		return false, false
	}
	acts := m.acts()
	argument := at(parsed.rest(), 1)
	switch c.name {
	case verb.Claim:
		return true, m.offer.claim
	case verb.Release:
		return true, m.offer.release
	case "comment":
		return true, m.offer.comment
	case verb.Block:
		return true, acts.Block
	case verb.Unblock:
		return true, acts.Unblock
	case "attach":
		return true, acts.Attach
	case "file":
		return true, acts.File
	case "archive":
		return true, acts.Archive
	case "delete":
		return true, acts.Delete
	case "edit":
		return true, acts.Edit
	case "link":
		return true, acts.LinkNew
	case verb.Move:
		for _, move := range m.offer.moves {
			if move.Ref == argument || move.Column == argument {
				return true, true
			}
		}
		return argument != "", false
	case verb.Raise:
		return argument != "", slices.Contains(acts.RaiseTiers, argument)
	case verb.Join:
		return argument != "", slices.Contains(acts.Joins, argument)
	case verb.Leave:
		return argument != "", slices.Contains(acts.Leaves, argument)
	case verb.GrantPermission:
		return argument != "", slices.Contains(acts.Grants, argument)
	case verb.RevokePermission:
		return argument != "", slices.Contains(acts.Revokes, argument)
	case "set":
		return argument != "", slices.Contains(acts.Fields, argument)
	}
	return false, false
}

// offeredItems are the target card's items that carry at least one offered
// act, in the order the card lists them.
func (m *interactiveModel) offeredItems() []verb.OfferedItem {
	var items []verb.OfferedItem
	for _, item := range m.acts().Items {
		if item.Offered() {
			items = append(items, item)
		}
	}
	return items
}

// openItems opens item mode over the target card.
func (m *interactiveModel) openItems() {
	m.itemRows = m.offeredItems()
	m.itemHighlight = 0
	m.mode = modeItems
}

// closeItems leaves item mode.
func (m *interactiveModel) closeItems() {
	m.itemRows = nil
	m.itemHighlight = 0
	m.mode = m.returnMode()
}

// refreshItems rebuilds item mode's rows from the fresh offer, keeping the
// highlight on the same item where it is still listed, and closes item mode
// with interactive.items.none where no row is left.
func (m *interactiveModel) refreshItems() {
	if m.mode != modeItems && !(m.pending != nil && m.pending.back == modeItems) {
		return
	}
	highlighted := ""
	if m.itemHighlight < len(m.itemRows) {
		highlighted = m.itemRows[m.itemHighlight].Ref
	}
	m.itemRows = m.offeredItems()
	m.itemHighlight = 0
	for i, item := range m.itemRows {
		if item.Ref == highlighted {
			m.itemHighlight = i
		}
	}
	if len(m.itemRows) > 0 {
		return
	}
	ref, _, _ := m.target()
	if m.mode == modeItems {
		m.mode = m.returnMode()
	}
	m.message = append(m.message, withoutControls(m.s.r.T("interactive.items.none", "ref", ref)))
}

// highlightedItem answers the item item mode highlights.
func (m *interactiveModel) highlightedItem() (verb.OfferedItem, bool) {
	if m.itemHighlight < 0 || m.itemHighlight >= len(m.itemRows) {
		return verb.OfferedItem{}, false
	}
	return m.itemRows[m.itemHighlight], true
}

// itemNamed answers the target card's item with a reference.
func (m *interactiveModel) itemNamed(ref any) (verb.OfferedItem, bool) {
	named, _ := ref.(string)
	for _, item := range m.acts().Items {
		if item.Ref == named {
			return item, true
		}
	}
	return verb.OfferedItem{}, false
}
