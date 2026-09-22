package bench

import (
	"errors"
	"path/filepath"
	"strconv"
	"strings"

	"dinah/internal/contract"
)

// Finding is one structural defect check reports, named together with the file
// it sits in so that whoever fixes it knows where to open an editor.
type Finding struct {
	// Path is the file the defect was found in.
	Path string
	// Key is the catalog key naming the defect, so the report is rendered
	// in the reader's own language rather than in the checker's.
	Key string
	// Detail is the identifier, column, state or field the defect is about.
	Detail string
	// Severity is how much the finding matters, one of SeverityDefect and
	// SeverityCleanup, and empty on a finding that declares none. Read it
	// through SeverityOf rather than off this field: an empty value means
	// defect, which is what every finding written before the severities
	// existed carries and what every structural invariant means.
	Severity string
}

// The catalog keys check reports its findings under. Each names one invariant
// the format document states.
const (
	FindingClaimWithoutActive = "check.claim-without-active"
	FindingActiveWithoutClaim = "check.active-without-claim"
	FindingBlockWithoutReason = "check.block-without-reason"
	FindingHolderOnUnheld     = "check.holder-on-unheld"
	FindingUnknownColumn      = "check.unknown-column"
	FindingDanglingLink       = "check.dangling-link"
	FindingPositionDiverges   = "check.position-diverges"
	FindingMissingAnchor      = "check.missing-anchor"
	FindingTornJournal        = "check.torn-journal"
	FindingUnknownState       = "check.unknown-state"
	FindingInterruptedAct     = "check.interrupted-act"
	FindingEntityAtBothPaths  = "check.entity-at-both-paths"
	FindingOrdinalMissing     = "check.ordinal-missing"
	FindingOrdinalDuplicate   = "check.ordinal-duplicate"
	// FindingCardNumberDuplicate names a registry line claiming a number
	// another well-formed line claims as well. It is modelled on
	// FindingOrdinalDuplicate, and it parts company with it in reporting
	// every member of a colliding group rather than the second one met. A
	// tombstone draws none of it, because a number somebody gave back is
	// free for the next filing to take. Path names the registry file and
	// Detail is the line as stored, which is what a reader searches the
	// file for.
	FindingCardNumberDuplicate = "check.card-number-duplicate"
	// FindingCardNumberRepeated names a registry line whose identifier
	// another well-formed line claims as well. A duplicate number is two
	// cards answering one number, and this is one card answering two, which
	// resolution reads as the first line that claims the identifier. No
	// repair flag resolves it, because the two lines disagree about when the
	// card was filed, and deciding which of them is true is an operator's
	// call.
	FindingCardNumberRepeated = "check.card-number-repeated"
	// FindingCardNumberMissing names a card no registry line claims, in
	// either half of the collection. Path is the card's anchor and Detail is
	// the card's identifier. A workbench the migration has not reached holds
	// an empty registry, so the finding floods one per card, and the flood
	// is the intended signal. The migration is the repair, and a workbench
	// waiting for it is exactly what one finding per card says.
	FindingCardNumberMissing = "check.card-number-missing"
	// FindingCardNumberStranded names a well-formed registry line whose
	// identifier names no card directory in either half of the collection.
	// Path names the registry file and Detail is the line as stored. A
	// directory that stands but will not load is reported through
	// unreadableCardFinding instead, since the defect there is the card
	// rather than the line, and the classifier the card walk already uses
	// keeps one condition from printing two spellings.
	FindingCardNumberStranded = "check.card-number-stranded"
	// FindingCardNumberMalformed names a registry line the grammar refuses.
	// The reader keeps the line and enters it in no index, on the terms a
	// card carrying an unknown level stays openable, so it can collide with
	// nothing and strand nothing and is named for its own bytes alone.
	// Detail is the line as stored, so an operator fixes the exact text
	// rather than a number this build guessed at.
	FindingCardNumberMalformed = "check.card-number-malformed"
	// FindingCardNumberInFrontmatter names a card on a workbench the
	// migration has reached that still carries a number in its anchor. The
	// write path stopped stamping the key when the registry arrived and the
	// migration strips it, so a card meeting this condition was written by a
	// hand or by a tool older than the registry. The number it carries is
	// not the one it answers to, because the registry line is. Path is the
	// card's anchor and Detail is the card's identifier.
	FindingCardNumberInFrontmatter = "check.card-number-in-frontmatter"
	FindingSlugMissing             = "check.slug-missing"
	FindingSlugMalformed           = "check.slug-malformed"
	FindingSlugDuplicate           = "check.slug-duplicate"
	FindingStrandedColumn          = "check.stranded-column"
	// FindingOrphanedColumnDirectory names a directory under columns/ that
	// the workbench's own columns sequence does not carry. It is the mirror
	// of FindingStrandedColumn and never fires over the same identifier,
	// since one names a sequence entry with no directory and the other a
	// directory with no sequence entry.
	//
	// Path names the directory itself rather than the workbench anchor,
	// which is where the two findings part company: a stranded identifier
	// names nothing a reader can open, and this names something they can.
	// No repair flag offers to remove it, because removing a directory that
	// may hold somebody's column is a destructive act, and reading it is
	// what an operator needs before deciding anything.
	FindingOrphanedColumnDirectory = "check.orphaned-column-directory"
	// FindingColumnBodyOverLimit names a declared column whose body is
	// longer, in bytes, than the column_body_limit the workbench declares.
	// Path is the column's anchor and Detail is the column's reference, then
	// the body's size and the limit written as size/limit. It is reported at
	// cleanup severity, because a long body works and only costs every
	// arrival at the column the reading of it.
	FindingColumnBodyOverLimit = "check.column-body-over-limit"
	// FindingColumnBodyLimitMalformed names a column_body_limit that is not a
	// whole number of bytes above zero. Path is the workbench's anchor and
	// Detail is the value as stored, and no body is measured in a run that
	// reports it.
	FindingColumnBodyLimitMalformed = "check.column-body-limit-malformed"
	// FindingBareWorkbench names a directory carrying a workbench.md this tool
	// recognises as its own and sitting outside any .dinah container, which
	// the containment rule means is no longer a workbench. It is reported
	// rather than refused because the repair moves a directory, and it is
	// reported separately from a foreign anchor because the two need
	// different sentences: a foreign anchor was never Dinah's, where this one
	// is a workbench somebody has been using that stopped being found.
	FindingBareWorkbench = "check.bare-workbench"
	// FindingDamagedWorkbench names a directory carrying a workbench.md this
	// tool cannot recognise, sitting exactly where the containment rule says
	// a workbench belongs. It is distinct from FindingBareWorkbench, which is
	// a recognised anchor in the wrong place, and from FindingIgnoredAnchor,
	// which is an anchor the discovery walk met and was never entitled to
	// claim: this one is a workbench that already claimed its place by
	// position and cannot speak for itself by content. Nothing
	// --migrate-container does repairs it, since the repair here is content
	// rather than position.
	FindingDamagedWorkbench = "check.damaged-workbench"
	// FindingDuplicateWorkbenchID names two or more directories sharing one
	// workbench identifier, with every path in Detail. It is never repaired
	// by the sweep, because a git clone of one workbench and a copy somebody
	// made instead of creating a second workbench look identical on disk and
	// want opposite repairs. `dinah check --remint <path>` is the explicit
	// act that settles it.
	FindingDuplicateWorkbenchID = "check.duplicate-workbench-id"
	// FindingUnknownLevel names a card whose stored severity, priority or
	// tier is no member of what this workbench declares for that axis, which
	// covers a declaration that has since changed and one that was never
	// made. It sits beside FindingUnknownColumn for the same reason: the
	// write path is the only place a level is validated, because refusing on
	// read would make a workbench unreadable the moment somebody edits its
	// declaration.
	//
	// Three places raise it, and each writes its own detail. A card's own
	// level carries "<axis> <value>". A column's tier default and a card's
	// per-column tier override each carry "tier@<column ref> <value>", so a
	// reader who has seen one of the two recognises the other.
	FindingUnknownLevel  = "check.unknown-level"
	FindingIgnoredAnchor = "check.ignored-anchor"
	// FindingCardVocabularyMixed and FindingCardVocabularyRetired name the
	// two headers the card reader refuses, and they exist because check is
	// the tool a reader reaches for when the reader has refused. Without
	// them every such card was reported as a directory carrying no anchor
	// file, which is untrue of a file that is plainly there and which
	// invites the reader to delete a directory holding a card.
	FindingCardVocabularyMixed   = "check.card-vocabulary-mixed"
	FindingCardVocabularyRetired = "check.card-vocabulary-retired"
	// FindingClaimWhereNoWorkIsTaken names a card held at a column where no
	// owner takes work up, which the acts are refused from the day this
	// build ships and which a board written before that day can still carry.
	// It sits beside FindingUnknownLevel on the same posture: refusing such
	// a workbench on read would leave it unopenable with no route back.
	FindingClaimWhereNoWorkIsTaken = "check.claim-where-no-work-is-taken"
	// FindingKindOutOfPosition names a column whose kind is not allowed to
	// stand where it stands in the flow, and is reported rather than
	// refused for the reason above. The repair is an edit to workbench.md or
	// to a column.md, so no flag offers to make it.
	FindingKindOutOfPosition = "check.kind-out-of-position"
	// FindingRejectTargetUnknown names a column whose reject_to names no
	// column this workbench carries.
	FindingRejectTargetUnknown = "check.reject-target-unknown"
	// FindingUnknownTierColumn names a card whose tier_at entry names no
	// column this workbench carries, which a reshape retiring a column and a
	// hand edit both produce. It is reported rather than refused on the same
	// posture reject_to keeps: a write catches a typo before it lands, and a
	// read tolerates a column that existed when the line was written.
	FindingUnknownTierColumn = "check.unknown-tier-column"
	// FindingItemColumnUnresolved names a checklist item whose column value
	// can never hold a card: either it resolves to no column at all, or it
	// resolves to one whose identifier the value does not spell, and
	// GatingItems compares an item's column against a column identifier by
	// string equality. A write made before the field ran a guard, and a hand
	// edit, both produce it. It is reported rather than refused on the same
	// posture the two above keep, since the write path is where a typo is
	// caught and a read tolerates a column that has since been retired.
	FindingItemColumnUnresolved = "check.item-column-unresolved"
	// FindingStoredCarriageReturn names a workbench text file carrying a
	// carriage return that stands for a line ending, which the format's
	// Encoding section forbids a writer to produce. The detail carries the
	// count the file's own transform would remove, and separately the count of
	// carriage returns that are not line endings, because the second number is
	// legal and the first is not.
	//
	// A file whose carriage returns are all of the legal kind is reported by
	// nothing at all, on the operator's ruling of 2026-09-15: such a file
	// conforms, and reporting it made dinah check exit non-zero for ever over a
	// store nothing could clear. A file carrying one of each is still reported
	// here, for the one that is not legal.
	//
	// The counts come from the transform rather than from a pattern, so the
	// finding and the migration can never disagree about which files are
	// dirty. It is what catches the one site no write-path change can reach,
	// which is an external editor writing an anchor.
	FindingStoredCarriageReturn = "check.stored-carriage-return"
	// FindingNewlineRepairUnsupported names a workbench text file the newline
	// repair refuses to decide: a file bearing one of the anchor names the
	// format fixes that does not round-trip through the anchor reader, or a
	// frontmatter key whose shape the repair cannot re-render. The detail
	// carries which.
	//
	// It is its own key rather than the one above because such a file need
	// carry no carriage return at all, and reporting one as storing a line
	// ending told a reader to repair a file that was never dirty.
	FindingNewlineRepairUnsupported = "check.newline-repair-unsupported"
	// FindingRejectTargetIsSelf names a column whose reject_to names itself.
	FindingRejectTargetIsSelf = "check.reject-target-is-self"
	// FindingRejectTargetForward names a column whose reject_to names a column
	// standing ahead of it in the flow whose kind is not done. A forward
	// reject_to landing in a done column is not reported, because a rejected
	// card ends in the same done queue a finished one ends in and carries
	// its own outcome, which is the ruling dinah-207 records as D-5. See the
	// column.md reject_to section of docs/design/format.md for the reasoning.
	FindingRejectTargetForward = "check.reject-target-forward"
	// FindingAtLoopLimit names a card whose regressive-departure count from
	// the column it stands in has reached that column's own declared
	// loop_limit. The next regressive move out of that column is refused, so
	// the card is waiting on the operator whether or not anybody has noticed,
	// and this is where a board says so before somebody meets the refusal.
	// The count keeps rising past the limit, because an override carries one
	// move rather than lifting the cap, so the finding stands for the rest of
	// the card's life at that column.
	FindingAtLoopLimit = "check.at-loop-limit"
	// FindingUnknownKind names a column carrying a layer's kind this build
	// does not implement. CORE-STATE-12 says such a column is read as though
	// its kind were work, and the sentence says so, because a reader
	// otherwise has no way to know what the tool did with it.
	FindingUnknownKind = "check.unknown-kind"
	// FindingDanglingWorkstream names a card listing a workstream identifier
	// that resolves in neither half of the workstreams collection, on the
	// same terms FindingDanglingLink already reports a link's to.
	FindingDanglingWorkstream = "check.dangling-workstream"
	// The three workstream slug findings mirror the column's own three. None
	// of them turns on the profile revision the workbench declares, because
	// the profile says nothing about a workstream at all.
	FindingWorkstreamSlugMissing   = "check.workstream-slug-missing"
	FindingWorkstreamSlugMalformed = "check.workstream-slug-malformed"
	FindingWorkstreamSlugDuplicate = "check.workstream-slug-duplicate"
	// The last two are raised by the slug migration rather than by the
	// checker, on the terms FindingSlugUnderivable and FindingSlugUnwritable
	// are raised for a column. They are separate names because the sentence
	// names the entity, and a workstream reported as a column would send a
	// reader to the wrong listing.
	FindingWorkstreamSlugUnderivable = "check.workstream-slug-underivable"
	FindingWorkstreamSlugUnwritable  = "check.workstream-slug-unwritable"
	// FindingWorkbenchSlugMissing names a workbench written before the
	// workbench-level slug field existed, on the same report-only terms
	// FindingSlugMissing already reports a column's absence.
	FindingWorkbenchSlugMissing = "check.workbench-slug-missing"
	// FindingAttachmentFilenameDrift names an attachment whose payload
	// file name differs from the filename the anchor records. A crash
	// between the two writes of a rename lands here, and a hand-written
	// anchor that nobody noticed is caught the same way.
	FindingAttachmentFilenameDrift = "check.attachment-filename-drift"
	// FindingAttachmentsWithoutAMount names an attachments directory sitting
	// below an entity whose kind the containment table gives no attachments
	// mount. The attach verb wrote them before it refused the act, and nothing
	// reaches them afterwards: descend refuses the path and the containment
	// walk cannot see them, so the entity holding them reports a count short by
	// what is inside.
	//
	// Path names the directory itself rather than the anchor above it, because
	// a reader has to open it to decide what to do with the files, and Detail
	// names the kind rather than an identifier, because the identifier is
	// already the last segment of the path. Nothing repairs it: the bytes
	// belong to whoever attached them, and the anchor beside each one records a
	// filename and a provenance that a silent removal would destroy.
	FindingAttachmentsWithoutAMount = "check.attachments-without-a-mount"
	// FindingWorkbenchSlugMalformed names a stored workbench slug that does
	// not conform to the grammar. Open validates the stored slug at no major,
	// so a slug written by hand reaches the checker rather than being
	// pre-empted by a refusal, and the workbench still opens while it is
	// reported.
	FindingWorkbenchSlugMalformed = "check.workbench-slug-malformed"
	// The last eight are raised by a repair rather than by the checker,
	// because each names something only the run that did the work can know:
	// which entity it placed by guesswork, which card a lock kept it out of,
	// which entity it could not write to, which title it could derive no
	// slug from, which column or workbench anchor it could not write a slug
	// to, and which card a renumbering gave a number it did not arrive
	// holding. None of them survives on disk for a later check to find.
	FindingOrdinalGuessed           = "check.ordinal-guessed"
	FindingOrdinalLocked            = "check.ordinal-locked"
	FindingOrdinalUnwritable        = "check.ordinal-unwritable"
	FindingSlugUnderivable          = "check.slug-underivable"
	FindingSlugUnwritable           = "check.slug-unwritable"
	FindingWorkbenchSlugUnderivable = "check.workbench-slug-underivable"
	// FindingCardNumberRenumbered names a card one of the two number repairs
	// gave a number different from the one it arrived holding, which answers
	// the operator who asks why a card is called something else this morning.
	// The renumbered event on the card's journal says the same thing where
	// history keeps it, and the finding says it where the run's report is
	// read. Path is the card's anchor and Detail is the card's identifier,
	// the shape every card-scoped finding in this file keeps.
	FindingCardNumberRenumbered = "check.card-number-renumbered"
	// FindingFieldDeclarationMalformed names one entry of the workbench's
	// fields block the declaration reader refused: a key the grammar refuses,
	// a type absent or outside the five, or a meaning absent or blank. The
	// entry declares nothing and the workbench still opens, which is the
	// posture readLevels keeps for a level block it cannot parse.
	FindingFieldDeclarationMalformed = "check.field-declaration-malformed"
	// FindingRequiredFieldUndeclared names a column whose require_fields
	// declaration carries a key the workbench does not declare. It holds
	// nothing, because a key nothing can ever be written under would make the
	// column unreachable, so the posture matches FindingItemColumnUnresolved,
	// which reports rather than refusing the workbench.
	FindingRequiredFieldUndeclared = "check.required-field-undeclared"
	// FindingBranchHeadingInBody names a card whose body still carries the
	// heading the declared field git.branch replaced, on a workbench that
	// declares FieldsFormat or higher. A workbench declaring less has not been
	// carried across the retirement, so the heading is still its convention
	// and nothing is reported.
	FindingBranchHeadingInBody = "check.branch-heading-in-body"
	// FindingWitnessLocked names a card the witness repair could not reach,
	// because a lock stood on it while the walk passed. The walk stepped over
	// it and carried on, so the card stays diverged until the repair is run
	// again or a touch that reads its position reaches it.
	FindingWitnessLocked = "check.witness-locked"

	// FindingTierWithoutModels names a member of levels.tier that the table
	// lists no model for, on a workbench that declares a table. A tier
	// nothing satisfies strands every card that asks for it.
	FindingTierWithoutModels = "check.tier-without-models"

	// FindingTiersWithoutLevels names a tiers block on a workbench that
	// declares no levels.tier. The block declares nothing, and the finding
	// says so rather than leaving a reader to wonder why a table they wrote
	// has no effect.
	FindingTiersWithoutLevels = "check.tiers-without-levels"

	// FindingRequirementsWithoutTable names a workbench whose cards carry
	// tier requirements and which declares no tiers block. Those requirements
	// refuse nobody, and this is where the workbench learns it. The detail
	// carries how many cards carry a requirement, so a reader can tell a
	// workbench midway through adopting the table from one that never meant
	// to.
	FindingRequirementsWithoutTable = "check.requirements-without-table"

	// FindingModelListedTwice names one model entry listed under more than
	// one tier. It keys on the whole triple of provider, model and server, so
	// the same model name listed once for a hosted address and once with no
	// address is two entries rather than a duplicate.
	FindingModelListedTwice = "check.model-listed-twice"

	// FindingTiersEntryMalformed names one entry of the tiers block the
	// reader refused: a model entry missing provider or model, an entry
	// naming a member outside the three, or a tier carrying no meaning or no
	// models sequence.
	FindingTiersEntryMalformed = "check.tiers-entry-malformed"
)

