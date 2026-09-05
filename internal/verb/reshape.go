package verb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// The dispositions a column takes in a reshape's report. Every column of the
// live workbench and every element of the new definition sorts into exactly
// one, and an adopted entry is its own row because it names no live column to
// hang a title on.
const (
	ReshapeKept    = "kept"
	ReshapeAdded   = "added"
	ReshapeRetired = "retired"
	ReshapeAdopted = "adopted"
)

// ReshapeColumn is one row of the report's column table.
type ReshapeColumn struct {
	// ID is the identifier the column carries after the run. An added
	// column already has one here, because the identifier is derived during
	// validation rather than drawn when the column is written, so a preview
	// names the same identifier the apply would write.
	ID string `json:"id"`
	// Title is the column's title as of the new definition, or as the live
	// workbench carries it for a retirement the definition no longer names.
	// An adopted entry carries none, since nothing declares one for it.
	Title string `json:"title,omitempty"`
	// Disposition is one of kept, added, retired and adopted.
	Disposition string `json:"disposition"`
}

// ReshapeRetirement is one column whose cards the run has to place: a live
// column the new definition drops, or an identifier the workbench stopped
// naming that live cards still carry.
type ReshapeRetirement struct {
	// ID is the retiring column's identifier.
	ID string `json:"id"`
	// Title is the retiring column's title, absent for an adopted entry.
	Title string `json:"title,omitempty"`
	// Adopted marks an entry that names no live column, so no archiving runs
	// for it and nothing is removed from the columns sequence.
	Adopted bool `json:"adopted,omitempty"`
	// Destination is the identifier the cards go to, empty where the run
	// resolved none and the column stands empty.
	Destination string `json:"destination,omitempty"`
	// DestinationTitle is that column's title as the new definition declares
	// it, so a preview reads without a second lookup.
	DestinationTitle string `json:"destination_title,omitempty"`
	// Cards is how many live cards stand in the column, which a preview
	// reports as what it would carry and an apply reports as what it did.
	Cards int `json:"cards"`
	// Blocked are the identifiers of the cards standing here that carry a
	// block, which the run carries with the block intact.
	Blocked []string `json:"carried_while_blocked,omitempty"`
}

// The write-phase steps a report names when a run stopped part way through
// one. Each is a thing the operator can see in the workbench afterwards,
// rather than a stage of the code, so the four are the four acts the write
// phase performs. The reorder is not among them: it is the last step, so a run
// that reached the end of it applied.
const (
	ReshapeWroteAdded    = "added"
	ReshapeWroteCarried  = "carried"
	ReshapeWroteArchived = "archived"
	ReshapeWroteUpdated  = "updated"
)

// ReshapeWrite is one write-phase step that had already written when the run
// stopped, and how much of the workbench it wrote.
type ReshapeWrite struct {
	// Step is one of added, carried, archived and updated.
	Step string `json:"step"`
	// Count is how many columns or cards that step wrote before the run
	// stopped, which for a step that refused part way through is what it
	// managed rather than what it set out to do.
	Count int `json:"count"`
}

// ReshapeReport is what reshape answers with, preview and apply alike. The
// preview is this report with Applied false, and the apply is the same report
// with the counts the write phase actually reached.
//
// A run refused after the write phase began answers with this report too,
// carrying Wrote and Refusal and leaving Applied false, because the caller has
// to be able to tell that state from a preview and from a refusal raised
// during validation. See applyReshape for what such a run leaves behind and
// what finishes it.
type ReshapeReport struct {
	// Applied is false for a preview, which wrote nothing at all, and true
	// for a run that completed its write phase.
	Applied bool `json:"applied"`
	// Wrote is what the write phase had written when the run stopped, empty
	// for a preview, for a run refused during validation, and for a run that
	// applied, whose Applied says the same thing more directly.
	Wrote []ReshapeWrite `json:"wrote,omitempty"`
	// Refusal is the name of the refusal that stopped a run whose write
	// phase had begun, empty everywhere else. It is carried on the report
	// because such a run answers with the report rather than with the
	// ordinary machine refusal, which would say nothing about what was
	// written.
	Refusal string `json:"refusal,omitempty"`
	// Source is the --from the run was given, as the caller wrote it.
	Source string `json:"source"`
	// Columns is every column of the run, in the order the new definition
	// lists them, with the retirements after them.
	Columns []ReshapeColumn `json:"columns"`
	// Retirements is every column whose cards the run had to place.
	Retirements []ReshapeRetirement `json:"retirements,omitempty"`
	// StrandedCards is how many live cards stand in a column this run knows
	// nothing about and no --map adopted. It is a count rather than a list
	// because reshape diffs two column lists and never walks for an
	// identifier; dinah check does that walk and names the identifiers.
	StrandedCards int `json:"stranded_cards,omitempty"`
	// Updated is the kept columns whose anchor the new definition actually
	// changed, which is what the run recorded a column_updated event for.
	Updated []string `json:"updated_columns,omitempty"`
}

// PartlyApplied reports whether this run wrote part of the new shape and then
// stopped, which is the one outcome a reader cannot infer from Applied alone:
// a preview and a half-applied run both carry Applied false, and they are the
// two states an operator most needs told apart.
func (r *ReshapeReport) PartlyApplied() bool {
	return !r.Applied && len(r.Wrote) > 0
}

// note records what a write-phase step wrote, and records nothing for a step
// that wrote nothing, so an empty Wrote means the workbench is untouched
// rather than that nobody looked.
func (r *ReshapeReport) note(step string, count int) {
	if count <= 0 {
		return
	}
	r.Wrote = append(r.Wrote, ReshapeWrite{Step: step, Count: count})
}

// reshapeElement is one element of the new definition, together with the
// identifier it resolves to and where it sits in the definition's array.
type reshapeElement struct {
	id       string
	position int
	element  map[string]json.RawMessage
	slug     string
	// live is the column this element names in the workbench as it stands,
	// nil for an element the run adds.
	live *bench.Column
}

