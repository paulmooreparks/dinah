package main

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"studio/internal/resource"
	"studio/internal/workspaces"
)

/* A kind of source registered with a provider is listed, read, previewed and
   described through the provider's capabilities, with nothing in the desktop
   knowing the kind (docs/design/source-kinds.md). The fake here is a small
   wiki: pages by ID with titles, child pages, and one picture attachment. */

type fakeWiki struct{ source workspaces.Source }

var fakeWikiPages = map[string]struct {
	title    string
	text     string
	children []string
}{
	"100": {title: "Customer Home", text: "# Customer Home\n\nWelcome.", children: []string{"200", "300"}},
	"200": {title: "Architecture", text: "# Architecture\n\nBoxes.", children: nil},
	"300": {title: "Agenda", text: "# Agenda", children: nil},
}

func (w fakeWiki) List(_ context.Context, folder string) ([]resource.Entry, error) {
	id := folder
	if folder == "." {
		id = workspaces.ExtraString(w.source.Extra, "pageId")
	}
	if folder == "attachments" || strings.HasSuffix(folder, "/attachments") {
		return []resource.Entry{{Name: "diagram.png", Size: 4, Kind: "attachment"}}, nil
	}
	page, ok := fakeWikiPages[id]
	if !ok {
		return nil, resource.ErrNotFound
	}
	out := []resource.Entry{}
	if id == "100" {
		out = append(out, resource.Entry{Name: "attachments", Folder: true, Title: "Attachments"})
	}
	for _, child := range page.children {
		c := fakeWikiPages[child]
		out = append(out, resource.Entry{Name: child, Title: c.title, Folder: true, Document: true, Kind: "wiki-page", Glyph: "globe", URL: "https://wiki.example/pages/" + child, Revision: "3"})
	}
	return out, nil
}

func (w fakeWiki) Snapshot(_ context.Context, path string) (resource.Snapshot, error) {
	id := strings.TrimPrefix(path, ".")
	if id == "" {
		id = workspaces.ExtraString(w.source.Extra, "pageId")
	}
	page, ok := fakeWikiPages[id]
	if !ok {
		return resource.Snapshot{}, resource.ErrNotFound
	}
	return resource.Snapshot{Text: page.text, Revision: "3", Format: "markdown", Title: page.title}, nil
}

func (w fakeWiki) ReadBlob(_ context.Context, path string) ([]byte, error) {
	if strings.HasSuffix(path, "/attachments/diagram.png") {
		return []byte("PNG!"), nil
	}
	return nil, resource.ErrNotFound
}

func (w fakeWiki) FileNames(context.Context, string) ([]string, error) {
	return []string{"diagram.png"}, nil
}
func (w fakeWiki) ReadFile(_ context.Context, _ string, name string) ([]byte, error) {
	if name == "diagram.png" {
		return []byte("PNG!"), nil
	}
	return nil, resource.ErrNotFound
}

var registerFakeWiki = func() func() {
	done := false
	return func() {
		if done {
			return
		}
		done = true
		workspaces.RegisterSourceKind("test-wiki", workspaces.SourceKind{
			Label: "Test Wiki Page",
			Glyph: "globe",
			URL: func(s workspaces.Source) string {
				return "https://wiki.example/pages/" + workspaces.ExtraString(s.Extra, "pageId")
			},
			Attach: &workspaces.Attach{
				Help:   "A page of the test wiki.",
				Fields: []workspaces.Field{{ID: "page", Label: "Page ID", Kind: "text", Required: true, MaxLength: 10}, {ID: "children", Label: "Include Child Pages", Kind: "check", Default: "true"}},
				Build: func(_ context.Context, v map[string]string) (workspaces.Source, error) {
					if _, ok := fakeWikiPages[v["page"]]; !ok {
						return workspaces.Source{}, errors.New("the page could not be found")
					}
					extra := workspaces.WithExtra(nil, "pageId", v["page"])
					extra = workspaces.WithExtra(extra, "children", v["children"] == "true")
					return workspaces.Source{Kind: "test-wiki", Name: fakeWikiPages[v["page"]].title, Extra: extra}, nil
				},
			},
			Provider: func(_ workspaces.Resolver, _ context.Context, s workspaces.Source) (any, error) {
				return fakeWiki{source: s}, nil
			},
		})
	}
}()

