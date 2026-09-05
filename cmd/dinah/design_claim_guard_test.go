package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The design documents argued for years from a premise that no longer holds:
// that the hosted product was a second, independently coded implementation of
// the coordination contract, and that Dinah and it were kept apart by a rule
// against sharing code. Dinah.Team runs on Dinah's own library, so both
// premises are false, and two passages that reasoned from them were rewritten.
//
// This check holds the rewrite. It reads the wording each passage used and
// fails when that wording returns, which catches the copy an editor restores
// from an older draft and the sentence a later card writes back out of habit.
//
// What it protects is narrow, and reading it wider than this is a mistake. It
// catches the verbatim return of the wordings listed below, and it is robust
// only to case, to line wrapping and to whitespace. A paraphrase of the same
// claim in words this file has never seen walks straight past it, so the check
// is a backstop and only a reader catches the general case. The list therefore
// carries the spellings a writer would reach for rather than only the one that
// stood in the tree, which is why the unhyphenated form and the founding
// sentence's own wording are here beside the two retired passages.

// retiredClaim is one wording the design documents used to carry, together with
// the reason it may not come back.
type retiredClaim struct {
	// wording is written in lower case and matched against the document with
	// its whitespace collapsed and its letters lowered, so neither a rewrap
	// that moves the words across a line break nor a return of the claim at
	// the head of a sentence escapes the match.
	wording string
	// why says what replaced the wording, so a finding explains itself to
	// somebody who was not here when it was written.
	why string
}

// retiredSecondImplementationClaims are the wordings the arbiter rule in
// format.md and the language paragraph in surfaces.md used to carry.
var retiredSecondImplementationClaims = []retiredClaim{
	{
		wording: "the two implementations meet at the contract",
		why:     "Dinah.Team runs on Dinah's own library, so the arbiter rule is derived from one implementation with two storage backends rather than from two implementations meeting at three named surfaces",
	},
	{
		wording: "no-shared-code rule",
		why:     "no rule against sharing code exists, and the conformance suite pins Dinah's own behaviour against what the design documents and the profile say the contract requires",
	},
	{
		wording: "no shared code rule",
		why:     "the same retired rule spelled without its hyphens, which is how a writer reaching for it in ordinary prose spells it, and no rule against sharing code exists",
	},
	{
		wording: "kept honest by a shared conformance suite",
		why:     "this is the founding sentence's own wording, which still stands in the Dinah workbench's standing instructions and is therefore the likeliest text copied back into a design document; Dinah.Team shares Dinah's library, so the two are not two implementations kept honest against each other",
	},
}

// designDocuments are the documents those claims were argued in, named from the
// repository root so a finding tells the reader which file to open.
var designDocuments = []string{
	filepath.Join("docs", "design", "format.md"),
	filepath.Join("docs", "design", "surfaces.md"),
}

// TestTheDesignDocumentsDoNotArgueFromASecondImplementation asserts that no
// design document carries a wording the second-implementation premise was
// argued in.
func TestTheDesignDocumentsDoNotArgueFromASecondImplementation(t *testing.T) {
	read := 0
	for _, relative := range designDocuments {
		raw, err := os.ReadFile(filepath.Join(repositoryRoot, relative))
		if err != nil {
			t.Fatalf("%s: %v", relative, err)
		}
		flattened := strings.ToLower(flattenWords(string(raw)))
		if flattened == "" {
			t.Errorf("%s: the document is empty, so this check read nothing of it", relative)
			continue
		}
		read++
		for _, claim := range retiredSecondImplementationClaims {
			at := strings.Index(flattened, claim.wording)
			if at < 0 {
				continue
			}
			t.Errorf("%s: the retired claim %q stands in %q, and %s", relative, claim.wording, around(flattened, at, len(claim.wording)), claim.why)
		}
	}
	if read != len(designDocuments) {
		t.Errorf("%d of the %d design documents were read, so this check proves less than it claims", read, len(designDocuments))
	}
}

// around returns the run of text a finding quotes, which is the match itself
// with the words on either side of it, so a reader recognises the sentence
// without opening the file. The quotation comes back lowered, because the
// search that found it reads a lowered copy of the document.
func around(text string, at, length int) string {
	const margin = 70
	start := at - margin
	if start < 0 {
		start = 0
	}
	end := at + length + margin
	if end > len(text) {
		end = len(text)
	}
	return text[start:end]
}
