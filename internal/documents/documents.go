// Package documents holds the document operations on drafts: opening a
// resource as a draft, changing it, deciding its suggestions, saving it,
// comparing it with its resource and merging. The Editor's bindings call
// these operations, so every change is made the same way and raises the
// same events.
package documents

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"studio/internal/drafts"
	"studio/internal/layout"
	"studio/internal/profile"
	"studio/internal/resource"
	"studio/internal/workspaces"
)

// Change is told as "draft:changed" after a draft is created, changed,
// saved or discarded, so every view of it stays current.
type Change struct {
	ID       string `json:"id"`
	Revision string `json:"revision"`
	Kind     string `json:"kind"`
}

// Saved is a saved draft with what the write could not keep.
type Saved struct {
	drafts.View
	Notes []string `json:"notes"`
}

// Providers resolve a draft's origin to its source's provider.
type Providers struct{ Resolver workspaces.Resolver }

func (p Providers) Provider(ctx context.Context, origin drafts.Origin) (any, error) {
	provider, source, err := p.Resolver.Provider(ctx, workspaces.Ref{WorkspaceID: origin.WorkspaceID, SourceID: origin.SourceID, Path: origin.Path})
	if err != nil {
		return nil, err
	}
	// A source replaced by another of a different kind under the same ID is
	// not the resource the draft was made from.
	if source.Kind != origin.Kind {
		return nil, errors.New("the draft's source has changed since the draft was made")
	}
	return provider, nil
}

// Service is the draft service for a state folder: the drafts are kept in
// its drafts folder, and their resources are reached through the resolver.
func Service(l layout.Layout, r workspaces.Resolver) (*drafts.Service, error) {
	store, err := drafts.Open(filepath.Join(l.State, "drafts"))
	if err != nil {
		return nil, err
	}
	return &drafts.Service{Store: store, Providers: Providers{Resolver: r}}, nil
}

// ErrorText turns the drafts' and resources' errors into what a person reads.
func ErrorText(err error) error {
	switch {
	case errors.Is(err, drafts.ErrConflict):
		return errors.New("the draft was changed in another window since this one read it; reload it before changing it again")
	case errors.Is(err, drafts.ErrNotFound):
		return errors.New("the draft no longer exists")
	case errors.Is(err, resource.ErrConflict):
		return errors.New("the document was changed elsewhere since it was opened; compare or reload before saving")
	case errors.Is(err, resource.ErrUnsupported):
		return errors.New("this source cannot be written from " + profile.Current().Name)
	case errors.Is(err, resource.ErrNotFound):
		return errors.New("the document no longer exists")
	}
	return err
}

// Ops are the document operations for one caller. A non-empty Workspace
// confines them to that workspace's drafts and resources, as an action's
// calling workspace does; the Editor acts for the person and is not
// confined. Emit, when set, is told of every change.
type Ops struct {
	Ctx       context.Context
	Workspace string
	Drafts    *drafts.Service
	Resolver  workspaces.Resolver
	Emit      func(event string, data any)
}

func (o Ops) emit(event string, data any) {
	if o.Emit != nil {
		o.Emit(event, data)
	}
}

// Allows reports whether the operations may touch a workspace's documents.
func (o Ops) Allows(workspaceID string) bool {
	return o.Workspace == "" || o.Workspace == workspaceID
}

// View is a draft as views show it, telling of a change of the kind given.
func (o Ops) View(d drafts.Draft, kind string) drafts.View {
	if kind != "" {
		o.emit("draft:changed", Change{ID: d.ID, Revision: d.Revision, Kind: kind})
	}
	return o.Drafts.View(o.Ctx, d)
}

// Draft reads a draft the operations may touch. A draft of another workspace
// is reported as missing, so its existence is not disclosed.
func (o Ops) Draft(id string) (drafts.Draft, error) {
	d, err := o.Drafts.Store.Get(id)
	if err == nil && !o.Allows(d.Origin.WorkspaceID) {
		err = drafts.ErrNotFound
	}
	if err != nil {
		return drafts.Draft{}, ErrorText(err)
	}
	return d, nil
}