// title is what the element declares, which is what the column carries once
// the run has written it.
func (e *reshapeElement) title() string {
	return bench.MemberString(e.element, "title")
}

// takesWorkUp is the answer Column.TakesWorkUp gives for the column this
// element would leave behind, read off the element rather than off the live
// column so that a kind the definition changes is judged by the new value.
func (e *reshapeElement) takesWorkUp() bool {
	var awaiting bool
	if raw, ok := e.element["awaiting_outside"]; ok {
		if err := json.Unmarshal(raw, &awaiting); err != nil {
			awaiting = false
		}
	}
	column := bench.Column{Kind: bench.MemberString(e.element, "kind"), AwaitingOutside: awaiting}
	return column.TakesWorkUp()
}

// reshapeRetirement is a retirement as validation resolved it, before the
// report is composed from it.
type reshapeRetirement struct {
	id string
	// column is the live column retiring, nil for an adopted identifier.
	column *bench.Column
	// destination is the element the cards go to, nil while unresolved.
	destination *reshapeElement
	cards       []*bench.Card
}

// reshapePlan is everything validation computed, which the write phase then
// applies without resolving anything a second time.
type reshapePlan struct {
	definition  *bench.Definition
	digest      string
	elements    []*reshapeElement
	retirements []*reshapeRetirement
	stranded    int
	// sequence is the columns sequence as validation read it, which is the
	// whole of what this plan had a view of. Step five tells an identifier
	// the plan decided to drop from one that arrived after the plan was made
	// by asking whether this list carries it.
	sequence []string
}

// Reshape carries a live workbench from its current column layout to the one
// a new definition declares, while cards stand in the flow.
//
// It closes two gaps the tool has today. Removing an occupied column has no
// verb, and the hand edit that does it fails open: once a card's column
// resolves to nothing, claimableColumn and operatorReservesClaim both no-op on
// the nil column, so a claim nobody's guard would refuse becomes available.
// Re-kinding a column has no verb and no gate either, and a work column
// redeclared as intake under a held card leaves a claim standing where no
// owner takes work up.
//
// Everything is validated read-only before anything is written, and the two
// phases are the same phase for a preview: without the confirmation the run
// stops after producing the report, having written nothing to the workbench
// anchor or to any column directory. A destination must resolve to a column
// that will still be there once the run has finished, never one the same run
// retires, and that refusal is raised by name during validation rather than
// met halfway through a write phase as an occupancy refusal on a column the
// run had already carried cards into.
//
// The write phase is idempotent against a partially applied prior run, so a
// retry of the same source after a crash finishes what the crash interrupted
// rather than leaving a second copy of it beside the first. An added column's
// identifier is derived from the run's own frozen inputs for that reason; see
// bench.DeriveColumnID, which also states how far that guarantee reaches.
func (l *Library) Reshape(req *Request) (*ReshapeReport, error) {
	if l.Bench.Operator == "" {
		return nil, contract.Refuse(contract.NoOperator, "")
	}
	if req.Actor == "" {
		return nil, contract.Refuse(contract.NoOwner, "")
	}
	source := strings.TrimSpace(req.From)
	if source == "" {
		return nil, contract.Refuse(contract.Malformed, "from")
	}
	definition, digest, err := readReshapeSource(source)
	if err != nil {
		return nil, err
	}
	now := bench.Stamp(l.Now())
	// Validation reads the workbench under its own lock, so the flow it sorts
	// and the flow it resolves destinations against are one flow, and no
	// writer can splice a column in between the two reads. The lock is given
	// back before the write phase, because the acts the write phase composes
	// take their own: Bench.Run acquires the workbench lock itself at the
	// first step of the structural protocol, and a lock this package holds
	// open across it would refuse the archive rather than serialize it.
	//
	// What this plan is, therefore, is the run's intent rather than a fact
	// about the workbench that stays true. Between two write steps another
	// writer can claim a card, add a column or archive one, so every step
	// asks its own questions again under the lock it writes beneath. See
	// applyReshape for what each step re-asks and for what a refusal raised
	// there leaves behind.
	plan, err := l.planReshape(req, definition, digest, now)
	if err != nil {
		return nil, err
	}
	report := composeReshapeReport(source, plan)
	if !req.Confirm {
		return report, nil
	}
	if err := l.applyReshape(req, plan, now, report); err != nil {
		// The report goes back with the error rather than being dropped for
		// it, because a refusal raised once the write phase has begun leaves
		// a workbench part way through the new shape and the caller has no
		// other way to learn that. A refusal from validation reaches here
		// with nothing recorded, so it reads exactly as it always did.
		if refusal, ok := err.(*contract.Refusal); ok && len(report.Wrote) > 0 {
			report.Refusal = refusal.Name
		}
		return report, err
	}
	report.Applied = true
	return report, nil
}

// readReshapeSource reads the new definition and the digest of the exact
// bytes it was parsed from.
//
// The two shapes are the two dinah init --from already accepts: an
// interchange file, and another workbench's directory opened uncontained and
// exported. A template URL is out of scope, so nothing here reaches the
// network. readSource resolves the same two shapes and is not called, because
// it returns the definition alone and the digest has to be taken from the
// bytes that produced it rather than recomputed from a re-encoding.
func readReshapeSource(source string) (*bench.Definition, string, error) {
	var data []byte
	if bench.Exists(filepath.Join(source, bench.WorkbenchAnchor)) {
		opened, err := bench.OpenUncontained(source)
		if err != nil {
			return nil, "", err
		}
		exported, err := opened.Export()
		if err != nil {
			return nil, "", err
		}
		data = exported
	} else {
		read, err := os.ReadFile(source)
		if err != nil {
			return nil, "", contract.With(contract.Refuse(contract.UnknownPath, source), "file", source)
		}
		data = read
	}
	definition, err := bench.ReadDefinition(data)
	if err != nil {
		return nil, "", contract.With(err, "file", source)
	}
	return definition, bench.SourceDigest(data), nil
}

