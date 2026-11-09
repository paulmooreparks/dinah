package main

import (
	"errors"
	"html"
	"path"
	"sort"
	"strings"

	"studio/internal/preview"
	"studio/internal/resource"
	"studio/internal/workspaces"
)

/* A source of a registered kind other than a local folder (a wiki's page
   tree, an issue tracker's project) is listed, read and previewed through
   its resource provider (internal/resource), which the kind registered
   with package workspaces (docs/design/source-kinds.md). Nothing here knows
   a kind: an entry lists when the provider lists it as a folder, reads when
   the provider reads text, opens in the Editor when the provider takes a
   snapshot, and opens in the browser when the entry has an address. A local
   folder keeps its own path (workspace_resources.go), which confines opens
   to the folder and governs agents' reads. */

// providerOf is the resource provider of a source that is not a local folder.
func (s workspaceResourceService) providerOf(source WorkspaceSource) (any, error) {
	p, err := s.resolver().ProviderOf(s.ctx, source)
	if errors.Is(err, resource.ErrUnsupported) {
		return nil, errors.New("unsupported workspace source")
	}
	return p, err
}

// providerSourceItem is the row of a source that is not a local folder: a
// folder that lists, when the build has a provider for its kind, and
// otherwise a source kept for a build that does.
func providerSourceItem(record WorkspaceRecord, source WorkspaceSource) ResourceItem {
	item := ResourceItem{Ref: ResourceRef{WorkspaceID: record.ID, SourceID: source.ID}, Title: source.Name, Kind: source.Kind, Directory: true, Actions: []string{"list"}}
	if !workspaces.HasProvider(source.Kind) {
		item.Directory, item.Unavailable, item.Actions = false, true, []string{}
	}
	if k, ok := workspaces.LookupSourceKind(source.Kind); ok {
		item.Glyph = k.Glyph
		if k.URL != nil {
			item.URL = k.URL(source)
		}
		if item.URL != "" && !item.Unavailable {
			item.Actions = append(item.Actions, "open-external")
		}
	}
	return item
}

// creates says whether a provider makes new documents, so that its folders
// are offered as places to save one.
func creates(p any) bool {
	_, ok := p.(resource.DocumentCreator)
	return ok
}

// providerItem is a provider's entry as the Files applet shows it.
func providerItem(parent ResourceRef, source WorkspaceSource, e resource.Entry, makes bool) ResourceItem {
	ref := parent
	ref.Path = path.Join(parent.Path, e.Name)
	item := ResourceItem{Ref: ref, Title: e.Title, Kind: e.Kind, Directory: e.Folder, Revision: e.Revision, URL: e.URL, Glyph: e.Glyph, Actions: []string{}}
	if item.Title == "" {
		item.Title = e.Name
	}
	if item.Kind == "" {
		if e.Folder {
			item.Kind = "directory"
		} else {
			item.Kind = "file"
		}
	}
	if !e.Folder {
		item.Size = e.Size
	}
	item.Creates = makes && e.Folder && e.Document
	switch {
	case e.Folder && e.Document:
		item.Actions = append(item.Actions, "list", "read", "preview", "edit")
	case e.Folder:
		item.Actions = append(item.Actions, "list")
	default:
		if editorOpens(e.Name, e.Size) {
			item.Actions = append(item.Actions, "read", "preview")
		}
		if imagePreviewable(e.Name, e.Size) {
			item.Actions = append(item.Actions, "preview")
		}
	}
	if e.URL != "" {
		item.Actions = append(item.Actions, "open-external")
	}
	return item
}

// listProvider lists a folder of a source through its provider's Lister.
func (s workspaceResourceService) listProvider(ref ResourceRef, source WorkspaceSource, start, limit int) (ResourcePage, error) {
	out := ResourcePage{Entries: []ResourceItem{}, NextStart: -1}
	p, err := s.providerOf(source)
	if err != nil {
		return out, err
	}
	lister, ok := p.(resource.Lister)
	if !ok {
		return out, errors.New("this source does not list its contents")
	}
	folder := ref.Path
	if folder == "" {
		folder = "."
	}
	entries, err := lister.List(s.ctx, folder)
	if err != nil {
		return out, err
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return folderOrder(entries[i].Folder, providerSortName(entries[i]), entries[j].Folder, providerSortName(entries[j]))
	})
	if start >= len(entries) {
		return out, nil
	}
	entries = entries[start:]
	if len(entries) > limit {
		out.NextStart = start + limit
		entries = entries[:limit]
	}
	makes := creates(p)
	for _, e := range entries {
		out.Entries = append(out.Entries, providerItem(ref, source, e, makes))
	}
	return out, nil
}

func providerSortName(e resource.Entry) string {
	if e.Title != "" {
		return e.Title
	}
	return e.Name
}

