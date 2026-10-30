package main

import (
	"path/filepath"

	"studio/internal/drafts"
	"studio/internal/recent"
	"studio/internal/workspaces"
)

// Documents › Recent. The person's
// opens are recorded here and by the command line's document open; an
// agent's open is not.

// RecentDocument is a recent entry as the menu and the Show All dialog show
// it, with its workspace's name as it is now.
type RecentDocument struct {
	recent.Entry
	WorkspaceName string `json:"workspaceName"`
}

// RecentChange is told as "documents:recent" after the list changes, with
// Error set when recording an open failed.
type RecentChange struct {
	Error string `json:"error,omitempty"`
}

// RecentDocuments lists the recently opened documents that can still be
// opened, newest first.
func (a *App) RecentDocuments() ([]RecentDocument, error) {
	entries, err := recent.For(stateLayout()).List()
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	records := workspaces.Copy(a.settings.WorkspaceRecords)
	a.mu.Unlock()
	out := []RecentDocument{}
	for _, e := range recent.Available(entries, records) {
		id := e.WorkspaceID
		if e.Ref != nil {
			id = e.Ref.WorkspaceID
		}
		name := "Home"
		if i := workspaces.Index(records, id); i >= 0 {
			name = records[i].Name
		} else if id != "" {
			name = ""
		}
		out = append(out, RecentDocument{Entry: e, WorkspaceName: name})
	}
	return out, nil
}

func (a *App) recordRecent(e recent.Entry) {
	change := RecentChange{}
	if err := recent.For(stateLayout()).Record(e); err != nil {
		change.Error = "The document opened, but it could not be added to Recent: " + err.Error()
	}
	a.emit("documents:recent", change)
}

// recordOpened records a draft's resource as opened. A document that exists
// only as a draft has nowhere to be reopened from, and is not recorded.
func (a *App) recordOpened(v drafts.View) {
	if v.Origin.Kind == drafts.OriginNew || v.Origin.Kind == "" {
		return
	}
	a.recordRecent(recent.Entry{Ref: &workspaces.Ref{WorkspaceID: v.Origin.WorkspaceID, SourceID: v.Origin.SourceID, Path: v.Origin.Path}, Kind: v.Origin.Kind, Title: v.Title})
}

// recordComputerFile records a file outside every workspace as opened.
func (a *App) recordComputerFile(workspaceID, path string) {
	a.recordRecent(recent.Entry{File: path, WorkspaceID: workspaceID, Kind: recent.KindComputerFile, Title: filepath.Base(path)})
}