// planReshape is the whole validation phase: it reads the workbench under its
// lock, sorts every column, resolves every destination, and raises every
// refusal this verb can raise. It writes nothing, and a preview is this
// function and the report composed from what it returns.
func (l *Library) planReshape(req *Request, definition *bench.Definition, digest, now string) (*reshapePlan, error) {
	lock, err := bench.Acquire(l.Bench.Root, req.Actor, now)
	if err != nil {
		return nil, err
	}
	defer lock.Release()
	fresh, err := bench.Open(l.Bench.Root)
	if err != nil {
		return nil, err
	}
	cards, err := fresh.Cards()
	if err != nil {
		return nil, err
	}
	slugs, err := bench.AssignColumnSlugs(definition.Columns)
	if err != nil {
		return nil, err
	}
	plan := &reshapePlan{definition: definition, digest: digest, sequence: fresh.ColumnSequence()}
	// Sorting. An element carrying a valid identifier of its own is taken at
	// that identifier; one carrying none is taken at the identifier the run
	// derives for its position. Either way the element is kept when the
	// workbench already carries a column at that identifier and added when it
	// does not, which is also what makes a retry of an interrupted run land
	// correctly: a column the crashed attempt finished writing is live by
	// then, so the same element sorts as kept on the second pass and step one
	// writes nothing for it a second time.
	claimed := map[string]bool{}
	for position, element := range definition.Columns {
		id := bench.MemberString(element, "id")
		if !bench.IsID(id) {
			id = bench.DeriveColumnID(digest, position)
		}
		if claimed[id] {
			return nil, contract.Refuse(contract.Malformed, "id "+id)
		}
		claimed[id] = true
		plan.elements = append(plan.elements, &reshapeElement{
			id:       id,
			position: position,
			element:  element,
			slug:     slugs[position],
			live:     fresh.Column(id),
		})
	}
	retiring := map[string]*reshapeRetirement{}
	for _, column := range fresh.Columns {
		if claimed[column.ID] {
			continue
		}
		entry := &reshapeRetirement{id: column.ID, column: column, cards: cardsIn(cards, column.ID)}
		retiring[column.ID] = entry
		plan.retirements = append(plan.retirements, entry)
	}
	if err := l.resolveReplaces(plan, retiring); err != nil {
		return nil, err
	}
	if err := l.resolveMap(req, plan, fresh, cards, retiring); err != nil {
		return nil, err
	}
	if err := reshapeNeedsDestinations(plan); err != nil {
		return nil, err
	}
	if err := reshapeHeldCards(plan, fresh, cards); err != nil {
		return nil, err
	}
	plan.stranded = strandedCards(plan, fresh, cards, retiring)
	return plan, nil
}

// cardsIn is the live cards standing in one column, in the order the card
// collection lists them, which is the order every carry runs in.
func cardsIn(cards []*bench.Card, id string) []*bench.Card {
	var standing []*bench.Card
	for _, card := range cards {
		if card.Column == id {
			standing = append(standing, card)
		}
	}
	return standing
}

// resolveReplaces reads the replaces declaration off every element of the new
// definition and attaches each named retirement to the element declaring it.
//
// The declaration is always read off the incoming definition rather than off a
// live column's frontmatter, even though it persists there. The definition is
// the one place holding this run's intent for both an added column, which has
// no frontmatter yet while validation runs, and a kept column, whose element
// may declare a different replaces than an earlier run wrote to its anchor.
// Definition wins for definition, for replaces exactly as for title and kind.
func (l *Library) resolveReplaces(plan *reshapePlan, retiring map[string]*reshapeRetirement) error {
	claimedBy := map[string]*reshapeElement{}
	for _, element := range plan.elements {
		for _, id := range bench.MemberStrings(element.element, bench.ReplacesKey) {
			if first, taken := claimedBy[id]; taken {
				return contract.RefuseWith(contract.Malformed, bench.ReplacesKey, map[string]string{
					"claimants": first.title() + ", " + element.title(),
					"retired":   id,
				})
			}
			claimedBy[id] = element
			entry, retires := retiring[id]
			if !retires {
				continue
			}
			entry.destination = element
		}
	}
	return nil
}

// resolveMap reads every --map entry, in the order the caller wrote them, and
// settles the destination each one names.
//
// A later entry for the same retirement wins over an earlier one, and any
// entry wins over a replaces declaration for the same identifier, since the
// flag is the operator overriding for one run what the template author
// declared for every run.
func (l *Library) resolveMap(req *Request, plan *reshapePlan, fresh *bench.Bench, cards []*bench.Card, retiring map[string]*reshapeRetirement) error {
	for _, entry := range req.Map {
		source, destination, ok := strings.Cut(entry, "=")
		source = strings.TrimSpace(source)
		destination = strings.TrimSpace(destination)
		if !ok || source == "" || destination == "" {
			return contract.Refuse(contract.Malformed, "map "+entry)
		}
		target, err := resolveReshapeDestination(plan, fresh, retiring, destination)
		if err != nil {
			return err
		}
		retirement, err := resolveMapSource(plan, fresh, cards, retiring, source)
		if err != nil {
			return err
		}
		retirement.destination = target
	}
	return nil
}

