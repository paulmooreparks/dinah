package bus

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// What the code calls a room is shown as a group chat (docs/plans/
// group-chats.md). A group's members are chats, by their sessions on the
// bus, and the person, who is always a member. A group has a key, which is
// its row's name and its window's address, and a title, which is what
// people and agents call it.
//
// A message in a group goes to every member chat except its sender. It
// wakes only the chats it mentions, and every other member reads it at the
// start of its next turn, so a busy group does not start a turn in every
// chat at every message. The mentions are a list the message keeps, given
// by its sender, since a title with spaces cannot be found reliably in the
// text. Everyone wakes every member, closed ones too. A member a message does
// not mention gets a notice of it (notice.go). A mention of the
// person also puts the message in the person's inbox, and an agent may
// mention only the group's members. A member that is closed keeps the
// message until it opens, and a message that wakes it is one of those
// Waking returns, for which the window opens the chat in the background.

// Everyone is the mention that wakes every member of a group, closed ones
// too. It is reserved: no chat can take it as its title
// (CleanTitle).
const Everyone = "everyone"

// reservedMention is true for a folded word that mentions every member.
func reservedMention(folded string) bool {
	return folded == Everyone
}

// WakesAll is the mark in a message's mentions that wakes every member of the
// group, closed ones too. A message that mentions everyone carries it. It is
// not a mention, and a message shows only the chats it names.
const WakesAll = "*"

// MemberKind is the kind of the line a group keeps when a chat joins or
// leaves it. Its text is added, removed, joined or left, its sender is who
// did it and its recipient the chat. It is delivered to nobody.
const MemberKind = "member"

// Mention is a chat or the person a group's message mentions. Key is the
// chat's session on the bus, or Human; Title is how a person reads it and
// Address how an agent writes it.
type Mention struct {
	Key     string
	Title   string
	Address string
}

// RoomMember is a group's member as its window lists it: a chat, by its
// title as it is now and its workspace, or the person, whose Key and Title
// are Human.
type RoomMember struct {
	Kind          string // session or human
	Key           string
	Title         string
	Workspace     string // the workspace's label
	WorkspaceName string
	Address       string
	Open          bool
}

// Room is a group chat. Name is its key and Title its name; LastAt is when
// its latest message was sent, and Unread how many of its messages the
// person has not yet seen in its window.
type Room struct {
	Name    string
	Title   string
	Members []RoomMember
	LastAt  string
	Unread  int
	// NewestID, NewestFrom and NewestLine are the number of the newest message the
	// person has not seen, who sent it and its notice line (NoticeLine), for the
	// window's unread marker. They are empty when nothing is unread.
	NewestID   int64
	NewestFrom string
	NewestLine string
	// Budget is the number of turns agents may start in the group between
	// the person's posts, or NoTurnBudget, and Used how many they have
	// started since the person last posted.
	Budget int
	Used   int
	// Lead is the chat whose team's channel the group is, or empty for an ordinary group (teams.go).
	Lead string
}

// NoTurnBudget is the turn budget of a group the person has set no limit on,
// which is what every group starts with.
const NoTurnBudget = -1

// budgetOpenSQL is true for a group's message that may still ask a chat to
// answer: the person's, which is never held, or any message of a group whose
// budget is off or not yet used up. It is meant for a message m outside a
// group too, where it is true.
const budgetOpenSQL = `(m.room = '' OR m.from_session = '' OR COALESCE((SELECT r.turn_budget < 0 OR r.turns_used < r.turn_budget FROM rooms r WHERE r.name = m.room), 1))`

// SetGroupBudget sets how many turns agents may start in a group between the
// person's posts, or NoTurnBudget for no limit, and refills it. Only the
// person sets it; no agent tool does.
func (b *Bus) SetGroupBudget(ctx context.Context, group string, budget int) error {
	if budget < NoTurnBudget {
		return errors.New("a turn budget is a count of turns, 0 or more, or -1 for no limit")
	}
	return b.tx(ctx, func(t *sql.Tx) error {
		key, _, err := findGroup(t, group)
		if err != nil {
			return err
		}
		_, err = t.Exec(`UPDATE rooms SET turn_budget = ?, turns_used = 0 WHERE name = ?`, budget, key)
		return err
	})
}

