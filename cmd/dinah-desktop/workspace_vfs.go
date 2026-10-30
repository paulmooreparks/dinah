package main

import (
	"context"
	_ "embed"
	"errors"
	"path/filepath"
	"sort"
	"strings"

	"studio/internal/workspaces"
)

//go:embed workspace_vfs.go
var workspaceVFSSource []byte

// MountItem is a virtual path presentation of a stable provider resource.
// Workspace catalogs retain their ID in Ref but cannot be read as files.
type MountItem struct {
	Path      string       `json:"path"`
	Title     string       `json:"title"`
	Kind      string       `json:"kind"`
	Directory bool         `json:"directory"`
	Revision  string       `json:"revision"`
	URL       string       `json:"url"`
	Actions   []string     `json:"actions"`
	Ref       *ResourceRef `json:"ref,omitempty"`
	// Size, Modified, Writable, Missing and Unavailable are as ResourceItem
	// has them.
	Size        int64  `json:"size,omitempty"`
	Modified    string `json:"modified,omitempty"`
	Writable    bool   `json:"writable,omitempty"`
	Missing     bool   `json:"missing,omitempty"`
	Unavailable bool   `json:"unavailable,omitempty"`
}
type MountPage struct {
	Entries   []MountItem `json:"entries"`
	NextStart int         `json:"nextStart"`
}
type MountRead struct {
	Resource MountItem `json:"resource"`
	Text     string    `json:"text"`
	Format   string    `json:"format"`
	Revision string    `json:"revision"`
}

type workspaceVFS struct {
	service          workspaceResourceService
	allowedWorkspace string
	expected         *ResourceRef
}
type mountTarget struct {
	path     string
	segments []string
	record   WorkspaceRecord
	source   WorkspaceSource
	ref      *ResourceRef
	home     string
}

func (a *App) ListWorkspaceMounts(path string, start, limit int) (MountPage, error) {
	return workspaceVFS{service: a.resourceService()}.list(path, start, limit)
}
func (a *App) StatWorkspaceMount(path string) (MountItem, error) {
	return workspaceVFS{service: a.resourceService()}.stat(path)
}
func (a *App) ReadWorkspaceMount(path string) (MountRead, error) {
	return workspaceVFS{service: a.resourceService()}.read(path)
}

// ResolveWorkspaceResourceMount follows stable IDs to the current public path.
// A retained address is never resolved by reusing another resource's old label.
func (a *App) ResolveWorkspaceResourceMount(ref ResourceRef) (MountItem, error) {
	return (workspaceVFS{service: a.resourceService()}).statRef(ref)
}

func (v workspaceVFS) statRef(ref ResourceRef) (MountItem, error) {
	if ref.WorkspaceID == "" && ref.SourceID == workspaces.HomeSourceID {
		path := v.service.app.homeMountPath()
		if ref.Path != "" {
			path += "/" + ref.Path
		}
		v.expected = &ref
		return v.stat(path)
	}
	if root, ok := systemRootOf(ref.SourceID); ok && ref.WorkspaceID == "" {
		v.expected = &ref
		return v.stat(systemMountPath(root, ref.Path))
	}
	for _, record := range v.records() {
		if record.ID != ref.WorkspaceID {
			continue
		}
		path := "/workspaces/" + record.MountAlias
		if ref.SourceID == "" {
			if ref.Path != "" {
				return MountItem{}, errors.New("a workspace catalog reference has no relative path")
			}
			v.expected = &ref
			return v.stat(path)
		}
		for _, source := range record.Sources {
			if source.ID != ref.SourceID {
				continue
			}
			path += "/" + source.MountAlias
			if ref.Path != "" {
				path += "/" + ref.Path
			}
			v.expected = &ref
			return v.stat(path)
		}
		return MountItem{}, errors.New("the source is no longer attached to this workspace")
	}
	return MountItem{}, errors.New("the workspace is no longer registered")
}

