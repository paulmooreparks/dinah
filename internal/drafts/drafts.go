// Package drafts keeps the working copies of documents that people and agents
// edit in Dinah: a document opened for editing becomes a draft, bound to
// the resource it came from and to the version of that resource it started
// from (its baseline). Drafts survive restarts. Every change is made against
// the revision the caller last read, so neither a person nor an agent can
// overwrite a change they have not seen.
//
// The store is shared by the desktop app and the command line. It keeps one
// JSON file per draft in a directory the caller chooses, written by atomic
// replacement, and serialises writers to the same draft, across processes,
// with an exclusive lock file.
package drafts

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"studio/internal/resource"
)

// Origin is the resource a draft edits.
type Origin struct {
	// Kind is the kind of source the resource belongs to, as its provider
	// calls it ("local-folder" and so on), or OriginNew for a document that
	// exists only as a draft so far. The store does not interpret it, so a
	// new kind of source needs no change here.
	Kind        string `json:"kind"`
	WorkspaceID string `json:"workspaceId,omitempty"`
	SourceID    string `json:"sourceId,omitempty"`
	// Path is the resource's path within its source.
	Path string `json:"path,omitempty"`
}

// Baseline is the resource as it was when the draft was made, or when it was
// last saved or published. Saving and publishing compare the resource's
// current state with it to find changes made elsewhere in the meantime.
type Baseline struct {
	// Text is the Markdown the draft started from.
	Text string `json:"text"`
	// Revision identifies the resource's state: the SHA-256 of a local file's
	// content, or the version number a remote service gives a page.
	Revision string `json:"revision"`
	// Storage is the resource's stored form at that version, for a source
	// that keeps another form than the text, which a block-level publication
	// needs.
	Storage string `json:"storage,omitempty"`
	// Title is the resource's title at that version.
	Title string `json:"title,omitempty"`
	// Labels are a page's labels at that version, which the document type is
	// recognised by.
	Labels []string `json:"labels,omitempty"`
}

// Draft is one working copy.
type Draft struct {
	ID       string   `json:"id"`
	Origin   Origin   `json:"origin"`
	Baseline Baseline `json:"baseline"`
	Title    string   `json:"title"`
	Text     string   `json:"text"`
	// Revision is Revision(Text); a change must name it.
	Revision string    `json:"revision"`
	Created  time.Time `json:"created"`
	Updated  time.Time `json:"updated"`
	// Suggestions are the changes proposed to Text and not yet accepted or
	// rejected. Every change of Text finds them again in the new text.
	Suggestions []Suggestion `json:"suggestions,omitempty"`
}

// Changed reports whether the draft differs from its baseline: its text, or a
// title the source keeps (a source that keeps no title records the one it was
// given, so a title edit there does not count as unsaved).
func (d Draft) Changed() bool { return d.Text != d.Baseline.Text || d.Title != d.Baseline.Title }

// InProgress reports whether the draft holds something not yet saved: a
// change, suggestions to decide, or, for a new document, any text at all. Any
// other draft is only the working copy of a document as it is stored, and is
// not shown to anyone as a draft.
func (d Draft) InProgress() bool {
	if len(d.Suggestions) > 0 {
		return true
	}
	if d.Origin.Kind == OriginNew {
		return strings.TrimSpace(d.Text) != ""
	}
	return d.Changed()
}

// OriginNew is the origin kind of a document that exists only as a draft.
const OriginNew = "new"

// ErrConflict means a change named a revision that is no longer current.
var ErrConflict = errors.New("the draft has changed since it was read; read it again")

// ErrNotFound means there is no draft with that ID.
var ErrNotFound = errors.New("the draft does not exist")

// MaxText bounds a draft's text, matching the resource service's read limit.
const MaxText = 1 << 20

// Revision is the revision of a draft's text. It is resource.ContentRevision,
// the value every text resource reports, so a draft and the file it edits can
// be compared directly.
func Revision(text string) string { return resource.ContentRevision(text) }