// askedSQL is true for a delivery that asks its session s to answer: any
// message that is not a group's, and a group's that mentions the session.
// Messages sent before groups were made of chats may also mention everyone
// or the session's workspace.
const askedSQL = `(m.room = '' OR m.mentions LIKE '%,' || s.id || ',%' OR m.mentions LIKE '%,everyone,%' OR m.mentions LIKE '%,*,%' OR m.mentions LIKE '%,' || s.workspace || ',%')`

// wakesSQL admits action requests only. Informational group deliveries remain
// unread until the recipient takes a turn for another reason.
const wakesSQL = askedSQL

// fanOut turns the workspaces a message is for into deliveries: one to each
// open session of the workspace other than the sender, or to the workspace
// itself, for its next session, when it has no other session open. Sessions
// and the person stay as they are.
func fanOut(t *sql.Tx, members []recipient, sender string) ([]recipient, error) {
	var out []recipient
	for _, m := range members {
		if m.kind != "workspace" {
			out = append(out, m)
			continue
		}
		rows, err := t.Query(`SELECT id FROM sessions WHERE workspace = ? AND ended_at IS NULL AND id <> ?`, m.value, sender)
		if err != nil {
			return nil, err
		}
		var live []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return nil, err
			}
			live = append(live, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		if len(live) == 0 {
			out = append(out, m)
			continue
		}
		for _, id := range live {
			out = append(out, recipient{"session", id})
		}
	}
	return out, nil
}

// querier is a transaction or the database, for a read that may happen in
// either.
type querier interface {
	QueryRow(query string, args ...any) *sql.Row
	Query(query string, args ...any) (*sql.Rows, error)
}

// missing is an error that says what is missing and counts as ErrNotFound.
type missing string

func (e missing) Error() string        { return string(e) }
func (e missing) Is(target error) bool { return target == ErrNotFound }

// findGroup finds a group by its key or its title, with or without #.
func findGroup(t querier, address string) (key, title string, err error) {
	a := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(address), "#"))
	if a == "" {
		return "", "", errors.New("say which group chat")
	}
	if t.QueryRow(`SELECT name, title FROM rooms WHERE name = ?`, strings.ToLower(a)).Scan(&key, &title) == nil {
		return key, groupTitle(key, title), nil
	}
	rows, err := t.Query(`SELECT name, title FROM rooms`)
	if err != nil {
		return "", "", err
	}
	defer rows.Close()
	for rows.Next() {
		var k, tt string
		if err := rows.Scan(&k, &tt); err != nil {
			return "", "", err
		}
		if fold(tt) == fold(a) {
			return k, groupTitle(k, tt), nil
		}
	}
	if err := rows.Err(); err != nil {
		return "", "", err
	}
	// A group that was called this once is now called something else, which the
	// refusal says, so a chat with stale instructions corrects itself.
	if old, err := t.Query(`SELECT room, old_title FROM room_renames ORDER BY rowid DESC`); err == nil {
		type renamed struct{ room, title string }
		var found *renamed
		for old.Next() {
			var r renamed
			if old.Scan(&r.room, &r.title) == nil && fold(r.title) == fold(a) && found == nil {
				found = &r
			}
		}
		old.Close()
		if found != nil {
			var title string
			if t.QueryRow(`SELECT title FROM rooms WHERE name = ?`, found.room).Scan(&title) == nil {
				now := groupTitle(found.room, title)
				return "", "", missing(fmt.Sprintf("the group chat %q is now called %q; send to #%s", a, now, now))
			}
		}
	}
	return "", "", missing(fmt.Sprintf("there is no group chat called %q", a))
}

func groupTitle(key, title string) string {
	if title != "" {
		return title
	}
	return key
}

