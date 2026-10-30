package resource

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"path"
	"strings"
	"time"
)

// FS presents a provider's folder as a read-only fs.FS, so that code which
// loads packages of files (document types, and later other kinds) reads them
// from any kind of source through its provider, with the provider's own
// confinement, and is tested against an in-memory fs.FS. The provider must be
// a TextReader and a Lister; root is the folder within the source, "." for the
// source's root. Files are text only, as TextReader reads them.
func FS(ctx context.Context, provider any, root string) (fs.FS, error) {
	r, ok := provider.(TextReader)
	if !ok {
		return nil, ErrUnsupported
	}
	l, ok := provider.(Lister)
	if !ok {
		return nil, ErrUnsupported
	}
	if root != "." && !fs.ValidPath(root) {
		return nil, &fs.PathError{Op: "open", Path: root, Err: fs.ErrInvalid}
	}
	return providerFS{ctx: ctx, reader: r, lister: l, root: root}, nil
}

type providerFS struct {
	ctx    context.Context
	reader TextReader
	lister Lister
	root   string
}

func (p providerFS) full(name string) string { return path.Join(p.root, name) }

func pathError(op, name string, err error) error {
	if errors.Is(err, ErrNotFound) {
		err = fs.ErrNotExist
	}
	return &fs.PathError{Op: op, Path: name, Err: err}
}

// ReadDir lists a folder, in the order the provider gives.
func (p providerFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, pathError("readdir", name, fs.ErrInvalid)
	}
	entries, err := p.lister.List(p.ctx, p.full(name))
	if err != nil {
		return nil, pathError("readdir", name, err)
	}
	out := make([]fs.DirEntry, len(entries))
	for i, e := range entries {
		out[i] = fs.FileInfoToDirEntry(info{name: e.Name, dir: e.Folder, size: e.Size})
	}
	return out, nil
}

// Stat finds a name in its parent folder's listing, since a provider is not
// required to describe a single entry.
func (p providerFS) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, pathError("stat", name, fs.ErrInvalid)
	}
	if name == "." {
		return info{name: ".", dir: true}, nil
	}
	entries, err := p.lister.List(p.ctx, p.full(path.Dir(name)))
	if err != nil {
		return nil, pathError("stat", name, err)
	}
	base := path.Base(name)
	for _, e := range entries {
		if e.Name == base {
			return info{name: base, dir: e.Folder, size: e.Size}, nil
		}
	}
	return nil, pathError("stat", name, fs.ErrNotExist)
}

func (p providerFS) Open(name string) (fs.File, error) {
	st, err := p.Stat(name)
	if err != nil {
		return nil, err
	}
	if st.IsDir() {
		entries, err := p.ReadDir(name)
		if err != nil {
			return nil, err
		}
		return &dirFile{info: st.(info), entries: entries}, nil
	}
	t, err := p.reader.ReadText(p.ctx, p.full(name))
	if err != nil {
		return nil, pathError("open", name, err)
	}
	i := st.(info)
	i.size = int64(len(t.Text))
	return &textFile{info: i, Reader: strings.NewReader(t.Text)}, nil
}

type info struct {
	name string
	dir  bool
	size int64
}

func (i info) Name() string { return i.name }
func (i info) Size() int64  { return i.size }
func (i info) Mode() fs.FileMode {
	if i.dir {
		return fs.ModeDir | 0o555
	}
	return 0o444
}
func (i info) ModTime() time.Time { return time.Time{} }
func (i info) IsDir() bool        { return i.dir }
func (i info) Sys() any           { return nil }

type textFile struct {
	info info
	*strings.Reader
}

func (f *textFile) Stat() (fs.FileInfo, error) { return f.info, nil }
func (f *textFile) Close() error               { return nil }

type dirFile struct {
	info    info
	entries []fs.DirEntry
	read    int
}

func (d *dirFile) Stat() (fs.FileInfo, error) { return d.info, nil }
func (d *dirFile) Close() error               { return nil }
func (d *dirFile) Read([]byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.info.name, Err: errors.New("is a directory")}
}
func (d *dirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	rest := d.entries[d.read:]
	if n <= 0 {
		d.read = len(d.entries)
		return rest, nil
	}
	if len(rest) == 0 {
		return nil, io.EOF
	}
	if n > len(rest) {
		n = len(rest)
	}
	d.read += n
	return rest[:n], nil
}
