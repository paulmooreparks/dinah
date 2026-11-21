package codexagent

import (
	"context"
	"strings"
)

// Codex keeps one store of threads for every program that uses it, and its
// own app lists them. The calls in this file keep Dinah's threads tidy in
// that store. Each one is in the app-server protocol's published schema
// (`codex app-server generate-json-schema`, release 0.162.0), and
// docs/design/codex-thread-housekeeping.md quotes the schema text.
//
// None of these methods reads the text of a Codex error. A caller logs a
// failure and goes on, since none of these calls is needed for a chat to
// work.

// OpenFailureAdvice is what the window adds below Codex's own words when a
// Codex chat could not be opened. It is shown for every failed open, whatever
// Codex said, because no documented call tells Dinah whether another program
// holds the thread.
const OpenFailureAdvice = "Codex gives a conversation to one program at a time, so if this conversation is open in another program that uses Codex, such as the Codex app or an editor, closing it there and pressing Try Again may free it."

// SetSessionTitle gives the session's thread a title with thread/name/set
// (ThreadSetNameParams: threadId and name, both required). Thread.name is
// documented as the "Optional user-facing thread title." Nothing is sent for
// an empty title, or for a loaded thread whose name is already the title.
func (c *Conn) SetSessionTitle(ctx context.Context, sessionID, title string) error {
	id, err := native(sessionID)
	if err != nil {
		return err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return nil
	}
	c.mu.Lock()
	s := c.sessions[sessionID]
	same := s != nil && s.name == title
	c.mu.Unlock()
	if same {
		return nil
	}
	if err := c.Conn.Call(ctx, "thread/name/set", map[string]string{"threadId": id, "name": title}, nil); err != nil {
		return err
	}
	c.mu.Lock()
	if c.sessions[sessionID] == s && s != nil {
		s.name = title
	}
	c.mu.Unlock()
	return nil
}

// ArchiveSession archives the session's thread with thread/archive
// (ThreadArchiveParams: threadId). The schema documents the effect on
// thread/list only: with ThreadListParams.archived false or null, "only
// non-archived threads are returned."
func (c *Conn) ArchiveSession(ctx context.Context, sessionID string) error {
	id, err := native(sessionID)
	if err != nil {
		return err
	}
	return c.Conn.Call(ctx, "thread/archive", map[string]string{"threadId": id}, nil)
}

// UnarchiveSession takes the session's thread out of the archive with
// thread/unarchive (ThreadUnarchiveParams: threadId).
func (c *Conn) UnarchiveSession(ctx context.Context, sessionID string) error {
	id, err := native(sessionID)
	if err != nil {
		return err
	}
	return c.Conn.Call(ctx, "thread/unarchive", map[string]string{"threadId": id}, nil)
}

// SessionEphemeral reports whether a loaded session's thread is ephemeral:
// started with ThreadStartParams.ephemeral true, or returned by Codex with
// Thread.ephemeral true ("Whether the thread is ephemeral and should not be
// materialized on disk."). Such a thread has nothing stored to delete, so it
// is ended by closing it.
func (c *Conn) SessionEphemeral(sessionID string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.sessions[sessionID]
	return s != nil && s.ephemeral
}
