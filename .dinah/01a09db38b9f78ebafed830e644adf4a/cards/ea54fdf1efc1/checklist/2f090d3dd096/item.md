---
kind: decision
state: resolved
column: 4b38abe7ebd5
owner: holder
ts: 2026-09-14T02:18:13Z
ordinal: 44
note: "`dinah workbench set slug --yes fx-later` works on the trunk because `workbench` is one of five commands main.go excludes from `resolveOpenTailFlags`: dinah-100 bounds each to one free-text word, so the parser applies a domain flag correctly wherever it is typed and there is nothing left to correct. `set` inherits both properties and was not on the list, so the same invocation was refused `dinah.multiple-words` reading `--yes fx-later` as two words.\n\nIt is on the list now, and both positions work: the flag reads as a flag before the value and after it. `set` also declares `bounded: 2`, so its reference and its field are its own arguments and the tail begins at the value.\n\nI found this by building the trunk in a second worktree and running the two invocations side by side, rather than by reasoning about the parser, which I had got wrong twice."
---
`set` joins the five commands main.go holds out of the open-tail flag resolution, or a flag typed before the value is read as prose.