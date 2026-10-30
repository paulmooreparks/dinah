package broker

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"studio/internal/layout"
)

// Notifications asks which saved invocations need a person after every
// change, and reading every record to answer took about half a second with
// six hundred of them. So the broker keeps a summary of every saved
// invocation in memory: it is built by one scan of the state folder, the
// first time it is needed, and kept current by every save and removal this
// process makes. Changes another program makes, such as the command line
// running a flow, reach it through Watch. Attention then reads in full only
// the invocations it lists.
//
// Every Broker of one state folder in a process shares the folder's index,
// so a save through any of them keeps it current.

// Summary is what is kept in memory of a saved invocation.
type Summary struct {
	Workspace string
	ID        string
	OwnerKind string
	OwnerID   string
	Action    string
	Status    string
	// Artifacts is how many artifacts the invocation made.
	Artifacts int
	Dismissed bool
	Version   int
	Created   time.Time
	Updated   time.Time
	Deadline  time.Time
}

// Summarize is the summary of an invocation.
func Summarize(v Invocation) Summary {
	return Summary{Workspace: v.Workspace, ID: v.ID, OwnerKind: v.OwnerKind, OwnerID: v.OwnerID, Action: v.Action, Status: v.Status, Artifacts: len(v.Artifacts), Dismissed: v.ResultDismissed, Version: v.Version, Created: v.Created, Updated: v.Updated, Deadline: v.Deadline}
}

type index struct {
	// build is held while the index is built, so that it is built once.
	build sync.Mutex
	mu    sync.Mutex
	built bool
	// scanErr is the first record the last build could not read.
	scanErr error
	entries map[string]Summary
	watch   *watch
	// cache is each saved invocation's summary with the signature of the
	// files it was read from (index_cache.go), and cacheDirty whether it
	// changed since its file was written.
	cache      map[string]cachedSummary
	cacheDirty bool
}

var indexes sync.Map

// stateKey is a state folder's key in indexes. Spellings that differ only in
// case share an index, as Windows compares paths without case.
func stateKey(state string) string {
	abs, err := filepath.Abs(state)
	if err != nil {
		abs = filepath.Clean(state)
	}
	return strings.ToLower(abs)
}

// index is the index of the broker's state folder.
func (b *Broker) index() *index {
	ix, _ := indexes.LoadOrStore(stateKey(b.State), &index{})
	return ix.(*index)
}

// set records a summary this process has just saved.
func (ix *index) set(key string, s Summary) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if ix.entries == nil {
		ix.entries = map[string]Summary{}
	}
	ix.entries[key] = s
}

// merge records a summary read from disk, unless the index already has a
// later version, and reports whether the index changed.
func (ix *index) merge(key string, s Summary) bool {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if ix.entries == nil {
		ix.entries = map[string]Summary{}
	}
	old, ok := ix.entries[key]
	if ok && (old.Version > s.Version || old == s) {
		return false
	}
	ix.entries[key] = s
	return true
}

func (ix *index) remove(key string) bool {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	_, ok := ix.entries[key]
	delete(ix.entries, key)
	return ok
}

func (ix *index) keys(match func(string) bool) []string {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	var out []string
	for key := range ix.entries {
		if match(key) {
			out = append(out, key)
		}
	}
	return out
}

func (ix *index) snapshot() []Summary {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	out := make([]Summary, 0, len(ix.entries))
	for _, s := range ix.entries {
		out = append(out, s)
	}
	return out
}

// indexKey is an invocation's key in the index: the name of its workspace's
// folder under the applets folder, a colon and its identity. It does not
// depend on how the state folder's path is spelled.
func indexKey(root, id string) string { return filepath.Base(root) + ":" + id }

// keyRoot is the name of the workspace folder of an index key.
func keyRoot(key string) string { return key[:strings.LastIndex(key, ":")] }

func (b *Broker) indexKey(workspace, id string) string { return indexKey(b.StoreRoot(workspace), id) }

// appletsDir is the folder every workspace's invocation folder is in.
func (b *Broker) appletsDir() string { return filepath.Join(b.State, "applets") }

