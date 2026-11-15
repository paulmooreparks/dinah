//go:build quality_group_mentions

package main

// The tagged receiving host for Quality's group mentions checks
// (docs/review/group-mentions-independent-freeze-2026-11-16.md on
// quality/group-mentions). It holds a real App and bus with a group, a team
// with a known lead, member chats, an outside chat and an outside chat with a
// member's title, and answers the page's bridge calls over loopback HTTP with
// the App's own methods. Production routing, authorization, dispatch and
// persistence are not replaced: the host only observes them. For each send it
// records the preview the page asked for separately from the committed
// recipients (the deliveries the bus stored for each chat) and the actual
// dispatch attempts (the prompts the provider received after the watcher
// rang), so a reference that woke anyone would show.
//
//   go test -c -tags "desktop,production,quality_group_mentions" -o <dir>/group-mentions-host.exe ./cmd/dinah-desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"studio/internal/acp"
	"studio/internal/bus"
)

type groupMentionsChat struct {
	Role      string `json:"role"`
	ChatID    string `json:"chatId"`
	BusID     string `json:"busId"`
	Title     string `json:"title"`
	Workspace string `json:"workspace"`
}

type groupMentionsFixture struct {
	t        *testing.T
	a        *App
	c        *testProviderConnection
	artifact string
	mu       sync.Mutex
	chats    []groupMentionsChat
	group    string
	team     string
	document map[string]string
	events   []map[string]any
}

func newGroupMentionsFixture(t *testing.T, artifact string) *groupMentionsFixture {
	a, c := providerTestApp(t)
	x := &groupMentionsFixture{t: t, a: a, c: c, artifact: artifact}
	// One native session for each chat, in the order they are made below.
	c.newIDs = []string{"gm-docs", "gm-engine", "gm-lead", "gm-crew", "gm-sheet", "gm-docs-atlas"}
	root := t.TempDir()
	add := func(role, title, folder string) *Session {
		// Chats of one workspace share its folder.
		dir := filepath.Join(root, folder)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		info, err := a.NewSessionAt(dir)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := a.RenameSession(info.ID, title); err != nil {
			t.Fatal(err)
		}
		s := a.session(info.ID)
		x.chats = append(x.chats, groupMentionsChat{Role: role, ChatID: s.ID(), BusID: s.BusID(), Title: title, Workspace: a.chatWorkspaceName(s.Cwd())})
		return s
	}
	docs := add("member", "Docs Pass", "docs")
	eng := add("member", "Engine Work", "docs")
	lead := add("lead", "Team Lead", "docs")
	crew := add("team-member", "Build Crew", "docs")
	add("outside", "Sheet Helper", "harbor")
	add("outside-same-title", "Docs Pass", "atlas")
	ref := func(s *Session, title string) BusChatRef { return BusChatRef{ID: s.ID(), Cwd: s.Cwd(), Title: title} }
	g, err := a.BusStartGroup("Release Review", []BusChatRef{ref(docs, "Docs Pass"), ref(eng, "Engine Work")})
	if err != nil {
		t.Fatal(err)
	}
	x.group = g.Name
	// The team: Build Crew reports to Team Lead, which makes the lead's team channel.
	team, err := a.SetTeamLead(crew.ID(), lead.ID())
	if err != nil || team == "" {
		t.Fatalf("team %q %v", team, err)
	}
	x.team = team
	// One document open in the host's Editor, which a document reference
	// points to; the page is given this same draft in its document list.
	d, err := a.NewEmptyDraft("")
	if err != nil {
		t.Fatal(err)
	}
	x.document = map[string]string{"draftId": d.ID, "title": "release-notes.md"}
	return x
}

// pending is each chat's stored, undelivered bus messages, by chat ID.
func (x *groupMentionsFixture) pending() map[string]int {
	out := map[string]int{}
	_ = x.a.busHost().With(func(b *bus.Bus) error {
		for _, c := range x.chats {
			n, err := b.PendingSessionDeliveries(context.Background(), c.BusID)
			if err == nil {
				out[c.ChatID] = n
			}
		}
		return nil
	})
	return out
}

