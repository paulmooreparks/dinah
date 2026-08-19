package verb

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// TreeNode is one node of a projected tree. Both producers emit this shape and
// every head renders it, so a consumer learns one node and reads both trees.
type TreeNode struct {
	// Kind is what this node is: workbench, state, card, comment, item,
	// attachment, a dotted extension kind, or group. Every value but group
	// names an entity kind of the format.
	Kind string `json:"kind"`
	// ID is the entity's 12-hex identifier, absent on a group node.
	ID string `json:"id,omitempty"`
	// Ref is what a person types to reach this node on show, path, or edit.
	// It is absent on a group node, which nothing addresses.
	Ref string `json:"ref,omitempty"`
	// Title is the entity's own title, or the group's label when the axis has
	// one to give. It is absent on every group node the tool can build today,
	// since no axis yet resolves a value to a title.
	Title string `json:"title,omitempty"`
	// Axis is the field a group node grouped on, absent on an entity node.
	Axis string `json:"axis,omitempty"`
	// Value is the canonical token a group node grouped at, absent on an
	// entity node. It is the empty string on the group holding the subjects
	// that carry no value on the axis.
	Value string `json:"value,omitempty"`
	// Count is how many subjects this node accounts for, after the filter and
	// before any depth truncation. Which subjects those are depends on the
	// producer: under the grouped producer they are the cards at or below the
	// node, a card node being one of its own, and under the containment
	// producer they are the entities strictly below the node, so a leaf
	// counts zero.
	Count int `json:"count"`
	// Hidden is what this node does not show, absent when it hides nothing.
	Hidden *Hidden `json:"hidden,omitempty"`
	// Children are the nodes below, absent on a leaf and on a node whose
	// children the depth cut off.
	Children []TreeNode `json:"children,omitempty"`
}

// Hidden is the account a node gives of what it is not showing. A node that
// hides nothing carries no Hidden at all rather than a Hidden of zeroes.
type Hidden struct {
	// Reason names why, in the fixed order depth, filter. Both may appear.
	Reason []string `json:"reason"`
	// Children is how many of this node's own direct children the depth cut
	// off, zero when it cut off none of them.
	Children int `json:"children"`
	// Subjects is how many surviving subjects those cut-off children hold,
	// zero when the depth cut off none of them.
	Subjects int `json:"subjects"`
	// Filtered is how many subjects at or below this node the filter removed,
	// absent when the filter removed none.
	Filtered int `json:"filtered,omitempty"`
}

// Tree is one whole projection, carrying what the reader has to know in order
// to read the counts.
type Tree struct {
	// Producer is grouped or containment.
	Producer string `json:"producer"`
	// Subject is what Count counts: card under the grouped producer, entity
	// under the containment producer.
	Subject string `json:"subject"`
	// GroupBy is the chain the grouped producer nested along, absent on a
	// containment tree.
	GroupBy []string `json:"group_by,omitempty"`
	// Depth is the named level the projection stopped at.
	Depth string `json:"depth"`
	// Root is the tree. It is the workbench under tree and the entity the
	// caller named under contents, and it is always present, including at
	// depth root where it carries no children.
	Root TreeNode `json:"root"`
}

// The two producers, as Tree.Producer names them.
const (
	ProducerGrouped     = "grouped"
	ProducerContainment = "containment"
)

// The two subjects, as Tree.Subject names them. They say what Count counts,
// and with it whether a node is one of its own subjects.
const (
	SubjectCard   = "card"
	SubjectEntity = "entity"
)

// NodeGroup is the one node kind that names no entity: a set of subjects
// sharing a value on one axis.
const NodeGroup = "group"

// The depth levels. Both ladders share root and cards, and every other level
// belongs to one producer alone.
const (
	LevelRoot     = "root"
	LevelGroups   = "groups"
	LevelCards    = "cards"
	LevelEntities = "entities"
	LevelAll      = "all"
)

// The two reasons a node hides something, in the order Hidden.Reason lists
// them when both apply.
const (
	ReasonDepth  = "depth"
	ReasonFilter = "filter"
)

