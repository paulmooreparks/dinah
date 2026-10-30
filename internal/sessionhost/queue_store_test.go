package sessionhost

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"studio/internal/acp"
)

func queuedChat(t *testing.T, store *QueueStore, id string) *Conversation {
	t.Helper()
	c := NewConversation(Identity{ID: id, Cwd: t.TempDir()}, InitialActivity{Open: true}, false, false)
	c.SetQueueSink(func(q []StoredPrompt) { store.Submit(id, q) })
	return c
}

func texts(ps []StoredPrompt) []string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Text)
	}
	return out
}

func queuedTexts(c *Conversation) []string {
	return c.Info().Queued
}

// Everything a queued message carries comes back as it was, in its order, and a
// prompt that is a waiting question is not kept.
func TestQueuedMessagesAreKeptWithWhatTheyCarry(t *testing.T) {
	store := NewQueueStore(t.TempDir())
	c := queuedChat(t, store, "chat")
	c.StartWork()
	picture := Attachment{Kind: "file", Name: "look.png"}
	first := NewPrompt("One", []acp.ContentBlock{{Type: "text", Text: "One"}, {Type: "image", Data: "aGk=", MimeType: "image/png"}}).WithDelivery("From a chat", []ConversationMessage{{Role: "agent", Text: "delivered"}}, []Attachment{picture})
	c.Enqueue(first)
	c.Enqueue(NewPrompt("Two", nil).AsLateSteer(7, 9))
	c.Enqueue(NewPrompt("A question", nil).WithReply(context.Background(), &sync.Mutex{}))
	store.Flush()

	kept, err := store.Load("chat")
	if err != nil || !slices.Equal(texts(kept), []string{"One", "Two"}) {
		t.Fatalf("kept %v, %v", texts(kept), err)
	}
	if kept[0].ID == "" || kept[0].ID == kept[1].ID || len(kept[0].Blocks) != 2 || kept[0].Blocks[1].Data != "aGk=" || kept[0].Label != "From a chat" ||
		len(kept[0].Delivered) != 1 || len(kept[0].Pictures) != 1 || !kept[1].Late || kept[1].SteerID != 7 || kept[1].LedgerID != 9 {
		t.Fatalf("what the messages carry: %+v", kept)
	}
	back := PromptFromStored(kept[0])
	if back.Text() != "One" || back.Label() != "From a chat" || len(back.Blocks()) != 2 || len(back.Pictures()) != 1 || len(back.Delivered()) != 1 {
		t.Fatalf("the prompt made from the kept one: %+v", back)
	}
}

// The file follows the queue: a message taken to run leaves it, the person's clearing
// empties and removes it, and a steer that waits is kept first.
func TestTheFileFollowsTheQueue(t *testing.T) {
	store := NewQueueStore(t.TempDir())
	c := queuedChat(t, store, "chat")
	c.StartWork()
	for _, text := range []string{"One", "Two", "Three"} {
		c.Enqueue(NewPrompt(text, nil))
	}
	c.SetInterrupt(&Prompt{text: "Steer"})
	store.Flush()
	kept, _ := store.Load("chat")
	if !slices.Equal(texts(kept), []string{"Steer", "One", "Two", "Three"}) {
		t.Fatalf("kept %v", texts(kept))
	}
	// The turn ends and takes the steer.
	if next, _, _, _ := c.NextTurn(nil, nil, true); next == nil || next.Text() != "Steer" {
		t.Fatalf("the next turn is %v", next)
	}
	store.Flush()
	kept, _ = store.Load("chat")
	if !slices.Equal(texts(kept), []string{"One", "Two", "Three"}) {
		t.Fatalf("after the steer ran: %v", texts(kept))
	}
	c.ClearQueue()
	store.Flush()
	if _, err := os.Stat(store.path("chat")); !os.IsNotExist(err) {
		t.Fatalf("the file of an empty queue remains: %v", err)
	}
}

