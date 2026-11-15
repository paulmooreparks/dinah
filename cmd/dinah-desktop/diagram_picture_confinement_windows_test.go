package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"

	"studio/internal/acp"
	"studio/internal/diagrampicture"
	"studio/internal/profile"
)

// This exercises the real host snapshot reader, including OpenRoot and the
// final-handle file policy. All files and reparse points belong to the fixture.
func TestDiagramLeaseLinkAndJunctionConfinement(t *testing.T) {
	a, _ := providerTestApp(t)
	root, outside := t.TempDir(), t.TempDir()
	record := newWorkspaceRecord(root)
	a.settings.WorkspaceRecords = []WorkspaceRecord{record}
	id := "codex:confined"
	a.runtime.PutConversationLocked(id, newSessionState(&sessionSeed{ID: id, Cwd: root, Provider: "codex"}, sessionInitial{Open: true}))
	previous := product.ToolServers
	defer func() { product.ToolServers = previous }()
	product.ToolServers = []profile.ToolServer{{Name: "fixture", Program: "fixture", DiagramPictures: true}}
	var pixels bytes.Buffer
	if err := png.Encode(&pixels, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	var draws atomic.Int32
	server := httptest.NewUnstartedServer(nil)
	a.drawingService = diagrampicture.NewService(server.Listener.Addr().String(), func(context.Context, diagrampicture.Drawing) (diagrampicture.Picture, error) {
		draws.Add(1)
		return diagrampicture.Picture{Data: pixels.Bytes(), Width: 2, Height: 2}, nil
	})
	server.Config.Handler = a.drawingService
	server.Start()
	defer server.Close()
	a.drawingEndpoint = server.URL
	defer a.drawingService.Close()
	text := []byte("graph TD; A-->B")
	for _, path := range []string{filepath.Join(root, "allowed.mmd"), filepath.Join(root, "denied.mmd"), filepath.Join(outside, "outside.mmd")} {
		if err := os.WriteFile(path, text, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(outside, "outside.mmd"), filepath.Join(root, "outside-link.mmd")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("denied.mmd", filepath.Join(root, "denied-link.mmd")); err != nil {
		t.Fatal(err)
	}
	policy, err := a.FileAccessPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.AddFileAccessEntry(filepath.Join(root, "denied.mmd"), policy.Version); err != nil {
		t.Fatal(err)
	}
	junction := filepath.Join(root, "outside-junction")
	cmd := exec.Command("pwsh", "-NoProfile", "-NonInteractive", "-Command", `$ErrorActionPreference='Stop'; New-Item -ItemType Junction -Path $env:DRAWING_TEST_LINK -Target $env:DRAWING_TEST_TARGET | Out-Null`)
	cmd.Env = append(os.Environ(), "DRAWING_TEST_LINK="+junction, "DRAWING_TEST_TARGET="+outside)
	acp.HideWindow(cmd)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create isolated junction: %v: %s", err, b)
	}
	defer os.Remove(junction)

	env, revoke, err := a.issueDiagramLease(id, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	defer revoke()
	client, err := diagrampicture.New(env["DIAGRAM_PICTURE_ENDPOINT"], env["DIAGRAM_PICTURE_LEASE"], record.ID)
	if err != nil {
		t.Fatal(err)
	}
	for i, name := range []string{"allowed.mmd", "outside-link.mmd", "outside-junction/outside.mmd", "denied-link.mmd"} {
		request := diagrampicture.Request{RequestID: fmt.Sprintf("bf1496d2-f919-4710-97eb-%012d", i), WorkspaceID: record.ID, Source: diagrampicture.Snapshot{Path: name, SHA256: diagrampicture.Hash(text)}, Format: "png", Scale: 1}
		picture, err := client.Draw(context.Background(), request)
		if i == 0 {
			if err != nil || len(picture.Data) == 0 {
				t.Fatalf("ordinary source: %v", err)
			}
			continue
		}
		if err == nil || len(picture.Data) != 0 || draws.Load() != 1 {
			t.Fatalf("snapshot escaped through %s", name)
		}
	}
}