// statProvider describes one entry of a source through its provider: the
// source's own row at the root, else the entry as its folder lists it.
func (s workspaceResourceService) statProvider(record WorkspaceRecord, ref ResourceRef, source WorkspaceSource) (ResourceItem, error) {
	p, err := s.providerOf(source)
	if err != nil {
		return ResourceItem{}, err
	}
	if ref.Path == "" || ref.Path == "." {
		item := providerSourceItem(record, source)
		item.Creates = creates(p)
		return item, nil
	}
	lister, ok := p.(resource.Lister)
	if !ok {
		return ResourceItem{}, errors.New("this source does not list its contents")
	}
	parent := ref
	parent.Path = path.Dir(ref.Path)
	if parent.Path == "." {
		parent.Path = ""
	}
	folder := parent.Path
	if folder == "" {
		folder = "."
	}
	entries, err := lister.List(s.ctx, folder)
	if err != nil {
		return ResourceItem{}, err
	}
	name := path.Base(ref.Path)
	for _, e := range entries {
		if e.Name == name {
			return providerItem(parent, source, e, creates(p)), nil
		}
	}
	return ResourceItem{}, resourceErrorText(resource.ErrNotFound)
}

// readProvider reads an entry as text: through the provider's TextReader,
// or its Snapshotter when it has no reader.
func (s workspaceResourceService) readProvider(record WorkspaceRecord, ref ResourceRef, source WorkspaceSource) (ResourceRead, error) {
	item, err := s.statProvider(record, ref, source)
	if err != nil {
		return ResourceRead{}, err
	}
	p, err := s.providerOf(source)
	if err != nil {
		return ResourceRead{}, err
	}
	var text resource.Text
	switch r := p.(type) {
	case resource.TextReader:
		text, err = r.ReadText(s.ctx, ref.Path)
	case resource.Snapshotter:
		var shot resource.Snapshot
		shot, err = r.Snapshot(s.ctx, ref.Path)
		text = resource.Text{Text: shot.Text, Revision: shot.Revision, Format: shot.Format}
	default:
		return ResourceRead{}, errors.New("this source does not read its contents as text")
	}
	if err != nil {
		return ResourceRead{}, resourceErrorText(err)
	}
	if len(text.Text) > 1<<20 {
		return ResourceRead{}, errors.New("the resource exceeds the 1 MiB text limit")
	}
	if text.Format == "" {
		text.Format = "text"
	}
	if text.Revision != "" {
		item.Revision = text.Revision
	}
	return ResourceRead{Resource: item, Text: text.Text, Format: text.Format, Revision: item.Revision}, nil
}

// previewProvider previews an entry: a picture through the provider's
// BlobReader, and anything else from its text.
func (s workspaceResourceService) previewProvider(record WorkspaceRecord, ref ResourceRef, source WorkspaceSource) (ResourcePreview, error) {
	if resourceImageExtension(ref.Path) {
		item, err := s.statProvider(record, ref, source)
		if err != nil {
			return ResourcePreview{}, err
		}
		name := path.Base(ref.Path)
		return ResourcePreview{Resource: item, Format: "image", Blocks: []preview.Block{{Line: 1, HTML: `<img class="dinah-attachment-image" data-attachment="` + html.EscapeString(name) + `" alt="` + html.EscapeString(name) + `"/>`}}, Problems: []Problem{}}, nil
	}
	read, err := s.readProvider(record, ref, source)
	if err != nil {
		return ResourcePreview{}, err
	}
	if read.Format != "markdown" {
		return ResourcePreview{Resource: read.Resource, Blocks: []preview.Block{{Line: 1, HTML: "<pre><code>" + html.EscapeString(read.Text) + "</code></pre>"}}, Problems: []Problem{}}, nil
	}
	blocks, problems := markdownPreview(read.Text)
	return ResourcePreview{Resource: read.Resource, Blocks: blocks, Problems: problems}, nil
}

// blobProvider reads a file of a source by its path, for a picture the
// Files applet shows, through the provider's BlobReader.
func (s workspaceResourceService) blobProvider(ref ResourceRef, source WorkspaceSource) ([]byte, error) {
	p, err := s.providerOf(source)
	if err != nil {
		return nil, err
	}
	reader, ok := p.(resource.BlobReader)
	if !ok {
		return nil, errors.New("this source does not provide its files")
	}
	data, err := reader.ReadBlob(s.ctx, ref.Path)
	if err != nil {
		return nil, resourceErrorText(err)
	}
	if len(data) > workspaceAttachmentLimit {
		return nil, errors.New("the file exceeds the 20 MiB preview limit")
	}
	return data, nil
}

// attachmentProvider reads a picture a document of a source shows, by the
// name the document uses, through the provider's FileSource.
func (s workspaceResourceService) attachmentProvider(ref ResourceRef, source WorkspaceSource, name string) ([]byte, error) {
	p, err := s.providerOf(source)
	if err != nil {
		return nil, err
	}
	files, ok := p.(resource.FileSource)
	if !ok {
		return nil, errors.New("this source does not provide image attachments")
	}
	data, err := files.ReadFile(s.ctx, ref.Path, strings.TrimPrefix(name, "./"))
	if err != nil {
		if errors.Is(err, resource.ErrNotFound) {
			return nil, errAttachmentNotFound
		}
		return nil, err
	}
	if len(data) > workspaceAttachmentLimit {
		return nil, errors.New("the image exceeds the 20 MiB preview limit")
	}
	return data, nil
}
