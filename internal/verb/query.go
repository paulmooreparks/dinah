package verb

import (
	"sort"
	"strings"
	"time"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// Matches is the cards a query selected, in the order CORE-QUEUE-3 fixes.
type Matches struct {
	// Query is the query string the caller passed, echoed byte for byte
	// before the parser trimmed anything, so a caller reading a stored
	// result knows what produced it.
	Query string `json:"query"`
	// Cards are the matching cards, earliest arrival first.
	Cards []CardView `json:"cards"`
	// Count is how many matched, so a caller need not measure the array.
	Count int `json:"count"`
}

// The twelve field names the query language admits. The vocabulary is closed,
// so a name absent from this list is refused whatever a card carries in its
// frontmatter.
const (
	FieldState      = "state"
	FieldSubstate   = "substate"
	FieldSeverity   = bench.SeverityField
	FieldPriority   = bench.PriorityField
	FieldHolder     = "holder"
	FieldBlockKind  = "block_kind"
	FieldWorkstream = "workstream"
	FieldActor      = "actor"
	FieldEvent      = "event"
	FieldEntered    = "entered"
	FieldLeft       = "left"
	FieldAt         = "at"
)

// QueryFields lists the twelve legal field names in the order the spec's
// field table states them, which is the order a refusal lists them back to a
// reader. severity and priority sit between substate and holder, matching the
// order CardView already reports a card in.
var QueryFields = []string{
	FieldState, FieldSubstate, FieldSeverity, FieldPriority, FieldHolder,
	FieldBlockKind, FieldWorkstream, FieldActor, FieldEvent, FieldEntered,
	FieldLeft, FieldAt,
}

// The six operators a term may carry. The equality pair is what the nine
// card-plane and act-plane fields take; the four ordered ones belong to at
// alone, since equality against an instant is a question nobody asks.
const (
	opIs       = ":"
	opIsNot    = "!="
	opAtLeast  = ">="
	opAtMost   = "<="
	opAfter    = ">"
	opBefore   = "<"
	queryDate  = "2006-01-02"
	queryTrims = " \t"
)

// operators are matched longest first, so >= reads as one operator rather
// than as > followed by a value beginning with =.
var operators = []string{opIsNot, opAtLeast, opAtMost, opIs, opAfter, opBefore}

// actPlane names the five fields compared against a card's recorded acts
// rather than against the card as it stands.
var actPlane = map[string]bool{
	FieldActor:   true,
	FieldEvent:   true,
	FieldEntered: true,
	FieldLeft:    true,
	FieldAt:      true,
}

// term is one parsed term of a query.
type term struct {
	// field is the field token as the reader typed it, before any check has
	// decided whether this tool has such a field.
	field string
	// op is the operator, one of the six above.
	op string
	// values are the value's comma-separated parts under : and !=, and the
	// one whole value under an ordered operator. A term whose value was the
	// explicit empty string carries one empty part.
	values []string
	// empty marks the term whose value was written as "", which asks for
	// absence and is exempt from every vocabulary check.
	empty bool
	// instant is the parsed bound of an at term, and the zero time on every
	// other field.
	instant time.Time
	// raw is the term as it was typed, which is what a refusal names back.
	raw string
}

// query is a parsed query: its terms split by plane, since a query carrying no
// act-plane term reads no journal at all.
type query struct {
	// cardTerms are the terms compared against the card as it stands.
	cardTerms []term
	// actTerms are the terms compared against one recorded act.
	actTerms []term
}

// Query reports the live cards matching a query string, in arrival order.
//
// The refusals run in the order the spec's section 10 fixes, and the order is
// normative: a query carrying two mistakes is refused for the earlier one, so
// that a second implementation's output is comparable. The first four checks
// read no card, and the last two read every live card because a card's own
// list is half of what a workstream term may name, and a query naming a
// severity or priority also needs every card's actual value to tolerate drift.
func (l *Library) Query(req *Request) (*Matches, error) {
	matched, _, err := l.selection(req.Query)
	if err != nil {
		return nil, err
	}
	sortByArrival(matched)
	matches := &Matches{Query: req.Query, Cards: []CardView{}}
	for _, card := range matched {
		matches.Cards = append(matches.Cards, *l.view(card))
	}
	matches.Count = len(matches.Cards)
	return matches, nil
}

// selection applies a query string to the workbench's live cards, returning
// the cards it selected and every card it was measured against.
//
// It is the whole of the query language in one call: the parse, the five
// vocabulary checks and the selection, in the order the spec's section 10
// fixes. Both the query command and the grouped tree go through it, so one
// string handed to either selects one set of cards, and the tree's account of
// what the filter removed is read off the same pair rather than recomputed.
func (l *Library) selection(text string) (matched, live []*bench.Card, err error) {
	parsed, err := parseQuery(text)
	if err != nil {
		return nil, nil, err
	}
	if err := l.checkVocabularies(parsed); err != nil {
		return nil, nil, err
	}
	cards, err := l.Bench.Cards()
	if err != nil {
		return nil, nil, err
	}
	for _, card := range cards {
		if err := l.lapseRead(card); err != nil {
			return nil, nil, err
		}
	}
	if err := l.checkWorkstreams(parsed, cards); err != nil {
		return nil, nil, err
	}
	if err := l.checkLevels(parsed, cards); err != nil {
		return nil, nil, err
	}
	kept, err := l.selectCards(parsed, cards)
	if err != nil {
		return nil, nil, err
	}
	return kept, cards, nil
}

// parseQuery runs checks 1 to 3: every term parses, every field is one this
// tool knows, and every operator is one the named field accepts.
func parseQuery(text string) (*query, error) {
	tokens, err := splitTerms(strings.Trim(text, queryTrims))
	if err != nil {
		return nil, err
	}
	parsed := &query{}
	for _, token := range tokens {
		t, err := parseTerm(token)
		if err != nil {
			return nil, err
		}
		parsed.append(t)
	}
	for _, t := range parsed.all() {
		if err := checkField(*t); err != nil {
			return nil, err
		}
	}
	for _, t := range parsed.all() {
		if err := checkOperator(*t); err != nil {
			return nil, err
		}
	}
	return parsed, nil
}

// append files a parsed term under the plane its field sits on.
func (q *query) append(t term) {
	if actPlane[t.field] {
		q.actTerms = append(q.actTerms, t)
		return
	}
	q.cardTerms = append(q.cardTerms, t)
}

// all returns a pointer to every term of the query, card plane first, which is
// the order the checks read them in. The pointers are what lets check 5
// rewrite a state-valued term's parts to the identifiers they resolved to.
func (q *query) all() []*term {
	terms := make([]*term, 0, len(q.cardTerms)+len(q.actTerms))
	for i := range q.cardTerms {
		terms = append(terms, &q.cardTerms[i])
	}
	for i := range q.actTerms {
		terms = append(terms, &q.actTerms[i])
	}
	return terms
}

// splitTerms cuts a trimmed query into its terms. Any run of spaces and tabs
// separates two terms, except inside a quoted value, where a space and a tab
// are ordinary characters. A backslash inside quotes hides whatever follows
// it from the scan here, so a quotation mark it escapes does not end the
// quoting; whether the escape itself is legal is parseValue's question.
func splitTerms(text string) ([]string, error) {
	var tokens []string
	var current strings.Builder
	quoted := false
	escaped := false
	for _, r := range text {
		if escaped {
			escaped = false
			current.WriteRune(r)
			continue
		}
		switch {
		case quoted && r == '\\':
			escaped = true
		case r == '"':
			quoted = !quoted
		case !quoted && (r == ' ' || r == '\t'):
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens, nil
}

// parseTerm reads one term, which is check 1 of the spec's section 10.
//
// The field token is the run before the first character of an operator, so a
// name this tool does not have reaches the check that can name it rather than
// dying here as text the parser could not read. A run carrying a character the
// grammar's fchar production excludes has no derivation, so it is malformed
// here and never reaches that check: the run-before-the-operator rule locates
// the token and does not widen it.
func parseTerm(token string) (term, error) {
	at, op := findOperator(token)
	if at < 0 {
		return term{}, contract.Refuse(contract.Malformed, token)
	}
	field := token[:at]
	if field == "" || strings.ContainsAny(field, "\" \t") {
		return term{}, contract.Refuse(contract.Malformed, token)
	}
	t := term{field: field, op: op, raw: token}
	value, quoted, err := parseValue(token[at+len(op):], token)
	if err != nil {
		return term{}, err
	}
	t.empty = value == ""
	if t.field == FieldAt {
		instant, err := parseInstant(value)
		if err != nil {
			return term{}, contract.Refuse(contract.Malformed, token)
		}
		t.instant = instant
		t.values = []string{value}
		return t, nil
	}
	if quoted || t.empty || op != opIs && op != opIsNot {
		t.values = []string{value}
		return t, nil
	}
	t.values = strings.Split(value, ",")
	for _, part := range t.values {
		if part == "" {
			return term{}, contract.Refuse(contract.Malformed, token)
		}
	}
	return t, nil
}

// findOperator reports where a term's operator starts and which one it is. The
// operator is the first of the four characters : ! > < the term carries, read
// longest first, so >= is one operator rather than > and a value that begins
// with =. A term carrying none of the four, and one carrying a bare ! that no
// = follows, have no derivation and report -1.
func findOperator(token string) (int, string) {
	at := strings.IndexAny(token, ":!><")
	if at < 0 {
		return -1, ""
	}
	rest := token[at:]
	for _, op := range operators {
		if strings.HasPrefix(rest, op) {
			return at, op
		}
	}
	return -1, ""
}

// parseValue reads a term's value, reporting whether it was quoted. A quoted
// value is one value however many commas it carries; a bare one splits on
// them. A bare value of no characters is not a value, so `holder:` is
// malformed while `holder:""` asks for absence.
func parseValue(raw, token string) (string, bool, error) {
	if raw == "" {
		return "", false, contract.Refuse(contract.Malformed, token)
	}
	if !strings.HasPrefix(raw, "\"") {
		if strings.ContainsAny(raw, "\"\\ \t") {
			return "", false, contract.Refuse(contract.Malformed, token)
		}
		return raw, false, nil
	}
	var value strings.Builder
	body := raw[1:]
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '"':
			if i != len(body)-1 {
				return "", false, contract.Refuse(contract.Malformed, token)
			}
			return value.String(), true, nil
		case '\\':
			if i+1 >= len(body) || body[i+1] != '"' && body[i+1] != '\\' {
				return "", false, contract.Refuse(contract.Malformed, token)
			}
			i++
			value.WriteByte(body[i])
		default:
			value.WriteByte(body[i])
		}
	}
	return "", false, contract.Refuse(contract.Malformed, token)
}

// parseInstant reads an at value, which is either a full RFC 3339 timestamp or
// a date that reads as midnight UTC at the start of that day.
//
// bench.ParseStamp is not the reader for it. That one returns the zero time
// for a value it cannot read and returns no error at all, by design, so a
// query value read through it would leave at>=notadate comparing every card
// against the zero instant and reporting the whole workbench as the answer.
// Every recorded act carries a stamp, so absence on an act's instant is not a
// question a workbench can answer, and the empty value is refused here rather
// than admitted as the absence the other nine fields grant it.
func parseInstant(value string) (time.Time, error) {
	if stamped, err := time.Parse(bench.TimeFormat, value); err == nil {
		return stamped, nil
	}
	return time.Parse(queryDate, value)
}

// checkField runs check 2: the field named is one this tool has.
func checkField(t term) error {
	for _, known := range QueryFields {
		if t.field == known {
			return nil
		}
	}
	return unknownField(t.field)
}

// checkOperator runs check 3: the operator is one the named field accepts. at
// takes the four ordered operators and no other field takes any of them, since
// nothing but an instant ranks in this language. severity and priority also
// rank internally (bench.Level.Rank), but the query does not expose that
// ranking, so they take the equality pair like every other card field.
func checkOperator(t term) error {
	ordered := t.op != opIs && t.op != opIsNot
	if ordered == (t.field == FieldAt) {
		return nil
	}
	return unknownField(t.field + t.op)
}

// unknownField raises the refusal both check 2 and check 3 answer with, since
// one name covers a field this tool does not have and a field given an
// operator it does not take.
//
// The field list is read off QueryFields rather than written into the catalog,
// so a field added to the language reaches the refusal without a translator
// being asked for anything. instantField names the one field the query itself
// compares in ranked order, which is what the ordered-operator clause is
// about; the four operators themselves are written in the catalog beside the
// sentence that frames them.
func unknownField(token string) error {
	return contract.RefuseWith(contract.UnknownField, token, map[string]string{
		"fields":       strings.Join(QueryFields, ", "),
		"instantField": FieldAt,
	})
}

// checkVocabularies runs checks 4 and 5, which read the workbench's own
// definition and no card. A term whose value is empty asks for absence rather
// than naming a value, so it passes over every vocabulary check.
func (l *Library) checkVocabularies(q *query) error {
	for _, t := range q.all() {
		legal := closedValues(t.field)
		if legal == nil || t.empty {
			continue
		}
		for _, value := range t.values {
			if !contains(legal, value) {
				return unknownValue(*t, value, legal)
			}
		}
	}
	for _, t := range q.all() {
		if !stateValued(t.field) || t.empty {
			continue
		}
		for i, value := range t.values {
			state := l.Bench.StateByRef(value)
			if state == nil {
				return contract.Refuse(contract.UnknownState, value)
			}
			t.values[i] = state.ID
		}
	}
	return nil
}

// closedValues returns the vocabulary a field's value is checked against when
// the workbench definition already holds it, and nil for a field that carries
// no such check. holder and actor take an owner name and a workbench declares
// no roster of owners, and block_kind is open by design, so all three stay
// open-valued; at is validated by its own parse instead.
func closedValues(field string) []string {
	switch field {
	case FieldSubstate:
		return []string{contract.SubstateReady, contract.SubstateActive, contract.SubstateBlocked}
	case FieldEvent:
		return contract.Events
	}
	return nil
}

// stateValued reports whether a field's value names a state of the workbench,
// which is the condition ls already reports as unknown-state.
func stateValued(field string) bool {
	return field == FieldState || field == FieldEntered || field == FieldLeft
}

// checkWorkstreams runs check 6, the first of two checks that read the cards
// as well as the workbench, and it normalises the values it admits on its way
// through.
//
// The roster half of the cards is every identifier at least one live card
// lists, read from every live card rather than from the cards the query's
// other terms leave, so `state:done workstream:a` refuses a workstream nothing
// lists rather than refusing one that only a card in another state lists. The
// archive is out of reach, so an identifier only an archived card carries is
// not in the roster.
//
// The normalisation is what lets the rest of the query stay as it was. A value
// the roster admitted is rewritten to the identifier a card's list would carry
// for it, once, here, so by the time any card is read a term holds identifiers
// alone and cardValues keeps returning card.Workstreams raw. Doing it per term
// rather than per card also resolves each slug once for the whole query rather
// than once for every card the comparison walks, and it is what keeps
// `workstream!=X` the exact complement of `workstream:X` when X has two
// spellings.
func (l *Library) checkWorkstreams(q *query, cards []*bench.Card) error {
	var roster []string
	loaded := false
	for i := range q.cardTerms {
		t := &q.cardTerms[i]
		if t.field != FieldWorkstream || t.empty {
			continue
		}
		if !loaded {
			roster, loaded = l.workstreamRoster(cards), true
		}
		for j, value := range t.values {
			if !contains(roster, value) {
				return unknownValue(*t, value, roster)
			}
			t.values[j] = l.workstreamIdentifier(value)
		}
	}
	return nil
}

// workstreamRoster is every name a workstream term may carry, sorted and
// without repeats: the slug and the identifier of each live workstream, and
// every identifier the live cards list.
//
// The cards are still read, rather than the collection alone, because a card
// written before the collection was readable can list an identifier no
// workstream answers to. Keeping those in the roster is what leaves somebody
// able to find the cards carrying one and adopt it with `dinah check
// --migrate-workstreams`.
//
// An archived workstream contributes nothing of its own. Its identifier
// reaches the roster only through a live card that still lists it, which is
// the same reach every other archived thing has here.
func (l *Library) workstreamRoster(cards []*bench.Card) []string {
	seen := map[string]bool{}
	var roster []string
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		roster = append(roster, name)
	}
	for _, workstream := range l.Bench.Workstreams() {
		add(workstream.ID)
		add(workstream.Slug)
	}
	for _, card := range cards {
		for _, name := range card.Workstreams {
			add(name)
		}
	}
	sort.Strings(roster)
	return roster
}

