// Package localfs is the resource provider for a local folder attached to a
// workspace. Every access is confined to the folder with os.Root, refuses
// symbolic links and hidden or system entries along the path, and refuses
// paths that would leave the folder.
package localfs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"studio/internal/resource"
)

// MaxText bounds a text read or write.
const MaxText = 1 << 20

// Folder is a local folder source.
type Folder struct {
	Root string
	// Dotfiles makes names starting with a dot ordinary names in this
	// folder. It is for a person's own home folder only; in a workspace's
	// folders such names, .devin and .git among them, stay out of reach.
	Dotfiles bool
}

var (
	_ resource.TextReader      = Folder{}
	_ resource.TextWriter      = Folder{}
	_ resource.Snapshotter     = Folder{}
	_ resource.DocumentWriter  = Folder{}
	_ resource.Lister          = Folder{}
	_ resource.DocumentCreator = Folder{}
	_ resource.FileSource      = Folder{}
)

// WriteDocument saves an edited file, provided it is still at base's
// revision. A file's title is its name, which saving does not change.
func (f Folder) WriteDocument(ctx context.Context, path string, base resource.Snapshot, edit resource.Edit) (resource.Written, error) {
	rev, err := f.WriteText(ctx, path, base.Revision, edit.Text)
	if err != nil {
		return resource.Written{}, err
	}
	// The title is recorded as given, so that a draft whose title was edited
	// does not stay unsaved; the note says the file itself keeps its name.
	out := resource.Written{Snapshot: resource.Snapshot{Text: edit.Text, Revision: rev, Format: Format(path), Title: base.Title}}
	if edit.Title != "" && edit.Title != base.Title {
		out.Snapshot.Title = edit.Title
		out.Notes = append(out.Notes, "A file's title is its name, so the file keeps its name; rename the file to change it.")
	}
	out.Notes = append(out.Notes, f.createFiles(path, edit.Files)...)
	return out, nil
}

// createFiles writes files beside the document at path, and returns a note
// for each that could not be written, such as one whose name is taken.
func (f Folder) createFiles(path string, files []resource.File) []string {
	var notes []string
	for _, file := range files {
		if err := f.createFile(pathpkg.Join(pathpkg.Dir(path), file.Name), file.Data); err != nil {
			notes = append(notes, "The file "+file.Name+" was not written beside the document: "+err.Error())
		}
	}
	return notes
}

// CreateDocument writes a new file, refusing with resource.ErrConflict when
// one exists. A file's title is its name, as Snapshot gives it, so the
// edit's title is not written. The edit's files are written beside it; one
// whose name is taken there is not replaced, and a note says so.
func (f Folder) CreateDocument(ctx context.Context, path string, edit resource.Edit) (resource.Written, error) {
	rev, err := f.WriteText(ctx, path, "", edit.Text)
	if err != nil {
		return resource.Written{}, err
	}
	out := resource.Written{Snapshot: resource.Snapshot{Text: edit.Text, Revision: rev, Format: Format(path), Title: filepath.Base(filepath.FromSlash(path))}}
	out.Notes = f.createFiles(path, edit.Files)
	return out, nil
}

// fileFolders are where a document's files are looked for, relative to its
// folder: beside it, then in attachments and images, as the preview looks.
var fileFolders = []string{"", "attachments", "images"}

// FilePlaces are the paths, in the source, where a file the document at
// docPath shows may be. A bare name is looked for beside the document, then
// in its attachments and images folders. A relative path, such as the
// src/static/logo.png a README shows, is first the file at that path from
// the document's folder (23 October 2026), and then its name in those
// places, as all images were looked for before. Nothing is offered for an
// empty or absolute name, or one with a backslash, a colon or a line break;
// the caller's cleaning keeps the path within the source.
func FilePlaces(docPath, name string) []string {
	if name == "" || strings.HasPrefix(name, "/") || strings.ContainsAny(name, "\\:\x00\r\n") {
		return nil
	}
	dir := pathpkg.Dir(docPath)
	var out []string
	if strings.Contains(name, "/") {
		out = append(out, pathpkg.Join(dir, name))
		name = pathpkg.Base(name)
		if name == "." || name == ".." || name == "/" {
			return out
		}
	}
	for _, sub := range fileFolders {
		out = append(out, pathpkg.Join(dir, sub, name))
	}
	return out
}

// FileNames lists the files beside the document at path and in its
// attachments and images folders, by name; a name in more than one is
// listed once.
func (f Folder) FileNames(ctx context.Context, path string) ([]string, error) {
	var names []string
	for _, sub := range fileFolders {
		entries, err := f.List(ctx, pathpkg.Join(pathpkg.Dir(path), sub))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.Folder && e.Name != pathpkg.Base(path) && !slices.Contains(names, e.Name) {
				names = append(names, e.Name)
			}
		}
	}
	return names, nil
}

