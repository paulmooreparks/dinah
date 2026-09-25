package pages

import (
	"encoding/json"
	"maps"
	"net/url"
	"slices"
	"sort"
	"strings"
)

// act is one form or link the card sheet draws for an affordance.
type act struct {
	// Name is the affordance the act is drawn for.
	Name string
	// Link is the href of an act drawn as a link, and empty for a form.
	Link string
	// Action is the form's action.
	Action string
	// Hidden are the form's hidden members, in name order.
	Hidden []member
	// Control names the control the form carries beyond its hidden members:
	// move, block, unblock, comment, or empty for none.
	Control string
	// Options are a move form's destinations.
	Options []member
	// ID prefixes the ids of the form's controls, so two windows on one page
	// write different ids.
	ID string
	// Button is the submit button's text, and Primary marks the one primary
	// button of the sheet.
	Button  string
	Primary bool
	// Note is a line drawn under the button.
	Note string
}

// member is a name and a value: a hidden input, or an option.
type member struct {
	Name, Value string
}

// sheet is what the card sheet draws.
type sheet struct {
	Ref, Title        string
	State             badge
	Chips             []string
	Acts              []act
	Rows              []member
	Body              string
	Checklist         []checklistRow
	Comments          []commentRow
	Comment           *act
	Links             []link
	Attachments       []member
	Layers            []member
	InstructionsOpen  bool
	InstructionsTitle string
}

// checklistRow is one checklist item.
type checklistRow struct {
	Kind, Text, Column, Owner string
	State                     badge
}

// commentRow is one comment of the index.
type commentRow struct {
	Href, Subject, Author, TS string
}

// basisActs are the acts whose form carries the card's revision as _basis.
var basisActs = map[string]bool{"claim": true, "pull": true, "release": true, "move": true, "block": true, "unblock": true}

// primaryActs are the acts whose button may be the sheet's primary one; the
// first of them the sheet draws is.
var primaryActs = map[string]bool{"claim": true, "pull": true, "unblock": true, "move": true}

// cardSheet draws the card sheet from a card's show payload and its
// instructions payload. prefix keeps the ids of two sheets on one page apart.
func cardSheet(c *Context, show, instructions []byte, prefix string) (sheet, cardView) {
	var shown showPayload
	json.Unmarshal(show, &shown)
	detail := detailView{}
	if shown.Detail != nil {
		detail = *shown.Detail
	}
	card := detail.Card
	var served instructionsPayload
	json.Unmarshal(instructions, &served)
	s := sheet{Ref: card.Ref, Title: card.Title, Body: detail.Body}

	s.State = badge{Text: stateLabel(c, card.State)}
	switch card.State {
	case "active":
		s.State = badge{Class: "accent", Text: c.R.T("page.state.held", "state", stateLabel(c, card.State), "holder", card.Holder)}
	case "blocked":
		s.State = badge{Class: "danger", Text: stateLabel(c, card.State)}
	}
	for _, chip := range []string{card.ColumnTitle, card.Severity, card.Priority, card.Route} {
		if chip != "" {
			s.Chips = append(s.Chips, chip)
		}
	}

	all := acts(c, shown.Affordances, card, served.Served.LegalMoves, prefix)
	for i := range all {
		if all[i].Name == "comment" {
			comment := all[i]
			s.Comment = &comment
			continue
		}
		s.Acts = append(s.Acts, all[i])
	}

	addRow := func(key, value string) {
		if value != "" {
			s.Rows = append(s.Rows, member{Name: c.R.T(key), Value: value})
		}
	}
	addRow("page.sheet.column", card.ColumnTitle)
	if card.Holder != "" {
		held := card.Holder
		if card.ClaimSince != "" {
			held = c.R.T("page.sheet.held-since", "holder", card.Holder, "since", card.ClaimSince)
		}
		if card.Expires != "" {
			held = c.R.T("page.sheet.held-until", "held", held, "expires", card.Expires)
		}
		addRow("page.sheet.holder", held)
	}
	addRow("page.sheet.block-reason", card.BlockReason)
	addRow("page.sheet.block-kind", card.BlockKind)
	addRow("page.sheet.pull-destination", titleOf(c, card.PullDestination))
	addRow("page.sheet.workstreams", strings.Join(card.Workstreams, ", "))
	for _, name := range slices.Sorted(maps.Keys(card.Fields)) {
		if card.Fields[name] != "" {
			s.Rows = append(s.Rows, member{Name: name, Value: card.Fields[name]})
		}
	}

	for _, item := range detail.Checklist {
		state := badge{Text: item.State}
		switch item.State {
		case "resolved", "verified":
			state.Class = "pr"
		case "failed":
			state.Class = "danger"
		}
		s.Checklist = append(s.Checklist, checklistRow{Kind: item.Kind, Text: item.Text, Column: item.ColumnTitle, Owner: item.Owner, State: state})
	}
	for _, comment := range detail.Comments {
		s.Comments = append(s.Comments, commentRow{Href: "/cards/" + escapeRef(comment.Ref), Subject: comment.Subject, Author: comment.Author, TS: comment.TS})
	}
	for _, l := range detail.Links {
		s.Links = append(s.Links, link{Href: "/cards/" + escapeRef(l.Ref), Text: l.Kind + " " + l.Ref})
	}
	for _, a := range detail.Attachments {
		s.Attachments = append(s.Attachments, member{Name: a.Filename, Value: a.Description})
	}

	layers := served.Served.Instructions
	for _, layer := range []member{{"page.layer.global", layers.Global}, {"page.layer.standing", layers.Standing}, {"page.layer.column", layers.Column}} {
		if layer.Value != "" {
			s.Layers = append(s.Layers, member{Name: c.R.T(layer.Name), Value: layer.Value})
		}
	}
	s.InstructionsTitle = c.R.T("page.sheet.instructions", "column", card.ColumnTitle)
	if len(c.Log) > 0 {
		newest := c.Log[0]
		s.InstructionsOpen = newest.Outcome == "ok" && (newest.Verb == "claim" || newest.Verb == "move") && newest.Card == card.Ref
	}
	return s, card
}

