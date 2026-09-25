package verb

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/msg"
)

// The values a views refusal carries beside its detail. view and section are
// attached to a whole-view refusal a section met, and no catalog fragment
// interpolates them, which is the arrangement contract.ValueColumn already
// uses: a caller reads them off the JSON envelope's context map.
const (
	viewValueView    = "view"
	viewValueSection = "section"
)

// The two reasons a views layer cannot be read, as the machine tokens the
// views-unreadable refusal carries.
const (
	viewsReasonUnreadable  = "unreadable"
	viewsReasonNotAMapping = "not-a-mapping"
)

// builtinView is one view Dinah declares itself. Its title and its sections'
// titles are catalog keys, because a title is prose and is read in the
// caller's language; its queries are the literal text a view reports.
type builtinView struct {
	name     string
	titleKey string
	layout   string
	order    string
	sections []builtinSection
}

// builtinSection is one section of a built-in view.
type builtinSection struct {
	titleKey string
	query    string
	scope    string
}

// builtinViews is every view Dinah declares, in the order a listing reads them
// before sorting. A workbench or a user replaces one by declaring a view of
// the same name; nothing removes one without replacing it.
//
// mine asks the journal for its second section. A block clears the holder, so
// holder:@me state:blocked could never match anything, and a card the caller
// blocked, claimed or not, is an obstacle the caller raised and is the
// caller's to chase. The act plane has no way to ask whether the caller's
// block is the current one, so a card the caller blocked and somebody else
// blocked again after an unblock still appears.
//
// agenda ranks the cards the caller can act on by urgency. Its selection is
// a scope rather than a query, because the cards next would offer depend on
// the caller's tier, every card's route and every claim, which no query can
// say, and a person replacing the view keeps the selection by writing the
// same scope.
var builtinViews = []builtinView{
	{
		name:     "agenda",
		titleKey: "view.agenda.title",
		layout:   bench.ViewLayoutList,
		order:    bench.ViewOrderUrgency,
		sections: []builtinSection{
			{titleKey: "view.agenda.actionable", scope: bench.ViewScopeActionable},
		},
	},
	{
		name:     "mine",
		titleKey: "view.mine.title",
		layout:   bench.ViewLayoutList,
		order:    bench.ViewOrderColumn,
		sections: []builtinSection{
			{titleKey: "view.mine.claimed", query: "holder:@me"},
			{titleKey: "view.mine.blocked", query: "state:blocked actor:@me event:blocked"},
		},
	},
	{
		name:     "board",
		titleKey: "view.board.title",
		layout:   bench.ViewLayoutColumns,
		order:    bench.ViewOrderColumn,
		sections: []builtinSection{
			{titleKey: "view.board.cards", query: "state:ready,active,blocked"},
		},
	},
}

// ViewListing is what dinah view answers with no name: one row per
// declaration the caller can see, shadowed ones included.
type ViewListing struct {
	Views []ViewRow `json:"views"`
}

// ViewRow is one declaration in the listing. Title, Layout and Order are the
// effective values, except that a malformed view reports its layout and its
// order as declared, empty where absent, and its title as declared or its
// name. Malformed is the defect token, or the empty string.
type ViewRow struct {
	Name      string `json:"name"`
	Title     string `json:"title"`
	Layout    string `json:"layout"`
	Order     string `json:"order"`
	Source    string `json:"source"`
	Used      bool   `json:"used"`
	Malformed string `json:"malformed"`
}

// ViewAnswer is what dinah view answers for a view it drew.
type ViewAnswer struct {
	View ViewBody `json:"view"`
}

// ViewBody is one drawn view. Title, Layout and Order are the effective
// values, and Actor is the actor the sections were asked as, empty where none
// resolved. Explained says the draw was asked to explain its ranks, so every
// ranked card carries all eight of its terms.
//
// Collapsed is the identifiers of the columns the view collapses on this
// workbench, resolved and in flow order. It is present on every view and is
// the empty array on a list view, where collapsing has no effect. An entry the
// view declares that names no column of this workbench does not appear in it.
type ViewBody struct {
	Name      string              `json:"name"`
	Title     string              `json:"title"`
	Layout    string              `json:"layout"`
	Order     string              `json:"order"`
	Source    string              `json:"source"`
	Actor     string              `json:"actor"`
	Explained bool                `json:"explained"`
	Collapsed []string            `json:"collapsed"`
	Sections  []ViewSectionAnswer `json:"sections"`
}