// A provider that stops drops the chat's queue from memory and not from disk, so the
// messages are there when the chat is opened again.
func TestAStoppedProviderLeavesTheQueueKept(t *testing.T) {
	store := NewQueueStore(t.TempDir())
	c := queuedChat(t, store, "chat")
	c.StartWork()
	c.Enqueue(NewPrompt("One", nil))
	store.Flush()
	if dropped := c.ProviderStopped(); len(dropped) != 1 {
		t.Fatalf("dropped %d", len(dropped))
	}
	store.Flush()
	if kept, _ := store.Load("chat"); len(kept) != 1 {
		t.Fatalf("kept %v", texts(kept))
	}
}

// A chat reopened has its file read before anything is written: messages typed while
// it loads go after the restored ones, and nothing replaces the file first.
func TestARestoreIsNotOverwrittenByMessagesTypedWhileLoading(t *testing.T) {
	dir := t.TempDir()
	store := NewQueueStore(dir)
	old := queuedChat(t, store, "chat")
	old.StartWork()
	old.Enqueue(NewPrompt("Before", nil))
	store.Flush()

	again := NewQueueStore(dir)
	c := queuedChat(t, again, "chat")
	c.StartWork()
	c.Enqueue(NewPrompt("While loading", nil))
	again.Flush()
	if kept, _ := again.Load("chat"); !slices.Equal(texts(kept), []string{"Before"}) {
		t.Fatalf("the file was replaced before the restore: %v", texts(kept))
	}
	NewQueueRestorer(again, QueueRestoreHost{Head: func(string) string { return "" }}).Restore(c)
	again.Flush()
	if !slices.Equal(queuedTexts(c), []string{"Before", "While loading"}) {
		t.Fatalf("the queue is %v", queuedTexts(c))
	}
	if kept, _ := again.Load("chat"); !slices.Equal(texts(kept), []string{"Before", "While loading"}) {
		t.Fatalf("the file is %v", texts(kept))
	}
}

// While a compaction carries the queue, the messages stay kept until the new chat has
// them, so a crash between loses none; and they come back if the compaction fails.
func TestAQueueBeingCarriedStaysKeptUntilSettled(t *testing.T) {
	store := NewQueueStore(t.TempDir())
	c := queuedChat(t, store, "chat")
	c.StartWork()
	c.Enqueue(NewPrompt("One", nil))
	c.Enqueue(NewPrompt("Two", nil))
	carried := c.TakeQueued()
	c.Enqueue(NewPrompt("Typed meanwhile", nil))
	store.Flush()
	kept, _ := store.Load("chat")
	if len(carried) != 2 || !slices.Equal(texts(kept), []string{"One", "Two", "Typed meanwhile"}) {
		t.Fatalf("carried %d, kept %v", len(carried), texts(kept))
	}
	c.SettleCarry()
	store.Flush()
	kept, _ = store.Load("chat")
	if !slices.Equal(texts(kept), []string{"Typed meanwhile"}) {
		t.Fatalf("after settling: %v", texts(kept))
	}
	// A failed carry puts them back in front.
	c2 := queuedChat(t, store, "other")
	c2.StartWork()
	c2.Enqueue(NewPrompt("A", nil))
	c2.Enqueue(NewPrompt("B", nil))
	taken := c2.TakeQueued()
	c2.RestoreQueued(taken)
	store.Flush()
	if kept, _ := store.Load("other"); !slices.Equal(texts(kept), []string{"A", "B"}) || !slices.Equal(queuedTexts(c2), []string{"A", "B"}) {
		t.Fatalf("after a failed carry: kept %v, queue %v", texts(kept), queuedTexts(c2))
	}
}

type restoreFixture struct {
	store     *QueueStore
	open      map[string]*Conversation
	heads     map[string]string
	deleted   map[string]bool
	sent      map[string][]string
	sendErr   error
	discarded map[string][]string
	damaged   []string
}

