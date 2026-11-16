package sessionhost

import (
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed links.go
var LinksSource []byte

// TranscriptHead is what the links between saved conversations need of
// each one, without its messages.
type TranscriptHead struct {
	SessionID       string `json:"sessionId"`
	Provider        string `json:"provider"`
	Workspace       string `json:"workspace"`
	ParentSessionID string `json:"parentSessionId,omitempty"`
	Deleted         bool   `json:"deleted,omitempty"`
	// PendingHandoff is set while the conversation it continues has not been
	// handed to its program, which is before anything was said in it.
	PendingHandoff bool `json:"pendingHandoff,omitempty"`
}

// UnfinishedTargets are the saved conversations left by a switch that did not
// finish. A switch hides its new session until it commits, and one that fails
// after the handoff was written leaves that session hidden, with the handoff
// never delivered and the conversation it was to continue recorded as its
// parent. Such a session is not a stage of the chat: the chat goes on where it
// was, and opening it must not land in a session nothing was said in.
//
// A hidden session whose handoff was delivered is a stage like any other. So
// is one that a later stage continues. Builds before the switch records
// existed left the newest stage of some chats marked hidden, and those chats
// were worked in for weeks afterwards.
func UnfinishedTargets(records map[string]*TranscriptHead, hidden func(string) bool) map[string]bool {
	children := map[string][]string{}
	for id, r := range records {
		if r != nil && r.ParentSessionID != "" && r.ParentSessionID != id {
			children[r.ParentSessionID] = append(children[r.ParentSessionID], id)
		}
	}
	out := map[string]bool{}
	// A target whose only children are themselves unfinished is unfinished, so
	// the leaves are found first and the pass repeats until nothing is added.
	for added := true; added; {
		added = false
		for id, r := range records {
			if r == nil || out[id] || r.ParentSessionID == "" || r.ParentSessionID == id || r.Deleted || !r.PendingHandoff || !hidden(id) {
				continue
			}
			continued := false
			for _, child := range children[id] {
				if !out[child] {
					continued = true
				}
			}
			if !continued {
				out[id] = true
				added = true
			}
		}
	}
	return out
}

// NewestStage follows a chat from one of its stages to the stage it goes on
// in now. A committed binding decides where a stage goes on; without one it
// is the one saved conversation that continues the stage and is neither
// deleted nor an unfinished target. The walk stops at a stage that nothing
// continues or that two conversations continue, and that stage is returned.
func NewestStage(id string, records map[string]*TranscriptHead, bindings map[string]string, unfinished map[string]bool) string {
	children := map[string][]string{}
	for child, r := range records {
		if r != nil && r.ParentSessionID != "" && r.ParentSessionID != child && !r.Deleted && !unfinished[child] {
			children[r.ParentSessionID] = append(children[r.ParentSessionID], child)
		}
	}
	seen := map[string]bool{}
	for !seen[id] {
		seen[id] = true
		next := bindings[id]
		if next == "" && len(children[id]) == 1 {
			next = children[id][0]
		}
		if next == "" {
			break
		}
		id = next
	}
	return id
}

// heads keeps each saved conversation's head with the size and time of the
// file it was read from, so that finding the links reads again only the
// files that changed since. There are hundreds of saved conversations,
// tens of megabytes, and the links are needed on every restore and send.
var heads struct {
	sync.Mutex
	dir   string
	files map[string]headFile
}

type headFile struct {
	size int64
	mod  time.Time
	head *TranscriptHead
}

func ConversationLinks(stateDir string) (map[string]string, map[string]*TranscriptHead) {
	dir := filepath.Join(stateDir, "conversations")
	heads.Lock()
	defer heads.Unlock()
	if heads.dir != dir || heads.files == nil {
		heads.dir, heads.files = dir, map[string]headFile{}
	}
	entries, _ := os.ReadDir(dir)
	present := map[string]bool{}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		present[e.Name()] = true
		info, err := e.Info()
		if err != nil {
			continue
		}
		if f, ok := heads.files[e.Name()]; ok && f.size == info.Size() && f.mod.Equal(info.ModTime()) {
			continue
		}
		f := headFile{size: info.Size(), mod: info.ModTime()}
		var h TranscriptHead
		if data, err := os.ReadFile(filepath.Join(dir, e.Name())); err == nil && json.Unmarshal(data, &h) == nil && h.SessionID != "" {
			f.head = &h
		}
		heads.files[e.Name()] = f
	}
	links := map[string]string{}
	records := map[string]*TranscriptHead{}
	for name, f := range heads.files {
		if !present[name] {
			delete(heads.files, name)
			continue
		}
		if f.head == nil {
			continue
		}
		h := *f.head
		records[h.SessionID] = &h
		if h.ParentSessionID != "" {
			links[h.ParentSessionID] = h.SessionID
		}
	}
	return links, records
}
