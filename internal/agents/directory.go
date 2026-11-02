package agents

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const MaxDirectoryEntries = 4096

// ReadDirectoryChecked takes policy decisions from opened directory identities.
// A regular-file target is represented by its own single metadata entry.
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
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		f, err := openReadFileChecked(base, rel, check)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		info, err = f.Stat()
		if err != nil {
			return nil, err
		}
		return []fs.DirEntry{fs.FileInfoToDirEntry(info)}, nil
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		return nil, err
	}
	defer func() { root.Close() }()
	info, err = root.Stat(".")
	if err != nil {
		return nil, err
	}
	chain := []os.FileInfo{info}
	if rel != "." {
		for _, part := range strings.Split(rel, string(filepath.Separator)) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			before, err := root.Lstat(part)
			if err != nil {
				return nil, err
			}
			if !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
				return nil, errors.New("the directory redirects or changed")
			}
			next, err := root.OpenRoot(part)
			if err != nil {
				return nil, err
			}
			opened, err := next.Stat(".")
			if err != nil || !os.SameFile(before, opened) {
				next.Close()
				return nil, errors.New("the directory identity changed")
			}
			chain = append(chain, opened)
			root.Close()
			root = next
		}
	}
	if check != nil {
		if err := check(chain); err != nil {
			return nil, err
		}
	}
	file, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	defer file.Close()
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
