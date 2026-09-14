---
title: "the git guard fails open: any exception in its decision path lets the command through"
column: 5ea2db0272fc
state: ready
severity: major
priority: soon
tier: workhorse
---
The destructive-git hook refuses a command by printing a JSON deny decision on stdout. It permits by printing nothing. Those are the same observable when the guard crashes, so an exception anywhere in its decision path is indistinguishable from a verdict of "this command is fine", and the command runs.

Read `main()` in `scripts/hooks/deny-destructive-git.py`. Its only `try` wraps `json.load(sys.stdin)`, and its comment says an unparseable payload should stay out of the way, which is a defensible ruling for a payload nobody wrote. Everything after that runs unguarded, including the whole of `decide(command)`, where the parsing, the span analysis, the quote handling and the worktree classification all live. That is several hundred lines of text processing over a string the guard did not choose.

## How this was found, corrected

An earlier version of this description said dinah-291's proposed fix would have introduced such a crash, by ordering a normalising call ahead of the check for whether `-C` carried a value. That claim was wrong and is withdrawn. The live hook already performs that check first, and the second design review on dinah-291 confirmed it against the source. The defect existed only in an illustrative snippet inside that card's spec, which the design stage caught before any implementation copied it.

What survives the correction is the property itself, which was read directly off `main()` rather than inferred from that card: the guard has no handler around its decision path, and its permit signal is silence. No crafted input is known to reach it today. This card exists because the consequence of one arriving is a silent bypass rather than a visible failure, and nothing on this board would report it.

The correction is worth keeping on the record rather than quietly editing away, because it is the same shape as the defects this repository has been finding all week: a claim written down once, carried forward by everyone who read it, and true of nothing.

## Why this is worth a card of its own

A guard that refuses too much is annoying and visible. That is how dinah-291 was found: three agents reported being blocked and one of them filed the card. A guard that permits too much is silent, and no agent reports work it was allowed to do.

## What the fix has to settle

**What a guard does when it cannot decide.** Failing closed on an unparseable command is the safe reading and it has a cost, since a crash then blocks every git command until somebody repairs the hook. Failing open is what happens today and it has a worse cost that nobody sees. A third shape exists, refusing only what the guard would have classified as mutating had it got that far, and that may not be knowable at the point the exception fires.

**Whether the stdin ruling stays.** An unparseable payload is a different case from an exception midway through a decision, and the existing comment gives a reason for the first that does not obviously carry to the second.

**Whether a crash should be visible.** Today it is not, beyond whatever the harness does with a non-zero exit. A guard that silently stops guarding is worse than one that stops loudly.

**Whether the test suite can express this.** `scripts/hooks/test-deny-destructive-git.py` tests verdicts. A test that a crafted command does not crash the guard is a different shape, and the suite may need a way to assert that the guard reached a verdict at all rather than which verdict it reached.

## Related

dinah-291 repairs the path classifier in this same hook and is where this property was read.
