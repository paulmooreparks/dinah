package ollamaagent

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"studio/internal/agents"
	"studio/internal/layout"
)

// FileScope says which folders the file tools may reach for a chat: the
// folders and true when the person has limited them, or nil and false when
// they may go anywhere. It is asked at every tool call, so a change the
// person makes in Settings applies from a running chat's next call
// (docs/plans/ollama-file-limits.md).
type FileScope func(session, cwd string) (folders []string, limited bool)

// SetFileScope gives the connection the function that says where the file
// tools may go. Without one they may go anywhere.
func (c *Conn) SetFileScope(scope FileScope) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.scope = scope
}

// settingsPlace is where the person changes the limit, as the refusal says.
const settingsPlace = "Settings > Agents > Ollama > File Access"

// fileSystem is what the file tools need of the files. The unlimited one is
// the operating system's; the limited one works inside folders opened as roots.
type fileSystem interface {
	readFile(p string) ([]byte, error)
	readDir(p string) ([]fs.DirEntry, error)
	walk(root string, fn fs.WalkDirFunc) error
	writeFile(p, content string) error
}

type osFiles struct{}

func (osFiles) readFile(p string) ([]byte, error)       { return os.ReadFile(p) }
func (osFiles) readDir(p string) ([]fs.DirEntry, error) { return os.ReadDir(p) }
func (osFiles) walk(root string, fn fs.WalkDirFunc) error {
	return filepath.WalkDir(root, fn)
}
func (osFiles) writeFile(p, content string) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(content), 0o644)
}

// rootFiles reaches files below one folder, opened as a root. A root refuses
// a name that leaves it by "..", and a symbolic link that leads out of it,
// which is documented behaviour of os.Root; this does not look at the text of
// a path for either.
type rootFiles struct {
	root *os.Root
	base string
}

// rel is a path below the folder as the root names it.
func (r rootFiles) rel(p string) (string, error) {
	rel, err := filepath.Rel(r.base, p)
	if err != nil || !layout.Inside(r.base, p) {
		return "", fmt.Errorf("%s is outside %s", p, r.base)
	}
	return rel, nil
}

func (r rootFiles) readFile(p string) ([]byte, error) {
	rel, err := r.rel(p)
	if err != nil {
		return nil, err
	}
	return r.root.ReadFile(rel)
}

func (r rootFiles) readDir(p string) ([]fs.DirEntry, error) {
	rel, err := r.rel(p)
	if err != nil {
		return nil, err
	}
	f, err := r.root.Open(rel)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.ReadDir(-1)
}

func (r rootFiles) walk(root string, fn fs.WalkDirFunc) error {
	rel, err := r.rel(root)
	if err != nil {
		return err
	}
	return fs.WalkDir(r.root.FS(), filepath.ToSlash(rel), func(name string, d fs.DirEntry, err error) error {
		return fn(filepath.Join(r.base, filepath.FromSlash(name)), d, err)
	})
}

func (r rootFiles) writeFile(p, content string) error {
	rel, err := r.rel(p)
	if err != nil {
		return err
	}
	if dir := filepath.Dir(rel); dir != "." {
		if err := r.root.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return r.root.WriteFile(rel, []byte(content), 0o644)
}

// stat checks that a path below the folder can be reached through the root.
func (r rootFiles) stat(p string) error {
	rel, err := r.rel(p)
	if err != nil {
		return err
	}
	_, err = r.root.Stat(rel)
	return err
}

// files returns the file system a tool call at path p works in, and a
// function that closes it. With a limit it is the one folder that contains
// p; with none it is the operating system's. A path inside none of the
// folders is refused, with a message for the model (refusal).
func (t turnTools) files(p string) (fileSystem, func(), string) {
	folders, limited := t.folders()
	if !limited {
		return osFiles{}, func() {}, ""
	}
	best := ""
	for _, f := range folders {
		if layout.Inside(f, p) && len(f) > len(best) {
			best = f
		}
	}
	if best == "" {
		return nil, nil, refusal(p, folders)
	}
	root, err := os.OpenRoot(best)
	if err != nil {
		return nil, nil, fmt.Sprintf("%s could not be opened: %s", best, err.Error())
	}
	return rootFiles{root: root, base: best}, func() { root.Close() }, ""
}

// checkDir refuses the folder a command is to start in when it is outside the
// folders the person limited the chat to, by a link included. It does not
// confine what the command does once it runs, which a shell cannot be
// confined to.
func (t turnTools) checkDir(dir string) string {
	fsys, done, refused := t.files(dir)
	if refused != "" {
		return refused
	}
	defer done()
	if r, limited := fsys.(rootFiles); limited {
		if err := r.stat(dir); err != nil {
			return t.failed(err)
		}
	}
	return ""
}

// folders asks for the folders the tools may reach.
func (t turnTools) folders() ([]string, bool) {
	if t.scope == nil {
		return nil, false
	}
	folders, limited := t.scope(t.session, t.cwd)
	if !limited {
		return nil, false
	}
	out := make([]string, 0, len(folders))
	for _, f := range folders {
		if f != "" {
			out = append(out, filepath.Clean(f))
		}
	}
	return out, true
}

// refusal is what the model is told when a path is outside the folders.
func refusal(p string, folders []string) string {
	list := "none"
	if len(folders) > 0 {
		list = strings.Join(folders, ", ")
	}
	return fmt.Sprintf("%s is outside the folders this chat may use: %s. The person sets this in %s. Work within those folders, or tell the person what you need.", p, list, settingsPlace)
}

// failed is what the model is told when a limited operation fails inside a
// folder: the error, and, unless the file is simply not there, that a link
// leading out of the folders cannot be followed.
func (t turnTools) failed(err error) string {
	if errors.Is(err, fs.ErrNotExist) {
		return err.Error()
	}
	folders, _ := t.folders()
	return fmt.Sprintf("%s. The folders this chat may use are %s, and a link that leaves them cannot be followed. The person sets this in %s.", err.Error(), strings.Join(folders, ", "), settingsPlace)
}

// readLimited reads a file for read_file inside a limited scope, as the
// host's reader does without one.
func (t turnTools) readLimited(fsys fileSystem, p string, offset, limit int) (string, bool) {
	b, err := fsys.readFile(p)
	if err != nil {
		return t.failed(err), false
	}
	var line, n *int
	if offset > 0 {
		line = &offset
	}
	if limit > 0 {
		n = &limit
	}
	content := agents.SliceLines(string(b), line, n)
	if len(content) > maxRead {
		return content[:maxRead] + fmt.Sprintf("\n\n[The file goes on; it was cut at %d KB. Read the rest with offset and limit.]", maxRead>>10), true
	}
	return content, true
}
