package agents

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"studio/internal/layout"
)

// NormalizeFilePath admits only local absolute host paths. Windows namespace
// aliases cannot create a second spelling that bypasses a drive-root policy.
func NormalizeFilePath(path string) (string, error) {
	if strings.ContainsRune(path, 0) {
		return "", errors.New("a file path must not contain NUL")
	}
	if runtime.GOOS == "windows" {
		path = strings.ReplaceAll(path, "/", `\`)
		if strings.HasPrefix(path, `\\?\`) {
			path = strings.TrimPrefix(path, `\\?\`)
		}
		if len(path) < 3 || !((path[0] >= 'A' && path[0] <= 'Z') || (path[0] >= 'a' && path[0] <= 'z')) || path[1] != ':' || path[2] != '\\' {
			return "", errors.New("agent file access requires an absolute drive-letter path; device and network paths are refused")
		}
		if strings.Contains(path[2:], ":") {
			return "", errors.New("alternate data streams are refused")
		}
	}
	if !filepath.IsAbs(path) {
		return "", errors.New("an absolute file path is required")
	}
	return filepath.Clean(path), nil
}

// ResolveFilePath resolves existing ancestors without inventing a filesystem
// identity for a missing target. The returned path is validated again.
func ResolveFilePath(path string) (string, error) {
	path, err := NormalizeFilePath(path)
	if err != nil {
		return "", err
	}
	var tail []string
	for {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return NormalizeFilePath(resolved)
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", err
		}
		tail = append(tail, filepath.Base(path))
		path = parent
	}
}

// IdentityContains compares a policy root with actual opened ancestors or file.
// A missing root cannot contain an existing object. Other errors fail closed.
func IdentityContains(root string, chain []os.FileInfo) (bool, error) {
	if root == "" {
		return false, nil
	}
	root, err := NormalizeFilePath(root)
	if err != nil {
		return false, err
	}
	info, err := os.Stat(root)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	for _, opened := range chain {
		if os.SameFile(info, opened) {
			return true, nil
		}
	}
	return false, nil
}

func protectedIdentity(l layout.Layout, chain []os.FileInfo, managedRoots ...string) error {
	for i, root := range ProtectedRoots(l) {
		found, err := IdentityContains(root.Path, chain)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		managed := false
		if i == 0 {
			for _, root := range managedRoots {
				if !l.Managed(root) {
					continue
				}
				resolved, err := filepath.EvalSymlinks(root)
				if err != nil || !layout.SamePath(root, resolved) {
					continue
				}
				found, err := IdentityContains(root, chain)
				if err != nil {
					return err
				}
				managed = managed || found
			}
		}
		if !managed {
			return errors.New(root.Reason)
		}
	}
	return nil
}

// pathIdentityChain is an early metadata check. Operation checks below also
// use the identities taken from the handles that actually perform the I/O.
func pathIdentityChain(path string) ([]os.FileInfo, error) {
	resolved, err := ResolveFilePath(path)
	if err != nil {
		return nil, err
	}
	var chain []os.FileInfo
	for {
		info, err := os.Stat(resolved)
		if err == nil {
			chain = append(chain, info)
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		parent := filepath.Dir(resolved)
		if parent == resolved {
			return chain, nil
		}
		resolved = parent
	}
}