// resolveMapSource settles which retirement a --map entry's left side names.
//
// The ordinary case is a live column the new definition drops, reached by
// identifier, slug or title. The other case is an identifier the workbench
// stopped naming that live cards still carry, which a hand edit produces and
// dinah check already reports as check.unknown-column: reshape's own sorting
// walks two column lists and never meets such an identifier, so it is
// untouched by default and adopted only where the operator names it here.
// An entry naming neither carries no card anywhere, and a typed identifier
// that carries nothing reads as a refusal rather than as a silent no-op.
func resolveMapSource(plan *reshapePlan, fresh *bench.Bench, cards []*bench.Card, retiring map[string]*reshapeRetirement, source string) (*reshapeRetirement, error) {
	if live := fresh.ColumnByRef(source); live != nil {
		if entry, retires := retiring[live.ID]; retires {
			return entry, nil
		}
		return nil, contract.Refuse(contract.ReshapeMapSourceEmpty, source)
	}
	if entry, adopted := retiring[source]; adopted {
		return entry, nil
	}
	standing := cardsIn(cards, source)
	if len(standing) == 0 {
		return nil, contract.Refuse(contract.ReshapeMapSourceEmpty, source)
	}
	entry := &reshapeRetirement{id: source, cards: standing}
	retiring[source] = entry
	plan.retirements = append(plan.retirements, entry)
	return entry, nil
}

// resolveReshapeDestination settles which column a destination names, in the
// order the design fixes: the live workbench first, then the elements the new
// definition adds, and a refusal where neither answers.
//
// The live lookup runs first because ColumnByRef is what every other verb
// resolves a column reference with, and a caller typing a slug means the
// column that slug names today. The added elements are the second tier
// because an added column exists only in the incoming JSON while validation
// runs, so ColumnByRef cannot see it: nothing has minted it a live column
// yet, and nothing will until the write phase's first step.
//
// A destination that resolves into the set this run retires is refused here,
// before anything is written. Without that refusal a map carrying one
// retirement's cards into another retiring column would leave those cards
// standing where the archive step is about to look, and that step would
// refuse dinah.column-occupied partway through a run that had already written
// some of its moves.
func resolveReshapeDestination(plan *reshapePlan, fresh *bench.Bench, retiring map[string]*reshapeRetirement, ref string) (*reshapeElement, error) {
	if live := fresh.ColumnByRef(ref); live != nil {
		if entry, retires := retiring[live.ID]; retires {
			return nil, contract.RefuseWith(contract.ReshapeDestinationRetiring, ref, map[string]string{"column": reshapeRetirementName(entry)})
		}
		for _, element := range plan.elements {
			if element.id == live.ID {
				return element, nil
			}
		}
	}
	if entry, adopted := retiring[ref]; adopted {
		return nil, contract.RefuseWith(contract.ReshapeDestinationRetiring, ref, map[string]string{"column": reshapeRetirementName(entry)})
	}
	var byTitle []*reshapeElement
	for _, element := range plan.elements {
		if element.live != nil {
			continue
		}
		if bench.MemberString(element.element, "id") == ref {
			return element, nil
		}
		if strings.EqualFold(element.title(), ref) {
			byTitle = append(byTitle, element)
		}
	}
	switch len(byTitle) {
	case 0:
		return nil, contract.Refuse(contract.UnknownColumn, ref)
	case 1:
		return byTitle[0], nil
	}
	// An added element has no live identifier to name yet, so the refusal
	// distinguishes the candidates by where each sits in the new definition's
	// columns array, which is the one handle both of them do carry.
	positions := make([]string, 0, len(byTitle))
	for _, element := range byTitle {
		positions = append(positions, strconv.Itoa(element.position))
	}
	return nil, contract.RefuseWith(contract.ReshapeDestinationAmbiguous, ref, map[string]string{"positions": strings.Join(positions, ", ")})
}

// reshapeRetirementName is what a refusal calls a retirement: the column's own
// reference where a live column stands behind it, and the bare identifier for
// an adopted entry, which names no column to have a slug.
func reshapeRetirementName(entry *reshapeRetirement) string {
	if entry.column != nil {
		return entry.column.Ref()
	}
	return entry.id
}

// reshapeNeedsDestinations refuses a retirement that live cards stand in and
// nothing named a destination for. Which station the work continues at is the
// operator's decision, so the tool refuses rather than choosing, and it names
// the column and the count so the refusal carries what the decision needs.
func reshapeNeedsDestinations(plan *reshapePlan) error {
	for _, entry := range plan.retirements {
		if entry.destination != nil || len(entry.cards) == 0 {
			continue
		}
		return contract.RefuseWith(contract.ReshapeNeedsDestination, reshapeRetirementName(entry), map[string]string{
			"cards": strconv.Itoa(len(entry.cards)),
		})
	}
	return nil
}

// reshapeHeldCards refuses a run that would leave a held card standing where
// no owner takes work up, in the two shapes that can produce one.
//
// The first is a retirement whose destination will not take work up once the
// run has written it, carrying a card somebody is holding. That one is the
// severe half: a card carried into such a column is a claim the guards would
// otherwise never see, in the same way a card left in a column that resolves
// to nothing is.
//
// The second is a kept column whose new kind or awaiting_outside stops it
// taking work up while a held card already stands there. That one is milder,
// because the column still resolves and a fresh claim there is refused the
// moment the kind flips; what is left unguarded is the claim already standing,
// which check reports and repairs nothing about. Preventing the write is
// cheaper than reporting it afterwards, so one predicate covers both.
func reshapeHeldCards(plan *reshapePlan, fresh *bench.Bench, cards []*bench.Card) error {
	for _, entry := range plan.retirements {
		if entry.destination == nil || entry.destination.takesWorkUp() {
			continue
		}
		if held := heldCardIDs(entry.cards); len(held) > 0 {
			return contract.RefuseWith(contract.ReshapeHeldCardInQueue, entry.destination.title(), map[string]string{
				"cards": strings.Join(held, ", "),
			})
		}
	}
	return reshapeKeptHeldCards(plan, fresh, cards)
}

