//go:build tui

package main

import (
	"runtime"
	"slices"

	tea "charm.land/bubbletea/v2"

	"dinah/internal/bench"
	"dinah/internal/verb"
)

// itemOwnerHolder is the owner value that stamps an item for whoever holds
// the card, the one the workbench's own instructions name beside operator.
// The tool enforces nothing about it, so it is offered as a word rather than
// read from a declaration.
const itemOwnerHolder = "holder"

// actionsEntry is how the actions menu offers one card verb: whether the
// library would accept it, and the arguments it gathers when chosen.
type actionsEntry struct {
	// offered answers whether the entry is listed, from the offer the
	// library answered for the target card.
	offered func(*interactiveModel) bool
	// steps gather the verb's arguments, in order, after it is chosen.
	steps []interactiveStep
	// target names the parameter the target card is given to, empty for a
	// verb that takes no card or whose steps name what it reaches.
	target string
	// run, when set, performs the entry once its steps have answered, in
	// place of the verb run through the library.
	run func(*interactiveModel, map[string]any) tea.Cmd
}

// acts answers the offer the library answered for the target card, empty
// where none was answered.
func (m *interactiveModel) acts() *verb.OfferedActs {
	if m.offer.acts == nil {
		return &verb.OfferedActs{}
	}
	return m.offer.acts
}

// valueRows turns plain values into menu rows, each drawn cleaned.
func valueRows(values []string) []stepRow {
	rows := make([]stepRow, 0, len(values))
	for _, value := range values {
		rows = append(rows, stepRow{label: withoutControls(value), value: value})
	}
	return rows
}

// itemStep is the step of an item entry of the actions menu: a menu of the
// target card's items on which the act is offered.
func itemStep(pick func(verb.OfferedItem) bool) interactiveStep {
	return interactiveStep{param: "item", kind: stepMenu, label: "interactive.menu.item", rows: func(m *interactiveModel, _ map[string]any) []stepRow {
		var picked []verb.OfferedItem
		var entries [][]string
		for _, item := range m.acts().Items {
			if pick(item) {
				picked = append(picked, item)
				entries = append(entries, []string{withoutControls(item.Ref), withoutControls(item.State), withoutControls(item.Text)})
			}
		}
		rows := make([]stepRow, 0, len(picked))
		for i, label := range interactiveColumns(entries) {
			rows = append(rows, stepRow{label: label, value: picked[i].Ref})
		}
		return rows
	}}
}

// anyItem answers whether some item of the target card offers an act.
func anyItem(pick func(verb.OfferedItem) bool) func(*interactiveModel) bool {
	return func(m *interactiveModel) bool {
		for _, item := range m.acts().Items {
			if pick(item) {
				return true
			}
		}
		return false
	}
}

// itemEntry is the actions menu's entry for one item act: a menu of the items
// on which it is offered, then the steps item mode asks for the same act.
func itemEntry(name string, pick func(verb.OfferedItem) bool) actionsEntry {
	row, _ := actNamed(name)
	return actionsEntry{
		offered: anyItem(pick),
		steps:   append([]interactiveStep{itemStep(pick)}, row.steps...),
	}
}

// columnRows are the columns an item of the target card may name, which are
// the columns the card's route carries, in flow order.
func columnRows(m *interactiveModel, _ map[string]any) []stepRow {
	ref, _, _ := m.target()
	found, err := m.l.Bench.ResolveCard(ref)
	var rows []stepRow
	for _, column := range m.l.Bench.Columns {
		if err == nil && !m.l.Bench.RouteCarries(found.Card, column) {
			continue
		}
		rows = append(rows, stepRow{label: withoutControls(column.Title), value: column.Ref()})
	}
	return rows
}

