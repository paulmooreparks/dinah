// diagnosticartifacts extracts restricted stack fields and prepares a separate
// text-only upload snapshot. Raw debugger output and dumps are never uploaded.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"studio/internal/agents"
)

type textFile struct {
	name string
	data []byte
}

func disjointDirectories(a, b string) bool {
	a, err := agents.ResolveFilePath(a)
	if err != nil {
		return false
	}
	b, err = agents.ResolveFilePath(b)
	if err != nil {
		return false
	}
	for _, pair := range [][2]string{{a, b}, {b, a}} {
		relative, err := filepath.Rel(pair[0], pair[1])
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return false
		}
	}
	return true
}

func checkedFiles(dir string) ([]textFile, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	var files []textFile
	var total int
	err = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return errors.New("cannot enumerate diagnostic input")
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return errors.New("diagnostic links are not uploadable")
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return errors.New("diagnostic input is not a regular file")
		}
		switch strings.ToLower(filepath.Ext(name)) {
		case ".log", ".txt", ".json", ".jsonl":
		default:
			return errors.New("diagnostic extension is not approved for upload")
		}
		file, err := root.Open(name)
		if err != nil {
			return errors.New("cannot open diagnostic input")
		}
		data, readErr := io.ReadAll(io.LimitReader(file, 32<<20+1))
		closeErr := file.Close()
		if readErr != nil || closeErr != nil {
			return errors.New("cannot read diagnostic input")
		}
		total += len(data)
		if len(data) > 32<<20 || total > 128<<20 || len(files) >= 4096 {
			return errors.New("diagnostic upload size limit exceeded")
		}
		if bytes.HasPrefix(data, []byte("MDMP")) || !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
			return errors.New("binary or dump content is not uploadable")
		}
		files = append(files, textFile{name, data})
		return nil
	})
	return files, err
}

func stage(source, parent string) (string, error) {
	if !filepath.IsAbs(source) || !filepath.IsAbs(parent) || strings.ContainsAny(parent, "\r\n") {
		return "", errors.New("upload paths must be absolute and single-line")
	}
	// Enumerate before creating the snapshot. Product/test children never
	// receive the new directory's unpredictable name in their environment.
	files, err := checkedFiles(source)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", errors.New("no diagnostic files to retain")
	}
	dir, err := os.MkdirTemp(parent, "go-diagnostic-upload-")
	if err != nil {
		return "", err
	}
	complete := false
	defer func() {
		if !complete && filepath.Dir(dir) == filepath.Clean(parent) && strings.HasPrefix(filepath.Base(dir), "go-diagnostic-upload-") {
			_ = os.RemoveAll(dir)
		}
	}()
	if !disjointDirectories(source, dir) {
		return "", errors.New("upload snapshot must be outside the diagnostic input")
	}
	for _, file := range files {
		path := filepath.Join(dir, filepath.FromSlash(file.name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return "", err
		}
		if err := os.WriteFile(path, file.data, 0600); err != nil {
			return "", err
		}
	}
	if _, err := checkedFiles(dir); err != nil {
		return "", err
	}
	complete = true
	return dir, nil
}

func main() {
	mode := flag.String("mode", "stage", "extract or stage")
	diagnostics := flag.String("diagnostics", "", "diagnostic text directory")
	scratch := flag.String("scratch", "", "private native scratch directory")
	parent := flag.String("upload-parent", "", "parent of a fresh upload snapshot")
	outputFile := flag.String("github-output", "", "GitHub step output file")
	flag.Parse()
	var err error
	switch *mode {
	case "extract":
		err = extract(*scratch, *diagnostics, findCDB(), runCDB)
	case "stage":
		var path string
		path, err = stage(*diagnostics, *parent)
		if err == nil {
			var output *os.File
			output, err = os.OpenFile(*outputFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
			if err == nil {
				_, err = fmt.Fprintln(output, "upload_path="+path)
				err = errors.Join(err, output.Close())
			}
		}
	default:
		err = errors.New("unknown diagnostic artifact mode")
	}
	if err != nil {
		// Do not print an underlying filesystem/debugger error that may carry
		// private paths or data. The safe extraction report records limits.
		fmt.Fprintln(os.Stderr, "diagnostic artifact preparation failed")
		os.Exit(1)
	}
}