// groupResolved is where a message to a group goes: to each member chat but
// its sender and, for an agent's message, to the person's unread count in
// the group's window. A chat that is not a member cannot write to the group.
func groupResolved(t *sql.Tx, from Sender, address string) (resolved, error) {
	key, title, err := findGroup(t, address)
	if err != nil {
		return resolved{}, err
	}
	r := resolved{label: "#" + title, room: key, roomTitle: title, toKind: "room", toKey: key}
	rows, err := t.Query(`SELECT member_kind, member FROM room_members WHERE room = ? ORDER BY rowid`, key)
	if err != nil {
		return resolved{}, err
	}
	defer rows.Close()
	retired, err := retiredMembers(t)
	if err != nil {
		return resolved{}, err
	}
	member := from.Session == nil
	for rows.Next() {
		var kind, value string
		if err := rows.Scan(&kind, &value); err != nil {
			return resolved{}, err
		}
		switch {
		case kind == "session" && from.Session != nil && value == from.Session.ID:
			member = true
		case kind == "session" && retired[value]:
			// A retired team member is not woken or told of the group's messages.
		case kind == "session":
			r.recipients = append(r.recipients, recipient{"session", value})
		case kind == "human" && from.Session != nil:
			r.recipients = append(r.recipients, recipient{humanRoom, Human})
		}
	}
	if err := rows.Err(); err != nil {
		return resolved{}, err
	}
	if !member {
		return resolved{}, fmt.Errorf("this chat is not in the group chat %q; join it with room_join before writing to it", title)
	}
	return r, nil
}

// groupChats are a group's member chats, in the order they joined.
func groupChats(t *sql.Tx, room string) ([]chat, error) {
	return scanChats(t.Query(chatSelect+` JOIN room_members m ON m.member_kind = 'session' AND m.member = s.id
		WHERE m.room = ? ORDER BY m.rowid`, room))
}

// matchMember finds the member chat a mention is for: by its session ID,
// or by its title as an address finds a chat (matchChat).
func matchMember(members []chat, mention string) (chat, error) {
	for _, c := range members {
		if c.id == mention {
			return c, nil
		}
	}
	return matchChat(members, mention)
}

func addressList(chats []chat) string {
	var names []string
	for _, c := range chats {
		names = append(names, c.address())
	}
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

// mentionList checks the mentions a message is sent with and records them
// on r. Each is a member chat by its session ID or its title, with an
// optional @; Everyone, which wakes every member, closed ones too; or the
// person, by the handle or by the name agents are told (person), which also
// puts the message in the person's inbox. A mention of a chat that is not
// a member is refused with the reason, as is any mention in a message that
// is not a group's. The sender's mention of itself is dropped.
func mentionList(t *sql.Tx, from Sender, r *resolved, mentions []string, person string) error {
	var list []string
	for _, raw := range mentions {
		if m := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "@")); m != "" {
			list = append(list, m)
		}
	}
	// A message wakes only the chats it mentions, whoever wrote it. Everyone
	// wakes every member, open or closed, and is kept as the mark WakesAll
	// beside the mention, so the message shows "everyone" and not each member.
	// The other members get a notice (notice.go).
	wakeAll := false
	if len(list) == 0 && from.Session == nil && r.room != "" {
		// The person's message in a team's group, with no mention, is a request to the team's lead
		// (docs/plans/teams.md); a mention of a particular chat goes to that chat as ever. A lead that
		// is no longer in the group, or is unknown, leaves the message as an ordinary unmentioned one.
		var lead string
		if err := t.QueryRow(`SELECT lead FROM rooms WHERE name = ?`, r.room).Scan(&lead); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if lead != "" {
			var in int
			if err := t.QueryRow(`SELECT COUNT(*) FROM room_members WHERE room = ? AND member_kind = 'session' AND member = ?`, r.room, lead).Scan(&in); err != nil {
				return err
			}
			if _, err := knownSession(t, lead); err == nil && in > 0 {
				list = []string{lead}
			}
		}
	}
	if len(list) == 0 {
		return nil
	}
	if r.room == "" {
		return fmt.Errorf("only a message in a group chat can mention chats; this one goes to %s alone, so send it without mentions", r.label)
	}
	members, err := groupChats(t, r.room)
	if err != nil {
		return err
	}
	self := ""
	if from.Session != nil {
		self = from.Session.ID
	}
	var ids []string
	seen := map[string]bool{}
	add := func(id string) {
		if id != self && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	for _, m := range list {
		switch f := fold(m); {
		case reservedMention(f):
			wakeAll = true
			add(Everyone)
			continue
		case f == Human || f == fold(person):
			add(Human)
			continue
		}
		c, err := matchMember(members, m)
		if errors.Is(err, errNoChat) {
			if other, e := findKnownChat(t, m); e == nil {
				return fmt.Errorf("%s is not in the group chat %q, so the message was not sent. Mention only the group's members (%s), or ask %s to add the chat to the group", other.address(), r.roomTitle, addressList(members), person)
			}
			return fmt.Errorf("no chat in the group chat %q is called %q, so the message was not sent. Its members are %s", r.roomTitle, m, addressList(members))
		}
		if err != nil {
			return err
		}
		if _, gone, _ := teamLeadOf(t, c.id); gone {
			return fmt.Errorf("%s is retired from its team, so the message was not sent. Reopen it first", c.address())
		}
		add(c.id)
	}
	if seen[Human] && from.Session != nil {
		// A team member's message to the person goes to its lead, as a request to relay; the
		// message still shows in the group, and the person is not notified (teams.go).
		lead, _, err := teamLeadOf(t, from.Session.ID)
		if err != nil {
			return err
		}
		if lead != "" {
			if _, err := knownSession(t, lead); err == nil {
				ids = slicesWithout(ids, Human)
				ids = append(ids, lead)
				r.recipients = append(r.recipients, recipient{"session", lead})
				r.relay = lead
				seen[Human] = false
			}
		}
	}
	if seen[Human] && from.Session != nil {
		r.recipients = append(r.recipients, recipient{"human", Human})
	}
	if wakeAll {
		ids = append(ids, WakesAll)
	}
	if len(ids) > 0 {
		r.mentions = "," + strings.Join(ids, ",") + ","
	}
	return nil
}