// Store keeps drafts in one directory.
type Store struct {
	dir string
	now func() time.Time
	// lockWait bounds how long a writer waits for another's lock; lockStale
	// is the age after which a lock left by a crashed writer is taken over.
	lockWait, lockStale time.Duration
}

// Open opens the store in dir, creating it when needed.
func Open(dir string) (*Store, error) {
	if dir == "" {
		return nil, errors.New("the draft store needs a directory")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("the draft store could not be created: %w", err)
	}
	return &Store{dir: dir, now: time.Now, lockWait: 5 * time.Second, lockStale: 30 * time.Second}, nil
}

var idRe = regexp.MustCompile(`^[0-9a-f]{32}$`)

func (s *Store) file(id string) (string, error) {
	if !idRe.MatchString(id) {
		return "", ErrNotFound
	}
	return filepath.Join(s.dir, id+".json"), nil
}

func newID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func checkText(text string) error {
	if len(text) > MaxText {
		return fmt.Errorf("a draft is limited to %d bytes", MaxText)
	}
	if strings.ContainsRune(text, '\x00') {
		return errors.New("a draft cannot contain NUL characters")
	}
	return nil
}

// Create makes a draft of origin from baseline. Its text starts as the
// baseline's text unless text is given, as when a file from the computer
// fills a new document.
func (s *Store) Create(origin Origin, baseline Baseline, title string, text *string) (Draft, error) {
	if origin.Kind == "" || len(origin.Kind) > 64 {
		return Draft{}, errors.New("a draft's origin needs the kind of its source")
	}
	body := baseline.Text
	if text != nil {
		body = *text
	}
	if err := checkText(body); err != nil {
		return Draft{}, err
	}
	if err := checkText(baseline.Text); err != nil {
		return Draft{}, err
	}
	id, err := newID()
	if err != nil {
		return Draft{}, err
	}
	now := s.now().UTC()
	d := Draft{ID: id, Origin: origin, Baseline: baseline, Title: title, Text: body, Revision: Revision(body), Created: now, Updated: now}
	if d.Title == "" {
		d.Title = baseline.Title
	}
	unlock, err := s.lock(id)
	if err != nil {
		return Draft{}, err
	}
	defer unlock()
	return d, s.write(d)
}

// Get reads a draft.
func (s *Store) Get(id string) (Draft, error) {
	name, err := s.file(id)
	if err != nil {
		return Draft{}, err
	}
	data, err := os.ReadFile(name)
	if errors.Is(err, os.ErrNotExist) {
		return Draft{}, ErrNotFound
	}
	if err != nil {
		return Draft{}, err
	}
	var d Draft
	if err := json.Unmarshal(data, &d); err != nil {
		return Draft{}, fmt.Errorf("the draft %s is damaged: %w", id, err)
	}
	if d.ID != id || d.Revision != Revision(d.Text) {
		return Draft{}, fmt.Errorf("the draft %s is damaged: its identity or revision does not match its content", id)
	}
	return d, nil
}

// List returns every draft, most recently updated first. A damaged draft file
// is skipped rather than hiding the others; Get reports it.
func (s *Store) List() ([]Draft, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	out := []Draft{}
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".json")
		if !ok || e.IsDir() || !idRe.MatchString(id) {
			continue
		}
		if d, err := s.Get(id); err == nil {
			out = append(out, d)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Updated.After(out[j].Updated) })
	return out, nil
}

// FindByOrigin returns the drafts of one resource, most recent first.
func (s *Store) FindByOrigin(origin Origin) ([]Draft, error) {
	all, err := s.List()
	if err != nil {
		return nil, err
	}
	out := []Draft{}
	for _, d := range all {
		if d.Origin == origin {
			out = append(out, d)
		}
	}
	return out, nil
}