// ViewSectionAnswer is one drawn section. Every member is present on every
// section and none is ever null.
//
// Query is the query exactly as declared, before @me is expanded, and empty
// on a section that selects by scope alone. Scope is the section's scope, or
// empty. Cards are the cards the section selected, in the view's order.
// Urgency maps each card reference to its rank and score on a view ordered
// by urgency, and is empty on a view ordered any other way. Items maps each card
// reference to the references of every item that witnessed its selection, in
// stored order, and is empty on a section whose query names no item field.
// The Refused members describe a section this workbench could not ask, which
// carries no cards; on every other section the four texts are empty and the
// context is an empty object. The context is the refusal's named values, the
// same map a refusal envelope publishes, and never a composed sentence.
type ViewSectionAnswer struct {
	Title          string                   `json:"title"`
	Query          string                   `json:"query"`
	Scope          string                   `json:"scope"`
	Cards          []CardView               `json:"cards"`
	Count          int                      `json:"count"`
	Items          map[string][]string      `json:"items"`
	Urgency        map[string]UrgencyAnswer `json:"urgency"`
	Refused        string                   `json:"refused"`
	RefusedDetail  string                   `json:"refused_detail"`
	RefusedField   string                   `json:"refused_field"`
	RefusedTerm    string                   `json:"refused_term"`
	RefusedContext map[string]string        `json:"refused_context"`
}

// ListViews answers dinah view with no name: every view the caller can see,
// one row per declaration, sorted by name in byte order and, within one name,
// in the order user, workbench, built in. The first row of each name is the
// one a draw of that name uses.
func (l *Library) ListViews(req *Request) (*ViewListing, error) {
	views, err := l.visibleViews(req)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(views, func(i, j int) bool { return views[i].Name < views[j].Name })
	listing := &ViewListing{Views: []ViewRow{}}
	seen := map[string]bool{}
	for _, view := range views {
		row := ViewRow{
			Name:      view.Name,
			Title:     view.EffectiveTitle(),
			Layout:    view.EffectiveLayout(),
			Order:     view.EffectiveOrder(),
			Source:    view.Source,
			Used:      !seen[view.Name],
			Malformed: view.Defect,
		}
		if view.Defect != "" {
			row.Layout, row.Order = view.Layout, view.Order
		}
		seen[view.Name] = true
		listing.Views = append(listing.Views, row)
	}
	return listing, nil
}

// DrawView answers dinah view <name>. It runs its refusals in the order the
// verb's contract fixes, and it composes the whole answer before returning
// it, so a refused view prints nothing at all. After the view is found and
// found well formed, --explain on a view that ranks nothing refuses, then a
// card that does not resolve, then a missing actor where the view ranks or
// scopes, then a dinah.urgency block that cannot be read where it ranks, and
// last, once every section is drawn, a card no section selected.
//
// Each section runs the whole query path with @me expanded to the caller. A
// section refused by a check that says this workbench's own vocabulary lacks
// a name the query uses is drawn as a refused section, because one user's
// view is read on every workbench that user opens. Every other refusal
// refuses the whole view, carrying the view's name and the section's position,
// and so does an error reading the store, which carries no fault.
func (l *Library) DrawView(req *Request) (*ViewAnswer, error) {
	views, err := l.visibleViews(req)
	if err != nil {
		return nil, err
	}
	view, found := resolveView(views, req.View)
	if !found {
		return nil, contract.RefuseWith(contract.UnknownView, req.View, map[string]string{
			"views": strings.Join(viewNames(views), ", "),
		})
	}
	if view.Defect != "" {
		return nil, contract.RefuseWith(contract.MalformedView, view.Name, map[string]string{
			"defect": view.Defect,
			"source": view.Source,
		})
	}
	order := view.EffectiveOrder()
	ranked := order == bench.ViewOrderUrgency
	if req.Explain && !ranked {
		return nil, contract.RefuseWith(contract.ViewNotRanked, view.Name, map[string]string{
			viewValueView: view.Name,
			"order":       order,
		})
	}
	var focus *bench.Card
	if req.Card != "" {
		found, err := l.Bench.ResolveCard(req.Card)
		if err != nil {
			return nil, err
		}
		focus = found.Card
	}
	scoped := viewScoped(view)
	if (ranked || scoped) && req.Actor == "" {
		var extra map[string]string
		if req.Harness != "" {
			extra = map[string]string{contract.ValueHarness: req.Harness}
		}
		return nil, contract.RefuseWith(contract.NoOwner, "", extra)
	}
	weights, defect := l.Bench.Urgency()
	if ranked && defect.Defect != "" {
		return nil, contract.RefuseWith(contract.MalformedUrgency, view.Name, map[string]string{
			"defect": defect.Defect,
			"term":   defect.Term,
			"read":   defect.Read,
		})
	}
	var draw *viewDraw
	if ranked || scoped {
		var err error
		draw, err = l.newViewDraw(req, weights)
		if err != nil {
			return nil, err
		}
	}
	me := meExpansion{enabled: true, actor: req.Actor, harness: req.Harness}
	body := ViewBody{
		Name:      view.Name,
		Title:     view.EffectiveTitle(),
		Layout:    view.EffectiveLayout(),
		Order:     order,
		Source:    view.Source,
		Actor:     req.Actor,
		Explained: req.Explain,
		Collapsed: l.collapsedColumns(view),
		Sections:  []ViewSectionAnswer{},
	}
	for i, section := range view.Sections {
		answer, err := l.drawSection(section, order, req.Actor, me, draw, req.Explain)
		if err != nil {
			err = contract.With(err, viewValueView, view.Name)
			return nil, contract.With(err, viewValueSection, strconv.Itoa(i+1))
		}
		body.Sections = append(body.Sections, *answer)
	}
	if focus != nil && !narrowTo(&body, focus.ID) {
		return nil, contract.RefuseWith(contract.CardNotInView, focus.Ref(l.Bench.Slug), map[string]string{
			viewValueView: view.Name,
		})
	}
	return &ViewAnswer{View: body}, nil
}

