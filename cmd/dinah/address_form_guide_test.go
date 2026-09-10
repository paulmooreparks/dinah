package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"dinah/internal/addressform"
	"dinah/internal/guide/guidepin"
)

// referencesGuideTopic is the guide every address form is taught in.
const referencesGuideTopic = "references"

// TestEveryDeclaredAddressFormIsTaughtByTheReferencesGuide holds each declared
// form to a sentence the references guide still carries.
//
// guidepin.Carries folds both the guide text and the pinned string with
// strings.Fields before it searches, so a re-wrap of the guide is not a
// failure and a sentence split across two source lines is still found. That
// matters here more than it looks: this board has shipped a prose check that
// searched a document for a phrase the document wraps across lines, so it
// passed before anything was edited.
func TestEveryDeclaredAddressFormIsTaughtByTheReferencesGuide(t *testing.T) {
	roster := addressform.Declarations()
	if len(roster) == 0 {
		t.Fatal("the address-form roster is empty, so this sweep read nothing")
	}

	checked := 0
	for _, entry := range roster {
		if strings.TrimSpace(entry.Guide) == "" {
			t.Errorf("the form %q pins no guide text, so nothing holds the guide to teaching it", entry.Form)
			continue
		}
		if err := guidepin.Carries(referencesGuideTopic, entry.Guide); err != nil {
			t.Errorf("%s: %v", entry.Form, err)
			continue
		}
		checked++
	}
	if checked != len(roster) {
		t.Errorf("%d of the %d declared forms were found in the %s guide, so this sweep read less than the roster it claims", checked, len(roster), referencesGuideTopic)
	}
	t.Logf("%d declared forms held against the %s guide", checked, referencesGuideTopic)
}

// sharedGuideSentence is one guide string and the forms that pin it.
type sharedGuideSentence struct {
	// sentence is the pinned text.
	sentence string
	// forms are the declared forms pinning it, in roster order.
	forms []addressform.AddressForm
}

// TestTheSharedGuideSentencesAreTheDeclaredOnes holds the shape of the roster's
// overlap to exactly the three groups this card declares.
//
// Several forms share one guide sentence, which is a real weakness declared
// rather than hidden: a surviving sentence does not tell you which member of
// its group it teaches. What separates them is the running guard, which
// asserts which entity each spelling resolved to. This check is what stops a
// fourth group appearing, which is how a form would quietly join an existing
// sentence rather than earning one.
//
// The three shared strings are quoted here rather than cited by line number. A
// line number in a check goes stale the first time the guide gains a
// paragraph, and this repository already carries a fixture family keyed on
// source lines that proves the point.
func TestTheSharedGuideSentencesAreTheDeclaredOnes(t *testing.T) {
	roster := addressform.Declarations()
	if len(roster) == 0 {
		t.Fatal("the address-form roster is empty, so this sweep read nothing")
	}

	grouped := map[string][]addressform.AddressForm{}
	var order []string
	for _, entry := range roster {
		if _, seen := grouped[entry.Guide]; !seen {
			order = append(order, entry.Guide)
		}
		grouped[entry.Guide] = append(grouped[entry.Guide], entry.Form)
	}

	var shared []sharedGuideSentence
	for _, sentence := range order {
		if len(grouped[sentence]) > 1 {
			shared = append(shared, sharedGuideSentence{sentence: sentence, forms: grouped[sentence]})
		}
	}

	want := []sharedGuideSentence{
		{
			sentence: "You write a column as its slug, its name, or its identifier",
			forms: []addressform.AddressForm{
				addressform.ColumnSlug, addressform.ColumnTitle, addressform.ColumnIdentifier,
			},
		},
		{
			sentence: "You write a workstream as the word workstream, a slash, and the workstream's slug or its identifier",
			forms: []addressform.AddressForm{
				addressform.WorkstreamPrefixedSlug, addressform.WorkstreamPrefixedIdentifier,
			},
		},
		{
			sentence: "The commands that take a workstream also accept the slug or the identifier on its own",
			forms: []addressform.AddressForm{
				addressform.WorkstreamBareSlug, addressform.WorkstreamBareIdentifier,
			},
		},
	}

	if len(grouped) != 13 {
		t.Errorf("the %d declarations pin %d distinct guide strings, and thirteen is what the roster declares", len(roster), len(grouped))
	}
	if len(shared) != len(want) {
		t.Fatalf("%d guide strings are pinned by more than one form and %d are declared: %v", len(shared), len(want), sharedSentenceNames(shared))
	}
	for index, group := range want {
		got := shared[index]
		if got.sentence != group.sentence {
			t.Errorf("the shared sentence at position %d is %q, wanted %q", index+1, got.sentence, group.sentence)
			continue
		}
		if !sameForms(got.forms, group.forms) {
			t.Errorf("the sentence %q is shared by %v, wanted %v", group.sentence, got.forms, group.forms)
		}
	}
	t.Logf("%d declarations over %d distinct guide strings, %d of them shared", len(roster), len(grouped), len(shared))
}

