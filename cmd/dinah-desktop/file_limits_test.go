package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"studio/internal/acp"
	"studio/internal/bus"
)

// Each provider starts with the limit its documentation leads to: Claude
// and Codex 20, with their sources, and Devin and Ollama none, with the
// sentence that their documentation gives none.
func TestFileLimitDefaultsComeFromTheDocumentation(t *testing.T) {
	views := fileLimitViews(Settings{})
	for id, want := range map[string]int{"claude": 20, "codex": 20, "devin": 0, "ollama": 0} {
		v := views[id]
		if v.Limit != want || v.Default != want || v.Chosen {
			t.Fatalf("%s starts with %+v", id, v)
		}
		if want == 0 && (v.Reason != "" || !strings.Contains(v.Source, "gives no limit")) {
			t.Fatalf("%s should say its documentation gives no limit: %+v", id, v)
		}
		if want > 0 && (v.SourceURL == "" || !strings.Contains(v.Reason, "Files per Message")) {
			t.Fatalf("%s should give its source and the setting: %+v", id, v)
		}
	}
	if !strings.Contains(views["claude"].Source, "600 images") || views["claude"].SourceURL != "https://platform.claude.com/docs/en/build-with-claude/vision" {
		t.Fatalf("Claude's source is %+v", views["claude"])
	}
	if !strings.Contains(views["codex"].Source, "1,500 images") || !strings.Contains(views["codex"].Source, "Paul's own choice") || views["codex"].Ceiling != 1500 {
		t.Fatalf("Codex's source is %+v", views["codex"])
	}
}