// mentioned reads the list a message keeps into m.Mentioned, each chat by
// its title as it is now.
func (b *Bus) mentioned(ctx context.Context, m *Message) error {
	m.Mentioned = nil
	for _, key := range strings.Split(m.mentions, ",") {
		switch key {
		case "", WakesAll:
			continue
		case Human, Everyone:
			m.Mentioned = append(m.Mentioned, Mention{Key: key, Title: key, Address: key})
			continue
		}
		var title, name, label string
		err := b.db.QueryRowContext(ctx, `SELECT s.title, w.name, w.label FROM sessions s JOIN workspaces w ON w.key = s.workspace
			WHERE s.id = ?`, key).Scan(&title, &name, &label)
		if errors.Is(err, sql.ErrNoRows) {
			// A message sent before groups were made of chats may mention a
			// workspace.
			if b.db.QueryRowContext(ctx, `SELECT name, label FROM workspaces WHERE key = ?`, key).Scan(&name, &label) == nil {
				if label == "" {
					label = name
				}
				m.Mentioned = append(m.Mentioned, Mention{Key: key, Title: label, Address: name})
			}
			continue
		}
		if err != nil {
			return err
		}
		if label == "" {
			label = name
		}
		shown := DisplayTitle(title, label)
		m.Mentioned = append(m.Mentioned, Mention{Key: key, Title: shown, Address: shown + " (" + name + ")"})
	}
	return nil
}

// madeUpName is the name a group takes from its chats' titles when it is
// given none: "A", "A and B" or "A, B and C".
func madeUpName(titles []string) string {
	switch len(titles) {
	case 0:
		return ""
	case 1:
		return titles[0]
	}
	return strings.Join(titles[:len(titles)-1], ", ") + " and " + titles[len(titles)-1]
}

// MadeUpGroupName is the name a group of chats with these titles takes when
// it is given none, as the window's picker shows it before the group is
// made.
func MadeUpGroupName(titles []string) string { return madeUpName(titles) }

// groupKey makes a group's key from its title: its letters and digits in
// lower case, with a hyphen for each run of anything else, and a number
// after it when another group has it.
func groupKey(t *sql.Tx, title string) string {
	var b strings.Builder
	gap := false
	for _, r := range strings.ToLower(title) {
		if r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			if gap && b.Len() > 0 {
				b.WriteByte('-')
			}
			gap = false
			b.WriteRune(r)
		} else {
			gap = true
		}
		if b.Len() >= 48 {
			break
		}
	}
	base := b.String()
	if base == "" {
		base = "group"
	}
	key := base
	for i := 2; taken(t, `SELECT 1 FROM rooms WHERE name = ?`, key); i++ {
		key = base + "-" + strconv.Itoa(i)
	}
	return key
}

