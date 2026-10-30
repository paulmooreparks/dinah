package sessionhost

import (
	_ "embed"
	"errors"
	"strings"
)

//go:embed turn_format.go
var TurnFormatSource []byte

func DropPrompts(prompts []Prompt, why string) {
	for _, p := range prompts {
		p.Resolve(End{StopReason: "error", Err: errors.New(why)})
	}
}
func (t *TurnIO) promptText(p *Prompt) string { return PromptText(p, t.host.VisibleConversationText) }
func FreshSelection(s *Conversation, doc *DocRef) *DocRef {
	key := ""
	if doc != nil && doc.Selection != "" {
		key = doc.DraftID + "\x00" + doc.Path + "\x00" + doc.Selection
	}
	same := s.Selection(key)
	if !same {
		return doc
	}
	out := *doc
	out.Selection = ""
	return &out
}

func PromptText(p *Prompt, visibleConversationText func(string) string) string {
	var parts []string
	for _, b := range p.Blocks() {
		switch {
		case b.Type == "text":
			parts = append(parts, visibleConversationText(b.Text))
		case b.Type == "resource" && b.Resource != nil && b.Resource.Blob == "":
			parts = append(parts, "Attached "+b.Resource.URI+":\n"+b.Resource.Text)
		case b.Type == "resource" && b.Resource != nil:
			parts = append(parts, "Attached "+b.Resource.URI+" (not text)")
		default:
			parts = append(parts, "Attached content of type "+b.Type)
		}
	}
	if len(parts) == 0 {
		return p.Text()
	}
	return strings.Join(parts, "\n\n")
}

func SentKind(p *Prompt, fallback string) string {
	if len(p.Delivered()) > 0 || p.Label() == busChatLabel {
		return "bus"
	}
	return fallback
}

// SentEvent is the agent:sent event for a prompt the window sends. A
// delivery has its entries, which the page draws as cards.
func SentEvent(id, text, kind, label string, delivered []ConversationMessage) map[string]any {
	if len(delivered) == 0 {
		return map[string]any{"sessionId": id, "text": text, "kind": kind, "label": label}
	}
	return map[string]any{"sessionId": id, "text": text, "kind": kind, "label": label, "delivered": delivered}
}
