package workspaces

import (
	"fmt"
	"path/filepath"

	"studio/internal/layout"
)

// LocalRef returns the reference of a file in one of a workspace's local
// folders, given its path on disk, or an error when no folder of the
// workspace holds it.
func LocalRef(record Record, file string) (Ref, error) {
	abs, err := filepath.Abs(file)
	if err != nil {
		return Ref{}, err
	}
	for _, s := range record.Sources {
		if s.Kind != "local-folder" || !layout.Inside(s.Path, abs) {
			continue
		}
		rel, err := filepath.Rel(s.Path, abs)
		if err != nil || !filepath.IsLocal(rel) {
			continue
		}
		return Ref{WorkspaceID: record.ID, SourceID: s.ID, Path: filepath.ToSlash(rel)}, nil
	}
	return Ref{}, fmt.Errorf("%s is not in a folder of the workspace %s", abs, record.Name)
}
