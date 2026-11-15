package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"studio/internal/programs"
)

// diagramChannel retains session launch authority in memory. It is never part
// of a transcript, settings file, agent context or tool argument.
type diagramChannel struct {
	mu      sync.Mutex
	session string
	closed  bool
	revokes []func()
	notice  string
}

func (d *diagramChannel) bind(session string) {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.closed {
		d.session = session
	}
}
func (d *diagramChannel) chat(a *App) *Session {
	d.mu.Lock()
	id, closed := d.session, d.closed
	d.mu.Unlock()
	if closed || id == "" {
		return nil
	}
	return a.session(id)
}
func (d *diagramChannel) revoke() {
	if d == nil {
		return
	}
	d.mu.Lock()
	d.closed = true
	revokes := d.revokes
	d.revokes = nil
	d.mu.Unlock()
	for _, revoke := range revokes {
		revoke()
	}
}

var diagramServerPrefix = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,39}$`)

const diagramUnsupported = "New Canvas and Mermaid pictures are unavailable for this chat's provider. Use a supported chat inside this workspace in the window, or publish through the window. Existing D2 rendering and verified-picture reuse remain available."

// The product supplies a restricted launch mode. A drawing permission without
// that mode never starts another copy of its ordinary full tools catalog.
func (a *App) withDiagramServers(channel *appletChannel, provider, workspace string, servers []any) ([]any, error) {
	if channel == nil {
		return nil, errors.New("the managed session channel is unavailable")
	}
	d := &diagramChannel{}
	channel.drawing = d
	for _, server := range product.ToolServers {
		if !server.DiagramPictures {
			continue
		}
		if provider != "codex" && provider != "claude" && provider != "ollama" {
			d.notice = diagramUnsupported
			continue
		}
		if !diagramServerPrefix.MatchString(server.DiagramPicturePrefix) || len(server.DiagramPictureArgs) == 0 {
			d.revoke()
			return nil, errors.New("the product's restricted drawing registration is incomplete")
		}
		command := filepath.Join(configDir(), "bin", programs.FileName(server.Program))
		if info, err := os.Stat(command); err != nil || !info.Mode().IsRegular() {
			d.revoke()
			return nil, errors.New("the installed tools program for drawing is unavailable")
		}
		// A fresh cryptographic suffix separates this registration from every
		// configured server without changing or copying that server's settings.
		alias := server.DiagramPicturePrefix + "-" + strings.ToLower(rand.Text()[:10])
		env, revoke, err := a.issueDiagramWorkspaceLease(server.Name, workspace, func() *Session { return d.chat(a) })
		if err != nil {
			d.revoke()
			return nil, err
		}
		d.revokes = append(d.revokes, revoke)
		// Register redaction before any protocol serialization or subprocess
		// configuration. Only the managed server receives this environment.
		if channel.conn != nil {
			channel.conn.RegisterTraceSecret(env["DIAGRAM_PICTURE_LEASE"])
		}
		values := make([]map[string]string, 0, len(env))
		for _, key := range []string{"DIAGRAM_PICTURE_LEASE", "DIAGRAM_PICTURE_ENDPOINT", "DIAGRAM_PICTURE_WORKSPACE", "DIAGRAM_PICTURE_ROOT"} {
			values = append(values, map[string]string{"name": key, "value": env[key]})
		}
		servers = append(servers, map[string]any{"name": alias, "command": command, "args": append([]string{}, server.DiagramPictureArgs...), "env": values})
		d.notice += fmt.Sprintf("The managed server %s publishes with diagram pictures and offers only those publication operations. Find its operations by their names and descriptions; the server suffix changes between sessions. Publication still requires its usual approval. If its drawing lease expires, reopen this chat to establish a fresh registration.\n", alias)
	}
	return servers, nil
}
