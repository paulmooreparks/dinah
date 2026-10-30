package sessionhost

import (
	_ "embed"
	"encoding/json"
	"slices"
	"strings"

	"studio/internal/acp"
	"studio/internal/providers"
)

//go:embed handler_updates.go
var HandlerUpdatesSource []byte

// Command retains the public skill/command representation.
type Command struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Hint        string `json:"hint,omitempty"`
	Source      string `json:"source"`
}

func (h *Handler) toolCall(s *Conversation) {
	// A chat that asked takes the agent's reply as what it says after its
	// last tool call; what it said before is its narration of the work,
	// such as "I need to read the brief", and never part of the reply.
	if s != nil {
		s.ResetCapture()
	}
}

func (h *Handler) agentMessage(s *Conversation, update json.RawMessage) {
	// A chat waiting on this turn keeps the agent's words; the page gets
	// them as always, below.
	if s != nil {
		var c struct {
			Content struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		}
		if json.Unmarshal(update, &c) == nil && c.Content.Type == "text" {
			s.AppendCapture(c.Content.Text)
		}
	}
}

func (h *Handler) configOptions(s *Conversation, options []acp.ConfigOption) {
	a := h.host
	if s != nil {
		a.SetOptions(s, options)
	}
}

func (h *Handler) availableCommands(sessionID string, update json.RawMessage) {
	a := h.host
	// The commands Devin offers in this session, its own and the skills
	// it found (https://agentclientprotocol.com/protocol/slash-commands).
	var c struct {
		AvailableCommands []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Input       *struct {
				Hint string `json:"hint"`
			} `json:"input"`
		} `json:"availableCommands"`
	}
	_ = json.Unmarshal(update, &c)
	cmds := []Command{}
	for _, x := range c.AvailableCommands {
		sk := Command{Name: x.Name, Description: x.Description, Source: providers.OwnCommandsProvider()}
		if strings.Contains(x.Name, ":") {
			// Devin imports skills from other tools under the tool's prefix
			// (https://docs.devin.ai/cli/reference/configuration/read-config-from).
			sk.Source = "imported"
		}
		if x.Input != nil {
			sk.Hint = x.Input.Hint
		}
		cmds = append(cmds, sk)
	}
	a.SetCommands(slices.Clone(cmds))
	a.Emit("agent:commands", map[string]any{"sessionId": sessionID, "commands": cmds})
}

func (h *Handler) sessionInfo(s *Conversation, titleValue, updatedAt *string) {
	a := h.host
	if s != nil {
		var title string
		if titleValue != nil {
			title = a.ResolveTitle(s.ID(), *titleValue)
		}

		var shown *string
		if titleValue != nil {
			shown = &title
		}
		s.UpdateDisplay(shown, updatedAt)

		EmitSession(a.Emit, s)
		if titleValue != nil {
			a.PublishBusTitle(s)
		}
	}
}

// withWindowKnown makes a usage update say whether its window is known, so that
// the page is never left to guess from a missing field. A size in an ACP agent's
// usage_update is the window the agent reports, so it is known; an adapter that
// is not sure, as the Ollama adapter is for a model on Ollama's own setting,
// says false itself, and that is kept.
func withWindowKnown(update json.RawMessage) json.RawMessage {
	var fields map[string]json.RawMessage
	if json.Unmarshal(update, &fields) != nil {
		return update
	}
	if _, said := fields["windowKnown"]; said {
		return update
	}
	if _, sized := fields["size"]; !sized {
		return update
	}
	fields["windowKnown"] = json.RawMessage("true")
	out, err := json.Marshal(fields)
	if err != nil {
		return update
	}
	return out
}
