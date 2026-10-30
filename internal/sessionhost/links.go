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
