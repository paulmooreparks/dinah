package main

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"studio/internal/diagrampicture"
	"studio/internal/drawingpage"
	"studio/internal/layout"
	"studio/internal/workspaces"
)

// issueDiagramLease is called only by managed provider launch configuration.
// Its server name must be an explicitly permitted profile registration.
// No App, applet or bus method exposes lease issuance.
func (a *App) issueDiagramLease(chatID, serverName string) (map[string]string, func(), error) {
	chat := a.session(chatID)
	if chat == nil || !chat.Open() {
		return nil, nil, errors.New("a managed chat is required")
	}
	return a.issueDiagramWorkspaceLease(serverName, chat.Cwd(), func() *Session { return chat })
}

// A pending channel may receive configuration before session/new has allocated
// an identity. Its reader refuses all paths until Bind establishes that identity.
func (a *App) issueDiagramWorkspaceLease(serverName, workspace string, currentChat func() *Session) (map[string]string, func(), error) {
	permitted, program := false, ""
	for _, server := range product.ToolServers {
		if server.Name == serverName && server.DiagramPictures {
			permitted = true
			program = server.Program
			break
		}
	}
	if !permitted {
		return nil, nil, errors.New("this managed tools registration cannot draw pictures")
	}
	a.mu.Lock()
	index := workspaces.IndexAt(a.settings.WorkspaceRecords, workspace)
	if index < 0 {
		a.mu.Unlock()
		return nil, nil, errors.New("the chat workspace is not registered")
	}
	record := a.settings.WorkspaceRecords[index]
	a.mu.Unlock()
	a.drawingMu.Lock()
	service, endpoint := a.drawingService, a.drawingEndpoint
	a.drawingMu.Unlock()
	if service == nil {
		return nil, nil, errors.New("the window drawing service is unavailable")
	}
	read := func(ctx context.Context, name string, limit int) ([]byte, error) {
		chat := currentChat()
		if chat == nil || !chat.Open() || !layout.SamePath(chat.Cwd(), record.ExecutionPath) {
			return nil, errors.New("the drawing chat is no longer available in this workspace")
		}
		a.mu.Lock()
		current := workspaces.Index(a.settings.WorkspaceRecords, record.ID)
		valid := current >= 0 && layout.SamePath(a.settings.WorkspaceRecords[current].ExecutionPath, record.ExecutionPath)
		scope := fileAccessScope(a.settings, record.ExecutionPath)
		a.mu.Unlock()
		if !valid {
			return nil, errors.New("the drawing workspace is no longer registered")
		}
		scope.Chat, scope.ChatTitle, scope.Source = chat.ID(), chat.State().Title, "diagram-picture"
		root, err := os.OpenRoot(record.ExecutionPath)
		if err != nil {
			return nil, err
		}
		defer root.Close()
		file, err := root.Open(filepath.FromSlash(name))
		if err != nil {
			return nil, err
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil || !info.Mode().IsRegular() {
			return nil, errors.New("the snapshot is not a regular file")
		}
		if info.Size() > int64(limit) {
			return nil, &diagrampicture.Refusal{HTTPStatus: 413, Code: "snapshot_limit", Message: "A snapshot exceeds its byte limit."}
		}
		authority, err := a.fileAccessService()
		if err != nil {
			return nil, err
		}
		return authority.ReadOpenedBytes(ctx, filepath.Join(record.ExecutionPath, filepath.FromSlash(name)), scope, file, limit)
	}
	lease, err := service.Issue(record.ID, program, read)
	if err != nil {
		return nil, nil, err
	}
	return map[string]string{"DIAGRAM_PICTURE_LEASE": lease, "DIAGRAM_PICTURE_ENDPOINT": endpoint, "DIAGRAM_PICTURE_WORKSPACE": record.ID, "DIAGRAM_PICTURE_ROOT": record.ExecutionPath}, func() { service.Revoke(lease) }, nil
}

func (a *App) drawDiagramSnapshot(assets fs.FS) diagrampicture.Renderer {
	return func(ctx context.Context, d diagrampicture.Drawing) (diagrampicture.Picture, error) {
		folder, err := os.MkdirTemp("", "diagram-picture-")
		if err != nil {
			return diagrampicture.Picture{}, err
		}
		// Browser state is private to this request and has no application bridge.
		// Cleanup is best effort after native Close releases the controller.
		cleanup := true
		defer func() {
			if cleanup {
				_ = os.RemoveAll(folder)
			}
		}()
		picture, err := drawingpage.Draw(ctx, assets, drawingpage.Request{Kind: d.Kind, Text: string(d.Source), Scale: d.Request.Scale, SourcePath: d.Request.Source.Path, Images: d.Images}, drawingpage.Options{DataFolder: filepath.Join(folder, "browser")})
		if err != nil {
			if errors.Is(err, drawingpage.ErrUnavailable) {
				return diagrampicture.Picture{}, &diagrampicture.Refusal{HTTPStatus: 503, Code: "renderer_unavailable", Message: err.Error()}
			}
			var pending *drawingpage.StartupError
			if errors.As(err, &pending) {
				cleanup = false
				settled := make(chan error, 1)
				go func() {
					failure := <-pending.Settled
					if failure == nil {
						_ = os.RemoveAll(folder)
					}
					settled <- failure
					close(settled)
				}()
				return diagrampicture.Picture{}, diagrampicture.RecoverAfterStartup(settled)
			}
			var disposal *drawingpage.DisposalError
			if errors.As(err, &disposal) {
				cleanup = false
				return diagrampicture.Picture{}, &diagrampicture.Refusal{HTTPStatus: 503, Code: "renderer_unavailable", Message: "Native drawing could not be disposed of; further drawing is disabled."}
			}
			return diagrampicture.Picture{}, err
		}
		return diagrampicture.Picture{Data: picture.PNG, Width: picture.Width, Height: picture.Height, Problems: []string{}}, nil
	}
}
