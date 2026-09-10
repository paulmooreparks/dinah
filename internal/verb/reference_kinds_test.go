package verb

import (
	"sort"
	"strings"
	"testing"
)

// TestEveryReferenceTakingCommandDeclaresItsKinds holds the declaration's key
// set against the roster in both directions. A nineteenth command joining the
// roster reddens here rather than rendering no clause at all, and an entry for
// a command that has left the roster reddens rather than sitting unread.
//
// The roster is asserted non-empty before either comparison, because a sweep
// over nothing agrees with everything.
func TestEveryReferenceTakingCommandDeclaresItsKinds(t *testing.T) {
	roster := ReferenceTakingCommands()
	if len(roster) == 0 {
		t.Fatal("no command points at the references guide, so this check read nothing")
	}
	declared := map[string]bool{}
	for command := range referenceKinds {
		declared[command] = true
	}
	for _, command := range roster {
		kinds, ok := ReferenceKindsFor(command)
		if !ok {
			t.Errorf("%s takes a reference and referenceKinds declares no kinds for it, so its help page would render no clause", command)
			continue
		}
		if len(kinds) == 0 {
			t.Errorf("%s declares an empty kind list, which says its reference may name nothing", command)
		}
		delete(declared, command)
	}
	for command := range declared {
		t.Errorf("referenceKinds declares kinds for %s and it is not on the references roster, so nothing renders them", command)
	}
	if len(referenceKinds) != len(roster) {
		t.Fatalf("referenceKinds carries %d entries and the roster names %d commands", len(referenceKinds), len(roster))
	}
	t.Logf("%d commands read", len(roster))
}

// TestNoReferenceTakingParameterAlsoDeclaresAVocabulary refuses the one
// combination ArgumentMeaning has no sentence for. A parameter naming both a
// vocabulary and the references guide would take both appends, the kinds clause
// from this package and the values clause from the cli head, and nobody has
// written what that composition reads like.
//
// It reads the parameter's own Guide field rather than grepping the source,
// because two spellings of that guide's name are what makes a source grep
// answer sixteen where the truth is eighteen.
func TestNoReferenceTakingParameterAlsoDeclaresAVocabulary(t *testing.T) {
	roster := ReferenceTakingCommands()
	if len(roster) == 0 {
		t.Fatal("no command points at the references guide, so this check read nothing")
	}
	read := 0
	for _, command := range roster {
		for _, param := range Params(command) {
			if param.Guide != referencesGuide {
				continue
			}
			read++
			if param.Vocabulary != "" {
				t.Errorf("%s's %s argument names the references guide and the vocabulary %q, and no sentence is written for both appends", command, param.Name, param.Vocabulary)
			}
		}
	}
	t.Logf("%d reference-taking parameters read", read)
	if read != len(roster) {
		t.Fatalf("the sweep read %d reference-taking parameters and the roster names %d commands, one parameter each", read, len(roster))
	}
}

// TestEveryDeclaredKindIsOneOfTheSix refuses a declaration written in a
// vocabulary ReferenceKindOrder does not carry, which would otherwise render as
// a missing catalogue label rather than as a defect in this file.
func TestEveryDeclaredKindIsOneOfTheSix(t *testing.T) {
	known := map[ReferenceKind]bool{}
	for _, kind := range ReferenceKindOrder() {
		known[kind] = true
	}
	if len(known) != 6 {
		t.Fatalf("ReferenceKindOrder draws %d distinct kinds and the references guide's opening sentence lists six", len(known))
	}
	for command, kinds := range referenceKinds {
		seen := map[ReferenceKind]bool{}
		for _, kind := range kinds {
			if !known[kind] {
				t.Errorf("%s declares the kind %q, which ReferenceKindOrder does not carry", command, kind)
			}
			if seen[kind] {
				t.Errorf("%s declares the kind %q twice, so its clause would name it twice", command, kind)
			}
			seen[kind] = true
		}
	}
}

// TestNoReferenceKindLabelKeyCollides pins the message key each kind resolves,
// so a kind renamed in the source without its catalogue entry following reddens
// here rather than printing an English fallback in seven languages.
func TestNoReferenceKindLabelKeyCollides(t *testing.T) {
	seen := map[string]ReferenceKind{}
	for _, kind := range ReferenceKindOrder() {
		key := kind.MessageKey()
		if !strings.HasPrefix(key, "reference.kind.") {
			t.Errorf("%s resolves the key %q, which is outside the reference.kind. family", kind, key)
		}
		if other, held := seen[key]; held {
			t.Errorf("%s and %s both resolve the key %q", kind, other, key)
		}
		seen[key] = kind
	}
}

// TestReferenceKindsForDrawsTheDeclaredOrder proves the accessor imposes
// ReferenceKindOrder rather than returning the map's own literal order, which
// is what lets the declaration be written a row at a time without a rendered
// clause changing its wording.
func TestReferenceKindsForDrawsTheDeclaredOrder(t *testing.T) {
	order := map[ReferenceKind]int{}
	for at, kind := range ReferenceKindOrder() {
		order[kind] = at
	}
	roster := ReferenceTakingCommands()
	if len(roster) == 0 {
		t.Fatal("no command points at the references guide, so this check read nothing")
	}
	for _, command := range roster {
		kinds, ok := ReferenceKindsFor(command)
		if !ok {
			continue
		}
		positions := make([]int, 0, len(kinds))
		for _, kind := range kinds {
			positions = append(positions, order[kind])
		}
		if !sort.IntsAreSorted(positions) {
			t.Errorf("%s draws its kinds as %v, which is not ReferenceKindOrder", command, kinds)
		}
	}
}