// workstreamIdentifier is what a card's own list carries for a value the
// roster has already admitted: the workstream's identifier where the value
// names a workstream, and the value unchanged where it names an identifier no
// workstream resolves to, which is a dangling membership and is stored exactly
// as it reads.
func (l *Library) workstreamIdentifier(value string) string {
	if workstream := l.Bench.WorkstreamByRef(value); workstream != nil {
		return workstream.ID
	}
	return value
}

// checkLevels runs check 7, the newest of the query's checks and the second
// to read the cards rather than the workbench alone. It admits a term whose
// value the workbench currently declares for that axis, or whose value some
// live card actually carries on that axis, the same drift-tolerant roster
// checkWorkstreams already builds for the workstream field, and for the same
// reason: dinah check can name a card carrying a level nobody declares
// anymore, and a query that could not find that card by the value check just
// reported would make the finding unactionable from the command people
// actually filter with.
func (l *Library) checkLevels(q *query, cards []*bench.Card) error {
	rosters := map[string][]string{}
	for i := range q.cardTerms {
		t := &q.cardTerms[i]
		if (t.field != FieldSeverity && t.field != FieldPriority) || t.empty {
			continue
		}
		axis := t.field
		roster, loaded := rosters[axis]
		if !loaded {
			roster = levelRoster(l.Bench, cards, axis)
			rosters[axis] = roster
		}
		for _, value := range t.values {
			if !contains(roster, value) {
				return unknownValue(*t, value, roster)
			}
		}
	}
	return nil
}