// joinable finds the chat an address is for, for a group to take it: open,
// or closed in the window, and titled, since two untitled chats in one
// workspace cannot be told apart in a group.
func joinable(t *sql.Tx, address string) (chat, error) {
	c, err := findKnownChat(t, address)
	if errors.Is(err, errNoChat) {
		return chat{}, fmt.Errorf("no chat is called %q", strings.TrimSpace(address))
	}
	if err != nil {
		return chat{}, err
	}
	if strings.TrimSpace(c.title) == "" {
		return chat{}, fmt.Errorf("%s has no title yet; give it one before it joins a group chat", c.display())
	}
	return c, nil
}

// CreateGroup makes a group chat of the person, the chat that makes it, if
// a chat makes it, and the chats given, each by its session ID or its title
// (findKnownChat). A group given no title takes one from its chats' titles.
// It returns the new group.
func (b *Bus) CreateGroup(ctx context.Context, from Sender, title string, members []string) (Room, error) {
	title = strings.Join(strings.Fields(strings.TrimPrefix(strings.TrimSpace(title), "#")), " ")
	var key string
	err := b.tx(ctx, func(t *sql.Tx) error {
		var chats []chat
		seen := map[string]bool{}
		take := func(c chat) {
			if !seen[c.id] {
				seen[c.id] = true
				chats = append(chats, c)
			}
		}
		if from.Session != nil {
			c, err := joinable(t, from.Session.ID)
			if err != nil {
				return err
			}
			take(c)
		}
		for _, m := range members {
			if strings.TrimSpace(m) == "" {
				continue
			}
			c, err := joinable(t, m)
			if err != nil {
				return err
			}
			take(c)
		}
		if len(chats) == 0 || (from.Session != nil && len(chats) == 1) {
			return errors.New("choose at least one other chat for the group")
		}
		if title == "" {
			var titles []string
			for _, c := range chats {
				titles = append(titles, c.display())
			}
			title = madeUpName(titles)
		}
		if _, _, err := findGroup(t, title); err == nil {
			return fmt.Errorf("a group chat called %q already exists", title)
		}
		key = groupKey(t, title)
		if _, err := t.Exec(`INSERT INTO rooms (name, title, created_at) VALUES (?, ?, ?)`, key, title, b.stamp()); err != nil {
			return err
		}
		for _, c := range chats {
			if _, err := t.Exec(`INSERT INTO room_members (room, member_kind, member) VALUES (?, 'session', ?)`, key, c.id); err != nil {
				return err
			}
		}
		_, err := t.Exec(`INSERT INTO room_members (room, member_kind, member) VALUES (?, 'human', ?)`, key, Human)
		return err
	})
	if err != nil {
		return Room{}, err
	}
	return b.Room(ctx, key)
}

// memberLine records in a group that a chat joined or left it.
func (b *Bus) memberLine(t *sql.Tx, by Sender, key, title string, c chat, verb string) error {
	_, err := b.insertMessage(t, by, MemberKind, resolved{label: c.display(), room: key, roomTitle: title, toKind: "session", toKey: c.id}, 0, verb, "", "", nil)
	return err
}

// renamedPrefix begins the text of the line a group keeps when it is renamed; the
// rest is the new title.
const renamedPrefix = "renamed "

// RenameGroup gives a group a new title. The group keeps its stable name, so its
// members, messages, budget and unread marks stay as they are, and messages
// delivered afterwards use the new title. The old title is remembered, and an
// address by it is refused with the new title in the reason. It is the person's
// to do: no agent tool calls it. A title another group has is refused, and one
// that differs only in case is a rename.
func (b *Bus) RenameGroup(ctx context.Context, by Sender, group, title string) (Room, error) {
	title = CleanTitle(strings.TrimPrefix(strings.TrimSpace(title), "#"))
	if title == "" {
		return Room{}, errors.New("a group chat needs a name")
	}
	var key string
	err := b.tx(ctx, func(t *sql.Tx) error {
		k, old, err := findGroup(t, group)
		if err != nil {
			return err
		}
		key = k
		if old == title {
			return nil
		}
		if other, _, err := findGroup(t, title); err == nil && other != key {
			return fmt.Errorf("a group chat called %q already exists", title)
		}
		if _, err := t.Exec(`UPDATE rooms SET title = ? WHERE name = ?`, title, key); err != nil {
			return err
		}
		if fold(old) != fold(title) {
			if _, err := t.Exec(`INSERT INTO room_renames (room, old_title, renamed_at) VALUES (?, ?, ?)`, key, old, b.stamp()); err != nil {
				return err
			}
		}
		_, err = b.insertMessage(t, by, MemberKind, resolved{label: "#" + title, room: key, roomTitle: title, toKind: "room", toKey: key}, 0, renamedPrefix+title, "", "", nil)
		return err
	})
	if err != nil {
		return Room{}, err
	}
	return b.Room(ctx, key)
}

