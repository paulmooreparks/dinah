package docservice

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"

	"studio/internal/resource"
)

// A web root is a folder the person chose as the top of a site. A document
// under it may show a picture by a path that starts with "/", which is read
// from that folder, as the site's server would serve it. The person chooses
// the folder in the system's dialog and the service never guesses one. A
// document cannot say where its own web root is, since that would let a
// file point the page at a folder of the file's choosing. The choice holds
// for every file under the folder, in every application.

func (st *store) webRootsFile() string { return filepath.Join(st.dir, "webroots.json") }

func (st *store) webRootsLocked() []string {
	var roots []string
	if data, err := os.ReadFile(st.webRootsFile()); err == nil {
		if json.Unmarshal(data, &roots) != nil {
			roots = nil
		}
	}
	return roots
}

// inside reports whether path is below the folder root.
func inside(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// webRoot is the web root of the file at path: the nearest folder above it
// that the person chose, or "" when there is none.
func (st *store) webRoot(path string) string {
	st.mu.Lock()
	defer st.mu.Unlock()
	best := ""
	for _, root := range st.webRootsLocked() {
		if inside(root, path) && len(root) > len(best) {
			best = root
		}
	}
	return best
}

func (st *store) addWebRoot(root string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	roots := st.webRootsLocked()
	for _, r := range roots {
		if r == root {
			return nil
		}
	}
	data, err := json.Marshal(append(roots, root))
	if err != nil {
		return err
	}
	return writeWhole(st.webRootsFile(), data)
}

func (st *store) removeWebRoot(root string) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	kept := []string{}
	for _, r := range st.webRootsLocked() {
		if r != root {
			kept = append(kept, r)
		}
	}
	data, err := json.Marshal(kept)
	if err != nil {
		return err
	}
	return writeWhole(st.webRootsFile(), data)
}

// WebRoot is the web root of the document the page shows, or "" when the
// person has chosen none for it.
func (p *Page) WebRoot(shown string) string {
	path, err := p.file(shown)
	if err != nil {
		return ""
	}
	return p.d.service.store.webRoot(path)
}

// PickWebRoot shows the system's folder dialog in the page's window and
// makes the folder chosen the web root of the document and of every file
// under that folder. It returns the folder, or "" when the person
// cancelled. A folder the document is not in is refused, since it could
// not be that document's web root.
func (p *Page) PickWebRoot(ctx context.Context, shown string) (string, error) {
	path, err := p.file(shown)
	if err != nil {
		return "", errors.New("save the document to a file before choosing its web root")
	}
	d, s := p.d, p.d.service
	start := s.store.webRoot(path)
	if start == "" {
		start = filepath.Dir(path)
	}
	chosen, err := s.choose(ctx, d, "folder", map[string]string{"folder": start})
	if err != nil || chosen == "" {
		return "", err
	}
	root, err := cleanPath(chosen)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return "", fmt.Errorf("%s is not a folder", root)
	}
	if !inside(root, path) {
		return "", fmt.Errorf("%s is not in %s, so that folder cannot be its web root", filepath.Base(path), root)
	}
	if err := s.store.addWebRoot(root); err != nil {
		return "", err
	}
	return root, nil
}

// ClearWebRoot forgets the web root of the document the page shows, for
// every file under it.
func (p *Page) ClearWebRoot(shown string) error {
	path, err := p.file(shown)
	if err != nil {
		return err
	}
	s := p.d.service
	if root := s.store.webRoot(path); root != "" {
		return s.store.removeWebRoot(root)
	}
	return nil
}

// readUnderWebRoot reads the file a site path stands for under the web
// root of the document at path. The path is cleaned first, so that ".."
// cannot climb above the root, and the file is opened through os.Root,
// which refuses any path, or link, that leaves the folder.
func (s *Service) readUnderWebRoot(path, name string) ([]byte, error) {
	root := s.store.webRoot(path)
	if root == "" || strings.ContainsAny(name, "\\:\x00\r\n") {
		return nil, resource.ErrNotFound
	}
	rel := strings.TrimPrefix(pathpkg.Clean("/"+name), "/")
	if rel == "" || !fs.ValidPath(rel) {
		return nil, resource.ErrNotFound
	}
	folder, err := os.OpenRoot(root)
	if err != nil {
		return nil, resource.ErrNotFound
	}
	defer folder.Close()
	file, err := folder.Open(filepath.FromSlash(rel))
	if err != nil {
		return nil, resource.ErrNotFound
	}
	defer file.Close()
	if info, err := file.Stat(); err != nil || !info.Mode().IsRegular() {
		return nil, resource.ErrNotFound
	}
	data, err := io.ReadAll(io.LimitReader(file, resource.MaxFile+1))
	if err != nil || len(data) > resource.MaxFile {
		return nil, errors.New("the file could not be read")
	}
	return data, nil
}
