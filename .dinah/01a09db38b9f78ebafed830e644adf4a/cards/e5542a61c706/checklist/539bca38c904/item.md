---
kind: acceptance_criterion
state: pending
column: c9428b3bc921
ts: 2026-09-14T02:16:58Z
ordinal: 6
---
.claude/settings.json registers scripts/hooks/deny-main-checkout-cwd.py as a PreToolUse entry under the existing Bash|PowerShell matcher, alongside (not replacing) the deny-destructive-git.py entry. Verified by parsing the file as JSON and asserting both command strings are present among the PreToolUse hooks' command fields.