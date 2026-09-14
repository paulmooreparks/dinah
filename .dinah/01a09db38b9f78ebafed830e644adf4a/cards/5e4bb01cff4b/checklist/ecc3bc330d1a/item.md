---
kind: acceptance_criterion
state: verified
column: c9428b3bc921
ts: 2026-09-14T02:17:00Z
ordinal: 2
note: "Test-stage re-verification. `go test -run TestATranslationUsesTheDeclaredWord -v ./internal/msg/` PASS. Armed independently: planted \"Eigentuemer\" for \"Akteur\" in de/check.attach.2, guard failed naming exactly that key (\"wanted the glossary word for \\\"owner\\\" (one of [Akteur]), got ...\"), restored byte-identical (git diff --stat confirmed 1 insertion/1 deletion before restore, 0 after), green again."
---
internal/msg/glossary.json and internal/msg/glossary.go declare the glossary type and loader; internal/msg/msg_test.go's new TestATranslationUsesTheDeclaredWord runs one subtest per tag in Tags() other than Base, reading each translated entry through the new CatalogEntry accessor and skipping Skeleton entries, strips {...} placeholders and skips context-declared-untranslatable entries before matching, and fails an entry whose text carries none of its triggered term's declared forms, naming the key, the term, the accepted forms, and the entry's actual text.