// sharedSentenceNames answers the form groups of a shared set, for a failure
// that has to say what it found rather than only what it wanted.
func sharedSentenceNames(shared []sharedGuideSentence) [][]addressform.AddressForm {
	groups := make([][]addressform.AddressForm, 0, len(shared))
	for _, group := range shared {
		groups = append(groups, group.forms)
	}
	return groups
}

// sameForms compares two form lists as sets, since what a group holds is the
// claim and the order within it is not.
func sameForms(got, want []addressform.AddressForm) bool {
	if len(got) != len(want) {
		return false
	}
	left := append([]addressform.AddressForm(nil), got...)
	right := append([]addressform.AddressForm(nil), want...)
	sort.Slice(left, func(i, j int) bool { return left[i] < left[j] })
	sort.Slice(right, func(i, j int) bool { return right[i] < right[j] })
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

// TestTheReferencesGuideNoLongerAddressesACardByItsHoldersIdentifier holds the
// narrowed sentence in both of the directions a rename can go wrong.
//
// The retired wording is asserted absent as well as the new wording present,
// because the failure this guards against is an edit that adds the corrected
// sentence and leaves the old one standing a paragraph away. That has happened
// on this board, in this guide.
func TestTheReferencesGuideNoLongerAddressesACardByItsHoldersIdentifier(t *testing.T) {
	retired := retiredMemberIdentifierWording
	if err := guidepin.Carries(referencesGuideTopic, retired); err == nil {
		t.Errorf("the %s guide still carries %q, which reads as though a card were reached by substituting its identifier into its reference", referencesGuideTopic, retired)
	}
	for _, sentence := range []string{
		"You may write a collection member's own identifier in place of its number",
		"You do not address a card this way.",
	} {
		if err := guidepin.Carries(referencesGuideTopic, sentence); err != nil {
			t.Errorf("%v", err)
		}
	}
}

// retiredMemberIdentifierWording is the sentence dinah-471 narrowed, kept in
// one place so the sweep below and the check above read the same string.
const retiredMemberIdentifierWording = "You may write an entity's own identifier in place of its number"

// retiredWordingSites are the two files allowed to carry the retired sentence,
// named rather than matched by a pattern, so a third copy appearing anywhere
// fails rather than being absorbed.
//
// The first is a design sketch that was retired rather than shipped, and
// rewriting a retired sketch would falsify the record of what was proposed.
// The second is this file, which has to spell the sentence in order to assert
// its absence.
var retiredWordingSites = map[string]bool{
	"docs/specs/dinah-172-help-ux-sketch.md": true,
	"cmd/dinah/address_form_guide_test.go":   true,
}

// TestTheRetiredMemberIdentifierWordingStandsOnlyWhereItIsAllowed sweeps the
// three trees that carry shipped prose about addressing and holds the retired
// sentence to the two files that may still spell it.
//
// Searching a document for a phrase finds copies of the phrase rather than
// copies of the claim, so this is a backstop rather than proof: a paragraph
// restating the same wrong claim in other words is invisible to it, and a
// reader is what catches that. What it does catch is the copy-and-paste
// failure, which is the one that has actually happened here.
func TestTheRetiredMemberIdentifierWordingStandsOnlyWhereItIsAllowed(t *testing.T) {
	folded := strings.Join(strings.Fields(retiredMemberIdentifierWording), " ")
	trees := []string{"internal/guide/guides", "docs", "cmd/dinah"}

	read := 0
	var carrying []string
	for _, tree := range trees {
		root := filepath.Join(repositoryRoot, filepath.FromSlash(tree))
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			name := entry.Name()
			if !strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			read++
			if !strings.Contains(strings.Join(strings.Fields(string(data)), " "), folded) {
				return nil
			}
			relative, err := filepath.Rel(repositoryRoot, path)
			if err != nil {
				return err
			}
			carrying = append(carrying, filepath.ToSlash(relative))
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", tree, err)
		}
	}
	if read == 0 {
		t.Fatalf("no .md or .go file stands under %v, so this sweep read nothing", trees)
	}

	sort.Strings(carrying)
	for _, site := range carrying {
		if !retiredWordingSites[site] {
			t.Errorf("%s carries %q, which reads as though a card were reached by substituting its identifier into its reference", site, folded)
		}
	}
	for site := range retiredWordingSites {
		if !contains(carrying, site) {
			t.Errorf("%s is named as a place the retired sentence still stands and it no longer carries it, so this sweep is holding a site that has moved", site)
		}
	}
	t.Logf("%d files read over %v, %d carrying the retired sentence: %v", read, trees, len(carrying), carrying)
}

// contains reports whether a sorted list of paths holds one.
func contains(list []string, want string) bool {
	for _, entry := range list {
		if entry == want {
			return true
		}
	}
	return false
}