// TreeLevels is the depth ladder of tree, in the order a refusal lists it.
var TreeLevels = []string{LevelRoot, LevelGroups, LevelCards}

// ContentsLevels is the depth ladder of contents, in the order a refusal lists
// it.
var ContentsLevels = []string{LevelRoot, LevelCards, LevelEntities, LevelAll}

// MaxChain is how many axes one group-by chain may nest along. A five-level
// nesting of a local workbench's cards is unreadable before it is useful, and
// the fan-out rule makes a long chain of multi-valued axes expensive as well.
const MaxChain = 4

// The two dispositions a field of the query vocabulary can have here.
const (
	// DispositionAxis says a tree may nest along the field.
	DispositionAxis = "axis"
	// DispositionRefused says it may not.
	DispositionRefused = "refused"
)

// The two enumerations a field can have. A closed axis draws a group for every
// member the workbench declares; an open one draws a group for every value
// some card the producer considered carries.
const (
	EnumerationClosed = "closed"
	EnumerationOpen   = "open"
)

// AxisDisposition is one row of the disposition table: what a tree does with
// one field of the query vocabulary, and how its values enumerate.
type AxisDisposition struct {
	// Field is the query field this row governs, spelled as the query
	// language spells it.
	Field string
	// Enumeration is closed or open.
	Enumeration string
	// Disposition is axis or refused.
	Disposition string
}

// AxisDispositions says what a tree does with each field of the query
// vocabulary, one row per field. The axis vocabulary is not a second list this
// package writes down: it is the query language's own field table read through
// this one, and a guard test pairs the two in both directions so a field added
// or renamed there fails the build here rather than becoming silently
// ungroupable.
//
// Nine of the ten group. at alone is refused, because an instant is a
// different value on every act and no bucket granularity has been chosen.
var AxisDispositions = []AxisDisposition{
	{Field: FieldState, Enumeration: EnumerationClosed, Disposition: DispositionAxis},
	{Field: FieldSubstate, Enumeration: EnumerationClosed, Disposition: DispositionAxis},
	{Field: FieldHolder, Enumeration: EnumerationOpen, Disposition: DispositionAxis},
	{Field: FieldBlockKind, Enumeration: EnumerationOpen, Disposition: DispositionAxis},
	{Field: FieldWorkstream, Enumeration: EnumerationOpen, Disposition: DispositionAxis},
	{Field: FieldActor, Enumeration: EnumerationOpen, Disposition: DispositionAxis},
	{Field: FieldEvent, Enumeration: EnumerationOpen, Disposition: DispositionAxis},
	{Field: FieldEntered, Enumeration: EnumerationOpen, Disposition: DispositionAxis},
	{Field: FieldLeft, Enumeration: EnumerationOpen, Disposition: DispositionAxis},
	{Field: FieldAt, Enumeration: EnumerationOpen, Disposition: DispositionRefused},
}

// GroupAxes lists the fields a tree nests along, in the disposition table's
// own order, which is the order a refusal lists them back to a reader.
func GroupAxes() []string {
	var axes []string
	for _, row := range AxisDispositions {
		if row.Disposition == DispositionAxis {
			axes = append(axes, row.Field)
		}
	}
	return axes
}

// DefaultChain is the chain a bare dinah tree nests along, which makes the
// no-argument call the status tree.
func DefaultChain() []string {
	return []string{FieldState, FieldSubstate}
}

// dispositionOf returns the table row governing a field, and whether the table
// has one.
func dispositionOf(field string) (AxisDisposition, bool) {
	for _, row := range AxisDispositions {
		if row.Field == field {
			return row, true
		}
	}
	return AxisDisposition{}, false
}

// closedAxis reports whether an axis draws a group for every member the
// workbench declares, which the table's Enumeration column rules field by
// field.
func closedAxis(axis string) bool {
	row, known := dispositionOf(axis)
	return known && row.Enumeration == EnumerationClosed
}

