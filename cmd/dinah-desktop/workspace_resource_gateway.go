package main

import (
	_ "embed"
	"errors"
	"os"
	"path/filepath"

	"studio/internal/resource/localfs"
	"studio/internal/workspaces"
)

//go:embed workspace_resource_gateway.go
var workspaceResourceGatewaySource []byte

func (a *App) documentResourceRef(workspace, rel string) (ResourceRef, error) {
	if _, _, err := documentPath(workspace, rel); err != nil {
		return ResourceRef{}, err
	}
	a.mu.Lock()
	index := workspaces.IndexAt(a.settings.WorkspaceRecords, workspace)
	if index < 0 {
		a.mu.Unlock()
		return ResourceRef{}, errors.New("the document workspace is no longer registered")
	}
	record := workspaces.Copy(a.settings.WorkspaceRecords[index : index+1])[0]
	a.mu.Unlock()
	source, ok := workspaces.PrimarySource(record)
	if !ok {
		return ResourceRef{}, errors.New("the document workspace has no primary local source")
	}
	return ResourceRef{WorkspaceID: record.ID, SourceID: source.ID, Path: rel}, nil
}

func (v workspaceVFS) localResourceFile(ref ResourceRef) (*os.File, error) {
	source, err := v.resourceSource(ref)
	if err != nil {
		return nil, err
	}
	if source.Kind != "local-folder" {
		return nil, errors.New("this operation needs a mounted local file")
	}
	return openWorkspaceResource(source, ref.Path)
}

// localPath is where a workspace file or folder is on this computer, and
// whether it is a folder. It is opened first, as every local read is, so a
// path outside its source, through a link or hidden is refused. A source that
// is not a local folder, such as a wiki's page tree, has no such path.
func (v workspaceVFS) localPath(ref ResourceRef) (string, bool, error) {
	source, err := v.resourceSource(ref)
	if err != nil {
		return "", false, err
	}
	if source.Kind != "local-folder" {
		return "", false, errors.New("this operation needs a mounted local file")
	}
	f, err := openWorkspaceResource(source, ref.Path)
	if err != nil {
		return "", false, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", false, err
	}
	// The open file's name is relative to the source's root, so the path is
	// made from the source's own folder.
	name, err := localfs.CleanPathFor(ref.Path, workspaces.Dotfiles(source))
	if err != nil {
		return "", false, err
	}
	return filepath.Join(source.Path, name), info.IsDir(), nil
}

func (v workspaceVFS) sourceRoots(workspaceID string) (ResourcePage, error) {
	page := ResourcePage{Entries: []ResourceItem{}, NextStart: -1}
	for _, record := range v.records() {
		if record.ID == workspaceID {
			for _, source := range record.Sources {
				page.Entries = append(page.Entries, workspaceSourceResource(record, source))
			}
			return page, nil
		}
	}
	return page, errors.New("the workspace is unavailable in this context")
}

// Resource IDs and mount paths enter the same VFS authority. Stable IDs remain
// valid when labels change, while workspace scope still applies to every read.
func (v workspaceVFS) resourceSource(ref ResourceRef) (WorkspaceSource, error) {
	if v.service.ctx != nil && v.service.ctx.Err() != nil {
		return WorkspaceSource{}, v.service.ctx.Err()
	}
	if v.allowedWorkspace != "" && ref.WorkspaceID != v.allowedWorkspace {
		return WorkspaceSource{}, errors.New("the resource is outside the calling workspace")
	}
	if v.expected != nil && (ref.WorkspaceID != v.expected.WorkspaceID || ref.SourceID != v.expected.SourceID) {
		return WorkspaceSource{}, errors.New("the resource mount identity has changed; select it again")
	}
	return v.service.source(ref)
}

func (v workspaceVFS) listResource(ref ResourceRef, start, limit int) (ResourcePage, error) {
	if _, err := v.resourceSource(ref); err != nil {
		return ResourcePage{}, err
	}
	return v.service.list(ref, start, limit)
}

func (v workspaceVFS) readResource(ref ResourceRef) (ResourceRead, error) {
	if _, err := v.resourceSource(ref); err != nil {
		return ResourceRead{}, err
	}
	return v.service.read(ref)
}

func (v workspaceVFS) previewResource(ref ResourceRef) (ResourcePreview, error) {
	if _, err := v.resourceSource(ref); err != nil {
		return ResourcePreview{}, err
	}
	return v.service.preview(ref)
}

func (v workspaceVFS) resourceAttachment(ref ResourceRef, name string) ([]byte, error) {
	if _, err := v.resourceSource(ref); err != nil {
		return nil, err
	}
	return v.service.attachment(ref, name)
}
