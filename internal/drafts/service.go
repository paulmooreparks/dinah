package drafts

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	pathpkg "path"
	"slices"
	"strings"

	"studio/internal/docfiles"
	"studio/internal/keylock"
	"studio/internal/resource"
	"studio/internal/textdiff"
)

// Providers resolves a draft's origin to the provider of its source. The
// desktop app and the command line each supply one; the service works only
// through the capabilities the provider offers (resource.Snapshotter,
// resource.DocumentWriter, resource.Publisher and later ones), so a new kind
// of source needs no change here.
type Providers interface {
	Provider(ctx context.Context, origin Origin) (any, error)
}

// Service opens resources as drafts and writes drafts back to them.
type Service struct {
	Store     *Store
	Providers Providers
	// writes is held, by draft, while the draft is written to its resource and
	// its baseline is moved to what was written, and while it is compared with
	// the resource. A comparison made between the two would see the resource
	// changed and the baseline not, and report the draft's own write as a change
	// made elsewhere.
	writes keylock.Set
	// Admit, when set, is asked before a draft is written to its resource. It
	// refuses the write with an error, or returns what to call when the write
	// has ended, so that whoever supplies it knows the writes that are under
	// way and can stop admitting new ones.
	Admit func() (func(), error)
}

// admit asks Admit, and admits every write when there is none.
func (s *Service) admit() (func(), error) {
	if s.Admit == nil {
		return func() {}, nil
	}
	return s.Admit()
}

// The operations a draft offers, as View.Actions lists them. Save and Publish
// are the same operation, Service.Save; a draft offers Publish instead of Save
// when its source is a resource.Publisher.
const (
	ActionUpdate  = "update"
	ActionSave    = "save"
	ActionPublish = "publish"
	ActionDiscard = "discard"
	// ActionSaveAs writes the draft to a new resource; see Service.SaveAs.
	ActionSaveAs = "save-as"
	// ActionRedrawPictures draws again the pictures a source keeps of the
	// diagrams a document shows; see resource.PictureRedrawer. It is offered
	// for a document of such a source whether or not the draft has changes.
	ActionRedrawPictures = "redraw-pictures"
)

// View is a draft with the operations currently legal for it, so that a
// caller offers only what will work rather than finding out by failing.
type View struct {
	Draft
	Changed bool     `json:"changed"`
	Actions []string `json:"actions"`
	// WriteBack is how the draft's source takes a write, ActionSave or
	// ActionPublish, or "" when it cannot take one, whether or not the draft
	// has changed, so an editor labels its button before there is anything
	// to write.
	WriteBack string `json:"writeBack"`
	// Spans is where each suggestion that is still in place sits in the
	// text, counted in UTF-16 code units, as an editor in the page counts.
	Spans map[string][2]int `json:"spans"`
}

// View describes a draft for a caller.
func (s *Service) View(ctx context.Context, d Draft) View {
	v := View{Draft: d, Changed: d.Changed(), Actions: []string{ActionUpdate, ActionDiscard, ActionSaveAs}, Spans: d.Spans()}
	if v.Suggestions == nil {
		v.Suggestions = []Suggestion{}
	}
	if d.Origin.Kind != OriginNew {
		if p, err := s.Providers.Provider(ctx, d.Origin); err == nil {
			if _, ok := p.(resource.DocumentWriter); ok {
				v.WriteBack = ActionSave
				if pub, ok := p.(resource.Publisher); ok && pub.Publishes() {
					v.WriteBack = ActionPublish
				}
			}
			if r, ok := p.(resource.PictureRedrawer); ok && r.RedrawsPictures(d.Origin.Path) {
				v.Actions = append(v.Actions, ActionRedrawPictures)
			}
		}
	}
	if v.Changed && v.WriteBack != "" {
		v.Actions = append(v.Actions, v.WriteBack)
	}
	return v
}

func (d Draft) snapshot() resource.Snapshot {
	return resource.Snapshot{Text: d.Baseline.Text, Revision: d.Baseline.Revision, Title: d.Baseline.Title, Native: d.Baseline.Storage}
}