// reshapeKeptHeldCards is the kept-column half of that predicate on its own,
// asked of whatever view of the workbench it is handed.
//
// It is a function rather than a loop inside reshapeHeldCards because it is
// asked twice: once during validation, so that a preview reports the refusal
// and a confirmed run stops before writing anything, and once again inside
// rewriteKeptColumns against a view opened under the lock that performs the
// kind flip. The second ask is the one that decides. A claim is taken under
// the card's own lock and contends with the workbench lock not at all, so a
// card claimed after validation read the flow is invisible to the first ask,
// and the write would otherwise flip the column under it.
func reshapeKeptHeldCards(plan *reshapePlan, fresh *bench.Bench, cards []*bench.Card) error {
	for _, element := range plan.elements {
		live := fresh.Column(element.id)
		if live == nil || !live.TakesWorkUp() || element.takesWorkUp() {
			continue
		}
		if held := heldCardIDs(cardsIn(cards, live.ID)); len(held) > 0 {
			return contract.RefuseWith(contract.ReshapeHeldCardInQueue, live.Ref(), map[string]string{
				"cards": strings.Join(held, ", "),
			})
		}
	}
	return nil
}

// heldCardIDs names the cards of a set that somebody is holding, by the same
// test check.go applies when it reports a claim standing where no work is
// taken up: a holder, a standing claim, or the active state.
func heldCardIDs(cards []*bench.Card) []string {
	var held []string
	for _, card := range cards {
		if card.Holder != "" || card.ClaimSince != "" || card.State == contract.StateActive {
			held = append(held, card.ID)
		}
	}
	return held
}

// strandedCards counts the live cards standing in a column this run knows
// nothing about: not in the workbench's own columns sequence, not in the new
// definition, and not adopted by a --map entry of this run.
//
// The count is reported rather than the identifiers, and the report says where
// the identifiers are found. Reshape computes a diff between two column lists;
// naming a stranded identifier takes a pass over every card's own column
// field, which dinah check already makes and this command does not duplicate.
func strandedCards(plan *reshapePlan, fresh *bench.Bench, cards []*bench.Card, retiring map[string]*reshapeRetirement) int {
	known := map[string]bool{}
	for _, id := range fresh.ColumnSequence() {
		known[id] = true
	}
	for _, element := range plan.elements {
		known[element.id] = true
	}
	count := 0
	for _, card := range cards {
		if known[card.Column] {
			continue
		}
		if _, adopted := retiring[card.Column]; adopted {
			continue
		}
		count++
	}
	return count
}

// composeReshapeReport renders the plan as the report a preview prints and an
// apply prints again with the counts it reached.
func composeReshapeReport(source string, plan *reshapePlan) *ReshapeReport {
	report := &ReshapeReport{Source: source, StrandedCards: plan.stranded}
	for _, element := range plan.elements {
		disposition := ReshapeAdded
		if element.live != nil {
			disposition = ReshapeKept
		}
		report.Columns = append(report.Columns, ReshapeColumn{
			ID:          element.id,
			Title:       element.title(),
			Disposition: disposition,
		})
	}
	for _, entry := range plan.retirements {
		row := ReshapeColumn{ID: entry.id, Disposition: ReshapeRetired}
		if entry.column != nil {
			row.Title = entry.column.Title
		} else {
			row.Disposition = ReshapeAdopted
		}
		report.Columns = append(report.Columns, row)
		retirement := ReshapeRetirement{
			ID:      entry.id,
			Title:   row.Title,
			Adopted: entry.column == nil,
			Cards:   len(entry.cards),
			Blocked: blockedCardIDs(entry.cards),
		}
		if entry.destination != nil {
			retirement.Destination = entry.destination.id
			retirement.DestinationTitle = entry.destination.title()
		}
		report.Retirements = append(report.Retirements, retirement)
	}
	return report
}

// blockedCardIDs names the cards of a set that carry a block. A reshape
// carries one with its block, its reason and its holder untouched, which
// CORE-MOVE-8 requires of every move, and the report names them so that
// nobody reading it afterwards mistakes a carried block for a cleared one.
func blockedCardIDs(cards []*bench.Card) []string {
	var blocked []string
	for _, card := range cards {
		if card.State == contract.StateBlocked {
			blocked = append(blocked, card.ID)
		}
	}
	return blocked
}

// The windows a reshape's write phase leaves open, named so that a test can
// say which one it wants a second writer to act in. Each is the interval
// between two steps, where the run holds no lock and another writer is refused
// by nothing.
//
// The card window is the exception that proves the rule: the carry holds the
// workbench lock across the whole step, and the window sits inside the loop,
// before a single card's own lock is taken. What it bounds is a decision taken
// above this seam and carried across the Acquire below it, which is the shape
// the carry once had. It does not bound a read written between the seam and
// that Acquire, and no seam could, because the two statements are adjacent, so
// a read placed there is the same defect with no window left to construct it
// in. What guards that is the order stated at Library.Do and repeated at
// carryOneCard, read by whoever next changes this loop.
const (
	reshapeStepCarry   = "carry"
	reshapeStepCard    = "card"
	reshapeStepArchive = "archive"
	reshapeStepRewrite = "rewrite"
	reshapeStepOrder   = "order"
)

// interpose gives a test the window between two write-phase steps. It does
// nothing at all in ordinary use, where the hook is nil.
func (l *Library) interpose(step string) {
	if l.Interpose != nil {
		l.Interpose(step)
	}
}

