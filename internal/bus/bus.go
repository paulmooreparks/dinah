package bus

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"studio/internal/appletcontext"
)

// Human is the address and sender name of the person who runs the bus.
const Human = "paul"

// MaxStopContinues is how many times in a row the Stop hook may keep a turn
// going to deliver messages. It matches the three-hop limit on automatic
// handoffs between agents.
const MaxStopContinues = 3

// MaxHops is the three-hop limit on automatic handoffs between agents. A
// message Paul sends is hop 0, and an agent's message is one hop past the
// furthest message that agent had read. The doorbell wakes a session only
// for messages within the limit; the rest wait until Paul speaks to that
// session or posts again.
const MaxHops = 3

// DoorbellTurn is the event Deliver is given in place of UserPromptSubmit
// when the prompt may have come from the doorbell rather than from Paul. It
// starts a fresh run of Stop continuations but keeps the hop count.
const DoorbellTurn = "DoorbellTurn"

// MaxAttachmentRead is the largest attachment read_attachment returns.
const MaxAttachmentRead = 1 << 20

// Session is a chat that has joined the bus. Title is the chat's title,
// empty when it has none yet, and WorkspaceLabel its workspace's label, or
// its name when it has no label. A chat is known by its title alone
// (titles.go).
type Session struct {
	ID             string
	Workspace      string // workspace key
	WorkspacePath  string
	WorkspaceName  string
	WorkspaceLabel string
	Title          string
	StartedAt      string
	LastSeen       string
	Ended          bool
}

// Workspace is a folder whose sessions use the bus. Name is its address on
// the bus, made from its folder, and Label its name in the window, empty
// until the window has given one.
type Workspace struct {
	Key   string
	Path  string
	Name  string
	Label string
}

// Attachment is a file sent with a message, as it was when it was sent. An
// agent's attachment is a live link to a file in its workspace, and Rel is
// the file's path there. The person's is a copy the bus keeps (Stored), Rel
// is the file's name as the person attached it, and Mime its type; Removed
// is set once the person has removed the copy (attachments.go).
type Attachment struct {
	Index   int
	Path    string
	Rel     string
	SHA256  string
	Size    int64
	Mime    string
	Stored  bool
	Removed bool
}

// Message is one message on the bus. FromName and ToLabel are the sender
// and the recipient as they were stored when it was sent. From, To and
// ToPerson are what a heading shows: a chat as it is titled now, and a
// workspace by its name for an agent (To) or its label for a person
// (ToPerson). Messages made in a test may leave them empty, and the headings
// then show FromName and ToLabel.
type Message struct {
	ID                 int64
	CreatedAt          string
	Kind               string // text, release, verdict or question (questions.go)
	FromSession        string
	FromName           string
	From               string
	FromWorkspace      string // workspace name, empty for the person
	FromWorkspaceLabel string // workspace label, empty for the person
	ToLabel            string
	To                 string
	ToPerson           string
	Room               string
	// RoomTitle is the group chat's name as it is now, for a message in a
	// group, and Mentioned the chats the message mentions, with the person
	// as Human when it mentions the person (rooms.go).
	RoomTitle   string
	Mentioned   []Mention
	ReplyTo     int64
	Text        string
	Version     string // for a release
	Verdict     string // for a verdict
	Attachments []Attachment
	Unread      bool // for the person's inbox
	// Trusted is set on a message delivered to a session across a trusted
	// link, so the agent may act on it as on Paul's request (links.go).
	Trusted bool
	// Asked is set on a message delivered to a session that is asked to
	// answer it: one that is not a group's, or a group's that mentions the
	// session (rooms.go).
	Asked bool
	// Held is set on a group's message that mentions the session after the
	// group's turn budget was used up. It is delivered as background and does
	// not ask the session to answer (rooms.go).
	Held bool
	// Notice is set on a group's message that does not mention the session. It
	// is delivered as one line, NoticeLine, and the session reads the whole
	// message with ReadMessage if it wants it (notice.go).
	Notice bool
	// ForPerson is set on a team member's message to the person that was rerouted to its lead:
	// it shows in the group for the person, who is not notified, and the lead is asked to
	// relay it. RelayLead is the lead's session and RelayLeadTitle its title as it is now.
	ForPerson      bool
	RelayLead      string
	RelayLeadTitle string
	// Relay is set on the lead's delivery of such a message: a request to relay (teams.go).
	Relay bool
	// mentions is the list as the messages table keeps it.
	mentions string
}

// Verdicts a consuming workspace may give a release.
var Verdicts = map[string]string{
	"upgrade": "Upgrade now",
	"sit_out": "Sit this cycle out",
	"blocked": "Blocked until the library changes",
}

// RegisterSession records a session that has started, or refreshes one that
// has resumed, in the workspace folder given.
func (b *Bus) RegisterSession(ctx context.Context, id, workspacePath, title string) (Session, error) {
	if id == "" {
		return Session{}, errors.New("a session needs an ID")
	}
	if workspacePath == "" {
		return Session{}, errors.New("a session needs a workspace folder")
	}
	title = CleanTitle(title)
	abs, err := filepath.Abs(workspacePath)
	if err != nil {
		return Session{}, err
	}
	err = b.tx(ctx, func(t *sql.Tx) error {
		ws, err := ensureWorkspace(t, abs)
		if err != nil {
			return err
		}
		now := b.stamp()
		if taken(t, `SELECT 1 FROM sessions WHERE id = ?`, id) {
			_, err = t.Exec(`UPDATE sessions SET last_seen = ?, ended_at = NULL, stop_continues = 0,
				title = CASE WHEN ? <> '' THEN ? ELSE title END WHERE id = ?`, now, title, title, id)
			return err
		}
		return insertSession(t, id, ws.Key, title, now, false)
	})
	if err != nil {
		return Session{}, err
	}
	return b.Session(ctx, id)
}

// EnsureWorkspace puts a folder on the bus as a workspace, before any session
// starts in it, so messages can be addressed to it and wait for its first
// session. A folder already on the bus is left as it is.
func (b *Bus) EnsureWorkspace(ctx context.Context, path string) (Workspace, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return Workspace{}, err
	}
	var ws Workspace
	err = b.tx(ctx, func(t *sql.Tx) error {
		ws, err = ensureWorkspace(t, abs)
		return err
	})
	return ws, err
}

