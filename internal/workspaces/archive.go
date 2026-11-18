package workspaces

import (
	"archive/zip"
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"studio/internal/statefile"
)

// An export file is a zip archive (archive/zip) since November 2026. Its
// entry workspace.json is the document Export describes; each chat's saved
// history is the entry chats/<key>.json, each group's conversation the
// entry groups/<key>.jsonl with one GroupMessage on each line, and each
// file attached to a group message the entry files/<sha256>, named by the
// SHA-256 of its content so that a file attached twice is kept once. An
// earlier build wrote the document alone, as bare JSON, and OpenArchive
// still reads that; the two are told apart by the zip signature at the start
// of the file. An entry this build does not know, with a safe name, is
// passed over, so that a later build can add one (such as a passphrase)
// without changing the others.

// The archive's entries.
const (
	ArchiveDocument = "workspace.json"
	archiveChats    = "chats/"
	archiveGroups   = "groups/"
	archiveFiles    = "files/"
)

// The limits an archive is read within. A history or a conversation can run
// to hundreds of megabytes; a group's file is at most what the bus accepts.
const (
	MaxExportDocument  = 8 << 20
	MaxHistoryEntry    = 1 << 30
	MaxGroupEntry      = 1 << 30
	MaxFileEntry       = 25 << 20
	MaxGroupMessages   = 1000000
	MaxArchiveEntries  = 200000
	MaxArchiveContents = 16 << 30
)

// HistoryEntry is the entry of a chat's saved history.
func HistoryEntry(key string) string { return archiveChats + key + ".json" }

// GroupEntry is the entry of a group's conversation.
func GroupEntry(key string) string { return archiveGroups + key + ".jsonl" }

// FileEntry is the entry of a file whose content has the SHA-256 sum, in
// lowercase hexadecimal.
func FileEntry(sum string) string { return archiveFiles + sum }

var fileSum = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ArchiveWriter writes an export archive. The entries are written as they
// come, and Finish writes the document last, once it knows them all.
type ArchiveWriter struct {
	zw    *zip.Writer
	names map[string]bool
}

// NewArchiveWriter starts an archive written to w.
func NewArchiveWriter(w io.Writer) *ArchiveWriter {
	return &ArchiveWriter{zw: zip.NewWriter(w), names: map[string]bool{}}
}

func (a *ArchiveWriter) create(name string) (io.Writer, error) {
	if a.names[name] {
		return nil, fmt.Errorf("the archive already has the entry %s", name)
	}
	a.names[name] = true
	return a.zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Now()})
}

