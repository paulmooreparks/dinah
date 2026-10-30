package bus

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"studio/internal/appletcontext"
)

// A chat is known by its title (docs/plans/group-chats.md). The bus keeps
// each session's title in its row, and the window writes it there whenever
// the title a person sees changes: when the chat joins the bus, when its
// agent titles it and when the person renames it. Nothing else keeps a copy,
// so every heading, list and address reads the title as it is now.
//
// A workspace's label is its name in the window, which the window writes in
// the same way. Its name on the bus, made from its folder, stays the address
// agents send to a whole workspace by.
//
// The handle a session was once given from its folder, such as studio-15,
// was kept as an address for one release after titles came, and is gone:
// a chat is reached by its title and by nothing else.

// WindowSessionPrefix begins the ID of every chat in Dinah's window on the
// bus. Such a chat keeps its ID across closing and reopening, so a closed
// one is still a chat that can be in a group; a session outside the window
// that has ended may never come back.
const WindowSessionPrefix = "window-"

// UntitledPrefix begins the title shown for a chat that has none yet, which
// ends with its workspace's label.
const UntitledPrefix = "Untitled Chat in "

// TitleLimit is the most characters a title may have.
const TitleLimit = 120

// injectedMarks begin text Dinah adds to a prompt: its context blocks, which
// carry a private applet authorization, and its handoff banner. An agent may
// make a chat's title from the whole of its first prompt, so a title can begin
// with the person's words and run into these.
var injectedMarks = []string{"[Dinah ", "dinah-applet-v1:"}

// CleanTitle is a title fit to be stored and shown: the first line of what the
// person or the agent wrote, without anything Dinah injected into the prompt,
// on one line and at most TitleLimit characters. A title that is only injected
// text is empty. It is applied wherever a title is stored, whatever its source
// (the person's name, the agent's title or an ancestor's name), since every
// agent that lists the chats reads them.
func CleanTitle(title string) string {
	line := firstLine(title)
	if reservedMention(fold(line)) {
		// A title that is a word mentioning every member could never be
		// mentioned by itself (Everyone).
		line += " chat"
	}
	return line
}

// firstLine is the first line of a text that has any, without anything Dinah
// injected, on one line and at most TitleLimit characters.
func firstLine(title string) string {
	for _, mark := range injectedMarks {
		if i := strings.Index(title, mark); i >= 0 {
			title = title[:i]
		}
	}
	for _, line := range strings.Split(title, "\n") {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" {
			continue
		}
		if r := []rune(line); len(r) > TitleLimit {
			line = strings.TrimRight(string(r[:TitleLimit-1]), " ,.;:") + "…"
		}
		return line
	}
	return ""
}