// The eleven findings a declared route and a card naming one can produce. Each
// is reported rather than refused on read, on the posture the whole of this
// file keeps: a write catches a typo before it lands, and a read tolerates a
// declaration somebody typed. A workbench whose routes are nonsense opens, is
// read as it stands, and is reported.
const (
	// FindingRouteNameMalformed is a route name outside ColumnSlugPattern.
	// The name is typed on a command line and read back out of a refusal,
	// and renderBlock refuses to write a frontmatter key outside its own
	// character class, so a name outside the grammar is a name the file would
	// stop round-tripping.
	FindingRouteNameMalformed = "check.route-name-malformed"
	// FindingRouteEmpty is a route declaring no column at all.
	FindingRouteEmpty = "check.route-empty"
	// FindingRouteUnknownColumn is a route naming an identifier the workbench
	// does not declare.
	FindingRouteUnknownColumn = "check.route-unknown-column"
	// FindingRouteDuplicateColumn is a route naming one column twice.
	FindingRouteDuplicateColumn = "check.route-duplicate-column"
	// FindingRouteOutOfOrder is a route whose declaration lists its columns in
	// an order other than flow order. The route still resolves, because
	// RouteOf sorts, and the finding says the file does not read the way it
	// runs.
	FindingRouteOutOfOrder = "check.route-out-of-order"
	// FindingRouteMissingFirstColumn is a route that does not carry the
	// workbench's first column, whatever that column's kind. Cards enter the
	// workbench there, because that is where dinah add files one, so a route
	// without it has no door. The finding is named for the position rather
	// than for the kind intake, because the format permits a workbench whose
	// first column carries another kind and the rule is about the door.
	FindingRouteMissingFirstColumn = "check.route-missing-first-column"
	// FindingRouteMissingTerminal is a route whose last column is not of kind
	// done. A card has to be able to finish.
	FindingRouteMissingTerminal = "check.route-missing-terminal"
	// FindingRouteRejectTargetOffRoute is a column the route carries whose
	// reject_to names a column the route does not carry. It is a finding
	// rather than a refusal because the state it names is survivable: the
	// rejection lands the card there by an ordinary move and the card walks
	// forward into its route again. What the finding buys is that somebody
	// sees a road whose push-backs leave it.
	FindingRouteRejectTargetOffRoute = "check.route-reject-target-off-route"
	// FindingCardUnknownRoute is a card naming a route the workbench does not
	// declare. The card walks the default route meanwhile.
	FindingCardUnknownRoute = "check.card-unknown-route"
	// FindingCardOffRoute is a card standing at a column its own route does
	// not carry, which is worth a person's attention on a board where nobody
	// meant to produce it.
	FindingCardOffRoute = "check.card-off-route"
	// FindingItemOffRoute is a pending checklist item naming a column the
	// card's route does not carry, which is a hold that would never fire.
	FindingItemOffRoute = "check.item-off-route"
	// FindingCardRouteSkipsOperatorColumn is a card whose route omits a column
	// the workbench reserves to its operator, standing at or after the card's
	// own column. Every verb that writes a route refuses to produce it, so it
	// arrives by a hand edit or by an earlier build. Unlike the standing
	// finding section 13 of the specification rejected, it reads the card's
	// position rather than the route alone, so it clears once the card has
	// passed the column and never fires on a correct board.
	FindingCardRouteSkipsOperatorColumn = "check.card-route-skips-operator-column"
)