// applyReshape is the write phase, run only once validation has passed and the
// caller has confirmed. Each step is idempotent against a partially applied
// prior run of the same source, so a retry finishes what a crash interrupted.
//
// Each step takes the locks its own act needs and gives them back, so another
// writer can act between two of them. Every step therefore asks its own
// questions again, against a view it opened under the lock that covers what it
// is about to write, rather than trusting what validation saw. Three steps can
// refuse at that point, and the third is the likeliest of them:
//
//   - the carry, on a card claimed since validation read the flow, where the
//     destination takes no work up;
//   - the kept rewrite, on a column whose kind is flipping under a card
//     claimed since;
//   - the archive, on a card another writer moved into a retiring column after
//     validation listed that column's cards, which is dinah.occupied, and on
//     any card anywhere in the workbench whose own lock is held while
//     ColumnOccupied walks, which is dinah.locked. Nothing unusual is needed
//     for the second of those: an ordinary concurrent claim landing during the
//     walk is enough, which is why this step refuses more often than the other
//     two. resolveReshapeDestination gives the same reasoning where it refuses
//     a retiring destination up front rather than letting the archive meet it
//     halfway through a run that had already written some of its moves.
//
// A refusal from any of them is late, and what it leaves behind is exactly
// what the earlier steps wrote. The run records that in the report as it goes,
// so the caller can say which parts landed rather than reporting the workbench
// as untouched; runReshape prints it, and ReshapeReport.PartlyApplied is how a
// reader asks.
//
// That is survivable rather than merely tolerable, and the reason is the
// idempotency above. Nothing is rolled back, because unwinding a carry would
// mean writing moves nobody decided, which is the practice this verb exists to
// end. The operator clears what the refusal names, which is a released claim
// for the carry and the rewrite, a card moved back out for dinah.occupied and
// nothing at all for dinah.locked, and runs the same source again: the added
// columns are live so they sort as kept, the carried cards are skipped by the
// carry, the archived columns are skipped by the archive, and the run finishes
// the rest.
//
// The retry converges even against an edited source, which is more than the
// idempotency argument alone gives. An added column the stopped run had
// already appended to the sequence is live, so a second run whose definition
// no longer names it sorts it as a retirement, finds no card standing in it,
// and archives it. No orphaned directory and no stranded identifier survive
// that, which TestAnEditedRetryAfterALateRefusalArchivesTheAddedColumn holds.
// The residue DeriveColumnID's last paragraph warns about is the narrower
// case, where the first run died inside step one between writing a column's
// directory and appending its identifier, so the directory was never live to
// be retired.
//
// A run that stops here reports itself as not applied, because it did not
// apply the shape it was asked for.
func (l *Library) applyReshape(req *Request, plan *reshapePlan, now string, report *ReshapeReport) error {
	added, err := l.writeAddedColumns(req, plan, now)
	report.note(ReshapeWroteAdded, added)
	if err != nil {
		return err
	}
	l.interpose(reshapeStepCarry)
	carried, err := l.carryReshapedCards(req, plan, now)
	for index := range report.Retirements {
		report.Retirements[index].Cards = carried[report.Retirements[index].ID]
	}
	report.note(ReshapeWroteCarried, totalCarried(carried))
	if err != nil {
		return err
	}
	l.interpose(reshapeStepArchive)
	archived, err := l.archiveRetiredColumns(req, plan, now)
	report.note(ReshapeWroteArchived, archived)
	if err != nil {
		return err
	}
	l.interpose(reshapeStepRewrite)
	updated, err := l.rewriteKeptColumns(req, plan, now)
	report.note(ReshapeWroteUpdated, len(updated))
	report.Updated = updated
	if err != nil {
		return err
	}
	l.interpose(reshapeStepOrder)
	return l.writeColumnOrder(req, plan, now)
}

// totalCarried is how many cards the carry moved across every retirement,
// which is the one number the operator of a stopped run needs: which columns
// they came from is in the report's own retirement rows.
func totalCarried(carried map[string]int) int {
	total := 0
	for _, count := range carried {
		total += count
	}
	return total
}

// writeAddedColumns is write-phase step one: every column the new definition
// adds exists before step two needs to name one in a moved event, so no moment
// arises where a destination validation promised is a destination no card can
// be carried into.
//
// The three writes are ordered so that a retry of the same source finishes a
// prior attempt rather than duplicating it. Creating the directory tolerates
// finding it there, writing the anchor is a whole-file rename that has either
// not run or completed, and the append to the sequence looks for its own
// identifier first.
//
// One thing a retry does not recover, and the honest place to say so is here.
// The created events are appended after the sequence, so a crash in that gap
// leaves the column live and the retry sorts it as kept, and no created event
// is ever written for it. Recording each event before the sequence append
// would close that gap and open a smaller one, an event for a column not yet
// in the flow, so the order stands and the loss is stated rather than fixed:
// what is lost is a journal line about a column the workbench does carry, and
// dinah check reads the columns rather than the events.
//
// No placement-disruption check runs, because that check
// guards a single insertion into a stable flow and a reshape replaces the
// whole flow at once.
//
// It answers how many columns it wrote, which is what a run stopped by a later
// step reports as the part of the new shape that did land.
func (l *Library) writeAddedColumns(req *Request, plan *reshapePlan, now string) (int, error) {
	var added []*reshapeElement
	for _, element := range plan.elements {
		if element.live == nil {
			added = append(added, element)
		}
	}
	if len(added) == 0 {
		return 0, nil
	}
	lock, err := bench.Acquire(l.Bench.Root, req.Actor, now)
	if err != nil {
		return 0, err
	}
	defer lock.Release()
	fresh, err := bench.Open(l.Bench.Root)
	if err != nil {
		return 0, err
	}
	sequence := fresh.ColumnSequence()
	present := map[string]bool{}
	for _, id := range sequence {
		present[id] = true
	}
	written := 0
	for _, element := range added {
		if err := bench.WriteColumnFromElement(fresh.Root, element.id, element.slug, element.element); err != nil {
			return written, err
		}
		written++
		if present[element.id] {
			continue
		}
		present[element.id] = true
		sequence = append(sequence, element.id)
	}
	if err := fresh.SetColumnSequence(sequence); err != nil {
		return written, err
	}
	ev := bench.Event{TS: now, Event: contract.EventCreated, Actor: req.Actor}
	for _, element := range added {
		ev.Title = element.title()
		ev.Note = element.id
		if err := bench.AppendEvent(fresh.JournalPath(), ev); err != nil {
			return written, err
		}
	}
	return written, nil
}

