package sessionhost

import (
	_ "embed"
	"encoding/json"
	"studio/internal/acp"
	"studio/internal/bus"
)

//go:embed steering_ledger.go
var SteeringLedgerSource []byte

type steeringPayload struct {
	Text        string             `json:"text"`
	Document    *DocRef            `json:"document,omitempty"`
	Attachments []Attachment       `json:"attachments"`
	Blocks      []acp.ContentBlock `json:"blocks"`
}

// SteeringLedgerHost supplies the existing bus transactions and session lookup.
// The ledger adds no synchronization or persistence guarantees of its own.
type SteeringLedgerHost struct {
	Conversation func(string) *Conversation
	Save         func(string, string) (int64, error)
	SetState     func(string, int64, string) error
	Recover      func(string) ([]bus.SteeringSubmission, error)
	LogError     func(error)
}

type SteeringLedger struct{ host SteeringLedgerHost }

func NewSteeringLedger(host SteeringLedgerHost) *SteeringLedger { return &SteeringLedger{host: host} }

func (l *SteeringLedger) Save(s *Conversation, text string, doc *DocRef, atts []Attachment, p Prompt) (int64, error) {
	payload, err := json.Marshal(steeringPayload{text, doc, atts, p.Blocks()})
	if err != nil {
		return 0, err
	}
	return l.host.Save(s.ID(), string(payload))
}

func (l *SteeringLedger) State(session string, id int64, state string) {
	if id == 0 {
		return
	}
	if err := l.host.SetState(session, id, state); err != nil {
		l.host.LogError(err)
	}
}

// Recover returns retained submissions only when the chat is idle.
func (l *SteeringLedger) Recover(id string) ([]bus.SteeringSubmission, error) {
	if s := l.host.Conversation(id); s != nil {
		state := s.State()
		busy := state.Working || len(state.Queued) > 0
		if busy {
			return []bus.SteeringSubmission{}, nil
		}
	}
	return l.host.Recover(id)
}

func (l *SteeringLedger) Dismiss(id string, submission int64) error {
	return l.host.SetState(id, submission, "dismissed")
}
