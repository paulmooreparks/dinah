package main

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"studio/internal/layout"
	"studio/internal/preview"
	"studio/internal/resource/localfs"
	"studio/internal/workspaces"
)

//go:embed workspace_resources.go
var workspaceResourcesSource []byte

type ResourceRef = workspaces.Ref
type ResourceItem struct {
	Ref       ResourceRef `json:"ref"`
	Title     string      `json:"title"`
	Kind      string      `json:"kind"`
	Directory bool        `json:"directory"`
	Revision  string      `json:"revision"`
	URL       string      `json:"url"`
	Actions   []string    `json:"actions"`
	// Size and Modified are a local file's size in bytes and its last
	// change, in RFC 3339; Writable says the Terminal may change it, which
	// is true of local folders and what is in them.
	Size     int64  `json:"size,omitempty"`
	Modified string `json:"modified,omitempty"`
	Writable bool   `json:"writable,omitempty"`
	// Missing marks a local folder that is not on the computer, and the
	// folders its import recorded inside it, which are shown until it is
	// (workspace_export.go).
	Missing bool `json:"missing,omitempty"`
	// Unavailable marks a source of a kind this build does not know, which
	// an import kept for a build that does; nothing can be done with it
	// here but removing it.
	Unavailable bool `json:"unavailable,omitempty"`
}
type ResourcePage struct {
	Entries   []ResourceItem `json:"entries"`
	NextStart int            `json:"nextStart"`
}
type ResourceRead struct {
	Resource ResourceItem `json:"resource"`
	Text     string       `json:"text"`
	Format   string       `json:"format"`
	Revision string       `json:"revision"`
}
type ResourcePreview struct {
	Resource ResourceItem    `json:"resource"`
	Format   string          `json:"format,omitempty"`
	Blocks   []preview.Block `json:"blocks"`
	Problems []Problem       `json:"problems"`
}

type workspaceResourceService struct {
	app *App
	ctx context.Context
}

func (a *App) resourceService() workspaceResourceService {
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return workspaceResourceService{app: a, ctx: ctx}
}
func (a *App) listWorkspaceResources(ctx context.Context, ref ResourceRef, start, limit int) (ResourcePage, error) {
	s := a.resourceService()
	s.ctx = ctx
	return (workspaceVFS{service: s, allowedWorkspace: ref.WorkspaceID}).listResource(ref, start, limit)
}
func (a *App) readWorkspaceResource(ctx context.Context, ref ResourceRef) (ResourceRead, error) {
	s := a.resourceService()
	s.ctx = ctx
	return (workspaceVFS{service: s, allowedWorkspace: ref.WorkspaceID}).readResource(ref)
}

// textExtensions are the kinds of local file that are read, previewed and
// edited as text: documents, data and configuration, scripts and source code.
// A file with no extension, such as a script in /bin, is text too.
var textExtensions = map[string]bool{
	"": true, ".md": true, ".markdown": true, ".txt": true, ".log": true,
	".csv": true, ".tsv": true, ".json": true, ".yaml": true, ".yml": true, ".toml": true, ".xml": true,
	".ini": true, ".cfg": true, ".conf": true, ".properties": true, ".sql": true,
	".lua": true, ".sh": true, ".bash": true, ".ps1": true, ".psm1": true, ".bat": true, ".cmd": true,
	".go": true, ".py": true, ".js": true, ".mjs": true, ".ts": true, ".java": true, ".cs": true, ".groovy": true,
	".css": true, ".html": true, ".d2": true, ".mmd": true,
}

// textExtension is a file's extension for textExtensions, in lower case. A
// name whose only dot is its first character, such as .profile, has none.
func textExtension(path string) string {
	base := filepath.Base(filepath.FromSlash(path))
	if strings.LastIndex(base, ".") == 0 {
		return ""
	}
	return strings.ToLower(filepath.Ext(base))
}

