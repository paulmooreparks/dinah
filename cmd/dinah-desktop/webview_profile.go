package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"studio/internal/statefile"
)

/* The browser profiles. Until 14 November 2026 each release had profiles of
   its own, in folders with the release in their names, so that a page file
   kept by an earlier build was never shown. That cost a person the page's
   own stored data at every update, and the reason for it is gone: every
   file the windows are served is sent with Cache-Control: no-store, so the
   browser keeps none of them.

   A profile is now chosen once and recorded, and a later release uses the
   recorded one. The first choice adopts what is there, so nothing is copied
   and nothing is lost: the folder of the running release when it exists, or
   the only earlier folder of that kind when there is one. With several
   earlier folders and none for this release, nothing says which one the
   person used last, so a new folder is made and the earlier ones are left
   as they are. No profile is ever deleted or merged here. */

// The kinds of profile: the main window's, the document windows' and the
// show's. Each signs in with a cookie of its own, so they stay apart.
const (
	profileMain      = "main"
	profileDocuments = "documents"
	profileShow      = "show"
)

// profileSuffix is how a release's folder for a kind ended.
var profileSuffix = map[string]string{profileMain: "", profileDocuments: "-documents", profileShow: "-show"}

// profileRecord is the file that says which folder each kind uses.
const profileRecord = "profiles.json"

var profileMu sync.Mutex

// webviewProfile is the folder of the browser profile of a kind, under
// root, for a build of the release version. It returns "" when the folder
// cannot be made, and the browser then uses its default.
func webviewProfile(root, kind, version string) string {
	profileMu.Lock()
	defer profileMu.Unlock()
	if os.MkdirAll(root, 0o700) != nil {
		return ""
	}
	// The main window, a document's window and the show are processes of
	// their own, so the record is read and written under its lock.
	name := ""
	err := statefile.Update(filepath.Join(root, profileRecord), func(previous []byte) ([]byte, error) {
		chosen := map[string]string{}
		if len(previous) > 0 {
			_ = json.Unmarshal(previous, &chosen)
		}
		name = chosen[kind]
		// A recorded folder that is gone, or a name that is not a plain
		// folder name, is chosen again.
		if name != "" && name == filepath.Base(name) && isDir(filepath.Join(root, name)) {
			return previous, nil
		}
		name = adoptProfile(root, kind, version)
		chosen[kind] = name
		logf("core", "info", "the browser profile for %s is %s", kind, name)
		return json.MarshalIndent(chosen, "", "  ")
	})
	if err != nil {
		logf("core", "warn", "the browser profile chosen for %s could not be recorded: %v", kind, err)
		if name == "" {
			name = adoptProfile(root, kind, version)
		}
	}
	p := filepath.Join(root, name)
	if os.MkdirAll(p, 0o700) != nil {
		return ""
	}
	return p
}

// adoptProfile chooses the folder a kind uses from now on.
func adoptProfile(root, kind, version string) string {
	if own := version + profileSuffix[kind]; isDir(filepath.Join(root, own)) {
		return own
	}
	var earlier []string
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if e.IsDir() && profileKind(e.Name()) == kind {
			earlier = append(earlier, e.Name())
		}
	}
	sort.Strings(earlier)
	if len(earlier) == 1 {
		return earlier[0]
	}
	if len(earlier) > 1 {
		logf("core", "info", "%d earlier browser profiles for %s were left as they are, since nothing says which was used last", len(earlier), kind)
	}
	return "profile-" + kind
}

// profileKind is the kind a folder made for a release belongs to.
func profileKind(name string) string {
	switch {
	case strings.HasPrefix(name, "profile-"):
		return ""
	case strings.HasSuffix(name, profileSuffix[profileDocuments]):
		return profileDocuments
	case strings.HasSuffix(name, profileSuffix[profileShow]):
		return profileShow
	}
	return profileMain
}

func isDir(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

func webviewRoot() string { return filepath.Join(configDir(), "webview2") }