// WriteHistory writes a chat's saved history.
func (a *ArchiveWriter) WriteHistory(key string, data []byte) error {
	if !ValidKey(key) {
		return errors.New("a chat's key cannot name an archive entry")
	}
	if len(data) > MaxHistoryEntry {
		return fmt.Errorf("the saved history of the chat %s is larger than an export can hold", key)
	}
	w, err := a.create(HistoryEntry(key))
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// WriteGroup writes a group's conversation, one message on each line.
func (a *ArchiveWriter) WriteGroup(key string, messages []GroupMessage) error {
	if !ValidKey(key) {
		return errors.New("a group's key cannot name an archive entry")
	}
	w, err := a.create(GroupEntry(key))
	if err != nil {
		return err
	}
	for _, m := range messages {
		line, err := json.Marshal(m)
		if err != nil {
			return err
		}
		if _, err := w.Write(append(line, '\n')); err != nil {
			return err
		}
	}
	return nil
}

// WriteFile writes a file attached to a group message, once however often
// it is attached, and returns its entry.
func (a *ArchiveWriter) WriteFile(data []byte) (string, error) {
	if len(data) > MaxFileEntry {
		return "", errors.New("the file is larger than an export can hold")
	}
	sum := sha256.Sum256(data)
	name := FileEntry(hex.EncodeToString(sum[:]))
	if a.names[name] {
		return name, nil
	}
	w, err := a.create(name)
	if err != nil {
		return "", err
	}
	_, err = w.Write(data)
	return name, err
}

// Finish writes the document and ends the archive. The document must refer
// only to entries that were written.
func (a *ArchiveWriter) Finish(export Export) error {
	for _, chat := range export.Chats {
		if !a.names[HistoryEntry(chat.Key)] {
			return fmt.Errorf("the chat %s has no saved history in the archive", chat.Key)
		}
	}
	for _, group := range export.Groups {
		if group.Messages > 0 && !a.names[GroupEntry(group.Key)] {
			return fmt.Errorf("the group %s has no conversation in the archive", group.Key)
		}
	}
	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > MaxExportDocument {
		return errors.New("the workspace's document is larger than an export can hold")
	}
	w, err := a.create(ArchiveDocument)
	if err != nil {
		return err
	}
	if _, err := w.Write(append(data, '\n')); err != nil {
		return err
	}
	return a.zw.Close()
}

// WriteArchiveFile writes an export to file through a temporary file beside
// it, which is moved into place only once write has finished, so a failed
// export never leaves a partly written file under the chosen name.
func WriteArchiveFile(file string, write func(io.Writer) error) error {
	tmp, err := os.CreateTemp(filepath.Dir(file), ".export-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err = write(tmp); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), file)
}

// Archive is an export file that has been opened and checked. Bare is set
// for a bare JSON document written by an earlier build, which has no
// entries.
type Archive struct {
	Export  Export
	Bare    bool
	file    *os.File
	entries map[string]*zip.File
}

// zipSignature starts a zip archive: a local file header, or the end of an
// archive with no entries.
func zipSignature(head []byte) bool {
	return bytes.HasPrefix(head, []byte("PK\x03\x04")) || bytes.HasPrefix(head, []byte("PK\x05\x06"))
}

// OpenArchive opens an export file, an archive or a bare document, and
// checks it as ReadArchive does. The caller closes it.
func OpenArchive(path string) (*Archive, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("the workspace export could not be read: %w", err)
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, ErrNotAnExport
	}
	a, err := ReadArchive(f, info.Size())
	if err != nil {
		f.Close()
		return nil, err
	}
	a.file = f
	return a, nil
}

// ReadArchive reads an export from r, of size bytes: a bare document of at
// most MaxExportDocument, or an archive. An archive's entries must have safe
// names, with no part that is empty, "." or "..", no "\" or ":", and no
// absolute path; no name may come twice; each entry must stay within its
// kind's limit and all of them within MaxArchiveContents; and the document
// must be there and read as ReadExport reads it, with an entry for each
// chat's history and each conversation it refers to.
func ReadArchive(r io.ReaderAt, size int64) (*Archive, error) {
	head := make([]byte, 4)
	n, _ := r.ReadAt(head, 0)
	if !zipSignature(head[:n]) {
		if size > MaxExportDocument {
			return nil, ErrNotAnExport
		}
		data := make([]byte, size)
		if _, err := r.ReadAt(data, 0); err != nil && !errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("the workspace export could not be read: %w", err)
		}
		export, err := ReadExport(data)
		if err != nil {
			return nil, err
		}
		return &Archive{Export: export, Bare: true, entries: map[string]*zip.File{}}, nil
	}
	zr, err := zip.NewReader(r, size)
	if err != nil && !errors.Is(err, zip.ErrInsecurePath) {
		return nil, ErrNotAnExport
	}
	if len(zr.File) > MaxArchiveEntries {
		return nil, errors.New("the workspace export has more entries than can be imported")
	}
	a := &Archive{entries: map[string]*zip.File{}}
	// Names are compared without regard to case, since a file system that
	// ignores case would extract two such entries to one file.
	folded := map[string]bool{}
	var total uint64
	for _, f := range zr.File {
		name := f.Name
		if strings.HasSuffix(name, "/") {
			if f.UncompressedSize64 != 0 || !safeEntryName(strings.TrimSuffix(name, "/")) {
				return nil, fmt.Errorf("the workspace export has an entry with an unsafe name: %q", name)
			}
			continue
		}
		if !safeEntryName(name) {
			return nil, fmt.Errorf("the workspace export has an entry with an unsafe name: %q", name)
		}
		if folded[strings.ToLower(name)] {
			return nil, fmt.Errorf("the workspace export has the entry %q twice", name)
		}
		folded[strings.ToLower(name)] = true
		if f.UncompressedSize64 > entryLimit(name) {
			return nil, fmt.Errorf("the workspace export's entry %q is larger than can be imported", name)
		}
		total += f.UncompressedSize64
		if total > MaxArchiveContents {
			return nil, errors.New("the workspace export is larger than can be imported")
		}
		a.entries[name] = f
	}
	data, err := a.read(ArchiveDocument, MaxExportDocument)
	if err != nil {
		return nil, ErrNotAnExport
	}
	if a.Export, err = ReadExport(data); err != nil {
		return nil, err
	}
	for _, chat := range a.Export.Chats {
		if a.entries[HistoryEntry(chat.Key)] == nil {
			return nil, fmt.Errorf("the workspace export has no saved history for the chat %q", chatName(chat))
		}
	}
	for _, group := range a.Export.Groups {
		if group.Messages > 0 && a.entries[GroupEntry(group.Key)] == nil {
			return nil, fmt.Errorf("the workspace export has no conversation for the group %q", group.Title)
		}
	}
	return a, nil
}

