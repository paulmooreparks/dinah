package bus

import "embed"

//go:embed answer_origin.go attachments.go attention.go bus.go decision_broadcast.go decision_entries.go decisions.go file_access.go format.go group_context.go human_inbox.go links.go mentions.go message_presentation.go notice.go pending_session.go person.go questions.go recipient_recycle.go remove_recipient.go request_settlement.go results.go room_lookup.go rooms.go settlement_sources.go steering_submissions.go steers.go store.go team_lead_acceptance.go team_lead_checkpoint.go team_relations.go teams.go teams_budget.go teams_relay.go teams_replace.go titles.go wake_waiting.go web.go
var settlementSources embed.FS

// SettlementSources identifies the compiled implementation used by the bundled
// request-withdrawal adapter. Test-only fixtures are excluded from its digest.
func SettlementSources() (map[string][]byte, error) {
	entries, err := settlementSources.ReadDir(".")
	if err != nil {
		return nil, err
	}
	out := map[string][]byte{}
	for _, entry := range entries {
		data, err := settlementSources.ReadFile(entry.Name())
		if err != nil {
			return nil, err
		}
		out[entry.Name()] = data
	}
	return out, nil
}
