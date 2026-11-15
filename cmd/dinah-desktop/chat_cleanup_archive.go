package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Recycle payloads live outside the directories scanned for startup, so a
// retained transcript or queue is never read by the normal startup sweeps.
type chatCleanupArchive struct {
	Version  int                      `json:"version"`
	Row      ChatCleanupRow           `json:"row"`
	Settings chatCleanupSettings      `json:"settings"`
	Files    []chatCleanupArchiveFile `json:"files"`
}

type chatCleanupArchiveFile struct {
	Path string `json:"path"`
	Slot string `json:"slot"`
}

func chatCleanupArchiveDir(id string) string {
	return filepath.Join(configDir(), "recycle", base64.RawURLEncoding.EncodeToString([]byte(id)))
}

func (r *chatCleanupArchive) save() error {
	if r.Version != 1 || r.Row.SessionID == "" {
		return errors.New("invalid chat recycle archive")
	}
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return writeStateFile(filepath.Join(chatCleanupArchiveDir(r.Row.SessionID), "manifest.json"), data)
}

func readChatCleanupArchive(id string) (*chatCleanupArchive, error) {
	data, err := readStateFile(filepath.Join(chatCleanupArchiveDir(id), "manifest.json"))
	if err != nil {
		return nil, err
	}
	var r chatCleanupArchive
	if err = json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	if r.Version != 1 || r.Row.SessionID != id {
		return nil, errors.New("the recycle archive does not match this chat")
	}
	return &r, nil
}

func cleanupLocalPath(root, rel string) (string, error) {
	if !filepath.IsLocal(rel) {
		return "", errors.New("the recycle archive contains an invalid local path")
	}
	return filepath.Join(root, rel), nil
}

func (r *chatCleanupArchive) addFile(path string) error {
	rel, err := filepath.Rel(configDir(), path)
	if err != nil {
		return err
	}
	if _, err = cleanupLocalPath(configDir(), rel); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("the retained chat file %q is not a regular file", rel)
	}
	for _, f := range r.Files {
		if f.Path == rel {
			return nil
		}
	}
	r.Files = append(r.Files, chatCleanupArchiveFile{Path: rel, Slot: fmt.Sprintf("files/%d", len(r.Files))})
	return nil
}

// moveFiles is idempotent after interruption. It never overwrites a file on
// either side: two existing copies signal a conflict which needs inspection.
func (r *chatCleanupArchive) moveFiles(restore bool) error {
	root := chatCleanupArchiveDir(r.Row.SessionID)
	for _, f := range r.Files {
		active, err := cleanupLocalPath(configDir(), f.Path)
		if err != nil {
			return err
		}
		retained, err := cleanupLocalPath(root, f.Slot)
		if err != nil {
			return err
		}
		source, target := active, retained
		if restore {
			source, target = retained, active
		}
		sourceInfo, sourceErr := os.Lstat(source)
		targetInfo, targetErr := os.Lstat(target)
		if sourceErr != nil && !errors.Is(sourceErr, os.ErrNotExist) {
			return sourceErr
		}
		if targetErr != nil && !errors.Is(targetErr, os.ErrNotExist) {
			return targetErr
		}
		if sourceErr == nil && !sourceInfo.Mode().IsRegular() {
			return errors.New("a retained chat source is not a regular file")
		}
		if targetErr == nil && !targetInfo.Mode().IsRegular() {
			return errors.New("a retained chat destination is not a regular file")
		}
		if sourceErr == nil && targetErr == nil {
			return fmt.Errorf("both retained and active chat files exist for %q", f.Path)
		}
		if sourceErr != nil && targetErr != nil {
			return fmt.Errorf("the retained chat file %q is missing", f.Path)
		}
		if sourceErr != nil {
			continue
		}
		if err = os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		if err = os.Rename(source, target); err != nil {
			return err
		}
	}
	return nil
}
