---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:07Z
ordinal: 17
note: "dinah-456 section 3.4 drafts the splice as `dinah cite {ref} attachment <id>`. `attachment` there is an evidence scheme, and schemes are declared per workbench: `Bench.EvidenceObservedRequired` reads them out of the workbench frontmatter, and a workbench made by `dinah init` carries no evidence block at all, which the probe workbench confirms. `dinah cite` accepts any scheme string and reports an undeclared one as a check finding rather than a refusal, so the drafted sentence would send a reader on a path that silently produces a finding. The splice therefore names the two steps and the command, and the reader gets the arguments from `dinah help cite`. `<id>` is also not a declared parameter name, and `TestEveryPlaceholderNamesSomethingDeclared` fails an angle placeholder no command declares."
---
The item advice names `dinah cite` and no evidence scheme, because a workbench declares its own schemes and a new one declares none.