// The directions an interrupted structural act is reported and finished in.
// The journal decides between the first two, the same way history determines
// the present everywhere else in this format; the last two are the columns the
// tool reports and refuses to resolve.
const (
	// DirectionForward means the act was past its point of record, so the
	// finish completes the move or the removal.
	DirectionForward = "forward"
	// DirectionRollback means nothing observable happened, so the finish
	// rolls the sibling away and leaves the entity alone.
	DirectionRollback = "rollback"
	// DirectionMissing means the directory is at neither path.
	DirectionMissing = "missing"
	// DirectionLocked means a lock naming somebody other than the
	// interrupted act stands inside the directory, so a live process holds
	// it and the finish stops.
	DirectionLocked = "locked"
)

// Check checks a bench for the structural defects the format's invariants
// forbid and returns every one it finds. A clean bench returns no findings.
func (b *Bench) Check() ([]Finding, error) {
	var findings []Finding
	for _, path := range b.Passed {
		findings = append(findings, Finding{Path: path, Key: FindingIgnoredAnchor})
	}
	// A damaged workbench the walk met and resolved without is named here
	// because nothing else on the default path names it. The search answered
	// with a healthy sibling and is silent about the rest by design, and the
	// tree sweep that also reports it is an invocation a reader has to already
	// suspect something to run. Detail carries the directory rather than the
	// workbench.md inside it, matching the refusal for the same condition, so
	// that what the sentence prints is what a reader pastes after --workbench.
	for _, dir := range b.Damaged {
		findings = append(findings, Finding{Path: dir, Key: FindingDamagedWorkbench, Detail: dir})
	}
	cardIDs, err := ListIDs(b.CardsRoot())
	if err != nil {
		return findings, err
	}
	for _, id := range cardIDs {
		dir := filepath.Join(b.CardsRoot(), id)
		// A card a structural act is in the middle of belongs to the
		// interrupted-act report below rather than to the card walk, and
		// the sibling is what tells a half-removed directory from a
		// directory somebody deleted an anchor out of.
		if Exists(SiblingPath(dir)) {
			continue
		}
		if !Exists(filepath.Join(dir, CardAnchor)) {
			findings = append(findings, Finding{Path: dir, Key: FindingMissingAnchor, Detail: id})
			continue
		}
		card, err := b.LoadCardIn(b.CardsRoot(), id)
		if err != nil {
			findings = append(findings, Finding{Path: dir, Key: unreadableCardFinding(err), Detail: id})
			continue
		}
		cardFindings, err := b.checkCard(card)
		if err != nil {
			return findings, err
		}
		findings = append(findings, cardFindings...)
	}
	findings = append(findings, b.checkFieldDeclarations()...)
	newlineFindings, err := b.checkStoredNewlines()
	if err != nil {
		return findings, err
	}
	findings = append(findings, newlineFindings...)
	tierFindings, err := b.checkTierTable()
	if err != nil {
		return findings, err
	}
	findings = append(findings, tierFindings...)
	findings = append(findings, b.checkRequiredFields()...)
	findings = append(findings, b.checkColumnKinds()...)
	findings = append(findings, b.checkRejectTargets()...)
	findings = append(findings, b.checkRoutes()...)
	findings = append(findings, b.checkColumnLevels()...)
	findings = append(findings, b.checkColumnBodySizes()...)
	workstreamFindings, err := b.checkWorkstreams()
	if err != nil {
		return findings, err
	}
	findings = append(findings, workstreamFindings...)
	mountlessFindings, err := b.checkAttachmentsWithoutAMount()
	if err != nil {
		return findings, err
	}
	findings = append(findings, mountlessFindings...)
	benchOrdinalFindings, err := b.checkBenchOrdinals()
	if err != nil {
		return findings, err
	}
	findings = append(findings, benchOrdinalFindings...)
	numberFindings, err := b.checkCardNumbers()
	if err != nil {
		return findings, err
	}
	findings = append(findings, numberFindings...)
	findings = append(findings, b.checkColumnSlugs()...)
	findings = append(findings, b.checkWorkbenchSlug()...)
	for _, id := range b.StrandedColumns {
		findings = append(findings, Finding{Path: filepath.Join(b.Root, WorkbenchAnchor), Key: FindingStrandedColumn, Detail: id})
	}
	for _, id := range b.OrphanedColumnDirectories {
		findings = append(findings, Finding{Path: filepath.Join(b.Root, ColumnsDir, id), Key: FindingOrphanedColumnDirectory, Detail: id})
	}
	standing, err := b.interruptions()
	if err != nil {
		return findings, err
	}
	for _, interrupted := range standing {
		findings = append(findings, interrupted.finding())
	}
	return findings, nil
}