// addColumnRows are the columns a new card may be filed into, in flow order,
// the flow's first column first, which is where a filing naming none lands.
func addColumnRows(m *interactiveModel, _ map[string]any) []stepRow {
	var rows []stepRow
	for _, column := range m.l.Bench.Columns {
		rows = append(rows, stepRow{label: withoutControls(column.Title), value: column.Ref()})
	}
	return rows
}

// closedValues answers the values set accepts for a field of the target
// card where the field has a closed set, and nil where it takes free text.
func closedValues(m *interactiveModel, field string) []string {
	switch field {
	case bench.SeverityField, bench.PriorityField, bench.TierField:
		return bench.LevelNames(m.l.Bench.Levels(field))
	case bench.RouteField:
		var routes []string
		for name := range m.l.Bench.Routes {
			routes = append(routes, name)
		}
		slices.Sort(routes)
		return routes
	}
	if declared := m.l.Bench.DeclaredFieldOf(field); declared != nil && len(declared.Values) > 0 {
		return declared.Values
	}
	return nil
}

// setValueRows are the closed values of the field the set entry chose.
func setValueRows(m *interactiveModel, args map[string]any) []stepRow {
	field, _ := args["field"].(string)
	return valueRows(closedValues(m, field))
}

// actionsMenu is the actions menu's definition of every card verb, keyed by
// verb. The menu lists the entries of the commands setting actsOnCard, in
// the command table's order, whose offered answers true.
//
// The table is filled at start rather than in its declaration because its
// item entries borrow item mode's steps from interactiveActs, whose actions
// row asks this table what the menu would list, and a declaration reaching
// its own reader would be an initialisation cycle.
var actionsMenu map[string]actionsEntry

