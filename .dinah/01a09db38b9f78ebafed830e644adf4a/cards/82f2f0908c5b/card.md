---
title: A whole-tree guard holds every directory read to answering its own failure
column: 5ea2db0272fc
state: ready
severity: major
priority: soon
links:
  - kind: spawned_from
    to: 374600f703c6
---
The fix on dinah-439 makes a directory-read failure impossible to mistake for an empty directory at the fifteen reads that exist in the tree today. Nothing there stops the sixteenth. A caller writing `ids, _ := ListIDs(dir)` restores the defect with the compiler's blessing, and that door has stood open on `ListWorkbenchIDs` since dinah-433 closed the listing function. This card is the repository-wide check that holds every directory read in the tree to answering its own failure. It was split off dinah-439 on 2026-09-11, by the operator's ruling, after five design reviews.

The split happened because a static analysis over the whole tree is a different kind of work from the fix it was riding with. Every one of those five reviews found something real, and every defect found was a branch in the analyser that condemned correct Go with no example anywhere in the current tree, which is precisely why no run over the current tree could see it. Kept on one card, the analysis held a fix the operator had already paid for five times over.

Whoever picks this up starts where round five ended rather than at the beginning. The approach that worked is enumeration: parse every non-test `.go` file in the tree, collect the reads from the syntax trees, and derive the rules from what this repository actually writes rather than from shapes somebody pictured. Two programs carry that work, an enumerating program and a rule-5 prototype. Both live as fenced source in `docs/specs/dinah-486-directory-read-shapes.md` on this card's branch, together with what each printed at `65a80ad`, moved there off dinah-439's branch byte-identical except for a status header. That header names the three defects round five left standing in them, and the criteria and the decision on this card carry the same three.

Every defect found in five reviews was a branch in the analyser condemning correct Go, with no example anywhere in the current tree and therefore invisible to any run over the current tree. Each round measured the prototype against the tree, found it clean, and shipped a branch nobody could falsify that way; the next reviewer then wrote three lines of correct Go by hand and watched the branch fire on them. The reviewer's answer to what would break that pattern is the most valuable sentence five rounds produced: the analyser needs a corpus of correct code it must stay silent on, rather than the handful of hand-written samples it has now. Building that corpus is this card's real work.

The figures this analyser pins are figures about the fixed tree, so they are measured after dinah-439 lands. That is an ordinary dependency rather than a blocker, because the design can be written at any time and only the numbers wait.

## Working notes

Scratch, carried from dinah-439's round-five design review (comment 7467 on that card) so that this card's Spec stage has the corrected-program text without re-reading the review.

CORRECTED PROGRAM TEXT for the two F1 arms, as the reviewer wrote it.

(a) In program 2's AssignStmt arm, delete the `found` variable and the `if !found { report(..., "the read's error is bound but no branch tests it") }` block. Rule 5 asks its question of statements lying on a failure path, and a bound error that no branch tests produces no failure path, so rule 1 is the rule that answers. The arm also fires where the error IS read, because it looks only for an `if`.

Two correct sources it condemns today:

	func boundAndReturned(root string) ([]os.DirEntry, error) {
		entries, err := os.ReadDir(root)
		return entries, err
	}

	func boundAndWrapped(root string) ([]os.DirEntry, error) {
		entries, err := os.ReadDir(root)
		if entries == nil {
			return nil, fmt.Errorf("read %s: %w", root, err)
		}
		return entries, fmt.Errorf("read %s: %w", root, err)
	}

(b) In the same arm, capture the index of the testing `if` in the inner loop and pass the statements AFTER it to `checkPath`, matching what the `IfStmt` arm does with its `rest`. Today the `successBranch` case with no `else` calls `checkPath(list[i+1:], fn, rel, readPos, true)` where `i` indexes the assignment, so the slice opens with the testing `if`, `walk` descends into `t.Body.List`, and the success branch's returns are read as failure-path exits.

The correct source it condemns today, on line 10, which is the success branch:

	func positiveFormDoneRight(root string) ([]os.DirEntry, error) {
		entries, err := os.ReadDir(root)
		if err == nil {
			return entries, nil
		}
		return nil, err
	}

Re-run both programs after the corrections and record the output, because the recorded output has to be reproducible from the source printed beside it in the companion document.

F2's corrected wording for the document, to land after the sentence ending "so rule 1 is the rule that reaches it":

	The analysis descends five statement kinds: an assignment, an `if` with its init, body and `else` block, a `for`, a `range` and a bare block. A read bound in a `switch`, type-switch or `select` case body, in a labelled statement, or in the body of a function literal is not analysed, and none of those binds nothing. None is in the tree at `65a80ad`, where the analysis reaches all 18 of the 55 examined calls that bind an error, the other 37 being 35 `ListIDs` calls ranged over or nested in another call and the two `filepath.WalkDir` calls at `container.go:705` and `container.go:799`. The population printed above the finding count is the census's rather than the analysis's, so a read bound in one of those three positions is counted as examined and analysed by nothing.

The reviewer's probe for F2 is one file holding four swallows, one in a `switch` case, one in a `select` case, one in a `WalkDir` callback and one at plain statement level. Program 2 prints EXAMINED HALF 1: 5 and reports the plain one alone.

WHERE THE OLD MATERIAL LIVES. dinah-439's checklist carries the guard's two criteria as obsolete items 17915 and 17916, and its decision item 17897 holds a 16,656-character note of round-two-through-round-five history preserved byte for byte. Read those rather than re-deriving them.

## Branch

dinah-486-a-whole-tree-guard-holds-every-directory-read-to-answering-its-own-failure
