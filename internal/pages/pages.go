// Package pages renders the HTML representation the HTTP head answers a
// browser with. Every renderer takes the bytes answer.Encode produced for a
// read, the same bytes a JSON client receives, and decodes the members it
// draws into view structs of its own, so a page cannot draw anything the
// payload does not carry. The package imports no library package: not verb,
// bench, answer, mcp or httphead. It may import msg and contract.
//
// Templates are html/template, so every value is escaped for its context, and
// no template writes a value into a script element or an on* attribute,
// because the pages have neither. Void elements are written with a closing
// slash so that a scriptless test can read a page with encoding/xml.
package pages

import (
	"bytes"
	"embed"
	"encoding/json"
	"html/template"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"dinah/internal/msg"
)

//go:embed templates/*.html
var templateFiles embed.FS

// Context is what the head hands every renderer beside the route's own
// payload: the language, the request's path and page parameters, and the
// reads every page draws from.
type Context struct {
	// R is the process's resolved language.
	R *msg.Renderer
	// Lang is its tag, for <html lang>.
	Lang string
	// Path is the request's path.
	Path string
	// Query is the query with the page parameters removed.
	Query url.Values
	// Rest are the same parameters in the order they arrived, which is the
	// order a window button's URL keeps them in.
	Rest []Param
	// Windows is the window state the page parameters name, with the keys
	// naming no live card already dropped.
	Windows WindowState
	// Status is the status payload: the workbench's title, the actor and the
	// columns. It is nil when the workbench did not open.
	Status []byte
	// Tree is the default tree payload, which the sidebar draws.
	Tree []byte
	// Cursor is the changes cursor taken before any other read.
	Cursor string
	// Log is the command log, newest first.
	Log []LogEntry
	// DefaultActor is whom a click acts as.
	DefaultActor string
	// WindowCards are the open windows' reads, one per key the window state
	// keeps.
	WindowCards []WindowCard
	// Affordances is the table GET /affordances publishes, which a card's
	// forms are drawn from.
	Affordances []byte
}

// WindowCard is what one open window draws: the key the URL names it by, the
// card's show payload and its instructions payload.
type WindowCard struct {
	Key          string
	Show         []byte
	Instructions []byte
}

// LogEntry is one command log entry as the pages draw it.
type LogEntry struct {
	// Seq is the entry's number, which Run again posts.
	Seq int
	// At is when the entry was recorded.
	At time.Time
	// Outcome is ok, refused, stale, unreachable or read.
	Outcome string
	// Verb is the command the entry ran, and empty when none was built.
	Verb string
	// Card is the reference of the card the answer carried.
	Card string
	// Line is what the entry's code element holds: the derived command line,
	// or for a typed line that could not be built, "dinah " and the typed
	// text.
	Line string
	// Rerun marks an entry with a derived line, which alone draws Run again.
	Rerun bool
	// Guarded marks an entry carrying a basis, whose Run again answers stale
	// once the card has changed.
	Guarded bool
	// Sentence is the refusal's or the stale answer's sentence, rendered,
	// with a refusal's next step in it.
	Sentence string
	// Target is the path of the card or comment the answer carried.
	Target string
}

// Refusal is a refusal an error page draws: its name, detail, and the
// sentence rendered for it, with its next step in it.
type Refusal struct {
	Status       int
	Name, Detail string
	Sentence     string
}

// templates caches one parsed template set per language.
var templates sync.Map

// set returns the template set for a renderer's language.
func set(r *msg.Renderer) *template.Template {
	if cached, ok := templates.Load(r.Tag); ok {
		return cached.(*template.Template)
	}
	parsed := template.Must(template.New("pages").Funcs(template.FuncMap{
		"t": func(key string, pairs ...string) string { return r.T(key, pairs...) },
	}).ParseFS(templateFiles, "templates/*.html"))
	templates.Store(r.Tag, parsed)
	return parsed
}

