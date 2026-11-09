package workspaces

import (
	"context"
	"errors"

	"studio/internal/layout"
	"studio/internal/resource"
	"studio/internal/resource/localfs"
)

// HomeSourceID is the source of the person's own home folder, which belongs
// to no workspace.
const HomeSourceID = "dinah-home"

// Dotfiles reports whether names starting with a dot are ordinary names in a
// source: in the person's home folder, where a .profile is theirs, and in no
// workspace's folder, where .devin and .git stay out of reach.
func Dotfiles(s Source) bool { return s.ID == HomeSourceID && s.Kind == "local-folder" }

// Resolver finds the sources references point into and their resource
// providers, from a program's view of the registry.
type Resolver struct {
	Layout layout.Layout
	// Records returns the registry as it is now.
	Records func() []Record
	// Personal, when set, returns a source that belongs to no workspace, for
	// a reference with no workspace: the person's home folder, HomeSourceID,
	// and any other the program has. It fails for an ID it does not have.
	Personal func(id string) (Source, error)
}

// Source is the source a reference points into, including the package
// collections of its workspace.
func (r Resolver) Source(ref Ref) (Source, error) {
	if ref.WorkspaceID == "" && r.Personal != nil {
		return r.Personal(ref.SourceID)
	}
	records := r.Records()
	i := Index(records, ref.WorkspaceID)
	if i < 0 {
		return Source{}, errors.New("the workspace is no longer registered")
	}
	for _, source := range WithCollections(r.Layout, records[i]).Sources {
		if source.ID == ref.SourceID {
			return source, nil
		}
	}
	return Source{}, errors.New("the source is no longer attached to this workspace")
}

// ProviderOf builds the resource provider of a source from its kind: a
// local folder's, or the one the kind was registered with
// (RegisterSourceKind). Callers then ask the provider for the capability
// they need (resource.TextReader, resource.TextWriter and the like) and
// report resource.ErrUnsupported when it is absent, instead of branching on
// the kind themselves. A kind with no provider is resource.ErrUnsupported.
func (r Resolver) ProviderOf(ctx context.Context, s Source) (any, error) {
	if s.Kind == LocalFolder {
		return localfs.Folder{Root: s.Path, Dotfiles: Dotfiles(s)}, nil
	}
	k, ok := LookupSourceKind(s.Kind)
	if !ok || k.Provider == nil {
		return nil, resource.ErrUnsupported
	}
	return k.Provider(r, ctx, s)
}

// Provider returns the provider of the source a reference points into.
func (r Resolver) Provider(ctx context.Context, ref Ref) (any, Source, error) {
	source, err := r.Source(ref)
	if err != nil {
		return nil, Source{}, err
	}
	p, err := r.ProviderOf(ctx, source)
	return p, source, err
}
