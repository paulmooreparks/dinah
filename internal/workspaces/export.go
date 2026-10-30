package workspaces

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"studio/internal/layout"
	"studio/internal/profile"
)

// A workspace export (docs/design/workspace-export.md) is a workspace's
// structure, so that the same workspace can be made on another computer:
// its name, its sources, its main folder, its pins and the folders inside
// its local folders. It holds no content, no IDs, no credentials, no chats
// and no approvals. Its format is a compatibility contract: fields are only
// ever added, and a reader ignores fields it does not know, or, for a
// source or pin, keeps them (extra.go).

// ExportFormat is the format this version writes and the newest it reads.
const ExportFormat = 1

// ExportExtension is an export file's extension, which the profile gives.
var ExportExtension = profile.Current().WorkspaceFileExtension

// The folders recorded inside a local folder stop at this depth and count.
const (
	ExportFolderDepth = 8
	ExportFolderLimit = 2000
)

// Export is a workspace export.
type Export struct {
	Format  int            `json:"format"`
	Name    string         `json:"name"`
	Sources []ExportSource `json:"sources"`
	Pins    []ExportPin    `json:"pins,omitempty"`
}

// ExportSource is a source in an export. Key tells the sources of one
// export apart and is what its pins refer to; it is not a source ID. A
// local folder has Local; a source of another kind has the fields that
// identify it in Extra, written beside the others.
type ExportSource struct {
	Key   string          `json:"key"`
	Kind  string          `json:"kind"`
	Name  string          `json:"name"`
	Main  bool            `json:"main,omitempty"`
	Local *ExportLocation `json:"local,omitempty"`
	// Extra holds the fields of a source of another kind (extra.go).
	Extra map[string]json.RawMessage `json:"-"`
}

// MarshalJSON writes the source with the fields Extra holds.
func (s ExportSource) MarshalJSON() ([]byte, error) {
	type plain ExportSource
	return marshalWithExtra(plain(s), s.Extra)
}

// UnmarshalJSON reads the source, keeping the fields it does not know in
// Extra.
func (s *ExportSource) UnmarshalJSON(data []byte) error {
	type plain ExportSource
	var p plain
	extra, err := unmarshalWithExtra(data, &p)
	if err != nil {
		return err
	}
	*s = ExportSource(p)
	s.Extra = extra
	return nil
}

// ExportLocation is where a local folder was, in three ways that the
// importing computer tries in turn (Locate), each separated by "/": from
// the export file's folder, from the person's home folder, and in full.
// Folders are the folders inside it, relative to it.
type ExportLocation struct {
	FromFile string   `json:"fromFile,omitempty"`
	FromHome string   `json:"fromHome,omitempty"`
	Absolute string   `json:"absolute,omitempty"`
	Folders  []string `json:"folders,omitempty"`
}

// ExportPin is a pin in an export: a resource of one of its sources, by the
// source's key and the path within it, or a pin of another kind, with the
// fields that identify it in Extra.
type ExportPin struct {
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	Source    string `json:"source,omitempty"`
	Path      string `json:"path,omitempty"`
	ItemKind  string `json:"itemKind,omitempty"`
	Directory bool   `json:"directory,omitempty"`
	// Extra holds the fields of a pin of another kind (extra.go).
	Extra map[string]json.RawMessage `json:"-"`
}

// MarshalJSON writes the pin with the fields Extra holds.
func (p ExportPin) MarshalJSON() ([]byte, error) {
	type plain ExportPin
	return marshalWithExtra(plain(p), p.Extra)
}

// UnmarshalJSON reads the pin, keeping the fields it does not know in Extra.
func (p *ExportPin) UnmarshalJSON(data []byte) error {
	type plain ExportPin
	var q plain
	extra, err := unmarshalWithExtra(data, &q)
	if err != nil {
		return err
	}
	*p = ExportPin(q)
	p.Extra = extra
	return nil
}