// execute renders one named template.
func execute(r *msg.Renderer, name string, data any) ([]byte, error) {
	var out bytes.Buffer
	if err := set(r).ExecuteTemplate(&out, name, data); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// document is the data the page skeleton draws.
type document struct {
	Lang, Title, Workbench string
	// HasWorkbench is false on an error page for a workbench that did not
	// open, which draws the topbar and the refusal alone.
	HasWorkbench bool
	Actor        string
	Pane         string
	Cursor       string
	Filter       string
	Chip         *link
	Tabs         []tab
	Sidebar      []sidebarRow
	Back         *link
	// Kind names the template the detail pane is drawn with, and Detail is
	// that template's data.
	Kind    string
	Detail  any
	Windows []windowView
	Log     []LogEntry
}

// link is an href and its text.
type link struct {
	Href, Text string
}

// tab is one dock tab.
type tab struct {
	Key, Href, Text string
	Min, Current    bool
}

// sidebarRow is one column row of the sidebar.
type sidebarRow struct {
	Slug, Title string
	Count       int
	Active      bool
}

// page carries what a renderer adds to the common context.
type page struct {
	title        string
	pane         string
	back         *link
	activeColumn string
	filter       string
	chip         *link
	kind         string
	detail       any
}

// shell composes the document around one page's detail and renders it.
func shell(c *Context, p page) ([]byte, error) {
	doc := document{Lang: c.Lang, Actor: c.DefaultActor, Pane: p.pane, Cursor: c.Cursor, Kind: p.kind, Detail: p.detail, Back: p.back, Filter: p.filter, Chip: p.chip}
	status := decodeStatus(c.Status)
	doc.HasWorkbench = c.Status != nil
	doc.Workbench = status.Status.Workbench
	doc.Title = p.title
	if doc.Workbench != "" {
		doc.Title = c.R.T("page.title", "page", p.title, "workbench", doc.Workbench)
	}
	if doc.HasWorkbench {
		doc.Sidebar = sidebar(c, status, p.activeColumn)
		doc.Windows = windows(c)
		doc.Tabs = dock(c, doc.Windows)
		doc.Log = c.Log
		if len(doc.Log) > 20 {
			doc.Log = doc.Log[:20]
		}
	}
	return execute(c.R, "page", doc)
}

// decodeStatus reads the status payload, empty when there is none.
func decodeStatus(data []byte) statusPayload {
	var status statusPayload
	if data != nil {
		json.Unmarshal(data, &status)
	}
	return status
}

// columnOf finds the status column a tree group value or a card's column
// member names, by identifier or by slug.
func columnOf(status statusPayload, value string) (statusColumn, bool) {
	for _, column := range status.Status.Columns {
		if column.ID == value || column.Slug == value {
			return column, true
		}
	}
	return statusColumn{}, false
}

// sidebar draws one row per column group node under the default tree's root,
// in the tree's order.
func sidebar(c *Context, status statusPayload, active string) []sidebarRow {
	var tree treePayload
	json.Unmarshal(c.Tree, &tree)
	var rows []sidebarRow
	for _, node := range tree.Tree.Root.Children {
		if node.Kind != "group" || node.Axis != "column" {
			continue
		}
		column, ok := columnOf(status, node.Value)
		if !ok {
			continue
		}
		rows = append(rows, sidebarRow{Slug: column.address(), Title: column.Title, Count: node.Count, Active: active != "" && (column.ID == active || column.Slug == active)})
	}
	return rows
}

// back is the .md-back link a detail page begins with, keeping the page's
// window state.
func back(c *Context, path, fragment, text string) *link {
	return &link{Href: URLFor(path, nil, c.Windows) + fragment, Text: text}
}

// Board renders the board: the default tree drawn as lanes.
func Board(c *Context) ([]byte, error) {
	var tree treePayload
	json.Unmarshal(c.Tree, &tree)
	lanes := drawLanes(c, decodeStatus(c.Status), tree.Tree.Root.Children, &idSet{})
	return shell(c, page{title: c.R.T("page.board"), pane: "list", kind: "lanes", detail: lanes})
}

// idSet remembers the element ids a page has written, so a view drawing one
// card in two sections writes its id once.
type idSet struct {
	seen map[string]bool
}

// claim reports whether an id is still free, and takes it.
func (s *idSet) claim(id string) bool {
	if s.seen == nil {
		s.seen = map[string]bool{}
	}
	if s.seen[id] {
		return false
	}
	s.seen[id] = true
	return true
}

// lane is one column's lane.
type lane struct {
	Title, Slug, Count string
	Sections           []laneSection
}

// laneSection is one state's cards in a lane.
type laneSection struct {
	Label string
	Cards []laneCard
}

// laneCard is one card in a lane.
type laneCard struct {
	Ref, Title, ID string
	Badge          *badge
}

// badge is a PUDL badge: its class and its text.
type badge struct {
	Class, Text string
}

// stateLabel is the reader's word for a card state.
func stateLabel(c *Context, state string) string {
	if c.R.Has("page.state." + state) {
		return c.R.T("page.state." + state)
	}
	return state
}

// stateBadge is the badge a lane card carries: accent for an active card,
// danger for a blocked one, and none otherwise.
func stateBadge(c *Context, state string) *badge {
	switch state {
	case "active":
		return &badge{Class: "accent", Text: stateLabel(c, state)}
	case "blocked":
		return &badge{Class: "danger", Text: stateLabel(c, state)}
	}
	return nil
}

// drawLanes draws column group nodes as lanes, one section per state group
// below each and one lane card per card node below that.
func drawLanes(c *Context, status statusPayload, nodes []treeNode, ids *idSet) []lane {
	var lanes []lane
	for _, node := range nodes {
		if node.Kind != "group" || node.Axis != "column" {
			continue
		}
		column, _ := columnOf(status, node.Value)
		l := lane{Title: column.Title, Slug: column.address(), Count: strconv.Itoa(node.Count)}
		if l.Title == "" {
			l.Title, l.Slug = node.Value, node.Value
		}
		if column.Capacity > 0 {
			l.Count = c.R.T("page.lane.of-capacity", "count", strconv.Itoa(node.Count), "capacity", strconv.Itoa(column.Capacity))
		}
		var loose []laneCard
		for _, child := range node.Children {
			switch {
			case child.Kind == "group":
				section := laneSection{Label: stateLabel(c, child.Value)}
				for _, card := range cardsBelow(child) {
					section.Cards = append(section.Cards, laneCardOf(c, card, child.Value, ids))
				}
				if len(section.Cards) > 0 {
					l.Sections = append(l.Sections, section)
				}
			case child.Kind == "card":
				loose = append(loose, laneCardOf(c, child, "", ids))
			}
		}
		if len(loose) > 0 {
			l.Sections = append([]laneSection{{Cards: loose}}, l.Sections...)
		}
		lanes = append(lanes, l)
	}
	return lanes
}

// cardsBelow collects the card nodes at or below a node, in the tree's order.
func cardsBelow(node treeNode) []treeNode {
	if node.Kind == "card" {
		return []treeNode{node}
	}
	var cards []treeNode
	for _, child := range node.Children {
		cards = append(cards, cardsBelow(child)...)
	}
	return cards
}

// laneCardOf draws one card node, giving it its id when the page has not
// written that id yet.
func laneCardOf(c *Context, node treeNode, state string, ids *idSet) laneCard {
	card := laneCard{Ref: node.Ref, Title: node.Title, Badge: stateBadge(c, state)}
	if ids.claim("card-" + node.Ref) {
		card.ID = "card-" + node.Ref
	}
	return card
}

// columnDetail is what the column page draws.
type columnDetail struct {
	Title        string
	Badges       []badge
	Instructions string
	Lanes        []lane
}

// Column renders a column's page: its kind and flags, its own instruction
// layer, and its lane. The kind and flags are the status read's, because a
// column's show payload is its file's text and carries no capacity.
func Column(c *Context, column string, instructions []byte) ([]byte, error) {
	status := decodeStatus(c.Status)
	found, _ := columnOf(status, column)
	var served instructionsPayload
	json.Unmarshal(instructions, &served)
	detail := columnDetail{Title: found.Title, Instructions: served.Served.Instructions.Column}
	detail.Badges = append(detail.Badges, badge{Text: found.Kind})
	if found.OperatorOwned {
		detail.Badges = append(detail.Badges, badge{Class: "warn", Text: c.R.T("page.column.operator-owned")})
	}
	if found.TakesWorkUp {
		detail.Badges = append(detail.Badges, badge{Class: "pr", Text: c.R.T("page.column.takes-work-up")})
	}
	if found.Capacity > 0 {
		detail.Badges = append(detail.Badges, badge{Text: c.R.T("page.column.capacity", "capacity", strconv.Itoa(found.Capacity))})
	}
	var tree treePayload
	json.Unmarshal(c.Tree, &tree)
	for _, node := range tree.Tree.Root.Children {
		if node.Kind == "group" && node.Axis == "column" && (node.Value == found.ID || node.Value == found.Slug) {
			detail.Lanes = drawLanes(c, status, []treeNode{node}, &idSet{})
		}
	}
	return shell(c, page{
		title: found.Title, pane: "detail", activeColumn: found.ID,
		back: back(c, "/", "#col-"+found.address(), c.R.T("page.back.columns")),
		kind: "column", detail: detail,
	})
}

// Card renders a card's page: the card sheet, with the card's column as the
// active sidebar row.
func Card(c *Context, show, instructions []byte) ([]byte, error) {
	sheet, card := cardSheet(c, show, instructions, "card-page")
	status := decodeStatus(c.Status)
	column, _ := columnOf(status, card.Column)
	return shell(c, page{
		title: card.Ref + " " + card.Title, pane: "detail", activeColumn: column.ID,
		back: back(c, "/columns/"+url.PathEscape(column.address()), "#card-"+card.Ref, column.Title),
		kind: "card", detail: sheet,
	})
}

// cardRow is one row of the card list's table.
type cardRow struct {
	Ref, Title, Column, Holder string
	State                      badge
}

// cardListDetail is what the card list draws.
type cardListDetail struct {
	Query   string
	Rows    []cardRow
	Columns []statusColumn
}

// CardList renders /cards: the cards the list or query read answered, the
// query form above them and the add form below.
func CardList(c *Context, payload []byte) ([]byte, error) {
	var listed listPayload
	json.Unmarshal(payload, &listed)
	detail := cardListDetail{Query: c.Query.Get("query"), Columns: decodeStatus(c.Status).Status.Columns}
	if listed.Matches != nil {
		for _, card := range listed.Matches.Cards {
			detail.Rows = append(detail.Rows, rowOf(c, card))
		}
	}
	return shell(c, page{title: c.R.T("page.cards"), pane: "detail", back: back(c, "/", "", c.R.T("page.back.columns")), kind: "cards", detail: detail})
}

// rowOf draws one card as a table row.
func rowOf(c *Context, card cardView) cardRow {
	row := cardRow{Ref: card.Ref, Title: card.Title, Column: card.ColumnTitle, Holder: card.Holder, State: badge{Text: stateLabel(c, card.State)}}
	if b := stateBadge(c, card.State); b != nil {
		row.State = *b
	}
	return row
}

// treeDetail is what the tree page draws.
type treeDetail struct {
	Lanes    []lane
	Sections []treeSection
}

// treeSection is one group node drawn as a nested section.
type treeSection struct {
	Heading  string
	Hidden   string
	Cards    []laneCard
	Sections []treeSection
}

// TreePage renders /tree: the tree the request asked for, as lanes when its
// chain begins with column and as nested sections otherwise, with the query
// as one filter chip.
func TreePage(c *Context, payload []byte) ([]byte, error) {
	var tree treePayload
	json.Unmarshal(payload, &tree)
	detail := treeDetail{}
	ids := &idSet{}
	if len(tree.Tree.GroupBy) > 0 && tree.Tree.GroupBy[0] == "column" {
		detail.Lanes = drawLanes(c, decodeStatus(c.Status), tree.Tree.Root.Children, ids)
	} else {
		detail.Sections = treeSections(c, tree.Tree.Root.Children, ids)
	}
	p := page{title: c.R.T("page.tree"), pane: "detail", back: back(c, "/", "", c.R.T("page.back.columns")), kind: "tree", detail: detail}
	if query := c.Query.Get("query"); query != "" {
		p.filter = query
		rest := url.Values{}
		for name, values := range c.Query {
			if name != "query" {
				rest[name] = values
			}
		}
		href := "/tree"
		if encoded := rest.Encode(); encoded != "" {
			href += "?" + encoded
		}
		p.chip = &link{Href: href, Text: query}
	}
	return shell(c, p)
}

// treeSections draws group nodes as nested sections.
func treeSections(c *Context, nodes []treeNode, ids *idSet) []treeSection {
	var sections []treeSection
	for _, node := range nodes {
		if node.Kind != "group" {
			continue
		}
		section := treeSection{Heading: c.R.T("page.tree.group", "axis", node.Axis, "value", node.Value)}
		if node.Hidden != nil {
			section.Hidden = c.R.T("page.tree.hidden", "children", strconv.Itoa(node.Hidden.Children), "subjects", strconv.Itoa(node.Hidden.Subjects), "filtered", strconv.Itoa(node.Hidden.Filtered))
		}
		for _, child := range node.Children {
			if child.Kind == "card" {
				section.Cards = append(section.Cards, laneCardOf(c, child, "", ids))
			}
		}
		section.Sections = treeSections(c, node.Children, ids)
		sections = append(sections, section)
	}
	return sections
}

// searchDetail is what the search page draws.
type searchDetail struct {
	Phrase, Query string
	Hits          []searchHit
}

// Search renders /search: the search form, then the hits.
func Search(c *Context, payload []byte) ([]byte, error) {
	var found searchPayload
	json.Unmarshal(payload, &found)
	detail := searchDetail{Phrase: c.Query.Get("phrase"), Query: c.Query.Get("query"), Hits: found.Results.Hits}
	return shell(c, page{title: c.R.T("page.search"), pane: "detail", back: back(c, "/", "", c.R.T("page.back.columns")), kind: "search", detail: detail})
}

// Views renders /views: a link to each view the payload names.
func Views(c *Context, payload []byte) ([]byte, error) {
	var listed viewsPayload
	json.Unmarshal(payload, &listed)
	var links []link
	for _, v := range listed.Views {
		links = append(links, link{Href: "/views/" + url.PathEscape(v.Name), Text: v.Title})
	}
	return shell(c, page{title: c.R.T("page.views"), pane: "detail", back: back(c, "/", "", c.R.T("page.back.columns")), kind: "views", detail: links})
}

// viewDetail is what one view's page draws.
type viewDetail struct {
	Sections []viewSectionView
}

// viewSectionView is one drawn view section.
type viewSectionView struct {
	Title   string
	Lanes   []lane
	Rows    []cardRow
	Refusal *Refusal
}

// View renders one view: a section per view section, as lanes of its cards
// grouped by column in a columns layout and as a table in a list layout.
func View(c *Context, payload []byte, refusal func(name, detail string, context map[string]string) Refusal) ([]byte, error) {
	var drawn viewPayload
	json.Unmarshal(payload, &drawn)
	status := decodeStatus(c.Status)
	ids := &idSet{}
	detail := viewDetail{}
	for _, section := range drawn.View.Sections {
		sv := viewSectionView{Title: section.Title}
		switch {
		case section.Refused != "":
			r := refusal(section.Refused, section.RefusedDetail, section.RefusedContext)
			sv.Refusal = &r
		case drawn.View.Layout == "columns":
			sv.Lanes = lanesOfCards(c, status, section.Cards, ids)
		default:
			for _, card := range section.Cards {
				sv.Rows = append(sv.Rows, rowOf(c, card))
			}
		}
		detail.Sections = append(detail.Sections, sv)
	}
	return shell(c, page{title: drawn.View.Title, pane: "detail", back: back(c, "/", "", c.R.T("page.back.columns")), kind: "view", detail: detail})
}

// lanesOfCards groups cards by column, in the status read's column order.
func lanesOfCards(c *Context, status statusPayload, cards []cardView, ids *idSet) []lane {
	var lanes []lane
	for _, column := range status.Status.Columns {
		l := lane{Title: column.Title, Slug: column.address()}
		section := laneSection{}
		for _, card := range cards {
			if card.Column == column.ID {
				node := treeNode{Ref: card.Ref, Title: card.Title}
				section.Cards = append(section.Cards, laneCardOf(c, node, card.State, ids))
			}
		}
		if len(section.Cards) == 0 {
			continue
		}
		l.Count = strconv.Itoa(len(section.Cards))
		l.Sections = []laneSection{section}
		lanes = append(lanes, l)
	}
	return lanes
}

// genericDetail is what the generic page draws: the payload as the JSON
// answer carries it, and the payload's affordances.
type genericDetail struct {
	Heading string
	Payload string
	Acts    []act
}

// Generic renders any route without a designed renderer: the payload bytes
// exactly as the JSON answer carries them, under the command name and the
// reference, with the payload's affordances drawn above as a card's are.
func Generic(c *Context, command, ref string, payload []byte) ([]byte, error) {
	heading := strings.TrimSpace(command + " " + ref)
	var carried struct {
		Affordances []string `json:"affordances"`
	}
	json.Unmarshal(payload, &carried)
	detail := genericDetail{Heading: heading, Payload: string(payload), Acts: acts(c, carried.Affordances, cardView{}, nil, "")}
	p := page{title: heading, pane: "detail", back: back(c, "/", "", c.R.T("page.back.columns")), kind: "generic", detail: detail}
	return shell(c, p)
}

// ErrorPage renders a refusal as a page: the skeleton with the refusal's name
// and detail, its sentence and its next step. When the workbench did not open
// the context carries no status, and the page is the topbar and the refusal
// alone.
func ErrorPage(c *Context, refusal Refusal) ([]byte, error) {
	title := refusal.Name
	if title == "" {
		title = c.R.T("page.defect.title")
	}
	p := page{title: title, pane: "detail", kind: "error", detail: refusal}
	if c.Status != nil {
		p.back = back(c, "/", "", c.R.T("page.back.columns"))
	}
	return shell(c, p)
}

// CommandLog renders GET /commands: the whole log, newest first, under the
// typed-line form.
func CommandLog(c *Context) ([]byte, error) {
	return shell(c, page{title: c.R.T("page.log"), pane: "detail", back: back(c, "/", "", c.R.T("page.back.columns")), kind: "commands", detail: c.Log})
}