// titleOf is the title of the column a slug or identifier names, or the
// value itself when no column carries it.
func titleOf(c *Context, column string) string {
	if column == "" {
		return ""
	}
	if found, ok := columnOf(decodeStatus(c.Status), column); ok {
		return found.Title
	}
	return column
}

// escapeRef escapes each segment of a reference for a path.
func escapeRef(ref string) string {
	segments := strings.Split(ref, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return strings.Join(segments, "/")
}

// acts draws a form or a link for each affordance name the payload carries
// that the affordance table has a row for, and nothing for any other. The
// page adds no condition of its own: an affordance the payload does not carry
// draws nothing, and a column legal_moves does not list is no option.
func acts(c *Context, names []string, card cardView, moves []legalMove, prefix string) []act {
	var table affordanceTable
	json.Unmarshal(c.Affordances, &table)
	rows := map[string]affordanceRow{}
	for _, row := range table.Affordances {
		rows[row.Affordance] = row
	}
	cardPath := "/cards/" + escapeRef(card.Ref)
	fill := func(href string) string {
		href = strings.ReplaceAll(href, "{card}", escapeRef(card.Ref))
		return strings.ReplaceAll(href, "{path}", cardPath)
	}
	primaryDrawn := false
	var drawn []act
	for _, name := range names {
		row, known := rows[name]
		if !known {
			continue
		}
		if row.Form == nil {
			if card.Ref == "" && strings.Contains(row.Href, "{") {
				continue
			}
			href := fill(row.Href)
			if href == c.Path || strings.Contains(href, "{") {
				continue
			}
			drawn = append(drawn, act{Name: name, Link: href, Button: actLabel(c, name)})
			continue
		}
		if card.Ref == "" {
			continue
		}
		a := act{Name: name, Action: fill(row.Form.Href), ID: prefix + "-" + name, Button: actLabel(c, name)}
		for _, key := range slices.Sorted(maps.Keys(row.Form.Members)) {
			a.Hidden = append(a.Hidden, member{Name: key, Value: row.Form.Members[key]})
		}
		if basisActs[name] {
			a.Hidden = append(a.Hidden, member{Name: "_basis", Value: card.Revision})
		}
		switch name {
		case "pull":
			a.Hidden = append(a.Hidden, member{Name: "column", Value: card.PullDestination})
			a.Button = c.R.T("page.act.pull", "column", titleOf(c, card.PullDestination))
			a.Note = c.R.T("page.act.pull-note", "column", titleOf(c, card.PullDestination))
		case "move":
			a.Control = "move"
			a.Options = moveOptions(c, moves)
			if len(a.Options) == 0 {
				continue
			}
		case "block", "unblock", "comment":
			a.Control = name
		}
		sort.SliceStable(a.Hidden, func(i, j int) bool { return a.Hidden[i].Name < a.Hidden[j].Name })
		if primaryActs[name] && !primaryDrawn {
			a.Primary = true
			primaryDrawn = true
		}
		drawn = append(drawn, a)
	}
	return drawn
}

// actLabel is the button text of an act, from the catalog where it names
// one and the affordance's own name otherwise.
func actLabel(c *Context, name string) string {
	if c.R.Has("page.act." + name) {
		return c.R.T("page.act." + name)
	}
	return name
}

// moveOptions orders a card's legal moves: the forward ones first in flow
// order, then the backward ones, each labelled with its title and a backward
// one marked as such.
func moveOptions(c *Context, moves []legalMove) []member {
	var forward, backward []member
	for _, move := range moves {
		if move.Direction == "backward" {
			backward = append(backward, member{Name: c.R.T("page.act.back", "column", move.Title), Value: move.Ref})
			continue
		}
		forward = append(forward, member{Name: move.Title, Value: move.Ref})
	}
	return append(forward, backward...)
}

// windowView is one window as the page draws it.
type windowView struct {
	Key, Ref, Title             string
	Mode                        string
	X, Y, W, H                  string
	Placed, Active, Hidden      bool
	PageHref                    string
	MinHref, MaxHref, CloseHref string
	MaxLabel                    string
	Sheet                       sheet
}

// windows draws the open windows in stacking order.
func windows(c *Context) []windowView {
	byKey := map[string]WindowCard{}
	for _, card := range c.WindowCards {
		byKey[card.Key] = card
	}
	var views []windowView
	for _, key := range c.Windows.Stacking() {
		card, ok := byKey[key]
		if !ok {
			continue
		}
		views = append(views, windowOf(c, card, false))
	}
	return views
}

// windowOf draws one window. Standalone is the window route's answer: no
// placement, no active class, and the three state buttons pointing at the
// card's page until the script adopts the window and rewrites them.
func windowOf(c *Context, card WindowCard, standalone bool) windowView {
	s, shown := cardSheet(c, card.Show, card.Instructions, "win-"+card.Key)
	ref := shown.Ref
	if ref == "" {
		ref = card.Key
	}
	page := "/cards/" + escapeRef(ref)
	v := windowView{Key: card.Key, Ref: ref, Title: shown.Title, Mode: "floating", PageHref: page, MinHref: page, MaxHref: page, CloseHref: page, MaxLabel: c.R.T("page.window.maximize"), Sheet: s}
	if standalone {
		return v
	}
	p := c.Windows.Place[card.Key]
	v.Mode, v.Placed = p.Mode, true
	v.X, v.Y, v.W, v.H = formatNumber(p.X), formatNumber(p.Y), formatNumber(p.W), formatNumber(p.H)
	v.Active = c.Windows.Top == card.Key
	v.Hidden = c.Windows.Min[card.Key]
	v.MinHref = URLFor(c.Path, c.Rest, c.Windows.Minimized(card.Key))
	v.MaxHref = URLFor(c.Path, c.Rest, c.Windows.MaximizeToggled(card.Key))
	v.CloseHref = URLFor(c.Path, c.Rest, c.Windows.Closed(card.Key))
	if p.Mode != "floating" {
		v.MaxLabel = c.R.T("page.window.restore")
	}
	return v
}

// dock draws one tab per open key, in the order the keys were opened.
func dock(c *Context, drawn []windowView) []tab {
	titles := map[string]string{}
	for _, w := range drawn {
		titles[w.Key] = w.Ref + " " + w.Title
	}
	var tabs []tab
	for _, key := range c.Windows.Open {
		text, ok := titles[key]
		if !ok {
			continue
		}
		tabs = append(tabs, tab{Key: key, Href: URLFor(c.Path, c.Rest, c.Windows.Tabbed(key)), Text: text, Min: c.Windows.Min[key], Current: c.Windows.Top == key})
	}
	return tabs
}

// Window renders the window route's answer: one window's markup alone, with
// no document around it.
func Window(c *Context, card WindowCard) ([]byte, error) {
	return execute(c.R, "window", windowOf(c, card, true))
}