// collapsedColumns resolves the columns a view collapses on this workbench,
// in flow order. A list view collapses nothing. A view declaring no collapsed
// member collapses the columns whose kind is intake or done, and one declaring
// the member collapses the columns its entries resolve to by ColumnByRef's
// reading of a column reference, [] collapsing nothing. An entry naming no
// column of this workbench is dropped rather than refusing the view, for the
// reason a section naming a missing column is drawn as not asked: one user's
// view is read on every workbench that user opens.
func (l *Library) collapsedColumns(view bench.View) []string {
	collapsed := []string{}
	if view.EffectiveLayout() != bench.ViewLayoutColumns {
		return collapsed
	}
	named := map[string]bool{}
	for _, entry := range view.Collapsed {
		if column := l.Bench.ColumnByRef(entry); column != nil {
			named[column.ID] = true
		}
	}
	for _, column := range l.Bench.Columns {
		if (view.HasCollapsed && named[column.ID]) || (!view.HasCollapsed && column.CollapsedByDefault()) {
			collapsed = append(collapsed, column.ID)
		}
	}
	return collapsed
}

// viewScoped reports whether any section of a view selects by scope, which
// needs the caller's actionable set and so an actor.
func viewScoped(view bench.View) bool {
	for _, section := range view.Sections {
		if section.Scope != "" {
			return true
		}
	}
	return false
}

// narrowTo keeps one card's row in every section that holds it, with the rank
// it holds in the full section, and empties every other section. A section
// this workbench could not ask keeps its refusal. It reports whether any
// section held the card.
func narrowTo(body *ViewBody, id string) bool {
	held := false
	for i := range body.Sections {
		section := &body.Sections[i]
		if section.Refused != "" {
			continue
		}
		kept := []CardView{}
		items := map[string][]string{}
		urgency := map[string]UrgencyAnswer{}
		for _, card := range section.Cards {
			if card.ID != id {
				continue
			}
			kept = append(kept, card)
			if witnesses, ok := section.Items[card.Ref]; ok {
				items[card.Ref] = witnesses
			}
			if answer, ok := section.Urgency[card.Ref]; ok {
				urgency[card.Ref] = answer
			}
		}
		held = held || len(kept) > 0
		section.Cards, section.Count, section.Items, section.Urgency = kept, len(kept), items, urgency
	}
	return held
}

