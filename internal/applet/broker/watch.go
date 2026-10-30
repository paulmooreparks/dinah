package broker

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"studio/internal/layout"
	"studio/internal/records"
)

// watchQuiet is how long the invocation folders must be quiet before the
// records that changed in them are read, so that a burst of saves is read
// once and a record is not read while it is being written.
const watchQuiet = 50 * time.Millisecond

// watch follows the invocation folders of a state folder through fsnotify,
// which uses each system's documented change notifications
// (ReadDirectoryChangesW on Windows, inotify on Linux, kqueue on macOS).
type watch struct {
	fw   *fsnotify.Watcher
	done chan struct{}
	// reading is held while changes are read, so that Close returns only
	// once nothing more is read.
	reading sync.Mutex

	mu      sync.Mutex
	records map[[2]string]bool
	roots   map[string]bool
	all     bool
	timer   *time.Timer
}

// Watch keeps the index current with the changes another program, such as
// the command line, makes to the saved invocations, and tells Emit
// "applet:invocation", without an invocation, after each burst of them. It
// watches the applets folder, each workspace's folder in it, and the folders
// between that and its records. When the system reports that it may have
// missed a change, every folder is read again. Close stops it.
func (b *Broker) Watch() error {
	ix := b.index()
	ix.mu.Lock()
	if ix.watch != nil {
		ix.mu.Unlock()
		return nil
	}
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		ix.mu.Unlock()
		return err
	}
	w := &watch{fw: fw, done: make(chan struct{}), records: map[[2]string]bool{}, roots: map[string]bool{}}
	ix.watch = w
	ix.mu.Unlock()
	fail := func(err error) error {
		ix.mu.Lock()
		ix.watch = nil
		ix.mu.Unlock()
		fw.Close()
		return err
	}
	if err := os.MkdirAll(b.appletsDir(), 0o755); err != nil {
		return fail(err)
	}
	if err := fw.Add(b.appletsDir()); err != nil {
		return fail(err)
	}
	entries, err := os.ReadDir(b.appletsDir())
	if err != nil {
		return fail(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			b.watchRoot(fw, filepath.Join(b.appletsDir(), entry.Name()))
		}
	}
	go b.follow(ix, w)
	// What changed before the folders were watched is read now.
	w.mu.Lock()
	w.all = true
	w.schedule(b, ix)
	w.mu.Unlock()
	return nil
}

// Close stops watching the invocation folders.
func (b *Broker) Close() error {
	ix := b.index()
	ix.mu.Lock()
	w := ix.watch
	ix.watch = nil
	ix.mu.Unlock()
	if w == nil {
		return nil
	}
	err := w.fw.Close()
	<-w.done
	w.mu.Lock()
	if w.timer != nil {
		w.timer.Stop()
	}
	w.mu.Unlock()
	w.reading.Lock()
	w.reading.Unlock()
	return err
}

// watchDirs are a workspace's folder and the folders down to its records.
func (b *Broker) watchDirs(root string) []string {
	runs := layout.Layout{State: b.State}.RunDir(root)
	dirs := []string{runs}
	for d := filepath.Dir(runs); d != root && layout.Inside(root, d); d = filepath.Dir(d) {
		dirs = append([]string{d}, dirs...)
	}
	return append([]string{root}, dirs...)
}

// watchRoot watches those of a workspace's folders that exist; the others
// are watched as they are made.
func (b *Broker) watchRoot(fw *fsnotify.Watcher, root string) {
	for _, dir := range b.watchDirs(root) {
		if err := fw.Add(dir); err != nil {
			return
		}
	}
}

func (b *Broker) follow(ix *index, w *watch) {
	defer close(w.done)
	for {
		select {
		case ev, ok := <-w.fw.Events:
			if !ok {
				return
			}
			b.changed(ix, w, ev)
		case _, ok := <-w.fw.Errors:
			if !ok {
				return
			}
			w.mu.Lock()
			w.all = true
			w.schedule(b, ix)
			w.mu.Unlock()
		}
	}
}

// changed notes the record or the workspace folder an event is about.
func (b *Broker) changed(ix *index, w *watch, ev fsnotify.Event) {
	rel, err := filepath.Rel(b.appletsDir(), ev.Name)
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		return
	}
	root := filepath.Join(b.appletsDir(), strings.Split(rel, string(filepath.Separator))[0])
	w.mu.Lock()
	defer w.mu.Unlock()
	if filepath.Dir(ev.Name) == (layout.Layout{State: b.State}).RunDir(root) {
		if id := records.ID(filepath.Base(ev.Name)); recordID.MatchString(id) {
			w.records[[2]string{root, id}] = true
			w.schedule(b, ix)
		}
		return
	}
	// A workspace folder, or one between it and its records, was made or
	// removed: what is in it is read again, and a new one is watched.
	for _, dir := range b.watchDirs(root) {
		if dir == ev.Name {
			if ev.Has(fsnotify.Create) {
				b.watchRoot(w.fw, root)
			}
			w.roots[root] = true
			w.schedule(b, ix)
			return
		}
	}
}

// schedule reads what changed once the folders have been quiet for
// watchQuiet. The caller holds w.mu.
func (w *watch) schedule(b *Broker, ix *index) {
	if w.timer != nil {
		w.timer.Stop()
	}
	w.timer = time.AfterFunc(watchQuiet, func() { b.flush(ix, w) })
}

// flush reads the records and folders that changed into the index, and
// tells Emit once if any summary changed.
func (b *Broker) flush(ix *index, w *watch) {
	w.reading.Lock()
	defer w.reading.Unlock()
	w.mu.Lock()
	all, roots, recs := w.all, w.roots, w.records
	w.all, w.roots, w.records = false, map[string]bool{}, map[[2]string]bool{}
	w.mu.Unlock()
	ix.mu.Lock()
	stopped := ix.watch != w
	ix.mu.Unlock()
	if stopped {
		return
	}
	changed := false
	if all {
		ix.build.Lock()
		ix.mu.Lock()
		built := ix.built
		ix.mu.Unlock()
		before := len(ix.snapshot())
		c, first, err := b.scan(ix)
		if err == nil {
			ix.mu.Lock()
			ix.built = true
			if !built {
				ix.scanErr = first
			}
			ix.mu.Unlock()
		}
		ix.build.Unlock()
		changed = len(c) > 0 || len(ix.snapshot()) != before
	}
	for root := range roots {
		if all {
			break
		}
		before := len(ix.snapshot())
		c, _ := b.scanRoot(ix, root)
		changed = changed || len(c) > 0 || len(ix.snapshot()) != before
		b.writeCache(ix)
	}
	for rec := range recs {
		if all || roots[rec[0]] {
			continue
		}
		root, id := rec[0], rec[1]
		key := indexKey(root, id)
		s, err := b.readSummary(root, id)
		switch {
		case err == nil:
			changed = ix.merge(key, s) || changed
		case errors.Is(err, os.ErrNotExist) || !(layout.Layout{State: b.State}).Runs(root).Exists(id):
			changed = ix.remove(key) || changed
		}
	}
	if changed {
		b.emit("applet:invocation", nil)
	}
}
