//go:build tui

package main

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"dinah/internal/answer"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// The modes a row of interactiveActs is read in.
const (
	// actBrowse is a key read in browse mode and card mode alike.
	actBrowse = "browse"
	// actItems is a key read in item mode.
	actItems = "items"
)

// interactiveAct is one act the terminal UI performs from a key of its own.
type interactiveAct struct {
	// name is the row's own name, which actFixtures keys its fixtures by.
	name string
	// verb is the command the act performs, which the command table classes
	// terminalDirect.
	verb string
	// mode is where the key is read: browse and card, or item mode.
	mode string
	// binding picks the row's binding out of the keys built for a frame.
	binding func(*interactiveKeys) key.Binding
	// offered answers whether the footer lists the binding and the key
	// acts, from the offer the library answered.
	offered func(*interactiveModel) bool
	// steps gather the act's arguments, in order, before it runs.
	steps []interactiveStep
	// read, when set, is the command line a read key runs through runLine,
	// built from the model's state and what the steps gathered.
	read func(*interactiveModel, map[string]any) []string
	// run, when set, performs the act once the steps have gathered its
	// arguments, in place of the verb run through the library. The keys the
	// head had before this card keep their own acts this way.
	run func(*interactiveModel, map[string]any) tea.Cmd
}

// stepKind is how one step gathers its argument.
type stepKind int

const (
	// stepMenu is a numbered menu of values the library accepts.
	stepMenu stepKind = iota
	// stepLine is the single-line prompt the jump and filter use.
	stepLine
	// stepText is the multi-line prompt the comment uses.
	stepText
)

// interactiveStep is one argument an act gathers before it runs.
type interactiveStep struct {
	// param is the verb.Param.Name the gathered value is stored under.
	param string
	// kind is stepMenu, stepLine or stepText.
	kind stepKind
	// label is the catalog key of the menu's title or the prompt's label.
	label string
	// rows answers a menu's rows, each a value the library accepts.
	rows func(*interactiveModel, map[string]any) []stepRow
	// optional says an empty answer leaves the parameter out rather than
	// ending the act.
	optional bool
	// when answers whether the step is asked at all, given what the earlier
	// steps gathered.
	when func(*interactiveModel, map[string]any) bool
	// marker, when set, is the value a chosen row stores as a marker flag
	// rather than as text, which is how a confirming menu gives --yes.
	marker bool
}

// stepRow is one row of a step menu: the text drawn, already cleaned, the
// value the step stores when it is chosen, and how the move menu marks it.
type stepRow struct {
	label  string
	value  string
	route  bool
	reject bool
	// also are further arguments the row stores beside its value, such as
	// the target a removed link names beside its kind.
	also map[string]any
}

// pendingAct is an act whose steps are being gathered.
type pendingAct struct {
	// name is the row's name or menu:<verb>, which the act's answer is
	// composed for.
	name string
	// verb is the command the act performs.
	verb string
	// steps are the act's steps, and index the one being asked.
	steps []interactiveStep
	index int
	// args are the arguments gathered so far, under their parameter names.
	args map[string]any
	// card and basis are the target card and its drawn revision when the
	// act started, and basis is empty for an act that carries none.
	card, basis string
	// item is the item an item act reaches, empty for every other act.
	item string
	// back is the mode the act returns to when it ends.
	back interactiveMode
	// finish performs the act once every step has answered.
	finish func(*interactiveModel, map[string]any) tea.Cmd
}

// textStep is a stepText for param labelled by label.
func textStep(param, label string) interactiveStep {
	return interactiveStep{param: param, kind: stepText, label: label}
}

// lineStep is a stepLine for param labelled by label.
func lineStep(param, label string) interactiveStep {
	return interactiveStep{param: param, kind: stepLine, label: label}
}

// itemOffered answers whether an item act is offered on the item item mode
// highlights.
func itemOffered(pick func(verb.OfferedItem) bool) func(*interactiveModel) bool {
	return func(m *interactiveModel) bool {
		item, ok := m.highlightedItem()
		return ok && pick(item)
	}
}

