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
// recipients (the reach the bus stored with the message), the message's own
// delivery rows with their states, and the actual dispatch attempts (every
// prompt the provider received, counted with repeats, once the watcher's work
// has settled), so a reference that woke anyone, or a member prompted twice,
// would show.
//
//   go test -c -tags "desktop,production,quality_group_mentions" -o <dir>/group-mentions-host.exe ./cmd/dinah-desktop

import (
	"context"
	"database/sql"
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
	// It has a title and text, so the protected-state snapshot has real
	// content to compare.
	d, err := a.NewEmptyDraft("")
	if err != nil {
		t.Fatal(err)
	}
	if d, err = a.UpdateDraft(d.ID, d.Revision, "# Release notes\n\nThe group mentions fixture's document.\n"); err != nil {
		t.Fatal(err)
	}
	if d, err = a.RetitleDraft(d.ID, d.Revision, "release-notes.md"); err != nil {
		t.Fatal(err)
	}
	x.document = map[string]string{"draftId": d.ID, "title": d.Title, "text": d.Text}
	return x
}

// deliveryRow is one stored delivery of a message, read from the bus's
// deliveries table through an independent read-only connection.
type deliveryRow struct {
	RecipientKind    string `json:"recipientKind"`
	Recipient        string `json:"recipient"`
	ChatID           string `json:"chatId"`
	DeliveredSession string `json:"deliveredSession"`
	DeliveredAt      string `json:"deliveredAt"`
	CancelledAt      string `json:"cancelledAt"`
	State            string `json:"state"`
}

