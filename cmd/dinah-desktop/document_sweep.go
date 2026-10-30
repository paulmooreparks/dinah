package main

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"studio/internal/drafts"
)

// draftSweepAge is how long a draft that holds nothing of the person's is
// kept after it last changed: one surface makes a draft for every document opened to
// read, and an unchanged one is made again when the document is next opened.
const draftSweepAge = 30 * 24 * time.Hour

// sweepDrafts forgets old unchanged drafts once, at startup. It keeps a
// draft whose Editor window is in the saved layout, so a restored window
// finds its draft.
func (a *App) sweepDrafts() {
	svc, err := a.draftService()
	if err != nil {
		return
	}
	layout, _ := os.ReadFile(filepath.Join(configDir(), "ui-state.json"))
	open := editorDrafts(layout)
	svc.Store.Sweep(time.Now().Add(-draftSweepAge), func(d drafts.Draft) bool {
		return open[d.ID]
	})
}

var windowKey = regexp.MustCompile(`item-[A-Za-z0-9_-]+`)

// editorDrafts lists the drafts of the Editor windows a saved layout holds.
// A window's key is "item-" and the unpadded URL-safe base64 of the JSON
// array [kind, workspace, path] (frontend/workbench.js, encode), and an
// Editor window's path is its draft's ID.
func editorDrafts(layout []byte) map[string]bool {
	out := map[string]bool{}
	for _, key := range windowKey.FindAll(layout, -1) {
		raw, err := base64.RawURLEncoding.DecodeString(string(key[len("item-"):]))
		if err != nil {
			continue
		}
		var item []string
		if json.Unmarshal(raw, &item) == nil && len(item) == 3 && item[0] == "editor" {
			out[item[2]] = true
		}
	}
	return out
}
