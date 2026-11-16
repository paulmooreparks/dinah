# Mention notification expectation correction

Paul approved the behavior change recorded under "Mentions are not decisions" in `docs/design/decisions-redesign.md`. The notification suite now expects Open in Chat, an exact marked target before acknowledgement, no mention row in Decisions, and a saved mention that remains pending when its group opens without a target. Generic card dismissal remains separately covered.

The four-suite selection covering mention notifications, mention navigation, Decisions attention entry and Decisions list passes in 56 seconds. Evidence is in the private temporary folder `dinah-mention-notification-approved-change`. The branch starts at the lead's d5c5309f merge candidate.

The wider scan found stale expectations in the manual `attention-hidden.cjs` and `decisions-hidden.cjs` audits. Those require actual desktop binaries and their isolated agent fixture. They remain outside this browser selection and will be ported next. Historical answer-origin rendering of a previously answered mention remains compatibility coverage.

The separate wording correction changes the empty-state explanation to say that questions from chats and tasks appear in Decisions. Its matching redesign assertion and mention-notification suite pass in 36 seconds. Evidence is in the private temporary folder `dinah-mention-empty-copy-correction`. This commit changes no host behavior or layout. Lead approved both commits and assigned the two manual audit ports before cleanup resumes.