// levelRoster is every value a term on one level axis may carry: the
// workbench's currently declared members for that axis, in the declaration
// order declaredLevelNames reports, followed by any value a live card
// actually carries there that the workbench does not declare, sorted among
// itself. Declaration order is the order a comparison like severity>=major
// would later need, per the "Levels reach the surfaces" workstream notes, so
// this reuses declaredLevelNames rather than sorting the whole roster the way
// workstreamRoster does; only the drifted tail is sorted, since nothing
// promises an order among values nobody declares.
func levelRoster(b *bench.Bench, cards []*bench.Card, axis string) []string {
	declared := declaredLevelNames(b.Levels(axis))
	seen := map[string]bool{}
	for _, name := range declared {
		seen[name] = true
	}
	var drift []string
	for _, card := range cards {
		name := card.LevelOf(axis)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		drift = append(drift, name)
	}
	sort.Strings(drift)
	roster := make([]string, 0, len(declared)+len(drift))
	roster = append(roster, declared...)
	roster = append(roster, drift...)
	return roster
}

// unknownValue composes check 4's and check 6's shared refusal, which names
// the offending value, the term it was written in, and what is legal in its
// place.
func unknownValue(t term, value string, legal []string) error {
	return contract.RefuseWith(contract.UnknownValue, value, map[string]string{
		"term":  t.raw,
		"field": t.field,
		"legal": strings.Join(legal, ", "),
	})
}

