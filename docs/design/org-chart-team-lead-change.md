# Team lead changes

Paul approved the redone chart with the lead rule recorded in docs/plans/org-chart-view.md at 9dcce671. A current lead cannot be assigned to another team. Reassigning every reporting chat ends its leadership; replacing it with a member also makes the former lead assignable. This host stage changes no UI.

Front End accepted the request in message 9504. OrgChartActions offers replace_team_lead on an eligible current member, labelled Make Team Lead…. ApplyOrgChartAction accepts kind, the new lead's canonical chatId, the team's stable groupId and the offered token. It accepts no member list or separate leadId. The existing make_lead operation continues to convert an ordinary group.

The host rechecks the expected old lead and the new lead's membership inside one bus transaction. The new lead leaves its reporting row, the other reporting rows point to it, and the old lead becomes a member. The team channel keeps its stable identity, history and membership. Ordinary groups are unchanged. A stale request or failed write rolls back every change. Team allowances follow the retained team; an existing allowance configuration on the new lead refuses replacement rather than overwriting it.

The new lead must not already lead another team, including an older nested team, as Lead specified in message 9503. Existing nested records remain readable, but the new operations refuse creating a second team role. An empty former team channel becomes an ordinary group when its final reporting member leaves, so the former lead can be assigned elsewhere without retaining a second team channel.

Tests must cover refusal through host actions and direct bus calls, replacement with a stable channel, stale and forged requests, nested-lead refusal, rollback after an injected SQL failure, and reassignment after either permitted way of ending leadership. The source branch starts at 9dcce671. No provider contact, fork source or installation belongs to this stage.

Lead's message 9542 also forbids deleting a current lead on every surface. Cleanup reports the same reason, direct removal rechecks it in the bus transaction, and bulk removal preserves independent row results. The chart reads one transactional relation snapshot and uses the stable team channel for its ID and title.