// BuildExport makes the export of a workspace, to be written to file, for
// a person whose home folder is home. A local folder that is on the
// computer has its folders listed; one that is not keeps those its own
// import recorded.
func BuildExport(record Record, file, home string) Export {
	out := Export{Format: ExportFormat, Name: record.Name, Sources: []ExportSource{}}
	keys := map[string]string{}
	main := ExecutionPath(record)
	for i, source := range record.Sources {
		if !ValidKind(source.Kind) {
			continue
		}
		key := fmt.Sprintf("s%d", i+1)
		keys[source.ID] = key
		item := ExportSource{Kind: source.Kind}
		if source.Kind == LocalFolder {
			item.Main = main != "" && layout.SamePath(source.Path, main)
			location := Location(source.Path, file, home)
			if info, err := os.Stat(source.Path); err == nil && info.IsDir() {
				location.Folders = ListFolders(source.Path)
			} else {
				location.Folders = append([]string{}, record.PlannedFolders[source.ID]...)
			}
			item.Local = &location
		} else if kind, ok := LookupSourceKind(source.Kind); ok && kind.Export != nil {
			item = kind.Export(source)
			item.Main, item.Local = false, nil
		} else {
			item.Extra = copyExtra(source.Extra)
		}
		item.Key, item.Kind, item.Name = key, source.Kind, source.Name
		out.Sources = append(out.Sources, item)
	}
	for _, pin := range record.Pins {
		switch pin.Kind {
		case "resource":
			if pin.Ref == nil || keys[pin.Ref.SourceID] == "" {
				continue
			}
			out.Pins = append(out.Pins, ExportPin{Kind: "resource", Title: pin.Title, Source: keys[pin.Ref.SourceID], Path: pin.Ref.Path, ItemKind: pin.ItemKind, Directory: pin.Directory})
		case "chat":
			// A chat is kept by the agent on the computer it ran on, so a
			// chat pin has nothing to open on another.
		default:
			if !ValidKind(pin.Kind) {
				continue
			}
			item := ExportPin{Kind: pin.Kind, Title: pin.Title, ItemKind: pin.ItemKind, Directory: pin.Directory, Extra: copyExtra(pin.Extra)}
			if pin.Ref != nil {
				if keys[pin.Ref.SourceID] == "" {
					continue
				}
				item.Source, item.Path = keys[pin.Ref.SourceID], pin.Ref.Path
			}
			out.Pins = append(out.Pins, item)
		}
	}
	return out
}

// Location records where a folder is: from the folder of the export file,
// when the folder and the file are both in the home folder, from the home
// folder when the folder is in it, and in full.
func Location(path, file, home string) ExportLocation {
	path = filepath.Clean(path)
	out := ExportLocation{Absolute: filepath.ToSlash(path)}
	if home == "" || !layout.Inside(home, path) {
		return out
	}
	if rel, err := filepath.Rel(home, path); err == nil {
		out.FromHome = filepath.ToSlash(rel)
	}
	if file != "" && layout.Inside(home, filepath.Dir(file)) {
		if rel, err := filepath.Rel(filepath.Dir(file), path); err == nil {
			out.FromFile = filepath.ToSlash(rel)
		}
	}
	return out
}

// Locate finds a recorded folder on this computer: the first of its three
// ways that is a folder here, trying the home folder first, then the
// export file's folder, then the full path. The export file's folder is
// trusted only when the folder it leads to has the recorded folder's
// name, since an export saved somewhere else, such as Downloads, would
// otherwise lead to the wrong folder. When none is a folder here, it
// proposes where the folder would be, from the home folder first, since a
// folder that OneDrive or a similar service brings arrives there, and
// found is false.
func Locate(location ExportLocation, file, home string) (path string, found bool) {
	var fromFile, fromHome, absolute string
	name := pathBase(location.Absolute)
	if name == "" {
		name = pathBase(location.FromHome)
	}
	if location.FromFile != "" && file != "" {
		fromFile = filepath.Join(filepath.Dir(file), filepath.FromSlash(location.FromFile))
		if !strings.EqualFold(filepath.Base(fromFile), name) {
			fromFile = ""
		}
	}
	if location.FromHome != "" && home != "" {
		fromHome = filepath.Join(home, filepath.FromSlash(location.FromHome))
	}
	if location.Absolute != "" && filepath.IsAbs(filepath.FromSlash(location.Absolute)) {
		absolute = filepath.Clean(filepath.FromSlash(location.Absolute))
	}
	for _, candidate := range []string{fromHome, fromFile, absolute} {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, true
		}
	}
	for _, candidate := range []string{fromHome, fromFile, absolute} {
		if candidate != "" {
			return candidate, false
		}
	}
	return "", false
}

// pathBase is the last part of a recorded path, separated by "/", which
// is also how a Windows path is recorded.
func pathBase(recorded string) string {
	recorded = strings.TrimRight(recorded, "/")
	return recorded[strings.LastIndex(recorded, "/")+1:]
}

// ListFolders lists the folders inside a folder, relative to it and
// separated by "/", to ExportFolderDepth levels and at most
// ExportFolderLimit of them. Hidden folders, symbolic links and files are
// left out, and a folder that cannot be read is passed over.
func ListFolders(root string) []string {
	var out []string
	filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if path == root {
			if err != nil {
				return fs.SkipAll
			}
			return nil
		}
		if err != nil {
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") {
			return fs.SkipDir
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return fs.SkipDir
		}
		rel = filepath.ToSlash(rel)
		if strings.Count(rel, "/")+1 > ExportFolderDepth {
			return fs.SkipDir
		}
		if len(out) >= ExportFolderLimit {
			return fs.SkipAll
		}
		out = append(out, rel)
		return nil
	})
	sort.Strings(out)
	return out
}

