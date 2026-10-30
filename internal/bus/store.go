// Package bus is Dinah's message bus: sessions, workspaces, messages,
// rooms, releases and verdicts, kept in one SQLite file that every dinah
// process opens.
package bus

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"studio/internal/profile"
)

// Bus is an open connection to the bus's database.
type Bus struct {
	db  *sql.DB
	now func() time.Time
	// dir is the folder the database is in.
	dir string
}

const schema = `
CREATE TABLE IF NOT EXISTS workspaces (
	key  TEXT PRIMARY KEY,
	path TEXT NOT NULL,
	name TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS sessions (
	id             TEXT PRIMARY KEY,
	name           TEXT NOT NULL,
	workspace      TEXT NOT NULL,
	title          TEXT NOT NULL DEFAULT '',
	started_at     TEXT NOT NULL,
	last_seen      TEXT NOT NULL,
	ended_at       TEXT,
	stop_continues INTEGER NOT NULL DEFAULT 0,
	hops           INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS messages (
	id             INTEGER PRIMARY KEY AUTOINCREMENT,
	created_at     TEXT NOT NULL,
	kind           TEXT NOT NULL,
	from_session   TEXT NOT NULL DEFAULT '',
	from_name      TEXT NOT NULL,
	from_workspace TEXT NOT NULL DEFAULT '',
	to_label       TEXT NOT NULL,
	room           TEXT NOT NULL DEFAULT '',
	reply_to       INTEGER NOT NULL DEFAULT 0,
	text           TEXT NOT NULL,
	version        TEXT NOT NULL DEFAULT '',
	verdict        TEXT NOT NULL DEFAULT '',
	hops           INTEGER NOT NULL DEFAULT 0,
	mentions       TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS attachments (
	message_id INTEGER NOT NULL,
	idx        INTEGER NOT NULL,
	path       TEXT NOT NULL,
	rel        TEXT NOT NULL,
	sha256     TEXT NOT NULL,
	size       INTEGER NOT NULL,
	mime       TEXT NOT NULL DEFAULT '',
	stored     INTEGER NOT NULL DEFAULT 0,
	removed_at TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (message_id, idx)
);
CREATE TABLE IF NOT EXISTS host_results (
	key        TEXT PRIMARY KEY,
	recipient  TEXT NOT NULL,
	message_id INTEGER NOT NULL REFERENCES messages(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS deliveries (
	id                INTEGER PRIMARY KEY AUTOINCREMENT,
	message_id        INTEGER NOT NULL,
	recipient_kind    TEXT NOT NULL,
	recipient         TEXT NOT NULL,
	delivered_session TEXT NOT NULL DEFAULT '',
	delivered_at      TEXT
);
CREATE INDEX IF NOT EXISTS deliveries_waiting ON deliveries (recipient_kind, recipient, delivered_at);
CREATE TABLE IF NOT EXISTS rooms (
	name       TEXT PRIMARY KEY,
	created_at TEXT NOT NULL,
	title      TEXT NOT NULL DEFAULT '',
	turn_budget INTEGER NOT NULL DEFAULT -1,
	turns_used  INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS room_renames (
	room       TEXT NOT NULL,
	old_title  TEXT NOT NULL,
	renamed_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS team_members (
	member  TEXT PRIMARY KEY,
	lead    TEXT NOT NULL,
	since   TEXT NOT NULL,
	retired INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS team_members_lead ON team_members (lead);
CREATE TABLE IF NOT EXISTS team_budgets (
	lead      TEXT NOT NULL,
	provider  TEXT NOT NULL,
	unit      TEXT NOT NULL,
	allowance INTEGER NOT NULL,
	PRIMARY KEY (lead, provider)
);
CREATE TABLE IF NOT EXISTS team_usage (
	session  TEXT NOT NULL,
	provider TEXT NOT NULL,
	unit     TEXT NOT NULL,
	at       INTEGER NOT NULL,
	amount   INTEGER NOT NULL,
	known    INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS team_usage_session ON team_usage (session, at);
CREATE TABLE IF NOT EXISTS relays (
	message_id INTEGER PRIMARY KEY,
	member     TEXT NOT NULL,
	lead       TEXT NOT NULL,
	created_at TEXT NOT NULL,
	state      TEXT NOT NULL DEFAULT 'pending',
	reason     TEXT NOT NULL DEFAULT '',
	reported   INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS room_members (
	room        TEXT NOT NULL,
	member_kind TEXT NOT NULL,
	member      TEXT NOT NULL,
	PRIMARY KEY (room, member_kind, member)
);
CREATE TABLE IF NOT EXISTS subscriptions (
	consumer TEXT NOT NULL,
	library  TEXT NOT NULL,
	PRIMARY KEY (consumer, library)
);
CREATE TABLE IF NOT EXISTS links (
	workspace_a TEXT NOT NULL,
	workspace_b TEXT NOT NULL,
	hop_limit   INTEGER NOT NULL,
	created_at  TEXT NOT NULL,
	PRIMARY KEY (workspace_a, workspace_b),
	CHECK (workspace_a <= workspace_b)
);
CREATE TABLE IF NOT EXISTS steers (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	session      TEXT NOT NULL,
	text         TEXT NOT NULL,
	created_at   TEXT NOT NULL,
	delivered_at TEXT,
	withdrawn_at TEXT
);
CREATE INDEX IF NOT EXISTS steers_waiting ON steers (session, delivered_at, withdrawn_at);
CREATE TABLE IF NOT EXISTS agent_sessions (
	agent_session TEXT PRIMARY KEY,
	session       TEXT NOT NULL,
	bound_at      TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS hook_turns (
 session TEXT PRIMARY KEY,
 token TEXT NOT NULL,
 active INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS steering_submissions (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 session TEXT NOT NULL,
 payload TEXT NOT NULL,
 state TEXT NOT NULL,
 created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS web_codes (
	code       TEXT PRIMARY KEY,
	expires_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS web_sessions (
	token      TEXT PRIMARY KEY,
	created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS changes (
	id      INTEGER PRIMARY KEY CHECK (id = 1),
	counter INTEGER NOT NULL
);
INSERT OR IGNORE INTO changes (id, counter) VALUES (1, 0);
CREATE TABLE IF NOT EXISTS person (
	id   INTEGER PRIMARY KEY CHECK (id = 1),
	name TEXT NOT NULL
);
`