func ensureWorkspace(t *sql.Tx, abs string) (Workspace, error) {
	key := WorkspaceKey(abs)
	var ws Workspace
	err := t.QueryRow(`SELECT key, path, name, label FROM workspaces WHERE key = ?`, key).Scan(&ws.Key, &ws.Path, &ws.Name, &ws.Label)
	if err == nil {
		return ws, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ws, err
	}
	base := strings.ToLower(filepath.Base(abs))
	name := base
	if taken(t, `SELECT 1 FROM workspaces WHERE name = ?`, name) {
		name = strings.ToLower(filepath.Base(filepath.Dir(abs))) + "." + base
	}
	for i := 2; taken(t, `SELECT 1 FROM workspaces WHERE name = ?`, name); i++ {
		name = base + "-" + strconv.Itoa(i)
	}
	ws = Workspace{Key: key, Path: abs, Name: name}
	_, err = t.Exec(`INSERT INTO workspaces (key, path, name) VALUES (?, ?, ?)`, ws.Key, ws.Path, ws.Name)
	return ws, err
}

func taken(t *sql.Tx, query string, args ...any) bool {
	var one int
	return t.QueryRow(query, args...).Scan(&one) == nil
}

// insertSession records a new session, or with closed set one that is known
// but not open, such as a closed chat the person puts in a group
// (KnowSession). The sessions table's name column once kept a handle made
// from the workspace's folder, such as studio-15, which agents could send
// to. Group chats made of chats removed that handle (docs/plans/
// group-chats.md); the column stays, since dropping a column from a file
// that older processes may still have open is not worth its risk, and a new
// row keeps the session's ID there, which nothing reads.
func insertSession(t *sql.Tx, id, workspace, title, now string, closed bool) error {
	var ended any
	if closed {
		ended = now
	}
	_, err := t.Exec(`INSERT INTO sessions (id, name, workspace, title, started_at, last_seen, ended_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, id, id, workspace, CleanTitle(title), now, now, ended)
	return err
}

// KnowSession records a chat that is not open, in its workspace folder and
// with its title, so that it can be put in a group chat and messages can
// wait for it. A session the bus already has keeps its state and takes the
// title given, when one is given. When the chat opens, RegisterSession
// finds it by its ID.
func (b *Bus) KnowSession(ctx context.Context, id, workspacePath, title string) (Session, error) {
	if id == "" || workspacePath == "" {
		return Session{}, errors.New("a chat needs an ID and a workspace folder")
	}
	abs, err := filepath.Abs(workspacePath)
	if err != nil {
		return Session{}, err
	}
	title = CleanTitle(title)
	err = b.tx(ctx, func(t *sql.Tx) error {
		ws, err := ensureWorkspace(t, abs)
		if err != nil {
			return err
		}
		if taken(t, `SELECT 1 FROM sessions WHERE id = ?`, id) {
			if title == "" {
				return nil
			}
			_, err = t.Exec(`UPDATE sessions SET title = ? WHERE id = ?`, title, id)
			return err
		}
		return insertSession(t, id, ws.Key, title, b.stamp(), true)
	})
	if err != nil {
		return Session{}, err
	}
	return b.Session(ctx, id)
}

// Session returns a session by its ID.
func (b *Bus) Session(ctx context.Context, id string) (Session, error) {
	row := b.db.QueryRowContext(ctx, sessionSelect+` WHERE s.id = ?`, id)
	s, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return s, fmt.Errorf("session %s: %w", id, ErrNotFound)
	}
	return s, err
}

const sessionSelect = `SELECT s.id, s.workspace, w.path, w.name, CASE WHEN w.label <> '' THEN w.label ELSE w.name END,
	s.title, s.started_at, s.last_seen, s.ended_at IS NOT NULL FROM sessions s JOIN workspaces w ON w.key = s.workspace`

type scanner interface{ Scan(...any) error }

func scanSession(r scanner) (Session, error) {
	var s Session
	err := r.Scan(&s.ID, &s.Workspace, &s.WorkspacePath, &s.WorkspaceName, &s.WorkspaceLabel, &s.Title,
		&s.StartedAt, &s.LastSeen, &s.Ended)
	return s, err
}

// Sessions returns the sessions that have not ended, by workspace and title.
func (b *Bus) Sessions(ctx context.Context) ([]Session, error) {
	rows, err := b.db.QueryContext(ctx, sessionSelect+` WHERE s.ended_at IS NULL ORDER BY w.name, s.title = '', lower(s.title), s.started_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// Workspaces returns every workspace the bus knows, by name.
func (b *Bus) Workspaces(ctx context.Context) ([]Workspace, error) {
	rows, err := b.db.QueryContext(ctx, `SELECT key, path, name, label FROM workspaces ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Workspace
	for rows.Next() {
		var w Workspace
		if err := rows.Scan(&w.Key, &w.Path, &w.Name, &w.Label); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// EndSession records that a session has ended.
func (b *Bus) EndSession(ctx context.Context, id string) error {
	return b.tx(ctx, func(t *sql.Tx) error {
		_, err := t.Exec(`UPDATE sessions SET ended_at = ? WHERE id = ? AND ended_at IS NULL`, b.stamp(), id)
		return err
	})
}

// Sender is who sends a message: a session, or the person when Session is nil.
type Sender struct {
	Session *Session
}

// name is the sender as a message records it: the person's handle, or a
// chat's title as it was when it sent the message. A heading reads the
// chat's title as it is now (scanMessage), and this is shown only for a
// chat the bus no longer has.
func (s Sender) name() string {
	if s.Session == nil {
		return Human
	}
	return s.Session.Display()
}

// recipient is one delivery to make. A delivery of kind humanRoom records
// that the person has not yet read a group's message in the group's window;
// one of kind human puts the message in the person's inbox.
type recipient struct {
	kind  string // session, workspace, human or human_room
	value string // session ID, workspace key, or Human
}

// humanRoom is the kind of the delivery that keeps a group's message unread
// for the person until the group's window shows it.
const humanRoom = "human_room"

// resolved is where an address leads. toKind and toKey record whom the
// message is addressed to, so its heading can show the recipient as it is
// titled now: a session, a workspace, the person, a room, or the subscribers
// of a workspace.
type resolved struct {
	label      string
	room       string
	roomTitle  string
	toKind     string
	toKey      string
	recipients []recipient
	// mentions is the list a group's message keeps (rooms.go, mentionList).
	mentions string
	// relay is the lead a team member's message to the person was rerouted to, and
	// relayMember the member (teams.go).
	relay string
}

func (b *Bus) resolve(t *sql.Tx, from Sender, to string) (resolved, error) {
	to = strings.TrimSpace(to)
	low := strings.ToLower(to)
	switch {
	case low == "":
		return resolved{}, errors.New("say who the message is for")
	case low == Human:
		return resolved{label: Human, toKind: "human", recipients: []recipient{{"human", Human}}}, nil
	case strings.HasPrefix(low, "#"):
		return groupResolved(t, from, strings.TrimPrefix(to, "#"))
	}
	// A workspace's name, its folder or its label reaches the workspace; any
	// other address is a chat's title (titles.go, findChat).
	var key, name string
	err := t.QueryRow(`SELECT key, name FROM workspaces WHERE name = ? OR key = ?`, low, WorkspaceKey(to)).Scan(&key, &name)
	if err == nil {
		return workspaceResolved(key, name), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return resolved{}, err
	}
	if key, name, ok, err := workspaceByLabel(t, to); err != nil {
		return resolved{}, err
	} else if ok {
		return workspaceResolved(key, name), nil
	}
	c, err := findChat(t, to)
	if err == nil {
		return chatResolved(c.id, c.display()), nil
	}
	if !errors.Is(err, errNoChat) {
		return resolved{}, err
	}
	return resolved{}, fmt.Errorf("nobody on the bus is called %q: use a chat's title, with its workspace in parentheses when two chats share it, a workspace's name, # and a group chat's name, or %s", to, Human)
}

func workspaceResolved(key, name string) resolved {
	return resolved{label: name, toKind: "workspace", toKey: key, recipients: []recipient{{"workspace", key}}}
}

func chatResolved(id, title string) resolved {
	return resolved{label: title, toKind: "session", toKey: id, recipients: []recipient{{"session", id}}}
}

// Send sends a text message, with any attachments, to an address: a chat's
// title (findChat), a workspace's name, # and a group chat's name, or paul.
// An empty address with a replyTo answers whoever sent that message.
func (b *Bus) Send(ctx context.Context, from Sender, to, text string, attach []string, replyTo int64) (Message, error) {
	return b.SendMentioning(ctx, from, to, text, attach, replyTo, nil)
}

// SendMentioning sends as Send does, with the chats a group's message
// mentions: each by its ID on the bus, by its title, or by its title with
// its workspace in parentheses, and Everyone or the person (rooms.go,
// mentionList). Only a message in a group chat may mention anyone. A
// session's attachments are paths in its workspace; the person attaches
// files with SendFiles, which keeps a copy of each (attachments.go).
func (b *Bus) SendMentioning(ctx context.Context, from Sender, to, text string, attach []string, replyTo int64, mentions []string) (Message, error) {
	if len(attach) > 0 && from.Session == nil {
		return Message{}, errors.New("the person has no workspace to link files from; attach the person's files with SendFiles")
	}
	atts, err := hashAttachments(from.Session, attach)
	if err != nil {
		return Message{}, err
	}
	return b.send(ctx, from, to, text, atts, replyTo, mentions)
}

// send records a message with its attachments already checked and hashed.
// A message needs words or at least one attachment.
func (b *Bus) send(ctx context.Context, from Sender, to, text string, atts []Attachment, replyTo int64, mentions []string) (Message, error) {
	if strings.TrimSpace(text) == "" && len(atts) == 0 {
		return Message{}, errors.New("the message is empty")
	}
	person := b.PersonName()
	var id int64
	err := b.tx(ctx, func(t *sql.Tx) error {
		var r resolved
		var err error
		if strings.TrimSpace(to) == "" {
			if replyTo == 0 {
				return errors.New("say who the message is for")
			}
			r, err = b.replyTarget(t, from, replyTo)
		} else {
			r, err = b.resolve(t, from, to)
		}
		if err != nil {
			return err
		}
		if err := mentionList(t, from, &r, mentions, person); err != nil {
			return err
		}
		id, err = b.insertMessage(t, from, "text", r, replyTo, text, "", "", atts)
		return err
	})
	if err != nil {
		return Message{}, err
	}
	return b.Message(ctx, id)
}

// ReplyInThread answers a thread: a message and the replies to it. The answer
// goes to whoever spoke last in the thread other than the sender, by the same
// rule as a reply (replyTarget), and it joins the thread, so a thread that
// began with the sender's own message stays one conversation.
func (b *Bus) ReplyInThread(ctx context.Context, from Sender, root int64, text string) (Message, error) {
	if strings.TrimSpace(text) == "" {
		return Message{}, errors.New("the message is empty")
	}
	var id int64
	err := b.tx(ctx, func(t *sql.Tx) error {
		var parent int64
		if err := t.QueryRow(`SELECT reply_to FROM messages WHERE id = ?`, root).Scan(&parent); errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("message %d: %w", root, ErrNotFound)
		} else if err != nil {
			return err
		}
		if parent != 0 {
			root = parent
		}
		// Whoever spoke is told apart by session, or for the person, by the
		// handle a message from no session keeps.
		mine := `from_session = '' AND from_name = '` + Human + `'`
		var me any = ""
		if from.Session != nil {
			mine, me = `from_session = ?`, from.Session.ID
		} else {
			mine += ` AND ? = ''`
		}
		var last int64
		err := t.QueryRow(`SELECT id FROM messages WHERE (id = ? OR reply_to = ?) AND kind <> '`+MemberKind+`' AND NOT (`+mine+`)
			ORDER BY id DESC LIMIT 1`, root, root, me).Scan(&last)
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("nobody else has spoken in this thread yet, so there is nobody to answer")
		}
		if err != nil {
			return err
		}
		r, err := b.replyTarget(t, from, last)
		if err != nil {
			return err
		}
		id, err = b.insertMessage(t, from, "text", r, root, text, "", "", nil)
		return err
	})
	if err != nil {
		return Message{}, err
	}
	return b.Message(ctx, id)
}

// replyTarget is where a reply to a message goes: its room, the person, the
// session that sent it while that session is open, or else its workspace,
// whose next session receives it.
func (b *Bus) replyTarget(t *sql.Tx, from Sender, id int64) (resolved, error) {
	var fromSession, fromWS, room, kind string
	err := t.QueryRow(`SELECT from_session, from_workspace, room, kind FROM messages WHERE id = ?`, id).
		Scan(&fromSession, &fromWS, &room, &kind)
	if errors.Is(err, sql.ErrNoRows) {
		return resolved{}, fmt.Errorf("message %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return resolved{}, err
	}
	if kind == ResultKind {
		return resolved{}, errors.New("a completion report needs no reply; continue the task or send a new question to its source")
	}
	if kind == MemberKind {
		return resolved{}, errors.New("a line saying who joined or left a group chat needs no reply")
	}
	if kind == QuestionKind {
		return resolved{}, errors.New("another chat's question is answered in the turn that receives it, and that turn's words go back to the asking chat by themselves; do not reply to it through the bus")
	}
	switch {
	case room != "":
		return b.resolve(t, from, "#"+room)
	case fromSession == "":
		return b.resolve(t, from, Human)
	}
	var live bool
	if t.QueryRow(`SELECT ended_at IS NULL FROM sessions WHERE id = ?`, fromSession).Scan(&live) == nil && live {
		title, err := sessionLabel(t, fromSession)
		if err != nil {
			return resolved{}, err
		}
		return chatResolved(fromSession, title), nil
	}
	var name string
	if err := t.QueryRow(`SELECT name FROM workspaces WHERE key = ?`, fromWS).Scan(&name); err != nil {
		return resolved{}, err
	}
	return workspaceResolved(fromWS, name), nil
}

func (b *Bus) insertMessage(t *sql.Tx, from Sender, kind string, r resolved, replyTo int64, text, version, verdict string, atts []Attachment) (int64, error) {
	// A message is read by other chats, so it never keeps a chat's applet
	// context, whoever wrote it (appletcontext).
	text = appletcontext.Redact(text)
	var fromSession, fromWS string
	hops := 0
	if from.Session != nil {
		fromSession, fromWS = from.Session.ID, from.Session.Workspace
		if err := t.QueryRow(`SELECT hops + 1 FROM sessions WHERE id = ?`, fromSession).Scan(&hops); err != nil {
			return 0, err
		}
	}
	res, err := t.Exec(`INSERT INTO messages (created_at, kind, from_session, from_name, from_workspace, to_label,
		to_kind, to_key, room, reply_to, text, version, verdict, hops, mentions, relay_lead) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		b.stamp(), kind, fromSession, from.name(), fromWS, r.label, r.toKind, r.toKey, r.room, replyTo, text, version, verdict, hops, r.mentions, r.relay)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if r.relay != "" {
		if _, err := t.Exec(`INSERT INTO relays (message_id, member, lead, created_at) VALUES (?, ?, ?, ?)`, id, fromSession, r.relay, b.stamp()); err != nil {
			return 0, err
		}
	}
	// The person speaking in a group refills its turn budget.
	if from.Session == nil && r.room != "" {
		if _, err := t.Exec(`UPDATE rooms SET turns_used = 0 WHERE name = ? AND turns_used <> 0`, r.room); err != nil {
			return 0, err
		}
	}
	for _, a := range atts {
		if _, err := t.Exec(`INSERT INTO attachments (message_id, idx, path, rel, sha256, size, mime, stored) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, a.Index, a.Path, a.Rel, a.SHA256, a.Size, a.Mime, a.Stored); err != nil {
			return 0, err
		}
	}
	// Every session open in a workspace the message is for sees it, the
	// sender's own workspace included, but not the sender; a workspace with
	// no other session open keeps it for the next. A group's member chats
	// are its recipients already (groupResolved).
	recipients, err := fanOut(t, r.recipients, fromSession)
	if err != nil {
		return 0, err
	}
	seen := map[recipient]bool{}
	for _, rc := range recipients {
		if seen[rc] {
			continue
		}
		seen[rc] = true
		if _, err := t.Exec(`INSERT INTO deliveries (message_id, recipient_kind, recipient) VALUES (?, ?, ?)`,
			id, rc.kind, rc.value); err != nil {
			return 0, err
		}
	}
	return id, nil
}

// hashAttachments checks that each file is inside the sender's workspace and
// records its hash, so a recipient can tell whether it has changed since.
func hashAttachments(s *Session, paths []string) ([]Attachment, error) {
	var out []Attachment
	for i, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(s.WorkspacePath, p)
		}
		p = filepath.Clean(p)
		rel, err := filepath.Rel(s.WorkspacePath, p)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("%s is outside this session's workspace (%s); only the workspace's own files can be attached", p, s.WorkspacePath)
		}
		sum, size, err := hashFile(p)
		if err != nil {
			return nil, err
		}
		out = append(out, Attachment{Index: i + 1, Path: p, Rel: filepath.ToSlash(rel), SHA256: sum, Size: size})
	}
	return out, nil
}

// FileHash returns the SHA-256 of a file, as attachments record it.
func FileHash(p string) (string, error) {
	sum, _, err := hashFile(p)
	return sum, err
}

func hashFile(p string) (string, int64, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return "", 0, err
	}
	if st.IsDir() {
		return "", 0, fmt.Errorf("%s is a folder; attach files", p)
	}
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// messageSelect reads a message with the sender's and the recipient's
// titles and workspaces as they are now, for its heading (scanMessage).
const messageSelect = `SELECT m.id, m.created_at, m.kind, m.from_session, m.from_name,
	COALESCE(fw.name, '') AS from_ws_name, COALESCE(CASE WHEN fw.label <> '' THEN fw.label ELSE fw.name END, '') AS from_ws_label,
	m.to_label, m.room, m.reply_to, m.text, m.version, m.verdict,
	fs.id IS NOT NULL AS from_known, COALESCE(fs.title, '') AS from_title,
	m.to_kind, ts.id IS NOT NULL AS to_known, COALESCE(ts.title, '') AS to_title,
	COALESCE(tw.name, '') AS to_ws_name, COALESCE(CASE WHEN tw.label <> '' THEN tw.label ELSE tw.name END, '') AS to_ws_label,
	COALESCE(CASE WHEN r.title <> '' THEN r.title ELSE r.name END, '') AS room_title, m.mentions, m.relay_lead
	FROM messages m
	LEFT JOIN rooms r ON m.room <> '' AND r.name = m.room
	LEFT JOIN workspaces fw ON fw.key = m.from_workspace
	LEFT JOIN sessions fs ON m.from_session <> '' AND fs.id = m.from_session
	LEFT JOIN sessions ts ON m.to_kind = 'session' AND ts.id = m.to_key
	LEFT JOIN workspaces tw ON tw.key = CASE WHEN m.to_kind = 'session' THEN ts.workspace
		WHEN m.to_kind IN ('workspace', 'subscribers') THEN m.to_key END`

func scanMessage(r scanner) (Message, error) {
	var m Message
	var fromKnown, toKnown bool
	var fromTitle, toKind, toTitle, toWS, toWSLabel string
	err := r.Scan(&m.ID, &m.CreatedAt, &m.Kind, &m.FromSession, &m.FromName, &m.FromWorkspace, &m.FromWorkspaceLabel,
		&m.ToLabel, &m.Room, &m.ReplyTo, &m.Text, &m.Version, &m.Verdict,
		&fromKnown, &fromTitle, &toKind, &toKnown, &toTitle, &toWS, &toWSLabel, &m.RoomTitle, &m.mentions, &m.RelayLead)
	m.ForPerson = m.RelayLead != ""
	m.From = m.FromName
	if fromKnown {
		m.From = DisplayTitle(fromTitle, m.FromWorkspaceLabel)
	}
	if m.Room != "" && m.RoomTitle == "" {
		m.RoomTitle = m.Room
	}
	m.To, m.ToPerson = m.ToLabel, m.ToLabel
	switch {
	case toKind == "room" && m.Room != "":
		m.To, m.ToPerson = "#"+m.RoomTitle, m.RoomTitle
	case toKind == "session" && toKnown:
		m.To = DisplayTitle(toTitle, toWSLabel)
		m.ToPerson = m.To
	case toKind == "workspace" && toWS != "":
		m.To, m.ToPerson = toWS, toWSLabel
	case toKind == "subscribers" && toWS != "":
		m.To, m.ToPerson = "subscribers of "+toWS, "subscribers of "+toWSLabel
	}
	return m, err
}

// Message returns one message with its attachments.
func (b *Bus) Message(ctx context.Context, id int64) (Message, error) {
	m, err := scanMessage(b.db.QueryRowContext(ctx, messageSelect+` WHERE m.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return m, fmt.Errorf("message %d: %w", id, ErrNotFound)
	}
	if err != nil {
		return m, err
	}
	ms := []Message{m}
	if err := b.attach(ctx, ms); err != nil {
		return m, err
	}
	return ms[0], nil
}

func (b *Bus) queryMessages(ctx context.Context, query string, args ...any) ([]Message, error) {
	rows, err := b.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	var out []Message
	for rows.Next() {
		m, err := scanMessage(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, b.attach(ctx, out)
}

func (b *Bus) attach(ctx context.Context, ms []Message) error {
	for i := range ms {
		if ms[i].RelayLead != "" {
			var title, workspace string
			if b.db.QueryRowContext(ctx, `SELECT s.title, COALESCE(w.label, '') FROM sessions s LEFT JOIN workspaces w ON w.key = s.workspace WHERE s.id = ?`, ms[i].RelayLead).Scan(&title, &workspace) == nil {
				ms[i].RelayLeadTitle = DisplayTitle(title, workspace)
			}
		}
		if err := b.mentioned(ctx, &ms[i]); err != nil {
			return err
		}
		rows, err := b.db.QueryContext(ctx, `SELECT idx, path, rel, sha256, size, mime, stored, removed_at <> '' FROM attachments
			WHERE message_id = ? ORDER BY idx`, ms[i].ID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var a Attachment
			if err := rows.Scan(&a.Index, &a.Path, &a.Rel, &a.SHA256, &a.Size, &a.Mime, &a.Stored, &a.Removed); err != nil {
				rows.Close()
				return err
			}
			ms[i].Attachments = append(ms[i].Attachments, a)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
	}
	return nil
}

// Deliver claims the messages waiting for a session, either addressed to it
// or to its workspace, and marks them delivered to it. event is the hook
// event that asks (SessionStart, UserPromptSubmit, PostToolUse or Stop), or
// "tool" when the agent asks through its inbox tool. For Stop, block reports
// whether the turn should continue so the agent can read them. The bus counts
// those continuations itself: the count resets when the person submits a
// prompt or a turn ends with nothing new, and after MaxStopContinues in a row
// further messages wait for the person's next prompt.
func (b *Bus) Deliver(ctx context.Context, sessionID, event string) (msgs []Message, block bool, err error) {
	var ids []int64
	trusted, asked, held := map[int64]bool{}, map[int64]bool{}, map[int64]bool{}
	err = b.tx(ctx, func(t *sql.Tx) error {
		var ws string
		var continues, hops int
		err := t.QueryRow(`SELECT workspace, stop_continues, hops FROM sessions WHERE id = ?`, sessionID).Scan(&ws, &continues, &hops)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("session %s: %w", sessionID, ErrNotFound)
		}
		if err != nil {
			return err
		}
		now := b.stamp()
		if _, err := t.Exec(`UPDATE sessions SET last_seen = ?, ended_at = NULL WHERE id = ?`, now, sessionID); err != nil {
			return err
		}
		switch event {
		case "UserPromptSubmit", DoorbellTurn:
			continues = 0
		case "Stop":
			if continues >= MaxStopContinues {
				return nil
			}
		}
		// A room message that asks nobody here is background: it goes with
		// the session's turns but never keeps a turn going at Stop.
		quiet := ""
		if event == "Stop" {
			quiet = " AND " + wakesSQL + " AND " + budgetOpenSQL
		}
		// A question is taken by the invocation that asked it, never handed
		// over here (questions.go).
		// A group's message does not advance the hop count (rooms.go).
		rows, err := t.Query(`SELECT d.id, d.message_id, CASE WHEN m.room = '' THEN m.hops ELSE 0 END, m.from_session = '' AND m.from_name = 'paul', `+trustedSQL("m.from_workspace", "?")+`, `+askedSQL+`, `+wakesSQL+`, m.room, m.from_session <> '' AND m.kind = 'text' FROM deliveries d
			JOIN messages m ON m.id = d.message_id JOIN sessions s ON s.id = ? WHERE d.delivered_at IS NULL AND m.kind <> '`+QuestionKind+`' AND
			((d.recipient_kind = 'session' AND d.recipient = s.id) OR (d.recipient_kind = 'workspace' AND d.recipient = ? AND m.from_session <> s.id))`+quiet+`
			ORDER BY d.message_id`, ws, ws, sessionID, ws)
		if err != nil {
			return err
		}
		var deliveryIDs []int64
		// The session's next message is one hop past the furthest message it
		// read, unless Paul spoke, which starts the count again.
		furthest, fromPaul := 0, false
		// The agents' requests handed over, by group (rooms.go).
		requests, roomOrder := map[string][]int64{}, []string{}
		for rows.Next() {
			var d, m int64
			var h int
			var human, trust, ask, wakes, fromAgent bool
			var room string
			if err := rows.Scan(&d, &m, &h, &human, &trust, &ask, &wakes, &room, &fromAgent); err != nil {
				rows.Close()
				return err
			}
			deliveryIDs = append(deliveryIDs, d)
			ids = append(ids, m)
			trusted[m] = trust
			asked[m] = ask
			if room != "" && wakes && fromAgent {
				if _, seen := requests[room]; !seen {
					roomOrder = append(roomOrder, room)
				}
				requests[room] = append(requests[room], m)
			}
			furthest = max(furthest, h)
			fromPaul = fromPaul || human
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		// Handing a chat the messages from agents in a group that ask it to
		// answer spends one turn of that group's budget, however many
		// messages there are and whether they start a turn or steer a
		// running one. Once the budget is used up they arrive as background.
		for _, room := range roomOrder {
			var budget, used int
			if err := t.QueryRow(`SELECT turn_budget, turns_used FROM rooms WHERE name = ?`, room).Scan(&budget, &used); err != nil {
				return err
			}
			switch {
			case budget < 0:
			case used < budget:
				if _, err := t.Exec(`UPDATE rooms SET turns_used = turns_used + 1 WHERE name = ?`, room); err != nil {
					return err
				}
			default:
				for _, m := range requests[room] {
					held[m] = true
				}
			}
		}
		for _, d := range deliveryIDs {
			if _, err := t.Exec(`UPDATE deliveries SET delivered_at = ?, delivered_session = ? WHERE id = ?`, now, sessionID, d); err != nil {
				return err
			}
		}
		if event == "Stop" {
			if len(ids) > 0 {
				block = true
				continues++
				// The agent carries on past this stop, in the turn it was in. If the window's prompt for that turn has
				// already come back, and ended its hook turn, the agent is still working in it, so the turn is open
				// again for steers (steers.go).
				if _, err := t.Exec(`UPDATE hook_turns SET active = 1, ended_at = '' WHERE session = ? AND active = 0`, sessionID); err != nil {
					return err
				}
			} else {
				continues = 0
			}
		}
		switch {
		case event == "UserPromptSubmit" || fromPaul:
			hops = 0
		case len(ids) > 0:
			hops = furthest
		}
		_, err = t.Exec(`UPDATE sessions SET stop_continues = ?, hops = ? WHERE id = ?`, continues, hops, sessionID)
		return err
	})
	if err != nil || len(ids) == 0 {
		return nil, false, err
	}
	for _, id := range ids {
		m, err := b.Message(ctx, id)
		if err != nil {
			return nil, false, err
		}
		m.Trusted = trusted[id]
		m.Relay = m.RelayLead == sessionID
		m.Held = held[id] && asked[id]
		m.Asked = m.Kind != ResultKind && (m.Room == "" || asked[id]) && !m.Held
		m.Notice = m.Room != "" && m.Kind == "text" && !asked[id]
		msgs = append(msgs, m)
	}
	return msgs, block, nil
}

// AttachmentContent is an attachment as it is now. Data is its bytes and
// Text the same bytes as a string. Missing is set when the file no longer
// exists, and Removed (in Attachment) when the person removed the bus's
// copy of a file they attached.
type AttachmentContent struct {
	Attachment
	Text          string
	Data          []byte
	Missing       bool
	Changed       bool
	CurrentSHA256 string
}

// mayRead reports whether a session may read a message's attachments: the
// session that sent it, and the chats the message reached. A message reaches
// a chat when the bus made a delivery of it to that chat, as it does to each
// member chat of a group for a group's message, or to the chat's workspace.
// The person's messages follow the same rule, so the chats a person's
// message reached read its files, and no other chat does.
func (b *Bus) mayRead(ctx context.Context, s Session, m Message) bool {
	if m.FromSession != "" && m.FromSession == s.ID {
		return true
	}
	var one int
	return b.db.QueryRowContext(ctx, `SELECT 1 FROM deliveries WHERE message_id = ? AND
		((recipient_kind = 'session' AND recipient = ?) OR (recipient_kind = 'workspace' AND recipient = ?))`,
		m.ID, s.ID, s.Workspace).Scan(&one) == nil
}

// ReadAttachment returns an attachment's current content to a session that
// may read it (mayRead).
func (b *Bus) ReadAttachment(ctx context.Context, sessionID string, messageID int64, index int) (AttachmentContent, error) {
	s, err := b.Session(ctx, sessionID)
	if err != nil {
		return AttachmentContent{}, err
	}
	m, err := b.Message(ctx, messageID)
	if err != nil {
		return AttachmentContent{}, err
	}
	if !b.mayRead(ctx, s, m) {
		return AttachmentContent{}, fmt.Errorf("message %d was not sent to this session or its workspace", messageID)
	}
	var a *Attachment
	for i := range m.Attachments {
		if m.Attachments[i].Index == index {
			a = &m.Attachments[i]
		}
	}
	if a == nil {
		return AttachmentContent{}, fmt.Errorf("message %d has no attachment %d", messageID, index)
	}
	out := AttachmentContent{Attachment: *a}
	if a.Removed {
		return out, nil
	}
	sum, size, err := hashFile(a.Path)
	if errors.Is(err, os.ErrNotExist) {
		out.Missing = true
		return out, nil
	}
	if err != nil {
		return out, err
	}
	out.CurrentSHA256 = sum
	out.Changed = sum != a.SHA256
	if size > MaxAttachmentRead {
		return out, fmt.Errorf("%s is %d bytes, more than the %d this tool returns", a.Rel, size, MaxAttachmentRead)
	}
	data, err := os.ReadFile(a.Path)
	if err != nil {
		return out, err
	}
	out.Data, out.Text = data, string(data)
	return out, nil
}

// Subscribe records that a session's workspace depends on a library
// workspace, so it receives the library's release notices.
func (b *Bus) Subscribe(ctx context.Context, s Session, library string) error {
	return b.tx(ctx, func(t *sql.Tx) error {
		var key string
		if err := t.QueryRow(`SELECT key FROM workspaces WHERE name = ?`, strings.ToLower(strings.TrimSpace(library))).Scan(&key); err != nil {
			return fmt.Errorf("no workspace is called %q; a library's workspace joins the bus when a session starts in it", library)
		}
		if key == s.Workspace {
			return errors.New("a workspace cannot subscribe to itself")
		}
		_, err := t.Exec(`INSERT OR IGNORE INTO subscriptions (consumer, library) VALUES (?, ?)`, s.Workspace, key)
		return err
	})
}

// AnnounceRelease sends a release notice from a session's workspace to every
// workspace subscribed to it, and to the person.
func (b *Bus) AnnounceRelease(ctx context.Context, s Session, version, notes string) (Message, int, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return Message{}, 0, errors.New("a release needs a version")
	}
	var id int64
	var n int
	err := b.tx(ctx, func(t *sql.Tx) error {
		rows, err := t.Query(`SELECT consumer FROM subscriptions WHERE library = ?`, s.Workspace)
		if err != nil {
			return err
		}
		r := resolved{label: "subscribers of " + s.WorkspaceName, toKind: "subscribers", toKey: s.Workspace, recipients: []recipient{{"human", Human}}}
		for rows.Next() {
			var c string
			if err := rows.Scan(&c); err != nil {
				rows.Close()
				return err
			}
			r.recipients = append(r.recipients, recipient{"workspace", c})
			n++
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		id, err = b.insertMessage(t, Sender{Session: &s}, "release", r, 0, notes, version, "", nil)
		return err
	})
	if err != nil {
		return Message{}, 0, err
	}
	m, err := b.Message(ctx, id)
	return m, n, err
}

// GiveVerdict answers a release notice for a session's workspace.
func (b *Bus) GiveVerdict(ctx context.Context, s Session, releaseID int64, verdict, reason string) (Message, error) {
	if _, ok := Verdicts[verdict]; !ok {
		return Message{}, fmt.Errorf("a verdict is upgrade, sit_out or blocked, not %q", verdict)
	}
	if verdict != "upgrade" && strings.TrimSpace(reason) == "" {
		return Message{}, errors.New("say why: sit_out and blocked need a reason")
	}
	var id int64
	err := b.tx(ctx, func(t *sql.Tx) error {
		var kind, libWS, version string
		err := t.QueryRow(`SELECT kind, from_workspace, version FROM messages WHERE id = ?`, releaseID).Scan(&kind, &libWS, &version)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && kind != "release") {
			return fmt.Errorf("message %d is not a release notice", releaseID)
		}
		if err != nil {
			return err
		}
		var libName string
		if err := t.QueryRow(`SELECT name FROM workspaces WHERE key = ?`, libWS).Scan(&libName); err != nil {
			return err
		}
		r := workspaceResolved(libWS, libName)
		r.recipients = append(r.recipients, recipient{"human", Human})
		id, err = b.insertMessage(t, Sender{Session: &s}, "verdict", r, releaseID, reason, version, verdict, nil)
		return err
	})
	if err != nil {
		return Message{}, err
	}
	return b.Message(ctx, id)
}

// Release is a release notice with the latest verdict from each workspace.
type Release struct {
	Message
	Verdicts []Message
	Waiting  []string // subscribed workspaces that have not answered, by label
}

// Releases returns recent release notices with their verdicts.
func (b *Bus) Releases(ctx context.Context, limit int) ([]Release, error) {
	ms, err := b.queryMessages(ctx, messageSelect+` WHERE m.kind = 'release' ORDER BY m.id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	out := make([]Release, len(ms))
	for i, m := range ms {
		out[i], err = b.release(ctx, m)
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// ReleaseByID returns one release notice with its verdicts.
func (b *Bus) ReleaseByID(ctx context.Context, id int64) (Release, error) {
	m, err := b.Message(ctx, id)
	if err != nil {
		return Release{}, err
	}
	if m.Kind != "release" {
		return Release{}, fmt.Errorf("message %d: %w", id, ErrNotFound)
	}
	return b.release(ctx, m)
}

func (b *Bus) release(ctx context.Context, m Message) (Release, error) {
	r := Release{Message: m}
	vs, err := b.queryMessages(ctx, messageSelect+` WHERE m.kind = 'verdict' AND m.reply_to = ?
		AND m.id = (SELECT MAX(v.id) FROM messages v WHERE v.kind = 'verdict' AND v.reply_to = m.reply_to AND v.from_workspace = m.from_workspace)
		ORDER BY m.id`, m.ID)
	if err != nil {
		return r, err
	}
	r.Verdicts = vs
	// A subscribed workspace has the notice for its next session, or one
	// copy for each session that was open (fanOut).
	rows, err := b.db.QueryContext(ctx, `SELECT DISTINCT CASE WHEN w.label <> '' THEN w.label ELSE w.name END FROM deliveries d
		LEFT JOIN sessions s ON d.recipient_kind = 'session' AND s.id = d.recipient
		JOIN workspaces w ON w.key = CASE d.recipient_kind WHEN 'workspace' THEN d.recipient ELSE s.workspace END
		WHERE d.message_id = ? AND d.recipient_kind IN ('workspace', 'session') AND NOT EXISTS (
			SELECT 1 FROM messages v WHERE v.kind = 'verdict' AND v.reply_to = d.message_id AND v.from_workspace = w.key)
		ORDER BY 1`, m.ID)
	if err != nil {
		return r, err
	}
	defer rows.Close()
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return r, err
		}
		r.Waiting = append(r.Waiting, n)
	}
	return r, rows.Err()
}

// HumanInbox returns the messages sent to the person, newest first, and
// marks them read when markRead is set.
func (b *Bus) HumanInbox(ctx context.Context, limit int, markRead bool) ([]Message, error) {
	rows, err := b.db.QueryContext(ctx, `SELECT d.message_id, d.delivered_at IS NULL FROM deliveries d
		WHERE d.recipient_kind = 'human' ORDER BY d.message_id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	type entry struct {
		id     int64
		unread bool
	}
	var es []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.id, &e.unread); err != nil {
			rows.Close()
			return nil, err
		}
		es = append(es, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var out []Message
	unread := false
	for _, e := range es {
		m, err := b.Message(ctx, e.id)
		if err != nil {
			return nil, err
		}
		m.Unread = e.unread
		unread = unread || e.unread
		out = append(out, m)
	}
	// Writing only when something is unread keeps the change counter still,
	// so a page that marks its messages read does not make itself reload.
	if markRead && unread {
		err = b.tx(ctx, func(t *sql.Tx) error {
			_, err := t.Exec(`UPDATE deliveries SET delivered_at = ?, delivered_session = '' WHERE recipient_kind = 'human' AND delivered_at IS NULL`, b.stamp())
			return err
		})
	}
	return out, err
}

// Recent returns the latest messages on the whole bus, newest first.
func (b *Bus) Recent(ctx context.Context, limit int) ([]Message, error) {
	return b.queryMessages(ctx, messageSelect+` ORDER BY m.id DESC LIMIT ?`, limit)
}

// Thread returns a message and every reply to it, oldest first.
func (b *Bus) Thread(ctx context.Context, id int64) ([]Message, error) {
	root, err := b.Message(ctx, id)
	if err != nil {
		return nil, err
	}
	replies, err := b.queryMessages(ctx, messageSelect+` WHERE m.reply_to = ? ORDER BY m.id`, id)
	if err != nil {
		return nil, err
	}
	return append([]Message{root}, replies...), nil
}

// Pending reports how many deliveries to a recipient are still waiting.
type Pending struct {
	Label string
	Count int
}

// Doorbell returns how many undelivered messages within the hop limit are
// waiting for a session, directly or through its workspace, and the newest
// one's number. It changes nothing. The limit is MaxHops, or the trusted
// link's own between the sender's workspace and the session's (links.go).
// A group's message has no hop limit and counts while its group's turn budget
// is not used up (rooms.go). A group's message that does not mention the
// session counts too, for its notice (notice.go).
// A question is not counted, since Deliver does not hand it over
// (questions.go).
func (b *Bus) Doorbell(ctx context.Context, sessionID string) (count int, newest int64, err error) {
	err = b.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(MAX(d.message_id), 0) `+doorbellFrom, sessionID).Scan(&count, &newest)
	return count, newest, err
}

// doorbellFrom is the rest of the query that finds the undelivered messages
// the doorbell counts for a session, bound to the session's ID.
var doorbellFrom = doorbellFor(wakesSQL)

// askingFrom is doorbellFrom for the deliveries that ask the session to
// answer, which leaves out a group's messages that only give it a notice.
var askingFrom = doorbellFor(askedSQL)

func doorbellFor(wake string) string {
	return `FROM deliveries d
		JOIN messages m ON m.id = d.message_id
		JOIN sessions s ON s.id = ?
		WHERE d.delivered_at IS NULL AND m.kind <> '` + QuestionKind + `' AND (m.room <> '' OR ` + hopLimitSQL("m.from_workspace", "s.workspace") + ` = ` + fmt.Sprint(NoHopLimit) + ` OR m.hops <= ` + hopLimitSQL("m.from_workspace", "s.workspace") + `) AND ` + budgetOpenSQL + ` AND ` + wake + ` AND
			((d.recipient_kind = 'session' AND d.recipient = s.id) OR (d.recipient_kind = 'workspace' AND d.recipient = s.workspace AND m.from_session <> s.id))`
}

// Asking returns how many of the messages the doorbell counts for a session
// ask it to answer, which is Doorbell without a group's notices. The window
// steers a busy chat only with these: a notice waits for the chat's next turn.
func (b *Bus) Asking(ctx context.Context, sessionID string) (int, error) {
	var n int
	err := b.db.QueryRowContext(ctx, `SELECT COUNT(*) `+askingFrom, sessionID).Scan(&n)
	return n, err
}

// Arriving returns the first limit of the messages the doorbell counts for a
// session, oldest first, and how many more there are. It changes nothing: the
// messages stay undelivered. A chat that is still replaying its saved
// conversation shows them as arriving until it can take them.
func (b *Bus) Arriving(ctx context.Context, sessionID string, limit int) (msgs []Message, more int, err error) {
	rows, err := b.db.QueryContext(ctx, `SELECT DISTINCT d.message_id `+doorbellFrom+` ORDER BY d.message_id`, sessionID)
	if err != nil {
		return nil, 0, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(ids) > limit {
		ids, more = ids[:limit], len(ids)-limit
	}
	for _, id := range ids {
		m, err := b.Message(ctx, id)
		if err != nil {
			return nil, 0, err
		}
		msgs = append(msgs, m)
	}
	return msgs, more, nil
}

// GroupChat is a chat that belongs to a group: its session and its workspace's
// folder.
type GroupChat struct{ Session, WorkspacePath string }

// GroupChats returns the member chats of the groups that have had a message
// since the time given, the chat of the most recently active group first and
// each chat once. The window opens them in the background when it starts
// (docs/plans/group-reopen.md).
func (b *Bus) GroupChats(ctx context.Context, since time.Time) ([]GroupChat, error) {
	rows, err := b.db.QueryContext(ctx, `SELECT m.member, w.path FROM room_members m
		JOIN sessions s ON s.id = m.member JOIN workspaces w ON w.key = s.workspace
		JOIN messages x ON x.room = m.room AND x.kind = 'text' AND x.created_at >= ?
		WHERE m.member_kind = 'session'
		GROUP BY m.member, w.path ORDER BY MAX(x.created_at) DESC, MIN(m.rowid)`, since.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GroupChat
	for rows.Next() {
		var c GroupChat
		if err := rows.Scan(&c.Session, &c.WorkspacePath); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Wake is a chat with messages waiting that would wake it: its session, its
// workspace folder, and the newest such message's number.
type Wake struct {
	Session       string
	WorkspacePath string
	Newest        int64
}

// Waking returns every chat with undelivered messages within the hop limit
// that ask it to answer: a message sent to the chat itself, and a group's
// message that mentions it by name. It changes nothing. Unlike Doorbell it
// leaves out a message for the chat's workspace, which no single chat is
// asked to answer, and the mentions of everyone or of a workspace that
// messages from before groups were made of chats may keep, because those
// were meant for the chats open then (decision 6 of docs/plans/
// group-chats.md). A question another chat asks counts whatever its hops
// (questions.go). The window opens a closed chat for the chats this returns
// (bus_wake.go).
func (b *Bus) Waking(ctx context.Context) ([]Wake, error) {
	limit := hopLimitSQL("m.from_workspace", "s.workspace")
	rows, err := b.db.QueryContext(ctx, `SELECT s.id, w.path, MAX(d.message_id) FROM deliveries d
		JOIN messages m ON m.id = d.message_id
		JOIN sessions s ON d.recipient_kind = 'session' AND s.id = d.recipient
		JOIN workspaces w ON w.key = s.workspace
		WHERE d.delivered_at IS NULL AND m.kind <> '`+MemberKind+`' AND (m.kind = '`+QuestionKind+`' OR (`+budgetOpenSQL+` AND (m.room <> '' OR `+limit+` = `+fmt.Sprint(NoHopLimit)+` OR m.hops <= `+limit+`))) AND
			(m.room = '' OR m.mentions LIKE '%,' || s.id || ',%' OR m.mentions LIKE '%,*,%')
		GROUP BY s.id, w.path ORDER BY MIN(d.message_id)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Wake
	for rows.Next() {
		var w Wake
		if err := rows.Scan(&w.Session, &w.WorkspacePath, &w.Newest); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// Waiting returns the undelivered messages per workspace or chat, by label
// and title, so the page can show what is queued for windows that have not
// taken a turn.
func (b *Bus) Waiting(ctx context.Context) ([]Pending, error) {
	rows, err := b.db.QueryContext(ctx, `SELECT d.recipient_kind, d.recipient, COALESCE(s.title, ''),
			COALESCE(CASE WHEN w.label <> '' THEN w.label ELSE w.name END, ''), COUNT(*)
		FROM deliveries d
		LEFT JOIN sessions s ON d.recipient_kind = 'session' AND s.id = d.recipient
		LEFT JOIN workspaces w ON w.key = CASE d.recipient_kind WHEN 'session' THEN s.workspace WHEN 'workspace' THEN d.recipient END
		WHERE d.delivered_at IS NULL AND d.recipient_kind NOT IN ('human', '`+humanRoom+`')
		GROUP BY d.recipient_kind, d.recipient`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Pending
	for rows.Next() {
		var p Pending
		var kind, recipient, title, label string
		if err := rows.Scan(&kind, &recipient, &title, &label, &p.Count); err != nil {
			return nil, err
		}
		switch {
		case kind == "session" && label != "":
			p.Label = DisplayTitle(title, label)
		case label != "":
			p.Label = label
		default:
			p.Label = recipient
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Label < out[j].Label })
	return out, rows.Err()
}
