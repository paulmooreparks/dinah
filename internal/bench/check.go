package bench

import (
	"errors"
	"os"
	"path/filepath"
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
	FindingSlugMissing        = "check.slug-missing"
	FindingSlugMalformed      = "check.slug-malformed"
	FindingSlugDuplicate      = "check.slug-duplicate"
	FindingStrandedColumn     = "check.stranded-column"
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
	// FindingUnknownLevel names a card whose stored severity or priority is
	// no member of what this workbench declares for that axis, which covers
	// a declaration that has since changed and one that was never made. It
	// sits beside FindingUnknownColumn for the same reason: the write path is
	// the only place a level is validated, because refusing on read would
	// make a workbench unreadable the moment somebody edits its declaration.
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
	// FindingWorkbenchSlugMalformed names a stored workbench slug that does
	// not conform to the grammar. Open validates the stored slug at no major,
	// so a slug written by hand reaches the checker rather than being
	// pre-empted by a refusal, and the workbench still opens while it is
	// reported.
	FindingWorkbenchSlugMalformed = "check.workbench-slug-malformed"
	// The last seven are raised by a repair rather than by the checker,
	// because each names something only the run that did the work can know:
	// which entity it placed by guesswork, which card a lock kept it out of,
	// which entity it could not write to, which title it could derive no
	// slug from, and which column or workbench anchor it could not write a
	// slug to. None of them survives on disk for a later check to find.
	FindingOrdinalGuessed           = "check.ordinal-guessed"
	FindingOrdinalLocked            = "check.ordinal-locked"
	FindingOrdinalUnwritable        = "check.ordinal-unwritable"
	FindingSlugUnderivable          = "check.slug-underivable"
	FindingSlugUnwritable           = "check.slug-unwritable"
	FindingWorkbenchSlugUnderivable = "check.workbench-slug-underivable"
	// FindingWitnessLocked names a card the witness repair could not reach,
	// because a lock stood on it while the walk passed. The walk stepped over
	// it and carried on, so the card stays diverged until the repair is run
	// again or a touch that reads its position reaches it.
	FindingWitnessLocked = "check.witness-locked"
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
	for _, id := range ListIDs(b.CardsRoot()) {
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
		card, err := LoadCard(b.CardsRoot(), id)
		if err != nil {
			findings = append(findings, Finding{Path: dir, Key: unreadableCardFinding(err), Detail: id})
			continue
		}
		findings = append(findings, b.checkCard(card)...)
	}
	findings = append(findings, b.checkColumnKinds()...)
	findings = append(findings, b.checkRejectTargets()...)
	findings = append(findings, b.checkWorkstreams()...)
	findings = append(findings, b.checkColumnSlugs()...)
	findings = append(findings, b.checkWorkbenchSlug()...)
	for _, id := range b.StrandedColumns {
		findings = append(findings, Finding{Path: filepath.Join(b.Root, WorkbenchAnchor), Key: FindingStrandedColumn, Detail: id})
	}
	for _, id := range b.OrphanedColumnDirectories {
		findings = append(findings, Finding{Path: filepath.Join(b.Root, ColumnsDir, id), Key: FindingOrphanedColumnDirectory, Detail: id})
	}
	for _, standing := range b.interruptions() {
		findings = append(findings, standing.finding())
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
func (b *Bench) checkCard(card *Card) []Finding {
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
	if card.Number == 0 {
		findings = append(findings, Finding{Path: anchor, Key: FindingOrdinalMissing, Detail: card.ID})
	}
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
	findings = append(findings, checkOrdinals(card.Dir)...)
	findings = append(findings, checkAttachmentFilename(card.Dir)...)
	events, torn, err := ReadJournal(card.JournalPath())
	if err != nil {
		return findings
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
	return findings
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
func checkOrdinals(cardDir string) []Finding {
	var findings []Finding
	for _, collection := range ordinalCollections(cardDir) {
		seen := map[int]bool{}
		for _, id := range ListIDs(collection.dir) {
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
	return findings
}

// checkAttachmentFilename reports every attachment whose anchor's filename
// disagrees with the name of the file in its payload directory. The two
// names agree on every rename the verb completes, and on every payload the
// attach verb lays down, so a drift is the residue of a crash between the
// two writes, or of a hand edit nobody noticed.
func checkAttachmentFilename(cardDir string) []Finding {
	collection := filepath.Join(cardDir, AttachmentsDir)
	var findings []Finding
	for _, id := range ListIDs(collection) {
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
		entries, err := os.ReadDir(payload)
		if err != nil || len(entries) == 0 {
			continue
		}
		if entries[0].Name() == wanted {
			continue
		}
		findings = append(findings, Finding{Path: anchor, Key: FindingAttachmentFilenameDrift, Detail: id})
	}
	return findings
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
