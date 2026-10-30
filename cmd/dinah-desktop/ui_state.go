package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Several open flow drafts can each contain up to 1 MiB of source, alongside
// the window layout and each window's state. Bound the complete store without limiting
// it to the size of a single editor.
const maxUIStateBytes = 32 << 20

type UIState struct {
	Version int                        `json:"version"`
	Values  map[string]json.RawMessage `json:"values"`
}

func validateUIState(state UIState) error {
	if state.Version != 1 {
		return fmt.Errorf("unsupported UI state version %d; the saved layout has not been replaced", state.Version)
	}
	if state.Values == nil {
		return errors.New("UI state values must be an object")
	}
	for key, value := range state.Values {
		if key == "" || !json.Valid(value) {
			return errors.New("UI state contains an invalid key or value")
		}
	}
	return nil
}

func (a *App) ReadUIState() (UIState, error) {
	state := UIState{Version: 1, Values: map[string]json.RawMessage{}}
	data, err := readStateFile(filepath.Join(configDir(), "ui-state.json"))
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	return decodeUIState(data)
}

func decodeUIState(data []byte) (UIState, error) {
	state := UIState{}
	if len(data) > maxUIStateBytes {
		return state, errors.New("the saved UI state exceeds 32 MiB")
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return state, fmt.Errorf("read saved UI state: %w", err)
	}
	return state, validateUIState(state)
}

func (a *App) SaveUIState(state UIState) error {
	if err := validateUIState(state); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > maxUIStateBytes {
		return errors.New("UI state exceeds 32 MiB")
	}
	// Check the previous schema under the writer's lock, including when the
	// caller skipped the initial read. A failed read must never reset a layout.
	return writeValidatedStateFile(filepath.Join(configDir(), "ui-state.json"), data, func(previous []byte) error { _, err := decodeUIState(previous); return err })
}

func (a *App) SetNewWindowMode(mode string) error {
	if mode != "floating" && mode != "maximized" {
		return errors.New("new window mode must be floating or maximized")
	}
	if err := a.updateSettings(func(s *Settings) error {
		s.NewWindowMode = mode
		return nil
	}); err != nil {
		return err
	}
	a.emitSettingsChanged()
	return nil
}