// identifier is what says which paths of the origin's source are one
// resource, or nil when the source has one resource for each path.
func (s *Service) identifier(ctx context.Context, origin Origin) resource.Identifier {
	if s.Providers == nil {
		return nil
	}
	p, err := s.Providers.Provider(ctx, origin)
	if err != nil {
		return nil
	}
	id, _ := p.(resource.Identifier)
	return id
}

// Find returns the drafts of a resource, most recent first. In a source that
// says what its resources are the same by (resource.Identifier), a draft
// opened by another path to the same resource is the resource's draft too.
// One page of a wiki, asked for by its own ID and by the IDs of the pages
// down to it, had two drafts, and an agent's suggestions went to the one the
// person was not looking at (Paul, 13 November 2026). A draft keeps the path
// it was opened by.
func (s *Service) Find(ctx context.Context, origin Origin) ([]Draft, error) {
	id := s.identifier(ctx, origin)
	if id == nil {
		return s.Store.FindByOrigin(origin)
	}
	want := id.Identity(origin.Path)
	return s.Store.find(func(o Origin) bool {
		path := o.Path
		o.Path = origin.Path
		return o == origin && (path == origin.Path || id.Identity(path) == want)
	})
}

// Open returns the most recent draft of a resource, or makes one from a
// snapshot of it. Reopening a resource therefore continues the work in
// progress instead of starting again from the resource.
func (s *Service) Open(ctx context.Context, origin Origin) (Draft, error) {
	if origin.Kind == OriginNew {
		return Draft{}, errors.New("a new document is created, not opened")
	}
	// Two opens of one resource at once, from two windows or a window and an
	// agent, must find one draft between them, so finding and creating it
	// hold the store's lock for that resource (9 November 2026).
	// The lock is the resource's, whichever of its paths it was asked for by.
	held := origin
	if id := s.identifier(ctx, origin); id != nil {
		held.Path = id.Identity(origin.Path)
	}
	unlock, err := s.Store.lockOrigin(held)
	if err != nil {
		return Draft{}, err
	}
	defer unlock()
	existing, err := s.Find(ctx, origin)
	if err != nil {
		return Draft{}, err
	}
	if len(existing) > 0 {
		return s.refreshed(ctx, existing[0]), nil
	}
	p, err := s.Providers.Provider(ctx, origin)
	if err != nil {
		return Draft{}, err
	}
	snap, ok := p.(resource.Snapshotter)
	if !ok {
		return Draft{}, resource.ErrUnsupported
	}
	shot, err := snap.Snapshot(ctx, origin.Path)
	if err != nil {
		return Draft{}, err
	}
	return s.Store.Create(origin, baselineOf(shot), shot.Title, nil)
}

// baselineOf is a resource's snapshot as a draft keeps it.
func baselineOf(shot resource.Snapshot) Baseline {
	return Baseline{Text: shot.Text, Revision: shot.Revision, Storage: shot.Native, Title: shot.Title, Labels: shot.Labels}
}

// refreshed is a reused draft brought up to its resource as it is now, when
// the draft holds nothing of the person's and the resource has moved on. A
// draft with changes is returned as it is, and its window says the resource
// moved on; so is any draft whose resource cannot be read now, since opening
// it must not depend on the connection.
func (s *Service) refreshed(ctx context.Context, d Draft) Draft {
	if d.Changed() || len(d.Suggestions) > 0 {
		return d
	}
	p, err := s.Providers.Provider(ctx, d.Origin)
	if err != nil {
		return d
	}
	snap, ok := p.(resource.Snapshotter)
	if !ok {
		return d
	}
	shot, err := snap.Snapshot(ctx, d.Origin.Path)
	if err != nil || shot.Revision == d.Baseline.Revision {
		return d
	}
	fresh, err := s.Store.Refresh(d.ID, d.Revision, baselineOf(shot))
	if err != nil {
		return d
	}
	return fresh
}

