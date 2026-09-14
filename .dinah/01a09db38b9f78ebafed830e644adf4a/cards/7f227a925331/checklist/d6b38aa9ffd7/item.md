---
kind: decision
state: resolved
owner: holder
ts: 2026-09-14T02:18:45Z
ordinal: 32
note: "Section 3.5 of the core profile binds its two excluded word lists to three places, and section 4 is one of them; the first list carries `merge`, and `TestExcludedTermsAreAbsent` matches whole words without regard to case. The round 4 wording, \"when two workbenches merge and two cards arrive holding one\", therefore could not have passed AC-11, which asserts that `go test ./internal/profile` passes. The shipping clause reads \"when two copies of one workbench are reconciled and two cards arrive holding the same one\", which says the same thing in the vocabulary the profile allows itself. Run on 2026-09-11 over a copy of the tree at C:/dinah-scratch/dinah-488-spec/profcheck: ok on the unmodified document, ok with the shipping entry spliced in after the **Field** entry, and `extract_test.go:223: the core vocabulary carries excluded terms [merge]` with the one clause reverted. The paraphrase prohibition follows from the same fact: any rewording risks walking back into either list, and DOC-CHG-1 makes the published text unamendable. The entry also drops round 4's closing cross-reference to CORE-QUEUE-3, because section 4 carries no CORE- identifier anywhere in its span and the statement already says the thing where it stands."
---
The section 4 entry states the reallocation case without the word `merge`, and the implementer copies the entry verbatim rather than paraphrasing it.