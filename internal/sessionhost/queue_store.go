package sessionhost

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"studio/internal/acp"
	"studio/internal/statefile"
)

// A chat's queued messages are kept on disk with the chat, so that a restart or a
// crash picks them up again in order (docs/plans/ollama-context-window.md, Queued
// Messages Are Kept). One file for each chat that has any, rewritten whole each time
// the queue changes and removed when it is empty. A message that is a waiting
// question from another chat is not kept, since its asker's wait ends with the
// process and the bus asks again.

// StoredPrompt is a queued message as it is kept. ID is its own, made when it was
// queued and kept when it is carried to another chat, so that a message found in
// two files is recognised as one.
type StoredPrompt struct {
	ID        string                `json:"id"`
	Text      string                `json:"text"`
	Label     string                `json:"label,omitempty"`
	Blocks    []acp.ContentBlock    `json:"blocks,omitempty"`
	Delivered []ConversationMessage `json:"delivered,omitempty"`
	Pictures  []Attachment          `json:"pictures,omitempty"`
	Late      bool                  `json:"late,omitempty"`
	LedgerID  int64                 `json:"ledgerId,omitempty"`
	SteerID   int64                 `json:"steerId,omitempty"`
}

type queueFile struct {
	Version int            `json:"version"`
	ID      string         `json:"id"`
	Queued  []StoredPrompt `json:"queued"`
}

const queueFileVersion = 1

// newQueueID makes the id of a queued message.
func newQueueID() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b[:])
}

// QueueStore keeps the files. Submit never waits for the disk: the newest queue of
// each chat is kept in memory and a writer goroutine writes it, so a change made
// under a chat's lock costs nothing, and Flush waits until what was submitted is
// written.
type QueueStore struct {
	dir string

	mu      sync.Mutex
	pending map[string][]StoredPrompt // the newest queue of each chat not yet written
	armed   map[string]bool           // chats whose file may be written
	writing bool
	closed  bool
	idle    *sync.Cond
	// Logf reports what could not be written or read.
	Logf func(format string, args ...any)
}

// NewQueueStore keeps queue files in dir.
func NewQueueStore(dir string) *QueueStore {
	q := &QueueStore{dir: dir, pending: map[string][]StoredPrompt{}, armed: map[string]bool{}}
	q.idle = sync.NewCond(&q.mu)
	return q
}

func (q *QueueStore) path(id string) string {
	return filepath.Join(q.dir, base64.RawURLEncoding.EncodeToString([]byte(id))+".json")
}

func (q *QueueStore) logf(format string, args ...any) {
	if q.Logf != nil {
		q.Logf(format, args...)
	}
}

// Chats lists the chats that have a file.
func (q *QueueStore) Chats() []string {
	entries, err := os.ReadDir(q.dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		raw, err := base64.RawURLEncoding.DecodeString(name[:len(name)-len(".json")])
		if err == nil {
			out = append(out, string(raw))
		}
	}
	return out
}

// Load reads a chat's file. A file that cannot be read is moved aside with a .bad
// ending, so that it is not read again and not lost, and the error says so; a chat
// with no file has no queue.
func (q *QueueStore) Load(id string) ([]StoredPrompt, error) {
	path := q.path(id)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err == nil {
		var f queueFile
		if err = json.Unmarshal(data, &f); err == nil && f.ID == id && f.Version == queueFileVersion {
			return f.Queued, nil
		}
		if err == nil {
			err = errors.New("the file is not this chat's queue, or is of another version")
		}
	}
	if moveErr := os.Rename(path, path+".bad"); moveErr != nil {
		q.logf("the queue file of chat %s could not be set aside: %v", id, moveErr)
	}
	return nil, err
}

// Submit says what a chat's queue is now. Until Arm, a chat that has a file keeps it
// as it is, because the queue is about to be restored from it and a write now would
// replace what the restore needs; the newest queue is held and written at Arm.
func (q *QueueStore) Submit(id string, queue []StoredPrompt) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	q.pending[id] = queue
	if !q.armed[id] {
		if _, err := os.Stat(q.path(id)); err != nil {
			q.armed[id] = true
		}
	}
	q.kickLocked()
}

// Arm lets a chat's file be written, after its queue was restored from it.
func (q *QueueStore) Arm(id string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.armed[id] = true
	q.kickLocked()
}

// Forget takes a chat's file away, for a chat whose messages were discarded or
// moved, and drops anything not yet written for it.
func (q *QueueStore) Forget(id string) {
	q.mu.Lock()
	delete(q.pending, id)
	q.armed[id] = true
	q.mu.Unlock()
	q.Flush()
	if err := os.Remove(q.path(id)); err != nil && !errors.Is(err, os.ErrNotExist) {
		q.logf("the queue file of chat %s could not be removed: %v", id, err)
	}
}

func (q *QueueStore) kickLocked() {
	if q.writing {
		return
	}
	q.writing = true
	go q.write()
}

func (q *QueueStore) write() {
	q.mu.Lock()
	for {
		var id string
		var queue []StoredPrompt
		found := false
		for k, v := range q.pending {
			if q.armed[k] {
				id, queue, found = k, v, true
				break
			}
		}
		if !found {
			q.writing = false
			q.idle.Broadcast()
			q.mu.Unlock()
			return
		}
		delete(q.pending, id)
		q.mu.Unlock()
		if err := q.writeFile(id, queue); err != nil {
			q.logf("the queued messages of chat %s could not be kept: %v", id, err)
		}
		q.mu.Lock()
	}
}

func (q *QueueStore) writeFile(id string, queue []StoredPrompt) error {
	path := q.path(id)
	if len(queue) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(q.dir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(queueFile{Version: queueFileVersion, ID: id, Queued: queue})
	if err != nil {
		return err
	}
	return statefile.Write(path, data)
}

// Close writes what was submitted and takes no more, for a program that is ending: a
// chat still running goes on changing its queue, which is then not kept.
func (q *QueueStore) Close() {
	q.Flush()
	q.mu.Lock()
	q.closed = true
	q.mu.Unlock()
	q.Flush()
}

// Flush waits until every queue that may be written has been.
func (q *QueueStore) Flush() {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.writing {
		q.idle.Wait()
	}
}

// AppendFile adds messages to the end of the file of a chat that is not open, leaving
// out those it already has, by their ids.
func (q *QueueStore) AppendFile(id string, add []StoredPrompt) {
	q.mu.Lock()
	defer q.mu.Unlock()
	existing, err := q.Load(id)
	if err != nil {
		q.logf("the queue file of chat %s could not be read, so its messages were set aside: %v", id, err)
	}
	have := map[string]bool{}
	for _, sp := range existing {
		have[sp.ID] = true
	}
	for _, sp := range add {
		if !have[sp.ID] {
			existing = append(existing, sp)
			have[sp.ID] = true
		}
	}
	if err := q.writeFile(id, existing); err != nil {
		q.logf("the queued messages of chat %s could not be kept: %v", id, err)
	}
}

// Dir is the folder the files are in.
func (q *QueueStore) Dir() string { return q.dir }