// Update replaces a draft's text and title, provided expected is its current
// revision; otherwise it returns ErrConflict and changes nothing. A title of
// nil leaves the title as it is.
func (s *Store) Update(id, expected, text string, title *string) (Draft, error) {
	if err := checkText(text); err != nil {
		return Draft{}, err
	}
	return s.change(id, expected, func(d *Draft) error {
		d.Text = text
		if title != nil {
			d.Title = *title
		}
		return nil
	})
}

// Rebase records that the draft's resource has been saved or published:
// baseline becomes the new baseline and origin may gain an identity it did not
// have (a new document's first publication). The text is unchanged, so the
// caller names the revision it saved.
func (s *Store) Rebase(id, expected string, origin *Origin, baseline Baseline) (Draft, error) {
	if err := checkText(baseline.Text); err != nil {
		return Draft{}, err
	}
	return s.change(id, expected, func(d *Draft) error {
		// A draft saved to a new place takes that place's title too: a new
		// document saved as plan.md is plan.md, not Untitled (23 October 2026).
		if origin != nil {
			d.Origin = *origin
			d.Title = baseline.Title
		}
		d.Baseline = baseline
		return nil
	})
}

// Rehome moves every draft of the resource from to the resource to, as when
// a file is renamed: each draft's origin becomes to, and its baseline title
// becomes title, which is the file's new name. A draft whose title was the
// old name takes the new one too; a title the person changed stays. The text
// is not changed, so no revision is needed. It returns the drafts it changed.
func (s *Store) Rehome(from, to Origin, title string) ([]Draft, error) {
	all, err := s.FindByOrigin(from)
	if err != nil {
		return nil, err
	}
	var out []Draft
	for _, d := range all {
		changed, err := s.changeSuggestions(d.ID, func(d *Draft) error {
			// The draft is read again under its lock, and is left as it is
			// when it no longer edits from.
			if d.Origin != from {
				return nil
			}
			if d.Title == d.Baseline.Title {
				d.Title = title
			}
			d.Origin = to
			d.Baseline.Title = title
			return nil
		})
		if err != nil {
			return out, err
		}
		out = append(out, changed)
	}
	return out, nil
}

// ErrTouched means a draft holds something of the person's (a change to its
// text or title, or suggestions to decide), so it is not refreshed.
var ErrTouched = errors.New("the draft has changes of its own")

// Refresh replaces an untouched draft's baseline and text with the resource
// as it is now, so a draft reused for reading shows the current version. A
// draft that holds anything of the person's is left as it is, with
// ErrTouched; the check is made under the draft's lock.
func (s *Store) Refresh(id, expected string, baseline Baseline) (Draft, error) {
	if err := checkText(baseline.Text); err != nil {
		return Draft{}, err
	}
	return s.change(id, expected, func(d *Draft) error {
		if d.Changed() || len(d.Suggestions) > 0 {
			return ErrTouched
		}
		d.Baseline = baseline
		d.Text = baseline.Text
		d.Title = baseline.Title
		return nil
	})
}

// Sweep forgets the drafts that hold nothing of the person's and have not
// changed since before: one surface makes a draft for every document opened
// to read, and an unchanged one is made again from the resource when it is
// next opened. A new document's draft stays unless nothing was ever written
// in it, as with an empty document started from Tools › Editor and left; a
// draft with changes or suggestions, and a draft keep says to keep, stay. It
// returns how many were forgotten.
func (s *Store) Sweep(before time.Time, keep func(Draft) bool) (int, error) {
	all, err := s.List()
	if err != nil {
		return 0, err
	}
	gone := 0
	for _, d := range all {
		if !sweepable(d, before) || keep != nil && keep(d) {
			continue
		}
		name, err := s.file(d.ID)
		if err != nil {
			continue
		}
		unlock, err := s.lock(d.ID)
		if err != nil {
			continue
		}
		// Read again under the lock, so a change made since the list stays.
		if again, err := s.Get(d.ID); err == nil && sweepable(again, before) && os.Remove(name) == nil {
			s.removeFiles(d.ID, nil)
			gone++
		}
		unlock()
	}
	return gone, nil
}