// checkAttachmentsWithoutAMount reports every attachments directory sitting
// below an entity whose kind mounts no attachments collection.
//
// The walk descends the containment table and asks MountOf at each entity
// rather than naming the kinds it expects, so a kind the table later gives an
// attachments mount stops being reported here with no second edit, and a kind
// added without one is covered from the day it exists.
//
// The workstreams are walked beside the table rather than through it, because a
// workstream is a membership rather than a container and the table deliberately
// leaves it out. The reference grammar reaches one all the same, so attach can
// be aimed at one and the collection has to be swept.
func (b *Bench) checkAttachmentsWithoutAMount() ([]Finding, error) {
	var findings []Finding
	for _, mount := range Contains(KindWorkbench) {
		dir := filepath.Join(b.Root, mount.Dir)
		ids, err := ListIDs(dir)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			below, err := b.mountlessAttachmentsBelow(filepath.Join(dir, id), mount.Kind)
			if err != nil {
				return nil, err
			}
			findings = append(findings, below...)
		}
	}
	root := b.WorkstreamsRoot()
	ids, err := ListIDs(root)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		below, err := b.mountlessAttachmentsBelow(filepath.Join(root, id), KindWorkstream)
		if err != nil {
			return nil, err
		}
		findings = append(findings, below...)
	}
	return findings, nil
}

// mountlessAttachmentsBelow visits one entity, and everything the containment
// table says hangs below it, reporting an attachments directory wherever the
// kind mounts none.
//
// A kind mounting no attachments is not a leaf of the grammar: it may still
// mount some other collection, a checklist item's own comments among them,
// and a stray below one of those members is exactly as much a stray as one
// sitting directly under the kind that mounts none. So the walk reports what
// it finds at this entity and then descends over every collection Contains
// names for this kind regardless of whether this kind itself mounts
// attachments, rather than stopping the moment attachments is the collection
// missing. Stopping at the first kind mounting no attachments was the
// defect: it left everything below a checklist item unreached the day an
// item gained a comments collection of its own.
func (b *Bench) mountlessAttachmentsBelow(dir, kind string) ([]Finding, error) {
	var findings []Finding
	if _, mounts := MountOf(kind, AttachmentsDir); !mounts {
		attachments := filepath.Join(dir, AttachmentsDir)
		if Exists(attachments) {
			findings = append(findings, Finding{Path: attachments, Key: FindingAttachmentsWithoutAMount, Detail: kind})
		}
	}
	for _, mount := range Contains(kind) {
		collection := filepath.Join(dir, mount.Dir)
		ids, err := ListIDs(collection)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			below, err := b.mountlessAttachmentsBelow(filepath.Join(collection, id), mount.Kind)
			if err != nil {
				return nil, err
			}
			findings = append(findings, below...)
		}
	}
	return findings, nil
}

