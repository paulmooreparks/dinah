package documents

import (
	"errors"
	"path"
	"strings"

	"studio/internal/drafts"
	"studio/internal/resource"
	"studio/internal/workspaces"
)

// Renamed is a file after Rename: where it is now, and the drafts of it,
// which went with it.
type Renamed struct {
	Ref    workspaces.Ref `json:"ref"`
	Name   string         `json:"name"`
	Drafts []drafts.View  `json:"drafts"`
}

// Rename gives the file at ref a new name in its own folder. Every draft of
// it goes with it, keeping its changes; a draft whose title was the file's
// old name takes the new one. A name another file of the folder has is
// refused, so nothing is replaced.
func (o Ops) Rename(ref workspaces.Ref, name string) (Renamed, error) {
	if !o.Allows(ref.WorkspaceID) {
		return Renamed{}, errors.New("the file is outside the calling workspace")
	}
	name = strings.TrimSpace(name)
	switch {
	case name == "" || name == "." || name == "..":
		return Renamed{}, errors.New("a file needs a name")
	case strings.ContainsAny(name, `/\:*?"<>|`):
		return Renamed{}, errors.New(`a file name cannot contain / \ : * ? " < > or |`)
	case len(name) > 255:
		return Renamed{}, errors.New("a file name is limited to 255 characters")
	}
	provider, source, err := o.Resolver.Provider(o.Ctx, ref)
	if err != nil {
		return Renamed{}, err
	}
	r, ok := provider.(resource.Renamer)
	if !ok {
		return Renamed{}, errors.New("documents in this source cannot be renamed here")
	}
	to := ref
	to.Path = path.Join(path.Dir(ref.Path), name)
	if err := r.Rename(o.Ctx, ref.Path, to.Path); err != nil {
		if errors.Is(err, resource.ErrConflict) {
			return Renamed{}, errors.New("a file called " + name + " is already in the folder")
		}
		return Renamed{}, ErrorText(err)
	}
	out := Renamed{Ref: to, Name: name, Drafts: []drafts.View{}}
	if to.Path == ref.Path {
		return out, nil
	}
	from := drafts.Origin{Kind: source.Kind, WorkspaceID: ref.WorkspaceID, SourceID: ref.SourceID, Path: ref.Path}
	moved := from
	moved.Path = to.Path
	changed, err := o.Drafts.Store.Rehome(from, moved, name)
	for _, d := range changed {
		out.Drafts = append(out.Drafts, o.View(d, "renamed"))
	}
	if err != nil {
		return out, errors.New("the file is renamed, but a draft of it could not be moved with it: " + err.Error())
	}
	return out, nil
}