// AddToGroup adds chats to a group, each by its session ID or its title. A
// chat already in it is left as it is. The group keeps a line for each chat
// added, which says the chat joined when it added itself.
func (b *Bus) AddToGroup(ctx context.Context, by Sender, group string, members []string) error {
	return b.tx(ctx, func(t *sql.Tx) error {
		key, title, err := findGroup(t, group)
		if err != nil {
			return err
		}
		// A team's group makes whoever joins it a member of the team, in this transaction, so a chat is never
		// in the group without being led by its lead; a refusal adds nobody.
		var teamLead string
		if err := t.QueryRow(`SELECT lead FROM rooms WHERE name = ?`, key).Scan(&teamLead); err != nil {
			return err
		}
		for _, m := range members {
			if strings.TrimSpace(m) == "" {
				continue
			}
			c, err := joinable(t, m)
			if err != nil {
				return err
			}
			wasIn := taken(t, `SELECT 1 FROM room_members WHERE room = ? AND member_kind = 'session' AND member = ?`, key, c.id)
			if teamLead != "" && c.id != teamLead {
				previous, _, err := teamLeadOf(t, c.id)
				if err != nil {
					return err
				}
				if previous != "" && previous != teamLead {
					prevTitle, _ := knownSession(t, previous)
					return fmt.Errorf("%s is already a member of %s's team, so nobody was added. Take it out of that team first", c.address(), prevTitle)
				}
				if _, err := b.setLead(t, c.id, teamLead); err != nil {
					return err
				}
			}
			if wasIn {
				continue
			}
			if _, err := t.Exec(`INSERT OR IGNORE INTO room_members (room, member_kind, member) VALUES (?, 'session', ?)`, key, c.id); err != nil {
				return err
			}
			verb := "added"
			if by.Session != nil && by.Session.ID == c.id {
				verb = "joined"
			}
			if err := b.memberLine(t, by, key, title, c, verb); err != nil {
				return err
			}
		}
		return nil
	})
}

// RemoveFromGroup takes a chat out of a group, by its session ID or its
// title. The person is always a member and cannot be taken out; a group the
// person no longer wants is deleted instead.
func (b *Bus) RemoveFromGroup(ctx context.Context, by Sender, group, member string) error {
	return b.tx(ctx, func(t *sql.Tx) error {
		key, title, err := findGroup(t, group)
		if err != nil {
			return err
		}
		if fold(member) == Human {
			return errors.New("the person is always in a group chat; delete the group instead")
		}
		members, err := groupChats(t, key)
		if err != nil {
			return err
		}
		c, err := matchMember(members, strings.TrimSpace(member))
		if errors.Is(err, errNoChat) {
			return fmt.Errorf("no chat in the group chat %q is called %q", title, strings.TrimSpace(member))
		}
		if err != nil {
			return err
		}
		if _, err := t.Exec(`DELETE FROM room_members WHERE room = ? AND member_kind = 'session' AND member = ?`, key, c.id); err != nil {
			return err
		}
		verb := "removed"
		if by.Session != nil && by.Session.ID == c.id {
			verb = "left"
		}
		return b.memberLine(t, by, key, title, c, verb)
	})
}

// DeleteRoom removes a group and its members. Its messages stay, as the
// history of what was said.
func (b *Bus) DeleteRoom(ctx context.Context, room string) error {
	return b.tx(ctx, func(t *sql.Tx) error {
		key, _, err := findGroup(t, room)
		if err != nil {
			return err
		}
		if _, err := t.Exec(`DELETE FROM room_renames WHERE room = ?`, key); err != nil {
			return err
		}
		if _, err := t.Exec(`DELETE FROM room_members WHERE room = ?`, key); err != nil {
			return err
		}
		_, err = t.Exec(`DELETE FROM rooms WHERE name = ?`, key)
		return err
	})
}

