package bench

import (
	"path/filepath"

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
	// Detail is the identifier, state or field the defect is about.
	Detail string
}

// The catalog keys check reports its findings under. Each names one invariant
// the format document states.
const (
	FindingClaimWithoutActive = "check.claim-without-active"
	FindingActiveWithoutClaim = "check.active-without-claim"
	FindingBlockWithoutReason = "check.block-without-reason"
	FindingHolderOnUnheld     = "check.holder-on-unheld"
	FindingUnknownState       = "check.unknown-state"
	FindingDanglingLink       = "check.dangling-link"
	FindingPositionDiverges   = "check.position-diverges"
	FindingMissingAnchor      = "check.missing-anchor"
	FindingTornJournal        = "check.torn-journal"
	FindingUnknownSubstate    = "check.unknown-substate"
	FindingInterruptedAct     = "check.interrupted-act"
	FindingEntityAtBothPaths  = "check.entity-at-both-paths"
	FindingOrdinalMissing     = "check.ordinal-missing"
	FindingOrdinalDuplicate   = "check.ordinal-duplicate"
	FindingSlugMissing        = "check.slug-missing"
	FindingSlugMalformed      = "check.slug-malformed"
	FindingSlugDuplicate      = "check.slug-duplicate"
	FindingIgnoredAnchor      = "check.ignored-anchor"
	// FindingWorkbenchSlugMissing names a workbench written before the
	// workbench-level slug field existed, on the same report-only terms
	// FindingSlugMissing already reports a state's absence.
	FindingWorkbenchSlugMissing = "check.workbench-slug-missing"
	// The last six are raised by a migration rather than by the checker,
	// because each names something only the run that did the work can know:
	// which entity it placed by guesswork, which card a lock kept it out of,
	// which entity it could not write to, which title it could derive no
	// slug from, and which state or workbench anchor it could not write a
	// slug to. None of them survives on disk for a later check to find.
	FindingOrdinalGuessed           = "check.ordinal-guessed"
	FindingOrdinalLocked            = "check.ordinal-locked"
	FindingOrdinalUnwritable        = "check.ordinal-unwritable"
	FindingSlugUnderivable          = "check.slug-underivable"
	FindingSlugUnwritable           = "check.slug-unwritable"
	FindingWorkbenchSlugUnderivable = "check.workbench-slug-underivable"
)

// The directions an interrupted structural act is reported and finished in.
// The journal decides between the first two, the same way history determines
// the present everywhere else in this format; the last two are the states the
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
			findings = append(findings, Finding{Path: dir, Key: FindingMissingAnchor, Detail: id})
			continue
		}
		findings = append(findings, b.checkCard(card)...)
	}
	findings = append(findings, b.checkStateSlugs()...)
	findings = append(findings, b.checkWorkbenchSlug()...)
	for _, standing := range b.interruptions() {
		findings = append(findings, standing.finding())
	}
	return findings, nil
}

// checkCard applies every card-level invariant to one card.
func (b *Bench) checkCard(card *Card) []Finding {
	var findings []Finding
	anchor := card.AnchorPath()
	claimed := card.Holder != "" || card.ClaimSince != ""
	switch card.Substate {
	case contract.SubstateActive:
		if !claimed {
			findings = append(findings, Finding{Path: anchor, Key: FindingActiveWithoutClaim, Detail: card.ID})
		}
	case contract.SubstateBlocked:
		if card.BlockReason == "" {
			findings = append(findings, Finding{Path: anchor, Key: FindingBlockWithoutReason, Detail: card.ID})
		}
		if claimed {
			findings = append(findings, Finding{Path: anchor, Key: FindingHolderOnUnheld, Detail: card.ID})
		}
	case contract.SubstateReady:
		if claimed {
			findings = append(findings, Finding{Path: anchor, Key: FindingClaimWithoutActive, Detail: card.ID})
		}
	default:
		findings = append(findings, Finding{Path: anchor, Key: FindingUnknownSubstate, Detail: card.Substate})
	}
	if b.State(card.State) == nil {
		findings = append(findings, Finding{Path: anchor, Key: FindingUnknownState, Detail: card.State})
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
	findings = append(findings, checkOrdinals(card.Dir)...)
	events, torn, err := ReadJournal(card.JournalPath())
	if err != nil {
		return findings
	}
	if torn {
		findings = append(findings, Finding{Path: card.JournalPath(), Key: FindingTornJournal, Detail: card.ID})
	}
	if position := replayPosition(events); position != "" && position != card.State {
		findings = append(findings, Finding{Path: anchor, Key: FindingPositionDiverges, Detail: position})
	}
	return findings
}

// checkOrdinals applies the creation-ordinal invariants to every collection
// below one card: each entity carries a positive ordinal, and no two entities
// of one collection carry the same one.
//
// A gap in a sequence is not reported. Deletion is directory removal, so a gap
// is the shape a deletion leaves, and closing it would renumber every entity
// after the deleted one and move every positional reference already written
// down. A duplicate is reported because it leaves a positional reference with
// two answers.
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

// replayPosition returns the state the journal says a card occupies, which is
// the state of its last recorded move, or the state it was created in when it
// has never moved. An empty answer means the journal says nothing about
// position at all, which is not itself a divergence.
func replayPosition(events []Event) string {
	position := ""
	for _, ev := range events {
		switch ev.Event {
		case contract.EventCreated:
			position = ev.To
		case contract.EventMoved:
			position = ev.To
		}
	}
	return position
}