// deliveries is every delivery row the bus stored for one message, with its
// state: cancelled, waiting, delivered to a session, or settled without one.
func (x *groupMentionsFixture) deliveries(message int64) ([]deliveryRow, error) {
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(filepath.Join(configDir(), "bus.db"))+"?mode=ro&_pragma=busy_timeout(10000)")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.QueryContext(context.Background(), `SELECT recipient_kind, recipient, delivered_session, COALESCE(delivered_at, ''), COALESCE(cancelled_at, '') FROM deliveries WHERE message_id = ? ORDER BY id`, message)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []deliveryRow{}
	for rows.Next() {
		var r deliveryRow
		if err := rows.Scan(&r.RecipientKind, &r.Recipient, &r.DeliveredSession, &r.DeliveredAt, &r.CancelledAt); err != nil {
			return nil, err
		}
		for _, c := range x.chats {
			if c.BusID == r.Recipient {
				r.ChatID = c.ChatID
			}
		}
		switch {
		case r.CancelledAt != "":
			r.State = "cancelled"
		case r.DeliveredAt == "":
			r.State = "waiting"
		case r.DeliveredSession != "":
			r.State = "delivered"
		default:
			r.State = "settled-without-session"
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// promptCall is one session/prompt the provider received, with the session
// it was for, whether or not that session is one of the fixture's chats.
type promptCall struct {
	SessionID string `json:"sessionId"`
	Known     bool   `json:"known"`
}

// prompts is every prompt the provider has received, in order.
func (x *groupMentionsFixture) prompts() []promptCall {
	out := []promptCall{}
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
		known := false
		for _, c := range x.chats {
			known = known || c.ChatID == id
		}
		out = append(out, promptCall{SessionID: id, Known: known})
	}
	return out
}

// settle is the completion barrier: it rings the watcher, waits until no
// chat has a turn in progress, and rings again, until a ring starts nothing
// new and every chat is idle. It fails if the work has not settled in time.
func (x *groupMentionsFixture) settle() error {
	deadline := time.Now().Add(15 * time.Second)
	for {
		before := len(x.prompts())
		ring(x.t, x.a)
		for x.anyWorking() {
			if time.Now().After(deadline) {
				return fmt.Errorf("the chats' turns did not end within 15 seconds")
			}
			time.Sleep(25 * time.Millisecond)
		}
		if len(x.prompts()) == before {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("the watcher kept starting turns for 15 seconds")
		}
	}
}

// anyWorking says whether any of the fixture's chats has a turn in progress.
func (x *groupMentionsFixture) anyWorking() bool {
	for _, c := range x.chats {
		if s := x.a.session(c.ChatID); s != nil && s.Working() {
			return true
		}
	}
	return false
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
		// Anything still in flight from earlier calls settles first, so every
		// prompt counted below belongs to this send.
		if err := x.settle(); err != nil {
			return result(nil, fmt.Errorf("observation before the send: %w", err))
		}
		promptsBefore := len(x.prompts())
		m, sendErr := x.a.BusSendToGroup(key, text, mentions, files)
		event := map[string]any{"kind": "send", "group": key, "text": text, "mentions": mentions, "files": files}
		record := func() {
			x.mu.Lock()
			x.events = append(x.events, event)
			x.mu.Unlock()
		}
		if sendErr != nil {
			event["error"] = sendErr.Error()
			// A refused send must still wake no one, so its prompts are kept too.
			if err := x.settle(); err != nil {
				event["observationError"] = err.Error()
			}
			event["dispatchCalls"] = x.prompts()[promptsBefore:]
			record()
			return result(nil, sendErr)
		}
		// The committed recipients are the ones the bus recorded the message
		// as reaching, which it asks to answer, by chat.
		committed := []string{}
		if m.Reach != nil {
			for _, r := range m.Reach.Reach {
				for _, c := range x.chats {
					if c.BusID == r.ID {
						committed = append(committed, c.ChatID)
					}
				}
			}
			sort.Strings(committed)
		}
		// The watcher rings every open chat, as the window's does, until its
		// work settles; every prompt since the send is a dispatch attempt,
		// counted with repeats and kept whether or not its session is known.
		settleErr := x.settle()
		calls := x.prompts()[promptsBefore:]
		dispatched := []string{}
		counts := map[string]int{}
		for _, c := range calls {
			dispatched = append(dispatched, c.SessionID)
			counts[c.SessionID]++
		}
		rows, rowsErr := x.deliveries(m.ID)
		stored := []string{}
		for _, r := range rows {
			id := r.ChatID
			if id == "" {
				id = r.Recipient
			}
			stored = append(stored, id)
		}
		event["messageId"] = m.ID
		event["reach"] = m.Reach
		event["committedRecipients"] = committed
		event["deliveryRows"] = rows
		event["dispatchCalls"] = calls
		event["dispatchCounts"] = counts
		record()
		if settleErr != nil {
			return result(nil, fmt.Errorf("observation after the send (message %d was sent): %w", m.ID, settleErr))
		}
		if rowsErr != nil {
			return result(nil, fmt.Errorf("reading the deliveries of message %d: %w", m.ID, rowsErr))
		}
		return map[string]any{"result": m, "committedRecipients": committed, "storedDeliveries": stored, "deliveryRows": rows, "dispatchAttempts": dispatched, "dispatchCounts": counts, "dispatchCalls": calls}
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
	case "protectedState":
		// The state a group mention must leave alone, with what a message is
		// expected to change (last activity, unread counts, the newest
		// message, chat states) left out: each group's title, lead, budget
		// and members (not its file limit, which the members' running
		// programs set and a restart has not loaded yet); each chat's saved ID, title and folder; and each
		// open document's ID, title, text and revision, the seeded one first.
		ov, err := x.a.BusOverview()
		if err != nil {
			return result(nil, err)
		}
		rooms := []map[string]any{}
		for _, r := range ov.Rooms {
			members := []map[string]string{}
			for _, m := range r.Members {
				members = append(members, map[string]string{"id": m.ID, "chat": m.Chat, "title": m.Title})
			}
			sort.Slice(members, func(i, j int) bool { return members[i]["id"] < members[j]["id"] })
			rooms = append(rooms, map[string]any{"name": r.Name, "title": r.Title, "lead": r.Lead, "budget": r.Budget, "members": members})
		}
		sort.Slice(rooms, func(i, j int) bool { return rooms[i]["name"].(string) < rooms[j]["name"].(string) })
		// The chats as their saved records say, which a restart reopens.
		inv, err := x.a.SavedChats()
		if err != nil {
			return result(nil, err)
		}
		chats := []map[string]string{}
		for _, c := range x.chats {
			row := map[string]string{"id": c.ChatID, "missing": "true"}
			for _, saved := range inv.Chats {
				if saved.ID == c.ChatID {
					row = map[string]string{"id": saved.ID, "chatId": saved.ChatID, "title": saved.Title, "cwd": saved.Cwd}
				}
			}
			chats = append(chats, row)
		}
		// The seeded document, read by its ID: it must be there with the
		// title and text it was seeded with, or the snapshot fails, so an
		// empty list can never stand for a preserved document. Any other
		// draft the list shows is kept too.
		seeded, err := x.a.GetDraft(x.document["draftId"])
		if err != nil {
			return result(nil, fmt.Errorf("the seeded document %s: %w", x.document["draftId"], err))
		}
		if seeded.Title != x.document["title"] || seeded.Text != x.document["text"] {
			return result(nil, fmt.Errorf("the seeded document %s changed: title %q, text %q", seeded.ID, seeded.Title, seeded.Text))
		}
		docs := []map[string]string{{"id": seeded.ID, "title": seeded.Title, "text": seeded.Text, "revision": seeded.Revision, "seeded": "true"}}
		views, err := x.a.ListDrafts()
		if err != nil {
			return result(nil, err)
		}
		for _, d := range views {
			if d.ID != seeded.ID {
				docs = append(docs, map[string]string{"id": d.ID, "title": d.Title, "text": d.Text, "revision": d.Revision})
			}
		}
		sort.Slice(docs[1:], func(i, j int) bool { return docs[1+i]["id"] < docs[1+j]["id"] })
		return result(map[string]any{"rooms": rooms, "chats": chats, "documents": docs}, nil)
	case "observations":
		x.mu.Lock()
		defer x.mu.Unlock()
		return result(map[string]any{"events": x.events, "prompts": x.prompts()}, nil)
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
	// The message's own delivery rows: the member's was taken by its session.
	rows, _ := out["deliveryRows"].([]deliveryRow)
	delivered := false
	for _, r := range rows {
		delivered = delivered || (r.ChatID == member && r.State == "delivered")
	}
	if !delivered {
		t.Fatalf("no delivered row for the member in %+v", rows)
	}
	// The raw counter keeps repeats: a second send to the member is a second
	// prompt for it, counted from the observations as a whole.
	if again := x.call("BusSendToGroup", []json.RawMessage{raw(x.group), raw("Second control"), raw([]string{memberBus}), raw([]Attachment{})}); again["error"] != nil {
		t.Fatal(again["error"])
	}
	n := 0
	for _, c := range x.prompts() {
		if c.SessionID == member {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("the member was prompted %d times over two sends, want 2", n)
	}
	// The snapshot holds the seeded document, and fails once it changes.
	snap := x.call("protectedState", nil)
	if snap["error"] != nil {
		t.Fatal(snap["error"])
	}
	docs := snap["result"].(map[string]any)["documents"].([]map[string]string)
	if len(docs) == 0 || docs[0]["id"] != x.document["draftId"] || docs[0]["text"] != x.document["text"] || docs[0]["title"] != "release-notes.md" {
		t.Fatalf("the snapshot lacks the seeded document: %+v", docs)
	}
	d, err := x.a.GetDraft(x.document["draftId"])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := x.a.UpdateDraft(d.ID, d.Revision, "changed"); err != nil {
		t.Fatal(err)
	}
	if changed := x.call("protectedState", nil); changed["error"] == nil {
		t.Fatal("the snapshot accepted a changed seeded document")
	}
}
