package main

import (
	"encoding/base64"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"studio/internal/layout"
	"studio/internal/workspaces"
)

/* Where a document a chat refers to opens (docs/plans/document-links.md).
   A chat's message may point to a file or a folder by a Markdown link, a
   file: address, or a path written as text or as inline code. This file is
   the one place that decides which part of Dinah's window opens it, so the
   page, its tooltips and anything else that asks agree:

   - a folder in one of Dinah's workspace sources opens in Files;
   - a file of a media type another part of the window has claimed opens
     there, as a picture or an SVG opens in the media viewer;
   - a file the Editor opens (editorOpens, the rule Files' Edit uses) opens
     in the Editor, in the view Settings chooses for its media type;
   - a file in a workspace source of a kind an enabled extension's view
     declares, and that no part of Dinah opens itself, opens in that view,
     with the file in the view's state (extension_claims.go);
   - anything else is left to the system, as a link did before: a document
     of a known kind opens in the system's program for it, and any other
     file, or a folder outside the workspace sources, is shown in the
     system's file manager (OpenPath).

   A file or folder inside a workspace source is given as that workspace's
   resource, so the Editor opens its draft and Recent and pins work as they
   do from Files. */

// Where a link opens, as DocumentLink.Opens says it.
const (
	opensEditor = "editor"
	opensMedia  = "media"
	opensFiles  = "files"
	opensSystem = "system"
	opensReveal = "reveal"
	// opensExtension is an extension's view, which Extension and
	// ExtensionView identify and State is the navigation state for.
	opensExtension = "extension"
)

// DocumentLink is where a reference in a chat opens. Ref is the reference as
// it was asked about. Found says a file or folder is there; when it is not,
// nothing else but Path is set. Line is the line a suffix such as :42 or
// #L42 asked for, which the Editor or an extension's view goes to. Label and
// Glyph are the link's tooltip and its PUDL glyph. Resource is the workspace
// resource the path is, when it is in a workspace source, and MountPath is
// the folder's path in Files. Extension, ExtensionView and State are set
// when the file opens in an extension's view: the extension's ID, the
// view's ID and the view's navigation state, which is the JSON text of the
// SDK's OpenState.
type DocumentLink struct {
	Ref       string       `json:"ref"`
	Found     bool         `json:"found"`
	Path      string       `json:"path,omitempty"`
	Line      int          `json:"line,omitempty"`
	Directory bool         `json:"directory,omitempty"`
	MediaType string       `json:"mediaType,omitempty"`
	Opens     string       `json:"opens,omitempty"`
	View      string       `json:"view,omitempty"`
	Label     string       `json:"label,omitempty"`
	Glyph     string       `json:"glyph,omitempty"`
	Resource  *ResourceRef `json:"resource,omitempty"`
	MountPath string       `json:"mountPath,omitempty"`

	Extension     string `json:"extension,omitempty"`
	ExtensionView string `json:"extensionView,omitempty"`
	State         string `json:"state,omitempty"`
}

// mediaTypes are the media types of the kinds of file Dinah tells apart by
// their extension. A text file the Editor opens and this table does not
// list is text/plain, and any other file application/octet-stream.
var mediaTypes = map[string]string{
	".md": "text/markdown", ".markdown": "text/markdown",
	".csv": "text/csv", ".html": "text/html", ".css": "text/css",
	".json": "application/json", ".xml": "application/xml", ".yaml": "application/yaml", ".yml": "application/yaml",
	".png": "image/png", ".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".gif": "image/gif", ".webp": "image/webp",
	".svg": "image/svg+xml", ".bmp": "image/bmp",
	".pdf":  "application/pdf",
	".docx": "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
	".xlsx": "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
	".pptx": "application/vnd.openxmlformats-officedocument.presentationml.presentation",
}

// mediaTypeOf is the media type of a file by its name.
func mediaTypeOf(path string) string {
	ext := textExtension(path)
	if t, ok := mediaTypes[ext]; ok {
		return t
	}
	if textExtensions[ext] {
		return "text/plain"
	}
	return "application/octet-stream"
}

// mediaClaim is a part of the window, other than the Editor, that opens
// files of a media type. Opens is how the page opens it, Label and Glyph
// are what a link to such a file shows, and Limit is the largest file it
// takes; a larger one is left to the system. Name is the claimant as
// Settings › Extensions shows it when an extension's claim gives way to it.
//
// An extension's claim also has the extension's ID and title, the view's
// ID, the media type the view declared, the file extensions it listed for
// that type, and when the extension was enabled, which orders the claims of
// extensions that declare the same kind of file.
type mediaClaim struct {
	Opens string
	Label string
	Glyph string
	Limit int64
	Name  string

	Extension      string
	View           string
	MediaType      string
	FileExtensions []string
	Since          time.Time
}