// drawSection asks one section's question and answers it, or answers the
// refusal a vocabulary check raised as a refused section.
//
// A section carrying a scope and no query selects the scope's cards, and one
// carrying both selects the cards the query matches that are also in the
// scope. draw is nil on a view that neither ranks nor scopes.
func (l *Library) drawSection(section bench.ViewSection, order, actor string, me meExpansion, draw *viewDraw, explained bool) (*ViewSectionAnswer, error) {
	answer := &ViewSectionAnswer{
		Title:          section.Heading(),
		Query:          section.Query,
		Scope:          section.Scope,
		Cards:          []CardView{},
		Items:          map[string][]string{},
		Urgency:        map[string]UrgencyAnswer{},
		RefusedContext: map[string]string{},
	}
	if strings.TrimSpace(section.Query) == "" {
		return l.fillSection(answer, nil, draw.actionableCards(), order, draw, explained)
	}
	parsed, matched, _, fault, err := l.selectionQuery(section.Query, actor, me)
	if err != nil {
		refusal, isRefusal := err.(*contract.Refusal)
		if !isRefusal || !fault.vocabulary() {
			return nil, err
		}
		answer.Refused = refusal.Name
		answer.RefusedDetail = refusal.Detail
		answer.RefusedField = fault.field
		answer.RefusedTerm = fault.term
		for name, value := range refusal.Extra {
			answer.RefusedContext[name] = value
		}
		return answer, nil
	}
	if section.Scope != "" {
		var inScope []*bench.Card
		for _, card := range matched {
			if draw.actionable[card.ID] {
				inScope = append(inScope, draw.current(card))
			}
		}
		matched = inScope
	}
	return l.fillSection(answer, parsed, matched, order, draw, explained)
}

// fillSection puts a section's selected cards in the view's order and writes
// them into its answer, with the items that witnessed each selection where a
// query named an item field, and each card's rank and score on a view ordered
// by urgency.
func (l *Library) fillSection(answer *ViewSectionAnswer, parsed *query, matched []*bench.Card, order string, draw *viewDraw, explained bool) (*ViewSectionAnswer, error) {
	var scores []urgencyScore
	if order == bench.ViewOrderUrgency {
		ranked, err := draw.rank(matched)
		if err != nil {
			return nil, err
		}
		scores = ranked
	} else {
		l.orderSection(matched, order)
	}
	for i, card := range matched {
		view, err := l.view(card)
		if err != nil {
			return nil, err
		}
		answer.Cards = append(answer.Cards, *view)
		if scores != nil {
			answer.Urgency[view.Ref] = scores[i].answer(i+1, explained)
		}
		if parsed == nil {
			continue
		}
		witnesses, err := l.itemWitnesses(parsed, card, card.Ref(l.Bench.Slug))
		if err != nil {
			return nil, err
		}
		if len(witnesses) > 0 {
			answer.Items[view.Ref] = witnesses
		}
	}
	answer.Count = len(answer.Cards)
	return answer, nil
}

// orderSection puts a section's cards in the view's order. arrival is the
// order dinah query returns. column sorts by the position of the card's
// column in the flow and by arrival within one column, and a card standing in
// a column the flow does not list sorts after every card that stands in one.
func (l *Library) orderSection(cards []*bench.Card, order string) {
	sortByArrival(cards)
	if order != bench.ViewOrderColumn {
		return
	}
	position := func(card *bench.Card) int {
		if column := l.Bench.Column(card.Column); column != nil {
			return column.Position
		}
		return len(l.Bench.Columns)
	}
	sort.SliceStable(cards, func(i, j int) bool { return position(cards[i]) < position(cards[j]) })
}

// visibleViews is every view the caller can see, in the order a name resolves
// through them: the user's, then the workbench's, then the built-ins. A layer
// that cannot be read refuses rather than reading as empty, and the user's is
// examined first.
func (l *Library) visibleViews(req *Request) ([]bench.View, error) {
	user, state := bench.LoadUserViews(l.Home)
	switch state {
	case bench.UserViewsUnreadable:
		return nil, viewsUnreadable(bench.UserViewsPath(l.Home), bench.ViewSourceUser, viewsReasonUnreadable)
	case bench.UserViewsNotAMapping:
		return nil, viewsUnreadable(bench.UserViewsPath(l.Home), bench.ViewSourceUser, viewsReasonNotAMapping)
	}
	shared, blockDefect := l.Bench.Views()
	if blockDefect {
		return nil, viewsUnreadable(filepath.Join(l.Bench.Root, bench.WorkbenchAnchor), bench.ViewSourceWorkbench, viewsReasonNotAMapping)
	}
	views := append([]bench.View{}, user...)
	views = append(views, shared...)
	return append(views, builtins(req.Lang)...), nil
}