// Expected operations bind a retained virtual location to its original mount.
// Path is deliberately excluded from the comparison so navigation within the
// same source can use the saved identity. Resource operations then use the
// resolved IDs, which cannot be redirected by subsequent alias reuse.
func (a *App) ListWorkspaceMountsExpected(path string, start, limit int, expected ResourceRef) (MountPage, error) {
	return workspaceVFS{service: a.resourceService(), expected: &expected}.list(path, start, limit)
}
func (a *App) StatWorkspaceMountExpected(path string, expected ResourceRef) (MountItem, error) {
	return workspaceVFS{service: a.resourceService(), expected: &expected}.stat(path)
}
func (a *App) ReadWorkspaceMountExpected(path string, expected ResourceRef) (MountRead, error) {
	return workspaceVFS{service: a.resourceService(), expected: &expected}.read(path)
}
func (a *App) OpenWorkspaceMount(path string) error {
	target, err := (workspaceVFS{service: a.resourceService()}).resolve(path)
	if err != nil {
		return err
	}
	if target.ref == nil {
		return errors.New("choose a source resource to open")
	}
	return a.OpenWorkspaceResource(*target.ref)
}

func mountPathParts(path, home string) (string, []string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		path = home + strings.TrimPrefix(path, "~")
	}
	if path == "/" {
		return path, []string{}, nil
	}
	if len(path) > 8192 || !strings.HasPrefix(path, "/") || strings.ContainsAny(path, "\\\x00") {
		return "", nil, errors.New("use an absolute virtual workspace path")
	}
	path = strings.TrimSuffix(path, "/")
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", nil, errors.New("virtual paths cannot contain empty or relative segments")
		}
	}
	if parts[0] != "workspaces" && parts[0] != "home" && servedRoots[parts[0]] == "" {
		if ReservedRoot(parts[0]) {
			return "", nil, errors.New("/" + parts[0] + " is reserved for " + product.Name + " and holds nothing yet")
		}
		return "", nil, errors.New("the virtual filesystem exposes only /home, /workspaces, /bin and /lib")
	}
	return path, parts, nil
}
func (v workspaceVFS) records() []WorkspaceRecord {
	v.service.app.mu.Lock()
	defer v.service.app.mu.Unlock()
	records := workspaces.Copy(v.service.app.settings.WorkspaceRecords)
	// A workspace's package collections, such as its skills, are mounted
	// as sources of it.
	for i := range records {
		records[i] = workspaces.WithCollections(stateLayout(), records[i])
	}
	if v.allowedWorkspace == "" {
		return records
	}
	for _, record := range records {
		if record.ID == v.allowedWorkspace {
			return []WorkspaceRecord{record}
		}
	}
	return []WorkspaceRecord{}
}
func (v workspaceVFS) resolve(path string) (mountTarget, error) {
	if v.service.ctx != nil && v.service.ctx.Err() != nil {
		return mountTarget{}, v.service.ctx.Err()
	}
	home := v.service.app.homeMountPath()
	clean, parts, err := mountPathParts(path, home)
	if err != nil {
		return mountTarget{}, err
	}
	target := mountTarget{path: clean, segments: parts, home: home}
	if id, ok := servedRoots[parts0(parts)]; ok {
		if v.allowedWorkspace != "" {
			return target, errors.New("/" + parts[0] + " is unavailable to workspace-scoped actions")
		}
		if v.expected != nil && (v.expected.WorkspaceID != "" || v.expected.SourceID != id) {
			return target, errors.New("the saved mount identity does not match " + clean)
		}
		target.source, err = systemSource(parts[0])
		if err != nil {
			return target, err
		}
		target.ref = &ResourceRef{SourceID: id, Path: strings.Join(parts[1:], "/")}
		return target, nil
	}
	if len(parts) > 0 && parts[0] == "home" {
		if v.allowedWorkspace != "" {
			return target, errors.New("Home is unavailable to workspace-scoped actions")
		}
		if len(parts) == 1 {
			if v.expected != nil {
				return target, errors.New("choose the " + product.Name + " user home below /home")
			}
			return target, nil
		}
		if "/home/"+parts[1] != home {
			return target, errors.New("the home folder is " + home)
		}
		if v.expected != nil && (v.expected.WorkspaceID != "" || v.expected.SourceID != workspaces.HomeSourceID) {
			return target, errors.New("the saved mount identity does not match Home")
		}
		target.source, err = dinahHomeSource()
		if err != nil {
			return target, err
		}
		target.ref = &ResourceRef{SourceID: workspaces.HomeSourceID, Path: strings.Join(parts[2:], "/")}
		return target, nil
	}
	if len(parts) < 2 {
		if v.expected != nil {
			return target, errors.New("the saved workspace mount is unavailable at this path")
		}
		return target, nil
	}
	for _, record := range v.records() {
		if record.MountAlias == parts[1] {
			target.record = record
			break
		}
	}
	if target.record.ID == "" {
		return target, errors.New("the workspace mount is unavailable in this context")
	}
	if v.expected != nil && (v.expected.WorkspaceID == "" || target.record.ID != v.expected.WorkspaceID) {
		return target, errors.New("the workspace mount identity has changed; select it again")
	}
	if len(parts) < 3 {
		if v.expected != nil && v.expected.SourceID != "" {
			return target, errors.New("the saved source mount is unavailable at this path")
		}
		return target, nil
	}
	for _, source := range target.record.Sources {
		if source.MountAlias == parts[2] {
			target.source = source
			break
		}
	}
	if target.source.ID == "" {
		return target, errors.New("the source mount is no longer attached")
	}
	if v.expected != nil && v.expected.SourceID != "" && target.source.ID != v.expected.SourceID {
		return target, errors.New("the source mount identity has changed; select it again")
	}
	target.ref = &ResourceRef{WorkspaceID: target.record.ID, SourceID: target.source.ID, Path: strings.Join(parts[3:], "/")}
	return target, nil
}
func parts0(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}
func mountCatalogItem(path, title, kind string) MountItem {
	return MountItem{Path: path, Title: title, Kind: kind, Directory: true, Actions: []string{"list"}}
}
func mountWorkspaceItem(record WorkspaceRecord) MountItem {
	item := mountCatalogItem("/workspaces/"+record.MountAlias, record.Name, "workspace")
	item.Ref = &ResourceRef{WorkspaceID: record.ID}
	return item
}
func mountResourceItem(target mountTarget, item ResourceItem) MountItem {
	path := "/workspaces/" + target.record.MountAlias + "/" + target.source.MountAlias
	if target.source.ID == workspaces.HomeSourceID {
		path = target.home
	} else if root, ok := systemRootOf(target.source.ID); ok && target.record.ID == "" {
		path = "/" + root
	}
	if item.Ref.Path != "" {
		path += "/" + item.Ref.Path
	} else if target.source.Name != "" {
		item.Title = target.source.Name
	}
	ref := item.Ref
	return MountItem{Path: path, Title: item.Title, Kind: item.Kind, Directory: item.Directory, Revision: item.Revision, URL: item.URL, Actions: item.Actions, Ref: &ref, Size: item.Size, Modified: item.Modified, Writable: item.Writable, Missing: item.Missing, Unavailable: item.Unavailable}
}
func (v workspaceVFS) list(path string, start, limit int) (MountPage, error) {
	out := MountPage{Entries: []MountItem{}, NextStart: -1}
	if start < 0 || start > 1000000 || limit < 1 || limit > 100 {
		return out, errors.New("invalid virtual directory pagination")
	}
	target, err := v.resolve(path)
	if err != nil {
		return out, err
	}
	if target.ref != nil {
		// resolve has checked the retained workspace and any retained source.
		// Bind the resource operation to the source selected beneath that catalog.
		v.expected = target.ref
		page, err := v.listResource(*target.ref, start, limit)
		if err != nil {
			return out, err
		}
		out.NextStart = page.NextStart
		for _, item := range page.Entries {
			out.Entries = append(out.Entries, mountResourceItem(target, item))
		}
		return out, nil
	}
	items := []MountItem{}
	switch len(target.segments) {
	case 0:
		if v.allowedWorkspace == "" {
			home := mountCatalogItem("/home", "home", "catalog")
			items = append(items, home)
		}
		items = append(items, mountCatalogItem("/workspaces", "Workspaces", "catalog"))
		if v.allowedWorkspace == "" {
			for _, name := range reservedRoots {
				if id, ok := servedRoots[name]; ok {
					item := mountCatalogItem("/"+name, name, "directory")
					item.Ref, item.Writable = &ResourceRef{SourceID: id}, true
					items = append(items, item)
				}
			}
		}
	case 1:
		if target.segments[0] == "home" {
			homePath := v.service.app.homeMountPath()
			home := mountCatalogItem(homePath, strings.TrimPrefix(homePath, "/home/"), "directory")
			home.Ref = &ResourceRef{SourceID: workspaces.HomeSourceID}
			items = append(items, home)
			break
		}
		for _, record := range v.records() {
			items = append(items, mountWorkspaceItem(record))
		}
	case 2:
		for _, source := range target.record.Sources {
			child := target
			child.source = source
			items = append(items, mountResourceItem(child, workspaceSourceResource(target.record, source)))
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Path < items[j].Path })
	return pageOf(items, start, limit), nil
}