// ParseChain reads the axis chain a caller wrote as one comma-separated word,
// with the default standing in for an empty one.
func ParseChain(text string) []string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return DefaultChain()
	}
	parts := strings.Split(trimmed, ",")
	chain := make([]string, 0, len(parts))
	for _, part := range parts {
		chain = append(chain, strings.TrimSpace(part))
	}
	return chain
}

// checkChain runs the three axis checks in the order the spec fixes, so a
// chain of five axes one of which is a word Dinah does not group on is refused
// for the unknown word rather than for its length.
func checkChain(chain []string) error {
	for _, axis := range chain {
		row, known := dispositionOf(axis)
		if !known || row.Disposition != DispositionAxis {
			return contract.RefuseWith(contract.UnknownAxis, axis, map[string]string{
				"axes": strings.Join(GroupAxes(), ", "),
			})
		}
	}
	seen := map[string]bool{}
	for _, axis := range chain {
		if seen[axis] {
			return contract.Refuse(contract.RepeatedAxis, axis)
		}
		seen[axis] = true
	}
	if len(chain) > MaxChain {
		return contract.RefuseWith(contract.ChainTooLong, strconv.Itoa(len(chain)), map[string]string{
			"asked":   strconv.Itoa(len(chain)),
			"allowed": strconv.Itoa(MaxChain),
		})
	}
	return nil
}

// checkLevel runs the depth check against one command's own ladder, so the
// sentence lists the levels of the command that refused rather than the union
// of both ladders.
func checkLevel(level string, ladder []string) error {
	for _, declared := range ladder {
		if level == declared {
			return nil
		}
	}
	return contract.RefuseWith(contract.UnknownDepth, level, map[string]string{
		"levels": strings.Join(ladder, ", "),
	})
}

// groupedLimit is the rank the grouped walk stops at: the root is rank zero,
// the chain's axes occupy ranks one upward, and the cards sit one past the
// last of them.
func groupedLimit(level string, chain int) int {
	switch level {
	case LevelRoot:
		return 0
	case LevelGroups:
		return chain
	}
	return chain + 1
}

// contentsLimit is the rank the containment walk stops at. The ranks are
// absolute rather than counted from the root the caller named, which is what
// makes a level name mean one thing wherever it is typed: the workbench is
// rank zero, a state and a card rank one, and what a card contains rank two.
func contentsLimit(level string) int {
	switch level {
	case LevelRoot:
		return 0
	case LevelCards:
		return 1
	case LevelEntities:
		return 2
	}
	return int(^uint(0) >> 1)
}

// rankOfKind is where one entity kind sits in the containment ladder, counted
// from the workbench.
func rankOfKind(kind string) int {
	switch kind {
	case bench.KindWorkbench:
		return 0
	case bench.KindState, bench.KindCard:
		return 1
	}
	return 2
}

// Tree projects the workbench's cards as a tree, nesting whatever the filter
// selected along an ordered chain of axes.
//
// The chain arrives as the caller wrote it and the level as the name a person
// typed, because the library is the only layer that knows both the ladder and
// the chain's length. The filter is the query language's own, applied once
// over the card set through the one selection the query command runs, so the
// two commands given one string select one set of cards.
func (l *Library) Tree(req *Request, chain []string, level string) (*Tree, error) {
	if len(chain) == 0 {
		chain = DefaultChain()
	}
	if err := checkChain(chain); err != nil {
		return nil, err
	}
	if err := checkLevel(level, TreeLevels); err != nil {
		return nil, err
	}
	kept, live, err := l.selection(req.Query)
	if err != nil {
		return nil, err
	}
	cut := withheld(live, kept)
	sortByArrival(kept)
	sortByArrival(cut)
	tree := &Tree{
		Producer: ProducerGrouped,
		Subject:  SubjectCard,
		GroupBy:  chain,
		Depth:    level,
		Root: TreeNode{
			Kind:  bench.KindWorkbench,
			Ref:   l.Bench.Slug,
			Title: l.Bench.Title,
			Count: len(kept),
		},
	}
	l.fillGrouped(&tree.Root, kept, cut, chain, 0, 0, groupedLimit(level, len(chain)))
	return tree, nil
}

