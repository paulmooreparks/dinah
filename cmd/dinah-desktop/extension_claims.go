package main

import (
	"encoding/json"
	"math"
	"regexp"
	"slices"
	"sort"

	extension "github.com/paulmooreparks/dinah-extension-go"
)

/* Files that open in an extension's view (docs/plans/document-links.md and
   docs/design/extensions.md). A view in a signed extension may declare the
   media types it opens, each with the file extensions that have it (the
   SDK's View.Opens, added in SDK 0.2.0). Dinah registers the declarations of
   every enabled extension, and a link to such a file opens the view with the
   file in its navigation state.

   Precedence, as Paul approved on 28 October 2026:

   - Dinah's own parts come first. A media type one of them claims (the
     media viewer's pictures and SVG) and a kind of file the Editor opens
     (textExtensions) stay with Dinah, so installing an extension never
     takes them away. A choice by the person to give such a type to an
     extension is left for later.
   - Among extensions, the one enabled first opens a type that two of them
     declare. An update keeps an extension's place; disabling and enabling
     it again moves it to the end. Settings › Extensions shows a view's
     declared types that open somewhere else.
   - A declaration never changes the media type of a file extension Dinah
     already knows. A file matches a claim when Dinah knows its extension as
     the claimed type, or when Dinah has no type for its extension and the
     claim lists it.
   - Only a file inside a source of one of the person's workspaces opens in
     an extension's view, since the view opens in the file's workspace and
     reads the file through the host's workspace services. */

// glyphName is a PUDL glyph's name as a view may give it.
var glyphName = regexp.MustCompile(`^[a-z0-9-]{1,40}$`)

// setExtensionClaims replaces the extensions' claims with these, ordered by
// when each extension was enabled and then by its ID.
func setExtensionClaims(claims []mediaClaim) {
	sort.SliceStable(claims, func(i, j int) bool {
		if !claims[i].Since.Equal(claims[j].Since) {
			return claims[i].Since.Before(claims[j].Since)
		}
		return claims[i].Extension < claims[j].Extension
	})
	mediaClaims.Lock()
	defer mediaClaims.Unlock()
	mediaClaims.extensions = claims
	mediaClaims.extensionsLoaded = true
}

// syncExtensionClaims registers the file types that the views of the
// enabled extensions declare, in place of those registered before. It runs
// when the extensions are first needed and after every installation,
// update, enabling, disabling or removal (resetExtensionCatalog). A damaged
// package, which Settings reports, claims nothing, and when the store
// cannot be read no extension claims anything.
func (a *App) syncExtensionClaims() {
	manager := a.extensionManager()
	claims := []mediaClaim{}
	list, err := manager.Store.List()
	if err == nil {
		for _, v := range list {
			if !v.Enabled || !declaresFileTypes(v.Manifest) {
				continue
			}
			if _, err := manager.Installed(v.Manifest.ID); err != nil {
				continue
			}
			for _, view := range v.Manifest.Views {
				glyph := "app"
				if glyphName.MatchString(view.Glyph) {
					glyph = view.Glyph
				}
				for _, t := range view.Opens {
					claims = append(claims, mediaClaim{Opens: opensExtension, Label: "Open in " + view.Title, Glyph: glyph, Limit: math.MaxInt64, Name: v.Manifest.Title, Extension: v.Manifest.ID, View: view.ID, MediaType: t.MediaType, FileExtensions: slices.Clone(t.Extensions), Since: v.EnabledSince()})
				}
			}
		}
	}
	setExtensionClaims(claims)
}

func declaresFileTypes(m extension.Manifest) bool {
	for _, v := range m.Views {
		if len(v.Opens) > 0 {
			return true
		}
	}
	return false
}

// loadExtensionClaims registers the extensions' claims once, the first time
// a link needs them.
func (a *App) loadExtensionClaims() {
	mediaClaims.RLock()
	loaded := mediaClaims.extensionsLoaded
	mediaClaims.RUnlock()
	if !loaded {
		a.syncExtensionClaims()
	}
}

// extensionClaimOf is the extension's claim that opens a file of this name,
// whose media type by Dinah's own table is mediaType, when no part of
// Dinah opens it.
func extensionClaimOf(path, mediaType string) (mediaClaim, bool) {
	ext := textExtension(path)
	if textExtensions[ext] {
		return mediaClaim{}, false
	}
	_, known := mediaTypes[ext]
	mediaClaims.RLock()
	defer mediaClaims.RUnlock()
	if _, ok := mediaClaims.byType[mediaType]; ok {
		return mediaClaim{}, false
	}
	for _, c := range mediaClaims.extensions {
		if (known && c.MediaType == mediaType) || (!known && slices.Contains(c.FileExtensions, ext)) {
			return c, true
		}
	}
	return mediaClaim{}, false
}

