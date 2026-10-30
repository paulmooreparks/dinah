package main

import (
	"context"
	"encoding/base64"
	"net/http/httptest"
	"strings"
	"testing"

	"studio/internal/bus"
)

// groupMessageWithFiles sends the person's message with a picture and a
// text file to a group of one chat, and returns it as the bus delivers it.
func groupMessageWithFiles(t *testing.T, a *App, workspace string) bus.Message {
	t.Helper()
	png := []byte("\x89PNG\r\n\x1a\nfixture")
	var m bus.Message
	err := a.busHost().With(func(b *bus.Bus) error {
		ctx := context.Background()
		if _, err := b.RegisterSession(ctx, "window-member", workspace, "Docs Pass"); err != nil {
			return err
		}
		g, err := b.CreateGroup(ctx, bus.Sender{}, "Release", []string{"Docs Pass"})
		if err != nil {
			return err
		}
		sent, err := b.SendFiles(ctx, "#"+g.Title, "Look at these.", []bus.PersonFile{
			{Name: "shot.png", Mime: "image/png", Data: png},
			{Name: "notes.txt", Mime: "text/plain", Data: []byte("the notes")},
		}, 0, []string{"Docs Pass"})
		if err != nil {
			return err
		}
		m, err = b.Message(ctx, sent.ID)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// A group's message reaches a chat whose agent reads images with the
// person's picture as an image block, and with every file listed for
// read_attachment; the text file is never put in the prompt or the chat's
// workspace.
func TestAGroupPictureIsAnImageForAModelThatReadsImages(t *testing.T) {
	a, _ := providerTestApp(t)
	{
		initial := a.runtime.ProviderLocked("codex").Initialization()
		initial.AgentCapabilities.PromptCapabilities.Image = true
		setTestInitialization(a, "codex", initial)
	}
	{
		initial := a.runtime.ProviderLocked("codex").Initialization()
		initial.AgentCapabilities.PromptCapabilities.EmbeddedContext = true
		setTestInitialization(a, "codex", initial)
	}
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := a.session(info.ID)
	m := groupMessageWithFiles(t, a, s.Cwd())
	p, err := a.busCoordinator().Prompt(s, []bus.Message{m}, false)
	if err != nil {
		t.Fatal(err)
	}
	var images, texts int
	for _, b := range p.Blocks() {
		switch b.Type {
		case "image":
			images++
			if b.MimeType != "image/png" || b.Data != base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\nfixture")) {
				t.Fatalf("the image block is %+v", b)
			}
		case "text":
			texts++
		default:
			t.Fatalf("the prompt has a %s block: %+v", b.Type, b)
		}
	}
	if images != 1 || texts != 1 || len(p.Pictures()) != 1 {
		t.Fatalf("the prompt has %d images and %d texts", images, texts)
	}
	for _, want := range []string{"Attachment 1: shot.png (image/png", "Attachment 2: notes.txt (text/plain, 9 bytes", "read_attachment tool (message_id", "are in this prompt as images, in this order: attachment 1 of message"} {
		if !strings.Contains(p.Text(), want) {
			t.Fatalf("the prompt does not say %q:\n%s", want, p.Text())
		}
	}
}

// For a chat whose agent does not read images, the picture is listed with
// the other files, by name, type and size, for read_attachment, and the
// prompt is text alone.
func TestAGroupPictureIsListedForAModelThatCannotReadImages(t *testing.T) {
	a, _ := providerTestApp(t)
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := a.session(info.ID)
	m := groupMessageWithFiles(t, a, s.Cwd())
	p, err := a.busCoordinator().Prompt(s, []bus.Message{m}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Blocks()) != 1 || p.Blocks()[0].Type != "text" || len(p.Pictures()) != 0 {
		t.Fatalf("the prompt is %+v", p.Blocks())
	}
	if !strings.Contains(p.Text(), "Attachment 1: shot.png (image/png") || strings.Contains(p.Text(), "as images") {
		t.Fatalf("the prompt says:\n%s", p.Text())
	}
}

// The group window's send takes the files as a single chat's box gives
// them, refuses one over the limit with the reason, and draws the message
// with each picture's address and each file's address for opening it.
// Settings counts the files older than an age, and removing them leaves
// the message with its attachments shown as removed.
func TestTheWindowSendsShowsAndRemovesTheFiles(t *testing.T) {
	a, _ := providerTestApp(t)
	var group string
	err := a.busHost().With(func(b *bus.Bus) error {
		ctx := context.Background()
		if _, err := b.RegisterSession(ctx, "window-member", t.TempDir(), "Docs Pass"); err != nil {
			return err
		}
		g, err := b.CreateGroup(ctx, bus.Sender{}, "Release", []string{"Docs Pass"})
		group = g.Name
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	encode := func(data []byte) string { return base64.StdEncoding.EncodeToString(data) }
	if _, err := a.BusSendToGroup(group, "Too big", nil, []Attachment{{Kind: "file", Name: "huge.bin", Data: encode(make([]byte, maxAttachmentFile+1))}}); err == nil || !strings.Contains(err.Error(), "huge.bin is larger than 25 MB") {
		t.Fatalf("the large file: %v", err)
	}
	png := []byte("\x89PNG\r\n\x1a\nfixture")
	m, err := a.BusSendToGroup(group, "", nil, []Attachment{
		{Kind: "file", Name: "shot.png", MimeType: "image/png", Data: encode(png)},
		{Kind: "file", Name: "notes.txt", MimeType: "text/plain", Data: encode([]byte("words"))},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Attachments) != 2 || !m.Attachments[0].Picture || m.Attachments[0].Src == "" || m.Attachments[1].Picture || !strings.HasPrefix(m.Attachments[1].URI, "file:///") {
		t.Fatalf("the window is given %+v", m.Attachments)
	}
	get := func(src string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		a.docFiles().ServeHTTP(w, httptest.NewRequest("GET", src, nil))
		return w
	}
	if w := get(m.Attachments[0].Src); w.Code != 200 || w.Body.String() != string(png) || w.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("the picture was served as %d %q", w.Code, w.Header().Get("Content-Type"))
	}
	if w := get(busAttachmentSrc(m.ID, 2)); w.Code != 404 {
		t.Fatalf("a file that is not a picture was served: %d", w.Code)
	}
	if n, err := a.OldAttachments(1); err != nil || n.Files != 0 {
		t.Fatalf("files older than a day: %+v %v", n, err)
	}
	if n, err := a.OldAttachments(0); err != nil || n.Files != 2 || n.Bytes != int64(len(png)+5) {
		t.Fatalf("files of any age: %+v %v", n, err)
	}
	if n, err := a.RemoveOldAttachments(0); err != nil || n.Files != 2 {
		t.Fatalf("removed %+v: %v", n, err)
	}
	ms, err := a.BusGroupMessages(group)
	if err != nil || len(ms) == 0 {
		t.Fatal(err)
	}
	last := ms[len(ms)-1]
	if last.ID != m.ID || len(last.Attachments) != 2 || !last.Attachments[0].Removed || last.Attachments[0].Src != "" || last.Attachments[1].URI != "" {
		t.Fatalf("after removing, the message is %+v", last)
	}
	if w := get(m.Attachments[0].Src); w.Code != 404 {
		t.Fatalf("a removed picture was served: %d", w.Code)
	}
}

// A picture the person has removed is listed as removed and never given as
// an image.
func TestARemovedGroupPictureIsNotGiven(t *testing.T) {
	a, _ := providerTestApp(t)
	{
		initial := a.runtime.ProviderLocked("codex").Initialization()
		initial.AgentCapabilities.PromptCapabilities.Image = true
		setTestInitialization(a, "codex", initial)
	}
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := a.session(info.ID)
	m := groupMessageWithFiles(t, a, s.Cwd())
	if _, err := a.RemoveOldAttachments(0); err != nil {
		t.Fatal(err)
	}
	a.busHost().With(func(b *bus.Bus) error { m, _ = b.Message(context.Background(), m.ID); return nil })
	p, err := a.busCoordinator().Prompt(s, []bus.Message{m}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Pictures()) != 0 || !strings.Contains(p.Text(), "has since removed it") {
		t.Fatalf("the prompt gives %d pictures and says:\n%s", len(p.Pictures()), p.Text())
	}
}