// mediaClaims are the media types claimed. Dinah's own parts claim by type
// with claimMediaType, and a claim of theirs comes before the Editor, so a
// picture written as SVG text opens as a picture. The enabled extensions'
// claims are kept apart, in the order the extensions were enabled, and are
// replaced together by setExtensionClaims; extensionsLoaded says they have
// been read from the extension store since Dinah started.
var mediaClaims = struct {
	sync.RWMutex
	byType           map[string]mediaClaim
	extensions       []mediaClaim
	extensionsLoaded bool
}{byType: map[string]mediaClaim{}}

// claimMediaType gives files of a media type to a part of the window.
func claimMediaType(mediaType string, claim mediaClaim) {
	mediaClaims.Lock()
	defer mediaClaims.Unlock()
	mediaClaims.byType[mediaType] = claim
}

func claimOf(mediaType string) (mediaClaim, bool) {
	mediaClaims.RLock()
	defer mediaClaims.RUnlock()
	c, ok := mediaClaims.byType[mediaType]
	return c, ok
}

// The media viewer claims the pictures Dinah shows itself.
func init() {
	for _, ext := range imageExtensions {
		claimMediaType(mediaTypeOf("picture"+ext), mediaClaim{Opens: opensMedia, Label: "Open in the Media Viewer", Glyph: "image", Limit: workspaceAttachmentLimit, Name: "the Media Viewer"})
	}
}

// linkLooks are the words and glyph of each place a link opens, other than
// a claim, which brings its own.
var linkLooks = map[string][2]string{
	opensEditor: {"Open in the Editor", "document"},
	opensFiles:  {"Show in Files", "folder"},
	opensSystem: {"Open with the System's Program", "open"},
	opensReveal: {"Show in the System's File Manager", "open"},
}

// A path may ask for a line after it: path:42 or path:42:7, as compilers
// and agents write it, and path#L42 or path#L42-L50, as GitHub writes it.
var (
	lineColon    = regexp.MustCompile(`:(\d+)(?::\d+)?$`)
	lineFragment = regexp.MustCompile(`#L(\d+)(?:C\d+)?(?:-L?\d+(?:C\d+)?)?$`)
	driveOnly    = regexp.MustCompile(`^[A-Za-z]$`)
	slashDrive   = regexp.MustCompile(`^/[A-Za-z]:[\\/]`)
)

// referencePath turns a reference into a path and the line it asks for. A
// file: address is read as an address, with its fragment as #L42 is read.
// Any other reference is a path, absolute or relative to cwd, and a path
// written with a slash before its drive letter, as Markdown often writes
// one, loses the slash. A path with no drive that begins with a slash is
// read from cwd on Windows, where it has no other meaning. It returns ""
// when the reference cannot be a path.
func referencePath(cwd, ref string) (string, string, int) {
	ref = strings.TrimSpace(ref)
	if ref == "" || len(ref) > 4096 || strings.ContainsAny(ref, "\x00\n\r") {
		return "", "", 0
	}
	if len(ref) > 5 && strings.EqualFold(ref[:5], "file:") {
		u, err := url.Parse(ref)
		if err != nil || u.Path == "" {
			return "", "", 0
		}
		p := u.Path
		if u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
			p = "//" + u.Host + p
		}
		line := 0
		if m := lineFragment.FindStringSubmatch("#" + u.Fragment); m != nil {
			line, _ = strconv.Atoi(m[1])
		}
		if slashDrive.MatchString(p) {
			p = p[1:]
		}
		return filepath.Clean(filepath.FromSlash(p)), "", line
	}
	if slashDrive.MatchString(ref) {
		ref = ref[1:]
	}
	whole := absolutePath(cwd, ref)
	stripped, line := ref, 0
	if m := lineFragment.FindStringSubmatchIndex(ref); m != nil {
		stripped = ref[:m[0]]
		line, _ = strconv.Atoi(ref[m[2]:m[3]])
	} else if m := lineColon.FindStringSubmatchIndex(ref); m != nil && m[0] > 0 && !driveOnly.MatchString(ref[:m[0]]) {
		stripped = ref[:m[0]]
		line, _ = strconv.Atoi(ref[m[2]:m[3]])
	}
	if line == 0 {
		return whole, "", 0
	}
	// A file whose name really ends as a line does is that file.
	return absolutePath(cwd, stripped), whole, line
}