// CleanFolder checks a recorded folder: relative, separated by "/", with
// no empty, "." or ".." part, nothing a path could escape through, and no
// deeper than ExportFolderDepth.
func CleanFolder(folder string) bool {
	if folder == "" || strings.ContainsAny(folder, "\\:\x00") || strings.HasPrefix(folder, "/") {
		return false
	}
	parts := strings.Split(folder, "/")
	if len(parts) > ExportFolderDepth {
		return false
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.TrimSpace(part) != part {
			return false
		}
	}
	return filepath.IsLocal(filepath.FromSlash(folder))
}

// ErrNotAnExport is returned for a file that is not a workspace export.
var ErrNotAnExport = errors.New("this file is not a workspace export")

// ReadExport reads an export and checks it: a format this version knows, a
// kind of plain form for each source, local folders with a location that
// stays inside their folder, sources of registered kinds that their kind
// accepts, keys that are unique and at most one main folder, which must be
// a local folder. Pins that refer to no source of the export, chat pins
// and pins their kind refuses are dropped. A source or pin of a kind this
// build does not know is kept as it is.
func ReadExport(data []byte) (Export, error) {
	var out Export
	if err := json.Unmarshal(data, &out); err != nil || out.Format == 0 {
		return Export{}, ErrNotAnExport
	}
	if out.Format > ExportFormat {
		name := profile.Current().Name
		return Export{}, fmt.Errorf("this workspace export was made by a newer version; update %s to import it", name)
	}
	if out.Format < 0 {
		return Export{}, ErrNotAnExport
	}
	out.Name = strings.TrimSpace(out.Name)
	if out.Name == "" {
		return Export{}, errors.New("the workspace export has no workspace name")
	}
	keys, main := map[string]bool{}, false
	for i := range out.Sources {
		source := &out.Sources[i]
		if source.Key == "" || keys[source.Key] {
			return Export{}, errors.New("the workspace export has two sources with one key")
		}
		keys[source.Key] = true
		source.Name = strings.TrimSpace(source.Name)
		if !ValidKind(source.Kind) {
			return Export{}, fmt.Errorf("the source %q in the workspace export has no kind that can be read", source.Name)
		}
		if source.Kind != LocalFolder {
			if source.Main || source.Local != nil {
				return Export{}, fmt.Errorf("the source %q in the workspace export is not a local folder but is recorded as one", source.Name)
			}
			if kind, ok := LookupSourceKind(source.Kind); ok && kind.Read != nil {
				cleaned, err := kind.Read(*source)
				if err != nil {
					return Export{}, err
				}
				cleaned.Key, cleaned.Kind, cleaned.Main, cleaned.Local = source.Key, source.Kind, false, nil
				*source = cleaned
			}
			continue
		}
		if source.Local == nil || (source.Local.Absolute == "" && source.Local.FromHome == "" && source.Local.FromFile == "") {
			return Export{}, fmt.Errorf("the folder %q in the workspace export has no location", source.Name)
		}
		for _, rel := range []string{source.Local.FromHome, source.Local.FromFile} {
			if rel != "" && (strings.Contains(rel, "\\") || filepath.IsAbs(filepath.FromSlash(rel))) {
				return Export{}, fmt.Errorf("the folder %q in the workspace export has an invalid location", source.Name)
			}
		}
		if len(source.Local.Folders) > ExportFolderLimit {
			return Export{}, fmt.Errorf("the folder %q in the workspace export lists more than %d folders", source.Name, ExportFolderLimit)
		}
		for _, folder := range source.Local.Folders {
			if !CleanFolder(folder) {
				return Export{}, fmt.Errorf("the folder %q in the workspace export lists an invalid folder", source.Name)
			}
		}
		// The fields a local folder does not have are not kept.
		source.Extra = nil
		if source.Main {
			if main {
				return Export{}, errors.New("the workspace export has more than one main folder")
			}
			main = true
		}
	}
	pins := []ExportPin{}
	for _, pin := range out.Pins {
		switch {
		case pin.Kind == "resource":
			if keys[pin.Source] {
				pin.Extra = nil
				pins = append(pins, pin)
			}
		case pin.Kind == "chat" || !ValidKind(pin.Kind):
		case pin.Source != "" && !keys[pin.Source]:
		default:
			if kind, ok := LookupPinKind(pin.Kind); ok && kind.Read != nil && !kind.Read(pin) {
				continue
			}
			pins = append(pins, pin)
		}
	}
	out.Pins = pins
	return out, nil
}

// KnownPin reports whether this build knows a kind of pin: a chat, a
// resource or a registered kind.
func KnownPin(kind string) bool {
	if kind == "chat" || kind == "resource" {
		return true
	}
	_, ok := LookupPinKind(kind)
	return ok
}