// carryReshapedCards is write-phase step two: every card standing in a
// retiring or adopted column is carried to the destination validation settled
// for it, one card at a time under that card's own lock.
//
// The move is an ordinary moved event through the ordinary write path, with
// from, to and both titles as they stand at the moment of the carry, and with
// the reshape marker set so that a reader can tell it from a decision somebody
// took about the work. It changes neither state nor holder, which CORE-MOVE-8
// requires of any move, so a blocked card is carried blocked and a held card
// keeps its holder. No capacity limit applies: a limit bounds work pulled into
// a station, and a card that already stood in the flow is not new work.
//
// One card at a time is deliberate. A crash mid-carry leaves some cards moved
// and some not, and the ones not moved are carried by the retry, where a
// single batched write would have to be all or nothing across locks no
// filesystem takes together.
//
// Two locks are taken, and each covers what it protects. The workbench lock is
// held across the step so that the destination column read at the top of it is
// the destination every card is carried into: a column another writer archives
// mid-step would otherwise take cards after it had stopped existing. The
// card's own lock is taken before the card is read, and every value written
// comes from that read rather than from the copy validation took, which is the
// order Library.Do states and gives the reason for. Deciding from the earlier
// copy would let a claim, a block, a release or a move landing in between be
// overwritten wholesale by Card.Save, which writes the whole anchor and
// compares nothing, leaving the card's own journal describing a state the
// card no longer carries.
func (l *Library) carryReshapedCards(req *Request, plan *reshapePlan, now string) (map[string]int, error) {
	carried := map[string]int{}
	lock, err := bench.Acquire(l.Bench.Root, req.Actor, now)
	if err != nil {
		return carried, err
	}
	defer lock.Release()
	fresh, err := bench.Open(l.Bench.Root)
	if err != nil {
		return carried, err
	}
	for _, entry := range plan.retirements {
		if entry.destination == nil {
			continue
		}
		destination := fresh.Column(entry.destination.id)
		if destination == nil {
			return carried, contract.Refuse(contract.UnknownColumn, entry.destination.id)
		}
		for _, standing := range entry.cards {
			l.interpose(reshapeStepCard)
			cardLock, err := bench.Acquire(standing.Dir, req.Actor, now)
			if err != nil {
				return carried, err
			}
			carriedOne, err := l.carryOneCard(req, entry, destination, standing.ID, fresh, now)
			cardLock.Release()
			if err != nil {
				return carried, err
			}
			if carriedOne {
				carried[entry.id]++
			}
		}
	}
	return carried, nil
}

// carryOneCard is the whole of one card's carry, run with that card's lock
// already held by the caller. It reports whether the card counts as carried,
// which a card a prior attempt already carried does.
//
// Everything it decides, it decides from the card it reads here. The
// destination is re-tested for taking work up against the card as it now
// stands, because a card claimed since validation read the flow would
// otherwise be carried into a column where no owner takes work up, which is
// the state the severe half of reshapeHeldCards refuses and the state no other
// guard would ever report.
func (l *Library) carryOneCard(req *Request, entry *reshapeRetirement, destination *bench.Column, id string, fresh *bench.Bench, now string) (bool, error) {
	card, err := bench.LoadCard(fresh.CardsRoot(), id)
	if err != nil {
		return false, err
	}
	if card.Column != entry.id {
		// A prior attempt already carried this one, so the retry has nothing
		// to do for it and says so by counting it.
		return true, nil
	}
	if !entry.destination.takesWorkUp() {
		if held := heldCardIDs([]*bench.Card{card}); len(held) > 0 {
			return false, contract.RefuseWith(contract.ReshapeHeldCardInQueue, entry.destination.title(), map[string]string{
				"cards": strings.Join(held, ", "),
			})
		}
	}
	ev := bench.Event{
		TS:        now,
		Event:     contract.EventMoved,
		Actor:     req.Actor,
		From:      card.Column,
		FromTitle: reshapeDepartureTitle(entry),
		To:        destination.ID,
		ToTitle:   destination.Title,
		Reshape:   true,
	}
	card.Column = destination.ID
	if err := card.Save(); err != nil {
		return false, err
	}
	if err := bench.AppendEvent(card.JournalPath(), ev); err != nil {
		return false, err
	}
	return true, nil
}

// reshapeDepartureTitle is the title a carried card's moved event records for
// where it came from: the retiring column's own title, and the empty string
// for an adopted identifier, which names no column to have one. titleOf
// answers the same way for a nil column on the ordinary move path.
func reshapeDepartureTitle(entry *reshapeRetirement) string {
	if entry.column == nil {
		return ""
	}
	return entry.column.Title
}

// archiveRetiredColumns is write-phase step three: each retired column goes
// into the archive by the structural act archive already runs, which
// serializes under the workbench lock, records its own event, and leaves a
// resumable sibling on a crash that check --finish already resolves.
//
// Nothing new is built for the recovery here. ColumnOccupied finds each column
// empty by construction, because step two carried its cards out and validation
// refused any destination that would have put cards back into a column this
// step is about to archive.
//
// An adopted entry is skipped, since it names no live column entity: the
// identifier was orphaned from the sequence by a hand edit before this run
// began, so there is nothing to archive and nothing to remove from the
// sequence either.
//
// It answers how many columns it archived, counted as it goes, so a run this
// step refuses part way through reports the ones that did go rather than none.
func (l *Library) archiveRetiredColumns(req *Request, plan *reshapePlan, now string) (int, error) {
	archived := 0
	for _, entry := range plan.retirements {
		if entry.column == nil {
			continue
		}
		dir := filepath.Join(l.Bench.Root, bench.ColumnsDir, entry.id)
		if !bench.Exists(dir) {
			continue
		}
		fresh, err := bench.Open(l.Bench.Root)
		if err != nil {
			return archived, err
		}
		act := &bench.StructuralAct{
			Dir:       dir,
			Op:        bench.OpArchive,
			Actor:     req.Actor,
			Now:       now,
			ColumnID:  entry.id,
			ColumnRef: entry.column.Ref(),
			Record: func() error {
				ev := bench.Event{TS: now, Event: contract.EventArchived, Actor: req.Actor, Note: entry.id}
				return bench.AppendEvent(fresh.JournalPath(), ev)
			},
		}
		if err := fresh.Run(act); err != nil {
			return archived, err
		}
		archived++
	}
	return archived, nil
}

