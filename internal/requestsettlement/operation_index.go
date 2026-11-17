package requestsettlement

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"studio/internal/records"
)

var ErrOperationChanged = errors.New("the settlement operation ID already describes different input")

type operationIndex struct {
	ID      string `json:"id"`
	Actor   string `json:"actor"`
	Command string `json:"command"`
	Input   Input  `json:"input"`
}

func ChatActor(id string) string    { return "chat:" + id }
func ServiceActor(id string) string { return "service:" + id }

// Claim reserves one actor/command operation identity across source stores.
// The index is only an immutable locator. A source receipt, never this record,
// establishes whether settlement committed. A failed source write may leave a
// locator behind, and lookup must still report its outcome as unknown.
func Claim(state, actor, command string, in Input) (func() error, error) {
	s := records.Store{Dir: filepath.Join(state, "request-settlement-operations")}
	id := Key(actor, command, in.OperationID)
	release, err := s.Hold(id)
	if err != nil {
		return nil, err
	}
	failed := true
	defer func() {
		if failed {
			_ = release()
		}
	}()
	data, err := s.Read(id)
	if err == nil {
		var old operationIndex
		if json.Unmarshal(data, &old) != nil || old.ID != id || old.Actor != actor || old.Command != command {
			return nil, errors.New("the settlement operation locator is invalid")
		}
		if old.Input != in {
			return nil, ErrOperationChanged
		}
	} else if errors.Is(err, os.ErrNotExist) {
		data, err = json.Marshal(operationIndex{ID: id, Actor: actor, Command: command, Input: in})
		if err == nil {
			err = s.Write(id, data)
		}
		if err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	failed = false
	return release, nil
}
