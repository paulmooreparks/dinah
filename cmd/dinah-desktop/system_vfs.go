package main

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"studio/internal/workspaces"
)

// The system folders: /bin holds Dinah's standard scripts and utilities and
// /lib the Lua modules they share. They are the folders bin and lib in
// Dinah's own folder, so they are changed like any folder: in the Terminal,
// in the Editor, or from outside Dinah, as by a sync client. Each
// release installs its standard files there and replaces the ones nobody has
// changed since the last install, and keeps the ones somebody has.

//go:embed all:system
var systemFiles embed.FS

// reservedRoots are the names the root folder keeps for itself, beside home
// and workspaces. bin and lib are served; the rest are kept for later.
var reservedRoots = []string{"bin", "lib", "etc", "usr", "var", "tmp"}

// servedRoots are the reserved roots that have something in them, with the
// source each is served from.
var servedRoots = map[string]string{"bin": "dinah-bin", "lib": "dinah-lib"}

// ReservedRoot reports whether a name is one the root folder keeps for itself.
func ReservedRoot(name string) bool {
	for _, r := range reservedRoots {
		if strings.EqualFold(name, r) {
			return true
		}
	}
	return name == "home" || name == "workspaces"
}

// systemRootOf returns the root folder a system source is served at, such as
// bin for dinah-bin.
func systemRootOf(sourceID string) (string, bool) {
	for root, id := range servedRoots {
		if id == sourceID {
			return root, true
		}
	}
	return "", false
}

// personalSource returns a source that belongs to no workspace: Home, /bin
// or /lib.
func personalSource(id string) (WorkspaceSource, error) {
	if id == workspaces.HomeSourceID {
		return dinahHomeSource()
	}
	if root, ok := systemRootOf(id); ok {
		return systemSource(root)
	}
	return WorkspaceSource{}, errors.New("the source is no longer attached")
}

// systemSource returns the local folder /bin or /lib is served from, with
// the standard files installed in it.
func systemSource(root string) (WorkspaceSource, error) {
	dir := configDir()
	if _, err := installSystemFiles(dir); err != nil {
		return WorkspaceSource{}, err
	}
	return WorkspaceSource{ID: servedRoots[root], Kind: "local-folder", Name: root, Path: filepath.Join(dir, root)}, nil
}

// installStandardFiles installs the standard files when Dinah starts,
// and tells the person under Notifications which ones were kept because
// they had been changed; the same list is told once.
func (a *App) installStandardFiles() {
	kept, err := installSystemFiles(configDir())
	if err != nil {
		logf("core", "error", "installing the standard files of /bin and /lib: %v", err)
		_, _ = a.SaveNotification(Notification{ID: "system-files-failed", Kind: "app", Severity: "warning", Title: "Standard Scripts Not Installed", Body: "The standard scripts and modules of /bin and /lib could not be installed: " + err.Error()})
		return
	}
	if len(kept) == 0 {
		logf("core", "info", "the standard files of /bin and /lib are installed")
		return
	}
	logf("core", "warning", "standard files kept because they were changed: %s", strings.Join(kept, ", "))
	id := systemHash([]byte(strings.Join(kept, "\n")))[:16]
	body := "These files in /bin and /lib were changed, so this release of " + product.Name + " did not replace them: " + strings.Join(kept, ", ") + ". Delete one to get this release's version of it."
	_, _ = a.SaveNotification(Notification{ID: "system-files-kept-" + id, Kind: "app", Severity: "info", Title: "Standard Scripts Kept", Body: body})
}

// systemInstalls records the Dinah folders the standard files have
// been installed in by this process, with the files kept there, so they are
// installed once each.
var systemInstalls = struct {
	sync.Mutex
	done map[string][]string
}{done: map[string][]string{}}

// systemManifest is system-files.json in Dinah's folder: the SHA-256 of
// each standard file as it was last installed, by its path under that folder,
// such as bin/seq. A file whose content still has that hash has not been
// changed since, so the next release may replace it.
type systemManifest struct {
	Files map[string]string `json:"files"`
}

func systemHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// installSystemFiles installs the standard files of this release in dir/bin
// and dir/lib. A file that is missing is written. A file that is as the last
// install left it is replaced by this release's. A file somebody changed is
// kept as it is, and so is a file somebody added. A standard file that this
// release no longer has is removed when nobody changed it. It returns the
// standard files it kept because they were changed.
func installSystemFiles(dir string) ([]string, error) {
	systemInstalls.Lock()
	defer systemInstalls.Unlock()
	if kept, ok := systemInstalls.done[dir]; ok {
		return kept, nil
	}
	manifestPath := filepath.Join(dir, "system-files.json")
	old := systemManifest{Files: map[string]string{}}
	if data, err := readStateFile(manifestPath); err == nil {
		if err := json.Unmarshal(data, &old); err != nil || old.Files == nil {
			old.Files = map[string]string{}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	next := systemManifest{Files: map[string]string{}}
	kept := []string{}
	err := fs.WalkDir(systemFiles, "system", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel := strings.TrimPrefix(p, "system/")
		data, err := systemFiles.ReadFile(p)
		if err != nil {
			return err
		}
		want := systemHash(data)
		target := filepath.Join(dir, filepath.FromSlash(rel))
		current, err := os.ReadFile(target)
		switch {
		case errors.Is(err, os.ErrNotExist):
		case err != nil:
			return err
		case systemHash(current) == want:
			next.Files[rel] = want
			return nil
		case old.Files[rel] != systemHash(current):
			// Somebody changed it: it stays, and still counts as changed.
			if old.Files[rel] != "" {
				next.Files[rel] = old.Files[rel]
			}
			kept = append(kept, rel)
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		if err := writeSystemFile(target, data); err != nil {
			return err
		}
		next.Files[rel] = want
		return nil
	})
	if err != nil {
		return nil, err
	}
	for rel, hash := range old.Files {
		if _, ok := next.Files[rel]; ok {
			continue
		}
		if _, err := fs.Stat(systemFiles, "system/"+rel); err == nil {
			continue
		}
		target := filepath.Join(dir, filepath.FromSlash(rel))
		if current, err := os.ReadFile(target); err == nil && systemHash(current) == hash {
			_ = os.Remove(target)
		}
	}
	for _, root := range []string{"bin", "lib"} {
		if err := os.MkdirAll(filepath.Join(dir, root), 0o700); err != nil {
			return nil, err
		}
	}
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := writeStateFile(manifestPath, data); err != nil {
		return nil, err
	}
	sort.Strings(kept)
	systemInstalls.done[dir] = kept
	return kept, nil
}

// writeSystemFile writes a standard file whole, through a file beside it that
// replaces it, so a script being run or read meanwhile is never half written.
func writeSystemFile(target string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(target), ".install-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), target)
}

// systemMountPath is the path a file of a system source has in the virtual
// file system, such as /bin/seq.
func systemMountPath(root, rel string) string {
	return path.Join("/"+root, rel)
}