// A limit the person sets is saved, read back after a restart, and can be
// no limit (0) or returned to the default; one above what the provider
// documents, a negative one and an unknown provider are refused, and a
// hand-edited file with such values loses them.
func TestFileLimitsAreSavedAndChecked(t *testing.T) {
	a, _ := providerTestApp(t)
	if err := a.SetProviderFileLimit("claude", 5); err != nil {
		t.Fatal(err)
	}
	if err := a.SetProviderFileLimit("codex", 0); err != nil {
		t.Fatal(err)
	}
	if err := a.SetProviderFileLimit("devin", 3); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []struct {
		provider string
		limit    int
	}{{"claude", 601}, {"codex", 1501}, {"ollama", -1}, {"nobody", 2}} {
		if err := a.SetProviderFileLimit(bad.provider, bad.limit); err == nil {
			t.Fatalf("%s %d was accepted", bad.provider, bad.limit)
		}
	}
	saved, err := loadSettings()
	if err != nil {
		t.Fatal(err)
	}
	views := fileLimitViews(saved)
	if views["claude"].Limit != 5 || !views["claude"].Chosen || views["codex"].Limit != 0 || !views["codex"].Chosen || views["codex"].Reason != "" || views["devin"].Limit != 3 || views["ollama"].Limit != 0 {
		t.Fatalf("after a restart the limits are %+v", views)
	}
	if err := a.ResetProviderFileLimit("claude"); err != nil {
		t.Fatal(err)
	}
	if saved, _ = loadSettings(); fileLimitOf(saved, "claude") != 20 || fileLimitViews(saved)["claude"].Chosen {
		t.Fatalf("after Use Default, Claude's limit is %d", fileLimitOf(saved, "claude"))
	}
	if err := os.WriteFile(filepath.Join(configDir(), "settings.json"), []byte(`{"fileLimits":{"claude":-4,"codex":2000,"devin":7,"other":3}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if saved, err = loadSettings(); err != nil || len(saved.FileLimits) != 1 || saved.FileLimits["devin"] != 7 {
		t.Fatalf("a hand-edited file gives %v: %v", saved.FileLimits, err)
	}
}

// A single chat's message with more files than its provider takes is
// refused with the setting's name; the workspace documents attached to it
// are not counted.
func TestAChatRefusesMoreFilesThanItsProviderTakes(t *testing.T) {
	a, _ := providerTestApp(t)
	if err := a.SetProviderFileLimit("codex", 1); err != nil {
		t.Fatal(err)
	}
	s := newSessionState(&sessionSeed{ID: "codex:limit", Provider: "codex", Cwd: t.TempDir()})
	file := Attachment{Kind: "file", Name: "a.bin", MimeType: "application/octet-stream", Data: base64.StdEncoding.EncodeToString([]byte("x"))}
	_, err := a.blocks(s, "two", []Attachment{file, file})
	if err == nil || !strings.Contains(err.Error(), "Codex takes 1 file per message (Settings › Agents › Codex › Files per Message)") {
		t.Fatalf("two files: %v", err)
	}
	if _, err := a.blocks(s, "one", []Attachment{file, {Kind: "link", Name: "doc", URI: "file:///doc.md"}}); err != nil {
		t.Fatalf("one file and a link: %v", err)
	}
}

func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x += 7 {
		img.Set(x, x%h, color.NRGBA{R: 200, A: 255})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// A picture over 2000 px on its longer side is reduced to 2000 px for a
// Claude chat, with its proportions and its format kept; one within the
// size is left as it was.
func TestClaudePicturesAreReducedTo2000Pixels(t *testing.T) {
	big := pngOf(t, 3000, 1000)
	out, changed, w, h, err := fitPicture("image/png", big, claudeLongestSide)
	if err != nil || !changed || w != 3000 || h != 1000 {
		t.Fatalf("fitting the 3000x1000 picture: %v %v %dx%d", err, changed, w, h)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(out))
	if err != nil || cfg.Width != 2000 || cfg.Height != 667 {
		t.Fatalf("the reduced picture is %dx%d: %v", cfg.Width, cfg.Height, err)
	}
	small := pngOf(t, 1500, 900)
	if out, changed, _, _, err := fitPicture("image/png", small, claudeLongestSide); err != nil || changed || !bytes.Equal(out, small) {
		t.Fatalf("the 1500 px picture was changed: %v %v", changed, err)
	}
	if w, h := fittedSize(1000, 4000, 2000); w != 500 || h != 2000 {
		t.Fatalf("a tall picture fits as %dx%d", w, h)
	}

	var jbuf bytes.Buffer
	if err := jpeg.Encode(&jbuf, image.NewRGBA(image.Rect(0, 0, 2400, 1800)), nil); err != nil {
		t.Fatal(err)
	}
	out, changed, _, _, err = fitPicture("image/jpeg", jbuf.Bytes(), claudeLongestSide)
	if cfg, cerr := jpeg.DecodeConfig(bytes.NewReader(out)); err != nil || cerr != nil || !changed || cfg.Width != 2000 || cfg.Height != 1500 {
		t.Fatalf("the JPEG is %dx%d: %v %v", cfg.Width, cfg.Height, err, cerr)
	}

	pal := color.Palette{color.Transparent, color.Black, color.White}
	anim := &gif.GIF{Image: []*image.Paletted{image.NewPaletted(image.Rect(0, 0, 4000, 2000), pal), image.NewPaletted(image.Rect(1000, 500, 3000, 1500), pal)}, Delay: []int{10, 10}}
	var gbuf bytes.Buffer
	if err := gif.EncodeAll(&gbuf, anim); err != nil {
		t.Fatal(err)
	}
	out, changed, _, _, err = fitPicture("image/gif", gbuf.Bytes(), claudeLongestSide)
	if err != nil || !changed {
		t.Fatalf("the GIF: %v %v", changed, err)
	}
	g, err := gif.DecodeAll(bytes.NewReader(out))
	if err != nil || g.Config.Width != 2000 || g.Config.Height != 1000 || len(g.Image) != 2 || g.Image[1].Bounds() != image.Rect(500, 250, 1500, 750) {
		t.Fatalf("the GIF is %+v with %d frames: %v", g.Config, len(g.Image), err)
	}

	// Through a Claude chat's prompt, the image block is the reduced one;
	// a Codex chat is given the picture as it was.
	a, _ := providerTestApp(t)
	a.runtime.PutProviderLocked("claude", newTestProvider(nil, acp.InitializeResult{AgentCapabilities: acp.AgentCapabilities{PromptCapabilities: acp.PromptCapabilities{Image: true}}}))
	{
		initial := a.runtime.ProviderLocked("codex").Initialization()
		initial.AgentCapabilities.PromptCapabilities.Image = true
		setTestInitialization(a, "codex", initial)
	}
	att := Attachment{Kind: "file", Name: "wide.png", MimeType: "image/png", Data: base64.StdEncoding.EncodeToString(big)}
	for provider, want := range map[string]int{"claude": 2000, "codex": 3000} {
		blocks, err := a.blocks(newSessionState(&sessionSeed{ID: provider + ":p", Provider: provider, Cwd: t.TempDir()}), "", []Attachment{att})
		if err != nil || len(blocks) != 1 || blocks[0].Type != "image" {
			t.Fatalf("%s: %+v %v", provider, blocks, err)
		}
		data, _ := base64.StdEncoding.DecodeString(blocks[0].Data)
		if cfg, err := png.DecodeConfig(bytes.NewReader(data)); err != nil || cfg.Width != want {
			t.Fatalf("%s is given a picture %d px wide: %v", provider, cfg.Width, err)
		}
	}
}

// A group's limit is the smallest among its member chats that have one,
// with the member that set it; a member without a limit sets none. Its send
// refuses a message over that limit, and a delivery to a chat gives as
// images only as many pictures as that chat's provider takes, leaving the
// rest listed for read_attachment.
func TestGroupsHonourEachMembersLimit(t *testing.T) {
	a, _ := providerTestApp(t)
	{
		initial := a.runtime.ProviderLocked("codex").Initialization()
		initial.AgentCapabilities.PromptCapabilities.Image = true
		setTestInitialization(a, "codex", initial)
	}
	members := []BusGroupMember{{ID: bus.Human, State: "person"}, {ID: "b1", Chat: "claude:one", Title: "Reviewer"}, {ID: "b2", Chat: "codex:two", Title: "Builder"}, {ID: "b3", Chat: "ollama:three", Title: "Local"}}
	if err := a.SetProviderFileLimit("claude", 3); err != nil {
		t.Fatal(err)
	}
	if err := a.SetProviderFileLimit("codex", 5); err != nil {
		t.Fatal(err)
	}
	limit := a.groupFileLimit(members)
	if limit == nil || limit.Limit != 3 || limit.Member != "Reviewer" || limit.Provider != "claude" || !strings.Contains(limit.Reason, "Reviewer uses Claude, which takes 3 files per message (Settings › Agents › Claude › Files per Message)") {
		t.Fatalf("the group's limit is %+v", limit)
	}
	if limit := a.groupFileLimit(members[3:]); limit != nil {
		t.Fatalf("a group of an Ollama chat has the limit %+v", limit)
	}

	if err := a.SetProviderFileLimit("codex", 1); err != nil {
		t.Fatal(err)
	}
	info, err := a.NewSessionAt(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := a.session(info.ID)
	a.mu.Lock()
	a.settings.BusSessions = map[string]string{info.ID: "window-member"}
	a.mu.Unlock()
	shot := []byte("\x89PNG\r\n\x1a\nfixture")
	var group string
	var m bus.Message
	err = a.busHost().With(func(b *bus.Bus) error {
		ctx := context.Background()
		if _, err := b.RegisterSession(ctx, "window-member", s.Cwd(), "Docs Pass"); err != nil {
			return err
		}
		g, err := b.CreateGroup(ctx, bus.Sender{}, "Release", []string{"Docs Pass"})
		if err != nil {
			return err
		}
		group = g.Name
		// Sent as the bus allows, past the window's limit, as a message sent
		// before the limit was lowered would be.
		sent, err := b.SendFiles(ctx, "#"+g.Title, "Two shots.", []bus.PersonFile{{Name: "one.png", Mime: "image/png", Data: shot}, {Name: "two.png", Mime: "image/png", Data: shot}}, 0, []string{"Docs Pass"})
		if err != nil {
			return err
		}
		m, err = b.Message(ctx, sent.ID)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	room, err := a.BusGroup(group)
	if err != nil || room.FileLimit == nil || room.FileLimit.Limit != 1 || room.FileLimit.Member != "Docs Pass" {
		t.Fatalf("the group window is given the limit %+v: %v", room.FileLimit, err)
	}
	two := []Attachment{{Kind: "file", Name: "one.png", MimeType: "image/png", Data: base64.StdEncoding.EncodeToString(shot)}, {Kind: "file", Name: "two.png", MimeType: "image/png", Data: base64.StdEncoding.EncodeToString(shot)}}
	if _, err := a.BusSendToGroup(group, "", nil, two); err == nil || !strings.Contains(err.Error(), "Docs Pass uses Codex, which takes 1 file per message") {
		t.Fatalf("sending two files: %v", err)
	}
	p, err := a.busCoordinator().Prompt(s, []bus.Message{m}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Pictures()) != 1 || !strings.Contains(p.Text(), "attachment 1 of message") || !strings.Contains(p.Text(), "takes at most 1 file in one message, so these pictures are not in this prompt, and read_attachment reads them: attachment 2 of message") || !strings.Contains(p.Text(), "Attachment 2: two.png") {
		t.Fatalf("the prompt gives %d pictures and says:\n%s", len(p.Pictures()), p.Text())
	}
}
