// Package antigravityagent runs Google's Antigravity command line as an
// agent program. `agy acp` is its documented Agent Client Protocol server on
// standard input and output, which reuses the person's cached sign-in
// (https://antigravity.google/docs/cli/acp/). Google made the Antigravity
// CLI its command line for Gemini on 19 May 2026 and stopped serving its
// consumer plans through the Gemini CLI on 18 June 2026
// (https://developers.googleblog.com/an-important-update-transitioning-gemini-cli-to-antigravity-cli/),
// which is why Dinah hosts this one for Gemini (Paul, 20 November 2026).
package antigravityagent

import (
	"context"
	"encoding/json"
	"io"
	"regexp"

	"studio/internal/acp"
	"studio/internal/acpprefix"
	"studio/internal/layout"
)

// Prefix begins the ID of every chat Antigravity makes, on Dinah's side.
const Prefix = "antigravity:"

// Program is the command line's name, and ServerArgs what starts its
// protocol server.
const Program = "agy"

var ServerArgs = []string{"acp"}

// Conn is a running Antigravity command line.
type Conn struct {
	*acpprefix.Conn
}

// Start runs the command line's protocol server in dir.
func Start(program, dir string, h acp.Handler, trace io.Writer) (*Conn, error) {
	c, err := acp.Start(program, ServerArgs, dir, acpprefix.Handler{H: h, Prefix: Prefix}, acp.Options{Trace: trace})
	if err != nil {
		return nil, err
	}
	return &Conn{Conn: &acpprefix.Conn{Conn: c, Prefix: Prefix}}, nil
}

// Call is the connection's, but for a list of sessions. Release 1.5.13
// answers session/list with the sessions of every folder, whatever folder
// was asked for, and writes a Windows folder as /C:/Users/... The host asks
// for one workspace's sessions and refuses a list with any other folder's in
// it, which showed as an error in Notifications at every start (Paul,
// 20 November 2026). So a list keeps the rows of the folder that was asked
// for, with their folder written as the host wrote it.
func (c *Conn) Call(ctx context.Context, method string, params, out any) error {
	if method != "session/list" || out == nil {
		return c.Conn.Call(ctx, method, params, out)
	}
	var asked struct {
		Cwd string `json:"cwd"`
	}
	if raw, err := json.Marshal(params); err == nil {
		_ = json.Unmarshal(raw, &asked)
	}
	var listed acp.ListSessionsResult
	if err := c.Conn.Call(ctx, method, params, &listed); err != nil {
		return err
	}
	kept := listed.Sessions[:0]
	for _, row := range listed.Sessions {
		if asked.Cwd == "" {
			row.Cwd = folder(row.Cwd)
			kept = append(kept, row)
		} else if layout.SamePath(folder(row.Cwd), asked.Cwd) {
			row.Cwd = asked.Cwd
			kept = append(kept, row)
		}
	}
	listed.Sessions = kept
	raw, err := json.Marshal(listed)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

var drivePath = regexp.MustCompile(`^/[A-Za-z]:(/|$)`)

// folder is a folder as the command line wrote it, without the slash it
// puts before a Windows drive.
func folder(path string) string {
	if drivePath.MatchString(path) {
		return path[1:]
	}
	return path
}
