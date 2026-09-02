package bench

import (
	"os"
	"path/filepath"
	"strconv"

	"dinah/internal/contract"
)

// The on-disk spellings this build retired. A workbench written before
// dinah-287 names its flow collection states/, names each member's anchor
// state.md, and carries a states: sequence on its own anchor. Nothing but the
// vocabulary migration reads these names, and no writer here produces them.
const (
	PreVocabularyDir         = "states"
	PreVocabularyAnchor      = "state.md"
	preVocabularySequenceKey = "states"
	preVocabularyColumnKey   = "state"
	preVocabularyStateKey    = "substate"
)

// The card keys this build writes. columnKey and stateKey are named beside
// the retired spellings above because one of the four names is shared:
// preVocabularyColumnKey and stateKey are both "state", which is the whole
// difficulty this migration has to hold in view. The word named the column
// before the rename and names the condition after it, so its presence on a
// card answers nothing about which vocabulary the card is written in.
//
// columnKey is the name that does answer it. No card written in the retired
// vocabulary carries it, since that vocabulary spells the flow position
// "state", and every card this build writes carries it, since Card.Save sets
// it on every write. Checked against the whole compat corpus before it was
// built on: all six cards of each pre-vocabulary fixture, at 0.4, 0.5, 0.6,
// 1.0 and 1.0-pre-slug, plus the archived card of each, carry state and
// substate and no column; all seven cards of the 0.7 fixture carry column and
// state and no substate.
const (
	columnKey = "column"
	stateKey  = "state"
)

// PreVocabularyFloor and PreVocabularyCeiling are the profile window this
// build accepted immediately before dinah-287 raised the floor. A workbench
// declaring a revision in this closed range still carries the state/substate
// vocabulary and columns/ is still named states/, and needs
// `dinah check --migrate-vocabulary`, not a refusal it cannot act on.
var (
	PreVocabularyFloor   = [2]int{0, 1}
	PreVocabularyCeiling = [2]int{0, 6}
)

// withinPreVocabulary reports whether a resolved revision sits in the closed
// window the vocabulary migration acts on.
func withinPreVocabulary(pair [2]int) bool {
	return !sortsBelow(pair, PreVocabularyFloor) && !sortsBelow(PreVocabularyCeiling, pair)
}

// admitProfileAfterVocabulary is what Open admits a declared profile with. It
// asks the vocabulary question first: a revision this build knows to be
// written in the retired vocabulary is refused by name, carrying the declared
// string, so the refusal can name the migration that repairs it. Everything
// else falls through to admitProfile unchanged, which refuses a revision
// outside this build's window exactly as it does today.
//
// The order matters and is the whole point. A workbench inside the window is
// refused before any column or card is read, so no reader ever takes an old
// card's state field, which holds a flow position under the old vocabulary,
// for one of ready, active or blocked.
func admitProfileAfterVocabulary(declared string) (int, int, error) {
	major, minor, _, ok := resolveProfile(declared, [2]int{ProfileMajor, ProfileMinor})
	if ok && withinPreVocabulary([2]int{major, minor}) {
		return 0, 0, contract.Refuse(contract.NeedsVocabularyMigration, declared)
	}
	return admitProfile(declared)
}

// admitPreVocabularyProfile is the mirror of admitProfileAfterVocabulary, and
// what OpenPreVocabulary admits a declared profile with. It wants the declared
// revision inside the pre-vocabulary window and refuses everything else: a
// revision below the window predates the chain this build's migration covers,
// a revision above it is either already migrated, in which case there is
// nothing to do and saying so plainly beats redoing the work, or a future this
// build has no reading for, which is the ordinary unsupported-version answer.
func admitPreVocabularyProfile(declared string) (int, int, error) {
	major, minor, _, ok := resolveProfile(declared, PreVocabularyCeiling)
	if !ok {
		return 0, 0, errProfileMalformed
	}
	pair := [2]int{major, minor}
	if withinPreVocabulary(pair) {
		return pair[0], pair[1], nil
	}
	if sortsBelow([2]int{ProfileMajor, ProfileMinor}, pair) {
		return 0, 0, contract.RefuseWith(contract.UnsupportedVer, declared, map[string]string{
			"floor":   revisionText(PreVocabularyFloor),
			"ceiling": revisionText(PreVocabularyCeiling),
		})
	}
	return 0, 0, contract.Refuse(contract.NeedsVocabularyMigration, declared)
}