// withheld is the cards a filter removed: every live card that is not one of
// the survivors.
func withheld(live, kept []*bench.Card) []*bench.Card {
	survived := make(map[string]bool, len(kept))
	for _, card := range kept {
		survived[card.ID] = true
	}
	var removed []*bench.Card
	for _, card := range live {
		if !survived[card.ID] {
			removed = append(removed, card)
		}
	}
	return removed
}

// fillGrouped gives one node of the grouped tree its children and its account
// of what it is not showing.
//
// The two accounts are reported differently and the asymmetry is deliberate.
// Depth reports locally, so a node names the children it cut off directly
// beneath it and says nothing about a cut further down; a subject the depth
// removed always has an emitted ancestor at the boundary to account for it.
// The filter reports cumulatively at every node at or above a removed card,
// because a subject the filter removed may have no emitted node anywhere near
// it and a group with no survivors would otherwise read as an idle station.
func (l *Library) fillGrouped(
	node *TreeNode,
	kept, cut []*bench.Card,
	chain []string,
	axisAt, rank, limit int,
) {
	hidden := &Hidden{Filtered: len(cut)}
	children := l.groupedChildren(kept, cut, chain, axisAt, rank, limit)
	if rank < limit {
		node.Children = children
	} else if len(children) > 0 {
		hidden.Reason = append(hidden.Reason, ReasonDepth)
		hidden.Children = len(children)
		hidden.Subjects = len(kept)
	}
	if len(cut) > 0 {
		hidden.Reason = append(hidden.Reason, ReasonFilter)
	}
	if len(hidden.Reason) > 0 {
		node.Hidden = hidden
	}
}

// groupedChildren builds the nodes one level below a node of the grouped tree.
// It builds them whatever the depth allows, because a node at the boundary has
// to count what it is holding back before it drops it.
func (l *Library) groupedChildren(
	kept, cut []*bench.Card,
	chain []string,
	axisAt, rank, limit int,
) []TreeNode {
	if axisAt == len(chain) {
		nodes := make([]TreeNode, 0, len(kept))
		for _, card := range kept {
			nodes = append(nodes, l.cardNode(card))
		}
		return nodes
	}
	axis := chain[axisAt]
	var nodes []TreeNode
	for _, group := range l.groupsOn(axis, kept, cut) {
		node := TreeNode{
			Kind:  NodeGroup,
			Axis:  axis,
			Value: group.value,
			Count: len(group.kept),
		}
		l.fillGrouped(&node, group.kept, group.cut, chain, axisAt+1, rank+1, limit)
		nodes = append(nodes, node)
	}
	return nodes
}

// cardNode is one card as a leaf of the grouped tree. A card is a subject of
// itself, so it counts one.
func (l *Library) cardNode(card *bench.Card) TreeNode {
	return TreeNode{
		Kind:  bench.KindCard,
		ID:    card.ID,
		Ref:   card.Ref(l.Bench.Slug),
		Title: card.Title,
		Count: 1,
	}
}

// group is one value of an axis together with the cards that carry it: the
// survivors it draws and the cards the filter removed from it, which it still
// has to account for.
type group struct {
	value string
	kept  []*bench.Card
	cut   []*bench.Card
}

// groupsOn nests a node's cards along one axis, in the order the axis fixes.
//
// A closed-enum axis draws a group for every member the workbench declares,
// including the members holding nothing, because enumerating the flow
// completely is what the status tree is for. An open-valued axis draws a group
// for every value some card the producer considered carries, survivor or
// removed, sorted by the value's bytes ascending. The group holding the cards
// that carry no value on the axis comes last, whatever the axis.
func (l *Library) groupsOn(axis string, kept, cut []*bench.Card) []group {
	keptBy := l.gather(axis, kept)
	cutBy := l.gather(axis, cut)
	var groups []group
	for _, value := range l.axisValueOrder(axis, keptBy, cutBy) {
		groups = append(groups, group{value: value, kept: keptBy[value], cut: cutBy[value]})
	}
	return groups
}

