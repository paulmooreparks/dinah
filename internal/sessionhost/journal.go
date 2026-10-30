package sessionhost

import (
	"bufio"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

// The protocol gives no time for a message: when a session is reopened, the
// agent replays its history without saying when each part was said. So the
// Dinah keeps its own journal of each session, one line per message as
// it happens: when the person's message was sent and when each message of
// the agent began. On a reopen, the page puts the times back on the replayed
// messages in order. A session Dinah never saw, such as one from
// Devin Desktop, has no journal, and its messages show no time rather than a
// wrong one.

// JournalEntry is one message's time: "you" for the person's message, "agent"
// for the start of one of the agent's.
type JournalEntry struct {
	Kind string    `json:"kind"`
	At   time.Time `json:"at"`
}

var safeSessionID = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// JournalPath is where a chat's journal is. A provider with a journal folder
// of its own (providers.Row.JournalFolder) keeps it there, under a name made
// from the ID.
func JournalPath(dir, id, folder string) string {
	if folder != "" {
		return filepath.Join(dir, "journal", folder, base64.RawURLEncoding.EncodeToString([]byte(id))+".jsonl")
	}
	return filepath.Join(dir, "journal", safeSessionID.ReplaceAllString(id, "_")+".jsonl")
}

// journal records a message of a session, now.
func (j *Journal) Record(id, kind string, path func(string) string) {
	b, err := json.Marshal(JournalEntry{Kind: kind, At: time.Now()})
	if err != nil {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	p := path(id)
	if os.MkdirAll(filepath.Dir(p), 0o700) != nil {
		return
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(b, '\n'))
}

// SessionTimes returns a session's journal, oldest first, or an empty list
// for a session Dinah never saw.
func (j *Journal) Times(id string, path func(string) string) []JournalEntry {
	out := []JournalEntry{}
	j.mu.Lock()
	defer j.mu.Unlock()
	f, err := os.Open(path(id))
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e JournalEntry
		if json.Unmarshal(sc.Bytes(), &e) == nil {
			out = append(out, e)
		}
	}
	return out
}

// forgetJournal removes a session's journal with the session.
func (j *Journal) Forget(id string, path func(string) string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	_ = os.Remove(path(id))
}

//go:embed journal.go
var JournalSource []byte

// Journal serializes message-time persistence for one host. It must not be copied
// after use. Path callbacks run under its lock and must not re-enter Journal.
type Journal struct{ mu sync.Mutex }
