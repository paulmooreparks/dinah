package agents

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

const MaxDirectoryEntries = 4096

// ReadDirectoryChecked checks held object identities before returning metadata.
func ReadDirectoryChecked(ctx context.Context, path string, check func([]os.FileInfo) error) ([]fs.DirEntry, error) {
	resolved, err := ResolveFilePath(path)
	if err != nil {
		return nil, err
	}
	base := filepath.VolumeName(resolved) + string(filepath.Separator)
	rel, err := filepath.Rel(base, resolved)
	if err != nil || !filepath.IsLocal(rel) && rel != "." {
		return nil, errors.New("a local path is required")
	}
	file, err := openObjectChecked(base, rel, true, check)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []fs.DirEntry{fs.FileInfoToDirEntry(info)}, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := file.ReadDir(MaxDirectoryEntries + 1)
	if err != nil {
		return nil, err
	}
	if len(entries) > MaxDirectoryEntries {
		return nil, errors.New("the directory exceeds 4096 entries; choose a smaller directory")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}
