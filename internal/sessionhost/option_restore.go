package sessionhost

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"studio/internal/acp"
	"studio/internal/agents"
)

//go:embed option_restore.go
var OptionRestoreSource []byte

func (o *Options) Restore(conn Connection, session string, opts []acp.ConfigOption, chosen map[string]json.RawMessage, reopen *Reopening) ([]acp.ConfigOption, []AppliedSubstitution, error) {
	var applied []AppliedSubstitution
	ids := make([]string, 0, len(chosen))
	modelIDs := map[string]bool{}
	for _, option := range opts {
		if option.Category == "model" || option.ID == "model" {
			modelIDs[option.ID] = true
		}
	}
	for id := range chosen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if modelIDs[ids[i]] != modelIDs[ids[j]] {
			return modelIDs[ids[i]]
		}
		return ids[i] < ids[j]
	})
	for _, id := range ids {
		var offered *acp.ConfigOption
		for i := range opts {
			if opts[i].ID == id {
				offered = &opts[i]
				break
			}
		}
		value := chosen[id]
		// Only the model must be the chat's own: it is what makes the
		// conversation the same one. A setting that depends on the model,
		// such as fast mode or effort, may be missing for the model the chat
		// reopened with, so it is left at the agent's value and said in the
		// chat; the chat's saved choice is kept, and used again when the agent
		// offers it. Paul asked on 28 October 2026 for Fable chats to reopen
		// reliably, after a chat refused over fast mode once its model was
		// substituted.
		if !modelIDs[id] && (offered == nil || !agents.Allowed(*offered, value)) {
			applied = append(applied, unofferedSetting(id, offered, value))
			continue
		}
		if offered != nil && !agents.Allowed(*offered, value) && reopen != nil {
			sub, ok := reopen.substitute(*offered, value)
			if !ok {
				return nil, nil, reopen.unoffered(*offered, value)
			}
			applied = append(applied, sub)
			value = sub.To
		}
		if offered == nil || !agents.Allowed(*offered, value) {
			return nil, nil, fmt.Errorf("this chat's saved %s setting is no longer available from its provider", id)
		}
		if bytes.Equal(bytes.TrimSpace(value), bytes.TrimSpace(offered.CurrentValue)) {
			continue
		}
		next, err := o.SetOption(conn, session, *offered, value)
		if err != nil {
			return nil, nil, fmt.Errorf("restore this chat's %s setting: %w", id, err)
		}
		confirmed := false
		for _, option := range next {
			if option.ID == id && bytes.Equal(bytes.TrimSpace(value), bytes.TrimSpace(option.CurrentValue)) {
				confirmed = true
			}
		}
		if !confirmed {
			return nil, nil, fmt.Errorf("the provider did not restore this chat's %s setting", id)
		}
		opts = next
	}
	return opts, applied, nil
}
