package bench

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/contract"
	"dinah/internal/durable"
)

// CardUnitFormat is the storage format from which a card is the unit of
// storage: a card's comments and checklist items are members of its journal
// rather than directories of their own, a column's comments are members of the
// workbench journal, and card.md is a projection the journal can reproduce.
// It is a constant of its own on ContainerFormat's reasoning: a later reader
// asking whether a store is in the card-unit layout asks this number, not
// whatever StorageFormat has since become.
const CardUnitFormat = 12

// CardUnitEnabled says whether this build migrates stores to the card-unit
// layout and creates new stores in it. dinah-637 ships it false; dinah-638
// sets it true in the pull request that adds its capability declarations.
const CardUnitEnabled = false

// cardUnitEnabled is the switch every reader consults. It starts as the
// constant and moves only under EnableCardUnitForTest, so a production binary
// reads exactly what CardUnitEnabled says.
var cardUnitEnabled = CardUnitEnabled

// CardUnitOn reports whether the card-unit layout is switched on in this
// process.
func CardUnitOn() bool {
	return cardUnitEnabled
}

// EnableCardUnitForTest switches the card-unit layout on for the length of one
// test and puts the switch back through t.Cleanup. A test that creates or
// opens a format-12 store calls it first. No non-test file calls it, which a
// test in this package asserts by scanning the module's source.
//
// A test that flips the switch cannot run in parallel with one that reads it,
// so it must not call t.Parallel, and neither may a test sharing its package
// with it while this is on.
func EnableCardUnitForTest(t testing.TB) {
	t.Helper()
	previous := cardUnitEnabled
	cardUnitEnabled = true
	t.Cleanup(func() { cardUnitEnabled = previous })
}

// EffectiveStorageFormat is the highest storage format this build opens and
// the format it creates new stores at: StorageFormat while the card-unit
// layout is switched on, and the format below CardUnitFormat while it is off.
// With the switch off, a build behaves toward a format-12 store exactly as a
// build whose StorageFormat is 11 does, which is how every released build
// refuses it.
func EffectiveStorageFormat() int {
	if cardUnitEnabled {
		return StorageFormat
	}
	return CardUnitFormat - 1
}

// MigratingKey is the workbench.md key a storage migration in progress
// carries, and MigratingFromKey the format the store declared before the run
// began. Both are written by the migration's first write and removed by its
// last, and a store carrying MigratingKey is refused by every ordinary open
// whatever the switch says.
const (
	MigratingKey     = "migrating"
	MigratingFromKey = "migrating_from"
	// MigratingStorage is the value MigratingKey carries during the storage
	// migration, the one migration that stamps it.
	MigratingStorage = "storage"
	// ValueMigration names, on dinah.store-awaiting-migration, the migration
	// the refused store is owed. It rides as MigratingStorage where that is
	// the storage migration and is absent where the store still owes the
	// designation conversion, which is the refusal's older sentence.
	ValueMigration = "migration"
)

// CardUnit reports whether this opened store is in the card-unit layout,
// which is the question every member read and write branches on.
func (b *Bench) CardUnit() bool {
	return b.Format >= CardUnitFormat
}

// Acquire takes the lock of an entity directory on this workbench, as the
// package-level Acquire does, and then reads workbench.md's format and
// migrating keys again. A store that has moved on since this process opened
// it is refused dinah.store-format-changed, naming the format it was opened
// at and the one it declares now, and the lock is given back: a write made
// against a layout the store no longer declares would land in files nothing
// reads. The long-lived heads reopen the workbench on that refusal.
func (b *Bench) Acquire(dir, actor, now string) (*Lock, error) {
	held, err := Acquire(dir, actor, now)
	if err != nil {
		return nil, err
	}
	if changed := b.formatChanged(); changed != nil {
		held.Release()
		return nil, changed
	}
	// In the card-unit layout the journal states the whole of card.md, so
	// a line appended on top of an unwitnessed hand edit would make the
	// journal reproduce the wrong anchor. The witness runs first, under the
	// lock just taken, at the start of every write to a card.
	if b.CardUnit() {
		if err := b.witnessOnAcquire(held, dir, actor, now); err != nil {
			held.Release()
			return nil, err
		}
	}
	return held, nil
}

// witnessOnAcquire witnesses a card whose lock was just taken, where the
// directory is a card's and the card has a created line to replay from.
func (b *Bench) witnessOnAcquire(held *Lock, dir, actor, now string) error {
	parent := filepath.Dir(dir)
	if !durable.SamePath(parent, b.CardsRoot()) && !durable.SamePath(parent, b.ArchivedCardsRoot()) {
		return nil
	}
	if !Exists(filepath.Join(dir, JournalName)) {
		return nil
	}
	card, err := b.LoadCardIn(parent, filepath.Base(dir))
	if err != nil {
		return err
	}
	_, err = b.WitnessDivergence(held, actor, now, card)
	return err
}

// formatChanged reads workbench.md again and answers the refusal a write owes
// when its format or migrating key differs from what this process opened, or
// nil when both still agree.
func (b *Bench) formatChanged() error {
	text, err := ReadText(filepath.Join(b.Root, WorkbenchAnchor))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	fm, _ := ParseAnchor(text)
	format := b.Format
	if declared := strings.TrimSpace(fm.Value("format")); declared != "" {
		if n, err := strconv.Atoi(declared); err == nil {
			format = n
		}
	}
	migrating := fm.Value(MigratingKey)
	if format == b.Format && migrating == b.Migrating {
		return nil
	}
	return contract.RefuseWith(contract.StoreFormatChanged, describeFormat(b.Format, b.Migrating), map[string]string{
		"now": describeFormat(format, migrating),
	})
}

// describeFormat spells a declared format and a migrating key the way the
// refusal names them: the number, and the migration in progress beside it.
func describeFormat(format int, migrating string) string {
	spelled := strconv.Itoa(format)
	if migrating != "" {
		spelled += " (" + MigratingKey + ": " + migrating + ")"
	}
	return spelled
}
