---
title: Library.Do fires Interleave too late to drive the race it exists for
column: 5ea2db0272fc
state: ready
severity: minor
priority: next
---
Narrowed twice, and this is what is left.

`blocked` and `held` exist for a card whose substate changed between a command choosing it and taking its lock. Testing that race needs a hook that fires inside the lock and before the command re-reads the card. `Library.Do`, which every contract verb runs through, acquires the lock, calls `bench.LoadCard`, runs `lapse`, and only then fires `Interleave`. The card the precondition list reads is therefore the one loaded before the hook ran, so a test that mutates the card inside the hook changes something the checks never look at. Only code that re-reads the filesystem afterwards can observe it, which is why a planted sibling lock file is the one race a test can currently drive.

dinah-181 fixed this for `pull` alone. Its own transaction now fires the hook after taking the lock and before loading the card, which is what made its AC-11 and AC-18 testable. `Library.Do` was deliberately left as it stands, so the five contract verbs still cannot be tested against the two rows that exist for the race, and a reordering of those rows relative to the rows above them still passes the whole suite.

Two things a fix has to settle. The first is whether `Do` adopts pull's placement, which is the obvious answer and which changes what every existing `Interleave` test drives, so each of those tests has to be read rather than assumed. The second is whether the two transactions should have differed in the first place: a hook that fires at one moment in `Do` and another moment in `pullTransaction` is a trap for whoever writes the next test against either.

Whichever lands should add, for `claim` and `move`, the tests dinah-181 was able to write for `pull`.

First raised at Implement on dinah-181 with the wrong cause, re-aimed after that card's Agent Code Review found the real one, and narrowed again once dinah-181 fixed pull's half.
