---
kind: decision
state: resolved
column: 0d86ad99cdbc
ts: 2026-09-14T02:18:37Z
ordinal: 23
note: No. The existing `--override` marker is already refused to anyone but the operator before either the entry or exit check runs. Reusing it for exit-hold costs nothing (it already can't be bought by a non-operator) and avoids the "two flags, a reviewer only checks one" shape a second, exit-specific flag would create.
---
Does exit-hold need its own override marker, separate from the operator's existing one?