// viewsUnreadable composes the refusal a layer that cannot be read raises.
func viewsUnreadable(path, source, reason string) error {
	return contract.RefuseWith(contract.ViewsUnreadable, path, map[string]string{
		"source": source,
		"reason": reason,
	})
}

// builtins renders the built-in views in the caller's language, which is the
// base catalog where the request names none.
func builtins(lang string) []bench.View {
	if lang == "" {
		lang = msg.Base
	}
	catalog := msg.For(lang)
	views := make([]bench.View, 0, len(builtinViews))
	for _, declared := range builtinViews {
		view := bench.View{
			Name:   declared.name,
			Title:  catalog.T(declared.titleKey),
			Layout: declared.layout,
			Order:  declared.order,
			Source: bench.ViewSourceBuiltIn,
		}
		for _, section := range declared.sections {
			view.Sections = append(view.Sections, bench.ViewSection{
				Title: catalog.T(section.titleKey),
				Query: section.query,
				Scope: section.scope,
			})
		}
		views = append(views, view)
	}
	return views
}

// resolveView is the first view of a name, in resolution order. A malformed
// view is not skipped, so a malformed user view shadows a well-formed
// workbench view rather than letting the caller's view vanish.
func resolveView(views []bench.View, name string) (bench.View, bool) {
	for _, view := range views {
		if view.Name == name {
			return view, true
		}
	}
	return bench.View{}, false
}

// viewNames is every visible view name, each once, in byte order.
func viewNames(views []bench.View) []string {
	seen := map[string]bool{}
	var names []string
	for _, view := range views {
		if seen[view.Name] {
			continue
		}
		seen[view.Name] = true
		names = append(names, view.Name)
	}
	sort.Strings(names)
	return names
}

// viewQueryRefusal is one section of a workbench view whose query this
// workbench refuses on the checks that read no card, which dinah check
// reports.
type viewQueryRefusal struct {
	view     string
	position int
	refusal  string
}

// refusedViewQueries runs checks 1 to 5 over every section of every
// well-formed workbench view, with @me left as the literal text so that the
// step that needs a caller never fires. Checks 6 to 9 read every card, and a
// bare check running them once per section would read every card that many
// times, so they are not run. A user's views are not checked, because dinah
// check describes a workbench and a user's view is not part of one.
func (l *Library) refusedViewQueries() []viewQueryRefusal {
	views, blockDefect := l.Bench.Views()
	if blockDefect {
		return nil
	}
	var found []viewQueryRefusal
	for _, view := range views {
		if view.Defect != "" {
			continue
		}
		for i, section := range view.Sections {
			if section.Query == "" {
				continue
			}
			parsed, _, err := l.parseQuery(section.Query)
			if err == nil {
				_, err = l.checkVocabularies(parsed)
			}
			refusal, isRefusal := err.(*contract.Refusal)
			if !isRefusal {
				continue
			}
			found = append(found, viewQueryRefusal{view: view.Name, position: i + 1, refusal: refusal.Name})
		}
	}
	return found
}

// viewQueryFindings turns the refused sections into check findings.
func (l *Library) viewQueryFindings() []bench.Finding {
	anchor := filepath.Join(l.Bench.Root, bench.WorkbenchAnchor)
	var findings []bench.Finding
	for _, refused := range l.refusedViewQueries() {
		findings = append(findings, bench.Finding{
			Path:     anchor,
			Key:      bench.FindingViewQueryRefused,
			Detail:   refused.view + " " + strconv.Itoa(refused.position) + " " + refused.refusal,
			Severity: bench.SeverityCleanup,
		})
	}
	return findings
}

// SectionHeading is the heading of one section of the view a request names,
// by its one-based position, and the empty string where the view or the
// section cannot be found. The cli head reads it to name the section a
// whole-view refusal came from.
func (l *Library) SectionHeading(req *Request, position int) string {
	views, err := l.visibleViews(req)
	if err != nil {
		return ""
	}
	view, found := resolveView(views, req.View)
	if !found || position < 1 || position > len(view.Sections) {
		return ""
	}
	return view.Sections[position-1].Heading()
}