// RoomMembers lists a group's member chats, in the order they joined, and
// then the person.
func (b *Bus) RoomMembers(ctx context.Context, room string) ([]RoomMember, error) {
	rows, err := b.db.QueryContext(ctx, `SELECT m.member_kind, m.member, COALESCE(s.title, ''), COALESCE(w.name, ''),
		COALESCE(w.label, ''), COALESCE(s.ended_at IS NULL, 0)
		FROM room_members m LEFT JOIN sessions s ON m.member_kind = 'session' AND s.id = m.member
		LEFT JOIN workspaces w ON w.key = s.workspace
		WHERE m.room = ? AND (m.member_kind = 'human' OR s.id IS NOT NULL)
		ORDER BY m.member_kind = 'human', m.rowid`, room)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RoomMember
	for rows.Next() {
		var m RoomMember
		var title string
		if err := rows.Scan(&m.Kind, &m.Key, &title, &m.WorkspaceName, &m.Workspace, &m.Open); err != nil {
			return nil, err
		}
		if m.Kind == "human" {
			m.Title, m.Address, m.Open = Human, Human, true
		} else {
			if m.Workspace == "" {
				m.Workspace = m.WorkspaceName
			}
			m.Title = DisplayTitle(title, m.Workspace)
			m.Address = m.Title + " (" + m.WorkspaceName + ")"
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Room returns one group by its key or its title.
func (b *Bus) Room(ctx context.Context, group string) (Room, error) {
	key, title, err := findGroup(b.db, group)
	if err != nil {
		return Room{}, err
	}
	return b.room(ctx, key, title)
}

func (b *Bus) room(ctx context.Context, key, title string) (Room, error) {
	r := Room{Name: key, Title: groupTitle(key, title)}
	var err error
	if r.Members, err = b.RoomMembers(ctx, key); err != nil {
		return r, err
	}
	err = b.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(created_at), ''),
		(SELECT COUNT(*) FROM deliveries d JOIN messages x ON x.id = d.message_id
			WHERE d.recipient_kind = ? AND d.delivered_at IS NULL AND x.room = ?)
		FROM messages WHERE room = ?`, humanRoom, key, key).Scan(&r.LastAt, &r.Unread)
	if err != nil {
		return r, err
	}
	if r.Unread > 0 {
		var newest int64
		err = b.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(d.message_id), 0) FROM deliveries d JOIN messages x ON x.id = d.message_id
			WHERE d.recipient_kind = ? AND d.delivered_at IS NULL AND x.room = ? AND x.kind = 'text'`, humanRoom, key).Scan(&newest)
		if err != nil {
			return r, err
		}
		if newest != 0 {
			m, err := b.Message(ctx, newest)
			if err != nil {
				return r, err
			}
			r.NewestID, r.NewestFrom, r.NewestLine = m.ID, m.sender(), NoticeLine(m)
		}
	}
	err = b.db.QueryRowContext(ctx, `SELECT turn_budget, turns_used, lead FROM rooms WHERE name = ?`, key).Scan(&r.Budget, &r.Used, &r.Lead)
	return r, err
}