// contains reports whether a vocabulary holds a value.
func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// selectCards keeps the cards every term of the query holds for.
func (l *Library) selectCards(q *query, cards []*bench.Card) ([]*bench.Card, error) {
	var kept []*bench.Card
	for _, card := range cards {
		if !l.cardMatches(q, card) {
			continue
		}
		witnessed, err := l.actWitnessed(q, card)
		if err != nil {
			return nil, err
		}
		if !witnessed {
			continue
		}
		kept = append(kept, card)
	}
	return kept, nil
}

// cardMatches reports whether every card-plane term holds for a card.
func (l *Library) cardMatches(q *query, card *bench.Card) bool {
	for _, t := range q.cardTerms {
		if !t.holdsFor(l.cardValues(t.field, card)) {
			return false
		}
	}
	return true
}

// actWitnessed reports whether one recorded act satisfies every act-plane term
// at once, which is the single-witness rule of the spec's section 4. A query
// carrying no act-plane term reads no journal at all, so the common case costs
// nothing.
func (l *Library) actWitnessed(q *query, card *bench.Card) (bool, error) {
	if len(q.actTerms) == 0 {
		return true, nil
	}
	events, _, err := bench.ReadJournal(card.JournalPath())
	if err != nil {
		return false, err
	}
	for _, event := range events {
		if l.actMatches(q, event) {
			return true, nil
		}
	}
	return false, nil
}