// chatName is how a message names a chat of an export.
func chatName(chat ExportChat) string {
	if chat.Title != "" {
		return chat.Title
	}
	if chat.JudgedTitle != "" {
		return chat.JudgedTitle
	}
	return chat.Key
}

// safeEntryName reports whether an entry's name could be extracted without
// leaving the folder it is extracted into, on any system.
func safeEntryName(name string) bool {
	if name == "" || len(name) > 512 || strings.ContainsAny(name, "\\:\x00") || strings.HasPrefix(name, "/") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return filepath.IsLocal(filepath.FromSlash(name))
}

// entryLimit is the largest an entry of the name may be.
func entryLimit(name string) uint64 {
	switch {
	case name == ArchiveDocument:
		return MaxExportDocument
	case strings.HasPrefix(name, archiveChats):
		return MaxHistoryEntry
	case strings.HasPrefix(name, archiveGroups):
		return MaxGroupEntry
	case strings.HasPrefix(name, archiveFiles):
		return MaxFileEntry
	}
	return MaxArchiveContents
}

// read reads an entry, refusing one that turns out larger than limit
// whatever its header said.
func (a *Archive) read(name string, limit int64) ([]byte, error) {
	f := a.entries[name]
	if f == nil {
		return nil, fmt.Errorf("the workspace export has no entry %s", name)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("the workspace export's entry %s could not be read: %w", name, err)
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if err != nil {
		return nil, fmt.Errorf("the workspace export's entry %s could not be read: %w", name, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("the workspace export's entry %s is larger than can be imported", name)
	}
	return data, nil
}

// Close closes the file the archive was opened from.
func (a *Archive) Close() error {
	if a.file == nil {
		return nil
	}
	return a.file.Close()
}

// History returns the saved history of the chat with the key, after
// checking that it is a JSON object whose session is the chat's newest
// stage.
func (a *Archive) History(key string) ([]byte, error) {
	var chat *ExportChat
	for i := range a.Export.Chats {
		if a.Export.Chats[i].Key == key {
			chat = &a.Export.Chats[i]
		}
	}
	if chat == nil {
		return nil, fmt.Errorf("the workspace export has no chat %s", key)
	}
	data, err := a.read(HistoryEntry(key), MaxHistoryEntry)
	if err != nil {
		return nil, err
	}
	var head struct {
		SessionID string `json:"sessionId"`
	}
	if json.Unmarshal(data, &head) != nil || head.SessionID != chat.Stages[len(chat.Stages)-1] {
		return nil, fmt.Errorf("the saved history of the chat %q in the workspace export is invalid", chatName(*chat))
	}
	return data, nil
}

// GroupMessages returns the conversation of the group with the key, after
// checking each message: an ID of its own above 0, references as the
// document's are checked, and attachments whose entry, when they have one,
// is in the archive under their SHA-256. The count must be the one the
// document gives.
func (a *Archive) GroupMessages(key string) ([]GroupMessage, error) {
	var group *ExportGroup
	for i := range a.Export.Groups {
		if a.Export.Groups[i].Key == key {
			group = &a.Export.Groups[i]
		}
	}
	if group == nil {
		return nil, fmt.Errorf("the workspace export has no group %s", key)
	}
	if group.Messages == 0 {
		return []GroupMessage{}, nil
	}
	f := a.entries[GroupEntry(key)]
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("the conversation of the group %q could not be read: %w", group.Title, err)
	}
	defer rc.Close()
	chats := map[string]string{}
	for _, chat := range a.Export.Chats {
		chats[chat.Key] = chat.ChatID
	}
	bad := func(why string) error {
		return fmt.Errorf("the conversation of the group %q in the workspace export %s", group.Title, why)
	}
	checkRef := func(ref *ChatRef) bool {
		return validIdentity(ref.ChatID) && (ref.Key == "" || chats[ref.Key] == ref.ChatID)
	}
	out := []GroupMessage{}
	ids := map[int64]bool{}
	scanner := bufio.NewScanner(io.LimitReader(rc, MaxGroupEntry+1))
	scanner.Buffer(make([]byte, 0, 64<<10), MaxExportDocument)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		if len(out) >= MaxGroupMessages || len(out) >= group.Messages {
			return nil, bad("has more messages than it says")
		}
		var m GroupMessage
		if err := json.Unmarshal(line, &m); err != nil {
			return nil, bad("has a line that cannot be read")
		}
		if m.ID <= 0 || ids[m.ID] {
			return nil, bad("has a message without an ID of its own")
		}
		ids[m.ID] = true
		if (m.OriginExporter == "") != (m.OriginID == 0) || m.OriginID < 0 || (m.OriginExporter != "" && !validIdentity(m.OriginExporter)) {
			return nil, bad("has a message whose origin cannot be read")
		}
		if m.From != nil && !checkRef(m.From) {
			return nil, bad("has a message from a chat it cannot refer to")
		}
		for i := range m.Mentions {
			if !checkRef(&m.Mentions[i]) {
				return nil, bad("mentions a chat it cannot refer to")
			}
		}
		for _, att := range m.Attachments {
			if !fileSum.MatchString(att.SHA256) || att.Size < 0 {
				return nil, bad("has an attachment that cannot be read")
			}
			if att.File != "" && (att.File != FileEntry(att.SHA256) || a.entries[att.File] == nil) {
				return nil, bad("refers to a file it does not have")
			}
		}
		out = append(out, m)
	}
	if err := scanner.Err(); err != nil {
		return nil, bad("could not be read: " + err.Error())
	}
	if len(out) != group.Messages {
		return nil, bad("has fewer messages than it says")
	}
	return out, nil
}