// ReadFile reads a file of the document at path, by its name or its path
// from the document's folder, from the first of its places that has it
// (FilePlaces), up to resource.MaxFile.
func (f Folder) ReadFile(ctx context.Context, path, name string) ([]byte, error) {
	places := FilePlaces(path, name)
	if len(places) == 0 {
		return nil, resource.ErrNotFound
	}
	root, err := os.OpenRoot(f.Root)
	if err != nil {
		return nil, errors.New("the local source is unavailable")
	}
	defer root.Close()
	for _, place := range places {
		clean, err := f.clean(place)
		if err != nil {
			continue
		}
		info, err := f.checkAncestors(root, clean)
		if err != nil || info == nil || !info.Mode().IsRegular() {
			continue
		}
		file, err := root.Open(clean)
		if err != nil {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(file, resource.MaxFile+1))
		file.Close()
		if err != nil {
			return nil, errors.New("the file could not be read")
		}
		if len(data) > resource.MaxFile {
			return nil, fmt.Errorf("the file is larger than %d MiB", resource.MaxFile>>20)
		}
		return data, nil
	}
	return nil, resource.ErrNotFound
}

// createFile writes a new file of any content, refusing one that exists.
func (f Folder) createFile(path string, data []byte) error {
	if strings.ContainsAny(pathpkg.Base(path), "/\\") || len(data) > resource.MaxFile {
		return errors.New("the file's name or size is not allowed")
	}
	name, err := f.clean(path)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(f.Root)
	if err != nil {
		return errors.New("the local source is unavailable")
	}
	defer root.Close()
	info, err := f.checkAncestors(root, name)
	if err != nil {
		return err
	}
	if info != nil {
		return errors.New("a file of that name is already there")
	}
	file, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return errors.New("a file of that name is already there, or the folder cannot be written")
	}
	_, werr := file.Write(data)
	cerr := file.Close()
	if err := errors.Join(werr, cerr); err != nil {
		root.Remove(name)
		return errors.New("the file could not be written")
	}
	return nil
}

// windowsReserved are the names Windows keeps for devices, with or without an
// extension (https://learn.microsoft.com/windows/win32/fileio/naming-a-file).
var windowsReserved = map[string]bool{"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true}

// NameProblem says why name cannot be a file's or folder's name on any of
// the systems Dinah runs on, or "" when it can.
func NameProblem(name string) string {
	switch {
	case strings.TrimSpace(name) == "":
		return "give it a name"
	case strings.TrimSpace(name) != name:
		return "a name cannot start or end with a space"
	case strings.ContainsAny(name, `/\:*?"<>|`) || strings.ContainsFunc(name, func(r rune) bool { return r < 32 }):
		return `a name cannot contain / \ : * ? " < > | or control characters`
	case strings.HasSuffix(name, "."):
		return "a name cannot end with a dot"
	case windowsReserved[strings.ToUpper(strings.SplitN(name, ".", 2)[0])]:
		return name + " is a name Windows keeps for itself"
	}
	return ""
}

// CreateFolder makes a new folder at path, whose parent must exist. It
// refuses a name that is taken, by a file or a folder.
func (f Folder) CreateFolder(path string) error {
	if p := NameProblem(pathpkg.Base(path)); p != "" {
		return errors.New(p)
	}
	name, err := f.clean(path)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(f.Root)
	if err != nil {
		return errors.New("the local source is unavailable")
	}
	defer root.Close()
	info, err := f.checkAncestors(root, name)
	if err != nil {
		return err
	}
	if info != nil {
		return resource.ErrConflict
	}
	if err := root.Mkdir(name, 0o755); err != nil {
		if errors.Is(err, os.ErrExist) {
			return resource.ErrConflict
		}
		return errors.New("the folder could not be made here")
	}
	return nil
}

// Snapshot reads a text file for editing; its title is the file's name.
func (f Folder) Snapshot(ctx context.Context, path string) (resource.Snapshot, error) {
	t, err := f.ReadText(ctx, path)
	if err != nil {
		return resource.Snapshot{}, err
	}
	return resource.Snapshot{Text: t.Text, Revision: t.Revision, Format: t.Format, Title: filepath.Base(filepath.FromSlash(path))}, nil
}

// CleanPath checks a source-relative path and returns it in the platform's
// form, or "." for the source's root. Paths use forward slashes; absolute
// paths, drive letters, backslashes, empty segments, segments with leading or
// trailing spaces, and segments starting with a dot (hidden entries and parent
// traversal) are refused.
func CleanPath(raw string) (string, error) { return CleanPathFor(raw, false) }

// CleanPathFor is CleanPath for a folder where, with dotfiles, a name
// starting with a dot is an ordinary name, as in a person's own home folder;
// "." and ".." are refused there too.
func CleanPathFor(raw string, dotfiles bool) (string, error) {
	if raw == "" || raw == "." {
		return ".", nil
	}
	if filepath.IsAbs(raw) || strings.ContainsAny(raw, ":\\\x00") {
		return "", errors.New("use a workspace-relative path with forward slashes")
	}
	for _, part := range strings.Split(raw, "/") {
		hidden := strings.HasPrefix(part, ".") && (!dotfiles || part == "." || part == "..")
		if part == "" || hidden || strings.TrimSpace(part) != part {
			return "", errors.New("hidden paths and parent traversal are unavailable")
		}
	}
	return filepath.FromSlash(raw), nil
}

// HiddenName reports whether a folder entry's name keeps it out of view: a
// name starting with a dot, unless the folder takes dotfiles.
func HiddenName(name string, dotfiles bool) bool {
	return strings.HasPrefix(name, ".") && (!dotfiles || name == "." || name == "..")
}

func (f Folder) clean(raw string) (string, error) { return CleanPathFor(raw, f.Dotfiles) }

// Format reports how a file's text is to be shown, from its extension.
func Format(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown":
		return "markdown"
	}
	return "text"
}