func newRestoreFixture(t *testing.T) (*restoreFixture, *QueueRestorer) {
	t.Helper()
	f := &restoreFixture{store: NewQueueStore(t.TempDir()), open: map[string]*Conversation{}, heads: map[string]string{}, deleted: map[string]bool{},
		sent: map[string][]string{}, discarded: map[string][]string{}}
	r := NewQueueRestorer(f.store, QueueRestoreHost{
		Head:    func(id string) string { return f.heads[id] },
		Deleted: func(id string) bool { return f.deleted[id] },
		Session: func(id string) *Conversation { return f.open[id] },
		Send: func(id string, p Prompt) error {
			if f.sendErr != nil {
				return f.sendErr
			}
			f.sent[id] = append(f.sent[id], p.Text())
			return nil
		},
		Discarded: func(id string, texts []string) { f.discarded[id] = texts },
		Damaged:   func(id string, err error) { f.damaged = append(f.damaged, id) },
	})
	return f, r
}

func keep(t *testing.T, store *QueueStore, id string, ids []string, texts ...string) {
	t.Helper()
	var ps []StoredPrompt
	for i, text := range texts {
		ps = append(ps, StoredPrompt{ID: ids[i], Text: text})
	}
	store.armed[id] = true
	if err := store.writeFile(id, ps); err != nil {
		t.Fatal(err)
	}
}

// A chat that was continued while Dinah was down sends its messages to its
// continuation once: those it already has, by id, are not sent again, and a
// continuation that is closed has them added to its file.
func TestAContinuedChatsMessagesGoToItsContinuationOnce(t *testing.T) {
	f, r := newRestoreFixture(t)
	keep(t, f.store, "old", []string{"a", "b", "c"}, "One", "Two", "Three")
	f.heads["old"] = "new"
	head := queuedChat(t, f.store, "new")
	head.StartWork()
	head.Enqueue(PromptFromStored(StoredPrompt{ID: "a", Text: "One"}))
	f.open["new"] = head
	r.Restore(queuedChat(t, f.store, "old"))
	if !slices.Equal(f.sent["new"], []string{"Two", "Three"}) {
		t.Fatalf("sent to the open continuation: %v", f.sent["new"])
	}
	if _, err := os.Stat(f.store.path("old")); !os.IsNotExist(err) {
		t.Fatalf("the old file remains: %v", err)
	}

	// A closed continuation.
	keep(t, f.store, "old2", []string{"x", "y"}, "Four", "Five")
	keep(t, f.store, "closed", []string{"y"}, "Five")
	f.heads["old2"] = "closed"
	r.Restore(queuedChat(t, f.store, "old2"))
	kept, _ := f.store.Load("closed")
	if !slices.Equal(texts(kept), []string{"Five", "Four"}) {
		t.Fatalf("the closed continuation's file: %v", texts(kept))
	}
}

// A continuation that cannot take a message now keeps it in its file.
func TestAContinuationThatCannotTakeAMessageKeepsItInItsFile(t *testing.T) {
	f, r := newRestoreFixture(t)
	keep(t, f.store, "old", []string{"a"}, "One")
	f.heads["old"] = "new"
	f.open["new"] = queuedChat(t, f.store, "new")
	f.sendErr = errors.New("busy")
	r.Restore(queuedChat(t, f.store, "old"))
	kept, _ := f.store.Load("new")
	if !slices.Equal(texts(kept), []string{"One"}) {
		t.Fatalf("the continuation's file: %v", texts(kept))
	}
}

// The messages of a deleted chat are discarded and their text is given to the person.
// The sweep at start reaches chats that are not open, and leaves a closed chat's file for
// when it is opened.
func TestADeletedChatsMessagesAreDiscardedAndTheirTextIsGiven(t *testing.T) {
	f, r := newRestoreFixture(t)
	keep(t, f.store, "gone", []string{"a", "b"}, "Look at this", "  And this  ")
	keep(t, f.store, "waiting", []string{"c"}, "For later")
	keep(t, f.store, "continued", []string{"d"}, "Moved")
	f.deleted["gone"] = true
	f.heads["continued"] = "newer"
	r.Sweep()
	if !slices.Equal(f.discarded["gone"], []string{"Look at this", "And this"}) {
		t.Fatalf("discarded %v", f.discarded)
	}
	if _, err := os.Stat(f.store.path("gone")); !os.IsNotExist(err) {
		t.Fatal("a deleted chat's file remains")
	}
	if kept, _ := f.store.Load("waiting"); !slices.Equal(texts(kept), []string{"For later"}) {
		t.Fatalf("a closed chat's file was touched: %v", texts(kept))
	}
	if kept, _ := f.store.Load("newer"); !slices.Equal(texts(kept), []string{"Moved"}) {
		t.Fatalf("a continued chat's messages: %v", texts(kept))
	}
}

