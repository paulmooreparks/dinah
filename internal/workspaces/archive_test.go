package workspaces

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// sampleChatsExport is a document with two chats, a group whose member in
// another workspace has no key, a team, a chat pin and fields this build
// does not know on each new record.
func sampleChatsExport() Export {
	return Export{Format: ExportFormat, Name: "Harbor", Exporter: "exp_sample",
		Sources: []ExportSource{{Key: "s1", Kind: LocalFolder, Name: "Harbor", Main: true, Local: &ExportLocation{Absolute: "C:/Harbor"}}},
		Pins:    []ExportPin{{Kind: "chat", Title: "Harbor Lead", Chat: "c1"}, {Kind: "chat", Title: "Gone", Chat: "c9"}},
		Chats: []ExportChat{
			{Key: "c1", ChatID: "chat-lead", Stages: []string{"old-lead", "new-lead"}, Provider: "claude", Options: map[string]json.RawMessage{"model": json.RawMessage(`"opus"`)}, Model: "Opus", Title: "Harbor Lead", Extra: map[string]json.RawMessage{"later": json.RawMessage(`{"a":1}`)}},
			{Key: "c2", ChatID: "chat-review", Stages: []string{"review"}, Provider: "codex", JudgedTitle: "Checkout Review", Hidden: true, Folder: "Checkout/Screens"},
		},
		Groups: []ExportGroup{{Key: "g1", Title: "Harbor Team", Lead: &ChatRef{ChatID: "chat-lead", Key: "c1", Title: "Harbor Lead", Workspace: "Harbor"}, Budget: 5, Person: true, Messages: 2,
			Members: []ChatRef{{ChatID: "chat-lead", Key: "c1", Title: "Harbor Lead", Workspace: "Harbor"}, {ChatID: "chat-docks", Title: "Docks Builder", Workspace: "Docks", Extra: map[string]json.RawMessage{"later": json.RawMessage(`true`)}}}}},
		Teams: []ExportTeam{{Lead: ChatRef{ChatID: "chat-lead", Key: "c1", Title: "Harbor Lead", Workspace: "Harbor"},
			Members: []ExportTeamMember{{Chat: ChatRef{ChatID: "chat-review", Key: "c2", Title: "Checkout Review", Workspace: "Harbor"}, Since: "2026-11-01T00:00:00Z"}},
			Budget:  []ExportAllowance{{Provider: "claude", Unit: "tokens", Allowance: 1000}}}},
	}
}

