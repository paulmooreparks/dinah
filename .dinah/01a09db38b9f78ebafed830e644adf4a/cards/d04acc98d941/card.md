---
title: An attachment's description is thrown away on the way in over MCP
column: 5ea2db0272fc
state: ready
severity: major
priority: next
---
`attach` declares a `description` parameter, so the MCP head advertises it in the tool's input schema and any agent reading that surface will send it. `request2Args` in `internal/mcp/mcp.go` looks the argument up by name and hands it to `assignValue`, whose switch has no case for `description`. The value falls off the end and the attachment is created with none.

Nothing fails. The call succeeds, the response is the canonical shape, and the agent has no way to tell that the description it supplied was discarded. `verb.Request` carries the field (`Description string`, "an attachment's optional description"), and the command-line head fills it in, so the two surfaces disagree about what `attach` accepts while both claiming the same parameter table.

The fix is one case in `assignValue`. What is worth a moment's thought is whether anything else should change: the schema is generated from `verb.Params` and the assignment is written by hand, so the generator and the consumer are only kept in step by somebody remembering. dinah-181 added `TestEveryDeclaredParameterReachesTheRequest` in `internal/mcp/mcp_test.go`, which drives every declared parameter of every tool through `request2Args` and fails when one lands nowhere. This defect is named in that test's exemption table with a pointer to this card; the fix is to remove the entry along with the defect, and the entry failing to fail afterwards is the check working.

The same test names one deliberate case, `version.catalogs`, where the marker genuinely selects nothing.

Found while adding an end-to-end MCP test for `pull` on dinah-181, which had the same defect in its own `no-claim` marker and fixed it there.
