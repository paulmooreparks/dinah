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
	"io"

	"studio/internal/acp"
	"studio/internal/acpprefix"
)

// Prefix begins the ID of every chat Antigravity makes, on Dinah's side.
const Prefix = "antigravity:"

// Program is the command line's name, and ServerArgs what starts its
// protocol server.
const Program = "agy"

var ServerArgs = []string{"acp"}

// Start runs the command line's protocol server in dir.
func Start(program, dir string, h acp.Handler, trace io.Writer) (*acpprefix.Conn, error) {
	c, err := acp.Start(program, ServerArgs, dir, acpprefix.Handler{H: h, Prefix: Prefix}, acp.Options{Trace: trace})
	if err != nil {
		return nil, err
	}
	return &acpprefix.Conn{Conn: c, Prefix: Prefix}, nil
}
