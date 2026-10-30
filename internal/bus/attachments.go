package bus

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// The person attaches files to messages in a group chat as in a single chat
// (docs/plans/group-attachments.md). The person has no workspace on the bus,
// so the bus keeps its own copy of each file in Dinah's home folder, under
// attachments and the date it was sent, and records the copy as an
// attachment of the message with its hash, as it records an agent's file.
// A copy lasts as long as its message, until the person removes the copies
// older than an age they choose (RemovePersonFiles); the message and the
// record of its attachment stay, marked as removed.

// MaxAttachmentFile is the largest file the person may attach to a message,
// in a single chat or a group chat.
const MaxAttachmentFile = 25 << 20

// PersonFile is a file the person attaches: its name as it was attached, its
// type, which may be empty, and its contents.
type PersonFile struct {
	Name string
	Mime string
	Data []byte
}

// TooLarge is the reason a file over MaxAttachmentFile is refused, in the
// words a single chat uses.
func TooLarge(name string) error {
	return fmt.Errorf("%s is larger than 25 MB, so it was not attached", name)
}

// SendFiles sends the person's message, with files the bus keeps a copy of,
// to an address as Send does, with the members of a group it mentions
// (SendMentioning). A message of files alone needs no words. A file over
// MaxAttachmentFile is refused, and then nothing is sent or kept.
func (b *Bus) SendFiles(ctx context.Context, to, text string, files []PersonFile, replyTo int64, mentions []string) (Message, error) {
	atts, err := b.storeFiles(files)
	if err != nil {
		return Message{}, err
	}
	m, err := b.send(ctx, Sender{}, to, text, atts, replyTo, mentions)
	if err != nil {
		for _, a := range atts {
			os.Remove(a.Path)
		}
	}
	return m, err
}

// attachmentDir is the folder the bus keeps the person's files in.
func (b *Bus) attachmentDir() string { return filepath.Join(b.dir, "attachments") }

// storeFiles copies the person's files into the attachment folder, under
// the local date, and returns them as attachments. Every file is checked
// before any is written, and what was written is removed again when a later
// one fails.
func (b *Bus) storeFiles(files []PersonFile) ([]Attachment, error) {
	for _, f := range files {
		if len(f.Data) > MaxAttachmentFile {
			return nil, TooLarge(personFileName(f.Name))
		}
	}
	if len(files) == 0 {
		return nil, nil
	}
	dir := filepath.Join(b.attachmentDir(), b.now().Format("2006-01-02"))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	var out []Attachment
	fail := func(err error) ([]Attachment, error) {
		for _, a := range out {
			os.Remove(a.Path)
		}
		return nil, err
	}
	for i, f := range files {
		name := personFileName(f.Name)
		p, err := writeNew(dir, name, f.Data)
		if err != nil {
			return fail(err)
		}
		sum := sha256.Sum256(f.Data)
		out = append(out, Attachment{Index: i + 1, Path: p, Rel: name, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(f.Data)),
			Mime: fileType(name, f.Mime, f.Data), Stored: true})
	}
	return out, nil
}

var unsafeFileChars = regexp.MustCompile(`[^\w.\- ()]+`)