// ClassifyVocabulary reads the profile a candidate workbench declares and
// resolves it, without opening the workbench and without reading a single
// card. ok is false when the anchor carries no profile key or carries one that
// does not parse, which is the profile-less shape a workbench recognized by
// its format or columns key alone can take; such a candidate is reported by
// the caller rather than opened, since neither opener will read it.
//
// The resolution is resolveProfile's, not splitProfile's, so a workbench
// declaring the retired dinah-core/1.0 spelling classifies here exactly as it
// does inside Open: as revision 0.1, a migration candidate, rather than as an
// unsupported future revision.
func ClassifyVocabulary(root string) (major, minor int, ok bool, err error) {
	path := filepath.Join(root, WorkbenchAnchor)
	text, err := ReadText(path)
	if err != nil {
		return 0, 0, false, contract.RefuseWith(contract.Malformed, WorkbenchAnchor, map[string]string{"path": path, contract.ValueWorkbench: root})
	}
	fm, _ := ParseAnchor(text)
	declared := fm.Value("profile")
	if declared == "" {
		return 0, 0, false, nil
	}
	major, minor, _, ok = resolveProfile(declared, [2]int{ProfileMajor, ProfileMinor})
	if !ok {
		return 0, 0, false, nil
	}
	return major, minor, true, nil
}

// MigrateVocabulary rewrites one workbench from the retired vocabulary to the
// current one, given a bench opened through OpenPreVocabulary. It renames each
// card's two frontmatter keys, renames the flow collection and every member
// anchor inside it, rewrites the workbench anchor's own sequence key, and
// declares the revision that carries the new vocabulary. It returns the
// identifiers of the cards it rewrote.
//
// A card's frontmatter is read and written through ParseAnchor and
// Frontmatter.Render rather than through LoadCard and Card.Save, because those
// two carry the current vocabulary's meaning for the state key: reading a
// pre-vocabulary card through them would take its flow position for its
// condition, which is the silent misread this whole migration exists to
// prevent. Every other key on the card, and its position in the header,
// survives untouched, which is what the raw-lines Frontmatter gives for free.
//
// The journal is not rewritten. Its own field names carry no vocabulary, and
// its history is append-only, so there is nothing here to migrate.
//
// A second run over the same workbench is an ordinary event rather than a
// mistake, so every step here is written to be re-entrant. A workbench the
// walk failed partway through is left half converted, and the report asks the
// operator to clear the cause and run it again, which means the resumed run
// meets cards that have been rewritten beside cards that have not, and a
// collection whose members have been renamed beside members that have not.
// Each step therefore recognizes its own output and steps over it: a card by
// the column key it now carries, a member anchor and the collection itself by
// the name no longer being there. What none of them do is act twice.
func MigrateVocabulary(b *Bench) ([]string, error) {
	// The anchor's own precondition is checked before the first card is
	// written, even though the anchor is rewritten last. An anchor carrying
	// both sequence keys is a shape no writer produces and the rename
	// refuses it, and refusing it at the end left a workbench whose cards
	// and collection had already been carried across and whose anchor never
	// could be, so every later run met the same refusal and the workbench
	// opened under neither opener. Asking first costs one read and leaves
	// such a workbench exactly as it was found.
	if err := checkBenchAnchorVocabulary(b); err != nil {
		return nil, err
	}
	touched, err := migrateCardVocabulary(b)
	if err != nil {
		return touched, err
	}
	if err := migrateColumnDirectories(b); err != nil {
		return touched, err
	}
	if err := migrateBenchAnchor(b); err != nil {
		return touched, err
	}
	return touched, nil
}

// migrateCardVocabulary renames the two keys on every card of the workbench,
// live and archived alike, and answers the identifiers it rewrote.
func migrateCardVocabulary(b *Bench) ([]string, error) {
	var touched []string
	for _, dir := range []string{
		filepath.Join(b.Root, CardsDir),
		filepath.Join(b.Root, ArchiveDir, CardsDir),
	} {
		ids, err := listIdentifiers(dir)
		if err != nil {
			return touched, err
		}
		for _, id := range ids {
			path := filepath.Join(dir, id, CardAnchor)
			if !Exists(path) {
				continue
			}
			text, err := ReadText(path)
			if err != nil {
				return touched, contract.RefuseWith(contract.Malformed, CardAnchor, map[string]string{"path": path, contract.ValueWorkbench: b.Root})
			}
			fm, body := ParseAnchor(text)
			// A card carrying the column key has been across this rename
			// already, and the run that carried it may have been an earlier
			// run of this same migration: a workbench the walk failed
			// partway through is left with some cards rewritten and some
			// not, and the report asks the operator to clear the cause and
			// run it again. So the second pass has to tell one shape from
			// the other, and the column key is the only name on the card
			// that tells it. Asking whether the card carries "state"
			// answers nothing, because both shapes carry that word.
			//
			// The migration's own skip-guard used to ask exactly that, and
			// the cost of the ambiguity was total: a rewritten card reads as
			// unrewritten, the column rename then maps "state" onto a
			// "column" the card already holds, and the card's real column is
			// replaced by its condition.
			if fm.Has(columnKey) {
				if !fm.Has(preVocabularyStateKey) {
					continue
				}
				// Column beside substate is a card half of each vocabulary,
				// which neither this migration nor any writer produces, and
				// guessing which key holds the flow position is how a
				// half-written card becomes a destroyed one. The detail
				// carries the identifier as well as the filename, because
				// the report of a tree-wide run prints the workbench's path
				// and would otherwise tell an operator that one card of
				// several hundred is the problem without saying which.
				return touched, contract.RefuseWith(contract.VocabularyMixed, filepath.Join(id, CardAnchor), map[string]string{"path": path})
			}
			if !fm.Has(preVocabularyStateKey) && !fm.Has(preVocabularyColumnKey) {
				continue
			}
			// The order is forced: the new name of the state key is the old
			// name of the column key, so renaming the column key first frees
			// the name the state key is about to take. Neither rename can
			// collide here, since the guard above has established the card
			// carries no column key, and both are checked anyway because a
			// rename that silently made room is what this migration is
			// recovering from.
			if err := fm.Rename(preVocabularyColumnKey, columnKey); err != nil {
				return touched, err
			}
			if err := fm.Rename(preVocabularyStateKey, stateKey); err != nil {
				return touched, err
			}
			if err := WriteText(path, fm.Render(body)); err != nil {
				return touched, err
			}
			touched = append(touched, id)
		}
	}
	return touched, nil
}