// Open opens a resource for editing: its draft in progress, or a new draft
// from the resource as it is now.
func (o Ops) Open(ref workspaces.Ref) (drafts.View, error) {
	if !o.Allows(ref.WorkspaceID) {
		return drafts.View{}, errors.New("the resource is outside the calling workspace")
	}
	source, err := o.Resolver.Source(ref)
	if err != nil {
		return drafts.View{}, err
	}
	origin := drafts.Origin{Kind: source.Kind, WorkspaceID: ref.WorkspaceID, SourceID: ref.SourceID, Path: ref.Path}
	before, _ := o.Drafts.Store.FindByOrigin(origin)
	d, err := o.Drafts.Open(o.Ctx, origin)
	if err != nil {
		return drafts.View{}, ErrorText(err)
	}
	kind := ""
	if len(before) == 0 {
		kind = "created"
	}
	return o.View(d, kind), nil
}

// Get reads a draft as views show it.
func (o Ops) Get(id string) (drafts.View, error) {
	d, err := o.Draft(id)
	if err != nil {
		return drafts.View{}, err
	}
	return o.View(d, ""), nil
}

// Update replaces a draft's text, and its title when one is given.
func (o Ops) Update(id, expected, text string, title *string) (drafts.View, error) {
	if _, err := o.Draft(id); err != nil {
		return drafts.View{}, err
	}
	if title != nil {
		t := strings.TrimSpace(*title)
		if t == "" || len(t) > 255 {
			return drafts.View{}, errors.New("a title needs between 1 and 255 characters")
		}
		title = &t
	}
	d, err := o.Drafts.Store.Update(id, expected, text, title)
	if err != nil {
		return drafts.View{}, ErrorText(err)
	}
	return o.View(d, "updated"), nil
}

// Save writes a draft back to its resource; see drafts.Service.Save.
func (o Ops) Save(id, expected string) (Saved, error) {
	if _, err := o.Draft(id); err != nil {
		return Saved{}, err
	}
	d, notes, err := o.Drafts.Save(o.Ctx, id, expected)
	if err != nil {
		return Saved{}, ErrorText(err)
	}
	if notes == nil {
		notes = []string{}
	}
	return Saved{View: o.View(d, "saved"), Notes: notes}, nil
}

// SaveAs writes a draft to a new resource, which becomes its origin.
func (o Ops) SaveAs(id, expected string, ref workspaces.Ref) (Saved, error) {
	if _, err := o.Draft(id); err != nil {
		return Saved{}, err
	}
	if !o.Allows(ref.WorkspaceID) {
		return Saved{}, errors.New("the place to save is outside the calling workspace")
	}
	source, err := o.Resolver.Source(ref)
	if err != nil {
		return Saved{}, err
	}
	origin := drafts.Origin{Kind: source.Kind, WorkspaceID: ref.WorkspaceID, SourceID: ref.SourceID, Path: ref.Path}
	d, notes, err := o.Drafts.SaveAs(o.Ctx, id, expected, origin)
	if errors.Is(err, resource.ErrConflict) {
		return Saved{}, errors.New("something already exists there; choose another name")
	}
	if errors.Is(err, resource.ErrUnsupported) {
		return Saved{}, errors.New("new documents cannot be created in this source")
	}
	if err != nil {
		return Saved{}, ErrorText(err)
	}
	if notes == nil {
		notes = []string{}
	}
	return Saved{View: o.View(d, "saved"), Notes: notes}, nil
}

// Decide accepts or rejects a suggestion, or every suggestion of its
// proposal when all is true. Accepting changes the text, so it needs the
// draft's revision.
func (o Ops) Decide(id, expected, suggestion string, accept, all bool) (drafts.View, error) {
	d, err := o.Draft(id)
	if err != nil {
		return drafts.View{}, err
	}
	group := ""
	for _, s := range d.Suggestions {
		if s.ID == suggestion {
			group = s.Group
		}
	}
	if group == "" {
		return drafts.View{}, errors.New("the suggestion is no longer there")
	}
	switch {
	case accept && all:
		d, err = o.Drafts.Store.AcceptGroup(id, expected, group)
	case accept:
		d, err = o.Drafts.Store.Accept(id, expected, suggestion)
	case all:
		d, err = o.Drafts.Store.RejectGroup(id, group)
	default:
		d, err = o.Drafts.Store.Reject(id, suggestion)
	}
	if errors.Is(err, drafts.ErrSuggestionStale) {
		return drafts.View{}, errors.New("the text this suggestion would change has been changed since it was made, so it can only be rejected")
	}
	if err != nil {
		return drafts.View{}, ErrorText(err)
	}
	return o.View(d, "updated"), nil
}

