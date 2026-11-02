package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"studio/internal/applet"
	"sync"
	"time"

	"studio/internal/fileaccess"
	"studio/internal/layout"
)

type fileAccessRuntime struct {
	resolveDefaults    func() ([]fileaccess.Rule, error)
	mu                 sync.Mutex
	service            *fileaccess.Service
	counts             map[string]int
	closed             bool
	version, highWater int64
	networkState       string
	stop               chan struct{}
}

func fileAccessScope(settings Settings, cwd string) fileaccess.Scope {
	return fileaccess.WorkspaceScope(stateLayout(), settings.WorkspaceRecords, cwd)
}

func (a *App) fileAccessService() (*fileaccess.Service, error) {
	f := &a.fileAccess
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil, errors.New("the file-access service is closed")
	}
	if f.service != nil {
		return f.service, nil
	}
	resolve := f.resolveDefaults
	if resolve == nil {
		resolve = resolvedFileAccessDefaults
	}
	defaults, err := resolve()
	if err != nil {
		return nil, err
	}
	store, err := fileaccess.Open(filepath.Join(configDir(), "file-access.db"))
	if err != nil {
		return nil, err
	}
	f.service = &fileaccess.Service{Store: store, Defaults: defaults}
	if err := f.service.WarmNetworkRoots(); err != nil {
		store.Close()
		f.service = nil
		return nil, err
	}
	f.version, f.highWater, err = store.Metadata()
	if err == nil {
		f.counts, err = store.RefusedCounts()
	}
	if err != nil {
		store.Close()
		f.service = nil
		return nil, err
	}
	return f.service, nil
}

func resolvedFileAccessDefaults() ([]fileaccess.Rule, error) { return fileaccess.CurrentDefaults() }

func (a *App) fileAccessRefused(chat string) int {
	f := &a.fileAccess
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.counts[chat]
}

func (a *App) fileAccessUpdated(event string, payload any) {
	f := &a.fileAccess
	f.mu.Lock()
	if f.service == nil || f.closed {
		f.mu.Unlock()
		return
	}
	version, high, err := f.service.Store.Metadata()
	var counts map[string]int
	if err == nil {
		counts, err = f.service.Store.RefusedCounts()
	}
	if err != nil {
		f.mu.Unlock()
		logf("core", "warning", "could not refresh file-access audit state")
		return
	}
	networkBytes, _ := json.Marshal(struct {
		Workers  []fileaccess.NetworkWorker
		Revision uint64
	}{f.service.NetworkWorkers(), f.service.NetworkRevision()})
	networkChanged := string(networkBytes) != f.networkState
	f.networkState = string(networkBytes)
	changed := version != f.version || high != f.highWater || !maps.Equal(counts, f.counts)
	if version != f.version {
		event, payload = "file-access:policy", version
	}
	f.version, f.highWater, f.counts = version, high, counts
	f.mu.Unlock()
	if networkChanged {
		a.emit("file-access:network", version)
	}
	if changed {
		a.emit(event, payload)
		a.chatStatesChanged()
	}
}

func (a *App) closeFileAccess() {
	f := &a.fileAccess
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return
	}
	f.closed = true
	if f.stop != nil {
		close(f.stop)
	}
	if f.service != nil {
		_ = f.service.Store.Close()
	}
}

func (a *App) currentFileAccessScope() fileaccess.Scope {
	a.mu.Lock()
	defer a.mu.Unlock()
	return fileAccessScope(a.settings, a.settings.Workspace)
}

func (a *App) FileAccessPolicy() (fileaccess.Policy, error) {
	service, err := a.fileAccessService()
	if err != nil {
		return fileaccess.Policy{}, err
	}
	return service.Policy(a.currentFileAccessScope())
}

func (a *App) CheckFileAccessPath(path, workspaceID string) (fileaccess.Check, error) {
	a.mu.Lock()
	var scope fileaccess.Scope
	found := false
	for _, record := range a.settings.WorkspaceRecords {
		if record.ID == workspaceID {
			scope = fileAccessScope(a.settings, record.ExecutionPath)
			found = true
			break
		}
	}
	a.mu.Unlock()
	if !found {
		return fileaccess.Check{}, errors.New("choose a registered workspace for the path preview")
	}
	service, err := a.fileAccessService()
	if err != nil {
		return fileaccess.Check{}, err
	}
	return service.Check(path, scope)
}

func (a *App) AddFileAccessEntry(path string, version int64) (fileaccess.Policy, error) {
	service, err := a.fileAccessService()
	if err != nil {
		return fileaccess.Policy{}, err
	}
	policy, err := service.Add(path, version, a.currentFileAccessScope())
	if err == nil {
		a.fileAccessUpdated("file-access:policy", policy.Version)
	}
	return policy, err
}
func (a *App) RemoveFileAccessEntry(id string, version int64) (fileaccess.Policy, error) {
	service, err := a.fileAccessService()
	if err != nil {
		return fileaccess.Policy{}, err
	}
	policy, err := service.Remove(id, version, a.currentFileAccessScope())
	if err == nil {
		a.fileAccessUpdated("file-access:policy", policy.Version)
	}
	return policy, err
}
func (a *App) SetFileAccessDefault(id string, enabled bool, version int64) (fileaccess.Policy, error) {
	service, err := a.fileAccessService()
	if err != nil {
		return fileaccess.Policy{}, err
	}
	policy, err := service.SetDefault(id, enabled, version, a.currentFileAccessScope())
	if err == nil {
		a.fileAccessUpdated("file-access:policy", policy.Version)
	}
	return policy, err
}
func (a *App) FileAccessLog(filter fileaccess.Filter) (fileaccess.Page, error) {
	service, err := a.fileAccessService()
	if err != nil {
		return fileaccess.Page{}, err
	}
	return service.Store.Log(filter)
}
func (a *App) AcknowledgeFileAccessLog(chat string, through int64) (int, error) {
	service, err := a.fileAccessService()
	if err != nil {
		return 0, err
	}
	count, err := service.Store.Acknowledge(chat, through)
	if err == nil {
		a.fileAccessUpdated("file-access:read", nil)
	}
	return count, err
}

