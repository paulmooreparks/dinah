package drafts

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"studio/internal/resource"
)

// A draft keeps the files added to it, such as an image pasted into the
// Editor, beside its record until they are saved with it: in the folder
// <id>.files of the store. Saving writes those the text uses to the document's
// source, as a page's attachments or as files beside a file, and they are then
// the source's. Discarding or sweeping the draft removes them.

func (s *Store) filesDir(id string) (string, error) {
	if !idRe.MatchString(id) {
		return "", ErrNotFound
	}
	return filepath.Join(s.dir, id+".files"), nil
}

// FileName makes a name a file can be kept under, in a draft and in any
// source: the last part of a path, without characters a Windows file name
// cannot have, and not starting with a dot.
func FileName(name string) (string, error) {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || strings.ContainsRune(`/\:*?"<>|`, r) {
			return '-'
		}
		return r
	}, name)
	name = strings.TrimLeft(strings.TrimSpace(name), ".")
	for len(name) > 200 {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}
	if name == "" {
		return "", errors.New("the file needs a name")
	}
	return name, nil
}

// unusedName is name, or name with a number before its extension, such as
// "chart (2).png", so that it is none of taken.
func unusedName(name string, taken []string) string {
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for n := 2; slices.ContainsFunc(taken, func(t string) bool { return strings.EqualFold(t, name) }); n++ {
		name = fmt.Sprintf("%s (%d)%s", stem, n, ext)
	}
	return name
}

// AddFile keeps a file with a draft and returns the name it is kept under:
// its own name, or a numbered one when a file kept with the draft or one of
// taken, the names already used where the draft will be saved, has it.
func (s *Store) AddFile(id, name string, data []byte, taken []string) (string, error) {
	if _, err := s.Get(id); err != nil {
		return "", err
	}
	if len(data) > resource.MaxFile {
		return "", fmt.Errorf("a file is limited to %d MiB", resource.MaxFile>>20)
	}
	name, err := FileName(name)
	if err != nil {
		return "", err
	}
	dir, err := s.filesDir(id)
	if err != nil {
		return "", err
	}
	unlock, err := s.lock(id)
	if err != nil {
		return "", err
	}
	defer unlock()
	kept, err := s.FileNames(id)
	if err != nil {
		return "", err
	}
	name = unusedName(name, append(kept, taken...))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", errors.New("the file could not be kept with the draft")
	}
	if err := writeAtomic(filepath.Join(dir, name), data); err != nil {
		return "", errors.New("the file could not be kept with the draft")
	}
	return name, nil
}

// FileNames lists the files kept with a draft.
func (s *Store) FileNames(id string) ([]string, error) {
	dir, err := s.filesDir(id)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.Type().IsRegular() && !strings.HasPrefix(e.Name(), ".") {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// ReadFile reads a file kept with a draft.
func (s *Store) ReadFile(id, name string) ([]byte, error) {
	dir, err := s.filesDir(id)
	if err != nil {
		return nil, err
	}
	if clean, err := FileName(name); err != nil || clean != name {
		return nil, ErrNotFound
	}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return data, err
}

// removeFiles removes the named files kept with a draft, or all of them
// when names is nil.
func (s *Store) removeFiles(id string, names []string) {
	dir, err := s.filesDir(id)
	if err != nil {
		return
	}
	if names == nil {
		os.RemoveAll(dir)
		return
	}
	for _, name := range names {
		os.Remove(filepath.Join(dir, name))
	}
	os.Remove(dir) // only when it is empty
}

// writeAtomic writes data to a new file beside name and renames it over
// name, so a reader finds the old file or the new one.
func writeAtomic(name string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(name), ".add-*.tmp")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), name); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}