func workspaceSourceResource(record WorkspaceRecord, source WorkspaceSource) ResourceItem {
	item := ResourceItem{Ref: ResourceRef{WorkspaceID: record.ID, SourceID: source.ID}, Title: source.Name, Kind: source.Kind, Directory: true, Actions: []string{"list"}, Writable: source.Kind == "local-folder"}
	switch {
	case source.Kind != "local-folder":
		item.Directory, item.Unavailable, item.Actions = false, true, []string{}
	case !folderExists(source.Path):
		item.Missing, item.Writable = true, false
		item.Actions = []string{"list", "create-folders"}
		// The main folder cannot move, since the workspace's chats find it
		// by its path.
		if record.ID != "" && !layout.SamePath(source.Path, workspaces.ExecutionPath(record)) {
			item.Actions = append(item.Actions, "choose-folder")
		}
	}
	return item
}
func (a *App) ListWorkspaceResources(ref ResourceRef, start, limit int) (ResourcePage, error) {
	return (workspaceVFS{service: a.resourceService()}).listResource(ref, start, limit)
}
func (a *App) ReadWorkspaceResource(ref ResourceRef) (ResourceRead, error) {
	return (workspaceVFS{service: a.resourceService()}).readResource(ref)
}
func (a *App) PreviewWorkspaceResource(ref ResourceRef) (ResourcePreview, error) {
	return (workspaceVFS{service: a.resourceService()}).previewResource(ref)
}

// resolver finds sources and their providers from the app's registry, as
// package workspaces does for both programs.
func (s workspaceResourceService) resolver() workspaces.Resolver {
	return workspaces.Resolver{
		Layout: stateLayout(),
		Records: func() []WorkspaceRecord {
			s.app.mu.Lock()
			defer s.app.mu.Unlock()
			return workspaces.Copy(s.app.settings.WorkspaceRecords)
		},
		Personal: personalSource,
	}
}

func (s workspaceResourceService) source(ref ResourceRef) (WorkspaceSource, error) {
	return s.resolver().Source(ref)
}

