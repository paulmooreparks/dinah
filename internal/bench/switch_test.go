package bench

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"dinah/internal/contract"
)

// stampWorkbench rewrites a fixture workbench's format and, where migrating
// is not empty, adds the migrating key beside it, the way the storage
// migration's first write leaves the anchor.
func stampWorkbench(t *testing.T, root string, format int, migrating string) {
	t.Helper()
	to := "format: " + strconv.Itoa(format)
	if migrating != "" {
		to += "\n" + MigratingKey + ": " + migrating
	}
	editWorkbench(t, root, "format: "+strconv.Itoa(EffectiveStorageFormat()), to)
}

// openers are the four ways a workbench is opened, each named so a failure
// says which one answered wrongly.
var openers = []struct {
	name string
	open func(string) (*Bench, error)
}{
	{"Open", Open},
	{"OpenAwaitingResolution", OpenAwaitingResolution},
	{"OpenUncontained", OpenUncontained},
	{"OpenPreVocabulary", OpenPreVocabulary},
}

// TestTheCardUnitSwitchShipsOffAndOnlyTestsTurnItOn is the last clause of
// dinah-637/criteria/11. The constant is false, and no non-test Go file in the
// repository calls EnableCardUnitForTest, so a production binary reads exactly
// what the constant says. The walk states how many files it read, and a
// planted call in a non-test file is what it exists to find.
func TestTheCardUnitSwitchShipsOffAndOnlyTestsTurnItOn(t *testing.T) {
	if CardUnitEnabled {
		t.Fatal("CardUnitEnabled is true, and dinah-637 ships the card-unit layout switched off; dinah-638 turns it on")
	}
	if CardUnitOn() {
		t.Fatal("the switch reads on outside any test that turned it on")
	}
	calls, scanned := enableCallsOutsideTests(t, filepath.Join("..", ".."))
	if scanned < 200 {
		t.Fatalf("the walk read %d non-test Go files, fewer than the module holds, so it is not reading the module", scanned)
	}
	t.Logf("read %d non-test Go files", scanned)
	for _, call := range calls {
		t.Errorf("%s calls EnableCardUnitForTest outside a test", call)
	}
}

// enableCallsOutsideTests answers every call of EnableCardUnitForTest in a
// non-test Go file under root, as file:line, and how many files it read.
func enableCallsOutsideTests(t *testing.T, root string) ([]string, int) {
	t.Helper()
	var calls []string
	scanned := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			name := entry.Name()
			if name == "testdata" || name == "node_modules" || (strings.HasPrefix(name, ".") && path != root) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		scanned++
		fileset := token.NewFileSet()
		file, err := parser.ParseFile(fileset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := ""
			switch fun := call.Fun.(type) {
			case *ast.Ident:
				name = fun.Name
			case *ast.SelectorExpr:
				name = fun.Sel.Name
			}
			if name == "EnableCardUnitForTest" {
				calls = append(calls, path+":"+strconv.Itoa(fileset.Position(call.Pos()).Line))
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk the module: %v", err)
	}
	return calls, scanned
}

// TestAFormatTwelveStoreIsRefusedWhileTheSwitchIsOff is the opener half of
// dinah-637/criteria/39. With the switch off, every opener refuses a store
// declaring format 12 unsupported-version with the detail format 12, which is
// the refusal and detail the integer test gave a build whose StorageFormat was
// 11, since that test compares against the effective ceiling. With the switch
// on, the same store opens.
func TestAFormatTwelveStoreIsRefusedWhileTheSwitchIsOff(t *testing.T) {
	root := newFixture(t)
	stampWorkbench(t, root, CardUnitFormat, "")
	// OpenPreVocabulary admits only a profile inside the retired
	// vocabulary's window, so it is handed a store written in that
	// vocabulary, stamped the same way.
	retired := preVocabularyFixture(t, "Fixture", []string{"b00000000001"}, []string{"b00000000001"})
	stampWorkbench(t, retired, CardUnitFormat, "")
	for _, opener := range openers {
		store := root
		if opener.name == "OpenPreVocabulary" {
			store = retired
		}
		_, err := opener.open(store)
		refusal, ok := err.(*contract.Refusal)
		if !ok || refusal.Name != contract.UnsupportedVer || refusal.Detail != "format 12" {
			t.Errorf("%s with the switch off: wanted %s naming format 12, got %v", opener.name, contract.UnsupportedVer, err)
		}
	}

	EnableCardUnitForTest(t)
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("with the switch on, a format-12 store: wanted it to open, got %v", err)
	}
	if !opened.CardUnit() {
		t.Error("the opened store does not read as the card-unit layout")
	}
}