func writeSampleArchive(t *testing.T, export Export) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := NewArchiveWriter(&buf)
	if err := w.WriteHistory("c1", []byte(`{"sessionId":"new-lead","messages":[]}`)); err != nil {
		t.Fatal(err)
	}
	if err := w.WriteHistory("c2", []byte(`{"sessionId":"review","messages":[]}`)); err != nil {
		t.Fatal(err)
	}
	file, err := w.WriteFile([]byte("plan"))
	if err != nil {
		t.Fatal(err)
	}
	again, _ := w.WriteFile([]byte("plan"))
	if again != file {
		t.Fatalf("a file attached twice was written twice: %s %s", file, again)
	}
	sum := strings.TrimPrefix(file, "files/")
	if err := w.WriteGroup("g1", []GroupMessage{
		{ID: 7, CreatedAt: "2026-11-17T21:04:00Z", Kind: "text", Person: true, Text: "Plan the checkout review.", Mentions: []ChatRef{{ChatID: "chat-lead", Key: "c1"}}},
		{ID: 9, CreatedAt: "2026-11-17T21:06:00Z", Kind: "text", From: &ChatRef{ChatID: "chat-docks", Title: "Docks Builder", Workspace: "Docks"}, Text: "Done.", ReplyTo: 7,
			Attachments: []GroupAttachment{{Name: "plan.txt", Size: 4, SHA256: sum, File: file}, {Name: "gone.txt", Size: 3, SHA256: strings.Repeat("0", 64)}}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := w.Finish(export); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// An archive written with chats, a group and a team reads back as it was
// written: the document, each history, the conversation and its file, with
// the fields this build does not know kept and a chat pin to a chat that is
// not there dropped.
func TestArchiveRoundTrip(t *testing.T) {
	data := writeSampleArchive(t, sampleChatsExport())
	path := filepath.Join(t.TempDir(), "Harbor"+ExportExtension)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := OpenArchive(path)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	e := a.Export
	if a.Bare || e.Name != "Harbor" || e.Exporter != "exp_sample" || len(e.Chats) != 2 || len(e.Groups) != 1 || len(e.Teams) != 1 {
		t.Fatalf("document: %+v", e)
	}
	if len(e.Pins) != 1 || e.Pins[0].Chat != "c1" {
		t.Fatalf("chat pins: %+v", e.Pins)
	}
	if c := e.Chats[0]; c.ChatID != "chat-lead" || strings.Join(c.Stages, ",") != "old-lead,new-lead" || string(c.Options["model"]) != `"opus"` || !SameExtra(c.Extra, map[string]json.RawMessage{"later": json.RawMessage(`{"a":1}`)}) {
		t.Fatalf("chat: %+v", c)
	}
	if c := e.Chats[1]; !c.Hidden || c.Folder != "Checkout/Screens" || c.JudgedTitle != "Checkout Review" {
		t.Fatalf("second chat: %+v", c)
	}
	if g := e.Groups[0]; g.Budget != 5 || !g.Person || g.Lead.Key != "c1" || g.Members[1].Key != "" || g.Members[1].Workspace != "Docks" || string(g.Members[1].Extra["later"]) != "true" {
		t.Fatalf("group: %+v", g)
	}
	if tm := e.Teams[0]; tm.Members[0].Chat.Key != "c2" || tm.Budget[0].Allowance != 1000 {
		t.Fatalf("team: %+v", tm)
	}
	history, err := a.History("c1")
	if err != nil || !strings.Contains(string(history), "new-lead") {
		t.Fatalf("history: %s %v", history, err)
	}
	messages, err := a.GroupMessages("g1")
	if err != nil || len(messages) != 2 || !messages[0].Person || messages[1].From.ChatID != "chat-docks" || messages[1].ReplyTo != 7 {
		t.Fatalf("messages: %+v %v", messages, err)
	}
	file, err := a.File(messages[1].Attachments[0].File)
	if err != nil || string(file) != "plan" || messages[1].Attachments[1].File != "" {
		t.Fatalf("file: %q %v %+v", file, err, messages[1].Attachments)
	}
	// Written again, the document keeps the fields this build does not know.
	again, _ := json.Marshal(e)
	if !strings.Contains(string(again), `"later":{"a":1}`) || !strings.Contains(string(again), `"later":true`) {
		t.Fatalf("written again, the unknown fields were lost: %s", again)
	}
}

// A bare JSON export written by an earlier build still reads, as a document
// with no entries.
func TestArchiveReadsABareExport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Old"+ExportExtension)
	bare := `{"format":1,"name":"Orchard","sources":[{"key":"s1","kind":"local-folder","name":"Orchard","main":true,"local":{"absolute":"C:/Orchard"}}],"pins":[{"kind":"chat","title":"Old chat pin"}]}`
	if err := os.WriteFile(path, []byte(bare), 0o600); err != nil {
		t.Fatal(err)
	}
	a, err := OpenArchive(path)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if !a.Bare || a.Export.Name != "Orchard" || len(a.Export.Sources) != 1 || len(a.Export.Pins) != 0 || len(a.Export.Chats) != 0 {
		t.Fatalf("bare export: %+v", a)
	}
	if err := os.WriteFile(path, []byte("not an export"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenArchive(path); err != ErrNotAnExport {
		t.Fatalf("a file that is not an export: %v", err)
	}
}

// zipWith writes an archive whose entries are given by name, in order.
func zipWith(t *testing.T, entries ...[2]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.Create(e[0])
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(e[1]))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// An archive with an entry whose name could leave the folder it is
// extracted into is refused whole, and so is one with a name twice, an
// entry over its limit, a chat without its history, or a document that is
// not there. An entry this build does not know, with a safe name, is passed
// over.
func TestArchiveRefusesUnsafeEntries(t *testing.T) {
	doc := `{"format":1,"name":"Harbor","sources":[]}`
	read := func(data []byte) error {
		_, err := ReadArchive(bytes.NewReader(data), int64(len(data)))
		return err
	}
	for _, name := range []string{"../escape.txt", "chats/../../escape.json", "/abs.json", `chats\c1.json`, "C:/x.json", "files/./x", "a//b", "files/\x00"} {
		if err := read(zipWith(t, [2]string{ArchiveDocument, doc}, [2]string{name, "x"})); err == nil || !strings.Contains(err.Error(), "unsafe name") {
			t.Errorf("the entry %q was not refused: %v", name, err)
		}
	}
	if err := read(zipWith(t, [2]string{ArchiveDocument, doc}, [2]string{"later/passphrase", "x"})); err != nil {
		t.Errorf("an unknown entry with a safe name was refused: %v", err)
	}
	if err := read(zipWith(t, [2]string{ArchiveDocument, doc}, [2]string{"Workspace.JSON", doc})); err == nil || !strings.Contains(err.Error(), "twice") {
		t.Errorf("a name that comes twice was not refused: %v", err)
	}
	if err := read(zipWith(t, [2]string{ArchiveDocument, doc}, [2]string{"files/" + strings.Repeat("a", 64), strings.Repeat("x", MaxFileEntry+1)})); err == nil || !strings.Contains(err.Error(), "larger") {
		t.Errorf("an entry over its limit was not refused: %v", err)
	}
	withChat := `{"format":1,"name":"Harbor","sources":[],"chats":[{"key":"c1","chatId":"x","stages":["x"],"provider":"claude"}]}`
	if err := read(zipWith(t, [2]string{ArchiveDocument, withChat})); err == nil || !strings.Contains(err.Error(), "no saved history") {
		t.Errorf("a chat without its history was not refused: %v", err)
	}
	if err := read(zipWith(t, [2]string{"chats/c1.json", "{}"})); err != ErrNotAnExport {
		t.Errorf("an archive without its document: %v", err)
	}
	badKey := `{"format":1,"name":"Harbor","sources":[],"chats":[{"key":"../c1","chatId":"x","stages":["x"],"provider":"claude"}]}`
	if err := read(zipWith(t, [2]string{ArchiveDocument, badKey})); err == nil {
		t.Error("a chat whose key could leave the archive's folder was not refused")
	}
	badRef := `{"format":1,"name":"Harbor","sources":[],"chats":[{"key":"c1","chatId":"x","stages":["x"],"provider":"claude"}],"groups":[{"key":"g1","title":"T","members":[{"chatId":"y","key":"c1"}]}]}`
	if err := read(zipWith(t, [2]string{ArchiveDocument, badRef}, [2]string{"chats/c1.json", `{"sessionId":"x"}`})); err == nil {
		t.Error("a reference whose key is another chat's was not refused")
	}
	badFolder := `{"format":1,"name":"Harbor","sources":[],"chats":[{"key":"c1","chatId":"x","stages":["x"],"provider":"claude","folder":"../out"}]}`
	if err := read(zipWith(t, [2]string{ArchiveDocument, badFolder}, [2]string{"chats/c1.json", `{"sessionId":"x"}`})); err == nil {
		t.Error("a chat folder outside the main folder was not refused")
	}
}

// A history whose session is not the chat's newest stage, and a group
// conversation with more or fewer messages than its document says, are
// refused when read.
func TestArchiveChecksWhatItReads(t *testing.T) {
	doc := `{"format":1,"name":"Harbor","sources":[],"chats":[{"key":"c1","chatId":"x","stages":["a","b"],"provider":"claude"}],"groups":[{"key":"g1","title":"T","budget":-1,"members":[{"chatId":"x","key":"c1"}],"messages":2}]}`
	data := zipWith(t, [2]string{ArchiveDocument, doc}, [2]string{"chats/c1.json", `{"sessionId":"a"}`}, [2]string{"groups/g1.jsonl", `{"id":1,"kind":"text","text":"one"}` + "\n"})
	a, err := ReadArchive(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.History("c1"); err == nil {
		t.Error("a history of an older stage was accepted")
	}
	if _, err := a.GroupMessages("g1"); err == nil || !strings.Contains(err.Error(), "fewer") {
		t.Errorf("a short conversation was accepted: %v", err)
	}
}

// The exporter identity is made once and then kept, even when two callers
// ask for it at the same moment.
func TestExporterIdentityIsMadeOnce(t *testing.T) {
	dir := t.TempDir()
	ids := make([]string, 8)
	var wg sync.WaitGroup
	for i := range ids {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id, err := ExporterIdentity(dir)
			if err != nil {
				t.Error(err)
			}
			ids[i] = id
		}(i)
	}
	wg.Wait()
	for _, id := range ids {
		if id == "" || id != ids[0] {
			t.Fatalf("the identity was made more than once: %v", ids)
		}
	}
	again, err := ExporterIdentity(dir)
	if err != nil || again != ids[0] {
		t.Fatalf("the identity changed: %s %s %v", ids[0], again, err)
	}
}