// prompts is how many prompts the provider has received for each chat.
func (x *groupMentionsFixture) prompts() map[string]int {
	out := map[string]int{}
	for _, call := range x.c.calls.Snapshot() {
		if call.method != "session/prompt" {
			continue
		}
		var id string
		switch p := call.params.(type) {
		case acp.PromptParams:
			id = p.SessionID
		case *acp.PromptParams:
			id = p.SessionID
		}
		if id != "" {
			out[id]++
		}
	}
	return out
}

func grew(before, after map[string]int) []string {
	var ids []string
	for id, n := range after {
		if n > before[id] {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func decodeArg[T any](raw []json.RawMessage, i int) (T, error) {
	var v T
	if i >= len(raw) {
		return v, fmt.Errorf("argument %d missing", i)
	}
	err := json.Unmarshal(raw[i], &v)
	return v, err
}

func (x *groupMentionsFixture) call(method string, args []json.RawMessage) map[string]any {
	result := func(v any, err error) map[string]any {
		if err != nil {
			return map[string]any{"error": err.Error()}
		}
		return map[string]any{"result": v}
	}
	switch method {
	case "fixture":
		return result(map[string]any{"chats": x.chats, "group": x.group, "team": x.team, "document": x.document}, nil)
	case "BusOverview":
		return result(x.a.BusOverview())
	case "Sessions":
		return result(x.a.Sessions())
	case "SavedChats":
		return result(x.a.SavedChats())
	case "AppletCatalog":
		return result(x.a.AppletCatalog())
	case "BusGroupMessages":
		key, err := decodeArg[string](args, 0)
		if err != nil {
			return result(nil, err)
		}
		return result(x.a.BusGroupMessages(key))
	case "BusMarkGroupRead":
		key, err := decodeArg[string](args, 0)
		if err != nil {
			return result(nil, err)
		}
		return result(nil, x.a.BusMarkGroupRead(key))
	case "BusGroupReach":
		key, err := decodeArg[string](args, 0)
		if err != nil {
			return result(nil, err)
		}
		ids, _ := decodeArg[[]string](args, 1)
		r, err := x.a.BusGroupReach(key, ids)
		x.mu.Lock()
		x.events = append(x.events, map[string]any{"kind": "preview", "group": key, "mentions": ids, "reach": r})
		x.mu.Unlock()
		return result(r, err)
	case "BusSendToGroup":
		key, err := decodeArg[string](args, 0)
		if err != nil {
			return result(nil, err)
		}
		text, _ := decodeArg[string](args, 1)
		mentions, _ := decodeArg[[]string](args, 2)
		files, _ := decodeArg[[]Attachment](args, 3)
		pendingBefore, promptsBefore := x.pending(), x.prompts()
		m, sendErr := x.a.BusSendToGroup(key, text, mentions, files)
		// Every member stores a group message to read; the committed recipients
		// are the ones the bus recorded the message as reaching, which it asks
		// to answer, by chat.
		stored := grew(pendingBefore, x.pending())
		var committed []string
		if sendErr == nil && m.Reach != nil {
			for _, r := range m.Reach.Reach {
				for _, c := range x.chats {
					if c.BusID == r.ID {
						committed = append(committed, c.ChatID)
					}
				}
			}
			sort.Strings(committed)
		}
		// The watcher rings every open chat, as the window's does, and the
		// prompts the provider receives are the actual dispatch attempts.
		ring(x.t, x.a)
		deadline := time.Now().Add(2 * time.Second)
		var dispatched []string
		for {
			dispatched = grew(promptsBefore, x.prompts())
			if len(dispatched) >= len(committed) || time.Now().After(deadline) {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		event := map[string]any{"kind": "send", "group": key, "text": text, "mentions": mentions, "files": files, "committedRecipients": committed, "storedDeliveries": stored, "dispatchAttempts": dispatched}
		if sendErr != nil {
			event["error"] = sendErr.Error()
		} else {
			event["messageId"] = m.ID
			event["reach"] = m.Reach
		}
		x.mu.Lock()
		x.events = append(x.events, event)
		x.mu.Unlock()
		if sendErr != nil {
			return result(nil, sendErr)
		}
		return map[string]any{"result": m, "committedRecipients": committed, "storedDeliveries": stored, "dispatchAttempts": dispatched}
	case "restart":
		// A new App over the same state reopens the committed data, as a
		// window restart does.
		x.a.busHost().CloseHandle()
		again := NewApp()
		again.settings.DefaultProvider = "codex"
		for _, id := range x.a.runtime.ProviderIDsLocked() {
			again.runtime.PutProviderLocked(id, x.a.runtime.ProviderLocked(id))
		}
		x.t.Cleanup(func() { again.busHost().CloseHandle() })
		x.a = again
		return result(true, nil)
	case "observations":
		x.mu.Lock()
		defer x.mu.Unlock()
		return result(map[string]any{"events": x.events, "pending": x.pending(), "prompts": x.prompts()}, nil)
	case "close":
		return result(true, nil)
	}
	return map[string]any{"error": "unknown method " + method}
}

func TestGroupMentionsAcceptanceServer(t *testing.T) {
	ready := os.Getenv("QUALITY_GROUP_MENTIONS_READY")
	if ready == "" {
		t.Skip("requires an isolated receiving driver")
	}
	artifact := os.Getenv("QUALITY_GROUP_MENTIONS_ARTIFACT")
	x := newGroupMentionsFixture(t, artifact)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closed := make(chan struct{})
	var once sync.Once
	var serial sync.Mutex
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Method string
			Args   []json.RawMessage
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<20)).Decode(&in); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		// One call at a time, so each send's before-and-after counts are its own.
		serial.Lock()
		out := x.call(in.Method, in.Args)
		serial.Unlock()
		_ = json.NewEncoder(w).Encode(out)
		if artifact != "" {
			if log, err := os.OpenFile(filepath.Join(artifact, "requests.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
				_ = json.NewEncoder(log).Encode(map[string]any{"request": in, "response": out})
				_ = log.Close()
			}
		}
		if in.Method == "close" {
			if artifact != "" {
				data, _ := json.Marshal(x.call("observations", nil))
				_ = os.WriteFile(filepath.Join(artifact, "final-observations.json"), data, 0o600)
			}
			once.Do(func() { close(closed) })
		}
	})
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Error(err)
		}
	}()
	if err := os.WriteFile(ready, []byte("http://"+listener.Addr().String()), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-closed:
	case <-time.After(5 * time.Minute):
		t.Error("receiving driver did not close")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

// The wake counter sees a real dispatch: a member mentioned in a group send
// is delivered to and prompted, and an outside chat is not.
func TestGroupMentionsHostCountersObserveRealDispatch(t *testing.T) {
	x := newGroupMentionsFixture(t, "")
	var member, memberBus, outside string
	for _, c := range x.chats {
		if c.Role == "member" && member == "" {
			member, memberBus = c.ChatID, c.BusID
		}
		if c.Role == "outside" {
			outside = c.ChatID
		}
	}
	raw := func(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
	out := x.call("BusSendToGroup", []json.RawMessage{raw(x.group), raw("Positive control"), raw([]string{memberBus}), raw([]Attachment{{Kind: "chat", SessionID: outside, Name: "Sheet Helper", Token: "@Sheet_Helper"}})})
	if out["error"] != nil {
		t.Fatal(out["error"])
	}
	committed, _ := out["committedRecipients"].([]string)
	dispatched, _ := out["dispatchAttempts"].([]string)
	stored, _ := out["storedDeliveries"].([]string)
	for _, id := range stored {
		if id == outside {
			t.Fatalf("the outside chat got a stored delivery: %v", stored)
		}
	}
	if len(committed) != 1 || committed[0] != member {
		t.Fatalf("committed %v, want only the member %s", committed, member)
	}
	if len(dispatched) != 1 || dispatched[0] != member {
		t.Fatalf("dispatched %v, want only the member %s", dispatched, member)
	}
}
