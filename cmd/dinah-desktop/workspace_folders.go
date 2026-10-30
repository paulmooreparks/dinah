package main

import (
	"errors"
	"path"
	"strings"

	"studio/internal/resource"
	"studio/internal/resource/localfs"
	"studio/internal/workspaces"
)

// CreateWorkspaceFolder makes a folder named name in a folder on the
// computer, a workspace's local folder or the home folder, for New Folder in
// Open, Save As and Files. It returns the new folder.
func (a *App) CreateWorkspaceFolder(parent ResourceRef, name string) (ResourceItem, error) {
	s := a.resourceService()
	source, err := s.source(parent)
	if err != nil {
		return ResourceItem{}, err
	}
	if source.Kind != "local-folder" {
		return ResourceItem{}, errors.New("a folder can be made only in a folder on the computer")
	}
	name = strings.TrimSpace(name)
	if p := localfs.NameProblem(name); p != "" {
		return ResourceItem{}, errors.New(p)
	}
	ref := parent
	ref.Path = path.Join(strings.Trim(parent.Path, "/"), name)
	if parent.Path == "" || parent.Path == "." {
		ref.Path = name
	}
	folder := localfs.Folder{Root: source.Path, Dotfiles: workspaces.Dotfiles(source)}
	if err := folder.CreateFolder(ref.Path); err != nil {
		if errors.Is(err, resource.ErrConflict) {
			return ResourceItem{}, errors.New(name + " already exists here; choose another name")
		}
		return ResourceItem{}, err
	}
	f, err := openWorkspaceResource(source, ref.Path)
	if err != nil {
		return ResourceItem{}, err
	}
	defer f.Close()
	item, err := localResourceItem(ref, source, f)
	if err != nil {
		return ResourceItem{}, err
	}
	return item, nil
}
