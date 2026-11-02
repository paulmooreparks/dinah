# Claude model choice matching

The reported note comes from literal option-value matching during restoration. The stored Claude catalog and substitution both associate fable with the display name Fable 5.1. A later offered catalog can use claude-fable-5-1 for that same display name.

[ACP config options](https://agentclientprotocol.com/protocol/session-config-options) supplies selectable value IDs and display names, including grouped choices. It does not supply an alias-resolution field. Claude Code documents aliases, but their targets depend on provider, environment overrides and version. Dinah must not invent an alias-to-version table or derive identity from an ID prefix. The newer SDK's documented resolvedModel field is not exposed by this ACP configuration contract.

For Claude model restoration, exact offered IDs take precedence. Otherwise Dinah may resolve a saved display name, or the provider's previously recorded display name for that saved value, to one unambiguous current offered value. It sends only that offered value and requires the usual provider confirmation. Equivalent name matching occurs before stored substitutions and emits no substitution note. Duplicate names with different IDs remain ambiguous. Unknown choices retain the existing explicit-substitution or unavailable-choice path.

Session settings can then retain the confirmed offered ID through ordinary option persistence. Provider defaults and stored substitutions are not rewritten: replacing a moving alias in global settings with a pinned version could change future model selection. No live settings migration or provider call is required for this fix.

The focused cases cover a saved alias with its recorded display name, a full offered ID, a direct display-name choice, a genuinely unknown model, and ambiguous display names. Existing substitution behavior for other providers remains unchanged.

The focused desktop and sessionhost checks passed in 1.630 seconds and 0.128 seconds on 2 November. The command used scripts/go-gate.ps1 for those packages with filter Test(ClaudeModel|ClaudeEquivalent|StoredSubstitution|RefusalCarries|RestoreModel|Reopening), count 1. Tests also reject an unrecorded alias rather than guessing its target. Compilation and whitespace checks passed; no broad gate, live setting change or restart occurred. Release owns installation from the exact scoped tip.