func init() {
	actionsMenu = map[string]actionsEntry{
		"add": {
			offered: func(m *interactiveModel) bool { return m.acts().Add },
			steps: []interactiveStep{
				lineStep("title", "interactive.prompt.add-title"),
				{param: "column", kind: stepMenu, label: "interactive.menu.add-column", rows: addColumnRows},
			},
		},
		verb.Claim: {
			offered: func(m *interactiveModel) bool { return m.offer.claim },
			run:     func(m *interactiveModel, _ map[string]any) tea.Cmd { m.act(verb.Claim, nil); return nil },
		},
		verb.Move: {
			offered: func(m *interactiveModel) bool { return len(m.offer.moves) > 0 },
			steps:   []interactiveStep{{param: "column", kind: stepMenu, label: "interactive.menu.title", rows: moveRows}},
			run: func(m *interactiveModel, args map[string]any) tea.Cmd {
				row, _ := actNamed("move")
				return row.run(m, args)
			},
		},
		verb.Pull: {
			offered: func(m *interactiveModel) bool { return m.acts().Pull },
		},
		verb.Release: {
			offered: func(m *interactiveModel) bool { return m.offer.release },
			run:     func(m *interactiveModel, _ map[string]any) tea.Cmd { m.act(verb.Release, nil); return nil },
		},
		verb.Block: {
			offered: func(m *interactiveModel) bool { return m.acts().Block },
			target:  "card",
			steps: []interactiveStep{
				textStep("reason", "interactive.prompt.block"),
				{param: "kind", kind: stepLine, label: "interactive.prompt.block-kind", optional: true},
			},
		},
		verb.Unblock: {
			offered: func(m *interactiveModel) bool { return m.acts().Unblock },
			target:  "card",
			steps:   []interactiveStep{{param: "reason", kind: stepText, label: "interactive.prompt.unblock", optional: true}},
		},
		verb.Raise: {
			offered: func(m *interactiveModel) bool { return m.acts().Raise },
			target:  "card",
			steps: []interactiveStep{
				{param: "tier", kind: stepMenu, label: "interactive.menu.tier", rows: func(m *interactiveModel, _ map[string]any) []stepRow {
					return valueRows(m.acts().RaiseTiers)
				}},
				textStep("reason", "interactive.prompt.raise"),
			},
		},
		"comment": {
			offered: func(m *interactiveModel) bool { return m.offer.comment },
			target:  "card",
			steps:   []interactiveStep{textStep("text", "interactive.prompt.comment")},
		},
		"attach": {
			offered: func(m *interactiveModel) bool { return m.acts().Attach },
			target:  "ref",
			steps: []interactiveStep{
				lineStep("file", "interactive.prompt.attach-file"),
				{param: "description", kind: stepLine, label: "interactive.prompt.attach-description", optional: true},
			},
		},
		"file": {
			offered: func(m *interactiveModel) bool { return m.acts().File },
			target:  "card",
			steps: []interactiveStep{
				{param: "kind", kind: stepMenu, label: "interactive.menu.item-kind", rows: func(*interactiveModel, map[string]any) []stepRow {
					return valueRows(bench.ItemKinds)
				}},
				{param: "column", kind: stepMenu, label: "interactive.menu.item-column", rows: columnRows},
				{param: "owner", kind: stepMenu, label: "interactive.menu.item-owner", optional: true, rows: func(m *interactiveModel, _ map[string]any) []stepRow {
					return []stepRow{
						{label: itemOwnerHolder, value: itemOwnerHolder},
						{label: bench.ItemOwnerOperator, value: bench.ItemOwnerOperator},
						{label: withoutControls(m.s.r.T("interactive.menu.item-owner.none")), value: ""},
					}
				}},
				textStep("text", "interactive.prompt.file"),
			},
		},
		"cite":     itemEntry("cite", func(i verb.OfferedItem) bool { return i.Cite }),
		"resolve":  itemEntry("resolve", func(i verb.OfferedItem) bool { return i.Resolve }),
		"verify":   itemEntry("verify", func(i verb.OfferedItem) bool { return i.Verify }),
		"fail":     itemEntry("fail", func(i verb.OfferedItem) bool { return i.Fail }),
		"waive":    itemEntry("waive", func(i verb.OfferedItem) bool { return i.Waive }),
		"withdraw": itemEntry("withdraw", func(i verb.OfferedItem) bool { return i.Withdraw }),
		"reopen":   itemEntry("reopen", func(i verb.OfferedItem) bool { return i.Reopen }),
		verb.GrantPermission: {
			offered: func(m *interactiveModel) bool { return len(m.acts().Grants) > 0 },
			target:  "card",
			steps: []interactiveStep{{param: "permission", kind: stepMenu, label: "interactive.menu.permission", rows: func(m *interactiveModel, _ map[string]any) []stepRow {
				return valueRows(m.acts().Grants)
			}}},
		},
		verb.RevokePermission: {
			offered: func(m *interactiveModel) bool { return len(m.acts().Revokes) > 0 },
			target:  "card",
			steps: []interactiveStep{{param: "permission", kind: stepMenu, label: "interactive.menu.permission", rows: func(m *interactiveModel, _ map[string]any) []stepRow {
				return valueRows(m.acts().Revokes)
			}}},
		},
		"link": {
			offered: func(m *interactiveModel) bool { return m.acts().LinkNew },
			target:  "card",
			steps: []interactiveStep{
				lineStep("kind", "interactive.prompt.link-kind"),
				lineStep("to", "interactive.prompt.link-to"),
			},
		},
		"unlink": {
			offered: func(m *interactiveModel) bool { return len(m.acts().Links) > 0 },
			target:  "card",
			steps: []interactiveStep{{param: "kind", kind: stepMenu, label: "interactive.menu.link", rows: func(m *interactiveModel, _ map[string]any) []stepRow {
				links := m.acts().Links
				entries := make([][]string, 0, len(links))
				for _, link := range links {
					entries = append(entries, []string{withoutControls(link.Kind), withoutControls(link.Ref)})
				}
				rows := make([]stepRow, 0, len(links))
				for i, label := range interactiveColumns(entries) {
					rows = append(rows, stepRow{label: label, value: links[i].Kind, also: map[string]any{"to": links[i].To}})
				}
				return rows
			}}},
		},
		verb.Join: {
			offered: func(m *interactiveModel) bool { return len(m.acts().Joins) > 0 },
			target:  "card",
			steps: []interactiveStep{{param: "workstream", kind: stepMenu, label: "interactive.menu.join", rows: func(m *interactiveModel, _ map[string]any) []stepRow {
				return valueRows(m.acts().Joins)
			}}},
		},
		verb.Leave: {
			offered: func(m *interactiveModel) bool { return len(m.acts().Leaves) > 0 },
			target:  "card",
			steps: []interactiveStep{{param: "workstream", kind: stepMenu, label: "interactive.menu.leave", rows: func(m *interactiveModel, _ map[string]any) []stepRow {
				return valueRows(m.acts().Leaves)
			}}},
		},
		"archive": {
			offered: func(m *interactiveModel) bool { return m.acts().Archive },
			steps: []interactiveStep{{param: "ref", kind: stepMenu, label: "interactive.menu.archive", rows: func(m *interactiveModel, _ map[string]any) []stepRow {
				ref, _, _ := m.target()
				return []stepRow{{label: withoutControls(m.s.r.T("interactive.actions.archive")), value: ref}}
			}}},
		},
		"restore": {
			offered: func(m *interactiveModel) bool { return m.acts().Restore },
			target:  "ref",
		},
		"delete": {
			offered: func(m *interactiveModel) bool { return m.acts().Delete },
			target:  "ref",
			steps: []interactiveStep{{param: "yes", kind: stepMenu, label: "interactive.menu.delete", marker: true, rows: func(m *interactiveModel, _ map[string]any) []stepRow {
				rows := []stepRow{{label: withoutControls(m.s.r.T("interactive.actions.delete")), value: "yes"}}
				if m.acts().DeleteForce {
					rows = append(rows, stepRow{label: withoutControls(m.s.r.T("interactive.menu.delete-force")), value: "yes", also: map[string]any{"force": true}})
				}
				return rows
			}}},
		},
		"accept-divergence": {
			offered: func(m *interactiveModel) bool { return len(m.acts().Divergences) > 0 },
			steps: []interactiveStep{{param: "comment", kind: stepMenu, label: "interactive.menu.divergence", rows: func(m *interactiveModel, _ map[string]any) []stepRow {
				return valueRows(m.acts().Divergences)
			}}},
		},
		"rename": {
			offered: func(m *interactiveModel) bool { return len(m.acts().Renames) > 0 },
			steps: []interactiveStep{
				{param: "ref", kind: stepMenu, label: "interactive.menu.rename", rows: func(m *interactiveModel, _ map[string]any) []stepRow {
					return valueRows(m.acts().Renames)
				}},
				lineStep("name", "interactive.prompt.rename"),
			},
		},
		"set": {
			offered: func(m *interactiveModel) bool { return len(m.acts().Fields) > 0 },
			target:  "ref",
			steps: []interactiveStep{
				{param: "field", kind: stepMenu, label: "interactive.menu.field", rows: func(m *interactiveModel, _ map[string]any) []stepRow {
					return valueRows(m.acts().Fields)
				}},
				{param: "value", kind: stepMenu, label: "interactive.menu.value", rows: setValueRows,
					when: func(m *interactiveModel, args map[string]any) bool { return len(setValueRows(m, args)) > 0 }},
				{param: "value", kind: stepText, label: "interactive.prompt.set",
					when: func(m *interactiveModel, args map[string]any) bool { return len(setValueRows(m, args)) == 0 }},
			},
		},
		"edit": {
			offered: func(m *interactiveModel) bool { return m.offer.edit },
			run: func(m *interactiveModel, _ map[string]any) tea.Cmd {
				ref, _, _ := m.target()
				return m.runLine([]string{"edit", ref}, "")
			},
		},
	}
}

