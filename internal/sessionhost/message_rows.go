package sessionhost

import "studio/internal/acp"

type messageKey struct{ turn, item string }
type messageRow struct {
	index int
	raw   string
	final bool
}

func (h *History) retireMessageAppends() {
	for _, rows := range h.messages {
		for _, row := range rows {
			row.raw, row.final = "", true
		}
	}
}

// RecordIdentifiedMessageLocked addresses a row independently of intervening
// notices. Only live unfinished rows retain raw text for split-token redaction.
func (h *History) RecordIdentifiedMessageLocked(replay bool, id string, identity acp.MessageIdentity, text string, visible func(string) string) {
	tr := h.saved
	if replay && h.replay != nil {
		tr = h.replay
	}
	if tr == nil || tr.Deleted || !identity.Identified() {
		return
	}
	if h.messages == nil {
		h.messages = map[*Transcript]map[messageKey]*messageRow{}
	}
	rows := h.messages[tr]
	if rows == nil {
		rows = map[messageKey]*messageRow{}
		for i, m := range tr.Messages {
			if m.Role == "agent" && m.TurnID != "" && m.ItemID != "" {
				// A restored historical row is retired, including an unfinished one.
				// A complete snapshot may reconcile it; late deltas cannot revive it.
				rows[messageKey{m.TurnID, m.ItemID}] = &messageRow{index: i, final: true}
			}
		}
		h.messages[tr] = rows
	}
	key := messageKey{identity.TurnID, identity.ItemID}
	row := rows[key]
	if row == nil {
		row = &messageRow{index: len(tr.Messages)}
		rows[key] = row
		tr.Messages = append(tr.Messages, ConversationMessage{Role: "agent", Provider: tr.Provider, SessionID: id, TurnID: identity.TurnID, ItemID: identity.ItemID})
	}
	if identity.ContentMode == "append" {
		if row.final {
			return
		}
		row.raw += text
		tr.Messages[row.index].Text = visible(row.raw)
	} else {
		row.raw, row.final = "", true
		tr.Messages[row.index].Text = visible(text)
		tr.Messages[row.index].MessageComplete = true
	}
}