func absolutePath(cwd, p string) string {
	p = filepath.FromSlash(p)
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	if !filepath.IsAbs(cwd) {
		return ""
	}
	if os.PathSeparator == '\\' && strings.HasPrefix(p, `\`) && !strings.HasPrefix(p, `\\`) {
		p = strings.TrimLeft(p, `\`)
	}
	return filepath.Join(cwd, p)
}

// ResolveDocumentLinks says where each of a chat's references opens, for
// the links a message draws. cwd is the folder of the workspace that wrote
// the message, which a relative path is read from. It takes at most 200
// references at once.
func (a *App) ResolveDocumentLinks(cwd string, refs []string) ([]DocumentLink, error) {
	if len(refs) > 200 {
		return nil, errors.New("ask about at most 200 references at once")
	}
	out := make([]DocumentLink, len(refs))
	for i, ref := range refs {
		out[i] = a.resolveDocumentLink(cwd, ref)
	}
	return out, nil
}

// ResolveDocumentLink says where one reference opens, when its link is
// pressed.
func (a *App) ResolveDocumentLink(cwd, ref string) (DocumentLink, error) {
	return a.resolveDocumentLink(cwd, ref), nil
}

func (a *App) resolveDocumentLink(cwd, ref string) DocumentLink {
	out := DocumentLink{Ref: ref}
	path, whole, line := referencePath(cwd, ref)
	if whole != "" {
		if _, err := os.Stat(whole); err == nil {
			path, line = whole, 0
		}
	}
	out.Path = path
	if path == "" {
		return out
	}
	st, err := os.Stat(path)
	if err != nil {
		return out
	}
	out.Found, out.Line, out.Directory = true, line, st.IsDir()
	if ref, ok := a.workspaceResourceAt(cwd, path); ok {
		out.Resource = &ref
	}
	if st.IsDir() {
		out.Line = 0
		if out.Resource != nil {
			if mount, err := a.ResolveWorkspaceResourceMount(*out.Resource); err == nil && mount.Directory {
				out.Opens, out.MountPath = opensFiles, mount.Path
			}
		}
		if out.Opens == "" {
			out.Opens = opensReveal
		}
		return withLook(out)
	}
	out.MediaType = mediaTypeOf(path)
	if claim, ok := claimOf(out.MediaType); ok && st.Size() <= claim.Limit {
		out.Opens, out.Label, out.Glyph, out.Line = claim.Opens, claim.Label, claim.Glyph, 0
		return out
	}
	if editorOpens(path, st.Size()) {
		a.mu.Lock()
		views := documentViews(a.settings)
		a.mu.Unlock()
		out.Opens, out.View = opensEditor, views[out.MediaType]
		if out.View == "" {
			out.View = "text"
		}
		return withLook(out)
	}
	if l, ok := a.extensionLink(out, path); ok {
		return l
	}
	out.Line = 0
	if openableExt[strings.ToLower(filepath.Ext(path))] {
		out.Opens = opensSystem
	} else {
		out.Opens = opensReveal
	}
	return withLook(out)
}

func withLook(l DocumentLink) DocumentLink {
	look := linkLooks[l.Opens]
	l.Label, l.Glyph = look[0], look[1]
	return l
}

// workspaceResourceAt is the workspace resource at an absolute path: in the
// folders of the workspace whose folder is cwd first, then of the other
// workspaces, then of the home folder, /bin and /lib. A path Files does not
// show, such as a hidden one, is not a resource.
func (a *App) workspaceResourceAt(cwd, path string) (ResourceRef, bool) {
	a.mu.Lock()
	id := ""
	for _, r := range a.settings.WorkspaceRecords {
		if cwd != "" && layout.SamePath(workspaces.ExecutionPath(r), cwd) {
			id = r.ID
			break
		}
	}
	a.mu.Unlock()
	ref, ok := a.localDocumentRef(id, path)
	if !ok {
		return ResourceRef{}, false
	}
	source, err := a.resourceService().source(ref)
	if err != nil || source.Kind != "local-folder" {
		return ResourceRef{}, false
	}
	f, err := openWorkspaceResource(source, ref.Path)
	if err != nil {
		return ResourceRef{}, false
	}
	f.Close()
	return ref, true
}

// ReadLinkedImage is the picture a chat's reference points to, as a data:
// address the media viewer shows. Only a reference that opens in the media
// viewer is read, and only a picture of a kind Dinah shows.
func (a *App) ReadLinkedImage(cwd, ref string) (string, error) {
	l := a.resolveDocumentLink(cwd, ref)
	if !l.Found || l.Opens != opensMedia {
		return "", errors.New("that reference is not a picture " + product.Name + " shows")
	}
	f, err := os.Open(l.Path)
	if err != nil {
		return "", errors.New(filepath.Base(l.Path) + " could not be read")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, workspaceAttachmentLimit+1))
	if err != nil || len(data) > workspaceAttachmentLimit {
		return "", errors.New(filepath.Base(l.Path) + " could not be read, or is larger than 20 MiB")
	}
	mime, err := attachmentImageType(data)
	if err != nil {
		return "", errors.New(filepath.Base(l.Path) + " is not a picture " + product.Name + " shows")
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}
