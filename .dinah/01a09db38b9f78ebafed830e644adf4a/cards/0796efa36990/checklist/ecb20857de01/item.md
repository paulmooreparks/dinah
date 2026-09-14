---
kind: decision
state: resolved
column: 4b38abe7ebd5
owner: holder
ts: 2026-09-14T02:18:26Z
ordinal: 1
note: "The diff rewrites the `context` member of `param.file.column.summary` in all eight catalogs (af, cs, de, en, es, fil, hi, id) and leaves every `text` byte-identical. The old context said \"because nothing reads it for enforcement yet\", which dinah-450's hold made false; the new one says the value has to name a column the workbench declares, because `file` resolves the reference to that column's identifier and a column gate matches on the identifier. No translated string was retranslated and none needs to be: the German and Hindi `text` values still render the same English sentence, and their `source` fingerprints (`a01f90ccb5da238c`) still match the unchanged English, so neither is stale. The glossary sweep is not re-run either, since no English `text` was added or changed. The context is translator guidance rather than shipped copy; `internal/msg/msg_test.go:56` and `internal/msg/address_keys_test.go:41` require only that it be non-empty, and nothing in the tree enforces cross-catalog context equality, which is why all eight were corrected together rather than English alone."
---
param.file.column.summary: read against the current English and its context; unchanged because the diff touches only the translator context, not the English text.