// A file that cannot be read is set aside, the person is told, and the chat opens
// with an empty queue that can be written.
func TestADamagedFileIsSetAsideAndDoesNotStopTheChat(t *testing.T) {
	f, r := newRestoreFixture(t)
	if err := os.MkdirAll(filepath.Dir(f.store.path("chat")), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f.store.path("chat"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := queuedChat(t, f.store, "chat")
	r.Restore(c)
	if len(queuedTexts(c)) != 0 || !slices.Equal(f.damaged, []string{"chat"}) {
		t.Fatalf("queue %v, damaged %v", queuedTexts(c), f.damaged)
	}
	if _, err := os.Stat(f.store.path("chat") + ".bad"); err != nil {
		t.Fatalf("the damaged file was not set aside: %v", err)
	}
	c.StartWork()
	c.Enqueue(NewPrompt("New", nil))
	f.store.Flush()
	if kept, err := f.store.Load("chat"); err != nil || !slices.Equal(texts(kept), []string{"New"}) {
		t.Fatalf("a queue written after the damage: %v %v", texts(kept), err)
	}
	// A file of another chat's or another version is treated the same way.
	keep(t, f.store, "mixed", []string{"a"}, "One")
	data, _ := os.ReadFile(f.store.path("mixed"))
	if err := os.WriteFile(f.store.path("mixed"), []byte(strings.Replace(string(data), `"id":"mixed"`, `"id":"someone-else"`, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.Load("mixed"); err == nil {
		t.Fatal("another chat's file was read")
	}
}

// A message carried to a chat and already started there is not sent again when the earlier
// chat's file, which a crash left unsettled, is restored: the continuation's history
// says it started, whether the continuation is open or closed.
func TestAMessageThatStartedInTheContinuationIsNotRunAgain(t *testing.T) {
	f, r := newRestoreFixture(t)
	started := map[string]bool{"a": true}
	r.host.Started = func(chat, id string) bool { return started[id] }
	keep(t, f.store, "old", []string{"a", "b", "c"}, "One", "Two", "Three")
	f.heads["old"] = "new"
	f.open["new"] = queuedChat(t, f.store, "new")
	r.Restore(queuedChat(t, f.store, "old"))
	if !slices.Equal(f.sent["new"], []string{"Two", "Three"}) {
		t.Fatalf("sent to the open continuation: %v", f.sent["new"])
	}

	keep(t, f.store, "old2", []string{"a", "x"}, "One", "Four")
	f.heads["old2"] = "closed"
	r.Restore(queuedChat(t, f.store, "old2"))
	kept, _ := f.store.Load("closed")
	if !slices.Equal(texts(kept), []string{"Four"}) {
		t.Fatalf("the closed continuation's file: %v", texts(kept))
	}
}

// The ids of the last messages that started are kept with the saved copy, and no more
// than a bounded number.
func TestTheSavedCopyKeepsTheIdsOfStartedMessagesUpToABound(t *testing.T) {
	h := &History{saved: &Transcript{SessionID: "chat"}}
	dir := t.TempDir()
	for i := 0; i < startedQueueIDsKept+5; i++ {
		h.FinishPromptLocked(dir, nil, fmt.Sprint("id", i))
	}
	h.FinishPromptLocked(dir, nil, "")
	ids := h.saved.StartedQueueIDs
	if len(ids) != startedQueueIDsKept || ids[len(ids)-1] != fmt.Sprint("id", startedQueueIDsKept+4) || ids[0] != "id5" {
		t.Fatalf("%d ids, from %s to %s", len(ids), ids[0], ids[len(ids)-1])
	}
}