// editorResolves answers whether edit typed now would find an editor, asked
// through the same ladder runEdit climbs and over the configuration the line
// session reads. OfferActs answers whether the card has something to edit,
// which the library knows; which editor would open it is the CLI's own
// setting, read here, so the menu never offers an edit that would end the
// program only to be refused as dinah.no-editor and start a new one.
func (m *interactiveModel) editorResolves() bool {
	_, err := bench.ResolveEditor(m.s.lineSession(nil, nil, m.draw(), nil).cfg, runtime.GOOS, onPath)
	return err == nil
}

// actionRow is one row of the actions menu: the verb and its entry.
type actionRow struct {
	verb  string
	entry actionsEntry
}

// actionRows are the actions menu's rows: the commands setting actsOnCard,
// in the command table's order, whose entry the offer accepts.
func (m *interactiveModel) actionRows() []actionRow {
	var rows []actionRow
	for _, c := range commands {
		if !c.actsOnCard {
			continue
		}
		entry, ok := actionsMenu[c.name]
		if !ok || !entry.offered(m) {
			continue
		}
		rows = append(rows, actionRow{verb: c.name, entry: entry})
	}
	return rows
}

// actionMenuRows are the actions menu's rows as a menu draws them.
func (m *interactiveModel) actionMenuRows() []stepRow {
	var rows []stepRow
	for _, row := range m.actionRows() {
		label := m.s.r.T("interactive.actions."+row.verb, "column", m.pullColumnTitle())
		rows = append(rows, stepRow{label: withoutControls(label), value: row.verb})
	}
	return rows
}

