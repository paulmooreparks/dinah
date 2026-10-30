package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	extension "github.com/paulmooreparks/dinah-extension-go"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"studio/internal/extensions"
	"studio/internal/layout"
	"studio/internal/workspaces"
)

func (a *App) extensionWorkspace(id string) (extension.Workspace, error) {
	a.mu.Lock()
	records := workspaces.Copy(a.settings.WorkspaceRecords)
	current := a.settings.WorkspaceID
	a.mu.Unlock()
	if id == "" {
		id = current
	}
	for _, record := range records {
		if record.ID != id && !layout.SamePath(workspaces.ExecutionPath(record), id) {
			continue
		}
		ws := extension.Workspace{ID: record.ID, Name: record.Name, Path: workspaces.ExecutionPath(record), Sources: []extension.Source{}}
		for _, source := range record.Sources {
			if source.Kind == "local-folder" {
				ws.Sources = append(ws.Sources, extension.Source{ID: source.ID, Name: source.Name, Path: source.Path})
			}
		}
		return ws, nil
	}
	return extension.Workspace{}, errors.New("open a registered workspace before using extensions")
}
func (a *App) extensionService() *extensions.Service {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.extensions == nil {
		a.extensions = extensions.NewService(filepath.Join(configDir(), "extensions"), extensions.Hooks{
			Workspace: a.extensionWorkspace,
			Notify: func(ctx context.Context, title, body string) error {
				_, err := a.SaveNotification(Notification{ID: "extension-" + rand.Text(), Kind: "app", Severity: "info", Title: title, Body: body, Created: time.Now().UTC()})
				return err
			},
			Changed: a.resetExtensionCatalog,
		}, a.emit)
	}
	return a.extensions
}
func (a *App) extensionManager() *extensions.Manager { return a.extensionService().Manager }
func (a *App) SetExtensionCommandReview(id string, enabled bool) error {
	return a.extensionService().SetCommandReview(id, enabled)
}
func (a *App) AnswerExtensionCommand(key, decision, arguments string) error {
	return a.extensionService().AnswerCommand(key, decision, arguments)
}

// PendingExtensionReviews is the command reviews waiting for the person, each
// as the extension:review event carried it. A page that missed the event
// reads them here.
func (a *App) PendingExtensionReviews() []extensions.Review {
	return a.extensionService().PendingReviews()
}

func (a *App) resetExtensionCatalog() {
	a.appletMu.Lock()
	host := a.appletHost
	a.appletMu.Unlock()
	if host != nil {
		host.Invalidate()
	}
	a.syncExtensionClaims()
	a.emit("extensions:changed", map[string]bool{"changed": true})
}

// ExtensionInfo is an installed extension as Settings › Extensions shows
// it. FileTypes lists the file types its views declare and which of their
// file extensions open somewhere else (extension_claims.go).
type ExtensionInfo struct {
	extension.Installation
	Error     string              `json:"error,omitempty"`
	FileTypes []ExtensionFileType `json:"fileTypes,omitempty"`
}

func (a *App) Extensions() ([]ExtensionInfo, error) {
	manager := a.extensionManager()
	list, err := manager.Store.List()
	if err != nil {
		return nil, err
	}
	a.loadExtensionClaims()
	out := []ExtensionInfo{}
	for _, v := range list {
		info := ExtensionInfo{Installation: v}
		if v.Enabled {
			_, e := manager.Installed(v.Manifest.ID)
			if e != nil {
				info.Error = e.Error()
			}
		}
		if info.Error == "" {
			info.FileTypes = extensionFileTypes(v)
		} else {
			info.FileTypes = extensionFileTypes(extension.Installation{Manifest: v.Manifest})
		}
		out = append(out, info)
	}
	return out, nil
}
func (a *App) ChooseExtensionSource(kind string) (string, error) {
	if kind == "folder" {
		return wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{Title: "Install a Signed Extension Folder"})
	}
	return wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{Title: "Install a Signed Extension ZIP", Filters: []wruntime.FileFilter{{DisplayName: "Signed extension (*.zip)", Pattern: "*.zip"}}})
}
func (a *App) PreviewExtension(source string) (extension.Preview, error) {
	return a.extensionService().Preview(source)
}
func (a *App) InstallExtension(source string, approval extension.Approval) (extension.Installation, error) {
	return a.extensionService().Install(source, approval)
}
func (a *App) EnableExtension(id string, enabled bool) error {
	return a.extensionService().Enable(id, enabled)
}
func (a *App) RemoveExtension(id string) error { return a.extensionService().Remove(id) }

func (a *App) ExtensionWorkspace(id string) (extension.Workspace, error) {
	return a.extensionWorkspace(id)
}
func (a *App) ExtensionCall(id, action, workspace, input, requestID string) (json.RawMessage, error) {
	ws, err := a.extensionWorkspace(workspace)
	if err != nil {
		return nil, err
	}
	host, err := a.appletHostService().Broker()
	if err != nil {
		return nil, err
	}
	ref := id + "/" + action + "@1"
	digest, err := host.Registry.Digest(ref)
	if err != nil {
		return nil, err
	}
	if requestID == "" {
		return nil, errors.New("an invocation ID is required")
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	result, err := host.Run(ctx, ws.Path, "applet", "extension:"+id, requestID, ref, input, digest, time.Now().Add(2*time.Minute))
	return json.RawMessage(result), err
}
func (a *App) CancelExtensionCall(id, workspace, requestID string) error {
	ws, err := a.extensionWorkspace(workspace)
	if err != nil {
		return err
	}
	host, err := a.appletHostService().Broker()
	if err != nil {
		return err
	}
	return host.Cancel(ws.Path, "applet", "extension:"+id, requestID)
}
func (a *App) extensionAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		http.Error(w, "read only", 405)
		return
	}
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/extension-assets/"), "/", 3)
	if len(parts) != 3 || !extension.SafePath(parts[2]) {
		http.NotFound(w, r)
		return
	}
	asset, err := a.extensionService().ReadAsset(parts[0], parts[1], parts[2])
	if err != nil {
		switch {
		case errors.Is(err, extensions.ErrAssetNotFound):
			http.NotFound(w, r)
		case errors.Is(err, extensions.ErrAssetRegistry):
			http.Error(w, err.Error(), 503)
		default:
			http.Error(w, err.Error(), 409)
		}
		return
	}
	origin := "http://" + r.Host
	policy := "sandbox allow-scripts allow-forms; default-src 'none'; script-src " + origin + "/extension-assets/ " + origin + "/extension-sdk/ " + origin + "/pudl/; style-src " + origin + "/extension-assets/ " + origin + "/pudl/ 'unsafe-inline'; img-src " + origin + "/extension-assets/ data: blob:; font-src " + origin + "/pudl/ data:; connect-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'self'"
	if asset.Network {
		policy = strings.Replace(policy, "connect-src 'none'", "connect-src https:", 1)
	}
	w.Header().Set("Content-Security-Policy", policy)
	w.Header().Set("Content-Type", asset.MediaType)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == "GET" {
		w.Write(asset.Data)
	}
}

func (a *App) StartPreparedExtensionTerminal(ticket string, cols, rows int) (TerminalStarted, error) {
	input, err := a.extensionService().TakeTerminal(ticket)
	if err != nil {
		return TerminalStarted{}, err
	}
	return a.terminalService().StartCommand(input.Program, input.Args, input.Cwd, cols, rows)
}