// ShortenTitles replaces every stored title that CleanTitle changes with its
// cleaned form, and says how many it changed. The window runs it once at
// start, for the titles stored before they were limited.
func (b *Bus) ShortenTitles(ctx context.Context) (int, error) {
	changed := 0
	err := b.tx(ctx, func(t *sql.Tx) error {
		rows, err := t.Query(`SELECT id, title FROM sessions`)
		if err != nil {
			return err
		}
		type fix struct{ id, title string }
		var fixes []fix
		for rows.Next() {
			var id, title string
			if err := rows.Scan(&id, &title); err != nil {
				rows.Close()
				return err
			}
			if clean := CleanTitle(title); clean != title {
				fixes = append(fixes, fix{id, clean})
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		for _, f := range fixes {
			if _, err := t.Exec(`UPDATE sessions SET title = ? WHERE id = ?`, f.title, f.id); err != nil {
				return err
			}
			changed++
		}
		return nil
	})
	return changed, err
}

// RedactStoredMessages removes the applet contexts from messages stored before
// the bus stopped keeping them, and says how many messages it changed. It
// changes only a message that holds one, so a clean message is left exactly as
// it was. A context from this run stays valid until its chat closes, so a
// message that held one was a usable authorization for any chat that read it.
// The window runs it at start.
func (b *Bus) RedactStoredMessages(ctx context.Context) (int, error) {
	changed := 0
	err := b.tx(ctx, func(t *sql.Tx) error {
		rows, err := t.Query(`SELECT id, text FROM messages WHERE instr(text, ?) > 0`, appletcontext.Prefix)
		if err != nil {
			return err
		}
		type fix struct {
			id   int64
			text string
		}
		var fixes []fix
		for rows.Next() {
			var id int64
			var text string
			if err := rows.Scan(&id, &text); err != nil {
				rows.Close()
				return err
			}
			if clean := appletcontext.Redact(text); clean != text {
				fixes = append(fixes, fix{id, clean})
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		for _, f := range fixes {
			if _, err := t.Exec(`UPDATE messages SET text = ? WHERE id = ?`, f.text, f.id); err != nil {
				return err
			}
			changed++
		}
		return nil
	})
	return changed, err
}

// DisplayTitle is the title shown for a chat titled title in a workspace
// labelled workspace.
func DisplayTitle(title, workspace string) string {
	if t := strings.TrimSpace(title); t != "" {
		return t
	}
	return UntitledPrefix + workspace
}

// Display is the workspace as a person reads it: its label, or its name on
// the bus when it has none.
func (w Workspace) Display() string {
	if w.Label != "" {
		return w.Label
	}
	return w.Name
}

// Display is the session's title, or the title shown for an untitled chat.
func (s Session) Display() string { return DisplayTitle(s.Title, s.WorkspaceLabel) }

// Address is how an agent writes the session as a recipient, with its
// workspace so that two chats of the same title can be told apart.
func (s Session) Address() string { return s.Display() + " (" + s.WorkspaceName + ")" }

// SetTitle records a session's title, or with an empty title, that it has
// none. The window calls it whenever a chat's title changes.
func (b *Bus) SetTitle(ctx context.Context, id, title string) error {
	title = CleanTitle(title)
	return b.tx(ctx, func(t *sql.Tx) error {
		res, err := t.Exec(`UPDATE sessions SET title = ? WHERE id = ? AND title <> ?`, title, id, title)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 && !taken(t, `SELECT 1 FROM sessions WHERE id = ?`, id) {
			return fmt.Errorf("session %s: %w", id, ErrNotFound)
		}
		return nil
	})
}

// LabelWorkspace puts a folder on the bus as a workspace, as EnsureWorkspace
// does, and records its label. The window calls it whenever a workspace's
// name changes.
func (b *Bus) LabelWorkspace(ctx context.Context, path, label string) (Workspace, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Workspace{}, err
	}
	label = strings.Join(strings.Fields(label), " ")
	var ws Workspace
	err = b.tx(ctx, func(t *sql.Tx) error {
		ws, err = ensureWorkspace(t, abs)
		if err != nil || ws.Label == label {
			return err
		}
		ws.Label = label
		_, err = t.Exec(`UPDATE workspaces SET label = ? WHERE key = ?`, label, ws.Key)
		return err
	})
	return ws, err
}

// fold is a title or name as addresses compare it: in lower case, with its
// runs of spaces made one.
func fold(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }

// chat is a session as an address finds it. open is whether it is open now.
type chat struct {
	id, title, workspace, wsName, wsLabel string
	open                                  bool
}

func (c chat) display() string { return DisplayTitle(c.title, c.wsLabel) }

// address is how an agent writes the chat: its title with its workspace's
// name in parentheses.
func (c chat) address() string { return c.display() + " (" + c.wsName + ")" }

const chatSelect = `SELECT s.id, s.title, s.workspace, w.name, w.label, s.ended_at IS NULL FROM sessions s
	JOIN workspaces w ON w.key = s.workspace`

func scanChats(rows *sql.Rows, err error) ([]chat, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []chat
	for rows.Next() {
		var c chat
		if err := rows.Scan(&c.id, &c.title, &c.workspace, &c.wsName, &c.wsLabel, &c.open); err != nil {
			return nil, err
		}
		if c.wsLabel == "" {
			c.wsLabel = c.wsName
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func liveChats(t *sql.Tx) ([]chat, error) {
	return scanChats(t.Query(chatSelect + ` WHERE s.ended_at IS NULL ORDER BY w.name, s.title, s.id`))
}

// knownChats are the chats a group can have: the open ones, and the chats
// of the window that are closed (WindowSessionPrefix).
func knownChats(t *sql.Tx) ([]chat, error) {
	return scanChats(t.Query(chatSelect+` WHERE s.ended_at IS NULL OR s.id LIKE ? ORDER BY w.name, s.title, s.id`, WindowSessionPrefix+"%"))
}

// errNoChat is findChat's answer when no chat has the title.
var errNoChat = errors.New("no chat")

// findChat finds the live chat an address is for (matchChat).
func findChat(t *sql.Tx, address string) (chat, error) {
	chats, err := liveChats(t)
	if err != nil {
		return chat{}, err
	}
	return matchChat(chats, address)
}

// findKnownChat finds the chat an address is for among the open chats, and
// when none of them matches, among the closed chats of the window. An exact
// session ID is also an address here, for the window, which knows its
// chats by their IDs.
func findKnownChat(t *sql.Tx, address string) (chat, error) {
	chats, err := knownChats(t)
	if err != nil {
		return chat{}, err
	}
	for _, c := range chats {
		if c.id == strings.TrimSpace(address) {
			return c, nil
		}
	}
	var open []chat
	for _, c := range chats {
		if c.open {
			open = append(open, c)
		}
	}
	if c, err := matchChat(open, address); !errors.Is(err, errNoChat) {
		return c, err
	}
	return matchChat(chats, address)
}

// matchChat finds the chat among chats an address is for. The address is a
// chat's title, compared without regard to case; or its title with its
// workspace in parentheses, as "Title (studio)" or "Title (workspace
// studio)", by the workspace's name or label, for two chats that share a
// title. A title that more than one chat has is refused with the chats it
// could be.
func matchChat(chats []chat, address string) (chat, error) {
	q := fold(address)
	var matches []chat
	for _, c := range chats {
		if fold(c.display()) == q {
			matches = append(matches, c)
		}
	}
	if len(matches) == 0 {
		if title, ws, ok := splitWorkspace(address); ok {
			for _, c := range chats {
				if fold(c.display()) == fold(title) && (fold(ws) == fold(c.wsName) || fold(ws) == fold(c.wsLabel)) {
					matches = append(matches, c)
				}
			}
		}
	}
	switch {
	case len(matches) == 1:
		return matches[0], nil
	case len(matches) > 1:
		return chat{}, ambiguous(address, matches)
	}
	return chat{}, errNoChat
}

// splitWorkspace splits "Title (workspace)" or "Title (workspace name)" into
// the title and the workspace.
func splitWorkspace(address string) (title, workspace string, ok bool) {
	a := strings.TrimSpace(address)
	if !strings.HasSuffix(a, ")") {
		return "", "", false
	}
	at := strings.LastIndex(a, "(")
	if at <= 0 {
		return "", "", false
	}
	title = strings.TrimSpace(a[:at])
	workspace = strings.TrimSpace(a[at+1 : len(a)-1])
	if w, found := strings.CutPrefix(strings.ToLower(workspace), "workspace "); found {
		workspace = strings.TrimSpace(w)
	}
	return title, workspace, title != "" && workspace != ""
}

func ambiguous(address string, matches []chat) error {
	var names []string
	for _, c := range matches {
		names = append(names, fmt.Sprintf("%q (workspace %s)", c.display(), c.wsName))
	}
	return fmt.Errorf("%d chats are called %q: %s. Add the workspace in parentheses, such as %q",
		len(matches), strings.TrimSpace(address), strings.Join(names, ", "), matches[0].display()+" ("+matches[0].wsName+")")
}

// workspaceByLabel finds the workspace whose label is the address, when
// exactly one workspace has that label.
func workspaceByLabel(t *sql.Tx, address string) (key, name string, ok bool, err error) {
	rows, err := t.Query(`SELECT key, name, label FROM workspaces WHERE label <> ''`)
	if err != nil {
		return "", "", false, err
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var k, nm, label string
		if err := rows.Scan(&k, &nm, &label); err != nil {
			return "", "", false, err
		}
		if fold(label) == fold(address) {
			key, name = k, nm
			n++
		}
	}
	return key, name, n == 1, rows.Err()
}

// sessionLabel is a session's title as it is now, for a message's label.
func sessionLabel(t *sql.Tx, id string) (string, error) {
	var title, name, label string
	err := t.QueryRow(`SELECT s.title, w.name, w.label FROM sessions s JOIN workspaces w ON w.key = s.workspace
		WHERE s.id = ?`, id).Scan(&title, &name, &label)
	if label == "" {
		label = name
	}
	return DisplayTitle(title, label), err
}
