// Package guidepin exists solely to be shared between test files in
// cmd/dinah and internal/bench, the way internal/bench/compattest exists
// solely to be shared between the compatibility test files in those same two
// packages. No production file imports it.
//
// It holds the sentences a guide states about behaviour that some test
// already proves, so the sentence and the proof are tied together. dinah-461
// found the shape this answers: the references guide said Dinah has no
// restore, thirty lines above a table listing restore, and nothing in the
// tree read that sentence, so it went stale under a green suite.
//
// The pins live here rather than in either test package because two of the
// proving tests are in internal/bench and three are in cmd/dinah, and a test
// in one package cannot read a helper declared in another package's test
// files. It takes no *testing.T, so it never imports testing, and it imports
// internal/guide and nothing else from this tree.
package guidepin

import (
	"errors"
	"fmt"
	"strings"

	"dinah/internal/guide"
)

// Statement is one claim a guide carries and one test proves.
type Statement struct {
	// Topic is the guide's topic, spelled as guide.Topics() spells it.
	Topic string
	// Text is the claim, folded to single spaces, as the guide carries it.
	// Most claims are a single sentence. The fifth carries two, because
	// dinah-478 corrected the second of them, and a pin holding only the
	// first would have left the corrected text unheld.
	Text string
	// Provenance is the name of the test function that proves the behaviour
	// this claim describes.
	Provenance string
}

// The five claims the references guide's "Reading the archive" section makes,
// each named for what it claims. The text lives here and nowhere else, so a
// reworded guide is corrected in one place.
const (
	// ArchivedReadsTheDeepestCollectionStep says which half of the workbench
	// each step of a reference is resolved in under the flag.
	ArchivedReadsTheDeepestCollectionStep = "`--archived` reads the archive mirror at a reference's deepest collection step, and the live half at every step above it."

	// PositionsCountTheMirrorsOwnMembers says a position under the flag is
	// counted within the archived half rather than across both.
	PositionsCountTheMirrorsOwnMembers = "Positions under the flag count the mirror's own members."

	// AnEntityComesBackWithItsHolder says an entity archived inside its
	// holder is restored by restoring the holder.
	AnEntityComesBackWithItsHolder = "An entity that travelled into the archive inside its holder comes back with that holder."

	// ARestoredColumnLandsAtTheEndOfTheOrder says where a restored column
	// sits in the workbench's column order.
	ARestoredColumnLandsAtTheEndOfTheOrder = "A restored column lands at the end of the column order, because the order is what the workbench's own definition records and a restore appends to it."

	// AnArchivedContentsRowIsTheAddressAfterRestore says what a row below an
	// archived root addresses, and where the listing says so. The second
	// sentence is the one dinah-478 corrected: the tool prints the sentence
	// naming the root first, so the notice never stood on the listing's own
	// first line.
	AnArchivedContentsRowIsTheAddressAfterRestore = "A reference printed under `dinah contents --archived` below the walk's root is the address that child will have once the root is restored, and it does not resolve while the root is archived. The listing says so on the line under the sentence naming the root."
)

// pinned is the roster, in the order the references guide carries it.
var pinned = []Statement{
	{Topic: "references", Text: ArchivedReadsTheDeepestCollectionStep, Provenance: "TestTheArchivedHalfIsReadAtTheDeepestCollectionStep"},
	{Topic: "references", Text: PositionsCountTheMirrorsOwnMembers, Provenance: "TestAPositionUnderTheFlagCountsTheMirrorsOwnMembers"},
	{Topic: "references", Text: AnEntityComesBackWithItsHolder, Provenance: "TestANestedArchiveRestoresInTwoActs"},
	{Topic: "references", Text: ARestoredColumnLandsAtTheEndOfTheOrder, Provenance: "TestRestoringAColumnReturnsItToTheOrderAndRepairsAStrandedCard"},
	{Topic: "references", Text: AnArchivedContentsRowIsTheAddressAfterRestore, Provenance: "TestAnArchivedReadShowsOneHalfAndWritesNothing"},
}

// Pinned returns every pinned statement, in the order its guide carries them.
func Pinned() []Statement {
	roster := make([]Statement, len(pinned))
	copy(roster, pinned)
	return roster
}

// The two refusals Carries can raise, declared as sentinels so a caller can
// tell them apart with errors.Is. They are distinguishable because they have
// to be: an unknown topic yields an empty guide text, and a plain containment
// answer over an empty text is already no, so a Carries that stopped refusing
// an unknown topic would go on refusing it by accident and a test asserting
// only that some error came back would stay green. internal/bench's
// ErrRenameCollides is the idiom this follows.
var (
	// ErrNoSentence refuses a pin naming no sentence. An empty needle is a
	// substring of every haystack, so this is the case that would otherwise
	// pass against any guide at all.
	ErrNoSentence = errors.New("guidepin: the pin names no sentence")
	// ErrUnknownTopic refuses a pin naming a guide this build does not serve.
	ErrUnknownTopic = errors.New("guidepin: the pin names a guide this build does not serve")
)

// Carries reports whether the named guide still carries the sentence. It
// folds the guide to single spaces first, so a re-wrap of the guide's source
// is not a failure.
//
// It refuses an empty sentence and an unknown topic rather than answering
// nil, because a pin naming nothing would otherwise pass and buy confidence
// it had not earned.
func Carries(topic, sentence string) error {
	folded := strings.Join(strings.Fields(sentence), " ")
	if folded == "" {
		return fmt.Errorf("%w, so it cannot be held against the %q guide", ErrNoSentence, topic)
	}
	text, err := guide.Text(topic)
	if err != nil {
		return fmt.Errorf("%w: %q: %v", ErrUnknownTopic, topic, err)
	}
	if !strings.Contains(strings.Join(strings.Fields(text), " "), folded) {
		return fmt.Errorf("the %s guide no longer carries the sentence a test proves: %q", topic, folded)
	}
	return nil
}