// Rooms returns every group with its members, the one spoken in last first.
func (b *Bus) Rooms(ctx context.Context) ([]Room, error) {
	rows, err := b.db.QueryContext(ctx, `SELECT r.name, r.title FROM rooms r
		ORDER BY COALESCE((SELECT MAX(m.created_at) FROM messages m WHERE m.room = r.name), r.created_at) DESC, lower(r.title)`)
	if err != nil {
		return nil, err
	}
	type row struct{ key, title string }
	var keys []row
	for rows.Next() {
		var x row
		if err := rows.Scan(&x.key, &x.title); err != nil {
			rows.Close()
			return nil, err
		}
		keys = append(keys, x)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]Room, 0, len(keys))
	for _, x := range keys {
		r, err := b.room(ctx, x.key, x.title)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// RoomMessages returns a group's messages, oldest first, with the lines
// saying who joined and left.
func (b *Bus) RoomMessages(ctx context.Context, room string, limit int) ([]Message, error) {
	return b.queryMessages(ctx, `SELECT * FROM (`+messageSelect+` WHERE m.room = ? ORDER BY m.id DESC LIMIT ?) ORDER BY id`, room, limit)
}

// MarkRoomRead records that the person has seen a group's messages in its
// window, those that mention them in their inbox included. Nothing is written
// when none is unread, which keeps the change
// counter still, so a window that marks its messages read does not make
// itself draw again.
func (b *Bus) MarkRoomRead(ctx context.Context, room string) error {
	var n int
	if err := b.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM deliveries d JOIN messages m ON m.id = d.message_id
		WHERE d.recipient_kind IN (?, 'human') AND d.delivered_at IS NULL AND m.room = ?`, humanRoom, room).Scan(&n); err != nil || n == 0 {
		return err
	}
	return b.tx(ctx, func(t *sql.Tx) error {
		_, err := t.Exec(`UPDATE deliveries SET delivered_at = ? WHERE recipient_kind IN (?, 'human') AND delivered_at IS NULL
			AND message_id IN (SELECT id FROM messages WHERE room = ?)`, b.stamp(), humanRoom, room)
		return err
	})
}

// RemoveWorkspace takes a workspace off the bus: one with no session open,
// such as one a session started in a folder by mistake. Its chats leave
// their groups, and its links, subscriptions and waiting deliveries go with
// it; its messages stay.
func (b *Bus) RemoveWorkspace(ctx context.Context, name string) error {
	name = strings.ToLower(strings.TrimSpace(name))
	return b.tx(ctx, func(t *sql.Tx) error {
		var key string
		if err := t.QueryRow(`SELECT key FROM workspaces WHERE name = ?`, name).Scan(&key); err != nil {
			return fmt.Errorf("no workspace on the bus is called %q", name)
		}
		if taken(t, `SELECT 1 FROM sessions WHERE workspace = ? AND ended_at IS NULL`, key) {
			return fmt.Errorf("%s has a session open; close it first", name)
		}
		for _, q := range []string{
			`DELETE FROM room_members WHERE member_kind = 'session' AND member IN (SELECT id FROM sessions WHERE workspace = ?)`,
			`DELETE FROM links WHERE workspace_a = ?1 OR workspace_b = ?1`,
			`DELETE FROM subscriptions WHERE consumer = ?1 OR library = ?1`,
			`DELETE FROM deliveries WHERE recipient_kind = 'workspace' AND recipient = ? AND delivered_at IS NULL`,
			`DELETE FROM agent_sessions WHERE session IN (SELECT id FROM sessions WHERE workspace = ?)`,
			`DELETE FROM sessions WHERE workspace = ?`,
			`DELETE FROM workspaces WHERE key = ?`,
		} {
			if _, err := t.Exec(q, key); err != nil {
				return err
			}
		}
		return nil
	})
}

// convertRooms makes the rooms of a bus from before group chats into groups
// of chats, once (Paul, 27 October 2026). A workspace member becomes the
// chats of that workspace the bus knows at the time: those open, and the
// window's closed chats (knownChats). A room with no title takes its name
// as its title, the person becomes a member of every room, and the rooms
// keep their messages. Run again, it finds nothing to do, and an older
// process that adds a workspace to a room has it converted at the next
// open.
func convertRooms(db *sql.DB) error {
	var n int
	if err := db.QueryRow(`SELECT (SELECT COUNT(*) FROM room_members WHERE member_kind = 'workspace')
		+ (SELECT COUNT(*) FROM rooms WHERE title = '')
		+ (SELECT COUNT(*) FROM rooms r WHERE NOT EXISTS (SELECT 1 FROM room_members m WHERE m.room = r.name AND m.member_kind = 'human'))`).Scan(&n); err != nil || n == 0 {
		return err
	}
	t, err := db.Begin()
	if err != nil {
		return err
	}
	for _, q := range []string{
		`INSERT OR IGNORE INTO room_members (room, member_kind, member)
			SELECT m.room, 'session', s.id FROM room_members m JOIN sessions s ON s.workspace = m.member
			WHERE m.member_kind = 'workspace' AND (s.ended_at IS NULL OR s.id LIKE '` + WindowSessionPrefix + `%')
			ORDER BY m.room, s.started_at, s.id`,
		`DELETE FROM room_members WHERE member_kind = 'workspace'`,
		`UPDATE rooms SET title = name WHERE title = ''`,
		`INSERT OR IGNORE INTO room_members (room, member_kind, member) SELECT name, 'human', '` + Human + `' FROM rooms`,
		`UPDATE changes SET counter = counter + 1 WHERE id = 1`,
	} {
		if _, err := t.Exec(q); err != nil {
			t.Rollback()
			return err
		}
	}
	return t.Commit()
}