// Save writes a draft back to its resource (saving a file, publishing a page),
// provided the draft is still at expected and the resource is still at the
// draft's baseline, so neither a later change to the draft nor a change made
// to the resource elsewhere is overwritten. What was written becomes the new
// baseline. The notes say what the write could not keep.
func (s *Service) Save(ctx context.Context, id, expected string) (Draft, []string, error) {
	done, err := s.admit()
	if err != nil {
		return Draft{}, nil, err
	}
	defer done()
	defer s.writes.Lock(id)()
	d, err := s.Store.Get(id)
	if err != nil {
		return Draft{}, nil, err
	}
	if d.Revision != expected {
		return Draft{}, nil, ErrConflict
	}
	if d.Origin.Kind == OriginNew {
		return Draft{}, nil, errors.New("choose where to save a new document first")
	}
	p, err := s.Providers.Provider(ctx, d.Origin)
	if err != nil {
		return Draft{}, nil, err
	}
	w, ok := p.(resource.DocumentWriter)
	if !ok {
		return Draft{}, nil, resource.ErrUnsupported
	}
	files, err := s.keptFiles(d)
	if err != nil {
		return Draft{}, nil, err
	}
	written, err := w.WriteDocument(ctx, d.Origin.Path, d.snapshot(), resource.Edit{Text: d.Text, Title: d.Title, Files: files})
	if err != nil {
		return Draft{}, nil, err
	}
	s.Store.removeFiles(id, fileNames(files))
	shot := written.Snapshot
	saved, err := s.Store.Rebase(id, expected, nil, baselineOf(shot))
	return saved, written.Notes, err
}

// AddFile keeps a file with a draft, to be saved with it, and returns the
// name it is kept under, which is not a name the draft's document already
// has a file of, so that saving it never replaces one.
func (s *Service) AddFile(ctx context.Context, id, name string, data []byte) (string, error) {
	d, err := s.Store.Get(id)
	if err != nil {
		return "", err
	}
	var taken []string
	if d.Origin.Kind != "" && d.Origin.Kind != OriginNew {
		if p, err := s.Providers.Provider(ctx, d.Origin); err == nil {
			if src, ok := p.(resource.FileSource); ok {
				if taken, err = src.FileNames(ctx, d.Origin.Path); err != nil {
					return "", fmt.Errorf("the files already beside the document could not be listed: %w", err)
				}
			}
		}
	}
	return s.Store.AddFile(id, name, data, taken)
}

// keptFiles reads the files kept with a draft that its text uses.
func (s *Service) keptFiles(d Draft) ([]resource.File, error) {
	names, err := s.Store.FileNames(d.ID)
	if err != nil {
		return nil, err
	}
	var files []resource.File
	for _, name := range names {
		if !uses(d.Text, name) {
			continue
		}
		data, err := s.Store.ReadFile(d.ID, name)
		if err != nil {
			return nil, fmt.Errorf("the file %s kept with the draft could not be read: %w", name, err)
		}
		files = append(files, resource.File{Name: name, Data: data})
	}
	return files, nil
}

// uses reports whether text refers to a file by its name, as written or
// escaped as a link writes it.
func uses(text, name string) bool {
	return strings.Contains(text, name) || strings.Contains(text, url.PathEscape(name))
}

func fileNames(files []resource.File) []string {
	names := make([]string, len(files))
	for i, f := range files {
		names[i] = f.Name
	}
	return names
}

// Comparison is a draft beside its resource as the resource is now: whether
// the resource has moved on from the draft's baseline, what changed there
// and what the draft changed, each as a unified diff from the baseline, and
// the two merged, with the regions both changed differently.
type Comparison struct {
	Moved    bool   `json:"moved"`
	Revision string `json:"revision"`
	Theirs   string `json:"theirs"`
	Mine     string `json:"mine"`
	Merged   string `json:"merged"`
	// Draft and Current are the two texts themselves, the draft's and the
	// resource's as it is now, for showing them side by side.
	Draft     string              `json:"draft"`
	Current   string              `json:"current"`
	Title     string              `json:"title"`
	Conflicts []textdiff.Conflict `json:"conflicts"`
	remote    resource.Snapshot
}