// actMatches reports whether one recorded act satisfies every act-plane term.
// A != here negates the value within this act rather than over the whole card,
// which is where this plane parts company with the card plane's membership
// rule.
func (l *Library) actMatches(q *query, event bench.Event) bool {
	for _, t := range q.actTerms {
		if t.field == FieldAt {
			if !t.instantHoldsFor(bench.ParseStamp(event.TS)) {
				return false
			}
			continue
		}
		if !t.holdsFor(l.actValues(t.field, event)) {
			return false
		}
	}
	return true
}

// cardValues is what a card carries under a card-plane field, as the set the
// term is compared against. Every field but workstream carries one value,
// which is the empty string on a card that has none, and workstream carries
// its whole list. An empty list reads as the one empty value, so
// `workstream:""` asks for the same absence `holder:""` asks for and
// `workstream!=X` stays the exact complement of `workstream:X`.
func (l *Library) cardValues(field string, card *bench.Card) []string {
	switch field {
	case FieldState:
		return []string{card.State}
	case FieldSubstate:
		return []string{card.Substate}
	case FieldSeverity:
		return []string{card.Severity}
	case FieldPriority:
		return []string{card.Priority}
	case FieldHolder:
		return []string{card.Holder}
	case FieldBlockKind:
		return []string{card.BlockKind}
	case FieldWorkstream:
		if len(card.Workstreams) == 0 {
			return []string{""}
		}
		return card.Workstreams
	}
	return []string{""}
}

