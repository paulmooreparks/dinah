package broker

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"studio/internal/layout"
	"studio/internal/records"
)

/* The index of saved invocations, kept between starts.

   Building the index read every saved invocation, with its inputs and
   results, to take its summary: thousands of records and tens of megabytes
   at each start, before the first question the window asks. The summaries
   are now kept in a file with the signature (size and time) of the files
   each came from, and a start reads again only the records whose files
   changed. */

type recordSig struct {
	Slot0  fileSig `json:"slot0"`
	Slot1  fileSig `json:"slot1"`
	Legacy fileSig `json:"legacy"`
}

type fileSig struct {
	Size    int64     `json:"size"`
	Changed time.Time `json:"changed"`
}

// same compares two signatures by their instants: a time read from the
// cache file and one read from the directory are the same instant in
// different forms.
func (s fileSig) same(o fileSig) bool { return s.Size == o.Size && s.Changed.Equal(o.Changed) }
func (s recordSig) same(o recordSig) bool {
	return s.Slot0.same(o.Slot0) && s.Slot1.same(o.Slot1) && s.Legacy.same(o.Legacy)
}

type cachedSummary struct {
	Sig     recordSig `json:"sig"`
	Summary Summary   `json:"summary"`
}

type indexCacheContent struct {
	Version int                      `json:"version"`
	Entries map[string]cachedSummary `json:"entries"`
}

// indexCacheVersion changes when the summary's shape changes, so a file an
// earlier program wrote is read again from the records.
const indexCacheVersion = 1

func (b *Broker) indexCacheFile() string { return filepath.Join(b.State, "cache", "applet-index.json") }

func (b *Broker) readIndexCache() map[string]cachedSummary {
	data, err := os.ReadFile(b.indexCacheFile())
	if err != nil {
		return map[string]cachedSummary{}
	}
	var content indexCacheContent
	if json.Unmarshal(data, &content) != nil || content.Version != indexCacheVersion || content.Entries == nil {
		return map[string]cachedSummary{}
	}
	return content.Entries
}

// writeIndexCache writes the cache whole and then in place, so a reader
// never sees a partial one.
func (b *Broker) writeIndexCache(entries map[string]cachedSummary) error {
	data, err := json.Marshal(indexCacheContent{Version: indexCacheVersion, Entries: entries})
	if err != nil {
		return err
	}
	path := b.indexCacheFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// signatures reads a root's run directory once and gives each saved
// invocation's signature, in the order of the directory.
func (b *Broker) signatures(root string) ([]string, map[string]recordSig, error) {
	entries, err := os.ReadDir(layout.Layout{State: b.State}.RunDir(root))
	if os.IsNotExist(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var ids []string
	sigs := map[string]recordSig{}
	for _, entry := range entries {
		id := records.ID(entry.Name())
		if id == "" || entry.IsDir() || !recordID.MatchString(id) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		sig, seen := sigs[id]
		if !seen {
			ids = append(ids, id)
		}
		fs := fileSig{Size: info.Size(), Changed: info.ModTime()}
		switch {
		case strings.HasSuffix(entry.Name(), ".run0.json"):
			sig.Slot0 = fs
		case strings.HasSuffix(entry.Name(), ".run1.json"):
			sig.Slot1 = fs
		default:
			sig.Legacy = fs
		}
		sigs[id] = sig
	}
	return ids, sigs, nil
}