// gather files each card under every value it carries on an axis. A card
// carrying several values is filed under each of them, which is what draws it
// under both of two workstreams and under every owner who has acted on it, and
// what makes a node's children legitimately count more than the node does.
func (l *Library) gather(axis string, cards []*bench.Card) map[string][]*bench.Card {
	by := map[string][]*bench.Card{}
	for _, card := range cards {
		for _, value := range l.axisValues(axis, card) {
			by[value] = append(by[value], card)
		}
	}
	return by
}

// axisValueOrder is the order the groups of one axis are drawn in: the values
// a closed axis declares, in the order it declares them, then any further
// value some card carries, sorted by its bytes ascending, then the no-value
// group last.
//
// A closed axis draws its declared members and is not limited to them. A card
// stands where it stands whatever the workbench file says today, so a state
// somebody deleted from the flow, and a substate written into a card by hand
// outside the three the contract names, both keep a group of their own. A card
// carrying no substate at all is not among the cases, because LoadCard supplies
// ready when the field is absent. Drawing only the declared members would drop
// those cards out of the tree with no node and no account while the root above
// them went on counting them, and a card missing from a view whose total says
// it is there is the one failure this projection must not have.
func (l *Library) axisValueOrder(axis string, keptBy, cutBy map[string][]*bench.Card) []string {
	seen := map[string]bool{}
	var order []string
	if closedAxis(axis) {
		for _, value := range l.declaredValues(axis) {
			if value == "" || seen[value] {
				continue
			}
			seen[value] = true
			order = append(order, value)
		}
	}
	var carried []string
	for _, by := range []map[string][]*bench.Card{keptBy, cutBy} {
		for value := range by {
			if value == "" || seen[value] {
				continue
			}
			seen[value] = true
			carried = append(carried, value)
		}
	}
	sort.Strings(carried)
	order = append(order, carried...)
	if len(keptBy[""]) > 0 || len(cutBy[""]) > 0 {
		order = append(order, "")
	}
	return order
}

// declaredValues enumerates a closed axis completely, in the order the
// workbench declares it: state follows the flow order of workbench.md, and
// substate follows ready, active, blocked.
//
// It is what the workbench says, rather than the whole of what the tree draws.
// A value no longer declared, and the absent value a card carrying nothing on
// the axis holds, are both drawn after these by axisValueOrder, which is where
// the order of the groups is settled.
func (l *Library) declaredValues(axis string) []string {
	if axis == FieldSubstate {
		return []string{contract.SubstateReady, contract.SubstateActive, contract.SubstateBlocked}
	}
	values := make([]string, 0, len(l.Bench.States))
	for _, state := range l.Bench.States {
		values = append(values, stateRef(state))
	}
	return values
}

// axisValues is what one card carries on an axis, as the set of groups it is
// drawn under. A card carrying nothing on the axis carries the one empty
// value, which is the no-value group.
//
// A card-plane axis reads the card as it stands. An act-plane axis reads the
// card's whole act record rather than the act that witnessed the filter, so a
// card's position in the tree does not depend on the query that found it. An
// act carrying no value on the axis contributes nothing, which is what keeps
// an axis such as entered from drawing a no-value group out of every comment
// somebody wrote.
func (l *Library) axisValues(axis string, card *bench.Card) []string {
	if !actPlane[axis] {
		return l.readableValues(axis, l.cardValues(axis, card))
	}
	events, _, err := bench.ReadJournal(card.JournalPath())
	if err != nil {
		return []string{""}
	}
	seen := map[string]bool{}
	var values []string
	for _, event := range events {
		for _, value := range l.actValues(axis, event) {
			if value == "" || seen[value] {
				continue
			}
			seen[value] = true
			values = append(values, value)
		}
	}
	if len(values) == 0 {
		return []string{""}
	}
	sort.Strings(values)
	return l.readableValues(axis, values)
}

