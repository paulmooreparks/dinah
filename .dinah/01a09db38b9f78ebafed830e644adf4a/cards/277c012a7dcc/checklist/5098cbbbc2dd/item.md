---
kind: acceptance_criterion
state: verified
column: 6c5b9d6f4414
ts: 2026-09-14T02:18:19Z
ordinal: 1
note: Verified. `go test ./internal/verb/ -run 'TestEveryReferenceTakingCommandDeclaresItsKinds|TestNoReferenceTakingParameterAlsoDeclaresAVocabulary' -v -count=1` passes and logs "18 commands read" and "18 reference-taking parameters read" (internal/verb/reference_kinds_test.go:42 and :71 are the two t.Logf calls; the two t.Fatalf guards at :40 and :74 are the assertions that redden on a short count). Both plants armed and run. Deleting restore's entry from referenceKinds reddened at reference_kinds_test.go:28 "restore takes a reference and referenceKinds declares no kinds for it, so its help page would render no clause" plus :40 "referenceKinds carries 17 entries and the roster names 18 commands". Adding a `pull` entry reddened at :37 "referenceKinds declares kinds for pull and it is not on the references roster, so nothing renders them" plus :40 at 19 entries. Restored byte-identically from a backup after each (diff confirmed empty) and green again.
---
`go test ./internal/verb/ -run 'TestEveryReferenceTakingCommandDeclaresItsKinds|TestNoReferenceTakingParameterAlsoDeclaresAVocabulary' -v` passes and its log reports eighteen commands and eighteen reference-taking parameters read; deleting the `restore` entry from `referenceKinds` reddens the first naming `restore`, and adding a `pull` entry reddens it naming `pull`.