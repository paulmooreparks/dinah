package agents

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"studio/internal/layout"
)

// MaxProviderText bounds returned text before and after UTF-8 replacement.
const MaxProviderText = 1 << 20
const UTF8ReplacementNote = "[Invalid UTF-8 sequences were replaced with U+FFFD.]\n"

var errProviderTextSize = errors.New("the requested text exceeds the 1 MiB response limit; use line and limit to request a smaller section")

type ProtectedReadError struct{ Path, Reason string }

func (e *ProtectedReadError) Error() string { return "this file is protected: " + e.Reason }

// ReadTextFile preserves ordinary filesystem access while withholding protected
// settings. It returns only a bounded selected range, never a truncated result.
func ReadTextFile(ctx context.Context, l layout.Layout, path string, managedRoots []string, line, limit *int) (string, error) {
	return ReadTextFileChecked(ctx, l, path, managedRoots, line, limit, nil)
}

// ReadTextFileChecked evaluates additional policy on the final canonical path
// before the identity-safe open. It never returns content from an unchecked path.
func ReadTextFileChecked(ctx context.Context, l layout.Layout, path string, managedRoots []string, line, limit *int, check func(string) error, identityChecks ...func([]os.FileInfo) error) (string, error) {
	if err := ValidateLineRange(line, limit); err != nil {
		return "", err
	}
	path, err := NormalizeFilePath(path)
	if err != nil {
		return "", err
	}
	if reason := ProtectedPath(l, path, managedRoots...); reason != "" {
		return "", &ProtectedReadError{Path: path, Reason: reason}
	}
	resolved, err := ResolveFilePath(path)
	if err != nil {
		return "", err
	}
	if reason := ProtectedPath(l, resolved, managedRoots...); reason != "" {
		return "", &ProtectedReadError{Path: resolved, Reason: reason}
	}
	if check != nil {
		if err := check(resolved); err != nil {
			return "", err
		}
	}
	base := filepath.VolumeName(resolved) + string(filepath.Separator)
	rel, err := filepath.Rel(base, resolved)
	if err != nil || !filepath.IsLocal(rel) || rel == "." {
		return "", errors.New("a regular file path is required")
	}
	file, err := openReadFileChecked(base, rel, func(chain []os.FileInfo) error {
		if err := protectedIdentity(l, chain, managedRoots...); err != nil {
			return &ProtectedReadError{Path: resolved, Reason: err.Error()}
		}
		for _, check := range identityChecks {
			if err := check(chain); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	defer file.Close()
	return readTextRange(ctx, file, line, limit)
}

// OpenCatalogFile rejects symbolic-link aliases, including ancestors, for a
// purpose-specific catalogue read. It grants no general provider file authority.
func OpenCatalogFile(path string) (*os.File, error) {
	path, err := NormalizeFilePath(path)
	if err != nil {
		return nil, err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	if !layout.SamePath(resolved, filepath.Clean(path)) {
		return nil, errors.New("catalogue entries must not redirect through symbolic links")
	}
	base := filepath.VolumeName(resolved) + string(filepath.Separator)
	rel, err := filepath.Rel(base, resolved)
	if err != nil || !filepath.IsLocal(rel) || rel == "." {
		return nil, errors.New("a regular catalogue file is required")
	}
	return openReadFile(base, rel)
}

// Open every directory through its parent's handle and compare identities.
// A concurrently replaced directory or final file cannot redirect this read.
func openReadFile(base, rel string) (*os.File, error) {
	return openReadFileChecked(base, rel, nil)
}

func openReadFileChecked(base, rel string, check func([]os.FileInfo) error) (*os.File, error) {
	return openObjectChecked(base, rel, false, check)
}

func openObjectChecked(base, rel string, directories bool, check func([]os.FileInfo) error) (*os.File, error) {
	root, err := os.OpenRoot(base)
	if err != nil {
		return nil, err
	}
	defer func() { root.Close() }()
	info, err := rootIdentitySnapshot(root)
	if err != nil {
		return nil, err
	}
	chain := []os.FileInfo{info}
	parts := strings.Split(rel, string(filepath.Separator))
	for _, part := range parts[:len(parts)-1] {
		before, err := root.Lstat(part)
		if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("the file directory changed or redirects through a symbolic link")
		}
		next, err := root.OpenRoot(part)
		if err != nil {
			return nil, err
		}
		opened, err := next.Stat(".")
		if err != nil || !os.SameFile(before, opened) {
			next.Close()
			return nil, errors.New("the file directory changed")
		}
		captured, err := rootIdentitySnapshot(next)
		if err != nil {
			next.Close()
			return nil, err
		}
		chain = append(chain, captured)
		root.Close()
		root = next
	}
	name := parts[len(parts)-1]
	before, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() && !(directories && before.IsDir()) {
		return nil, errors.New("only regular text files can be read")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil || (!opened.Mode().IsRegular() && !(directories && opened.IsDir())) || !os.SameFile(before, opened) {
		file.Close()
		return nil, errors.New("the file changed while opening it")
	}
	captured, err := snapshotOpenedIdentity(file, opened)
	if err != nil {
		file.Close()
		return nil, err
	}
	chain = append(chain, captured)
	if check != nil {
		if err := check(chain); err != nil {
			file.Close()
			return nil, err
		}
	}
	return file, nil
}

func readTextRange(ctx context.Context, file io.Reader, line, limit *int) (string, error) {
	if err := ValidateLineRange(line, limit); err != nil {
		return "", err
	}
	start := 1
	if line != nil {
		start = *line
	}
	reader := bufio.NewReaderSize(file, 32<<10)
	var data []byte
	current, selected := 1, 0
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		fragment, err := reader.ReadSlice('\n')
		if current >= start {
			if len(data)+len(fragment) > MaxProviderText {
				return "", errProviderTextSize
			}
			data = append(data, fragment...)
		}
		if err == bufio.ErrBufferFull {
			continue
		}
		if err != nil && err != io.EOF {
			return "", err
		}
		if current >= start {
			selected++
			if limit != nil && selected >= *limit {
				break
			}
		}
		if err == io.EOF {
			if current < start {
				return "", errors.New("line is past the end of the file")
			}
			break
		}
		current++
	}
	content := string(data)
	if !utf8.Valid(data) {
		content = UTF8ReplacementNote + strings.ToValidUTF8(content, "\uFFFD")
	}
	if len(content) > MaxProviderText {
		return "", errProviderTextSize
	}
	return content, nil
}

// CheckOpenedFile verifies a source handle against a second identity-safe walk.
// No contents are read by this function.
func CheckOpenedFile(path string, file *os.File, check func([]os.FileInfo) error) error {
	return checkOpenedPath(path, file, false, check)
}

// CheckOpenedPath also permits directory metadata handles.
func CheckOpenedPath(path string, file *os.File, check func([]os.FileInfo) error) error {
	return checkOpenedPath(path, file, true, check)
}

func checkOpenedPath(path string, file *os.File, directories bool, check func([]os.FileInfo) error) error {
	resolved, err := ResolveFilePath(path)
	if err != nil {
		return err
	}
	base := filepath.VolumeName(resolved) + string(filepath.Separator)
	rel, err := filepath.Rel(base, resolved)
	if err != nil || !filepath.IsLocal(rel) && rel != "." || rel == "." && !directories {
		return errors.New("a local file path is required")
	}
	checked, err := openObjectChecked(base, rel, directories, check)
	if err != nil {
		return err
	}
	defer checked.Close()
	a, err := file.Stat()
	if err != nil {
		return err
	}
	b, err := checked.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(a, b) {
		return errors.New("the source file identity changed")
	}
	return nil
}

// OpenIdentityPath returns a held local file or directory for policy metadata.
func OpenIdentityPath(path string) (*os.File, error) {
	resolved, err := ResolveFilePath(path)
	if err != nil {
		return nil, err
	}
	base := filepath.VolumeName(resolved) + string(filepath.Separator)
	rel, err := filepath.Rel(base, resolved)
	if err != nil || !filepath.IsLocal(rel) && rel != "." {
		return nil, errors.New("a local object path is required")
	}
	return openObjectChecked(base, rel, true, nil)
}
