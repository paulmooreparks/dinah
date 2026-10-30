package mcpserver

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"studio/internal/bus"
)

// read_attachment returns a picture the person attached as MCP image
// content, a text file as its text, and says when the person has removed a
// file. A chat the message did not reach is refused.
func TestReadAttachmentGivesThePersonsFiles(t *testing.T) {
	ctx := context.Background()
	b, err := bus.OpenAt(filepath.Join(t.TempDir(), "bus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	chat, _ := b.RegisterSession(ctx, "window-1", t.TempDir(), "First")
	other, _ := b.RegisterSession(ctx, "window-2", t.TempDir(), "Second")
	png := []byte("\x89PNG\r\n\x1a\nfixture")
	m, err := b.SendFiles(ctx, "First", "Look.", []bus.PersonFile{{Name: "shot.png", Mime: "image/png", Data: png}, {Name: "notes.txt", Data: []byte("plain words")}, {Name: "blob.bin", Data: []byte{0xff, 0xfe, 0x00}}}, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	read := func(index int) *mcp.CallToolResult {
		a, err := b.ReadAttachment(ctx, chat.ID, m.ID, index)
		if err != nil {
			t.Fatal(err)
		}
		return attachmentResult(a)
	}
	pic := read(1)
	if len(pic.Content) != 2 {
		t.Fatalf("the picture gave %d parts", len(pic.Content))
	}
	if img, ok := pic.Content[1].(*mcp.ImageContent); !ok || img.MIMEType != "image/png" || string(img.Data) != string(png) {
		t.Fatalf("the picture gave %#v", pic.Content[1])
	}
	if words := read(2).Content[0].(*mcp.TextContent).Text; !strings.HasSuffix(words, "plain words") {
		t.Fatalf("the text file gave %q", words)
	}
	if words := read(3).Content[0].(*mcp.TextContent).Text; !strings.Contains(words, "are not text") {
		t.Fatalf("the binary file gave %q", words)
	}
	if _, err := b.ReadAttachment(ctx, other.ID, m.ID, 1); err == nil {
		t.Fatal("a chat the message did not reach read its picture")
	}
	if _, err := b.RemovePersonFiles(ctx, time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if words := read(1).Content[0].(*mcp.TextContent).Text; !strings.Contains(words, "removed by the person") {
		t.Fatalf("the removed picture gave %q", words)
	}
}
