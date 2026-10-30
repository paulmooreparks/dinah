package sessionhost

import (
	"context"
	_ "embed"
	"strings"
	"sync"

	"studio/internal/acp"
)

//go:embed prompt.go
var PromptSource []byte

// Prompt is immutable input plus an optional private reply identity. Copies
// retain that identity, including when steering falls back to a later turn.
type Prompt struct {
	text, label       string
	blocks            []acp.ContentBlock
	delivered         []ConversationMessage
	pictures          []Attachment
	late              bool
	ledgerID, steerID int64
	reply             *reply
	// qid is the message's own id while it is queued, made when it first is, and
	// kept when it is carried to another chat (queue_store.go).
	qid string
}

type reply struct {
	mu    *sync.Mutex
	ctx   context.Context
	words strings.Builder
	done  chan struct{}
	once  sync.Once
	end   End
}

// End is a completed turn's result.
type End struct {
	StopReason string
	Err        error
}

// Stored is the prompt as the queue file keeps it. A prompt that is a waiting
// question has no stored form, since its asker's wait does not outlive the process.
func (p Prompt) Stored() (StoredPrompt, bool) {
	if p.reply != nil {
		return StoredPrompt{}, false
	}
	return StoredPrompt{ID: p.qid, Text: p.text, Label: p.label, Blocks: copyContent(p.blocks), Delivered: copyMessages(p.delivered),
		Pictures: copyAttachments(p.pictures), Late: p.late, LedgerID: p.ledgerID, SteerID: p.steerID}, true
}

// PromptFromStored is the prompt a queue file's entry was made from.
func PromptFromStored(s StoredPrompt) Prompt {
	return Prompt{text: s.Text, label: s.Label, blocks: copyContent(s.Blocks), delivered: copyMessages(s.Delivered), pictures: copyAttachments(s.Pictures),
		late: s.Late, ledgerID: s.LedgerID, steerID: s.SteerID, qid: s.ID}
}

func NewPrompt(text string, blocks []acp.ContentBlock) Prompt {
	return Prompt{text: text, blocks: copyContent(blocks)}
}
func (p Prompt) Text() string                     { return p.text }
func (p Prompt) Label() string                    { return p.label }
func (p Prompt) Blocks() []acp.ContentBlock       { return copyContent(p.blocks) }
func (p Prompt) Delivered() []ConversationMessage { return copyMessages(p.delivered) }
func (p Prompt) Pictures() []Attachment           { return copyAttachments(p.pictures) }
func (p Prompt) Asked() bool                      { return p.reply != nil }
func (p Prompt) Late() bool                       { return p.late }
func (p Prompt) LedgerID() int64                  { return p.ledgerID }
func (p Prompt) SteerID() int64                   { return p.steerID }
func (p Prompt) WithLabel(label string) Prompt    { p.label = label; return p }
func (p Prompt) WithDelivery(label string, delivered []ConversationMessage, pictures []Attachment) Prompt {
	p.label, p.delivered, p.pictures = label, copyMessages(delivered), copyAttachments(pictures)
	return p
}
func (p Prompt) AsLateSteer(id, ledgerID int64) Prompt {
	if p.label == "" {
		p.label = "You, steering"
	}
	p.late, p.steerID, p.ledgerID = true, id, ledgerID
	return p
}

// WithReply binds capture to the conversation's existing mutex before the
// prompt is published. The mutex is never copied or exposed by this package.
func (p Prompt) WithReply(ctx context.Context, mu *sync.Mutex) Prompt {
	p.reply = &reply{mu: mu, ctx: ctx, done: make(chan struct{})}
	return p
}
func (p Prompt) Cancelled() bool             { return p.reply != nil && p.reply.ctx.Err() != nil }
func (p Prompt) SameReply(other Prompt) bool { return p.reply == other.reply }

// Capture operations require the constructor-injected session mutex. These
// temporary Locked adapters disappear with the adjacent session state.
func (p Prompt) resetCaptureLocked() {
	if p.reply != nil {
		p.reply.words.Reset()
	}
}
func (p Prompt) appendCaptureLocked(text string) {
	if p.reply != nil {
		p.reply.words.WriteString(text)
	}
}

// Resolve is idempotent and must be called after releasing session and
// steering locks. It does not alter capture or cancel the prompt's context.
func (p Prompt) Resolve(end End) {
	if p.reply != nil {
		p.reply.once.Do(func() { p.reply.end = end; close(p.reply.done) })
	}
}
func (p Prompt) Wait(ctx context.Context) (string, End, bool) {
	select {
	case <-p.reply.done:
		p.reply.mu.Lock()
		words := p.reply.words.String()
		p.reply.mu.Unlock()
		return words, p.reply.end, true
	case <-ctx.Done():
		return "", End{StopReason: "error", Err: ctx.Err()}, false
	}
}

// capture returns an opaque identity only when this prompt captures a reply.
func (p Prompt) capture() *Prompt {
	if p.reply == nil {
		return nil
	}
	return &p
}