// HostedVar is set in the environment of the agents Dinah's window runs.
// Those agents load the person's own Claude Code settings, so they start
// `dinah mcp` and run `dinah hook` as any session does; the window puts its
// chats on the bus itself, and those copies stand down when they see this
// variable. Claude Code documents that a hook process inherits the parent
// environment (https://code.claude.com/docs/en/hooks). The profile gives
// its name, DINAH_HOSTED for Dinah.
var HostedVar = profile.Current().HostedVar()

// Home returns the folder where Dinah keeps its state: the profile's home
// variable (DINAH_HOME) when it is set, otherwise the profile's home folder
// (.dinah) in the person's home folder.
func Home() (string, error) {
	p := profile.Current()
	if h := os.Getenv(p.HomeVar()); h != "" {
		return h, nil
	}
	u, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(u, p.HomeFolder), nil
}

// Open opens the bus database in Home, creating it when it is missing.
func Open() (*Bus, error) {
	home, err := Home()
	if err != nil {
		return nil, err
	}
	return OpenAt(filepath.Join(home, "bus.db"))
}

// OpenAt opens the bus database at path.
func OpenAt(path string) (*Bus, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	dsn := "file:" + filepath.ToSlash(path) +
		"?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("prepare the bus database: %w", err)
	}
	if err := upgrade(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("upgrade the bus database: %w", err)
	}
	return &Bus{db: db, now: time.Now, dir: filepath.Dir(path)}, nil
}

// columns added since the first release, which CREATE TABLE IF NOT EXISTS
// does not add to a database made before them.
var added = []struct{ table, column, decl string }{
	{"steers", "turn_token", "TEXT NOT NULL DEFAULT ''"},
	{"agent_sessions", "turn_token", "TEXT NOT NULL DEFAULT ''"},
	{"agent_sessions", "prompt_id", "TEXT NOT NULL DEFAULT ''"},
	{"sessions", "hops", "INTEGER NOT NULL DEFAULT 0"},
	{"messages", "hops", "INTEGER NOT NULL DEFAULT 0"},
	{"messages", "mentions", "TEXT NOT NULL DEFAULT ''"},
	// A workspace's label is its name in the window (titles.go), and a
	// message's to_kind and to_key say whom it was addressed to, so its
	// heading can show the recipient's current title.
	{"workspaces", "label", "TEXT NOT NULL DEFAULT ''"},
	{"messages", "to_kind", "TEXT NOT NULL DEFAULT ''"},
	{"messages", "to_key", "TEXT NOT NULL DEFAULT ''"},
	// A group chat's name as people and agents read it; the row's name is
	// its key (rooms.go).
	{"rooms", "title", "TEXT NOT NULL DEFAULT ''"},
	// A group's turn budget and the turns used since the person last posted
	// in it (rooms.go); -1 is no budget, which every group starts with.
	{"rooms", "turn_budget", "INTEGER NOT NULL DEFAULT -1"},
	{"rooms", "turns_used", "INTEGER NOT NULL DEFAULT 0"},
	// The person's attached files are copies the bus keeps in its own
	// folder (attachments.go): stored marks them, mime is the type the
	// window gave, and removed_at is when the person removed the copy.
	// A team's channel is marked with its lead's session, and a message a team member
	// addressed to the person carries the lead it was rerouted to (teams.go).
	{"rooms", "lead", "TEXT NOT NULL DEFAULT ''"},
	{"messages", "relay_lead", "TEXT NOT NULL DEFAULT ''"},
	{"attachments", "mime", "TEXT NOT NULL DEFAULT ''"},
	{"attachments", "stored", "INTEGER NOT NULL DEFAULT 0"},
	{"attachments", "removed_at", "TEXT NOT NULL DEFAULT ''"},
}

