---
kind: decision
state: resolved
ts: 2026-09-14T02:18:05Z
ordinal: 24
note: "The workbench document \"Prose standard\" makes the serial comma a rule of the board's prose, names inserting it a safe transformation that changes no word, and governs the guides as user-facing documentation. The shipped guide corpus already carries it everywhere except the references guide's line naming the three short checklist spellings. Seven lists take the comma: the four things DinahPath leaves out, the workstream sentence's six names, the five checklist verbs, the caveat's three commands, the three short spellings, and the head list in each of the two seam paragraphs. Consequences, both verified at b825059: TestTheReferencesGuideIntroducesDinahPathWithItsCorrection asserts the comma'd phrase (AC-9), and the form literal at main_test.go:7176 was observed failing against the comma'd draft with \"the references guide does not teach\", so spec section 6 carries that edit. Leaving the call to the implementer was the alternative and it is worse, because the literal and the copy would then be settled by two people on different days."
---
Every list of three or more items in the shipped copy takes the serial comma, and the one existing test literal that pinned a comma-less list moves with it.