// checkColumnKinds applies the position rules to the flow and reports a column
// carrying a kind this build does not implement.
//
// The rules are three. An intake column stands first, so at most one column is
// of that kind. A done column stands in the terminal region, which is the run
// of columns at the end of the list every member of which is of kind done, and
// no column that is not done stands after a column that is. A buffer stands
// neither first nor in the terminal region.
//
// Every rule is reported rather than refused. A board whose kinds sit outside
// these positions opens and is read as it stands, because refusing it on read
// would leave it unopenable the moment somebody reorders a flow, and the acts
// that would create the condition afresh are refused on their own.
func (b *Bench) checkColumnKinds() []Finding {
	var findings []Finding
	start := b.terminalRegionStart()
	for _, column := range b.Columns {
		path := b.ColumnAnchorPath(column.ID)
		if strings.Contains(column.Kind, ".") && column.Kind != contract.KindBuffer {
			findings = append(findings, Finding{Path: path, Key: FindingUnknownKind, Detail: column.Ref()})
			continue
		}
		if b.kindStandsWrong(column, start) {
			findings = append(findings, Finding{Path: path, Key: FindingKindOutOfPosition, Detail: column.Ref()})
		}
	}
	return findings
}

// checkRejectTargets reports a declared reject_to this build cannot cleanly
// act on: one naming no column, one naming its own column, and one naming a
// column ahead of it in the flow whose kind is not done. It resolves each
// declaration for itself rather than calling RejectTarget, because
// RejectTarget answers one nil for all three conditions and a reader telling a
// person what to fix needs to tell them which one applies. Whether the target
// is a done column is asked of Column.Terminal rather than compared here, which
// is the one answer dinah-273 left for that question.
//
// A forward declaration landing in a done column is not reported. A rejected
// card ends where a finished card ends, carrying its own outcome, so naming
// the terminal is a thing a board may legitimately want to say.
func (b *Bench) checkRejectTargets() []Finding {
	var findings []Finding
	for _, column := range b.Columns {
		if column.RejectTo == "" {
			continue
		}
		path := b.ColumnAnchorPath(column.ID)
		target := b.ColumnByRef(column.RejectTo)
		switch {
		case target == nil:
			findings = append(findings, Finding{Path: path, Key: FindingRejectTargetUnknown, Detail: column.Ref()})
		case target.ID == column.ID:
			findings = append(findings, Finding{Path: path, Key: FindingRejectTargetIsSelf, Detail: column.Ref()})
		case target.Position > column.Position && !target.Terminal():
			findings = append(findings, Finding{Path: path, Key: FindingRejectTargetForward, Detail: column.Ref()})
		}
	}
	return findings
}

// checkColumnLevels reports a column whose own tier default names no member
// of the workbench's declared tier set, which a hand-edited column.md and a
// later edit to the workbench's levels declaration both produce.
//
// It is worth a human's attention because a stale default silently breaks a
// relative override write at that column, and because a reader answering "what
// class of worker does the work here need" would be misled by it. It is never
// worth more than a report: the claim gate does not read a column's tier
// default at all, so a stale one cannot make a claim behave differently from a
// column carrying no default whatsoever.
func (b *Bench) checkColumnLevels() []Finding {
	var findings []Finding
	for _, column := range b.Columns {
		if column.Tier == "" || b.Level(TierField, column.Tier) != nil {
			continue
		}
		findings = append(findings, Finding{
			Path:   b.ColumnAnchorPath(column.ID),
			Key:    FindingUnknownLevel,
			Detail: TierField + "@" + column.Ref() + " " + column.Tier,
		})
	}
	return findings
}

// checkColumnBodySizes measures each declared column's body against the
// column_body_limit the workbench declares, in bytes of the body as the anchor
// parser returns it, which is the text every arrival at the column is served.
// The frontmatter is not counted, because it is never served. An absent
// declaration measures nothing, and a declaration that is not a whole number
// above zero is reported in place of any measurement.
func (b *Bench) checkColumnBodySizes() []Finding {
	if b.FM == nil || !b.FM.Has(ColumnBodyLimitField) {
		return nil
	}
	stored := b.FM.Value(ColumnBodyLimitField)
	limit, err := strconv.Atoi(stored)
	if err != nil || limit < 1 {
		malformed := Finding{
			Path:   filepath.Join(b.Root, WorkbenchAnchor),
			Key:    FindingColumnBodyLimitMalformed,
			Detail: stored,
		}
		return []Finding{malformed}
	}
	var findings []Finding
	for _, column := range b.Columns {
		size := len(column.Instructions)
		if size <= limit {
			continue
		}
		detail := column.Ref() + " " + strconv.Itoa(size) + "/" + strconv.Itoa(limit)
		findings = append(findings, Finding{
			Path:     b.ColumnAnchorPath(column.ID),
			Key:      FindingColumnBodyOverLimit,
			Detail:   detail,
			Severity: SeverityCleanup,
		})
	}
	return findings
}

// checkTierOverrides reports what a card's tier_at entries say that this
// workbench cannot act on: an entry naming no column it carries, and an entry
// whose tier names no member of the declared tier set.
//
// It mirrors checkRejectTargets rather than refusing on read, for the reason
// that function gives: a reshape can retire a column long after somebody wrote
// the override, so a reader has to tolerate a reference that no longer
// resolves. The write path is where a typo is caught.
func (b *Bench) checkTierOverrides(card *Card) []Finding {
	var findings []Finding
	anchor := card.AnchorPath()
	for _, override := range card.ColumnTiers {
		if b.ColumnByRef(override.Column) == nil {
			findings = append(findings, Finding{
				Path:   anchor,
				Key:    FindingUnknownTierColumn,
				Detail: card.Ref(b.Slug) + " " + override.Column,
			})
		}
		if override.Tier != "" && b.Level(TierField, override.Tier) == nil {
			findings = append(findings, Finding{
				Path:   anchor,
				Key:    FindingUnknownLevel,
				Detail: TierField + "@" + override.Column + " " + override.Tier,
			})
		}
	}
	return findings
}

// checkItemColumns reports every checklist item of a card whose column value
// cannot hold the card anywhere: a value resolving to no column, and a value
// resolving to a column by its slug or its title rather than by the identifier
// GatingItems compares against. An item filed without a column names nothing
// and is passed over.
//
// It reads rather than repairs, as checkTierOverrides does and as everything
// else in this file does. The repair is a write through the field, which
// resolves the spelling it is given and refuses one that resolves to nothing.
func (b *Bench) checkItemColumns(card *Card) ([]Finding, error) {
	var findings []Finding
	named, err := itemsWhere(card.Dir, func(item *Item) bool {
		return item.Column != ""
	})
	if err != nil {
		return nil, err
	}
	for _, item := range named {
		resolved := b.ColumnByRef(item.Column)
		if resolved != nil && resolved.ID == item.Column {
			continue
		}
		findings = append(findings, Finding{
			Path:   filepath.Join(item.Dir, ItemAnchor),
			Key:    FindingItemColumnUnresolved,
			Detail: card.Ref(b.Slug) + " " + item.ID + " " + item.Column,
		})
	}
	return findings, nil
}