// checkAncestors refuses the path when any existing segment of it is a
// symbolic link or a hidden or system entry. It returns the information of
// the final segment, or nil when it does not exist.
func (f Folder) checkAncestors(root *os.Root, name string) (os.FileInfo, error) {
	if name == "." {
		return nil, nil
	}
	part := ""
	segments := strings.Split(name, string(filepath.Separator))
	for i, segment := range segments {
		part = filepath.Join(part, segment)
		info, err := root.Lstat(part)
		if errors.Is(err, os.ErrNotExist) && i == len(segments)-1 {
			return nil, nil
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("the source path is unavailable or is a symbolic link")
		}
		hidden, err := PathHidden(filepath.Join(f.Root, part))
		if err != nil || hidden {
			return nil, errors.New("hidden and system paths are unavailable")
		}
		if i == len(segments)-1 {
			return info, nil
		}
	}
	return nil, nil
}

// readCurrent reads the file at name within root and returns its text, or
// ok false when it does not exist.
func readCurrent(root *os.Root, name string, info os.FileInfo) (text string, ok bool, err error) {
	if info == nil {
		return "", false, nil
	}
	if !info.Mode().IsRegular() {
		return "", false, errors.New("the resource is not a regular file")
	}
	file, err := root.Open(name)
	if err != nil {
		return "", false, errors.New("the source path could not be opened")
	}
	defer file.Close()
	st, err := file.Stat()
	if err != nil || !os.SameFile(info, st) {
		return "", false, errors.New("the source path changed while it was being read")
	}
	if hidden, err := FileHidden(file); err != nil || hidden {
		return "", false, errors.New("hidden and system paths are unavailable")
	}
	data, err := io.ReadAll(io.LimitReader(file, MaxText+1))
	if err != nil {
		return "", false, errors.New("the file could not be read")
	}
	if len(data) > MaxText || !utf8.Valid(data) || strings.ContainsRune(string(data), '\x00') {
		return "", false, fmt.Errorf("the file is not UTF-8 text within %d bytes", MaxText)
	}
	return string(data), true, nil
}

// ReadText reads a text file.
func (f Folder) ReadText(ctx context.Context, path string) (resource.Text, error) {
	if err := ctx.Err(); err != nil {
		return resource.Text{}, err
	}
	name, err := f.clean(path)
	if err != nil {
		return resource.Text{}, err
	}
	root, err := os.OpenRoot(f.Root)
	if err != nil {
		return resource.Text{}, errors.New("the local source is unavailable")
	}
	defer root.Close()
	info, err := f.checkAncestors(root, name)
	if err != nil {
		return resource.Text{}, err
	}
	text, ok, err := readCurrent(root, name, info)
	if err != nil {
		return resource.Text{}, err
	}
	if !ok {
		return resource.Text{}, resource.ErrNotFound
	}
	return resource.Text{Text: text, Revision: resource.ContentRevision(text), Format: Format(path)}, nil
}

// MaxEntries bounds a folder listing.
const MaxEntries = 10000