// readableValues turns the values a card stores into the tokens a person
// types. A state-valued axis stores identifiers and a person types a slug, so
// the group is labelled with what somebody could type to reach the state. A
// value naming no state of this workbench is left as it stands, since nothing
// better is known about it.
func (l *Library) readableValues(axis string, stored []string) []string {
	if !stateValued(axis) {
		return stored
	}
	readable := make([]string, 0, len(stored))
	for _, value := range stored {
		if state := l.Bench.State(value); state != nil {
			readable = append(readable, stateRef(state))
			continue
		}
		readable = append(readable, value)
	}
	return readable
}

// Contents projects the containment grammar as a tree, walking down from any
// entity the reference resolver reaches.
//
// It takes no query. The query language's fields name card fields, and a walk
// from a comment has no cards in it, so a filter there would either mean
// nothing or mean something the grammar does not say.
func (l *Library) Contents(req *Request, level string) (*Tree, error) {
	entity, err := l.Bench.ResolveEntity(req.Ref)
	if err != nil {
		return nil, err
	}
	if err := checkLevel(level, ContentsLevels); err != nil {
		return nil, err
	}
	tree := &Tree{
		Producer: ProducerContainment,
		Subject:  SubjectEntity,
		Depth:    level,
		Root:     l.rootOf(entity),
	}
	rank := rankOfKind(entity.Kind)
	tree.Root.Count = containedCount(entity.Dir, entity.Kind)
	l.fillContained(&tree.Root, entity.Dir, entity.Kind, tree.Root.Ref, rank, contentsLimit(level))
	return tree, nil
}

// rootOf is the node the containment walk starts from, named the way a person
// would name it.
func (l *Library) rootOf(entity *bench.EntityRef) TreeNode {
	switch entity.Kind {
	case bench.KindWorkbench:
		return TreeNode{Kind: entity.Kind, Ref: l.Bench.Slug, Title: l.Bench.Title}
	case bench.KindState:
		node := TreeNode{Kind: entity.Kind, ID: entity.ID, Ref: entity.Ref}
		if state := l.Bench.State(entity.ID); state != nil {
			node.Title = state.Title
		}
		return node
	case bench.KindCard:
		return TreeNode{
			Kind:  entity.Kind,
			ID:    entity.ID,
			Ref:   entity.Card.Ref(l.Bench.Slug),
			Title: entity.Card.Title,
		}
	}
	node := TreeNode{
		Kind:  entity.Kind,
		ID:    entity.ID,
		Title: anchorTitle(entity.Dir, anchorOfKind(entity.Kind)),
	}
	if entity.Card != nil {
		node.Ref = refBelowCard(entity.Card.Ref(l.Bench.Slug), entity.Card.Dir, entity.Dir)
	}
	return node
}

// fillContained gives one node of the containment tree its children and its
// depth report. The filter never reaches this producer, so a containment node
// hides nothing but what the depth cut off.
func (l *Library) fillContained(node *TreeNode, dir, kind, ref string, rank, limit int) {
	children := l.containedChildren(dir, kind, ref, rank, limit)
	if rank < limit {
		node.Children = children
		return
	}
	if len(children) == 0 {
		return
	}
	subjects := 0
	for _, child := range children {
		subjects += child.Count
	}
	node.Hidden = &Hidden{Reason: []string{ReasonDepth}, Children: len(children), Subjects: subjects}
}

// containedChildren builds the nodes one level below an entity, reading the
// containment grammar rather than the kind. A kind the grammar does not name
// is a leaf, which is what lets a declared extension kind appear here with no
// line written for it.
//
// The workbench's own two collections are ordered by their own rules: states
// come in the flow's declared order and cards in arrival order. Every other
// collection comes in the creation order a positional reference counts in.
func (l *Library) containedChildren(dir, kind, ref string, rank, limit int) []TreeNode {
	var nodes []TreeNode
	for _, mount := range bench.Contains(kind) {
		collection := filepath.Join(dir, mount.Dir)
		for position, id := range l.membersOf(collection, mount) {
			child := l.containedNode(collection, id, position+1, mount, ref)
			l.fillContained(&child, filepath.Join(collection, id), mount.Kind, child.Ref, rank+1, limit)
			nodes = append(nodes, child)
		}
	}
	return nodes
}