// pullColumn is the column a pull from the actions menu pulls into: the
// focused lane's column.
func (m *interactiveModel) pullColumn() *bench.Column {
	lane, ok := m.focusedLane()
	if !ok || lane.column == "" {
		return nil
	}
	return m.l.Bench.Column(lane.column)
}

// pullColumnTitle is the focused lane's column's title, cleaned, empty where
// the lane is no column's.
func (m *interactiveModel) pullColumnTitle() string {
	if column := m.pullColumn(); column != nil {
		return withoutControls(column.Title)
	}
	return ""
}

// openActions opens the actions menu over the target card.
func (m *interactiveModel) openActions() tea.Cmd {
	rows := m.actionMenuRows()
	if len(rows) == 0 {
		return nil
	}
	ref, _, _ := m.target()
	title := withoutControls(m.s.r.T("interactive.actions.title", "ref", ref))
	back := m.returnMode()
	m.openMenu(&interactiveMenuState{
		title: title,
		rows:  rows,
		back:  back,
		choose: func(row stepRow) tea.Cmd {
			m.menu = nil
			m.mode = back
			return m.chooseAction(row.value, back)
		},
		refresh: func() ([]stepRow, bool) {
			rows := m.actionMenuRows()
			return rows, len(rows) > 0
		},
	})
	return nil
}

// chooseAction starts the act of one actions-menu entry.
func (m *interactiveModel) chooseAction(name string, back interactiveMode) tea.Cmd {
	entry, ok := actionsMenu[name]
	if !ok || !entry.offered(m) {
		return nil
	}
	ref, basis, _ := m.target()
	args := map[string]any{}
	if entry.target != "" {
		args[entry.target] = ref
	}
	if name == verb.Pull || name == "add" {
		basis = ""
	}
	if column := m.pullColumn(); name == verb.Pull && column != nil {
		args["column"] = column.Ref()
	}
	pending := &pendingAct{name: "menu:" + name, verb: name, steps: entry.steps, args: args, card: ref, basis: basis, back: back}
	if name == "add" || name == verb.Pull {
		pending.card = ""
	}
	pending.finish = func(m *interactiveModel, args map[string]any) tea.Cmd {
		if entry.run != nil {
			return entry.run(m, args)
		}
		m.runVerb(pending, args)
		return nil
	}
	return m.startPending(pending)
}