// actValues is what one recorded act carries under an act-plane field. entered
// reads the state the act moved the card into and left the state it moved the
// card out of, so an act that moved the card nowhere, which a comment and an
// attachment both are, carries the empty value under each.
func (l *Library) actValues(field string, event bench.Event) []string {
	switch field {
	case FieldActor:
		return []string{event.Actor}
	case FieldEvent:
		return []string{event.Event}
	case FieldEntered:
		return []string{event.To}
	case FieldLeft:
		return []string{event.From}
	}
	return []string{""}
}

// holdsFor reports whether a term holds against the values something carries
// under its field. A bare value's comma-separated parts read as or, and != is
// the exact complement of : on the same parts, so of the two operators on one
// value a card satisfies one and only one.
func (t term) holdsFor(carried []string) bool {
	found := false
	for _, want := range t.values {
		if contains(carried, want) {
			found = true
			break
		}
	}
	if t.op == opIsNot {
		return !found
	}
	return found
}

// instantHoldsFor reports whether an act's stamp satisfies an at term. The
// comparison is on instants rather than on text, so a date-only bound and a
// stored timestamp compare correctly against each other.
func (t term) instantHoldsFor(stamp time.Time) bool {
	switch t.op {
	case opAtLeast:
		return !stamp.Before(t.instant)
	case opAtMost:
		return !stamp.After(t.instant)
	case opAfter:
		return stamp.After(t.instant)
	case opBefore:
		return stamp.Before(t.instant)
	}
	return false
}
