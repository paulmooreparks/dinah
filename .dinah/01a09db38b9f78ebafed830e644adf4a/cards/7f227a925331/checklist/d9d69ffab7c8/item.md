---
kind: decision
state: resolved
owner: holder
ts: 2026-09-14T02:18:44Z
ordinal: 23
note: "The profile constrains the abstract model and docs/design/format.md's Versioning section already calls the on-disk layout Dinah's private business, so a statement naming a file would put storage into the interchange contract. CORE-QUEUE-3 already leans on the creation ordinal and nothing requires it to be unique, which is the actual hole. CORE-CARD-1 constrains the hex identifier and is untouched, so the amendment retires nothing and takes a minor increment, which is 0.14 per D-13 and the spec body. The exact published sentence is \"Every card MUST carry a creation ordinal unique within its workbench.\", settled in round 3 and written into the spec as the text that ships. Round 2 proposed \"No two cards in one workbench MUST carry one creation ordinal\", which denies an obligation rather than forbidding a shared ordinal; RFC 2119 has no \"No two X MUST Y\" form and none of the document's 138 statements opens with \"No\", \"Two\" or \"Neither\". The wording chosen is CORE-CARD-1's shape (:534) with one noun changed, matching CORE-STATE-1 (:485) and CORE-STATE-10 (:505). Nothing mechanical would have caught the error: extract.go rules on shape alone and TestHouseStyle (internal/profile/extract_test.go:575) bans only dashes and smart quotes, and DOC-CHG-1 forbids editing the sentence once published, so it has to be right on the way in."
---
The profile statement is CORE-CARD-10 about the creation ordinal's uniqueness, and it says nothing about a registry.