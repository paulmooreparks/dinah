package verb

import (
	"regexp"
	"sort"
	"strconv"
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

// The built-in field names the query language admits. Beside them a term may
// name any field the workbench in front of the reader declares for its cards,
// so the vocabulary is the built-in list plus that workbench's own card keys,
// and a name in neither is refused whatever a card carries in its frontmatter.
const (
	FieldColumn     = "column"
	FieldState      = "state"
	FieldSeverity   = bench.SeverityField
	FieldPriority   = bench.PriorityField
	FieldHolder     = "holder"
	FieldBlockKind  = "block_kind"
	FieldWorkstream = "workstream"
	FieldRoute      = "route"
	// FieldStartAfter, FieldStartBy and FieldDue are a card's three
	// scheduling dates, which take all six operators and a relative value.
	FieldStartAfter = bench.StartAfterField
	FieldStartBy    = bench.StartByField
	FieldDue        = bench.DueField
	// FieldSchedule is derived rather than stored: the schedule conditions
	// that hold for the card today, from the closed set
	// contract.ScheduleConditions.
	FieldSchedule = "schedule"
	FieldActor    = "actor"
	FieldEvent    = "event"
	FieldEntered  = "entered"
	FieldLeft     = "left"
	FieldAt       = "at"
	// FieldItemOwner and FieldItemState describe one of the card's live
	// checklist items rather than the card itself, and every item-plane term
	// of a query is held to one and the same item.
	FieldItemOwner = "item_owner"
	FieldItemState = "item_state"
)

// QueryFields lists the built-in field names in the order the spec's field
// table states them, which is the order a refusal lists them back to a reader,
// ahead of the card keys the workbench declares. severity and priority sit
// between state and holder, matching the order CardView already reports a
// card in, and route follows workstream, which is where the card's own
// classifications end. The three scheduling dates and the schedule derived
// from them follow route, which is where CardView carries them. The two item
// fields follow at, because they describe the card's checklist rather than
// the card or its journal. A declared key joins the language per workbench
// and never joins this list.
var QueryFields = []string{
	FieldColumn, FieldState, FieldSeverity, FieldPriority, FieldHolder,
	FieldBlockKind, FieldWorkstream, FieldRoute,
	FieldStartAfter, FieldStartBy, FieldDue, FieldSchedule,
	FieldActor, FieldEvent,
	FieldEntered, FieldLeft, FieldAt, FieldItemOwner, FieldItemState,
}

// dateFields names the three built-in fields whose values are calendar
// dates. A declared card key of type date joins them per workbench, which
// Library.dateField answers.
var dateFields = map[string]bool{
	FieldStartAfter: true,
	FieldStartBy:    true,
	FieldDue:        true,
}

// The six operators a term may carry. The equality pair is what every field
// takes but at; the four ordered ones belong to at, since equality against an
// instant is a question nobody asks, and to the date fields, which take all
// six.
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

// itemPlane names the two fields compared against one live checklist item of
// the card rather than against the card or one of its acts.
var itemPlane = map[string]bool{
	FieldItemOwner: true,
	FieldItemState: true,
}

// meWord is the placeholder a view's query may write for the caller. It is
// expanded only in a view, and only as a whole bare part under : or !=.
const meWord = "@me"

// meExpansion is what @me stands for in the query being parsed. The zero
// value expands nothing, which is what query, tree, and search pass.
type meExpansion struct {
	enabled bool
	actor   string
	harness string // for the harness variant of the no-owner refusal
}

// queryStage names the check of the query's normative order that raised a
// refusal. The values are ordered as the checks run.
type queryStage int

const (
	stageParse       queryStage = iota + 1 // check 1
	stageField                             // check 2
	stageOperator                          // check 3
	stageMe                                // the @me step
	stageClosedValue                       // check 4
	stageColumn                            // check 5
	stageWorkstream                        // check 6
	stageLevel                             // check 7
	stageRoute                             // check 8
	stageDeclared                          // check 9
)

// queryFault is where a refused query failed: the check, and the term it was
// reading, with the field that term named. term and field are empty for a
// refusal that belongs to no one term.
type queryFault struct {
	stage queryStage
	field string // the term's field token as typed
	term  string // the term's raw text
}

// vocabulary reports whether the fault is a vocabulary refusal, which is one
// raised by checks 5 to 9 and so says this workbench's own vocabulary lacks a
// name the query uses. A nil fault, which is what an error reading the store
// carries, is never one, so a read failure is never drawn as a refused
// section.
func (f *queryFault) vocabulary() bool {
	return f != nil && f.stage >= stageColumn
}

// faultAt records that a check raised its refusal while reading one term.
func faultAt(stage queryStage, t *term) *queryFault {
	return &queryFault{stage: stage, field: t.field, term: t.raw}
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
	// bound is the resolved date of an ordered term on a date field, and no
	// date on every other term. A relative value has already been resolved
	// against today, so no card read later moves it.
	bound bench.Date
	// raw is the term as it was typed, which is what a refusal names back.
	raw string
	// expandable marks, part by part, the values a view may replace with the
	// caller: a part written bare, under : or !=, that is exactly @me.
	expandable []bool
}

// query is a parsed query: its terms split by plane, since a query carrying no
// act-plane term reads no journal at all.
type query struct {
	// cardTerms are the terms compared against the card as it stands.
	cardTerms []term
	// actTerms are the terms compared against one recorded act.
	actTerms []term
	// itemTerms are the terms compared against one live checklist item.
	itemTerms []term
	// today is the day the query's relative dates and its schedule field
	// were resolved against, read once before any card is read.
	today bench.Date
}

// Query reports the live cards matching a query string, in arrival order.
//
// The refusals run in the order the spec's section 10 fixes, and the order is
// normative: a query carrying two mistakes is refused for the earlier one, so
// that a second implementation's output is comparable. The first four checks
// read no card, and the rest read every live card because a card's own list
// is half of what a workstream term may name, a query naming a severity or
// priority needs every card's actual value to tolerate drift, and a declared
// key some card still stores after its declaration went has to stay
// findable.
func (l *Library) Query(req *Request) (*Matches, error) {
	matched, _, err := l.selection(req.Query, req.Actor)
	if err != nil {
		return nil, err
	}
	sortByArrival(matched)
	matches := &Matches{Query: req.Query, Cards: []CardView{}}
	for _, card := range matched {
		view, err := l.view(card)
		if err != nil {
			return nil, err
		}
		matches.Cards = append(matches.Cards, *view)
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
//
// The actor travels with the text because the walk lapses an expired claim as
// it goes, and a lapse that notices a hand-edited position records who was
// reading when it noticed.
func (l *Library) selection(text, actor string) (matched, live []*bench.Card, err error) {
	matched, live, _, err = l.selectionFor(text, actor, meExpansion{})
	return matched, live, err
}

// selectionFor is selection with the placeholder a view expands, and it
// reports, beside any refusal, the check that raised it. The query, tree and
// search paths reach it through selection, which passes the zero expansion
// and discards the fault, so nothing they print depends on either.
func (l *Library) selectionFor(text, actor string, me meExpansion) (matched, live []*bench.Card, fault *queryFault, err error) {
	_, matched, live, fault, err = l.selectionQuery(text, actor, me)
	return matched, live, fault, err
}

// selectionQuery is selectionFor answering the parsed query as well, which a
// view reads again to name the items that witnessed each card it drew. An
// error reading the store is returned with no fault.
func (l *Library) selectionQuery(text, actor string, me meExpansion) (parsed *query, matched, live []*bench.Card, fault *queryFault, err error) {
	parsed, fault, err = l.parseQuery(text)
	if err != nil {
		return nil, nil, nil, fault, err
	}
	if fault, err := expandMe(parsed, me); err != nil {
		return nil, nil, nil, fault, err
	}
	if fault, err := l.checkVocabularies(parsed); err != nil {
		return nil, nil, nil, fault, err
	}
	cards, err := l.Bench.Cards()
	if err != nil {
		return nil, nil, nil, nil, err
	}
	for _, card := range cards {
		if err := l.lapseRead(card, actor); err != nil {
			return nil, nil, nil, nil, err
		}
	}
	if fault, err := l.checkWorkstreams(parsed, cards); err != nil {
		return nil, nil, nil, fault, err
	}
	if fault, err := l.checkLevels(parsed, cards); err != nil {
		return nil, nil, nil, fault, err
	}
	if fault, err := l.checkRoutes(parsed, cards); err != nil {
		return nil, nil, nil, fault, err
	}
	if fault, err := l.checkDeclaredFields(parsed, cards); err != nil {
		return nil, nil, nil, fault, err
	}
	kept, err := l.selectCards(parsed, cards)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return parsed, kept, cards, nil, nil
}

// parseQuery runs checks 1 to 3: every term parses, every field is one this
// tool knows or one shaped like a declared key, and every operator is one the
// named field accepts. It is a method only because the refusal it raises
// lists the workbench's own card keys beside the built-in names.
func (l *Library) parseQuery(text string) (*query, *queryFault, error) {
	tokens, err := splitTerms(strings.Trim(text, queryTrims))
	if err != nil {
		return nil, &queryFault{stage: stageParse}, err
	}
	parsed := &query{today: l.Bench.Today(l.Now())}
	for _, token := range tokens {
		t, err := parseTerm(token)
		if err != nil {
			return nil, &queryFault{stage: stageParse, term: token}, err
		}
		if l.dateField(t.field) {
			if err := resolveDates(&t, parsed.today); err != nil {
				return nil, &queryFault{stage: stageParse, term: token}, err
			}
		}
		parsed.append(t)
	}
	for _, t := range parsed.all() {
		if err := l.checkField(*t); err != nil {
			return nil, faultAt(stageField, t), err
		}
	}
	for _, t := range parsed.all() {
		if err := l.checkOperator(*t); err != nil {
			return nil, faultAt(stageOperator, t), err
		}
	}
	return parsed, nil, nil
}

// expandMe is the step between check 3 and check 4 that replaces every
// expandable @me with the caller. It does anything only where the expansion
// is enabled, which is a view being drawn, and it refuses no-owner only when
// the query carries an expandable part and no actor resolved, so a view that
// never asks about the caller draws for anybody.
func expandMe(q *query, me meExpansion) (*queryFault, error) {
	if !me.enabled {
		return nil, nil
	}
	for _, t := range q.all() {
		for i := range t.values {
			if i >= len(t.expandable) || !t.expandable[i] {
				continue
			}
			if me.actor == "" {
				var extra map[string]string
				if me.harness != "" {
					extra = map[string]string{contract.ValueHarness: me.harness}
				}
				return &queryFault{stage: stageMe}, contract.RefuseWith(contract.NoOwner, "", extra)
			}
			t.values[i] = me.actor
		}
	}
	return nil, nil
}

// append files a parsed term under the plane its field sits on.
func (q *query) append(t term) {
	if actPlane[t.field] {
		q.actTerms = append(q.actTerms, t)
		return
	}
	if itemPlane[t.field] {
		q.itemTerms = append(q.itemTerms, t)
		return
	}
	q.cardTerms = append(q.cardTerms, t)
}

// all returns a pointer to every term of the query, card plane first, then the
// act plane, then the item plane, which is the order the checks read them in
// and so decides which of two mistakes a query is refused for. The pointers
// are what lets check 5 rewrite a column-valued term's parts to the
// identifiers they resolved to.
func (q *query) all() []*term {
	terms := make([]*term, 0, len(q.cardTerms)+len(q.actTerms)+len(q.itemTerms))
	for i := range q.cardTerms {
		terms = append(terms, &q.cardTerms[i])
	}
	for i := range q.actTerms {
		terms = append(terms, &q.actTerms[i])
	}
	for i := range q.itemTerms {
		terms = append(terms, &q.itemTerms[i])
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
	t.expandable = make([]bool, len(t.values))
	for i, part := range t.values {
		if part == "" {
			return term{}, contract.Refuse(contract.Malformed, token)
		}
		t.expandable[i] = part == meWord
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

// dateField reports whether a field's values are calendar dates: one of the
// three scheduling dates, or a declared key whose declaration reaches cards
// and has type date. It reads the workbench's declarations and no card, which
// is what lets check 1 read a date term's values before any card is read.
func (l *Library) dateField(field string) bool {
	if dateFields[field] {
		return true
	}
	if !declaredTerm(field) {
		return false
	}
	declared := l.Bench.DeclaredFieldOf(field)
	return declared != nil && declared.Type == bench.FieldTypeDate && declared.Declares(bench.KindCard)
}

// relativeDate matches a relative value on a date field: today, optionally
// followed by a sign and one to four digits of calendar days.
var relativeDate = regexp.MustCompile(`^today(?:([+-])([0-9]{1,4}))?$`)

// resolveDates is the part of check 1 that reads the values of a term on a
// date field. Every value must be a date written YYYY-MM-DD or a relative
// date, and anything else refuses the term malformed. A relative value is
// rewritten here to the date it names, so a term compares against one fixed
// date whichever card it is read against, and an ordered term also carries
// its one value as a date.
//
// A term written `:""` or `!=""` asks for absence and carries no date, so it
// is left as it stands. An ordered term takes one value, as at does, so a
// comma in it is malformed, and so is an empty value, which would otherwise
// fall through to the equality test and select the undated cards.
func resolveDates(t *term, today bench.Date) error {
	ordered := t.op != opIs && t.op != opIsNot
	if t.empty {
		if ordered {
			return contract.Refuse(contract.Malformed, t.raw)
		}
		return nil
	}
	if ordered && strings.Contains(t.values[0], ",") {
		return contract.Refuse(contract.Malformed, t.raw)
	}
	for i, value := range t.values {
		date, ok := dateValue(value, today)
		if !ok {
			return contract.Refuse(contract.Malformed, t.raw)
		}
		t.values[i] = date.String()
		if ordered {
			t.bound = date
		}
	}
	return nil
}

// dateValue reads one value of a date term: a calendar date, or today moved
// by a whole number of calendar days. Nothing else is relative.
func dateValue(value string, today bench.Date) (bench.Date, bool) {
	if date, ok := bench.ParseDate(value); ok {
		return date, true
	}
	m := relativeDate.FindStringSubmatch(value)
	if m == nil {
		return bench.Date{}, false
	}
	if m[1] == "" {
		return today, true
	}
	days, err := strconv.Atoi(m[2])
	if err != nil {
		return bench.Date{}, false
	}
	if m[1] == "-" {
		days = -days
	}
	return today.AddDays(days), true
}

// checkField runs check 2: the field named is one this tool has, or one shaped
// like a declared field key. The static check cannot know the workbench, so
// it admits the shape here and check 9 answers for the name once the cards are
// read.
func (l *Library) checkField(t term) error {
	if contains(QueryFields, t.field) || declaredTerm(t.field) {
		return nil
	}
	return l.unknownField(t.field)
}

// declaredTerm reports whether a term's field is a declared key rather than a
// built-in name, which is any token bench.DeclaredFieldKey accepts. No
// built-in name carries a full stop, so the two sets never meet.
func declaredTerm(field string) bool {
	return !contains(QueryFields, field) && bench.DeclaredFieldKey(field)
}

// checkOperator runs check 3: the operator is one the named field accepts. at
// takes the four ordered operators and neither of the equality pair. A date
// field, built in or declared with type date on cards, takes all six. Every
// other field takes the equality pair alone: severity and priority rank
// internally (bench.Level.Rank), but the query does not expose that ranking,
// and a declared key of any other type, or one nobody declares, has no order
// the language could compare in.
func (l *Library) checkOperator(t term) error {
	ordered := t.op != opIs && t.op != opIsNot
	switch {
	case l.dateField(t.field):
		return nil
	case t.field == FieldAt:
		if ordered {
			return nil
		}
	case !ordered:
		return nil
	}
	return l.unknownField(t.field + t.op)
}

// orderedFields are the fields taking >=, <=, > and <, in the order a refusal
// lists them: the built-in ones in QueryFields order, then this workbench's
// declared card keys of type date in declaration order.
func (l *Library) orderedFields() []string {
	var fields []string
	for _, field := range QueryFields {
		if field == FieldAt || dateFields[field] {
			fields = append(fields, field)
		}
	}
	for _, declared := range l.Bench.DeclaredFieldsOn(bench.KindCard) {
		if declared.Type == bench.FieldTypeDate {
			fields = append(fields, declared.Key)
		}
	}
	return fields
}

// unknownField raises the refusal checks 2, 3 and 9 answer with, since one
// name covers a field this tool does not have, a field given an operator it
// does not take, and a declared key neither this workbench declares nor any of
// its cards stores.
//
// The field list is read off QueryFields and then off the workbench's own
// declaration rather than written into the catalog, so a field added to the
// language and a key a workbench declares both reach the refusal without a
// translator being asked for anything. orderedFields names the fields the
// query compares in ranked order, which is what the ordered-operator clause is
// about; the four operators themselves are written in the catalog beside the
// sentence that frames them.
func (l *Library) unknownField(token string) error {
	fields := append([]string(nil), QueryFields...)
	fields = append(fields, l.Bench.DeclaredFieldKeysOn(bench.KindCard)...)
	return contract.RefuseWith(contract.UnknownField, token, map[string]string{
		"fields":        strings.Join(fields, ", "),
		"orderedFields": strings.Join(l.orderedFields(), ", "),
	})
}

// checkVocabularies runs checks 4 and 5, which read the workbench's own
// definition and no card. A term whose value is empty asks for absence rather
// than naming a value, so it passes over every vocabulary check.
func (l *Library) checkVocabularies(q *query) (*queryFault, error) {
	for _, t := range q.all() {
		legal := closedValues(t.field)
		if legal == nil || t.empty {
			continue
		}
		for _, value := range t.values {
			if !contains(legal, value) {
				return faultAt(stageClosedValue, t), unknownValue(*t, value, legal)
			}
		}
	}
	for _, t := range q.all() {
		if !columnValued(t.field) || t.empty {
			continue
		}
		for i, value := range t.values {
			column := l.Bench.ColumnByRef(value)
			if column == nil {
				return faultAt(stageColumn, t), contract.Refuse(contract.UnknownColumn, value)
			}
			t.values[i] = column.ID
		}
	}
	return nil, nil
}

// closedValues returns the vocabulary a field's value is checked against when
// the workbench definition already holds it, and nil for a field that carries
// no such check. holder and actor take an owner name and a workbench declares
// no roster of owners, and block_kind is open by design, so all three stay
// open-valued; at is validated by its own parse instead.
func closedValues(field string) []string {
	switch field {
	case FieldState:
		return []string{contract.StateReady, contract.StateActive, contract.StateBlocked}
	case FieldEvent:
		return contract.Events
	case FieldItemState:
		return bench.ItemStates
	case FieldSchedule:
		return contract.ScheduleConditions
	}
	return nil
}

// columnValued reports whether a field's value names a column of the workbench,
// which is the condition ls already reports as unknown-column.
func columnValued(field string) bool {
	return field == FieldColumn || field == FieldEntered || field == FieldLeft
}

// SplitQueryTerm reads one term of the query language as far as its operator,
// answering the field before it and the operator, and false for a term that
// carries no operator yet. It is the reading findOperator gives a term, offered
// to a head completing one.
func SplitQueryTerm(term string) (field, operator string, ok bool) {
	at, op := findOperator(term)
	if at < 0 {
		return "", "", false
	}
	return term[:at], op, true
}

// QueryFieldValues are the values a completion offers after a query field's
// operator, read from the workbench definition and its workstreams and never
// from a card. The first rule that answers decides: the field's closed set,
// the columns in flow order for a column-valued field, the declared levels
// for severity and priority, each live workstream's slug or identifier for
// workstream, the declared routes for route, and a declared key's enumerated
// values. Every other field, which today is holder, actor and block_kind,
// answers nothing, and so does a value a card still carries after its
// declaration went, since that is a finding for dinah check rather than a
// value to suggest.
func (l *Library) QueryFieldValues(field string) ([]string, error) {
	if closed := closedValues(field); closed != nil {
		return append([]string(nil), closed...), nil
	}
	if columnValued(field) {
		refs := make([]string, 0, len(l.Bench.Columns))
		for _, column := range l.Bench.Columns {
			refs = append(refs, column.Ref())
		}
		return refs, nil
	}
	switch field {
	case FieldSeverity, FieldPriority:
		return bench.LevelNames(l.Bench.Levels(field)), nil
	case FieldWorkstream:
		return l.workstreamHandles()
	case FieldRoute:
		return append([]string(nil), l.Bench.RouteNames...), nil
	}
	if declared := l.Bench.DeclaredFieldOf(field); declared != nil {
		return append([]string(nil), declared.Values...), nil
	}
	return nil, nil
}

// workstreamHandles are the live workstreams as a query term names them, the
// slug where one is carried and the identifier otherwise, sorted.
func (l *Library) workstreamHandles() ([]string, error) {
	workstreams, err := l.Bench.Workstreams()
	if err != nil {
		return nil, err
	}
	handles := make([]string, 0, len(workstreams))
	for _, workstream := range workstreams {
		handle := workstream.Slug
		if handle == "" {
			handle = workstream.ID
		}
		handles = append(handles, handle)
	}
	sort.Strings(handles)
	return handles, nil
}

// checkWorkstreams runs check 6, the first of two checks that read the cards
// as well as the workbench, and it normalises the values it admits on its way
// through.
//
// The roster half of the cards is every identifier at least one live card
// lists, read from every live card rather than from the cards the query's
// other terms leave, so `column:done workstream:a` refuses a workstream nothing
// lists rather than refusing one that only a card in another column lists. The
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
func (l *Library) checkWorkstreams(q *query, cards []*bench.Card) (*queryFault, error) {
	var roster []string
	loaded := false
	for i := range q.cardTerms {
		t := &q.cardTerms[i]
		if t.field != FieldWorkstream || t.empty {
			continue
		}
		if !loaded {
			known, err := l.workstreamRoster(cards)
			if err != nil {
				return nil, err
			}
			roster, loaded = known, true
		}
		for j, value := range t.values {
			if !contains(roster, value) {
				return faultAt(stageWorkstream, t), unknownValue(*t, value, roster)
			}
			t.values[j] = l.workstreamIdentifier(value)
		}
	}
	return nil, nil
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
func (l *Library) workstreamRoster(cards []*bench.Card) ([]string, error) {
	seen := map[string]bool{}
	var roster []string
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		roster = append(roster, name)
	}
	workstreams, err := l.Bench.Workstreams()
	if err != nil {
		return nil, err
	}
	for _, workstream := range workstreams {
		add(workstream.ID)
		add(workstream.Slug)
	}
	for _, card := range cards {
		for _, name := range card.Workstreams {
			add(name)
		}
	}
	sort.Strings(roster)
	return roster, nil
}

// workstreamIdentifier is what a card's own list carries for a value the
// roster has already admitted: the workstream's identifier where the value
// names a workstream, and the value unchanged where it names an identifier no
// workstream resolves to, which is a dangling membership and is stored exactly
// as it reads.
func (l *Library) workstreamIdentifier(value string) string {
	workstream, err := l.Bench.WorkstreamByRef(value)
	if err == nil && workstream != nil {
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
func (l *Library) checkLevels(q *query, cards []*bench.Card) (*queryFault, error) {
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
				return faultAt(stageLevel, t), unknownValue(*t, value, roster)
			}
		}
	}
	return nil, nil
}

// levelRoster is every value a term on one level axis may carry: the
// workbench's currently declared members for that axis, in the declaration
// order bench.LevelNames reports, followed by any value a live card
// actually carries there that the workbench does not declare, sorted among
// itself. Declaration order is the order a comparison like severity>=major
// would later need, per the "Levels reach the surfaces" workstream notes, so
// this reuses bench.LevelNames rather than sorting the whole roster the way
// workstreamRoster does; only the drifted tail is sorted, since nothing
// promises an order among values nobody declares.
func levelRoster(b *bench.Bench, cards []*bench.Card, axis string) []string {
	declared := bench.LevelNames(b.Levels(axis))
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

// checkRoutes runs check 8, the third of the checks that read the cards rather
// than the workbench alone. It admits a term whose value the workbench
// currently declares, or one some live card actually carries, which is the
// drift-tolerant roster checkWorkstreams and checkLevels already build and for
// the same reason: dinah check names a card carrying a route nobody declares,
// and a query that could not find that card by the value the finding just
// reported would make the finding unactionable from the command people filter
// with.
func (l *Library) checkRoutes(q *query, cards []*bench.Card) (*queryFault, error) {
	var roster []string
	for i := range q.cardTerms {
		t := &q.cardTerms[i]
		if t.field != FieldRoute || t.empty {
			continue
		}
		if roster == nil {
			roster = routeRoster(l.Bench, cards)
		}
		for _, value := range t.values {
			if !contains(roster, value) {
				return faultAt(stageRoute, t), unknownValue(*t, value, roster)
			}
		}
	}
	return nil, nil
}

// checkDeclaredFields runs check 9, the last of the checks and the fourth to
// read the cards. A term naming a declared key is admitted where the
// workbench declares the key for cards, or where some live card stores a
// value under it, which is the drift tolerance checkLevels and checkRoutes
// already give and for the same reason: a stored value nobody declares any
// more has to stay findable from the command people filter with. Anything
// else is refused as an unknown field. It runs after the existing eight so
// that the refusal every query carrying two mistakes already meets is
// unchanged.
//
// The value is never checked. A declared key's values are open, as holder's
// are, so a term compares whatever the card stores and ignores applicability:
// severity:major finds a value kept on a card its condition no longer admits,
// which is what makes the finding reporting it actionable from here.
func (l *Library) checkDeclaredFields(q *query, cards []*bench.Card) (*queryFault, error) {
	for i := range q.cardTerms {
		t := &q.cardTerms[i]
		if !declaredTerm(t.field) {
			continue
		}
		if field := l.Bench.DeclaredFieldOf(t.field); field != nil && field.Declares(bench.KindCard) {
			continue
		}
		if l.anyCardStores(cards, t.field) {
			continue
		}
		return faultAt(stageDeclared, t), l.unknownField(t.field)
	}
	return nil, nil
}

// anyCardStores reports whether some live card stores a value under a key,
// declared or not.
func (l *Library) anyCardStores(cards []*bench.Card, key string) bool {
	for _, card := range cards {
		if bench.FieldValue(card.FM, key) != "" {
			return true
		}
	}
	return false
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
		matched, err := l.cardMatches(q, card)
		if err != nil {
			return nil, err
		}
		if !matched {
			continue
		}
		witnessed, err := l.actWitnessed(q, card)
		if err != nil {
			return nil, err
		}
		if !witnessed {
			continue
		}
		witnessed, err = l.itemWitnessed(q, card)
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

// cardMatches reports whether every card-plane term holds for a card. An
// ordered term on a date field compares the card's date against the term's
// bound, and a schedule term reads the conditions that hold for the card
// today, which is the one card-plane read that may open the card's journal.
func (l *Library) cardMatches(q *query, card *bench.Card) (bool, error) {
	for _, t := range q.cardTerms {
		if !t.bound.IsZero() {
			if !t.dateHoldsFor(l.cardValues(t.field, card)[0]) {
				return false, nil
			}
			continue
		}
		if t.field == FieldSchedule {
			held, err := l.scheduleOf(card, q.today)
			if err != nil {
				return false, err
			}
			if len(held) == 0 {
				held = []string{""}
			}
			if !t.holdsFor(held) {
				return false, nil
			}
			continue
		}
		if !t.holdsFor(l.cardValues(t.field, card)) {
			return false, nil
		}
	}
	return true, nil
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

// itemWitnessed reports whether one live checklist item satisfies every
// item-plane term at once, which is the act plane's single-witness rule
// applied to the checklist. A query carrying no item-plane term reads no
// checklist, and a card carrying no live item has nothing to witness a term
// under either operator.
func (l *Library) itemWitnessed(q *query, card *bench.Card) (bool, error) {
	if len(q.itemTerms) == 0 {
		return true, nil
	}
	items, err := bench.Items(card.Dir)
	if err != nil {
		return false, err
	}
	for _, item := range items {
		if itemMatches(q, item) {
			return true, nil
		}
	}
	return false, nil
}

// itemWitnesses returns the reference of every live item satisfying every
// item-plane term, in stored order, which is what a view reports beside each
// card it drew. A query carrying no item-plane term has no witness to name.
func (l *Library) itemWitnesses(q *query, card *bench.Card, cardRef string) ([]string, error) {
	if len(q.itemTerms) == 0 {
		return nil, nil
	}
	items, err := bench.Items(card.Dir)
	if err != nil {
		return nil, err
	}
	var refs []string
	kindPosition := map[string]int{}
	for position, item := range items {
		kindPosition[item.Kind]++
		if !itemMatches(q, item) {
			continue
		}
		refs = append(refs, itemRef(cardRef, item.Kind, kindPosition[item.Kind], position+1))
	}
	return refs, nil
}

// itemMatches reports whether one item satisfies every item-plane term. A !=
// negates inside this one item, as it does inside one act.
func itemMatches(q *query, item *bench.Item) bool {
	for _, t := range q.itemTerms {
		if !t.holdsFor(itemValues(t.field, item)) {
			return false
		}
	}
	return true
}

// itemValues is what one item carries under an item-plane field: the owner it
// stores, which is the empty string where it stores none, and the state it
// stores. Neither is read into a default, so item_owner:operator never matches
// an item that stores no owner.
func itemValues(field string, item *bench.Item) []string {
	switch field {
	case FieldItemOwner:
		return []string{item.Owner}
	case FieldItemState:
		return []string{item.State}
	}
	return []string{""}
}

// cardValues is what a card carries under a card-plane field, as the set the
// term is compared against. Every field but workstream carries one value,
// which is the empty string on a card that has none, and workstream carries
// its whole list. An empty list reads as the one empty value, so
// `workstream:""` asks for the same absence `holder:""` asks for and
// `workstream!=X` stays the exact complement of `workstream:X`. A declared
// key carries what the card's own value block stores under it, which is the
// empty string where it stores nothing.
func (l *Library) cardValues(field string, card *bench.Card) []string {
	if declaredTerm(field) {
		return []string{bench.FieldValue(card.FM, field)}
	}
	switch field {
	case FieldColumn:
		return []string{card.Column}
	case FieldState:
		return []string{card.State}
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
	case FieldRoute:
		return []string{card.Route}
	case FieldStartAfter:
		return []string{card.StartAfter}
	case FieldStartBy:
		return []string{card.StartBy}
	case FieldDue:
		return []string{card.Due}
	}
	return []string{""}
}

// actValues is what one recorded act carries under an act-plane field. entered
// reads the column the act moved the card into and left the column it moved the
// card out of, so an act that moved the card nowhere, which a comment and an
// attachment both are, carries the empty value under each.
func (l *Library) actValues(field string, event bench.Event) []string {
	switch field {
	case FieldActor:
		return []string{event.Actor.Name}
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

// dateHoldsFor reports whether a card's stored date satisfies an ordered term
// on a date field. A card carrying no date, or carrying one that does not
// parse, is on neither side of any bound and never matches.
func (t term) dateHoldsFor(stored string) bool {
	date, ok := bench.ParseDate(stored)
	if !ok {
		return false
	}
	switch t.op {
	case opAtLeast:
		return !date.Before(t.bound)
	case opAtMost:
		return !date.After(t.bound)
	case opAfter:
		return date.After(t.bound)
	case opBefore:
		return date.Before(t.bound)
	}
	return false
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