// migrateColumnDirectories renames the flow collection and every member's own
// anchor, under the live tree and under the archive both, since an archived
// flow position is written the same way a live one is.
func migrateColumnDirectories(b *Bench) error {
	for _, parent := range []string{b.Root, filepath.Join(b.Root, ArchiveDir)} {
		from := filepath.Join(parent, PreVocabularyDir)
		if !Exists(from) {
			continue
		}
		ids, err := listIdentifiers(from)
		if err != nil {
			return err
		}
		for _, id := range ids {
			anchor := filepath.Join(from, id, PreVocabularyAnchor)
			if !Exists(anchor) {
				continue
			}
			if err := os.Rename(anchor, filepath.Join(from, id, ColumnAnchor)); err != nil {
				return err
			}
		}
		if err := os.Rename(from, filepath.Join(parent, ColumnsDir)); err != nil {
			return err
		}
	}
	return nil
}

// checkBenchAnchorVocabulary refuses a workbench anchor carrying both sequence
// keys, which is half of each vocabulary and which no writer produces. Dinah
// refuses rather than choosing which of the two lists the workbench's flow
// really is, and the refusal names the anchor and its path so that an operator
// reading a tree-wide report is told which file to edit.
//
// The header it asks about is the one the opener already parsed and kept on
// the bench, so the check costs no read and no parse of its own.
func checkBenchAnchorVocabulary(b *Bench) error {
	if b.FM.Has(preVocabularySequenceKey) && b.FM.Has(currentVocabulary.SequenceKey) {
		return contract.RefuseWith(contract.VocabularyMixed, WorkbenchAnchor, map[string]string{"path": filepath.Join(b.Root, WorkbenchAnchor)})
	}
	return nil
}

// migrateBenchAnchor renames the workbench anchor's own sequence key and
// declares the revision the current vocabulary arrived in. The rename cannot
// collide, because checkBenchAnchorVocabulary asked before the first card was
// written, and it is checked anyway because a rename that silently made room
// is what this migration is recovering from.
func migrateBenchAnchor(b *Bench) error {
	path := filepath.Join(b.Root, WorkbenchAnchor)
	text, err := ReadText(path)
	if err != nil {
		return contract.RefuseWith(contract.Malformed, WorkbenchAnchor, map[string]string{"path": path, contract.ValueWorkbench: b.Root})
	}
	fm, body := ParseAnchor(text)
	if err := fm.Rename(preVocabularySequenceKey, currentVocabulary.SequenceKey); err != nil {
		return contract.RefuseWith(contract.VocabularyMixed, WorkbenchAnchor, map[string]string{"path": path})
	}
	fm.Set("profile", ProfileVersion)
	return WriteText(path, fm.Render(body))
}

// listIdentifiers answers the member directories of a collection, or nothing
// at all when the collection is not there. A collection this format never
// wrote is an ordinary shape rather than a defect, so an absent directory is
// not an error here.
func listIdentifiers(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ids []string
	for _, entry := range entries {
		if entry.IsDir() {
			ids = append(ids, entry.Name())
		}
	}
	return ids, nil
}

// revisionOf spells a resolved revision the way a report names one.
func revisionOf(major, minor int) string {
	return ProfileName + "/" + strconv.Itoa(major) + "." + strconv.Itoa(minor)
}

// WithinPreVocabulary reports whether a resolved revision sits in the window
// the vocabulary migration acts on, which is the question a caller sorting
// candidates asks about each one.
func WithinPreVocabulary(major, minor int) bool {
	return withinPreVocabulary([2]int{major, minor})
}

// WithinCurrent reports whether a resolved revision sits in the window this
// build opens ordinarily, which is a workbench with nothing to migrate.
func WithinCurrent(major, minor int) bool {
	pair := [2]int{major, minor}
	return !sortsBelow(pair, [2]int{ProfileFloorMajor, ProfileFloorMinor}) &&
		!sortsBelow([2]int{ProfileMajor, ProfileMinor}, pair)
}

// RevisionText spells a resolved revision the way the interchange form spells
// the one this build declares, which is the form a report names.
func RevisionText(major, minor int) string {
	return revisionOf(major, minor)
}