// All local opens use Root confinement. Hidden attributes and aliases assume
// trusted local processes; they do not protect against local ancestor mutation.
func openWorkspaceResource(source WorkspaceSource, path string) (*os.File, error) {
	name, err := localfs.CleanPathFor(path, workspaces.Dotfiles(source))
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(source.Path)
	if err != nil {
		return nil, errors.New("the local source is unavailable")
	}
	defer root.Close()
	var expected os.FileInfo
	part := ""
	if name != "." {
		for _, segment := range strings.Split(name, string(filepath.Separator)) {
			part = filepath.Join(part, segment)
			info, e := root.Lstat(part)
			if e != nil || info.Mode()&os.ModeSymlink != 0 {
				return nil, errors.New("the source path is unavailable or is a symbolic link")
			}
			hidden, e := workspacePathHidden(filepath.Join(source.Path, part))
			if e != nil || hidden {
				return nil, errors.New("hidden and system paths are unavailable")
			}
			expected = info
		}
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, errors.New("the source path could not be opened")
	}
	st, err := f.Stat()
	hidden, attrErr := workspaceFileHidden(f)
	if err != nil || attrErr != nil || hidden || (expected != nil && !os.SameFile(expected, st)) {
		f.Close()
		return nil, errors.New("the source path changed or is unavailable")
	}
	return f, nil
}
func resourceText(f *os.File) (string, error) {
	st, err := f.Stat()
	if err != nil || !st.Mode().IsRegular() || st.Size() > 1<<20 {
		return "", errors.New("preview requires a regular text file no larger than 1 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil || len(data) > 1<<20 || !utf8.Valid(data) || strings.ContainsRune(string(data), '\x00') {
		return "", errors.New("the resource is not UTF-8 text within the 1 MiB preview limit")
	}
	return string(data), nil
}
func localResourceItem(ref ResourceRef, source WorkspaceSource, f *os.File) (ResourceItem, error) {
	st, err := f.Stat()
	if err != nil {
		return ResourceItem{}, errors.New("the source path could not be inspected")
	}
	item := ResourceItem{Ref: ref, Title: st.Name(), Kind: "file", Directory: st.IsDir(), Revision: fmt.Sprintf("%d:%d", st.ModTime().UnixNano(), st.Size()), Actions: []string{}, Modified: st.ModTime().UTC().Format(time.RFC3339), Writable: true}
	if !st.IsDir() {
		item.Size = st.Size()
	}
	if ref.Path == "" || ref.Path == "." {
		item.Title = source.Name
	}
	if st.IsDir() {
		item.Kind = "directory"
		item.Actions = []string{"list"}
		return item, nil
	}
	if !st.Mode().IsRegular() {
		return item, nil
	}
	item.Actions = []string{"reveal"}
	if openableExt[strings.ToLower(filepath.Ext(ref.Path))] {
		item.Actions = []string{"open-external"}
	}
	if editorOpens(ref.Path, st.Size()) {
		item.Actions = append(item.Actions, "read", "preview", "edit")
	}
	if st.Size() <= 1<<20 && strings.EqualFold(filepath.Ext(ref.Path), ".mermaid") {
		item.Actions = append(item.Actions, "read", "preview")
	}
	if imagePreviewable(ref.Path, st.Size()) {
		item.Actions = append(item.Actions, "preview")
	}
	// A file Dinah knows the kind of can be renamed in its folder
	// (RenameDocumentFile); one it can only show in its folder is left as
	// it is.
	if len(item.Actions) > 1 || item.Actions[0] != "reveal" {
		item.Actions = append(item.Actions, "rename")
	}
	return item, nil
}
func (s workspaceResourceService) list(ref ResourceRef, start, limit int) (ResourcePage, error) {
	out := ResourcePage{Entries: []ResourceItem{}, NextStart: -1}
	if s.ctx != nil && s.ctx.Err() != nil {
		return out, s.ctx.Err()
	}
	if start < 0 || start > 1000000 || limit < 1 || limit > 100 {
		return out, errors.New("use a start between 0 and 1000000 and a limit between 1 and 100")
	}
	source, err := s.source(ref)
	if err != nil {
		return out, err
	}
	if source.Kind != "local-folder" {
		return out, errors.New("unsupported workspace source")
	}
	if !folderExists(source.Path) {
		return plannedPage(ref, s.plannedFolders(ref), start, limit), nil
	}
	f, err := openWorkspaceResource(source, ref.Path)
	if err != nil {
		return out, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || !st.IsDir() {
		return out, errors.New("the resource is not a directory")
	}
	// Offsets count filename-sorted raw entries, including filtered entries.
	// Enumeration reads only this directory's metadata. Each page inspects at
	// most limit candidates and never reads file bodies or child directories.
	// The first page reads the directory, and later pages of the same browse
	// use that reading (folderListings), so the pages are one snapshot of it.
	// Each entry is still opened and checked when its page is listed, so an
	// entry removed since the reading is skipped.
	entries, err := folderListings.get(source.Path+"|"+ref.Path, st, start, func() ([]os.DirEntry, error) {
		return sortedWorkspaceDirectory(s.ctx, f, 100000)
	})
	if err != nil {
		return out, err
	}
	if start >= len(entries) {
		return out, nil
	}
	entries = entries[start:]
	if len(entries) > limit {
		out.NextStart = start + limit
		entries = entries[:limit]
	}
	for _, e := range entries {
		if s.ctx != nil && s.ctx.Err() != nil {
			return out, s.ctx.Err()
		}
		if localfs.HiddenName(e.Name(), workspaces.Dotfiles(source)) || e.Type()&os.ModeSymlink != 0 {
			continue
		}
		child := ref
		child.Path = filepath.ToSlash(filepath.Join(filepath.FromSlash(ref.Path), e.Name()))
		opened, e := openWorkspaceResource(source, child.Path)
		if e != nil {
			continue
		}
		item, e := localResourceItem(child, source, opened)
		opened.Close()
		if e == nil {
			out.Entries = append(out.Entries, item)
		}
	}
	return out, nil
}

// folderListings keeps a folder's sorted listing between the pages of one
// browse. The first page reads the folder, and a later page within
// listingKept uses that reading, instead of reading and sorting the whole
// folder again for every page of 100. A reading is used only for the same
// directory it was made from, by os.SameFile, so a folder replaced at the
// same path is read again.
var folderListings = &listingCache{byPath: map[string]listing{}}

const listingKept = 30 * time.Second

type listing struct {
	entries []os.DirEntry
	dir     os.FileInfo
	at      time.Time
}

type listingCache struct {
	mu     sync.Mutex
	byPath map[string]listing
}

// get returns the sorted listing of the directory at key, whose information
// is dir. A first page (start 0) always reads the directory with read; a
// later page uses the kept reading when it is recent and of the same
// directory. Expired readings are dropped whenever a new one is kept.
func (c *listingCache) get(key string, dir os.FileInfo, start int, read func() ([]os.DirEntry, error)) ([]os.DirEntry, error) {
	c.mu.Lock()
	kept, ok := c.byPath[key]
	c.mu.Unlock()
	if start > 0 && ok && time.Since(kept.at) < listingKept && os.SameFile(kept.dir, dir) {
		return kept.entries, nil
	}
	entries, err := read()
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for p, l := range c.byPath {
		if time.Since(l.at) >= listingKept {
			delete(c.byPath, p)
		}
	}
	c.byPath[key] = listing{entries: entries, dir: dir, at: time.Now()}
	return entries, nil
}

func sortedWorkspaceDirectory(ctx context.Context, f *os.File, maximum int) ([]os.DirEntry, error) {
	entries := []os.DirEntry{}
	for {
		if ctx != nil && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		batch, err := f.ReadDir(100)
		if len(entries)+len(batch) > maximum {
			return nil, fmt.Errorf("the directory exceeds the browsing limit of %d entries", maximum)
		}
		entries = append(entries, batch...)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.New("the directory could not be listed")
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return folderOrder(entries[i].IsDir(), entries[i].Name(), entries[j].IsDir(), entries[j].Name())
	})
	return entries, nil
}

// folderOrder is the order of a folder's entries: folders before files, each
// by name regardless of case, and names that differ only in case by their
// exact spelling, so the order is the same on every listing.
func folderOrder(aDir bool, a string, bDir bool, b string) bool {
	if aDir != bDir {
		return aDir
	}
	if la, lb := strings.ToLower(a), strings.ToLower(b); la != lb {
		return la < lb
	}
	return a < b
}

func (s workspaceResourceService) read(ref ResourceRef) (ResourceRead, error) {
	if s.ctx != nil && s.ctx.Err() != nil {
		return ResourceRead{}, s.ctx.Err()
	}
	source, err := s.source(ref)
	if err != nil {
		return ResourceRead{}, err
	}
	if source.Kind != "local-folder" {
		return ResourceRead{}, errors.New("unsupported workspace source")
	}
	f, err := openWorkspaceResource(source, ref.Path)
	if err != nil {
		return ResourceRead{}, err
	}
	defer f.Close()
	item, err := localResourceItem(ref, source, f)
	if err != nil {
		return ResourceRead{}, err
	}
	text, err := resourceText(f)
	if err != nil {
		return ResourceRead{}, err
	}
	sum := sha256.Sum256([]byte(text))
	item.Revision = hex.EncodeToString(sum[:])
	format := "text"
	if strings.EqualFold(filepath.Ext(ref.Path), ".md") || strings.EqualFold(filepath.Ext(ref.Path), ".markdown") {
		format = "markdown"
	}
	return ResourceRead{Resource: item, Text: text, Format: format, Revision: item.Revision}, nil
}

// imageExtensions are the kinds of picture Dinah shows itself, in a
// preview and in the media viewer.
var imageExtensions = []string{".png", ".jpg", ".jpeg", ".gif", ".webp", ".svg"}

func resourceImageExtension(name string) bool {
	return slices.Contains(imageExtensions, strings.ToLower(filepath.Ext(name)))
}

// editorOpens says whether the Editor opens a file of this name and size:
// text it knows the kind of, no larger than 1 MiB. Files and a link in a
// chat both decide by it.
func editorOpens(path string, size int64) bool {
	return size <= maxComputerDocument && textExtensions[textExtension(path)]
}

// imagePreviewable says whether a picture of this name and size is shown
// by Dinah: a kind it shows, no larger than 20 MiB.
func imagePreviewable(path string, size int64) bool {
	return size <= workspaceAttachmentLimit && resourceImageExtension(path)
}

func (s workspaceResourceService) preview(ref ResourceRef) (ResourcePreview, error) {
	source, err := s.source(ref)
	if err != nil {
		return ResourcePreview{}, err
	}
	if source.Kind == "local-folder" && resourceImageExtension(ref.Path) {
		f, err := openWorkspaceResource(source, ref.Path)
		if err != nil {
			return ResourcePreview{}, err
		}
		defer f.Close()
		item, err := localResourceItem(ref, source, f)
		if err != nil {
			return ResourcePreview{}, err
		}
		st, err := f.Stat()
		if err != nil || !st.Mode().IsRegular() || st.Size() > 20<<20 {
			return ResourcePreview{}, errors.New("image preview requires a regular image no larger than 20 MiB")
		}
		name := filepath.Base(ref.Path)
		return ResourcePreview{Resource: item, Format: "image", Blocks: []preview.Block{{Line: 1, HTML: `<img class="dinah-attachment-image" data-attachment="` + html.EscapeString(name) + `" alt="` + html.EscapeString(name) + `"/>`}}, Problems: []Problem{}}, nil
	}
	read, err := s.read(ref)
	if err != nil {
		return ResourcePreview{}, err
	}
	text := read.Text
	if source.Kind == "local-folder" {
		language := ""
		switch strings.ToLower(filepath.Ext(ref.Path)) {
		case ".d2":
			language = "d2"
		case ".mmd", ".mermaid":
			language = "mermaid"
		}
		if language != "" {
			return ResourcePreview{Resource: read.Resource, Format: language, Blocks: []preview.Block{{Line: 1, HTML: `<div data-preview-diagram="resource-diagram"><pre>` + html.EscapeString(text) + `</pre></div>`, Diagrams: []preview.Diagram{{ID: "resource-diagram", Language: language, Source: text, Line: 1}}}}, Problems: []Problem{}}, nil
		}
	}
	if read.Format != "markdown" {
		return ResourcePreview{Resource: read.Resource, Blocks: []preview.Block{{Line: 1, HTML: "<pre><code>" + html.EscapeString(text) + "</code></pre>"}}, Problems: []Problem{}}, nil
	}
	blocks, problems := markdownPreview(text)
	return ResourcePreview{Resource: read.Resource, Blocks: blocks, Problems: problems}, nil
}

// markdownPreview renders Markdown as preview blocks. A resource's preview
// and a draft's preview both use it, so they look the same.
func markdownPreview(text string) ([]preview.Block, []Problem) {
	blocks, found := preview.Render(text)
	if blocks == nil {
		blocks = []preview.Block{}
	}
	problems := []Problem{}
	for _, problem := range found {
		problems = append(problems, Problem{Line: problem.Line, Severity: "warning", Message: problem.Message})
	}
	return blocks, problems
}

// OpenWorkspaceResource is an explicit user action. Applet read actions do not
// expose it because opening a local association may execute a program.
func (a *App) OpenWorkspaceResource(ref ResourceRef) error {
	s := a.resourceService()
	source, err := (workspaceVFS{service: s}).resourceSource(ref)
	if err != nil {
		return err
	}
	if source.Kind != "local-folder" {
		return errors.New("unsupported workspace source")
	}
	f, err := openWorkspaceResource(source, ref.Path)
	if err != nil {
		return err
	}
	st, err := f.Stat()
	f.Close()
	if err != nil || !st.Mode().IsRegular() {
		return errors.New("choose a regular file to open")
	}
	name, err := workspaceRelativePath(ref.Path)
	if err != nil {
		return err
	}
	path := filepath.Join(source.Path, name)
	if openableExt[strings.ToLower(filepath.Ext(path))] {
		return shellOpen(path)
	}
	return showInFolder(path)
}
