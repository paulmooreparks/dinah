package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"studio/internal/diagrampicture"
	"studio/internal/profile"
)

func TestDiagramLeaseUsesProfileWorkspaceAndFileAuthority(t *testing.T) {
	a, _ := providerTestApp(t)
	root := t.TempDir()
	record := newWorkspaceRecord(root)
	a.settings.WorkspaceRecords = []WorkspaceRecord{record}
	chat := newSessionState(&sessionSeed{ID: "codex:drawing", Cwd: root, Provider: "codex"}, sessionInitial{Open: true})
	a.runtime.PutConversationLocked(chat.ID(), chat)
	original := product.ToolServers
	defer func() { product.ToolServers = original }()
	product.ToolServers = []profile.ToolServer{{Name: "plain-tools", Program: "plain-tools"}, {Name: "drawing-tools", Program: "drawing-tools", DiagramPictures: true}}
	s, err := newShellServer(a, fstest.MapFS{}, "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer s.close()
	go s.serve()
	// This test exercises host authority with an invented renderer. Native
	// rendering and disposal have their separate tagged acceptance fixture.
	var pngBytes bytes.Buffer
	_ = png.Encode(&pngBytes, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	fixtureService := diagrampicture.NewService(s.addr, func(ctx context.Context, d diagrampicture.Drawing) (diagrampicture.Picture, error) {
		return diagrampicture.Picture{Data: pngBytes.Bytes(), Width: 2, Height: 2}, nil
	})
	s.pictures.Close()
	s.pictures = fixtureService
	a.drawingService = fixtureService
	if _, _, err := a.issueDiagramLease(chat.ID(), "plain-tools"); err == nil {
		t.Fatal("profile without permission received a lease")
	}
	env, revoke, err := a.issueDiagramLease(chat.ID(), "drawing-tools")
	if err != nil {
		t.Fatal(err)
	}
	defer revoke()
	text := []byte("graph TD; A-->B")
	name := filepath.Join(root, "diagram.mmd")
	if err = os.WriteFile(name, text, 0600); err != nil {
		t.Fatal(err)
	}
	request := diagrampicture.Request{RequestID: "bf1496d2-f919-4710-97eb-40d8a339fd20", WorkspaceID: record.ID, Source: diagrampicture.Snapshot{Path: "diagram.mmd", SHA256: diagrampicture.Hash(text)}, Format: "png", Scale: 1}
	client, err := diagrampicture.New(env["DIAGRAM_PICTURE_ENDPOINT"], env["DIAGRAM_PICTURE_LEASE"], record.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = client.Draw(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	policy, err := a.FileAccessPolicy()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.AddFileAccessEntry(name, policy.Version); err != nil {
		t.Fatal(err)
	}
	request.RequestID = "cf1496d2-f919-4710-97eb-40d8a339fd20"
	if p, err := client.Draw(context.Background(), request); err == nil || len(p.Data) != 0 {
		t.Fatal("drawing bypassed file-access policy")
	}
	// A page cookie never replaces the lease on this separate channel.
	r, _ := http.NewRequest("OPTIONS", "http://"+s.addr+diagrampicture.Route, nil)
	r.AddCookie(&http.Cookie{Name: shellCookie, Value: s.token})
	response, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 401 {
		t.Fatal("page cookie authorized drawing")
	}
	for _, origin := range []string{"https://outside.invalid", "http://" + s.addr} {
		r, _ = http.NewRequest("OPTIONS", "http://"+s.addr+diagrampicture.Route, nil)
		r.Header.Set("Authorization", "Bearer "+env["DIAGRAM_PICTURE_LEASE"])
		r.Header.Set("Origin", origin)
		response, err = http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 403 {
			t.Fatal("browser origin authorized drawing")
		}
	}
	// Unknown Host is refused before routing or body decoding.
	r, _ = http.NewRequest("OPTIONS", "http://"+s.addr+diagrampicture.Route, nil)
	r.Host = "outside.invalid"
	response, err = http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 421 {
		t.Fatal("unknown host accepted")
	}
	revoke()
	body, _ := json.Marshal(request)
	r, _ = http.NewRequest("POST", "http://"+s.addr+diagrampicture.Route, bytes.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+env["DIAGRAM_PICTURE_LEASE"])
	response, err = http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	b, _ := io.ReadAll(response.Body)
	if response.StatusCode != 401 || strings.Contains(string(b), env["DIAGRAM_PICTURE_LEASE"]) {
		t.Fatal("revoked lease accepted or disclosed")
	}
}