func sweepable(d Draft, before time.Time) bool {
	return !d.InProgress() && d.Updated.Before(before)
}

// Discard deletes a draft, provided expected is its current revision.
func (s *Store) Discard(id, expected string) error {
	name, err := s.file(id)
	if err != nil {
		return err
	}
	unlock, err := s.lock(id)
	if err != nil {
		return err
	}
	defer unlock()
	d, err := s.Get(id)
	if err != nil {
		return err
	}
	if d.Revision != expected {
		return ErrConflict
	}
	if err := os.Remove(name); err != nil {
		return err
	}
	s.removeFiles(id, nil)
	return nil
}

func (s *Store) change(id, expected string, apply func(*Draft) error) (Draft, error) {
	if _, err := s.file(id); err != nil {
		return Draft{}, err
	}
	unlock, err := s.lock(id)
	if err != nil {
		return Draft{}, err
	}
	defer unlock()
	d, err := s.Get(id)
	if err != nil {
		return Draft{}, err
	}
	if d.Revision != expected {
		return Draft{}, ErrConflict
	}
	before := d.Text
	if err := apply(&d); err != nil {
		return Draft{}, err
	}
	if d.Text != before {
		d.Suggestions = rebase(d.Suggestions, d.Text)
	}
	d.Revision = Revision(d.Text)
	d.Updated = s.now().UTC()
	return d, s.write(d)
}

// changeSuggestions changes a draft's suggestions and nothing else, so it
// needs no revision: the text it was read at does not matter to them.
func (s *Store) changeSuggestions(id string, apply func(*Draft) error) (Draft, error) {
	if _, err := s.file(id); err != nil {
		return Draft{}, err
	}
	unlock, err := s.lock(id)
	if err != nil {
		return Draft{}, err
	}
	defer unlock()
	d, err := s.Get(id)
	if err != nil {
		return Draft{}, err
	}
	if err := apply(&d); err != nil {
		return Draft{}, err
	}
	d.Updated = s.now().UTC()
	return d, s.write(d)
}

// write replaces the draft's file atomically: a complete temporary file in the
// same directory is renamed over it, so a reader sees the old draft or the new
// one, never part of either.
func (s *Store) write(d Draft) error {
	name, err := s.file(d.ID)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, d.ID+".*.tmp")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	serr := tmp.Sync()
	cerr := tmp.Close()
	if err := errors.Join(werr, serr, cerr); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("the draft could not be saved: %w", err)
	}
	if err := os.Rename(tmp.Name(), name); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("the draft could not be saved: %w", err)
	}
	return nil
}

// lock takes the draft's lock file, which exists only while one writer holds
// it. Creating a file with O_EXCL fails when it already exists, on every
// platform Go supports, so two writers cannot both hold it. A lock older than
// lockStale was left by a writer that stopped, and is taken over.
//
// On Windows a lock file that its holder has just removed stays pending
// deletion until every handle to it is closed, and creating it meanwhile fails
// with access denied (the documented behaviour of DeleteFile), so a refusal of
// permission is waited out like a held lock, and reported only if it outlasts
// the wait.
func (s *Store) lock(id string) (func(), error) {
	name := filepath.Join(s.dir, id+".lock")
	deadline := s.now().Add(s.lockWait)
	for {
		f, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			f.Close()
			return func() { os.Remove(name) }, nil
		}
		if errors.Is(err, os.ErrPermission) {
			if s.now().After(deadline) {
				return nil, fmt.Errorf("the draft could not be locked: %w", err)
			}
			time.Sleep(20 * time.Millisecond)
			continue
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("the draft could not be locked: %w", err)
		}
		if st, serr := os.Stat(name); serr == nil && s.now().Sub(st.ModTime()) > s.lockStale {
			os.Remove(name)
			continue
		}
		if s.now().After(deadline) {
			return nil, errors.New("the draft is being changed elsewhere; try again")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