// ErrMergeConflict means the resource and the draft changed the same part of
// the document differently, so the resource's changes cannot be brought into
// the draft without the person choosing.
var ErrMergeConflict = errors.New("the document and the draft both changed the same part")

// Compare reads the draft's resource as it is now and compares it with the
// draft, without changing either.
func (s *Service) Compare(ctx context.Context, id string) (Comparison, error) {
	defer s.writes.Lock(id)()
	return s.compare(ctx, id)
}

// compare is Compare for a caller that holds the draft's writes lock.
func (s *Service) compare(ctx context.Context, id string) (Comparison, error) {
	d, err := s.Store.Get(id)
	if err != nil {
		return Comparison{}, err
	}
	if d.Origin.Kind == OriginNew {
		return Comparison{}, errors.New("a new document has nothing to compare with")
	}
	p, err := s.Providers.Provider(ctx, d.Origin)
	if err != nil {
		return Comparison{}, err
	}
	snap, ok := p.(resource.Snapshotter)
	if !ok {
		return Comparison{}, resource.ErrUnsupported
	}
	remote, err := snap.Snapshot(ctx, d.Origin.Path)
	if err != nil {
		return Comparison{}, err
	}
	merged, conflicts := textdiff.Merge3(d.Baseline.Text, d.Text, remote.Text)
	if conflicts == nil {
		conflicts = []textdiff.Conflict{}
	}
	// The title follows the resource unless the draft changed it.
	title := d.Title
	if d.Title == d.Baseline.Title {
		title = remote.Title
	}
	return Comparison{
		Moved: remote.Revision != d.Baseline.Revision, Revision: remote.Revision,
		Theirs: textdiff.UnifiedDiff(d.Baseline.Text, remote.Text, "as opened", "as it is now"),
		Mine:   textdiff.UnifiedDiff(d.Baseline.Text, d.Text, "as opened", "the draft"),
		Merged: merged, Draft: d.Text, Current: remote.Text, Title: title, Conflicts: conflicts, remote: remote,
	}, nil
}

// Merge brings the changes made to the draft's resource since the draft was
// opened into the draft, provided the draft is still at expected and the two
// changed no part of the document differently; the resource as it is now
// becomes the draft's baseline, so a later save or publication writes only
// the draft's own changes over the resource's. It is refused with
// ErrMergeConflict, changing nothing, when there are conflicts.
func (s *Service) Merge(ctx context.Context, id, expected string) (Draft, error) {
	defer s.writes.Lock(id)()
	c, err := s.compare(ctx, id)
	if err != nil {
		return Draft{}, err
	}
	if len(c.Conflicts) > 0 {
		return Draft{}, ErrMergeConflict
	}
	if err := checkText(c.Merged); err != nil {
		return Draft{}, err
	}
	return s.Store.change(id, expected, func(d *Draft) error {
		d.Text = c.Merged
		d.Title = c.Title
		d.Baseline = Baseline{Text: c.remote.Text, Revision: c.remote.Revision, Storage: c.remote.Native, Title: c.remote.Title}
		return nil
	})
}