// rewriteKeptColumns is write-phase step four: every column the new definition
// keeps is written again from that definition's own element.
//
// The write is bench.WriteColumnFromElement, the same call step one makes for
// an added column, rather than a hand-picked list of fields. That is what lets
// a member absent from knownColumnKeys, replaces and reject_to alike, be
// rewritten from the new element exactly as a named field is: declared in the
// new element it lands in the rewritten anchor, and absent from it, it does
// not. A hand-picked list would need a new entry every time an unrecognised
// member is invented, and a member nobody remembered to add would survive a
// definition that dropped it.
//
// A column whose rendered anchor the rewrite does not change records nothing,
// so a reshape that only adds and retires leaves no noise on the columns it
// never touched.
//
// The held-card predicate is asked again here, against a view opened under
// this step's own lock, because this step is where a kind flip happens and
// validation's answer is by then several acts old. Take the lock, then read,
// then decide: the same order Library.Do states and columns.go's insertion
// follows. A refusal raised here is late, and what it leaves behind is
// described under applyReshape.
func (l *Library) rewriteKeptColumns(req *Request, plan *reshapePlan, now string) ([]string, error) {
	lock, err := bench.Acquire(l.Bench.Root, req.Actor, now)
	if err != nil {
		return nil, err
	}
	defer lock.Release()
	current, err := bench.Open(l.Bench.Root)
	if err != nil {
		return nil, err
	}
	standing, err := current.Cards()
	if err != nil {
		return nil, err
	}
	if err := reshapeKeptHeldCards(plan, current, standing); err != nil {
		return nil, err
	}
	var updated []string
	for _, element := range plan.elements {
		if element.live == nil {
			continue
		}
		// A column another writer archived after validation read the flow is
		// left alone rather than rewritten from the definition, because
		// writing its anchor again would put a directory back under columns/
		// that the sequence no longer names, which is the very finding this
		// card ships as check.orphaned-column-directory.
		if current.Column(element.id) == nil {
			continue
		}
		before := bench.ColumnAnchorText(l.Bench.Root, element.id)
		if err := bench.WriteColumnFromElement(l.Bench.Root, element.id, element.slug, element.element); err != nil {
			return updated, err
		}
		if bench.ColumnAnchorText(l.Bench.Root, element.id) == before {
			continue
		}
		updated = append(updated, element.id)
	}
	if len(updated) == 0 {
		return nil, nil
	}
	for _, id := range updated {
		ev := bench.Event{TS: now, Event: contract.EventColumnUpdated, Actor: req.Actor, Note: id}
		if err := bench.AppendEvent(current.JournalPath(), ev); err != nil {
			return updated, err
		}
	}
	return updated, nil
}

// writeColumnOrder is write-phase step five: the sequence takes the new
// definition's full order, kept and added identifiers interleaved exactly as
// the definition lists them. This is an ordinary reorder, which the format
// already guarantees is safe under live cards.
//
// A stranded identifier the sequence carried and neither list names is dropped
// here, because the sequence is rewritten from the new definition and the
// definition does not name it. That is the same removal dinah check
// --migrate-columns already offers, arriving as a consequence of adopting a
// new shape rather than as a repair somebody asked for.
//
// The order written is composed from the sequence this step reads under its
// own lock, not from the one validation read. Take the lock, then read, then
// decide, which is what step one already does for this same collection and
// what columns.go states outright. Writing the earlier snapshot back would
// drop a column another writer added, leaving its directory behind as
// check.orphaned-column-directory, and would return a column another writer
// archived to the sequence with no directory behind it, which is
// check.stranded-column. Those are the two findings this verb ships a check
// for, and the run must not be the thing that produces them.
//
// An identifier the sequence carries now and did not carry when validation
// read it is carried forward rather than dropped, at the end of the order. Its
// position is not preserved, because the new definition decides the flow's
// order and a reorder is safe under live cards; its membership is, because
// dropping it would be this run deciding something about a column nobody asked
// it about. The test is against the sequence validation read rather than
// against the plan's two lists, so an identifier the plan saw and deliberately
// dropped stays dropped.
func (l *Library) writeColumnOrder(req *Request, plan *reshapePlan, now string) error {
	lock, err := bench.Acquire(l.Bench.Root, req.Actor, now)
	if err != nil {
		return err
	}
	defer lock.Release()
	fresh, err := bench.Open(l.Bench.Root)
	if err != nil {
		return err
	}
	sequence := fresh.ColumnSequence()
	live := map[string]bool{}
	for _, id := range sequence {
		live[id] = true
	}
	seen := map[string]bool{}
	for _, id := range plan.sequence {
		seen[id] = true
	}
	order := make([]string, 0, len(sequence))
	for _, element := range plan.elements {
		if live[element.id] {
			order = append(order, element.id)
		}
	}
	placed := map[string]bool{}
	for _, id := range order {
		placed[id] = true
	}
	for _, id := range sequence {
		if !seen[id] && !placed[id] {
			order = append(order, id)
		}
	}
	if equalSequences(sequence, order) {
		return nil
	}
	return fresh.SetColumnSequence(order)
}

// equalSequences reports whether two identifier sequences carry the same
// identifiers in the same order, which is what tells a reorder that has
// already happened from one that still has to.
func equalSequences(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