func upgrade(db *sql.DB) error {
	for _, a := range added {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, a.table, a.column).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		if _, err := db.Exec(fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, a.table, a.column, a.decl)); err != nil {
			// Another dinah process may have added it first.
			if strings.Contains(err.Error(), "duplicate column") {
				continue
			}
			return err
		}
		if a.table == "messages" && a.column == "to_key" {
			if err := addressOldMessages(db); err != nil {
				return err
			}
		}
	}
	if err := widenLinks(db); err != nil {
		return err
	}
	return convertRooms(db)
}

// widenLinks lets a workspace trust itself in a bus file made when a link
// had to join two different workspaces (links.go, Trust). SQLite cannot
// change a table's CHECK in place, so the table is built again under the
// new rule, in one transaction, by the first process to open the file. The
// table's text comes from sqlite_master, which SQLite documents as the
// schema's record.
func widenLinks(db *sql.DB) error {
	var ddl string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'links'`).Scan(&ddl); err != nil {
		return err
	}
	if !strings.Contains(ddl, "workspace_a < workspace_b") {
		return nil
	}
	t, err := db.Begin()
	if err != nil {
		return err
	}
	for _, q := range []string{
		`ALTER TABLE links RENAME TO links_old`,
		`CREATE TABLE links (
	workspace_a TEXT NOT NULL,
	workspace_b TEXT NOT NULL,
	hop_limit   INTEGER NOT NULL,
	created_at  TEXT NOT NULL,
	PRIMARY KEY (workspace_a, workspace_b),
	CHECK (workspace_a <= workspace_b)
)`,
		`INSERT INTO links SELECT workspace_a, workspace_b, hop_limit, created_at FROM links_old`,
		`DROP TABLE links_old`,
	} {
		if _, err := t.Exec(q); err != nil {
			t.Rollback()
			// Another dinah process may have rebuilt it first.
			if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "no such table") {
				return nil
			}
			return err
		}
	}
	return t.Commit()
}

// addressOldMessages records whom each message sent before to_kind existed
// was addressed to, where its record shows it, so its heading shows the
// recipient's current title rather than the name it was sent to. A message
// to a workspace has that workspace's name as its label; one to a chat was
// delivered to that chat alone. Any other message keeps its label. The
// process that adds the column does this once, in one transaction.
func addressOldMessages(db *sql.DB) error {
	t, err := db.Begin()
	if err != nil {
		return err
	}
	for _, q := range []string{
		`UPDATE messages SET to_kind = 'human' WHERE to_kind = '' AND room = '' AND to_label = 'paul'`,
		`UPDATE messages SET to_kind = 'room', to_key = room WHERE to_kind = '' AND room <> ''`,
		`UPDATE messages SET to_kind = 'workspace', to_key = (SELECT key FROM workspaces WHERE name = messages.to_label)
			WHERE to_kind = '' AND kind <> 'release' AND to_label IN (SELECT name FROM workspaces)`,
		`UPDATE messages SET to_kind = 'session', to_key = (SELECT recipient FROM deliveries d WHERE d.message_id = messages.id)
			WHERE to_kind = '' AND kind IN ('text', 'host_result')
			AND (SELECT COUNT(*) FROM deliveries d WHERE d.message_id = messages.id) = 1
			AND (SELECT COUNT(*) FROM deliveries d WHERE d.message_id = messages.id AND d.recipient_kind = 'session') = 1`,
	} {
		if _, err := t.Exec(q); err != nil {
			t.Rollback()
			return err
		}
	}
	return t.Commit()
}

// Close closes the database.
func (b *Bus) Close() error { return b.db.Close() }

func (b *Bus) stamp() string { return b.now().UTC().Format(time.RFC3339Nano) }

// tx runs f in an immediate transaction and records a change for the page.
func (b *Bus) tx(ctx context.Context, f func(*sql.Tx) error) error {
	t, err := b.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := f(t); err != nil {
		t.Rollback()
		return err
	}
	if _, err := t.Exec(`UPDATE changes SET counter = counter + 1 WHERE id = 1`); err != nil {
		t.Rollback()
		return err
	}
	return t.Commit()
}

// ChangeCounter returns a number that grows on every change to the bus.
func (b *Bus) ChangeCounter(ctx context.Context) (int64, error) {
	var n int64
	err := b.db.QueryRowContext(ctx, `SELECT counter FROM changes WHERE id = 1`).Scan(&n)
	return n, err
}

// WorkspaceKey returns the key under which a workspace folder is stored.
// Windows and macOS compare paths without regard to case.
func WorkspaceKey(path string) string {
	p := filepath.Clean(path)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		p = strings.ToLower(p)
	}
	return p
}

// ErrNotFound is returned when a session, workspace, room or message does
// not exist.
var ErrNotFound = errors.New("not found")
