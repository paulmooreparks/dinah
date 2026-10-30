package main

import (
	"errors"
	"strings"
	"unicode/utf8"

	"studio/internal/workspaces"
)

// Pinned items: chats and
// resources of a workspace's sources, pinned to the top of its pane in the
// person's order. They are kept on the workspace's record, so they follow
// the workspace and the command line can read them.

// WorkspacePin is a pin as the page gives and receives it.
type WorkspacePin = workspaces.Pin

// PinWorkspaceItem pins an item to a workspace, at the end of its pins. An
// item already pinned keeps its place, and its title is brought up to date.
func (a *App) PinWorkspaceItem(workspaceID string, pin WorkspacePin) (WorkspaceRecord, error) {
	return a.changeWorkspacePins(workspaceID, func(record WorkspaceRecord, pins []WorkspacePin) ([]WorkspacePin, error) {
		if err := checkPin(record, &pin); err != nil {
			return nil, err
		}
		key := workspaces.PinKey(pin)
		for i, p := range pins {
			if workspaces.PinKey(p) == key {
				pins[i].Title = pin.Title
				return pins, nil
			}
		}
		return append(pins, pin), nil
	})
}

// UnpinWorkspaceItem takes a pin off a workspace by its key.
func (a *App) UnpinWorkspaceItem(workspaceID, key string) (WorkspaceRecord, error) {
	return a.changeWorkspacePins(workspaceID, func(_ WorkspaceRecord, pins []WorkspacePin) ([]WorkspacePin, error) {
		out := []WorkspacePin{}
		for _, p := range pins {
			if workspaces.PinKey(p) != key {
				out = append(out, p)
			}
		}
		return out, nil
	})
}

// MoveWorkspacePin moves a pin to a place among the workspace's pins,
// counted from 0; a place past the end puts it last.
func (a *App) MoveWorkspacePin(workspaceID, key string, index int) (WorkspaceRecord, error) {
	return a.changeWorkspacePins(workspaceID, func(_ WorkspaceRecord, pins []WorkspacePin) ([]WorkspacePin, error) {
		from := -1
		for i, p := range pins {
			if workspaces.PinKey(p) == key {
				from = i
			}
		}
		if from < 0 {
			return nil, errors.New("that item is not pinned")
		}
		moved := pins[from]
		rest := append(append([]WorkspacePin{}, pins[:from]...), pins[from+1:]...)
		if index < 0 {
			index = 0
		}
		if index > len(rest) {
			index = len(rest)
		}
		return append(append(append([]WorkspacePin{}, rest[:index]...), moved), rest[index:]...), nil
	})
}

// checkPin checks that a pin names one item of the workspace and keeps only
// the fields of its kind.
func checkPin(record WorkspaceRecord, p *WorkspacePin) error {
	p.Title = strings.TrimSpace(p.Title)
	if p.Title == "" || !utf8.ValidString(p.Title) || utf8.RuneCountInString(p.Title) > 300 {
		return errors.New("a pinned item needs a title of up to 300 characters")
	}
	switch p.Kind {
	case "chat":
		if p.Session == "" || len(p.Session) > 200 {
			return errors.New("a pinned chat needs its session")
		}
		*p = WorkspacePin{Kind: "chat", Title: p.Title, Session: p.Session}
	case "resource":
		if p.Ref == nil || p.Ref.WorkspaceID != record.ID {
			return errors.New("a pinned item must belong to this workspace")
		}
		found := false
		for _, s := range record.Sources {
			if s.ID == p.Ref.SourceID {
				found = true
			}
		}
		if !found {
			return errors.New("the pinned item's source is not attached to this workspace")
		}
		ref := *p.Ref
		*p = WorkspacePin{Kind: "resource", Title: p.Title, Ref: &ref, ItemKind: p.ItemKind, Directory: p.Directory}
	default:
		return errors.New("only a chat or an item of a source can be pinned")
	}
	return nil
}

// changeWorkspacePins changes one workspace's pins and saves the registry.
func (a *App) changeWorkspacePins(workspaceID string, change func(WorkspaceRecord, []WorkspacePin) ([]WorkspacePin, error)) (WorkspaceRecord, error) {
	if a.startupErr != nil {
		return WorkspaceRecord{}, a.startupErr
	}
	var record WorkspaceRecord
	err := a.updateSettings(func(s *Settings) error {
		index := workspaces.Index(s.WorkspaceRecords, workspaceID)
		if index < 0 {
			return errors.New("that workspace is not registered")
		}
		record = s.WorkspaceRecords[index]
		pins, err := change(record, append([]WorkspacePin{}, record.Pins...))
		if err != nil {
			return err
		}
		s.WorkspaceRecords[index].Pins = pins
		record = workspaces.Copy(s.WorkspaceRecords[index : index+1])[0]
		return nil
	})
	if err != nil {
		return WorkspaceRecord{}, err
	}
	a.emitSettingsChanged()
	return record, nil
}