// membersOf lists one collection's live members in the order the walk draws
// them.
func (l *Library) membersOf(collection string, mount bench.Mount) []string {
	switch mount.Kind {
	case bench.KindState:
		ids := make([]string, 0, len(l.Bench.States))
		for _, state := range l.Bench.States {
			ids = append(ids, state.ID)
		}
		return ids
	case bench.KindCard:
		cards, err := l.Bench.Cards()
		if err != nil {
			return nil
		}
		sortByArrival(cards)
		ids := make([]string, 0, len(cards))
		for _, card := range cards {
			ids = append(ids, card.ID)
		}
		return ids
	}
	return bench.SortByOrdinal(collection, mount.Anchor, bench.ListIDs(collection))
}

// containedNode is one entity as a node of the containment tree, with the
// reference a person types to reach it.
func (l *Library) containedNode(
	collection, id string,
	position int,
	mount bench.Mount,
	parentRef string,
) TreeNode {
	dir := filepath.Join(collection, id)
	node := TreeNode{
		Kind:  mount.Kind,
		ID:    id,
		Title: anchorTitle(dir, mount.Anchor),
		Count: containedCount(dir, mount.Kind),
	}
	switch mount.Kind {
	case bench.KindState:
		if state := l.Bench.State(id); state != nil {
			node.Ref, node.Title = stateRef(state), state.Title
		}
	case bench.KindCard:
		card, err := bench.LoadCard(collection, id)
		if err == nil {
			node.Ref, node.Title = card.Ref(l.Bench.Slug), card.Title
		}
	default:
		node.Ref = parentRef + "/" + mount.Dir + "/" + strconv.Itoa(position)
	}
	return node
}

// containedCount is how many entities sit strictly below a directory.
//
// It measures the subject set by walking the grammar and counting what it
// finds, rather than adding up the counts of the children the projection drew,
// so the number is the same whatever the depth left out. The identity that a
// node's count equals its children plus their counts follows from the walk
// rather than producing it, because a containment tree partitions its entities
// and nothing appears in it twice.
func containedCount(dir, kind string) int {
	total := 0
	for _, mount := range bench.Contains(kind) {
		collection := filepath.Join(dir, mount.Dir)
		for _, id := range bench.ListIDs(collection) {
			member := filepath.Join(collection, id)
			if !bench.Exists(filepath.Join(member, mount.Anchor)) {
				continue
			}
			total += 1 + containedCount(member, mount.Kind)
		}
	}
	return total
}

// anchorOfKind is the anchor filename one entity kind carries, read off the
// containment grammar.
func anchorOfKind(kind string) string {
	for _, mounts := range [][]bench.Mount{
		bench.Contains(bench.KindWorkbench),
		bench.Contains(bench.KindCard),
		bench.Contains(bench.KindComment),
	} {
		for _, mount := range mounts {
			if mount.Kind == kind {
				return mount.Anchor
			}
		}
	}
	return ""
}

// anchorTitle is what an entity below a card is called. The format gives these
// kinds no title field, so the anchor's own naming fields answer in turn and a
// node with nothing to say carries no title at all.
func anchorTitle(dir, anchor string) string {
	if anchor == "" {
		return ""
	}
	text, err := bench.ReadText(filepath.Join(dir, anchor))
	if err != nil {
		return ""
	}
	fm, body := bench.ParseAnchor(text)
	for _, field := range []string{"title", "text", "description", "filename"} {
		if value := fm.Value(field); value != "" {
			return value
		}
	}
	return firstLine(body)
}

// firstLine is the first line of an entity's body carrying anything, which is
// what a comment offers a reader in place of a title.
func firstLine(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// refBelowCard rebuilds the reference of an entity the caller named directly,
// as the card's own reference followed by the path below it.
func refBelowCard(cardRef, cardDir, dir string) string {
	below, err := filepath.Rel(cardDir, dir)
	if err != nil {
		return cardRef
	}
	return cardRef + "/" + filepath.ToSlash(below)
}
