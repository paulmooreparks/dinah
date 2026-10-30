package bus

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The person's files are copied into the attachment folder beside the bus,
// under the date, and recorded with their names, types and hashes. Every
// member chat of the group may read them, and a chat outside it may not.
func TestThePersonsFilesAreKeptAndReachTheGroup(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	b.now = func() time.Time { return time.Date(2026, 10, 28, 9, 0, 0, 0, time.Local) }
	studio, pudl := workspace(t, root, "studio"), workspace(t, root, "pudl")
	lead, _ := b.RegisterSession(ctx, "window-lead", studio, "Lead")
	docs, _ := b.RegisterSession(ctx, "window-docs", pudl, "Docs Pass")
	outside, _ := b.RegisterSession(ctx, "window-outside", pudl, "Somebody Else")
	g, err := b.CreateGroup(ctx, Sender{}, "Release", []string{"Lead", "Docs Pass"})
	if err != nil {
		t.Fatal(err)
	}
	png := []byte("\x89PNG\r\n\x1a\nfixture")
	notes := []byte("the notes")
	m, err := b.SendFiles(ctx, "#"+g.Title, "Have a look.", []PersonFile{
		{Name: "shot.png", Mime: "image/png", Data: png},
		{Name: `C:\Users\paul\notes:draft?.md`, Data: notes},
		{Name: "shot.png", Data: png},
	}, 0, []string{"Docs Pass"})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Attachments) != 3 {
		t.Fatalf("attachments %+v", m.Attachments)
	}
	day := filepath.Join(root, "attachments", "2026-10-28")
	first, second, third := m.Attachments[0], m.Attachments[1], m.Attachments[2]
	if first.Path != filepath.Join(day, "shot.png") || third.Path != filepath.Join(day, "shot (2).png") || third.Rel != "shot.png" {
		t.Fatalf("the copies are %s and %s", first.Path, third.Path)
	}
	if second.Rel != "notes_draft_.md" || second.Mime != "text/markdown; charset=utf-8" && second.Mime != "text/markdown" && second.Mime != "text/plain" {
		t.Fatalf("the notes are %+v", second)
	}
	sum := sha256.Sum256(png)
	if !first.Stored || first.Mime != "image/png" || first.Size != int64(len(png)) || first.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("the picture is %+v", first)
	}
	if data, err := os.ReadFile(first.Path); err != nil || !bytes.Equal(data, png) {
		t.Fatalf("the copy holds %q: %v", data, err)
	}
	for _, s := range []Session{lead, docs} {
		got, err := b.ReadAttachment(ctx, s.ID, m.ID, 1)
		if err != nil || !bytes.Equal(got.Data, png) || got.Changed || got.Missing {
			t.Fatalf("%s read %+v: %v", s.Title, got, err)
		}
	}
	if _, err := b.ReadAttachment(ctx, outside.ID, m.ID, 1); err == nil {
		t.Fatal("a chat outside the group read the person's file")
	}
	text := FormatOne(m, "Paul")
	if !strings.Contains(text, fmt.Sprintf("Attachment 1: shot.png (image/png, %d bytes", len(png))) || !strings.Contains(text, "read_attachment tool (message_id") {
		t.Fatalf("the agent is told %s", text)
	}
}