// TestTheSwitchOnRefusesAStoreBelowTwelveAndCreatesTwelve is the switch-on
// half of dinah-637/criteria/11 at the store. With the switch on, an ordinary
// open of a format-11 store is refused dinah.store-awaiting-migration with the
// storage migration named, the openers that serve a migration or check still
// open it, and a new store is created at format 12. With the switch off the
// same store opens and a new store is created at 11.
func TestTheSwitchOnRefusesAStoreBelowTwelveAndCreatesTwelve(t *testing.T) {
	root := newFixture(t)
	if _, err := Open(root); err != nil {
		t.Fatalf("with the switch off, a format-11 store: wanted it to open, got %v", err)
	}
	created := containedPath(t.TempDir())
	if err := Instantiate(created, "fx", "alka", minimalDefinition(t)); err != nil {
		t.Fatalf("instantiate with the switch off: %v", err)
	}
	if format, _ := declaredFormat(created); format != CardUnitFormat-1 {
		t.Errorf("with the switch off a new store declares format %d, wanted %d", format, CardUnitFormat-1)
	}

	EnableCardUnitForTest(t)
	_, err := Open(root)
	refusal, ok := err.(*contract.Refusal)
	if !ok || refusal.Name != contract.StoreAwaitingMigration || refusal.Extra[ValueMigration] != MigratingStorage {
		t.Fatalf("with the switch on, a format-11 store: wanted %s naming the storage migration, got %v", contract.StoreAwaitingMigration, err)
	}
	if _, err := OpenAwaitingResolution(root); err != nil {
		t.Errorf("the opener dinah check and the migrations use refused the store: %v", err)
	}
	fresh := containedPath(t.TempDir())
	if err := Instantiate(fresh, "fx", "alka", minimalDefinition(t)); err != nil {
		t.Fatalf("instantiate with the switch on: %v", err)
	}
	if format, _ := declaredFormat(fresh); format != CardUnitFormat {
		t.Errorf("with the switch on a new store declares format %d, wanted %d", format, CardUnitFormat)
	}
}

// TestAStoreMidStorageMigrationIsRefusedWithEitherSetting is the migrating
// clause of dinah-637/criteria/11. A store carrying migrating: storage is
// refused by every ordinary open with the switch on as with it off. With the
// switch off the integer test answers first, because the store declares
// format 12, and the answer is the one every older build gives; with it on the
// store is refused dinah.store-awaiting-migration naming the storage
// migration. The migration's own opener reads it with the switch on.
func TestAStoreMidStorageMigrationIsRefusedWithEitherSetting(t *testing.T) {
	root := newFixture(t)
	stampWorkbench(t, root, CardUnitFormat, MigratingStorage)
	_, err := Open(root)
	if refusal, ok := err.(*contract.Refusal); !ok || refusal.Name != contract.UnsupportedVer {
		t.Errorf("with the switch off, a store mid-migration: wanted %s, got %v", contract.UnsupportedVer, err)
	}

	EnableCardUnitForTest(t)
	_, err = Open(root)
	refusal, ok := err.(*contract.Refusal)
	if !ok || refusal.Name != contract.StoreAwaitingMigration || refusal.Extra[ValueMigration] != MigratingStorage {
		t.Errorf("with the switch on, a store mid-migration: wanted %s naming the storage migration, got %v", contract.StoreAwaitingMigration, err)
	}
	opened, err := OpenAwaitingResolution(root)
	if err != nil {
		t.Fatalf("the migration's own opener refused the store: %v", err)
	}
	if opened.Migrating != MigratingStorage {
		t.Errorf("the opened store reads migrating %q, wanted %q", opened.Migrating, MigratingStorage)
	}
}

// TestAWriteRefusesAStoreWhoseFormatMovedUnderIt is dinah-637/criteria/26 at
// the lock. A bench opened on a format-11 store whose workbench.md is then
// rewritten to declare format 12 with migrating: storage refuses its next
// acquisition dinah.store-format-changed, naming the format it opened and the
// one declared now, and gives the lock back. The accepting case, an
// acquisition on an unchanged store, sits beside it.
func TestAWriteRefusesAStoreWhoseFormatMovedUnderIt(t *testing.T) {
	root := newFixture(t)
	opened, err := Open(root)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	card := filepath.Join(root, CardsDir, "c00000000001")
	held, err := opened.Acquire(card, "alka", "2026-09-28T00:00:00Z")
	if err != nil {
		t.Fatalf("an acquisition on an unchanged store: wanted the lock, got %v", err)
	}
	held.Release()

	stampWorkbench(t, root, CardUnitFormat, MigratingStorage)
	_, err = opened.Acquire(card, "alka", "2026-09-28T00:00:01Z")
	refusal, ok := err.(*contract.Refusal)
	if !ok || refusal.Name != contract.StoreFormatChanged {
		t.Fatalf("wanted %s, got %v", contract.StoreFormatChanged, err)
	}
	if refusal.Detail != "11" || refusal.Extra["now"] != "12 (migrating: storage)" {
		t.Errorf("the refusal names %q and now %q, wanted 11 and 12 (migrating: storage)", refusal.Detail, refusal.Extra["now"])
	}
	if Exists(filepath.Join(card, LockName)) {
		t.Error("the refused acquisition left the card's lock standing")
	}
}

// minimalDefinition is a definition of one column, which is the least a new
// store is instantiated from.
func minimalDefinition(t *testing.T) *Definition {
	t.Helper()
	definition, err := ReadDefinition([]byte(`{"profile": "dinah-core/0.7", "title": "Fixture", "columns": [{"id": "b00000000001", "title": "Doing", "kind": "work"}]}`))
	if err != nil {
		t.Fatalf("read the definition: %v", err)
	}
	return definition
}