// readSummary reads the summary of a record in a workspace folder of the
// state folder.
func (b *Broker) readSummary(root, id string) (Summary, error) {
	data, err := layout.Layout{State: b.State}.Runs(root).Read(id)
	if err != nil {
		return Summary{}, err
	}
	var v struct {
		ID              string     `json:"id"`
		Workspace       string     `json:"workspace"`
		OwnerKind       string     `json:"ownerKind"`
		OwnerID         string     `json:"ownerId"`
		Action          string     `json:"action"`
		Status          string     `json:"status"`
		Deadline        time.Time  `json:"deadline"`
		Created         time.Time  `json:"created"`
		Updated         time.Time  `json:"updated"`
		Version         int        `json:"version"`
		Artifacts       []struct{} `json:"artifacts"`
		ResultDismissed bool       `json:"resultDismissed"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return Summary{}, err
	}
	if b.StoreRoot(v.Workspace) != root || v.ID != id {
		return Summary{}, fmt.Errorf("%w: workspace record mismatch", ErrRecovery)
	}
	return Summary{Workspace: v.Workspace, ID: v.ID, OwnerKind: v.OwnerKind, OwnerID: v.OwnerID, Action: v.Action, Status: v.Status, Artifacts: len(v.Artifacts), Dismissed: v.ResultDismissed, Version: v.Version, Created: v.Created, Updated: v.Updated, Deadline: v.Deadline}, nil
}

// scanRoot brings the index up to date with one workspace's folder and
// returns the summaries that changed, and the first record it could not
// read.
func (b *Broker) scanRoot(ix *index, root string) (changed []Summary, first error) {
	// An entry is dropped only when it was indexed before the folder was
	// listed and is not in the listing, so that a record saved during the
	// scan is kept.
	before := ix.keys(func(key string) bool { return keyRoot(key) == filepath.Base(root) })
	ids, sigs, err := b.signatures(root)
	if err != nil {
		return nil, err
	}
	ix.loadCache(b)
	present := map[string]bool{}
	for _, id := range ids {
		key := indexKey(root, id)
		present[key] = true
		// A record whose files are as they were when its summary was
		// last taken is not read again (index_cache.go).
		s, known := Summary{}, false
		if c, ok := ix.cacheGet(key); ok && c.Sig.same(sigs[id]) {
			s, known = c.Summary, true
		}
		if !known {
			s, err = b.readSummary(root, id)
			if err != nil {
				if first == nil {
					first = err
				}
				continue
			}
			ix.cacheSet(key, cachedSummary{Sig: sigs[id], Summary: s})
		}
		if ix.merge(key, s) {
			changed = append(changed, s)
		}
	}
	for _, key := range before {
		if !present[key] {
			ix.remove(key)
			ix.cacheRemove(key)
		}
	}
	return changed, first
}

// loadCache reads the index's cache of summaries by signature from its file
// the first time it is needed.
func (ix *index) loadCache(b *Broker) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if ix.cache == nil {
		ix.cache = b.readIndexCache()
	}
}

func (ix *index) cacheGet(key string) (cachedSummary, bool) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	c, ok := ix.cache[key]
	return c, ok
}

func (ix *index) cacheSet(key string, c cachedSummary) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	ix.cache[key] = c
	ix.cacheDirty = true
}

func (ix *index) cacheRemove(key string) {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if _, ok := ix.cache[key]; ok {
		delete(ix.cache, key)
		ix.cacheDirty = true
	}
}

// writeCache writes the cache when it changed since it was last written.
func (b *Broker) writeCache(ix *index) {
	ix.mu.Lock()
	if !ix.cacheDirty || ix.cache == nil {
		ix.mu.Unlock()
		return
	}
	ix.cacheDirty = false
	entries := maps.Clone(ix.cache)
	ix.mu.Unlock()
	_ = b.writeIndexCache(entries)
}

// scan brings the index up to date with every workspace's folder.
func (b *Broker) scan(ix *index) (changed []Summary, first error, err error) {
	before := ix.keys(func(string) bool { return true })
	entries, err := os.ReadDir(b.appletsDir())
	if errors.Is(err, os.ErrNotExist) {
		entries, err = nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	roots := map[string]bool{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		roots[entry.Name()] = true
		c, e := b.scanRoot(ix, filepath.Join(b.appletsDir(), entry.Name()))
		changed = append(changed, c...)
		if first == nil {
			first = e
		}
	}
	for _, key := range before {
		if !roots[keyRoot(key)] {
			ix.remove(key)
			ix.cacheRemove(key)
		}
	}
	b.writeCache(ix)
	return changed, first, nil
}

// ready returns the state folder's index, built.
func (b *Broker) ready() (*index, error) {
	ix := b.index()
	ix.build.Lock()
	defer ix.build.Unlock()
	ix.mu.Lock()
	built := ix.built
	ix.mu.Unlock()
	if built {
		return ix, nil
	}
	_, first, err := b.scan(ix)
	if err != nil {
		return nil, err
	}
	ix.mu.Lock()
	ix.built, ix.scanErr = true, first
	ix.mu.Unlock()
	return ix, nil
}

// Attention lists, from every workspace, the invocations include accepts,
// oldest first, reading in full only those it lists. An invocation still
// running or waiting is first recovered or timed out, as Get does, so that
// one a stopped program left is not listed as it was. A record that cannot
// be read is left out rather than failing the list, so that one damaged
// record does not keep every question and result from Notifications; the
// error is returned only when no record could be read at all.
func (b *Broker) Attention(include func(Summary) bool) ([]Invocation, error) {
	ix, err := b.ready()
	if err != nil {
		return nil, err
	}
	list := []Invocation{}
	entries := ix.snapshot()
	for _, s := range entries {
		if TerminalStatus(s.Status) && !include(s) {
			continue
		}
		var v Invocation
		if TerminalStatus(s.Status) && b.failure(b.key(s.Workspace, s.ID)) == nil {
			// A finished invocation is not recovered or timed out.
			v, err = b.Read(s.Workspace, s.ID)
			v = PublicView(v)
		} else {
			v, err = b.Get(s.Workspace, s.OwnerKind, s.OwnerID, s.ID)
		}
		if err != nil {
			if !b.store(s.Workspace).Exists(s.ID) {
				ix.remove(b.indexKey(s.Workspace, s.ID))
			}
			continue
		}
		if include(Summarize(v)) {
			list = append(list, v)
		}
	}
	ix.mu.Lock()
	first := ix.scanErr
	ix.mu.Unlock()
	if len(entries) == 0 && first != nil {
		return nil, first
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Created.Before(list[j].Created) })
	return list, nil
}