// extensionLink completes a link to a file, out, that no part of Dinah
// opens, when an extension's view opens it: the file is in a source of one
// of the person's workspaces and an enabled extension claims its kind.
func (a *App) extensionLink(out DocumentLink, path string) (DocumentLink, bool) {
	if out.Resource == nil || out.Resource.WorkspaceID == "" {
		return out, false
	}
	a.loadExtensionClaims()
	claim, ok := extensionClaimOf(path, out.MediaType)
	if !ok {
		return out, false
	}
	r := *out.Resource
	state, err := json.Marshal(extension.OpenState{Open: extension.OpenedFile{Path: path, MediaType: claim.MediaType, Resource: extension.FileResource{WorkspaceID: r.WorkspaceID, SourceID: r.SourceID, Path: r.Path}, Line: out.Line}})
	if err != nil || len(state) > extension.OpenStateLimit {
		return out, false
	}
	out.Opens, out.MediaType, out.Label, out.Glyph = opensExtension, claim.MediaType, claim.Label, claim.Glyph
	out.Extension, out.ExtensionView, out.State = claim.Extension, claim.View, string(state)
	return out, true
}

// ExtensionFileType is a file type a view of an installed extension
// declares, as Settings › Extensions lists it. Elsewhere lists the file
// extensions of the type that open somewhere other than this view, while
// the extension is enabled.
type ExtensionFileType struct {
	View       string              `json:"view"`
	ViewTitle  string              `json:"viewTitle"`
	MediaType  string              `json:"mediaType"`
	Extensions []string            `json:"extensions"`
	Elsewhere  []FileTypeElsewhere `json:"elsewhere,omitempty"`
}

// FileTypeElsewhere is a file extension that does not open in the view
// that declares it. Why is the reason and By the detail for it:
//
//   - "dinah": a part of Dinah opens it, and By is that part ("the Editor",
//     "the Media Viewer");
//   - "earlier": an extension enabled before this one opens it, and By is
//     that extension's title;
//   - "type": Dinah knows the file extension as another media type, which
//     By is, so the declaration does not match it.
type FileTypeElsewhere struct {
	FileExtension string `json:"fileExtension"`
	By            string `json:"by"`
	Why           string `json:"why"`
}

// extensionFileTypes lists the file types an installation's views declare,
// and, when it is enabled, which of their file extensions open elsewhere.
// The file extensions of a type are those its declaration lists and those
// Dinah knows as that type.
func extensionFileTypes(v extension.Installation) []ExtensionFileType {
	out := []ExtensionFileType{}
	for _, view := range v.Manifest.Views {
		for _, t := range view.Opens {
			row := ExtensionFileType{View: view.ID, ViewTitle: view.Title, MediaType: t.MediaType, Extensions: slices.Clone(t.Extensions)}
			if v.Enabled {
				exts := slices.Clone(t.Extensions)
				for ext, known := range mediaTypes {
					if known == t.MediaType && !slices.Contains(exts, ext) {
						exts = append(exts, ext)
					}
				}
				sort.Strings(exts[len(t.Extensions):])
				for _, ext := range exts {
					if e, ok := opensElsewhere(ext, t.MediaType, v.Manifest.ID, view.ID); ok {
						row.Elsewhere = append(row.Elsewhere, e)
					}
				}
			}
			out = append(out, row)
		}
	}
	return out
}

// opensElsewhere says why a file with this extension, which a view of the
// extension id declares as declared, does not open in that view, when it
// does not.
func opensElsewhere(ext, declared, id, view string) (FileTypeElsewhere, bool) {
	name := "file" + ext
	mediaType := mediaTypeOf(name)
	if c, ok := claimOf(mediaType); ok {
		return FileTypeElsewhere{FileExtension: ext, By: c.Name, Why: "dinah"}, true
	}
	if textExtensions[ext] {
		return FileTypeElsewhere{FileExtension: ext, By: "the Editor", Why: "dinah"}, true
	}
	if known, ok := mediaTypes[ext]; ok && known != declared {
		return FileTypeElsewhere{FileExtension: ext, By: known, Why: "type"}, true
	}
	c, ok := extensionClaimOf(name, mediaType)
	if !ok || (c.Extension == id && c.View == view) {
		return FileTypeElsewhere{}, false
	}
	return FileTypeElsewhere{FileExtension: ext, By: c.Name, Why: "earlier"}, true
}
