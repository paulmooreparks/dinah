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
	a.chatCleanupMu.Lock()
	defer a.chatCleanupMu.Unlock()
	return a.saveUIStateForCleanup(state)
}

// UIStateChanges is what the page changed in the saved UI state since it last
// saved: the values to set and the keys to remove.
type UIStateChanges struct {
	Version int                        `json:"version"`
	Set     map[string]json.RawMessage `json:"set"`
	Remove  []string                   `json:"remove"`
}

// SaveUIStateChanges applies the page's changes to the saved UI state. The
// page saves a moment after every change to a window, a scroll position or a
// draft, and the whole store, which is hundreds of kilobytes on a long-used
// installation, used to cross the bridge each time; the page now sends what
// changed, and the store is read, changed and written here, off the page's
// thread. A store that cannot be read is left as it is and the changes are
// refused, as a whole save is.
func (a *App) SaveUIStateChanges(changes UIStateChanges) error {
	if changes.Version != 1 {
		return fmt.Errorf("unsupported UI state version %d; the saved layout has not been changed", changes.Version)
	}
	for key, value := range changes.Set {
		if key == "" || !json.Valid(value) {
			return errors.New("UI state changes contain an invalid key or value")
		}
	}
	a.chatCleanupMu.Lock()
	defer a.chatCleanupMu.Unlock()
	state, err := a.ReadUIState()
	if err != nil {
		return err
	}
	for _, key := range changes.Remove {
		delete(state.Values, key)
	}
	for key, value := range changes.Set {
		state.Values[key] = value
	}
	return a.saveUIStateForCleanup(state)
}

// saveUIStateForCleanup requires chatCleanupMu, also held by final deletion.
// Copy the map before filtering because the page may retry its stale snapshot.
func (a *App) saveUIStateForCleanup(state UIState) error {
	if err := validateUIState(state); err != nil {
		return err
	}

	values := make(map[string]json.RawMessage, len(state.Values))
	for key, value := range state.Values {
		values[key] = value
	}
	state.Values = values
	a.mu.Lock()
	for id, removal := range a.settings.RemovedChats {
		if removal.Stage == "removed" || removal.Stage == "deleted" || removal.Stage == "finalizing" || removal.Stage == "erasing" {
			delete(state.Values, "dinah-draft:chat:"+id)
		}
	}
	a.mu.Unlock()
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