// kindStandsWrong reports whether one column's kind is disallowed at the
// position the column stands in, given where the terminal region starts.
func (b *Bench) kindStandsWrong(column *Column, terminalStart int) bool {
	switch column.Kind {
	case contract.KindIntake:
		return column.Position != 0
	case contract.KindDone:
		return column.Position < terminalStart
	case contract.KindBuffer:
		return column.Position == 0 || column.Position >= terminalStart
	}
	return false
}

// terminalRegionStart returns the position the terminal region begins at,
// which is the length of the flow when the flow ends in a column that is not
// done. Walking back from the end is what makes the region a run rather than
// a set: a done column with anything but done columns after it lies outside it.
func (b *Bench) terminalRegionStart() int {
	start := len(b.Columns)
	for i := len(b.Columns) - 1; i >= 0; i-- {
		if b.Columns[i].Kind != contract.KindDone {
			break
		}
		start = i
	}
	return start
}

// checkCard applies every card-level invariant to one card.
func (b *Bench) checkCard(card *Card) ([]Finding, error) {
	var findings []Finding
	anchor := card.AnchorPath()
	claimed := card.Holder != "" || card.ClaimSince != ""
	switch card.State {
	case contract.StateActive:
		if !claimed {
			findings = append(findings, Finding{Path: anchor, Key: FindingActiveWithoutClaim, Detail: card.ID})
		}
	case contract.StateBlocked:
		if card.BlockReason == "" {
			findings = append(findings, Finding{Path: anchor, Key: FindingBlockWithoutReason, Detail: card.ID})
		}
		if claimed {
			findings = append(findings, Finding{Path: anchor, Key: FindingHolderOnUnheld, Detail: card.ID})
		}
	case contract.StateReady:
		if claimed {
			findings = append(findings, Finding{Path: anchor, Key: FindingClaimWithoutActive, Detail: card.ID})
		}
	default:
		findings = append(findings, Finding{Path: anchor, Key: FindingUnknownState, Detail: card.State})
	}
	column := b.Column(card.Column)
	if column == nil {
		findings = append(findings, Finding{Path: anchor, Key: FindingUnknownColumn, Detail: card.Column})
	}
	// A claim standing where no owner takes work up is history rather than
	// something a card acquires afresh, since claim, move and pull all refuse
	// to put one there. The finding names the column, because that is what an
	// operator edits or moves the card out of.
	held := claimed || card.State == contract.StateActive
	if column != nil && held && !column.TakesWorkUp() {
		findings = append(findings, Finding{Path: anchor, Key: FindingClaimWhereNoWorkIsTaken, Detail: column.Ref()})
	}
	// Each axis is asked about its own declaration and never about whether
	// the workbench declares any levels at all, so a card carrying a
	// severity this workbench declares and a priority it does not is
	// reported once, over the priority.
	for _, axis := range LevelAxes {
		stored := card.LevelOf(axis)
		if stored == "" || b.Level(axis, stored) != nil {
			continue
		}
		findings = append(findings, Finding{Path: anchor, Key: FindingUnknownLevel, Detail: axis + " " + stored})
	}
	// The heading is reported only on a workbench the retirement has reached,
	// which is what the format number is for here and the whole of what it
	// gates: a declared field works on a workbench declaring any format this
	// build opens.
	if b.Format >= FieldsFormat && CarriesBranchHeading(card.Body) {
		findings = append(findings, Finding{Path: anchor, Key: FindingBranchHeadingInBody, Detail: card.ID})
	}
	findings = append(findings, b.checkTierOverrides(card)...)
	findings = append(findings, b.checkCardRoute(card)...)
	itemColumnFindings, err := b.checkItemColumns(card)
	if err != nil {
		return findings, err
	}
	findings = append(findings, itemColumnFindings...)
	itemRouteFindings, err := b.checkItemRoutes(card)
	if err != nil {
		return findings, err
	}
	findings = append(findings, itemRouteFindings...)
	// A card carrying no registry line is checkCardNumbers' finding rather
	// than this walk's, because the line lives in the registry file rather
	// than in the anchor this walk reads, and a workbench the migration has
	// not reached owes one finding per card there.
	for _, link := range card.Links {
		if b.HasIdentifier(link.To) {
			continue
		}
		findings = append(findings, Finding{Path: anchor, Key: FindingDanglingLink, Detail: link.To})
	}
	for _, id := range card.Workstreams {
		if b.HasWorkstream(id) {
			continue
		}
		findings = append(findings, Finding{Path: anchor, Key: FindingDanglingWorkstream, Detail: id})
	}
	ordinalFindings, err := checkOrdinals(card.Dir)
	if err != nil {
		return findings, err
	}
	findings = append(findings, ordinalFindings...)
	filenameFindings, err := checkAttachmentFilename(card.Dir)
	if err != nil {
		return findings, err
	}
	findings = append(findings, filenameFindings...)
	commentFindings, err := b.checkComments(card)
	if err != nil {
		return findings, err
	}
	findings = append(findings, commentFindings...)
	noteFindings, err := b.checkRetiredNotes(card)
	if err != nil {
		return findings, err
	}
	findings = append(findings, noteFindings...)
	events, torn, err := ReadJournal(card.JournalPath())
	if err != nil {
		return findings, nil
	}
	if torn {
		findings = append(findings, Finding{Path: card.JournalPath(), Key: FindingTornJournal, Detail: card.ID})
	}
	if position := ReplayPosition(events); position != "" && position != card.Column {
		findings = append(findings, Finding{Path: anchor, Key: FindingPositionDiverges, Detail: position})
	}
	// The count is read off the events this function has already read, so a
	// card standing at a column declaring no limit costs nothing and one
	// standing at a declaring column costs a walk of a slice already in hand.
	if column != nil && column.LoopLimit > 0 {
		if count := b.RegressiveDepartures(events, column.ID); count >= column.LoopLimit {
			findings = append(findings, Finding{Path: anchor, Key: FindingAtLoopLimit, Detail: column.Ref()})
		}
	}
	return findings, nil
}

// RegressiveDepartures counts how many times a card has left one column by a
// regressive move: a moved event whose from is columnID and whose to names a
// column this workbench still declares, standing earlier than columnID and not
// of kind done.
//
// A manual_correction is never counted. ReplayPosition treats one as a
// position update, because the witness records where the anchor already
// stands, but nobody chose that transition and a limit on what agents and the
// operator do has nothing to say about a repair.
//
// The count is derived from the events against the workbench's current column
// order, the same basis ReplayPosition reads against, so a column reordered or
// repositioned changes this answer exactly as it changes that one. A departure
// whose to no longer resolves to a live column is not counted, because there
// is no current position left to compare it against: an unresolvable reference
// reads as nothing to say here, the way Bench.RejectTarget and
// checkRejectTargets already read one.
func (b *Bench) RegressiveDepartures(events []Event, columnID string) int {
	count := 0
	for _, ev := range events {
		if ev.Event != contract.EventMoved || ev.From != columnID {
			continue
		}
		from := b.Column(ev.From)
		to := b.Column(ev.To)
		if from == nil || to == nil || to.Terminal() {
			continue
		}
		if to.Position < from.Position {
			count++
		}
	}
	return count
}

// checkOrdinals applies the creation-ordinal invariants to every collection
// below one card: each entity carries a positive ordinal, and no two entities
// of one collection carry the same one.
//
// A gap in a sequence is not reported. Deletion is directory removal, so an
// ordinal disappears with the entity that carried it, and the value a
// survivor carries stays a record of where it fell in the write order: a
// deleted neighbour does not change where that was, so closing the gap would
// rewrite a historical fact on entities nobody touched. A duplicate is
// reported because it leaves a position with two answers.
func checkOrdinals(cardDir string) ([]Finding, error) {
	collections, err := ordinalCollections(cardDir, KindCard, nil)
	if err != nil {
		return nil, err
	}
	return ordinalFindings(collections)
}

