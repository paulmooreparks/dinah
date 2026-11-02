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

// ReadTextFile preserves ordinary filesystem access while withholding protected
// settings. It returns only a bounded selected range, never a truncated result.
func ReadTextFile(ctx context.Context, l layout.Layout, path string, managedRoots []string, line, limit *int) (string, error) {
	if err := ValidateLineRange(line, limit); err != nil {
		return "", err
	}
	if !filepath.IsAbs(path) {
		return "", errors.New("an absolute file path is required")
	}
	path = filepath.Clean(path)
	if reason := ProtectedPath(l, path, managedRoots...); reason != "" {
		return "", errors.New("this file is protected: " + reason)
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	if reason := ProtectedPath(l, resolved, managedRoots...); reason != "" {
		return "", errors.New("this file is protected: " + reason)
	}
	base := filepath.VolumeName(resolved) + string(filepath.Separator)
	rel, err := filepath.Rel(base, resolved)
	if err != nil || !filepath.IsLocal(rel) || rel == "." {
		return "", errors.New("a regular file path is required")
	}
	file, err := openReadFile(base, rel)
	if err != nil {
		return "", err
	}
	defer file.Close()
	return readTextRange(ctx, file, line, limit)
}

// Open every directory through its parent's handle and compare identities.
// A concurrently replaced directory or final file cannot redirect this read.
func openReadFile(base, rel string) (*os.File, error) {
	root, err := os.OpenRoot(base)
	if err != nil {
		return nil, err
	}
	defer func() { root.Close() }()
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
		root.Close()
		root = next
	}
	name := parts[len(parts)-1]
	before, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, errors.New("only regular text files can be read")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		file.Close()
		return nil, errors.New("the file changed while opening it")
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
