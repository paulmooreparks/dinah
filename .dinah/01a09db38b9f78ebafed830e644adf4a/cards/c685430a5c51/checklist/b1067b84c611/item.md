---
kind: decision
state: resolved
column: 0d86ad99cdbc
owner: holder
ts: 2026-09-14T02:18:06Z
ordinal: 14
note: "The parent says the refusal carries `ref` and `kind`. The head already fills the caller's reference into a slot called `detail`: `session.refusalValues` at cmd/dinah/render.go:1049 seeds the map with it, and `dinah.not-renamable` carries `entity.Ref` there today, which `dinah rename probe-1/oq/1 x.txt --json` shows as `\"detail\": \"probe-1/checklist/1\"` beside `\"context\": {\"kind\": \"item\"}`. Declaring a second value named `ref` holding the same string would put the reference twice into the machine payload and change nothing a reader sees. Only the placeholder spelling moves; the wording the parent minted is kept."
---
The base sentence spells the reference `{detail}` rather than the `{ref}` dinah-456 section 3.4 wrote, and the printed sentence is unchanged by that.