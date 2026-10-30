package main

import (
	"context"
	"errors"
	"os"

	"studio/internal/resource"
	"studio/internal/resource/localfs"
	"studio/internal/workspaces"
)

// Writes through the virtual file system, for the person's own Terminal.
// They are for local folders, which are the workspaces' folders and Home;
// any other kind of source stays read-only here, because a document there is
// changed through the Editor, where the person sees what changes. Agents and
// flows do not have these bindings.
//
// Each takes an expected identity for each path, as the read bindings do,
// so a terminal whose folder was renamed or detached cannot write into
// whatever now has its old name. The zero identity checks nothing.

// The ways WriteWorkspaceMount may write.
const (
	WriteCreate  = "create"  // a new file; an existing one is a conflict
	WriteReplace = "replace" // whatever is there now, as the shell's > does
	WriteExpect  = "expect"  // only while the file is at the given revision
)

func expectedOrNil(ref ResourceRef) *ResourceRef {
	if ref.WorkspaceID == "" && ref.SourceID == "" {
		return nil
	}
	return &ref
}

// localMount resolves path to a local folder and the path within it.
func (a *App) localMount(path string, expected ResourceRef) (mountTarget, localfs.Folder, error) {
	v := workspaceVFS{service: a.resourceService(), expected: expectedOrNil(expected)}
	target, err := v.resolve(path)
	if err != nil {
		return target, localfs.Folder{}, err
	}
	if target.ref == nil {
		return target, localfs.Folder{}, errors.New(path + " is a catalog of workspaces and sources, which cannot be changed here")
	}
	switch target.source.Kind {
	case "local-folder":
		return target, localfs.Folder{Root: target.source.Path, Dotfiles: workspaces.Dotfiles(target.source)}, nil
	}
	return target, localfs.Folder{}, errors.New("this source cannot be changed")
}

func (a *App) contentChanged(targets ...mountTarget) {
	seen := map[string]bool{}
	for _, t := range targets {
		key := t.record.ID + "\x00" + t.source.ID
		if seen[key] {
			continue
		}
		seen[key] = true
		a.emitWorkspaceMountChanged("content-changed", t.record.ID, t.source.ID)
	}
}

func vfsError(path string, err error) error {
	switch {
	case errors.Is(err, os.ErrNotExist), errors.Is(err, resource.ErrNotFound):
		return errors.New(path + ": no such file or folder")
	case errors.Is(err, localfs.ErrExists):
		return errors.New(path + ": already exists")
	case errors.Is(err, resource.ErrConflict):
		return errors.New(path + ": the file changed since it was read")
	}
	return err
}

// WriteWorkspaceMount writes text to a file in a local folder, in one of the
// modes above, and returns the file as it now is.
func (a *App) WriteWorkspaceMount(path, text, mode, revision string, expected ResourceRef) (MountItem, error) {
	target, folder, err := a.localMount(path, expected)
	if err != nil {
		return MountItem{}, err
	}
	ctx := context.Background()
	switch mode {
	case WriteCreate:
		revision = ""
	case WriteExpect:
		if revision == "" {
			return MountItem{}, errors.New("a write that expects a revision needs one")
		}
	case WriteReplace:
		current, err := folder.ReadText(ctx, target.ref.Path)
		switch {
		case errors.Is(err, resource.ErrNotFound):
			revision = ""
		case err != nil:
			return MountItem{}, vfsError(path, err)
		default:
			revision = current.Revision
		}
	default:
		return MountItem{}, errors.New("the write mode must be create, replace or expect")
	}
	if _, err := folder.WriteText(ctx, target.ref.Path, revision, text); err != nil {
		return MountItem{}, vfsError(path, err)
	}
	a.contentChanged(target)
	return workspaceVFS{service: a.resourceService()}.stat(target.path)
}

// MakeWorkspaceDirectory makes a folder in a local folder; with parents, as
// mkdir -p does.
func (a *App) MakeWorkspaceDirectory(path string, parents bool, expected ResourceRef) (MountItem, error) {
	target, folder, err := a.localMount(path, expected)
	if err != nil {
		return MountItem{}, err
	}
	if err := folder.MakeFolder(context.Background(), target.ref.Path, parents); err != nil {
		return MountItem{}, vfsError(path, err)
	}
	a.contentChanged(target)
	return workspaceVFS{service: a.resourceService()}.stat(target.path)
}

// RemoveWorkspaceMount removes a file or folder in a local folder and
// returns how many entries it removed.
func (a *App) RemoveWorkspaceMount(path string, recursive bool, expected ResourceRef) (int, error) {
	target, folder, err := a.localMount(path, expected)
	if err != nil {
		return 0, err
	}
	n, err := folder.Remove(context.Background(), target.ref.Path, recursive)
	if err != nil {
		return 0, vfsError(path, err)
	}
	a.contentChanged(target)
	return n, nil
}

// CopyWorkspaceMount copies a file, or with recursive a folder, between
// local folders and returns how many entries it copied.
func (a *App) CopyWorkspaceMount(from string, fromExpected ResourceRef, to string, toExpected ResourceRef, recursive bool) (int, error) {
	src, sf, err := a.localMount(from, fromExpected)
	if err != nil {
		return 0, err
	}
	dst, df, err := a.localMount(to, toExpected)
	if err != nil {
		return 0, err
	}
	n, err := localfs.Copy(context.Background(), sf, src.ref.Path, df, dst.ref.Path, recursive)
	if err != nil {
		if errors.Is(err, localfs.ErrExists) {
			return 0, vfsError(to, err)
		}
		return 0, vfsError(from, err)
	}
	a.contentChanged(dst)
	return n, nil
}

// MoveWorkspaceMount moves or renames a file or folder between local
// folders. Within one folder it is a rename; between two it is a copy and
// then a removal of the original, which is not atomic: if the removal fails,
// the copy remains and the error says so.
func (a *App) MoveWorkspaceMount(from string, fromExpected ResourceRef, to string, toExpected ResourceRef) (MountItem, error) {
	src, sf, err := a.localMount(from, fromExpected)
	if err != nil {
		return MountItem{}, err
	}
	dst, df, err := a.localMount(to, toExpected)
	if err != nil {
		return MountItem{}, err
	}
	ctx := context.Background()
	if src.record.ID == dst.record.ID && src.source.ID == dst.source.ID {
		if err := sf.Move(ctx, src.ref.Path, dst.ref.Path); err != nil {
			if errors.Is(err, localfs.ErrExists) {
				return MountItem{}, vfsError(to, err)
			}
			return MountItem{}, vfsError(from, err)
		}
	} else {
		if _, err := localfs.Copy(ctx, sf, src.ref.Path, df, dst.ref.Path, true); err != nil {
			if errors.Is(err, localfs.ErrExists) {
				return MountItem{}, vfsError(to, err)
			}
			return MountItem{}, vfsError(from, err)
		}
		if _, err := sf.Remove(ctx, src.ref.Path, true); err != nil {
			a.contentChanged(src, dst)
			return MountItem{}, errors.New("copied to " + to + ", but " + from + " could not be removed: " + vfsError(from, err).Error())
		}
	}
	a.contentChanged(src, dst)
	return workspaceVFS{service: a.resourceService()}.stat(dst.path)
}
