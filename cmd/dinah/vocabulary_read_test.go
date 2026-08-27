package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
)

// The reader's side of the rename, which is where the first version of this
// guard looked in the wrong place.
//
// A card is on the wrong side of the rename in more than one shape, and the
// shape a hand edit reaches most easily is not the shape a whole-workbench
// unwind produces. The guard's first version asked whether a card carried the
// retired substate key. The harm it exists to prevent comes from the state
// key, which holds the card's column before the rename and its condition
// after, so a card can carry the harm without carrying the trigger: rename
// column back to state, delete the condition line, and the file carries a
// column identifier under a key both vocabularies have and nothing else at
// all. Dinah listed such a card with its column identifier printed under the
// heading naming its condition, and exited 0.
//
// The guard now asks what D-14 settled: whether the card carries the column
// key. No card written before the rename carries it and Card.Save writes it on
// every card this build writes, so a card that never came across the rename
// cannot answer yes, and the one-line hand edit below is caught. The
// migration's own skip-guard has asked it that way from the start; only the
// read path went the other way.
//
// What the guard asks is narrower than whether the card is on the right side
// of the rename. It tests that the column key is present and tests nothing
// about what either key holds, so a card carrying a column key beside a
// condition value the rename would never have produced still reads through.
// That weakness is older than this rename, nothing here closes it, and it is
// filed as dinah-306.

// unwindOneCardHalfway makes the shape a hand edit reaches most easily: the
// column key renamed back to its retired spelling and the condition key
// deleted with it.
func unwindOneCardHalfway(t *testing.T, root, id string) {
	t.Helper()
	rewriteFile(t, filepath.Join(root, bench.CardsDir, id, bench.CardAnchor), func(text string) string {
		text = regexp.MustCompile("\nstate: [^\n]*").ReplaceAllString(text, "")
		return strings.Replace(text, "\ncolumn: ", "\nstate: ", 1)
	})
}

// TestACardMissingTheColumnKeyIsRefusedWhateverElseItCarries asserts the shape
// the substate-keyed guard let through, on the four routes a reader reaches a
// card by. The card here carries neither the substate key nor the column key,
// which is what a hand edit of a single line produces and what no roster of
// retired key names can see.
func TestACardMissingTheColumnKeyIsRefusedWhateverElseItCarries(t *testing.T) {
	root, _, _, _, current := buildTreeFixture(t)
	ids := bench.ListIDs(filepath.Join(current, bench.CardsDir))
	if len(ids) == 0 {
		t.Fatalf("%s holds no cards, so this test asserts nothing", current)
	}
	unwindOneCardHalfway(t, current, ids[0])
	if cardCarries(t, current, ids[0], "substate") {
		t.Fatalf("the card %s carries a substate key, so this fixture is not the shape the test needs", ids[0])
	}

	// show is asked twice, by the card's human reference and by its
	// identifier, because the two take different routes into the collection
	// and only one of them carried the refusal out. The identifier route
	// reported that the workbench held no such card, which is untrue of a card
	// whose file is right there, so which route a reader took decided which
	// answer they got.
	for _, argv := range [][]string{{"ls"}, {"status"}, {"show", "fx-1"}, {"show", ids[0]}} {
		got := runCLI(t, root, append([]string{"--workbench", current}, argv...)...)
		if got.code == 0 {
			t.Errorf("%v over a workbench holding a card that never came across the rename exited 0:\n%s", argv, got.out)
		}
		if !strings.Contains(got.errw, contract.VocabularyRetired) {
			t.Errorf("%v refused with %q, wanted the %s refusal", argv, got.errw, contract.VocabularyRetired)
		}
		if !strings.Contains(got.errw, ids[0]) {
			t.Errorf("%v refused without naming the card %s:\n%s", argv, ids[0], got.errw)
		}
	}

	// The listing must not print the card's column identifier as though it
	// were the card's condition, which is the misread itself rather than the
	// exit status that reports it.
	listed := runCLI(t, root, "--workbench", current, "ls")
	column := columnOfCard(t, current, ids[0])
	if strings.Contains(listed.out, column) {
		t.Errorf("the listing prints the column identifier %s where the card's condition belongs:\n%s", column, listed.out)
	}
}