// checkBenchOrdinals applies the ordinal invariants to the collections a
// positional reference reaches below the workbench itself and below each
// column. Cards are not descended into, because checkCard sweeps each card
// under the per-card guards a workbench-rooted walk does not apply.
//
// The collections it covers are the workbench's own attachments, each
// column's comments and attachments, and the attachments of each column
// comment. A collection whose members carry no ordinal, which is the
// workbench's columns and its cards, is outside the sweep because the mount
// says so rather than because this function names the kinds.
func (b *Bench) checkBenchOrdinals() ([]Finding, error) {
	collections, err := ordinalCollections(b.Root, KindWorkbench, map[string]bool{KindCard: true})
	if err != nil {
		return nil, err
	}
	return ordinalFindings(collections)
}

// ordinalFindings applies the two invariants to a list of collections. Both
// rooted sweeps share it, so the invariants are stated once and a collection
// reached from the workbench is judged exactly as one reached from a card is.
func ordinalFindings(collections []ordinalCollection) ([]Finding, error) {
	var findings []Finding
	for _, collection := range collections {
		seen := map[int]bool{}
		ids, err := ListIDs(collection.dir)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			path := filepath.Join(collection.dir, id, collection.anchor)
			if !Exists(path) {
				continue
			}
			ordinal := EntityOrdinal(collection.dir, id, collection.anchor)
			if ordinal == 0 {
				findings = append(findings, Finding{Path: path, Key: FindingOrdinalMissing, Detail: id})
				continue
			}
			if seen[ordinal] {
				findings = append(findings, Finding{Path: path, Key: FindingOrdinalDuplicate, Detail: id})
				continue
			}
			seen[ordinal] = true
		}
	}
	return findings, nil
}

// checkAttachmentFilename reports every attachment whose anchor's filename
// disagrees with the name of the file in its payload directory. The two
// names agree on every rename the verb completes, and on every payload the
// attach verb lays down, so a drift is the residue of a crash between the
// two writes, or of a hand edit nobody noticed.
func checkAttachmentFilename(cardDir string) ([]Finding, error) {
	collection := filepath.Join(cardDir, AttachmentsDir)
	var findings []Finding
	ids, err := ListIDs(collection)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		dir := filepath.Join(collection, id)
		anchor := filepath.Join(dir, AttachmentAnchor)
		if !Exists(anchor) {
			continue
		}
		fm, _ := loadAnchor(anchor)
		wanted := fm.Value("filename")
		if wanted == "" {
			continue
		}
		payload := filepath.Join(dir, PayloadDir)
		entries, err := readCollection(payload)
		if err != nil {
			return nil, err
		}
		if len(entries) == 0 {
			continue
		}
		if entries[0].Name() == wanted {
			continue
		}
		findings = append(findings, Finding{Path: anchor, Key: FindingAttachmentFilenameDrift, Detail: id})
	}
	return findings, nil
}

// checkCardNumbers reports the six states a card-number registry can be left
// in. Four are states of lines: two lines claiming one number, two lines
// claiming one identifier, a line naming no card, and a line the grammar
// refuses. Two are states of cards: a card no line claims, and a card on a
// migrated workbench still carrying its number in frontmatter. A finding
// about a line names the registry file and carries the line as stored, which
// is what a reader searches the file for, and a finding about a card names
// the card's anchor and carries the identifier, which is what a reader opens.
//
// A workbench below the format the registry arrived at holds an empty
// registry, so the four line findings stay quiet over it and the missing
// finding floods one per card. The flood is the intended signal rather than a
// defect of the detector, because the migration is the repair and a
// workbench waiting for it is exactly what one finding per card says. The
// by-number index the pre-registry reader synthesizes from frontmatter feeds
// resolution alone, and none of these findings read it, so a synthesized
// workbench reports its cards as carrying no lines rather than as holding
// duplicates.
//
// The stranded probe reads every well-formed line's identifier against the two
// collections, and a directory that stands but will not load is reported
// through unreadableCardFinding rather than as stranded, because the defect
// there is the card rather than the line. The probe reaches for the free
// reader rather than the stamping one, because the question is whether the
// directory reads as a card at all, and stamping the answer would read it
// through the very index the line being probed is about to be weighed
// against. The live half is left to Bench.Check's own card walk, which
// already reports every live directory that will not load, so a probe here
// would name each one twice. The walk below carries the archived half of the
// same property: an archived card whose anchor will not load is reported
// rather than passed over, which is the dinah-439 property this detector has
// always owed, and the probe and the walk partition that half between them,
// since the probe owns the cards a line claims and the walk owns the rest.
//
// A collection ListIDs cannot walk ends the check with an error, and the
// findings gathered before it survive on the error's back, on the terms every
// collection read in this package answers its own failure.
func (b *Bench) checkCardNumbers() ([]Finding, error) {
	var findings []Finding
	file := filepath.Join(b.Root, CardNumbersName)
	// The repeated-identifier finding needs a count per identifier, and the
	// registry's own indexes answer every other question this walk asks. The
	// count is built in a pass of its own because a group a later line joins
	// is a group its earlier members already belonged to.
	claimants := map[string]int{}
	for _, line := range b.Numbers.Lines {
		if line.Number == 0 || line.ID == "-" {
			continue
		}
		claimants[line.ID]++
	}
	for _, line := range b.Numbers.Lines {
		if line.Number == 0 {
			// A malformed line enters neither index, so it collides with
			// nothing, strands nothing, and is named for its own bytes
			// alone.
			findings = append(findings, Finding{Path: file, Key: FindingCardNumberMalformed, Detail: line.Raw})
			continue
		}
		if line.ID != "-" {
			if len(b.Numbers.ByNumber[line.Number]) >= 2 {
				findings = append(findings, Finding{Path: file, Key: FindingCardNumberDuplicate, Detail: line.Raw})
			}
			if claimants[line.ID] >= 2 {
				findings = append(findings, Finding{Path: file, Key: FindingCardNumberRepeated, Detail: line.Raw})
			}
		}
		// A tombstone is a number somebody gave back, so it claims no card
		// and no card claims it, and the missing and stranded findings are
		// both built to pass it over.
		if line.ID == "-" {
			continue
		}
		if Exists(filepath.Join(b.CardsRoot(), line.ID)) {
			// The live half's unreadable directories are Bench.Check's own
			// card walk's report, and a probe here would name each one
			// twice.
			continue
		}
		archived := filepath.Join(b.ArchivedCardsRoot(), line.ID)
		if !Exists(archived) {
			findings = append(findings, Finding{Path: file, Key: FindingCardNumberStranded, Detail: line.Raw})
			continue
		}
		if _, err := LoadCard(b.ArchivedCardsRoot(), line.ID); err != nil {
			findings = append(findings, Finding{Path: archived, Key: unreadableCardFinding(err), Detail: line.ID})
		}
	}
	for _, root := range []string{b.CardsRoot(), b.ArchivedCardsRoot()} {
		ids, err := ListIDs(root)
		if err != nil {
			return findings, err
		}
		archived := root == b.ArchivedCardsRoot()
		for _, id := range ids {
			dir := filepath.Join(root, id)
			// The guards mirror Bench.Check's own card walk, so a card a
			// structural act is in the middle of and a directory carrying
			// no anchor belong to the reports those states already have
			// rather than to a finding about numbers.
			if Exists(SiblingPath(dir)) {
				continue
			}
			anchor := filepath.Join(dir, CardAnchor)
			if !Exists(anchor) {
				continue
			}
			_, claimed := b.Numbers.ByID[id]
			card, loadErr := b.LoadCardIn(root, id)
			if loadErr != nil {
				// The card the walk cannot load is the card the line
				// findings have not reached, so the missing finding would
				// name an anchor nobody can open. The unreadable finding is
				// the honest report, and the cards a line claims are the
				// probe's half of the partition above. The live half is
				// Bench.Check's own card walk's report either way.
				if archived && !claimed {
					findings = append(findings, Finding{Path: dir, Key: unreadableCardFinding(loadErr), Detail: id})
				}
				continue
			}
			if !claimed {
				findings = append(findings, Finding{Path: anchor, Key: FindingCardNumberMissing, Detail: id})
			}
			if b.Format >= RegistryFormat && card.FM.Has("number") {
				findings = append(findings, Finding{Path: anchor, Key: FindingCardNumberInFrontmatter, Detail: id})
			}
		}
	}
	return findings, nil
}

