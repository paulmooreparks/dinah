---
title: The registry check is ungated on format, so every card of an unmigrated workbench is reported as having no number
column: 5ea2db0272fc
state: ready
severity: major
priority: now
---
Found on the operator's live customer data on 2026-09-13, on the binary built from `3ada380` (the commit that shipped dinah-488), while migrating a real workbench at his request.

## What happens

`dinah check` on a workbench declaring `format: 2` reports every one of its cards:

```
006a49c65700 has no line in the card-number registry, so no number names it
```

Ninety cards, ninety findings, on a workbench that is correct and simply has not been migrated yet. The sentence is also false in its second clause. A number does name that card: it is sitting in the card's own frontmatter, which is exactly where a workbench below the registry revision is supposed to keep it.

## Why

`checkCardNumbers` in `internal/bench/check.go` carries no format gate, and neither does its caller at `check.go:354`. Its sibling finding three lines further down is gated correctly:

```go
if !claimed {
    findings = append(findings, Finding{Path: anchor, Key: FindingCardNumberMissing, Detail: id})
}
if b.Format >= RegistryFormat && card.FM.Has("number") {
    findings = append(findings, Finding{Path: anchor, Key: FindingCardNumberInFrontmatter, Detail: id})
}
```

So one of the pair asks whether the registry binds on this workbench and the other does not. `RegistryFormat`'s own declaration in `bench.go` states the rule the missing half breaks: "A workbench declaring this number or a higher one carries its card numbers in card-numbers.txt; one declaring less carries them in card frontmatter and is read that way until it is migrated."

## Why it matters more than a noisy check

The people who meet this are the people mid-migration, which is the one population `dinah check` exists to help. On a real workbench the ninety findings bury anything else the run had to say, and the report tells an operator that every card in the workbench has lost its number, which is the most alarming thing the tool could say and it is not true.

It also fires on the exact shape this board has now met twice in a fortnight: a check applied outside the scope its own documentation states. The other half of the pair proves the author knew the gate was needed.

## What the fix probably is

Gate `FindingCardNumberMissing` on `b.Format >= RegistryFormat`, or gate the whole of `checkCardNumbers` at its caller, which also saves reading the registry on a workbench that has none. Whichever is chosen, the arming case is a format-2 workbench with correct frontmatter numbers: it fires ninety times today and must fire zero times after, while a format-3 workbench genuinely missing a registry line must still be reported. Pin both, because a gate written the natural way can silence the finding everywhere.

Worth checking the neighbouring findings in the same function for the same omission rather than fixing only the one that was seen: `FindingCardNumberStranded`, `FindingCardNumberMalformed`, `FindingCardNumberDuplicate` and `FindingCardNumberRepeated` all read `b.Numbers` and none of them is gated either. On a format-2 workbench that structure is empty, so they happen to stay silent, but that is luck rather than a rule, and the rule is what dinah-488's own constant says.

## Not affected

The migration itself is sound. Run against the same workbench it wrote ninety registry lines, renumbered nothing, stripped the number key from all ninety anchors and stamped the anchor to format 3, and the resulting number-to-card mapping is byte-identical to the mapping taken from the frontmatter beforehand. Only the check is wrong.