// citeSteps are the steps of cite: the scheme, from a menu of the schemes
// the workbench declares or a prompt where it declares none, then the target
// and, where the scheme demands one, the observation.
var citeSteps = []interactiveStep{
	{param: "scheme", kind: stepMenu, label: "interactive.menu.scheme", rows: schemeRows,
		when: func(m *interactiveModel, _ map[string]any) bool { return len(m.l.Bench.EvidenceSchemes()) > 0 }},
	{param: "scheme", kind: stepLine, label: "interactive.prompt.cite-scheme",
		when: func(m *interactiveModel, _ map[string]any) bool { return len(m.l.Bench.EvidenceSchemes()) == 0 }},
	lineStep("target", "interactive.prompt.cite-target"),
	{param: "observed", kind: stepLine, label: "interactive.prompt.cite-observed",
		when: func(m *interactiveModel, args map[string]any) bool {
			scheme, _ := args["scheme"].(string)
			return m.l.Bench.EvidenceObservedRequired(scheme)
		}},
}

// schemeRows are the evidence schemes a citation of the highlighted item may
// name: the one the item demands first where it demands one, then the other
// schemes the workbench declares in byte order.
func schemeRows(m *interactiveModel, args map[string]any) []stepRow {
	demanded := ""
	if item, ok := m.itemNamed(args["item"]); ok {
		demanded = item.Scheme
	}
	var schemes []string
	for scheme := range m.l.Bench.EvidenceSchemes() {
		schemes = append(schemes, scheme)
	}
	slices.Sort(schemes)
	var rows []stepRow
	if demanded != "" {
		rows = append(rows, stepRow{label: withoutControls(demanded), value: demanded})
	}
	for _, scheme := range schemes {
		if scheme == demanded {
			continue
		}
		rows = append(rows, stepRow{label: withoutControls(scheme), value: scheme})
	}
	return rows
}

// moveRows are the destinations the offer answered, marked as the move menu
// has always marked them.
func moveRows(m *interactiveModel, _ map[string]any) []stepRow {
	var rows []stepRow
	for _, move := range m.offer.moves {
		rows = append(rows, stepRow{label: withoutControls(move.Title), value: move.Ref, route: move.OnRoute, reject: move.Reject})
	}
	return rows
}

