// Package requestsettlement defines request dispositions without granting actor authority.
package requestsettlement

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"unicode/utf8"
)

type Input struct {
	OperationID string `json:"operationId"`
	ID          string `json:"id"`
	Revision    int    `json:"revision"`
}

type Receipt struct {
	OperationID string `json:"operationId"`
	ID          string `json:"id"`
	Revision    int    `json:"revision"`
	State       string `json:"state"`
	Outcome     string `json:"outcome,omitempty"`
	SettledAt   string `json:"settledAt,omitempty"`
	Code        string `json:"code,omitempty"`
	Text        string `json:"text,omitempty"`
}

type Disposition struct {
	Status    string `json:"status"`
	SettledAt string `json:"settledAt,omitempty"`
	ActorKind string `json:"actorKind,omitempty"`
}

type Command struct {
	Name       string `json:"name"`
	Enabled    bool   `json:"enabled"`
	Reason     string `json:"reason,omitempty"`
	Invocation string `json:"invocation,omitempty"`
}

func ValidOperationID(id string) bool {
	return utf8.ValidString(id) && strings.TrimSpace(id) != "" && utf8.RuneCountInString(id) <= 128
}

func Key(parts ...string) string {
	b, _ := json.Marshal(parts)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func Refuse(in Input, code, text string) Receipt {
	return Receipt{OperationID: in.OperationID, ID: in.ID, Revision: in.Revision, State: "refused", Code: code, Text: text}
}

// Commands describes availability; the invocation name supplies no authority.
func Commands(id, status string, person, owner bool, answerNames []string) []Command {
	out := []Command{}
	for _, name := range append([]string{"Dismiss", "Withdraw"}, answerNames...) {
		enabled := status == "open" && ((name == "Withdraw" && owner) || (name != "Withdraw" && person))
		c := Command{Name: name, Enabled: enabled}
		if enabled {
			c.Invocation = id
		} else if status != "open" {
			c.Reason = "This request is no longer open."
		} else if name == "Withdraw" {
			c.Reason = "Only the available asking owner can withdraw this request."
		} else {
			c.Reason = "Only the person can use this command."
		}
		out = append(out, c)
	}
	return out
}