// pageOf is the page of items from start, at most limit long.
func pageOf(items []MountItem, start, limit int) MountPage {
	out := MountPage{Entries: []MountItem{}, NextStart: -1}
	if start >= len(items) {
		return out
	}
	end := start + limit
	if end < len(items) {
		out.NextStart = end
	} else {
		end = len(items)
	}
	out.Entries = items[start:end]
	return out
}

func dinahHomeSource() (WorkspaceSource, error) {
	path := filepath.Join(configDir(), "home")
	if err := stateDirectory(path); err != nil {
		return WorkspaceSource{}, err
	}
	return WorkspaceSource{ID: workspaces.HomeSourceID, Kind: "local-folder", Name: "Home", Path: path}, nil
}
func (v workspaceVFS) stat(path string) (MountItem, error) {
	target, err := v.resolve(path)
	if err != nil {
		return MountItem{}, err
	}
	if target.ref == nil {
		switch len(target.segments) {
		case 0:
			return mountCatalogItem("/", "Workspace Filesystem", "catalog"), nil
		case 1:
			if target.path == "/home" {
				return mountCatalogItem("/home", "home", "catalog"), nil
			}
			return mountCatalogItem(target.path, "Workspaces", "catalog"), nil
		default:
			return mountWorkspaceItem(target.record), nil
		}
	}
	// A source of a kind this build does not know, and a local folder that
	// is not on the computer, are described as the source's own row is.
	if target.ref.Path == "" && target.record.ID != "" && (target.source.Kind != "local-folder" || !folderExists(target.source.Path)) {
		return mountResourceItem(target, workspaceSourceResource(target.record, target.source)), nil
	}
	if target.source.Kind != "local-folder" {
		return MountItem{}, errors.New("unsupported workspace source")
	}
	f, err := openWorkspaceResource(target.source, target.ref.Path)
	if err != nil {
		return MountItem{}, err
	}
	defer f.Close()
	item, err := localResourceItem(*target.ref, target.source, f)
	if err != nil {
		return MountItem{}, err
	}
	return mountResourceItem(target, item), nil
}
func (v workspaceVFS) read(path string) (MountRead, error) {
	target, err := v.resolve(path)
	if err != nil {
		return MountRead{}, err
	}
	if target.ref == nil {
		return MountRead{}, errors.New("choose a source resource to read")
	}
	v.expected = target.ref
	read, err := v.readResource(*target.ref)
	if err != nil {
		return MountRead{}, err
	}
	return MountRead{Resource: mountResourceItem(target, read.Resource), Text: read.Text, Format: read.Format, Revision: read.Revision}, nil
}
func (a *App) scopedWorkspaceVFS(ctx context.Context) (workspaceVFS, error) {
	record, err := a.callingWorkspaceRecord(ctx)
	if err != nil {
		return workspaceVFS{}, err
	}
	service := a.resourceService()
	service.ctx = ctx
	return workspaceVFS{service: service, allowedWorkspace: record.ID}, nil
}