// Revise changes the text a suggestion proposes, as the person edited it.
func (o Ops) Revise(id, suggestion, after string) (drafts.View, error) {
	if _, err := o.Draft(id); err != nil {
		return drafts.View{}, err
	}
	d, err := o.Drafts.Store.Revise(id, suggestion, after)
	if errors.Is(err, drafts.ErrSuggestionStale) {
		return drafts.View{}, errors.New("the text this suggestion would change has been changed since it was made, so it can only be rejected")
	}
	if errors.Is(err, drafts.ErrSuggestionGone) {
		return drafts.View{}, errors.New("the suggestion is no longer there")
	}
	if err != nil {
		return drafts.View{}, ErrorText(err)
	}
	return o.View(d, "updated"), nil
}

// Compare compares a draft with its resource as it is now.
func (o Ops) Compare(id string) (drafts.Comparison, error) {
	if _, err := o.Draft(id); err != nil {
		return drafts.Comparison{}, err
	}
	c, err := o.Drafts.Compare(o.Ctx, id)
	if err != nil {
		return drafts.Comparison{}, ErrorText(err)
	}
	return c, nil
}

// Merge brings the resource's changes since the draft was opened into the
// draft; see drafts.Service.Merge.
func (o Ops) Merge(id, expected string) (drafts.View, error) {
	if _, err := o.Draft(id); err != nil {
		return drafts.View{}, err
	}
	d, err := o.Drafts.Merge(o.Ctx, id, expected)
	if errors.Is(err, drafts.ErrMergeConflict) {
		return drafts.View{}, errors.New("the document and the draft both changed the same part, so the changes cannot be combined; change the draft to settle the conflicting parts first")
	}
	if err != nil {
		return drafts.View{}, ErrorText(err)
	}
	return o.View(d, "updated"), nil
}

// Discard deletes a draft, provided it is still at expected. The resource is
// not changed.
func (o Ops) Discard(id, expected string) error {
	if _, err := o.Draft(id); err != nil {
		return err
	}
	if err := o.Drafts.Store.Discard(id, expected); err != nil {
		return ErrorText(err)
	}
	o.emit("draft:changed", Change{ID: id, Kind: "discarded"})
	return nil
}

// AddFile keeps a file with a draft, to be saved with it, and returns the
// name it is kept under; see drafts.Service.AddFile.
func (o Ops) AddFile(id, name string, data []byte) (string, error) {
	if _, err := o.Draft(id); err != nil {
		return "", err
	}
	kept, err := o.Drafts.AddFile(o.Ctx, id, name, data)
	if err != nil {
		return "", ErrorText(err)
	}
	return kept, nil
}

// File reads a file a draft's text shows: one kept with the draft, or else
// the one of that name beside the document it was opened from.
func (o Ops) File(id, name string) ([]byte, error) {
	d, err := o.Draft(id)
	if err != nil {
		return nil, err
	}
	if data, err := o.Drafts.Store.ReadFile(id, name); err == nil {
		return data, nil
	}
	if d.Origin.Kind == "" || d.Origin.Kind == drafts.OriginNew {
		return nil, drafts.ErrNotFound
	}
	p, err := o.Drafts.Providers.Provider(o.Ctx, d.Origin)
	if err != nil {
		return nil, err
	}
	src, ok := p.(resource.FileSource)
	if !ok {
		return nil, drafts.ErrNotFound
	}
	return src.ReadFile(o.Ctx, d.Origin.Path, name)
}

// List lists the drafts the operations may touch that hold something not yet
// saved, most recent first. Any other draft is only the working copy of a
// document as it is stored, and is left out.
func (o Ops) List() ([]drafts.View, error) {
	all, err := o.Drafts.Store.List()
	if err != nil {
		return nil, err
	}
	out := make([]drafts.View, 0, len(all))
	for _, d := range all {
		if d.InProgress() && o.Allows(d.Origin.WorkspaceID) {
			out = append(out, o.View(d, ""))
		}
	}
	return out, nil
}