// File returns a file attached to a group message, by its entry, after
// checking that its content has the SHA-256 its name gives.
func (a *Archive) File(name string) ([]byte, error) {
	sum, ok := strings.CutPrefix(name, archiveFiles)
	if !ok || !fileSum.MatchString(sum) {
		return nil, fmt.Errorf("the workspace export has no file %s", name)
	}
	data, err := a.read(name, MaxFileEntry)
	if err != nil {
		return nil, err
	}
	got := sha256.Sum256(data)
	if hex.EncodeToString(got[:]) != sum {
		return nil, fmt.Errorf("the file %s in the workspace export is damaged", name)
	}
	return data, nil
}

// ExporterFile is the file in the state folder that keeps this Dinah's
// exporter identity.
const ExporterFile = "exporter.json"

// ExporterIdentity is the random identity of the Dinah whose state folder
// is stateDir, which an export records so that an import knows where its
// group messages came from. It is made on the first export and kept in its
// own file, which the window and the command line both reach through
// statefile.Update, so the two never make two identities. A file that
// cannot be read is replaced.
func ExporterIdentity(stateDir string) (string, error) {
	type record struct {
		Identity string `json:"identity"`
	}
	var id string
	err := statefile.Update(filepath.Join(stateDir, ExporterFile), func(previous []byte) ([]byte, error) {
		var saved record
		if previous != nil && json.Unmarshal(previous, &saved) == nil && validIdentity(saved.Identity) {
			id = saved.Identity
			return nil, nil
		}
		id = "exp_" + rand.Text()
		return json.MarshalIndent(record{Identity: id}, "", "  ")
	})
	if err != nil {
		return "", err
	}
	return id, nil
}