func TestRegisteredKindIsListedReadAndPreviewedThroughItsProvider(t *testing.T) {
	registerFakeWiki()
	source := WorkspaceSource{ID: "wiki", Kind: "test-wiki", Name: "Customer Wiki", Extra: workspaces.WithExtra(nil, "pageId", "100")}
	service, ref := resourceTestService(t, source)
	service.app.settings.WorkspaceRecords[0].MountAlias = "resources"
	service.app.settings.WorkspaceRecords[0].Sources[0].MountAlias = "wiki"
	record := service.app.settings.WorkspaceRecords[0]
	source = record.Sources[0]

	row := workspaceSourceResource(record, source)
	if !row.Directory || row.Unavailable || !slices.Contains(row.Actions, "list") {
		t.Fatalf("the source's row does not list: %+v", row)
	}
	// The source's own row takes its kind's glyph and address, so it is drawn
	// as what it is and has Copy Link and Open Externally like its pages.
	if row.Glyph != "globe" || row.URL != "https://wiki.example/pages/100" || !slices.Contains(row.Actions, "open-external") {
		t.Fatalf("the source's row lacks its kind's glyph or address: %+v", row)
	}
	other := workspaceSourceResource(record, WorkspaceSource{ID: "x", Kind: "nobody-knows", Name: "Kept"})
	if !other.Unavailable || other.Directory {
		t.Fatalf("a kind without a provider is not kept as unavailable: %+v", other)
	}

	page, err := service.list(ref, 0, 100)
	if err != nil || len(page.Entries) != 3 {
		t.Fatalf("list: %+v %v", page, err)
	}
	// Folders first, then by title: Attachments, Agenda, Architecture; the
	// attachments folder is a plain folder, the pages are documents too.
	titles := []string{}
	for _, e := range page.Entries {
		titles = append(titles, e.Title)
	}
	if strings.Join(titles, ",") != "Agenda,Architecture,Attachments" && strings.Join(titles, ",") != "Attachments,Agenda,Architecture" {
		t.Fatalf("order: %v", titles)
	}
	var arch ResourceItem
	for _, e := range page.Entries {
		if e.Title == "Architecture" {
			arch = e
		}
	}
	if arch.Ref.Path != "200" || arch.Kind != "wiki-page" || arch.Revision != "3" || arch.URL == "" || arch.Glyph != "globe" {
		t.Fatalf("the page's item: %+v", arch)
	}
	for _, action := range []string{"list", "read", "preview", "edit", "open-external"} {
		if !slices.Contains(arch.Actions, action) {
			t.Fatalf("the page lacks %s: %v", action, arch.Actions)
		}
	}

	paged, err := service.list(ref, 1, 1)
	if err != nil || len(paged.Entries) != 1 || paged.NextStart != 2 {
		t.Fatalf("paging: %+v %v", paged, err)
	}

	read, err := service.read(ResourceRef{WorkspaceID: ref.WorkspaceID, SourceID: "wiki", Path: "200"})
	if err != nil || read.Text != "# Architecture\n\nBoxes." || read.Format != "markdown" || read.Revision != "3" || read.Resource.Title != "Architecture" {
		t.Fatalf("read: %+v %v", read, err)
	}
	preview, err := service.preview(ResourceRef{WorkspaceID: ref.WorkspaceID, SourceID: "wiki", Path: "200"})
	if err != nil || len(preview.Blocks) == 0 || !strings.Contains(preview.Blocks[0].HTML, "Architecture") {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	image, err := service.preview(ResourceRef{WorkspaceID: ref.WorkspaceID, SourceID: "wiki", Path: "attachments/diagram.png"})
	if err != nil || image.Format != "image" {
		t.Fatalf("image preview: %+v %v", image, err)
	}
	data, err := service.attachment(ResourceRef{WorkspaceID: ref.WorkspaceID, SourceID: "wiki", Path: "200"}, "diagram.png")
	if err != nil || string(data) != "PNG!" {
		t.Fatalf("attachment: %q %v", data, err)
	}
	if _, err := service.read(ResourceRef{WorkspaceID: ref.WorkspaceID, SourceID: "wiki", Path: "999"}); err == nil {
		t.Fatal("a missing page was read")
	}

	vfs := workspaceVFS{service: service}
	item, err := vfs.stat("/workspaces/" + record.MountAlias + "/" + source.MountAlias + "/200")
	if err != nil || item.Title != "Architecture" || !item.Directory {
		t.Fatalf("stat through the mount: %+v %v", item, err)
	}
	root, err := vfs.stat("/workspaces/" + record.MountAlias + "/" + source.MountAlias)
	if err != nil || root.Title != "Customer Wiki" || !root.Directory {
		t.Fatalf("the source through the mount: %+v %v", root, err)
	}
}

func TestAttachWorkspaceSourceBuildsThroughTheKind(t *testing.T) {
	registerFakeWiki()
	service, ref := resourceTestService(t, WorkspaceSource{ID: "local", Kind: "local-folder", Name: "Local", Path: t.TempDir()})
	a := service.app

	kinds := a.WorkspaceSourceKinds()
	var wiki *SourceKindInfo
	for i := range kinds {
		if kinds[i].Kind == "test-wiki" {
			wiki = &kinds[i]
		}
	}
	if wiki == nil || wiki.Label != "Test Wiki Page" || len(wiki.Fields) != 2 || wiki.Fields[1].Kind != "check" {
		t.Fatalf("the kind is not offered as registered: %+v", kinds)
	}

	if _, err := a.AttachWorkspaceSource(ref.WorkspaceID, "test-wiki", map[string]string{"page": ""}); err == nil || !strings.Contains(err.Error(), "Page ID") {
		t.Fatalf("an empty required field: %v", err)
	}
	if _, err := a.AttachWorkspaceSource(ref.WorkspaceID, "test-wiki", map[string]string{"page": "12345678901"}); err == nil {
		t.Fatal("a value over the field's length was accepted")
	}
	if _, err := a.AttachWorkspaceSource(ref.WorkspaceID, "test-wiki", map[string]string{"page": "999"}); err == nil || !strings.Contains(err.Error(), "could not be found") {
		t.Fatalf("the kind's own check: %v", err)
	}
	if _, err := a.AttachWorkspaceSource(ref.WorkspaceID, "local-folder", nil); err == nil {
		t.Fatal("a local folder was attached through the form")
	}
	record, err := a.AttachWorkspaceSource(ref.WorkspaceID, "test-wiki", map[string]string{"page": "100", "children": "true"})
	if err != nil || len(record.Sources) != 2 {
		t.Fatalf("attach: %+v %v", record, err)
	}
	added := record.Sources[1]
	if added.Kind != "test-wiki" || added.Name != "Customer Home" || !strings.HasPrefix(added.ID, "src_") || added.MountAlias == "" || workspaces.ExtraString(added.Extra, "pageId") != "100" || !workspaces.ExtraBool(added.Extra, "children") {
		t.Fatalf("the added source: %+v", added)
	}
	again, err := a.AttachWorkspaceSource(ref.WorkspaceID, "test-wiki", map[string]string{"page": "100", "children": "true"})
	if err != nil || len(again.Sources) != 2 {
		t.Fatalf("the same source twice: %+v %v", again, err)
	}
	// The registry keeps the source's fields, so a restart lists it again.
	settings, _, err := migrateWorkspaceSettings(a.settings)
	if err != nil {
		t.Fatalf("the registry refuses the source: %v", err)
	}
	if got := settings.WorkspaceRecords[0].Sources[1]; got.Kind != "test-wiki" || workspaces.ExtraString(got.Extra, "pageId") != "100" {
		t.Fatalf("the registry lost the source's fields: %+v", got)
	}
}
