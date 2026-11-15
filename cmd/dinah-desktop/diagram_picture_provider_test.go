package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"studio/internal/acp"
	"studio/internal/diagrampicture"
	"studio/internal/profile"
	"studio/internal/programs"
)

type drawingTestConnection struct {
	*testProviderConnection
	secrets []string
}

func (c *drawingTestConnection) RegisterTraceSecret(secret string) {
	c.secrets = append(c.secrets, secret)
}

func TestDiagramManagedRegistration(t *testing.T) {
	for _, provider := range []string{"codex", "claude", "ollama", "devin"} {
		t.Run(provider, func(t *testing.T) {
			a, base := providerTestApp(t)
			root := t.TempDir()
			record := newWorkspaceRecord(root)
			a.settings.WorkspaceRecords = []WorkspaceRecord{record}
			original := product.ToolServers
			t.Cleanup(func() { product.ToolServers = original })
			args := []string{"fixture-mode", "one argument with spaces"}
			product.ToolServers = []profile.ToolServer{{Name: "fixture-tools", Program: "fixture-tools", Args: []string{"ordinary"}, DiagramPictures: true, DiagramPicturePrefix: "fixture-drawing", DiagramPictureArgs: args}}
			program := filepath.Join(configDir(), "bin", programs.FileName("fixture-tools"))
			if err := os.MkdirAll(filepath.Dir(program), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(program, []byte("stand-in; not executed by this host test"), 0600); err != nil {
				t.Fatal(err)
			}
			shell, err := newShellServer(a, fstest.MapFS{}, "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer shell.close()
			go shell.serve()
			conn := &drawingTestConnection{testProviderConnection: base}
			channel, _, err := a.newAppletChannel(conn, root)
			if err != nil {
				t.Fatal(err)
			}
			defer a.revokeAppletChannel(channel)
			custom := map[string]any{"name": "fixture-tools", "command": "person-owned", "args": []string{"keep-me"}, "env": []map[string]string{{"name": "KEEP", "value": "custom"}}}
			before, _ := json.Marshal(custom)
			servers, err := a.withDiagramServers(channel, provider, root, []any{custom})
			if err != nil {
				t.Fatal(err)
			}
			after, _ := json.Marshal(custom)
			if string(before) != string(after) {
				t.Fatal("custom registration changed")
			}
			if provider == "devin" {
				if len(servers) != 1 || !strings.Contains(channel.diagramNotice(), "supported chat") {
					t.Fatal("unsupported provider was silent or received a lease")
				}
				return
			}
			if len(servers) != 2 {
				t.Fatal("missing restricted registration")
			}
			var managed struct {
				Name, Command string
				Args          []string
				Env           []struct{ Name, Value string }
			}
			if err := acp.Decode(servers[1], &managed); err != nil {
				t.Fatal(err)
			}
			if !strings.HasPrefix(managed.Name, "fixture-drawing-") || managed.Command != program || !reflect.DeepEqual(managed.Args, args) {
				t.Fatal("profile launch shape changed")
			}
			env := map[string]string{}
			for _, value := range managed.Env {
				env[value.Name] = value.Value
			}
			if len(env) != 4 || env["DIAGRAM_PICTURE_ROOT"] != record.ExecutionPath || env["DIAGRAM_PICTURE_WORKSPACE"] != record.ID || env["DIAGRAM_PICTURE_ENDPOINT"] == "" || env["DIAGRAM_PICTURE_LEASE"] == "" {
				t.Fatal("incomplete managed environment")
			}
			if !containsSecret(conn.secrets, env["DIAGRAM_PICTURE_LEASE"]) || strings.Contains(channel.diagramNotice(), env["DIAGRAM_PICTURE_LEASE"]) {
				t.Fatal("lease not redacted before registration, or exposed in context")
			}
			client, err := diagrampicture.New(env["DIAGRAM_PICTURE_ENDPOINT"], env["DIAGRAM_PICTURE_LEASE"], record.ID)
			if err != nil {
				t.Fatal(err)
			}
			source := []byte("graph TD; A-->B")
			if err = os.WriteFile(filepath.Join(root, "flow.mmd"), source, 0600); err != nil {
				t.Fatal(err)
			}
			request := diagrampicture.Request{RequestID: "bf1496d2-f919-4710-97eb-40d8a339fd20", WorkspaceID: record.ID, Source: diagrampicture.Snapshot{Path: "flow.mmd", SHA256: diagrampicture.Hash(source)}, Format: "png", Scale: 1}
			// Configuration is delivered before the provider has allocated a chat.
			if p, err := client.Draw(context.Background(), request); err == nil || len(p.Data) != 0 {
				t.Fatal("unbound registration read a snapshot")
			}
			id := provider + ":drawing"
			a.runtime.PutConversationLocked(id, newSessionState(&sessionSeed{ID: id, Cwd: root, Provider: provider}, sessionInitial{Open: true}))
			if err = a.bindAppletChannel(channel, id); err != nil {
				t.Fatal(err)
			}
			if channel.drawing.chat(a) == nil {
				t.Fatal("channel did not bind its reader")
			}
			// Reopening gets another alias and revokes the earlier registration.
			next, _, err := a.newAppletChannel(conn, root)
			if err != nil {
				t.Fatal(err)
			}
			defer a.revokeAppletChannel(next)
			fresh, err := a.withDiagramServers(next, provider, root, nil)
			if err != nil {
				t.Fatal(err)
			}
			if fresh[0].(map[string]any)["name"] == managed.Name {
				t.Fatal("alias reused across registrations")
			}
			if err = a.bindAppletChannel(next, id); err != nil {
				t.Fatal(err)
			}
			req, _ := http.NewRequest("OPTIONS", env["DIAGRAM_PICTURE_ENDPOINT"]+diagrampicture.Route, nil)
			req.Header.Set("Authorization", "Bearer "+env["DIAGRAM_PICTURE_LEASE"])
			req.Header.Set("X-Helper-Channel", "diagram-picture")
			response, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != 401 {
				t.Fatal("replaced channel retained drawing authority")
			}
		})
	}
}

func containsSecret(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