// interactiveActs is every act the terminal UI performs from a key. The
// footers of browse, card and item mode list a binding by walking this table
// in its order and asking each row's offered, so no footer builder names an
// act by hand.
var interactiveActs = []interactiveAct{
	{name: "show", verb: "show", mode: actBrowse, binding: func(k *interactiveKeys) key.Binding { return k.show },
		offered: func(*interactiveModel) bool { return true }},
	{name: "claim", verb: verb.Claim, mode: actBrowse, binding: func(k *interactiveKeys) key.Binding { return k.claim },
		offered: func(m *interactiveModel) bool { return m.offer.claim },
		run:     func(m *interactiveModel, _ map[string]any) tea.Cmd { m.act(verb.Claim, nil); return nil }},
	{name: "accept", verb: verb.Move, mode: actBrowse, binding: func(k *interactiveKeys) key.Binding { return k.accept },
		offered: func(m *interactiveModel) bool { return m.offer.forward != nil && m.offer.forwardTerminal },
		run:     func(m *interactiveModel, _ map[string]any) tea.Cmd { m.act(verb.Move, m.offer.forward); return nil }},
	{name: "advance", verb: verb.Move, mode: actBrowse, binding: func(k *interactiveKeys) key.Binding { return k.advance },
		offered: func(m *interactiveModel) bool { return m.offer.forward != nil && !m.offer.forwardTerminal },
		run:     func(m *interactiveModel, _ map[string]any) tea.Cmd { m.act(verb.Move, m.offer.forward); return nil }},
	{name: "send-back", verb: verb.Move, mode: actBrowse, binding: func(k *interactiveKeys) key.Binding { return k.sendBack },
		offered: func(m *interactiveModel) bool { return m.offer.back != nil },
		run:     func(m *interactiveModel, _ map[string]any) tea.Cmd { m.act(verb.Move, m.offer.back); return nil }},
	{name: "move", verb: verb.Move, mode: actBrowse, binding: func(k *interactiveKeys) key.Binding { return k.move },
		offered: func(m *interactiveModel) bool { return len(m.offer.moves) > 0 },
		steps:   []interactiveStep{{param: "column", kind: stepMenu, label: "interactive.menu.title", rows: moveRows}},
		run: func(m *interactiveModel, args map[string]any) tea.Cmd {
			column, _ := args["column"].(string)
			for i := range m.offer.moves {
				if m.offer.moves[i].Ref == column {
					row := m.offer.moves[i]
					m.act(verb.Move, &row)
				}
			}
			return nil
		}},
	{name: "release", verb: verb.Release, mode: actBrowse, binding: func(k *interactiveKeys) key.Binding { return k.release },
		offered: func(m *interactiveModel) bool { return m.offer.release },
		run:     func(m *interactiveModel, _ map[string]any) tea.Cmd { m.act(verb.Release, nil); return nil }},
	{name: "comment", verb: "comment", mode: actBrowse, binding: func(k *interactiveKeys) key.Binding { return k.comment },
		offered: func(m *interactiveModel) bool { return m.offer.comment },
		steps:   []interactiveStep{textStep("text", "interactive.prompt.comment")}},
	{name: "items", verb: "", mode: actBrowse, binding: func(k *interactiveKeys) key.Binding { return k.items },
		offered: func(m *interactiveModel) bool { return len(m.offeredItems()) > 0 }},
	{name: "actions", verb: "", mode: actBrowse, binding: func(k *interactiveKeys) key.Binding { return k.actions },
		offered: func(m *interactiveModel) bool { return len(m.actionRows()) > 0 }},
	{name: "query", verb: "query", mode: actBrowse, binding: func(k *interactiveKeys) key.Binding { return k.filter },
		offered: func(m *interactiveModel) bool { return !m.cardOpen }},
	{name: "view", verb: "view", mode: actBrowse, binding: func(k *interactiveKeys) key.Binding { return k.jump },
		offered: func(*interactiveModel) bool { return true }},
	{name: "next", verb: "next", mode: actBrowse, binding: func(k *interactiveKeys) key.Binding { return k.next },
		offered: func(*interactiveModel) bool { return true },
		read:    func(*interactiveModel, map[string]any) []string { return []string{"next"} }},
	{name: "status", verb: "status", mode: actBrowse, binding: func(k *interactiveKeys) key.Binding { return k.status },
		offered: func(*interactiveModel) bool { return true },
		read:    func(*interactiveModel, map[string]any) []string { return []string{"status"} }},
	{name: "search", verb: "search", mode: actBrowse, binding: func(k *interactiveKeys) key.Binding { return k.search },
		offered: func(*interactiveModel) bool { return true },
		steps:   []interactiveStep{lineStep("phrase", "interactive.prompt.search")},
		read: func(_ *interactiveModel, args map[string]any) []string {
			phrase, _ := args["phrase"].(string)
			return []string{"search", phrase}
		}},
	{name: "changes", verb: "changes", mode: actBrowse, binding: func(k *interactiveKeys) key.Binding { return k.changes },
		offered: func(*interactiveModel) bool { return true },
		read: func(m *interactiveModel, _ map[string]any) []string {
			if ref, _, ok := m.target(); ok {
				return []string{"changes", "--card", ref}
			}
			return []string{"changes"}
		}},
	{name: "whoami", verb: "whoami", mode: actBrowse, binding: func(k *interactiveKeys) key.Binding { return k.whoami },
		offered: func(*interactiveModel) bool { return true },
		read:    func(*interactiveModel, map[string]any) []string { return []string{"whoami"} }},

	{name: "resolve", verb: "resolve", mode: actItems, binding: func(k *interactiveKeys) key.Binding { return k.itemResolve },
		offered: itemOffered(func(i verb.OfferedItem) bool { return i.Resolve }),
		steps:   []interactiveStep{textStep("text", "interactive.prompt.resolve")}},
	{name: "verify", verb: "verify", mode: actItems, binding: func(k *interactiveKeys) key.Binding { return k.itemVerify },
		offered: itemOffered(func(i verb.OfferedItem) bool { return i.Verify }),
		steps:   []interactiveStep{textStep("text", "interactive.prompt.verify")}},
	{name: "fail", verb: "fail", mode: actItems, binding: func(k *interactiveKeys) key.Binding { return k.itemFail },
		offered: itemOffered(func(i verb.OfferedItem) bool { return i.Fail }),
		steps:   []interactiveStep{textStep("text", "interactive.prompt.fail")}},
	{name: "waive", verb: "waive", mode: actItems, binding: func(k *interactiveKeys) key.Binding { return k.itemWaive },
		offered: itemOffered(func(i verb.OfferedItem) bool { return i.Waive }),
		steps:   []interactiveStep{textStep("text", "interactive.prompt.waive")}},
	{name: "withdraw", verb: "withdraw", mode: actItems, binding: func(k *interactiveKeys) key.Binding { return k.itemWithdraw },
		offered: itemOffered(func(i verb.OfferedItem) bool { return i.Withdraw }),
		steps:   []interactiveStep{textStep("text", "interactive.prompt.withdraw")}},
	{name: "reopen", verb: "reopen", mode: actItems, binding: func(k *interactiveKeys) key.Binding { return k.itemReopen },
		offered: itemOffered(func(i verb.OfferedItem) bool { return i.Reopen }),
		steps:   []interactiveStep{textStep("reason", "interactive.prompt.reopen")}},
	{name: "cite", verb: "cite", mode: actItems, binding: func(k *interactiveKeys) key.Binding { return k.itemCite },
		offered: itemOffered(func(i verb.OfferedItem) bool { return i.Cite }),
		steps:   citeSteps},
}