// List lists a folder's entries in name order. Symbolic links and hidden or
// system entries are left out, as they cannot be read either.
func (f Folder) List(ctx context.Context, path string) ([]resource.Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	name, err := f.clean(path)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(f.Root)
	if err != nil {
		return nil, errors.New("the local source is unavailable")
	}
	defer root.Close()
	info, err := f.checkAncestors(root, name)
	if err != nil {
		return nil, err
	}
	if name != "." && info == nil {
		return nil, resource.ErrNotFound
	}
	if info != nil && !info.IsDir() {
		return nil, errors.New("the resource is not a folder")
	}
	dir, err := root.Open(name)
	if err != nil {
		return nil, errors.New("the folder could not be opened")
	}
	defer dir.Close()
	entries, err := dir.ReadDir(MaxEntries + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, errors.New("the folder could not be read")
	}
	if len(entries) > MaxEntries {
		return nil, fmt.Errorf("the folder has more than %d entries", MaxEntries)
	}
	out := []resource.Entry{}
	for _, e := range entries {
		if HiddenName(e.Name(), f.Dotfiles) || e.Type()&os.ModeSymlink != 0 || (!e.IsDir() && !e.Type().IsRegular()) {
			continue
		}
		if hidden, err := PathHidden(filepath.Join(f.Root, name, e.Name())); err != nil || hidden {
			continue
		}
		entry := resource.Entry{Name: e.Name(), Folder: e.IsDir()}
		if !e.IsDir() {
			st, err := e.Info()
			if err != nil {
				continue
			}
			entry.Size = st.Size()
		}
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// WriteText replaces or creates a text file, as resource.TextWriter
// describes. The new content is written to a temporary file beside the
// target and renamed over it, so the file holds the old text or the new,
// never part of either. The parent folder must exist. The current revision is
// checked just before the rename; a program outside Dinah that writes
// the file in that instant is not detected, which no portable file API can
// prevent.
func (f Folder) WriteText(ctx context.Context, path, expected, text string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if len(text) > MaxText || !utf8.ValidString(text) || strings.ContainsRune(text, '\x00') {
		return "", fmt.Errorf("the text must be UTF-8 within %d bytes and contain no NUL characters", MaxText)
	}
	name, err := f.clean(path)
	if err != nil {
		return "", err
	}
	if name == "." {
		return "", errors.New("choose a file to write")
	}
	root, err := os.OpenRoot(f.Root)
	if err != nil {
		return "", errors.New("the local source is unavailable")
	}
	defer root.Close()
	check := func() (os.FileInfo, error) {
		info, err := f.checkAncestors(root, name)
		if err != nil {
			return nil, err
		}
		current, exists, err := readCurrent(root, name, info)
		if err != nil {
			return nil, err
		}
		switch {
		case expected == "" && exists:
			return nil, resource.ErrConflict
		case expected != "" && !exists:
			return nil, resource.ErrConflict
		case exists && resource.ContentRevision(current) != expected:
			return nil, resource.ErrConflict
		}
		return info, nil
	}
	info, err := check()
	if err != nil {
		return "", err
	}
	if parent := filepath.Dir(name); parent != "." {
		st, err := root.Stat(parent)
		if err != nil || !st.IsDir() {
			return "", errors.New("the folder for the file does not exist")
		}
	}
	tmp, err := tempName(root, name)
	if err != nil {
		return "", err
	}
	if err := writeFile(root, tmp, text, info); err != nil {
		root.Remove(tmp)
		return "", err
	}
	if _, err := check(); err != nil {
		root.Remove(tmp)
		return "", err
	}
	if err := root.Rename(tmp, name); err != nil {
		root.Remove(tmp)
		return "", fmt.Errorf("the file could not be replaced: %w", err)
	}
	return resource.ContentRevision(text), nil
}

// tempName chooses a file beside name that does not exist. Its name starts
// with a dot, so browsing never lists it while the write is in progress.
func tempName(root *os.Root, name string) (string, error) {
	dir, base := filepath.Split(name)
	for i := 0; i < 100; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf(".%s.dinah-%d.tmp", base, i))
		if _, err := root.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		}
	}
	return "", errors.New("no temporary file name was available beside the file")
}

// writeFile writes text to a new file, keeping the permissions of the file it
// will replace, and syncs it before the rename.
func writeFile(root *os.Root, name, text string, replaces os.FileInfo) error {
	mode := os.FileMode(0o644)
	if replaces != nil {
		mode = replaces.Mode().Perm()
	}
	file, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("the file could not be written: %w", err)
	}
	_, werr := io.WriteString(file, text)
	serr := file.Sync()
	cerr := file.Close()
	if err := errors.Join(werr, serr, cerr); err != nil {
		return fmt.Errorf("the file could not be written: %w", err)
	}
	return nil
}