// SaveAs writes a draft to a new resource, which becomes the draft's origin:
// a new document's first save, or a copy of an opened one kept somewhere
// else. It is refused when a resource already exists at origin, so nothing is
// replaced that the person has not seen.
func (s *Service) SaveAs(ctx context.Context, id, expected string, origin Origin) (Draft, []string, error) {
	done, err := s.admit()
	if err != nil {
		return Draft{}, nil, err
	}
	defer done()
	defer s.writes.Lock(id)()
	d, err := s.Store.Get(id)
	if err != nil {
		return Draft{}, nil, err
	}
	if d.Revision != expected {
		return Draft{}, nil, ErrConflict
	}
	// The path is the provider's to check: an empty one may be the root of a
	// source that keeps pages, and is no file at all for a local folder.
	if origin.Kind == "" || origin.Kind == OriginNew {
		return Draft{}, nil, errors.New("choose where to save the document")
	}
	p, err := s.Providers.Provider(ctx, origin)
	if err != nil {
		return Draft{}, nil, err
	}
	c, ok := p.(resource.DocumentCreator)
	if !ok {
		return Draft{}, nil, resource.ErrUnsupported
	}
	// The files added to the draft, and those of the original the text uses;
	// an added file of the same name is the one the text shows.
	kept, err := s.keptFiles(d)
	if err != nil {
		return Draft{}, nil, err
	}
	copied, notes := s.filesOf(ctx, d, fileNames(kept))
	files := append(kept, copied...)
	written, err := c.CreateDocument(ctx, origin.Path, resource.Edit{Text: d.Text, Title: d.Title, Files: files})
	if err != nil {
		return Draft{}, nil, err
	}
	s.Store.removeFiles(id, fileNames(kept))
	if written.Path != "" {
		origin.Path = written.Path
	}
	shot := written.Snapshot
	saved, err := s.Store.Rebase(id, expected, &origin, baselineOf(shot))
	return saved, append(notes, written.Notes...), err
}

// filesOf reads the files of the document a draft was opened from that a
// copy saved elsewhere needs: the pictures its text shows and the files it
// links to, each by the path the text gives, wherever under the source that
// is (internal/docfiles), and the other files kept beside it that the text
// refers to by name, such as a page's attachments. A file that cannot be
// read is left out, and a note says so; so are the names in except.
func (s *Service) filesOf(ctx context.Context, d Draft, except []string) ([]resource.File, []string) {
	if d.Origin.Kind == "" || d.Origin.Kind == OriginNew {
		return nil, nil
	}
	p, err := s.Providers.Provider(ctx, d.Origin)
	if err != nil {
		return nil, nil
	}
	src, ok := p.(resource.FileSource)
	if !ok {
		return nil, nil
	}
	var files []resource.File
	var notes []string
	// A file is copied under the path the text gives, so a file of the same
	// name found by the listing below is the same file and is skipped.
	taken := map[string]bool{}
	var nested []string
	var byPath func(names []string, what, how string)
	byPath = func(names []string, what, how string) {
		for _, name := range names {
			if taken[name] {
				continue
			}
			taken[name], taken[pathpkg.Base(name)] = true, true
			if slices.Contains(except, name) {
				continue
			}
			data, err := src.ReadFile(ctx, d.Origin.Path, name)
			if errors.Is(err, resource.ErrNotFound) {
				notes = append(notes, "The "+what+" "+name+how+" was not found beside the original, so the copy is without it.")
				continue
			}
			if err != nil {
				notes = append(notes, "The "+what+" "+name+" could not be copied from the original: "+err.Error())
				continue
			}
			files = append(files, resource.File{Name: name, Data: data})
			// A canvas diagram shows pictures of its own, which are read
			// from beside it and go with it.
			if strings.HasSuffix(strings.ToLower(name), ".canvas") {
				nested = append(nested, docfiles.CanvasFiles(name, data)...)
			}
		}
	}
	byPath(docfiles.Shown(d.Text), "picture", "")
	byPath(nested, "picture", ", which a diagram shows,")
	byPath(docfiles.Linked(d.Text), "file", ", which the document links to,")
	if shared := docfiles.Definitions(d.Text); shared != "" {
		byPath([]string{shared}, "file of shortcode definitions", "")
	}
	names, err := src.FileNames(ctx, d.Origin.Path)
	if err != nil {
		return files, append(notes, "The files beside the original could not be listed, so only its pictures were copied: "+err.Error())
	}
	for _, name := range names {
		if taken[name] || !uses(d.Text, name) || slices.Contains(except, name) {
			continue
		}
		data, err := src.ReadFile(ctx, d.Origin.Path, name)
		if err != nil {
			notes = append(notes, "The file "+name+" could not be copied from the original: "+err.Error())
			continue
		}
		files = append(files, resource.File{Name: name, Data: data})
	}
	return files, notes
}