// actNamed answers the row of interactiveActs with a name.
func actNamed(name string) (interactiveAct, bool) {
	for _, row := range interactiveActs {
		if row.name == name {
			return row, true
		}
	}
	return interactiveAct{}, false
}

// startKeyAct starts the act of one key row: its steps where it has any, and
// then its run, its read or its verb.
func (m *interactiveModel) startKeyAct(row interactiveAct) tea.Cmd {
	args := map[string]any{}
	ref, basis, _ := m.target()
	pending := &pendingAct{name: row.name, verb: row.verb, steps: row.steps, args: args, card: ref, basis: basis, back: m.returnMode()}
	switch {
	case row.mode == actItems:
		item, _ := m.highlightedItem()
		pending.item = item.Ref
		pending.basis = ""
		pending.back = modeItems
		args["item"] = item.Ref
		pending.finish = func(m *interactiveModel, args map[string]any) tea.Cmd {
			m.runVerb(pending, args)
			return nil
		}
	case row.read != nil:
		pending.basis = ""
		pending.finish = func(m *interactiveModel, args map[string]any) tea.Cmd {
			return m.runLine(row.read(m, args), "")
		}
	case row.run != nil:
		pending.finish = row.run
	default:
		args["card"] = ref
		pending.finish = func(m *interactiveModel, args map[string]any) tea.Cmd {
			m.runVerb(pending, args)
			return nil
		}
	}
	return m.startPending(pending)
}

// startPending begins gathering a pending act's arguments, running it at once
// where it has no step to ask.
func (m *interactiveModel) startPending(pending *pendingAct) tea.Cmd {
	m.pending = pending
	return m.advanceStep()
}

// advanceStep asks the pending act's next step, skipping every step whose
// when answers false, and performs the act once no step is left.
func (m *interactiveModel) advanceStep() tea.Cmd {
	p := m.pending
	for p.index < len(p.steps) {
		step := p.steps[p.index]
		if step.when != nil && !step.when(m, p.args) {
			p.index++
			continue
		}
		return m.askStep(step)
	}
	m.pending = nil
	m.closeStepUI(p.back)
	return p.finish(m, p.args)
}

// askStep opens the menu or the prompt a step gathers its value through. A
// menu with no row to offer ends the act and calls nothing.
func (m *interactiveModel) askStep(step interactiveStep) tea.Cmd {
	title := m.stepLabel(step.label)
	switch step.kind {
	case stepMenu:
		rows := step.rows(m, m.pending.args)
		if len(rows) == 0 {
			m.endPending()
			return nil
		}
		m.openMenu(&interactiveMenuState{title: title, rows: rows, choose: m.chooseStepRow, refresh: m.refreshStepRows, back: m.pending.back})
		return nil
	case stepLine:
		return m.openPrompt(promptStep, "")
	}
	return m.openPrompt(promptText, "")
}

// stepLabel renders a step's catalog key with every value a label may name:
// the target card, the item, the scheme and the column gathered so far.
func (m *interactiveModel) stepLabel(label string) string {
	ref, _, _ := m.target()
	values := []string{"ref", ref}
	if m.pending != nil {
		if m.pending.card != "" {
			values[1] = m.pending.card
		}
		scheme, _ := m.pending.args["scheme"].(string)
		values = append(values, "item", m.pending.item, "scheme", scheme)
	}
	return withoutControls(m.s.r.T(label, values...))
}

// currentStep answers the step the pending act is asking.
func (m *interactiveModel) currentStep() (interactiveStep, bool) {
	if m.pending == nil || m.pending.index >= len(m.pending.steps) {
		return interactiveStep{}, false
	}
	return m.pending.steps[m.pending.index], true
}

