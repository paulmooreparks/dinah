//go:build chatcleanup_acceptance

package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"studio/internal/acp"
	"studio/internal/diagrampicture"
	"studio/internal/profile"
	"studio/internal/programs"
)

// The fixture substitutes only rendering and the profile's invented program.
// Issuance, registration, binding, revocation and HTTP authority remain real.
func (a *App) chatCleanupAcceptanceDrawingLease(t *testing.T, id, workspace string, renderer diagrampicture.Renderer) map[string]string {
	t.Helper()
	previous := product.ToolServers
	product.ToolServers = []profile.ToolServer{{Name: "cleanup-fixture", Program: "cleanup-fixture", DiagramPictures: true, DiagramPicturePrefix: "cleanup-fixture", DiagramPictureArgs: []string{"invented-mode"}}}
	t.Cleanup(func() { product.ToolServers = previous })
	program := filepath.Join(configDir(), "bin", programs.FileName("cleanup-fixture"))
	if err := os.MkdirAll(filepath.Dir(program), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(program, []byte("Invented registration; never executed"), 0600); err != nil {
		t.Fatal(err)
	}
	shell, err := newShellServer(a, fstest.MapFS{}, "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shell.close)
	shell.pictures.Close()
	shell.pictures = diagrampicture.NewService(shell.addr, renderer)
	a.drawingMu.Lock()
	a.drawingService = shell.pictures
	a.drawingMu.Unlock()
	go shell.serve()
	a.mu.Lock()
	conn := a.providerConnLocked(a.settings.SessionProviders[id])
	a.mu.Unlock()
	if conn == nil {
		t.Fatal("install a stand-in provider before issuing its managed lease")
	}
	channel, _, err := a.newAppletChannel(conn, workspace)
	if err != nil {
		t.Fatal(err)
	}
	servers, err := a.withDiagramServers(channel, a.settings.SessionProviders[id], workspace, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 {
		t.Fatal("missing restricted managed registration")
	}
	if err := a.bindAppletChannel(channel, id); err != nil {
		t.Fatal(err)
	}
	var managed struct {
		Env []struct{ Name, Value string }
	}
	if err := acp.Decode(servers[0], &managed); err != nil {
		t.Fatal(err)
	}
	env := map[string]string{}
	for _, entry := range managed.Env {
		env[entry.Name] = entry.Value
	}
	return env
}

func TestCleanupAcceptanceBinningRevokesLiveDrawingAndCancelsWork(t *testing.T) {
	a, c := providerTestApp(t)
	p := a.runtime.ProviderLocked("codex")
	go a.lifecycle().Watch("codex", p)
	t.Cleanup(func() { c.Close(); p.Wait() })
	root := t.TempDir()
	record := newWorkspaceRecord(root)
	a.settings.WorkspaceRecords = []WorkspaceRecord{record}
	id := "codex:cleanup-drawing"
	cleanupFixtureChat(t, a, id, "codex", root, "window-cleanup-drawing")
	a.runtime.PutConversationLocked(id, newSessionState(&sessionSeed{ID: id, Cwd: root, Provider: "codex"}, sessionInitial{Open: true}))
	entered, cancelled := make(chan struct{}), make(chan struct{})
	env := a.chatCleanupAcceptanceDrawingLease(t, id, root, func(ctx context.Context, _ diagrampicture.Drawing) (diagrampicture.Picture, error) {
		close(entered)
		<-ctx.Done()
		close(cancelled)
		return diagrampicture.Picture{}, ctx.Err()
	})
	text := []byte("graph TD; A-->B")
	if err := os.WriteFile(filepath.Join(root, "flow.mmd"), text, 0600); err != nil {
		t.Fatal(err)
	}
	client, err := diagrampicture.New(env["DIAGRAM_PICTURE_ENDPOINT"], env["DIAGRAM_PICTURE_LEASE"], record.ID)
	if err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), testWait)
	defer cancel()
	go func() {
		picture, err := client.Draw(ctx, diagrampicture.Request{RequestID: "bf1496d2-f919-4710-97eb-40d8a339fd20", WorkspaceID: record.ID, Source: diagrampicture.Snapshot{Path: "flow.mmd", SHA256: diagrampicture.Hash(text)}, Format: "png", Scale: 1})
		if len(picture.Data) != 0 {
			err = errors.New("cancelled work returned picture bytes")
		}
		finished <- err
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("real lease never reached renderer")
	}
	if err := a.RemoveChat(id); err != nil {
		t.Fatal(err)
	}
	select {
	case <-cancelled:
	default:
		t.Fatal("binning returned before cancellation")
	}
	if err := <-finished; err == nil {
		t.Fatal("cancelled drawing succeeded")
	}
	checkRefused := func() {
		t.Helper()
		request, _ := http.NewRequestWithContext(ctx, "OPTIONS", env["DIAGRAM_PICTURE_ENDPOINT"]+diagrampicture.Route, nil)
		request.Header.Set("Authorization", "Bearer "+env["DIAGRAM_PICTURE_LEASE"])
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 401 {
			t.Fatalf("old lease returned %d", response.StatusCode)
		}
	}
	checkRefused()
	if result := a.RestoreRemovedChat(id); !result.OK {
		t.Fatal(result)
	}
	checkRefused()
}