// personFileName is a file's name made safe to keep in a folder: its last
// element, with any character other than letters, digits, spaces, dots,
// dashes and parentheses made an underscore.
func personFileName(name string) string {
	name = strings.TrimSpace(unsafeFileChars.ReplaceAllString(filepath.Base(strings.ReplaceAll(name, `\`, "/")), "_"))
	if name == "" || name == "." || name == ".." {
		return "attachment"
	}
	return name
}

// writeNew writes data to a new file of the name given in dir, or of the
// name with (2), (3) and so on before its extension when the name is taken.
// The file is created only if it does not exist, so two processes sending
// at once never write over each other's files.
func writeNew(dir, name string, data []byte) (string, error) {
	ext := filepath.Ext(name)
	p := filepath.Join(dir, name)
	for i := 2; ; i++ {
		f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if errors.Is(err, os.ErrExist) {
			p = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", strings.TrimSuffix(name, ext), i, ext))
			continue
		}
		if err != nil {
			return "", err
		}
		_, err = f.Write(data)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(p)
			return "", err
		}
		return p, nil
	}
}

// fileType is a file's media type: the type given, or the one its
// extension has, or the one its contents suggest, without parameters.
func fileType(name, given string, data []byte) string {
	t := given
	if t == "" {
		t = mime.TypeByExtension(strings.ToLower(filepath.Ext(name)))
	}
	if t == "" {
		t = http.DetectContentType(data)
	}
	if parsed, _, err := mime.ParseMediaType(t); err == nil {
		t = parsed
	}
	return t
}

// StoredFiles is a count of the person's attached files that the bus keeps
// and their size in bytes.
type StoredFiles struct {
	Files int
	Bytes int64
}

// storedFile is one of the person's files the bus keeps.
type storedFile struct {
	message int64
	index   int
	path    string
	size    int64
}

// storedBefore returns the person's files the bus still keeps from messages
// sent before the time given.
func (b *Bus) storedBefore(ctx context.Context, before time.Time) ([]storedFile, error) {
	rows, err := b.db.QueryContext(ctx, `SELECT a.message_id, a.idx, a.path, a.size, m.created_at FROM attachments a
		JOIN messages m ON m.id = a.message_id WHERE a.stored = 1 AND a.removed_at = '' ORDER BY a.message_id, a.idx`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []storedFile
	for rows.Next() {
		var f storedFile
		var created string
		if err := rows.Scan(&f.message, &f.index, &f.path, &f.size, &created); err != nil {
			return nil, err
		}
		at, err := time.Parse(time.RFC3339Nano, created)
		if err != nil || !at.Before(before) {
			continue
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// PersonFilesBefore counts the person's files the bus keeps from messages
// sent before the time given, so the person can be told what removing them
// would free.
func (b *Bus) PersonFilesBefore(ctx context.Context, before time.Time) (StoredFiles, error) {
	files, err := b.storedBefore(ctx, before)
	var out StoredFiles
	for _, f := range files {
		out.Files++
		out.Bytes += f.size
	}
	return out, err
}

// RemovePersonFiles removes the bus's copies of the person's files from
// messages sent before the time given and marks each attachment removed.
// The messages stay, and so does the record of each attachment, which the
// window shows as removed. A copy already gone counts as removed. A copy
// that cannot be removed, or that is not inside the attachment folder,
// stays as it is, and the first such failure is returned with the count of
// those that were removed.
func (b *Bus) RemovePersonFiles(ctx context.Context, before time.Time) (StoredFiles, error) {
	files, err := b.storedBefore(ctx, before)
	if err != nil {
		return StoredFiles{}, err
	}
	root := b.attachmentDir()
	var done []storedFile
	var out StoredFiles
	var first error
	dirs := map[string]bool{}
	for _, f := range files {
		rel, err := filepath.Rel(root, f.path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			if first == nil {
				first = fmt.Errorf("%s is outside the attachment folder %s, so it was left alone", f.path, root)
			}
			continue
		}
		if err := os.Remove(f.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			if first == nil {
				first = err
			}
			continue
		}
		dirs[filepath.Dir(f.path)] = true
		done = append(done, f)
		out.Files++
		out.Bytes += f.size
	}
	if len(done) > 0 {
		now := b.stamp()
		err := b.tx(ctx, func(t *sql.Tx) error {
			for _, f := range done {
				if _, err := t.Exec(`UPDATE attachments SET removed_at = ? WHERE message_id = ? AND idx = ?`, now, f.message, f.index); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return out, err
		}
	}
	// A date's folder left empty goes too; one that still has files fails
	// to be removed, which is what is wanted.
	for d := range dirs {
		os.Remove(d)
	}
	return out, first
}

// PersonFile returns one of the person's attachments that the bus keeps, for
// the window to show: a message's attachment by its number, when the bus
// keeps a copy of it and the copy has not been removed.
func (b *Bus) PersonFile(ctx context.Context, messageID int64, index int) (Attachment, error) {
	m, err := b.Message(ctx, messageID)
	if err != nil {
		return Attachment{}, err
	}
	for _, a := range m.Attachments {
		if a.Index != index {
			continue
		}
		if !a.Stored || a.Removed {
			break
		}
		return a, nil
	}
	return Attachment{}, fmt.Errorf("message %d has no kept attachment %d: %w", messageID, index, ErrNotFound)
}