// A message the person sends to one chat with a file reaches that chat,
// which may read the file; another chat may not.
func TestThePersonsDirectMessageCarriesFiles(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	one, _ := b.RegisterSession(ctx, "window-one", workspace(t, root, "alpha"), "First")
	two, _ := b.RegisterSession(ctx, "window-two", workspace(t, root, "beta"), "Second")
	m, err := b.SendFiles(ctx, "First", "", []PersonFile{{Name: "data.csv", Data: []byte("a,b\n1,2\n")}}, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := b.ReadAttachment(ctx, one.ID, m.ID, 1); err != nil || got.Text != "a,b\n1,2\n" {
		t.Fatalf("the chat sent to read %+v: %v", got, err)
	}
	if _, err := b.ReadAttachment(ctx, two.ID, m.ID, 1); err == nil {
		t.Fatal("a chat the message did not reach read its file")
	}
	if _, err := b.SendFiles(ctx, "First", "  ", nil, 0, nil); err == nil {
		t.Fatal("a message with neither words nor files was sent")
	}
	if _, err := b.Send(ctx, Sender{}, "First", "see", []string{"data.csv"}, 0); err == nil {
		t.Fatal("the person linked a file as if from a workspace")
	}
}

// A file over the limit is refused with the reason, and nothing of the
// message is sent or kept.
func TestAFileOverTheLimitIsRefused(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	b.RegisterSession(ctx, "window-one", workspace(t, root, "alpha"), "First")
	_, err := b.SendFiles(ctx, "First", "two files", []PersonFile{
		{Name: "small.txt", Data: []byte("fine")},
		{Name: "huge.bin", Data: make([]byte, MaxAttachmentFile+1)},
	}, 0, nil)
	if err == nil || !strings.Contains(err.Error(), "huge.bin is larger than 25 MB") {
		t.Fatalf("the large file: %v", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(root, "attachments")); len(entries) != 0 {
		t.Fatalf("something was kept: %v", entries)
	}
	if ms, _ := b.queryMessages(ctx, messageSelect); len(ms) != 0 {
		t.Fatalf("a message was sent: %+v", ms)
	}
}

// Removing the files older than an age removes only their copies: the
// messages stay, each attachment is marked removed, and a chat reading one
// is told it was removed. Newer files are kept.
func TestRemovingOldFilesKeepsTheirMessages(t *testing.T) {
	ctx := context.Background()
	b, root := open(t)
	one, _ := b.RegisterSession(ctx, "window-one", workspace(t, root, "alpha"), "First")
	old := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	b.now = func() time.Time { return old }
	early, err := b.SendFiles(ctx, "First", "old", []PersonFile{{Name: "a.txt", Data: []byte("12345")}, {Name: "b.txt", Data: []byte("123")}}, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	b.now = func() time.Time { return old.AddDate(0, 1, 0) }
	late, err := b.SendFiles(ctx, "First", "new", []PersonFile{{Name: "c.txt", Data: []byte("1")}}, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	cutoff := old.AddDate(0, 0, 7)
	if n, err := b.PersonFilesBefore(ctx, cutoff); err != nil || n.Files != 2 || n.Bytes != 8 {
		t.Fatalf("before removing: %+v %v", n, err)
	}
	n, err := b.RemovePersonFiles(ctx, cutoff)
	if err != nil || n.Files != 2 || n.Bytes != 8 {
		t.Fatalf("removed %+v: %v", n, err)
	}
	if _, err := os.Stat(early.Attachments[0].Path); !os.IsNotExist(err) {
		t.Fatalf("the old copy is still there: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(early.Attachments[0].Path)); !os.IsNotExist(err) {
		t.Fatalf("the emptied date folder is still there: %v", err)
	}
	if _, err := os.Stat(late.Attachments[0].Path); err != nil {
		t.Fatalf("the newer copy went too: %v", err)
	}
	m, err := b.Message(ctx, early.ID)
	if err != nil || m.Text != "old" || len(m.Attachments) != 2 || !m.Attachments[0].Removed || !m.Attachments[1].Removed {
		t.Fatalf("the old message is %+v: %v", m, err)
	}
	got, err := b.ReadAttachment(ctx, one.ID, early.ID, 1)
	if err != nil || !got.Removed {
		t.Fatalf("reading a removed file: %+v %v", got, err)
	}
	if !strings.Contains(FormatOne(m, "Paul"), "has since removed it") {
		t.Fatalf("the agent is told %s", FormatOne(m, "Paul"))
	}
	if _, err := b.PersonFile(ctx, early.ID, 1); err == nil {
		t.Fatal("a removed file is still offered to the window")
	}
	if a, err := b.PersonFile(ctx, late.ID, 1); err != nil || a.Rel != "c.txt" {
		t.Fatalf("the kept file: %+v %v", a, err)
	}
	if n, _ := b.PersonFilesBefore(ctx, cutoff); n.Files != 0 {
		t.Fatalf("after removing, %d are left", n.Files)
	}
}
