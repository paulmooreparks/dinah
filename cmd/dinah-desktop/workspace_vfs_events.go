package main

// WorkspaceMountChange is emitted after a workspace registry change is saved.
// Consumers re-read their retained path using its saved resource identity.
type WorkspaceMountChange struct {
	WorkspaceID string `json:"workspaceId"`
	SourceID    string `json:"sourceId,omitempty"`
	Kind        string `json:"kind"`
}

func (a *App) emitWorkspaceMountChanged(kind, workspaceID, sourceID string) {
	a.emit("workspace:changed", WorkspaceMountChange{WorkspaceID: workspaceID, SourceID: sourceID, Kind: kind})
}