// Agent-owned applet reads retain the broker's bound caller. Person-owned
// document previews continue to use their existing resource reader.
func (a *App) readAgentResourceText(ctx context.Context, path string, file *os.File) (string, error) {
	invocation, ok := applet.InvocationFromContext(ctx)
	if !ok || invocation.OwnerKind != "chat" {
		return resourceText(file)
	}
	chat := a.session(invocation.OwnerID)
	if chat == nil || !chat.Open() || !layout.SamePath(chat.Cwd(), invocation.Workspace) {
		return "", errors.New("the file-reading chat is no longer registered in this workspace")
	}
	a.mu.Lock()
	scope := fileAccessScope(a.settings, chat.Cwd())
	a.mu.Unlock()
	scope.Chat, scope.ChatTitle = chat.ID(), chat.State().Title
	service, err := a.fileAccessService()
	if err != nil {
		return "", err
	}
	text, err := service.ReadOpened(ctx, path, scope, file, resourceText)
	if !fileaccess.InOperation(ctx) {
		a.fileAccessUpdated("file-access:read", nil)
	}
	return text, err
}

func (a *App) auditPathlessRead(ctx context.Context, action, key string, result applet.Result) (applet.Result, error) {
	switch action {
	case "dinah.skills/list@1", "dinah.skills/read@1", chatHistoryAction:
	default:
		return applet.Result{}, errors.New("this action is not a documented pathless read")
	}
	invocation, ok := applet.InvocationFromContext(ctx)
	if !ok || invocation.OwnerID == "" {
		return applet.Result{}, errors.New("a bound catalogue caller is required")
	}
	service, err := a.fileAccessService()
	if err != nil {
		return applet.Result{}, err
	}
	a.mu.Lock()
	scope := fileAccessScope(a.settings, invocation.Workspace)
	a.mu.Unlock()
	title := ""
	if chat := a.session(invocation.OwnerID); chat != nil {
		title = chat.State().Title
	}
	_, err = service.Store.Append(fileaccess.Record{Chat: invocation.OwnerID, ChatTitle: title, Workspace: scope.Workspace, Path: "catalog:" + key, Bytes: len([]byte(result.OutputJSON)), Outcome: "allowed", Source: action, Reason: "Read through a catalogued action"})
	if err != nil {
		return applet.Result{}, errors.New("could not commit catalogue-read audit; no content returned")
	}
	a.fileAccessUpdated("file-access:read", nil)
	return result, nil
}

func (a *App) watchFileAccess(ctx context.Context) {
	f := &a.fileAccess
	f.mu.Lock()
	if f.closed || f.stop != nil {
		f.mu.Unlock()
		return
	}
	f.stop = make(chan struct{})
	stop := f.stop
	f.mu.Unlock()
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case <-ticker.C:
				a.fileAccessUpdated("file-access:read", nil)
			}
		}
	}()
}

func (a *App) agentResourceScope(ctx context.Context) (fileaccess.Scope, bool, error) {
	invocation, ok := applet.InvocationFromContext(ctx)
	if !ok || invocation.OwnerKind != "chat" {
		return fileaccess.Scope{}, false, nil
	}
	chat := a.session(invocation.OwnerID)
	if chat == nil || !chat.Open() || !layout.SamePath(chat.Cwd(), invocation.Workspace) {
		return fileaccess.Scope{}, true, errors.New("the file-reading chat is no longer registered in this workspace")
	}
	a.mu.Lock()
	scope := fileAccessScope(a.settings, chat.Cwd())
	a.mu.Unlock()
	scope.Chat, scope.ChatTitle = chat.ID(), chat.State().Title
	scope.Source = "dinah.workspace/resources@1"
	return scope, true, nil
}
func (a *App) agentResourceDirectory(ctx context.Context, path string, file *os.File) ([]fs.DirEntry, bool, error) {
	scope, handled, err := a.agentResourceScope(ctx)
	if !handled || err != nil {
		return nil, handled, err
	}
	service, err := a.fileAccessService()
	if err != nil {
		return nil, true, err
	}
	// The source handle has already established source confinement.
	if _, err := service.ReadMetadata(ctx, path, scope, file); err != nil {
		if !fileaccess.InOperation(ctx) {
			a.fileAccessUpdated("file-access:read", nil)
		}
		return nil, true, err
	}
	entries, err := service.ReadDirectory(ctx, path, scope)
	if !fileaccess.InOperation(ctx) {
		a.fileAccessUpdated("file-access:read", nil)
	}
	return entries, true, err
}
func (a *App) checkAgentResourceMetadata(ctx context.Context, path string, file *os.File, source ...string) error {
	scope, handled, err := a.agentResourceScope(ctx)
	if !handled || err != nil {
		return err
	}
	if len(source) > 0 {
		scope.Source = source[0]
	}
	service, err := a.fileAccessService()
	if err != nil {
		return err
	}
	_, err = service.ReadMetadata(ctx, path, scope, file)
	if !fileaccess.InOperation(ctx) {
		a.fileAccessUpdated("file-access:read", nil)
	}
	return err
}