// answerStep stores what a prompt step was answered with and asks the next
// step. A required step answered with empty or all-white-space text ends the
// act and calls nothing, and an optional one leaves its parameter out.
func (m *interactiveModel) answerStep(text string) tea.Cmd {
	step, ok := m.currentStep()
	if !ok {
		return nil
	}
	if strings.TrimSpace(text) == "" {
		if !step.optional {
			m.endPending()
			return nil
		}
	} else {
		m.pending.args[step.param] = text
	}
	m.pending.index++
	m.input.Reset()
	m.area.Reset()
	return m.advanceStep()
}

// chooseStepRow stores the chosen row of a step menu and asks the next step.
func (m *interactiveModel) chooseStepRow(row stepRow) tea.Cmd {
	step, ok := m.currentStep()
	if !ok {
		return nil
	}
	switch {
	case step.marker:
		if row.value != "" {
			m.pending.args[row.value] = true
		}
	case row.value != "":
		m.pending.args[step.param] = row.value
	case !step.optional:
		m.endPending()
		return nil
	}
	for name, value := range row.also {
		m.pending.args[name] = value
	}
	if step.param == "item" {
		m.pending.item = row.value
		m.pending.basis = ""
	}
	m.pending.index++
	return m.advanceStep()
}

// refreshStepRows recomputes the open step menu's rows after a read, and
// answers false where none is left.
func (m *interactiveModel) refreshStepRows() ([]stepRow, bool) {
	step, ok := m.currentStep()
	if !ok || step.kind != stepMenu {
		return nil, false
	}
	ref, _, found := m.target()
	if m.pending.card != "" && (!found || ref != m.pending.card) {
		return nil, false
	}
	rows := step.rows(m, m.pending.args)
	return rows, len(rows) > 0
}

// endPending ends the pending act without performing it, returning to the
// mode it started from.
func (m *interactiveModel) endPending() {
	back := modeBrowse
	if m.pending != nil {
		back = m.pending.back
	}
	m.pending = nil
	m.closeStepUI(back)
}

// closeStepUI closes whatever menu or prompt a step had open and returns to
// the mode an act started from.
func (m *interactiveModel) closeStepUI(back interactiveMode) {
	m.menu = nil
	m.input.Blur()
	m.input.Reset()
	m.area.Blur()
	m.area.Reset()
	m.mode = back
	if back == modeCard && !m.cardOpen {
		m.mode = modeBrowse
	}
	if back == modeItems && len(m.itemRows) == 0 {
		m.mode = m.returnMode()
	}
}

// identityInto puts the head's identity on a request.
func (m *interactiveModel) identityInto(req *verb.Request) {
	req.Actor = m.req.Actor
	req.Harness = m.req.Harness
	req.Provider = m.req.Provider
	req.Model = m.req.Model
	req.Server = m.req.Server
}

// runVerb performs a gathered act through the library: its request is built
// from the gathered arguments, with the head's identity and, for an act on a
// card itself, the drawn revision as its basis, and it runs as the machine
// heads run a verb. The answer is shown and the view read again.
func (m *interactiveModel) runVerb(pending *pendingAct, args map[string]any) {
	req := answer.Build(pending.verb, args)
	m.identityInto(req)
	req.Basis = pending.basis
	_, response := answer.Run(pending.verb, m.l, req)
	if response == nil {
		response = &verb.Response{Outcome: contract.OutcomeOK}
	}
	subject := pending.card
	if pending.item != "" {
		subject = pending.item
	}
	m.actedOn(response, pending.verb, subject, args)
	m.reread()
}

// actedOn shows the answer of an act the step machinery ran: the refusal the
// command that performs the act writes, or interactive.acted.<verb> naming
// what it acted on.
func (m *interactiveModel) actedOn(response *verb.Response, name, subject string, args map[string]any) {
	if response.Outcome != contract.OutcomeOK {
		composer := *m.s
		composer.command = name
		m.message = m.cleaned(composer.outcomeLines(response))
		return
	}
	values := []string{"ref", subject, "item", subject}
	switch name {
	case "add":
		if response.Card != nil {
			values = []string{"ref", response.Card.Ref}
		}
	case verb.Pull:
		if response.Card != nil {
			values = []string{"ref", response.Card.Ref, "column", withoutControls(response.Card.ColumnTitle)}
		}
	}
	m.message = []string{withoutControls(m.s.r.T("interactive.acted."+name, values...))}
}
