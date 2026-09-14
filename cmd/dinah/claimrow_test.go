package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/msg"
	"dinah/internal/profile"
	"dinah/internal/verb"
)

// claimRowSentence is CORE-CLAIM-10's row of the claim's ordered list, in the
// profile's own words. It is written out here rather than read from one of the
// places below, because every place below is checked against it and a sentence
// read from one of them would compare that place with itself.
const claimRowSentence = "every unresolved item the card carries names a declared column"

// retiredClaimRowSentence is the row CORE-CLAIM-9 published, which no shipped
// catalogue may still carry.
const retiredClaimRowSentence = "the card carries no structured item that is not resolved"

// localesDir is the directory of shipped catalogues, named from this package.
const localesDir = "../../internal/msg/locales"

// shippedCatalogues is how many catalogue files ship. A sweep that read none
// would report success, so each half below states the number it read and this
// is what it is held to.
const shippedCatalogues = 8

// TestTheClaimsSeventhRowReadsOneSentenceEverywhere holds the four places the
// claim's seventh row is published to one another: the profile's own fence in
// section 6.3, the two catalogue keys that quote it, and the help the tool
// renders for claim and for pull.
//
// TestPerCommandHelpFollowsTheProfile already compares the rendered row
// against the document for every contract verb, so it is what fails when one
// of the three drifts. This test names the sentence, which is what says which
// way a drift went, and it reaches the rendered help of claim and of pull,
// where that test renders move alone.
func TestTheClaimsSeventhRowReadsOneSentenceEverywhere(t *testing.T) {
	document, err := os.ReadFile(filepath.Join("..", "..", "docs", "spec", "core-profile.md"))
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	rows := profile.Preconditions(string(document))[verb.Claim]
	if len(rows) < 7 {
		t.Fatalf("the profile's claim list carries %d rows, so it has no seventh", len(rows))
	}
	if rows[6].Check != claimRowSentence {
		t.Errorf("the profile's seventh claim row reads %q, wanted %q", rows[6].Check, claimRowSentence)
	}

	catalog := msg.For(msg.Base)
	// The pull list's copy of this row moved from 15 to 16 at dinah-498,
	// which inserted the destination's field requirement above it.
	for _, key := range []string{"check.claim.7", "check.pull.16"} {
		if rendered := catalog.T(key); rendered != claimRowSentence {
			t.Errorf("%s renders %q, wanted %q", key, rendered, claimRowSentence)
		}
	}

	root := newBench(t)
	for _, command := range []string{"claim", "pull"} {
		got := runCLI(t, root, "help", command)
		if got.code != 0 {
			t.Fatalf("help %s: %d %s", command, got.code, got.errw)
		}
		if !strings.Contains(got.out, claimRowSentence) {
			t.Errorf("dinah help %s does not carry the row: %s", command, got.out)
		}
		if strings.Contains(got.out, retiredClaimRowSentence) {
			t.Errorf("dinah help %s still carries the retired row", command)
		}
	}
}

// TestEveryCatalogueCarriesTheNarrowedClaimRow sweeps the shipped catalogues
// for the two keys that quote the claim's seventh row and for the sentence
// CORE-CLAIM-9 published.
//
// Each half states the number of files it read and each half is fatal on a
// count that misses, because a sweep whose directory had moved would read
// nothing and report exactly what a clean sweep reports. The halves are
// counted separately rather than together: one number satisfied by the keys
// alone would stay whole while the retired sweep read nothing at all.
//
// The retired half reads the file's bytes rather than the two keys, because
// what it is looking for is the sentence surviving anywhere in a catalogue,
// including under a key this test does not name.
func TestEveryCatalogueCarriesTheNarrowedClaimRow(t *testing.T) {
	entries, err := os.ReadDir(localesDir)
	if err != nil {
		t.Fatalf("read %s: %v", localesDir, err)
	}

	carrying := 0
	clean := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(localesDir, entry.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		text := string(body)

		missing := false
		for _, key := range []string{`"check.claim.7"`, `"check.pull.15"`} {
			if !strings.Contains(text, key) {
				t.Errorf("%s carries no entry under %s", entry.Name(), key)
				missing = true
			}
		}
		if !missing {
			carrying++
		}

		if strings.Contains(text, retiredClaimRowSentence) {
			t.Errorf("%s still carries the sentence CORE-CLAIM-9 published", entry.Name())
			continue
		}
		clean++
	}

	if carrying != shippedCatalogues {
		t.Fatalf("%d catalogues carry both keys, wanted %d", carrying, shippedCatalogues)
	}
	if clean != shippedCatalogues {
		t.Fatalf("%d catalogues are free of the retired sentence, wanted %d", clean, shippedCatalogues)
	}
}
