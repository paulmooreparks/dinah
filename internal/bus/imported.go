package bus

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// A workspace import brings group chats and their conversations from
// another computer (docs/design/workspace-export.md, "Groups and Teams").
// Each message it brings gets a new ID here, and message_origins records
// where it was first made: the identity of the Dinah that made it and its
// ID there, which travel with the message through every export. A message
// whose origin is already recorded, such as one that another workspace's
// export of the same group brought before, is not added again. An imported message is history: it is delivered to no one, so it
// wakes no chat and notifies nobody, and it does not change a group's turns.

// ImportedFile is a file attached to an imported message. Data is nil when
// the file did not travel, and the attachment is then recorded as removed.
type ImportedFile struct {
	Name   string
	Mime   string
	SHA256 string
	Size   int64
	Data   []byte
}

// ImportedMessage is a group message an import brings. FromSession is the
// sender's session on this bus, or empty for the person or for a chat that
// is not here, whose name FromName keeps. ReplyTo is the ID here of the
// message it answers, or 0. Mentions are sessions on this bus, with Human for
// the person.
type ImportedMessage struct {
	Exporter    string
	Origin      int64
	Group       string
	CreatedAt   string
	Kind        string
	FromSession string
	FromName    string
	Text        string
	ReplyTo     int64
	Mentions    []string
	Version     string
	Verdict     string
	Files       []ImportedFile
}

// ImportedMessageID is the ID here of the message an import brought from
// the exporter with the ID origin there, and whether there is one.
func (b *Bus) ImportedMessageID(ctx context.Context, exporter string, origin int64) (int64, bool, error) {
	var id int64
	err := b.db.QueryRowContext(ctx, `SELECT message_id FROM message_origins WHERE exporter = ? AND origin = ?`, exporter, origin).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	return id, err == nil, err
}

// MessageOrigin is where a message was first made, when an import brought
// it here: the exporter it came from and its ID there. ok is false for a
// message made here.
func (b *Bus) MessageOrigin(ctx context.Context, id int64) (exporter string, origin int64, ok bool, err error) {
	err = b.db.QueryRowContext(ctx, `SELECT exporter, origin FROM message_origins WHERE message_id = ?`, id).Scan(&exporter, &origin)
	if errors.Is(err, sql.ErrNoRows) {
		return "", 0, false, nil
	}
	return exporter, origin, err == nil, err
}