// ReplayPosition returns the column the journal says a card occupies, which is
// the column of its last recorded move or witnessed correction, or the column
// it was created in when it has never left one. An empty answer means the
// journal says nothing about position at all, which is not itself a
// divergence.
//
// A manual_correction line counts alongside a move because the witness records
// the position the anchor already holds, so replay tracks the last recorded
// position rather than the last position somebody chose. That is what stops a
// witnessed card from being reported again the instant after it is witnessed,
// and it is also what keeps a second hand edit after a witness detectable.
func ReplayPosition(events []Event) string {
	position := ""
	for _, ev := range events {
		switch ev.Event {
		case contract.EventCreated:
			position = ev.To
		case contract.EventMoved:
			position = ev.To
		case contract.EventManualCorrection:
			position = ev.To
		}
	}
	return position
}

// unreadableCardFinding names the defect a card the reader refused actually
// has. The reader owns the vocabulary conditions, because a card whose keys
// mean the wrong thing cannot be read at all; the checker's job is to say so
// in its own report rather than to translate every refusal into the one
// finding it had a key for. Anything else the reader refuses is still a
// directory the checker cannot make a card out of, which is what
// FindingMissingAnchor says.
func unreadableCardFinding(err error) string {
	var refusal *contract.Refusal
	if !errors.As(err, &refusal) {
		return FindingMissingAnchor
	}
	switch refusal.Name {
	case contract.VocabularyMixed:
		return FindingCardVocabularyMixed
	case contract.VocabularyRetired:
		return FindingCardVocabularyRetired
	}
	return FindingMissingAnchor
}

// checkFieldDeclarations reports every entry of the workbench's fields block
// the declaration reader refused. The reader has already skipped them, so this
// is where a person meets the damage, and the finding names the key rather
// than counting the entries because a count tells nobody which line to open.
func (b *Bench) checkFieldDeclarations() []Finding {
	var findings []Finding
	anchor := filepath.Join(b.Root, WorkbenchAnchor)
	for _, key := range b.malformedFields {
		findings = append(findings, Finding{Path: anchor, Key: FindingFieldDeclarationMalformed, Detail: key})
	}
	return findings
}

// checkTierTable reports the five defects a workbench's tier table can carry.
//
// Each sweep counts what it examined rather than reporting only what it found,
// because a sweep that reads nothing reports success and reads exactly like a
// sweep that found nothing wrong. The counts travel in the findings' own
// details where a reader needs them and in the tests otherwise.
func (b *Bench) checkTierTable() ([]Finding, error) {
	anchor := filepath.Join(b.Root, WorkbenchAnchor)
	var findings []Finding
	declaresTable := b.FM.Has(TiersKey)
	rungs := b.Levels(TierField)
	for _, entry := range b.malformedTiers {
		findings = append(findings, Finding{Path: anchor, Key: FindingTiersEntryMalformed, Detail: entry.Detail()})
	}
	if declaresTable && len(rungs) == 0 {
		findings = append(findings, Finding{Path: anchor, Key: FindingTiersWithoutLevels})
	}
	if declaresTable {
		listed := map[string]bool{}
		for _, entry := range b.tiers {
			if len(entry.Models) > 0 {
				listed[entry.Tier] = true
			}
		}
		for _, rung := range rungs {
			if listed[rung.Name] {
				continue
			}
			findings = append(findings, Finding{Path: anchor, Key: FindingTierWithoutModels, Detail: rung.Name})
		}
		findings = append(findings, b.checkDuplicateModels(anchor)...)
	}
	if !declaresTable {
		requiring, err := b.cardsRequiringATier()
		if err != nil {
			return findings, err
		}
		if requiring > 0 {
			findings = append(findings, Finding{
				Path:   anchor,
				Key:    FindingRequirementsWithoutTable,
				Detail: strconv.Itoa(requiring),
			})
		}
	}
	return findings, nil
}

// checkDuplicateModels reports one model entry listed under more than one
// tier, keyed on the whole triple of provider, model and server. The first
// occurrence wins, and the finding names both tiers so the duplicate can be
// removed from the wrong one.
func (b *Bench) checkDuplicateModels(anchor string) []Finding {
	var findings []Finding
	first := map[string]string{}
	for _, entry := range b.tiers {
		for _, model := range entry.Models {
			triple := model.Render()
			held, seen := first[triple]
			if !seen {
				first[triple] = entry.Tier
				continue
			}
			if held == entry.Tier {
				continue
			}
			findings = append(findings, Finding{
				Path:   anchor,
				Key:    FindingModelListedTwice,
				Detail: triple + " " + held + " " + entry.Tier,
			})
		}
	}
	return findings
}

// cardsRequiringATier counts the live cards carrying a tier requirement,
// either as their own baseline or as a per-column override.
//
// The walk is Check's own rather than Bench.Cards, which refuses the whole read
// over one card it cannot load. A card a structural act is in the middle of, a
// card with no anchor and a card the reader refuses are each stepped over here
// exactly as the card walk above steps over them, because a workbench carrying
// one damaged card still deserves to be told that its tier requirements refuse
// nobody.
func (b *Bench) cardsRequiringATier() (int, error) {
	ids, err := ListIDs(b.CardsRoot())
	if err != nil {
		return 0, err
	}
	requiring := 0
	for _, id := range ids {
		dir := filepath.Join(b.CardsRoot(), id)
		if Exists(SiblingPath(dir)) || !Exists(filepath.Join(dir, CardAnchor)) {
			continue
		}
		card, err := b.LoadCardIn(b.CardsRoot(), id)
		if err != nil {
			continue
		}
		if card.Tier != "" {
			requiring++
			continue
		}
		for _, override := range card.ColumnTiers {
			if override.Tier != "" {
				requiring++
				break
			}
		}
	}
	return requiring, nil
}

// checkRequiredFields reports every require_fields entry naming a key the
// workbench does not declare. The detail carries the column's reference and
// then the key, in that order, which is the shape FindingUnknownTierColumn
// already uses for a finding about a column and a value together.
func (b *Bench) checkRequiredFields() []Finding {
	var findings []Finding
	for _, column := range b.Columns {
		for _, key := range column.RequireFields {
			if b.DeclaredFieldOf(key) != nil {
				continue
			}
			findings = append(findings, Finding{
				Path:   filepath.Join(b.Root, ColumnsDir, column.ID, ColumnAnchor),
				Key:    FindingRequiredFieldUndeclared,
				Detail: column.Ref() + " " + key,
			})
		}
	}
	return findings
}