// TestCheckNamesAnUnreadableCardRatherThanReportingItAbsent asserts the
// checker's half. check is the tool a reader reaches for once a reader has
// refused, so it has to say what is actually wrong. It used to turn every
// refusal the card reader raised into the finding for a directory carrying no
// anchor file, which is untrue of a file that is plainly there and which
// invites the reader to delete a directory holding a card.
func TestCheckNamesAnUnreadableCardRatherThanReportingItAbsent(t *testing.T) {
	for _, shape := range []struct {
		name   string
		unwind func(t *testing.T, root string, ids []string)
		want   string
	}{
		{
			name:   "the whole card in the retired vocabulary",
			unwind: func(t *testing.T, root string, ids []string) { unwindCards(t, root) },
			want:   "is written in the vocabulary this format retired",
		},
		{
			name:   "the column key renamed back and the condition deleted",
			unwind: func(t *testing.T, root string, ids []string) { unwindOneCardHalfway(t, root, ids[0]) },
			want:   "is written in the vocabulary this format retired",
		},
		{
			name:   "half of each vocabulary in one header",
			unwind: func(t *testing.T, root string, ids []string) { mixOneCard(t, root, ids[0]) },
			want:   "carries a key from each of the two vocabularies",
		},
	} {
		t.Run(shape.name, func(t *testing.T) {
			root, _, _, _, current := buildTreeFixture(t)
			ids := bench.ListIDs(filepath.Join(current, bench.CardsDir))
			if len(ids) == 0 {
				t.Fatalf("%s holds no cards, so this test asserts nothing", current)
			}
			shape.unwind(t, current, ids)

			got := runCLI(t, root, "--workbench", current, "check")
			if got.code == 0 {
				t.Errorf("check over a workbench of unreadable cards exited 0:\n%s", got.out)
			}
			if !strings.Contains(got.out, ids[0]) {
				t.Errorf("check does not name the card %s:\n%s", ids[0], got.out)
			}
			if !strings.Contains(got.out, shape.want) {
				t.Errorf("check does not report the defect the card has, wanted %q:\n%s", shape.want, got.out)
			}
			for _, wrong := range []string{"carries no anchor file", "does not count as part of the workbench"} {
				if strings.Contains(got.out, wrong) {
					t.Errorf("check reports a card whose anchor file is there as having none:\n%s", got.out)
				}
			}
		})
	}
}

// TestACardCarryingHalfOfEachVocabularyIsRefusedAsMixed keeps the two reader
// refusals apart. A card carrying the column key beside the retired substate
// key really is one file holding half of each vocabulary, so its reader is
// told to remove one of the two. A card written wholly in the retired
// vocabulary is internally consistent and disagrees with the anchor above it,
// so it gets the other refusal. One sentence over both shapes told the reader
// of the second that their file mixed two vocabularies and asked them to undo
// a mixture that was not there.
func TestACardCarryingHalfOfEachVocabularyIsRefusedAsMixed(t *testing.T) {
	root, _, _, _, current := buildTreeFixture(t)
	ids := bench.ListIDs(filepath.Join(current, bench.CardsDir))
	if len(ids) == 0 {
		t.Fatalf("%s holds no cards, so this test asserts nothing", current)
	}
	mixOneCard(t, current, ids[0])

	got := runCLI(t, root, "--workbench", current, "ls")
	if got.code == 0 {
		t.Errorf("a workbench holding a card in both vocabularies listed and exited 0:\n%s", got.out)
	}
	if !strings.Contains(got.errw, contract.VocabularyMixed) {
		t.Errorf("the refusal is %q, wanted %s", got.errw, contract.VocabularyMixed)
	}
	if strings.Contains(got.errw, contract.VocabularyRetired) {
		t.Errorf("a card holding half of each vocabulary was refused as a retired one:\n%s", got.errw)
	}
}

// mixOneCard puts the retired substate key on a card that already carries the
// current column key, which is one header holding half of each vocabulary.
func mixOneCard(t *testing.T, root, id string) {
	t.Helper()
	rewriteFile(t, filepath.Join(root, bench.CardsDir, id, bench.CardAnchor), func(text string) string {
		return strings.Replace(text, "\nstate: ", "\nsubstate: ready\nstate: ", 1)
	})
}

// columnOfCard reads the column identifier a card stands in off the file, so
// the listing check compares against the value on disk rather than against a
// literal this test would have to keep in step with the fixture.
func columnOfCard(t *testing.T, root, id string) string {
	t.Helper()
	text, err := os.ReadFile(filepath.Join(root, bench.CardsDir, id, bench.CardAnchor))
	if err != nil {
		t.Fatalf("read the card %s: %v", id, err)
	}
	fm, _ := bench.ParseAnchor(string(text))
	for _, key := range []string{"column", "state"} {
		if value := fm.Value(key); value != "" {
			return value
		}
	}
	t.Fatalf("the card %s carries neither a column nor a state key, so this fixture is not the shape the test needs", id)
	return ""
}