// GroupHasMessage reports whether the message with the ID is in the group,
// by the group's key or title.
func (b *Bus) GroupHasMessage(ctx context.Context, group string, id int64) (bool, error) {
	key, _, err := findGroup(b.db, group)
	if err != nil {
		return false, err
	}
	var n int
	err = b.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM messages WHERE id = ? AND room = ?`, id, key).Scan(&n)
	return n > 0, err
}

// ImportMessage adds a group message an import brought, with the files that
// travelled with it, kept as the person's files are (attachments.go). It
// returns the message's ID here and whether it was added; a message whose
// origin is recorded is not added again, and its ID is returned.
func (b *Bus) ImportMessage(ctx context.Context, m ImportedMessage) (int64, bool, error) {
	if m.Exporter == "" || m.Origin <= 0 {
		return 0, false, errors.New("an imported message needs its exporter and its ID there")
	}
	if id, ok, err := b.ImportedMessageID(ctx, m.Exporter, m.Origin); err != nil || ok {
		return id, false, err
	}
	var stored []PersonFile
	for _, f := range m.Files {
		if f.Data != nil {
			sum := sha256.Sum256(f.Data)
			if hex.EncodeToString(sum[:]) != strings.ToLower(f.SHA256) {
				return 0, false, errors.New("an imported file does not have the content it was sent with")
			}
			stored = append(stored, PersonFile{Name: f.Name, Mime: f.Mime, Data: f.Data})
		}
	}
	copies, err := b.storeFiles(stored)
	if err != nil {
		return 0, false, err
	}
	var id int64
	added := false
	err = b.tx(ctx, func(t *sql.Tx) error {
		// Another import may have added the message since it was looked for.
		if t.QueryRow(`SELECT message_id FROM message_origins WHERE exporter = ? AND origin = ?`, m.Exporter, m.Origin).Scan(&id) == nil {
			return nil
		}
		key, title, err := findGroup(t, m.Group)
		if err != nil {
			return err
		}
		var fromWS string
		if m.FromSession != "" {
			if err := t.QueryRow(`SELECT workspace FROM sessions WHERE id = ?`, m.FromSession).Scan(&fromWS); err != nil {
				return err
			}
		}
		mentions := ""
		if len(m.Mentions) > 0 {
			mentions = "," + strings.Join(m.Mentions, ",") + ","
		}
		kind := m.Kind
		if kind == "" {
			kind = "text"
		}
		res, err := t.Exec(`INSERT INTO messages (created_at, kind, from_session, from_name, from_workspace, to_label, to_kind, to_key, room, reply_to, text, version, verdict, hops, mentions)
			VALUES (?, ?, ?, ?, ?, ?, 'room', ?, ?, ?, ?, ?, ?, 0, ?)`,
			m.CreatedAt, kind, m.FromSession, m.FromName, fromWS, "#"+title, key, key, m.ReplyTo, m.Text, m.Version, m.Verdict, mentions)
		if err != nil {
			return err
		}
		if id, err = res.LastInsertId(); err != nil {
			return err
		}
		index, next := 0, 0
		for _, f := range m.Files {
			index++
			if f.Data == nil {
				if _, err := t.Exec(`INSERT INTO attachments (message_id, idx, path, rel, sha256, size, mime, stored, removed_at) VALUES (?, ?, '', ?, ?, ?, ?, 1, ?)`,
					id, index, f.Name, strings.ToLower(f.SHA256), f.Size, f.Mime, b.stamp()); err != nil {
					return err
				}
				continue
			}
			c := copies[next]
			next++
			if _, err := t.Exec(`INSERT INTO attachments (message_id, idx, path, rel, sha256, size, mime, stored) VALUES (?, ?, ?, ?, ?, ?, ?, 1)`,
				id, index, c.Path, c.Rel, c.SHA256, c.Size, c.Mime); err != nil {
				return err
			}
		}
		if _, err := t.Exec(`INSERT INTO message_origins (message_id, exporter, origin) VALUES (?, ?, ?)`, id, m.Exporter, m.Origin); err != nil {
			return err
		}
		added = true
		return nil
	})
	if err != nil || !added {
		// The copies of a message that was not added are not kept.
		for _, c := range copies {
			os.Remove(c.Path)
		}
	}
	return id, added, err
}

// ImportedGroup is a group an import brings: its title, its turn budget for
// a group it makes, the session that leads it, if any, and its member
// sessions, all on this bus.
type ImportedGroup struct {
	Title   string
	Budget  int
	Lead    string
	Members []string
}

// ImportedGroupResult is what importing a group did. KeptLead is the lead a
// group already here kept, when the import's lead was another. Skipped are
// the members not added, because each is already in another lead's team and
// the group is a team's.
type ImportedGroupResult struct {
	Key      string
	Created  bool
	KeptLead string
	Added    int
	Skipped  []string
}

// ImportGroup joins an imported group to the group of the same title here,
// or makes it. The person is a member of every group, as always. A group
// that has no lead takes the import's, when no other group is that lead's
// team already; a group that has a lead keeps it. A member is added as a
// chat joins a team's group: it becomes a member of the lead's team, and one
// that is in another team is passed over. No line is written for a member
// added, since the conversation the import brings has the group's own.
func (b *Bus) ImportGroup(ctx context.Context, g ImportedGroup) (ImportedGroupResult, error) {
	var out ImportedGroupResult
	title := strings.Join(strings.Fields(strings.TrimPrefix(strings.TrimSpace(g.Title), "#")), " ")
	if title == "" {
		return out, errors.New("an imported group needs a name")
	}
	err := b.tx(ctx, func(t *sql.Tx) error {
		out = ImportedGroupResult{}
		key, _, err := findGroup(t, title)
		if errors.Is(err, ErrNotFound) {
			key = groupKey(t, title)
			budget := g.Budget
			if budget < NoTurnBudget {
				budget = NoTurnBudget
			}
			if _, err := t.Exec(`INSERT INTO rooms (name, title, created_at, turn_budget) VALUES (?, ?, ?, ?)`, key, title, b.stamp(), budget); err != nil {
				return err
			}
			if _, err := t.Exec(`INSERT INTO room_members (room, member_kind, member) VALUES (?, 'human', ?)`, key, Human); err != nil {
				return err
			}
			out.Created = true
		} else if err != nil {
			return err
		}
		out.Key = key
		var lead string
		if err := t.QueryRow(`SELECT lead FROM rooms WHERE name = ?`, key).Scan(&lead); err != nil {
			return err
		}
		if g.Lead != "" {
			switch {
			case lead == "":
				if !taken(t, `SELECT 1 FROM rooms WHERE lead = ?`, g.Lead) {
					if _, err := t.Exec(`UPDATE rooms SET lead = ? WHERE name = ?`, g.Lead, key); err != nil {
						return err
					}
					lead = g.Lead
				}
			case lead != g.Lead:
				out.KeptLead = lead
			}
		}
		members := append([]string{}, g.Members...)
		if lead != "" && !slicesContains(members, lead) {
			members = append([]string{lead}, members...)
		}
		for _, member := range members {
			if !taken(t, `SELECT 1 FROM sessions WHERE id = ? AND removed_at IS NULL`, member) {
				continue
			}
			if taken(t, `SELECT 1 FROM room_members WHERE room = ? AND member_kind = 'session' AND member = ?`, key, member) {
				continue
			}
			if lead != "" && member != lead {
				previous, _, err := teamLeadOf(t, member)
				if err != nil {
					return err
				}
				if previous != "" && previous != lead {
					out.Skipped = append(out.Skipped, member)
					continue
				}
				if previous == "" {
					if err := checkTeamLead(t, member, lead); err != nil {
						out.Skipped = append(out.Skipped, member)
						continue
					}
					if _, err := b.assignLead(t, member, lead); err != nil {
						return err
					}
					out.Added++
					continue
				}
			}
			if _, err := t.Exec(`INSERT OR IGNORE INTO room_members (room, member_kind, member) VALUES (?, 'session', ?)`, key, member); err != nil {
				return err
			}
			out.Added++
		}
		return nil
	})
	return out, err
}

// WorkspaceNameAt is the name the bus knows the workspace in a folder by,
// or "" when the bus does not know the folder.
func (b *Bus) WorkspaceNameAt(ctx context.Context, path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var name string
	err = b.db.QueryRowContext(ctx, `SELECT name FROM workspaces WHERE key = ?`, WorkspaceKey(abs)).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return name, err
}
