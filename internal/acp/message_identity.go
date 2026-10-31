package acp

// MessageIdentity is additive host metadata on text updates. Providers without
// both original identities retain the legacy chunk interface.
type MessageIdentity struct {
	TurnID      string `json:"turnId,omitempty"`
	ItemID      string `json:"itemId,omitempty"`
	ContentMode string `json:"contentMode,omitempty"`
}

func (m MessageIdentity) Identified() bool {
	return m.TurnID != "" && m.ItemID != "" && (m.ContentMode == "append" || m.ContentMode == "snapshot")
}
