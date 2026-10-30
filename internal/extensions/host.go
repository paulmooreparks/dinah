package extensions

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	extension "github.com/paulmooreparks/dinah-extension-go"
)

type limitedBuffer struct {
	mu        sync.Mutex
	data      []byte
	limit     int
	truncated bool
}

func (b *limitedBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	n := len(data)
	room := b.limit - len(b.data)
	if len(data) > room {
		data = data[:max(0, room)]
		b.truncated = true
	}
	b.data = append(b.data, data...)
	return n, nil
}
func (b *limitedBuffer) String() string { b.mu.Lock(); defer b.mu.Unlock(); return string(b.data) }

type CommandEntry struct {
	At          time.Time           `json:"at"`
	Cwd         string              `json:"cwd"`
	Program     string              `json:"program"`
	Args        []string            `json:"args"`
	ArgsBase64  []string            `json:"argsBase64,omitempty"`
	HasStdin    bool                `json:"hasStdin,omitempty"`
	Explanation string              `json:"explanation"`
	Result      extension.RunResult `json:"result"`
}

func decodeInput(raw json.RawMessage, out any) error {
	if len(raw) > 5<<20 {
		return errors.New("host input exceeds limit")
	}
	return json.Unmarshal(raw, out)
}
func confined(ws extension.Workspace, p string, write bool) (string, error) {
	if strings.ContainsRune(p, 0) || p == "" {
		return "", errors.New("a workspace path is required")
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(ws.Path, p)
	}
	p = filepath.Clean(p)
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil && write && os.IsNotExist(err) {
		parent, e := filepath.EvalSymlinks(filepath.Dir(p))
		if e != nil {
			return "", e
		}
		resolved = filepath.Join(parent, filepath.Base(p))
		err = nil
	}
	if err != nil {
		return "", err
	}
	roots := []string{ws.Path}
	for _, source := range ws.Sources {
		roots = append(roots, source.Path)
	}
	for _, root := range roots {
		base, e := filepath.EvalSymlinks(root)
		if e != nil {
			continue
		}
		rel, e := filepath.Rel(base, resolved)
		if e == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
			return resolved, nil
		}
	}
	return "", errors.New("the path is outside this invocation's workspace sources")
}

// Keep the root handle open through every file operation. An earlier path
// validation alone cannot protect a later read from a replaced symlink.
func fileRoot(ws extension.Workspace, p string, create bool) (*os.Root, string, error) {
	resolved, err := confined(ws, p, create)
	if err != nil {
		return nil, "", err
	}
	roots := []string{ws.Path}
	for _, source := range ws.Sources {
		roots = append(roots, source.Path)
	}
	for _, path := range roots {
		base, err := filepath.EvalSymlinks(path)
		if err != nil {
			continue
		}
		rel, err := filepath.Rel(base, resolved)
		if err != nil || !filepath.IsLocal(rel) {
			continue
		}
		root, err := os.OpenRoot(base)
		return root, rel, err
	}
	return nil, "", errors.New("file is outside workspace sources")
}
func (m *Manager) hostHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/control" {
		m.controlHTTP(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	fail := func(status int, err error) {
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(extension.HostResponse{Error: err.Error()})
	}
	if r.Method != "POST" || r.URL.Path != "/host" || r.Host != m.listener.Addr().String() || r.Header.Get("Origin") != "" {
		fail(403, errors.New("host callback is not allowed"))
		return
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	m.mu.Lock()
	scope, ok := m.scopes[token]
	m.mu.Unlock()
	if !ok || scope.ctx.Err() != nil {
		fail(403, errors.New("host scope is missing or expired"))
		return
	}
	var request extension.HostRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 6<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		fail(400, err)
		return
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		fail(400, errors.New("trailing request data"))
		return
	}
	ctx, cancel := context.WithCancel(scope.ctx)
	defer cancel()
	stop := context.AfterFunc(r.Context(), cancel)
	defer stop()
	result, err := m.hostCall(ctx, scope, request.Tool, request.Input)
	if err != nil {
		fail(400, err)
		return
	}
	data, err := json.Marshal(result)
	if err != nil {
		fail(500, err)
		return
	}
	json.NewEncoder(w).Encode(extension.HostResponse{Result: data})
}
func require(s scope, permission string) error {
	if !extension.Has(s.permissions, permission) {
		return fmt.Errorf("action did not declare %s", permission)
	}
	return nil
}
func (m *Manager) hostCall(ctx context.Context, s scope, tool string, raw json.RawMessage) (any, error) {
	switch tool {
	case "host.workspaces":
		if err := require(s, "workspace.read"); err != nil {
			return nil, err
		}
		return []extension.Workspace{s.workspace}, nil
	case "host.run":
		var in extension.RunInput
		if err := decodeInput(raw, &in); err != nil {
			return nil, err
		}
		if err := require(s, "process.run:"+in.Program); err != nil {
			return nil, err
		}
		installations, err := m.Store.List()
		if err != nil {
			return nil, err
		}
		review := false
		for _, installed := range installations {
			if installed.Manifest.ID == s.extensionID {
				review = installed.ReviewCommands
			}
		}
		// Show Me First reviews the commands of actions that rewrite history or
		// reach a network, and of actions with no declared effect. A read or a
		// reversible local change runs without a review.
		if review && extension.EffectAsks(s.effect) {
			if m.Hooks.Review == nil {
				return nil, errors.New("command review is enabled but no review interface is available")
			}
			in.Args, err = m.Hooks.Review(ctx, s.extensionID, in)
			if err != nil {
				return nil, err
			}
		}
		if !extension.ValidPermission("process.run:"+in.Program) || len(in.Args) > 256 || len(in.Stdin) > 4<<20 {
			return nil, errors.New("invalid executable or oversized command")
		}
		size := 0
		for _, arg := range in.Args {
			size += len(arg)
			if strings.ContainsRune(arg, 0) {
				return nil, errors.New("command contains NUL")
			}
		}
		if size > 128<<10 {
			return nil, errors.New("command arguments exceed limit")
		}
		cwd, err := confined(s.workspace, in.Cwd, false)
		if err != nil {
			return nil, err
		}
		if in.TimeoutSeconds <= 0 {
			in.TimeoutSeconds = 30
		}
		if in.TimeoutSeconds > 120 {
			return nil, errors.New("command timeout exceeds 120 seconds")
		}
		ctx, cancel := context.WithTimeout(ctx, time.Duration(in.TimeoutSeconds)*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, in.Program, in.Args...)
		cmd.WaitDelay = 2 * time.Second
		cmd.Dir = cwd
		cmd.Env = cleanEnvironment(os.Environ())
		cmd.Stdin = strings.NewReader(in.Stdin)
		out, stderr := &limitedBuffer{limit: 2 << 20}, &limitedBuffer{limit: 2 << 20}
		cmd.Stdout, cmd.Stderr = out, stderr
		hideProcess(cmd)
		start := time.Now()
		err = cmd.Run()
		result := extension.RunResult{Stdout: out.String(), Stderr: stderr.String(), ExitCode: 0, Truncated: out.truncated || stderr.truncated, Milliseconds: time.Since(start).Milliseconds()}
		if err != nil {
			result.ExitCode = -1
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				result.ExitCode = exit.ExitCode()
			} else {
				result.Stderr += err.Error()
			}
		}
		if logErr := m.logCommand(s.extensionID, CommandEntry{At: time.Now().UTC(), Cwd: cwd, Program: in.Program, Args: in.Args, ArgsBase64: extension.EncodedArguments(in.Args), HasStdin: in.Stdin != "", Explanation: in.Explanation, Result: result}); logErr != nil {
			return nil, fmt.Errorf("command ran but its log could not be saved: %w", logErr)
		}
		if ctx.Err() != nil {
			return nil, fmt.Errorf("command ended at its deadline; inspect the command log before retrying: %w", ctx.Err())
		}
		return result, nil
	case "host.resolve":
		if err := require(s, "workspace.read"); err != nil {
			return nil, err
		}
		var in struct {
			Path   string `json:"path"`
			Create bool   `json:"create"`
		}
		if err := decodeInput(raw, &in); err != nil {
			return nil, err
		}
		path, err := confined(s.workspace, in.Path, in.Create)
		return map[string]string{"path": path}, err
	case "host.read":
		if err := require(s, "workspace.read"); err != nil {
			return nil, err
		}
		var in extension.FileInput
		if err := decodeInput(raw, &in); err != nil {
			return nil, err
		}
		root, p, err := fileRoot(s.workspace, in.Path, false)
		if err != nil {
			return nil, err
		}
		defer root.Close()
		f, err := root.Open(p)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		data, err := io.ReadAll(io.LimitReader(f, 2<<20+1))
		if err != nil {
			return nil, err
		}
		if len(data) > 2<<20 {
			return nil, errors.New("file exceeds 2 MiB")
		}
		sum := sha256.Sum256(data)
		return extension.FileResult{Text: string(data), SHA256: hex.EncodeToString(sum[:])}, nil
	case "host.write":
		if err := require(s, "workspace.write"); err != nil {
			return nil, err
		}
		var in extension.FileInput
		if err := decodeInput(raw, &in); err != nil {
			return nil, err
		}
		if len(in.Text) > 2<<20 {
			return nil, errors.New("file exceeds 2 MiB")
		}
		root, p, err := fileRoot(s.workspace, in.Path, true)
		if err != nil {
			return nil, err
		}
		defer root.Close()
		m.mu.Lock()
		defer m.mu.Unlock()
		var old []byte
		f, err := root.Open(p)
		if err == nil {
			old, err = io.ReadAll(io.LimitReader(f, 2<<20+1))
			f.Close()
			if len(old) > 2<<20 {
				return nil, errors.New("existing file exceeds 2 MiB")
			}
		}
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		sum := sha256.Sum256(old)
		if in.ExpectedSHA256 == "" {
			if err == nil {
				return nil, errors.New("existing writes require expectedSha256")
			}
		} else if err != nil || hex.EncodeToString(sum[:]) != in.ExpectedSHA256 {
			return nil, errors.New("file changed since it was read")
		}
		name := filepath.Join(filepath.Dir(p), ".dinah-extension-"+rand.Text())
		temp, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return nil, err
		}
		defer root.Remove(name)
		_, err = temp.WriteString(in.Text)
		if err == nil {
			err = temp.Sync()
		}
		closeErr := temp.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if err = root.Rename(name, p); err != nil {
			return nil, err
		}
		sum = sha256.Sum256([]byte(in.Text))
		return extension.FileResult{Text: in.Text, SHA256: hex.EncodeToString(sum[:])}, nil
	case "host.notify":
		if err := require(s, "notification.post"); err != nil {
			return nil, err
		}
		var in struct {
			Title string `json:"title"`
			Body  string `json:"body"`
		}
		if err := decodeInput(raw, &in); err != nil {
			return nil, err
		}
		if len(in.Title) > 200 || len(in.Body) > 20000 {
			return nil, errors.New("notification exceeds limit")
		}
		if m.Hooks.Notify == nil {
			return nil, errors.New("notification service unavailable")
		}
		return map[string]bool{"posted": true}, m.Hooks.Notify(ctx, in.Title, in.Body)
	case "host.terminal":
		if err := require(s, "terminal.open"); err != nil {
			return nil, err
		}
		var in extension.TerminalInput
		if err := decodeInput(raw, &in); err != nil {
			return nil, err
		}
		if err := require(s, "process.run:"+in.Program); err != nil {
			return nil, err
		}
		cwd, err := confined(s.workspace, in.Cwd, false)
		if err != nil {
			return nil, err
		}
		in.Cwd = cwd
		if m.Hooks.Terminal == nil {
			return nil, errors.New("terminal service unavailable")
		}
		return map[string]bool{"opened": true}, m.Hooks.Terminal(ctx, in)
	case "host.commandlog":
		if err := require(s, "workspace.read"); err != nil {
			return nil, err
		}
		return m.commandLog(s.extensionID, s.workspace)
	case "host.decorate":
		if err := require(s, "workspace.decorate"); err != nil {
			return nil, err
		}
		if len(raw) > 256<<10 {
			return nil, errors.New("decorations exceed limit")
		}
		if m.Hooks.Decorate == nil {
			return nil, errors.New("decorations are not available in this host")
		}
		return map[string]bool{"updated": true}, m.Hooks.Decorate(ctx, raw)
	case "host.watch":
		if err := require(s, "workspace.read"); err != nil {
			return nil, err
		}
		var in struct {
			Path string `json:"path"`
		}
		if err := decodeInput(raw, &in); err != nil {
			return nil, err
		}
		p, err := confined(s.workspace, in.Path, false)
		if err != nil {
			return nil, err
		}
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			return nil, err
		}
		defer watcher.Close()
		if err = watcher.Add(p); err != nil {
			return nil, err
		}
		timer := time.NewTimer(20 * time.Second)
		defer timer.Stop()
		select {
		case event := <-watcher.Events:
			return map[string]any{"changed": true, "path": event.Name}, nil
		case err := <-watcher.Errors:
			return nil, err
		case <-timer.C:
			return map[string]bool{"changed": false}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return nil, errors.New("unknown host tool")
}
func (m *Manager) logCommand(id string, entry CommandEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	dir := filepath.Join(m.Store.Root, id, "data")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	p := filepath.Join(dir, "commands.jsonl")
	if stat, err := os.Stat(p); err == nil && stat.Size() > 8<<20 {
		os.Remove(p + ".previous")
		if err = os.Rename(p, p+".previous"); err != nil {
			return err
		}
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(data, '\n'))
	if err == nil {
		err = f.Sync()
	}
	return err
}
func (m *Manager) commandLog(id string, ws extension.Workspace) ([]CommandEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, err := os.Open(filepath.Join(m.Store.Root, id, "data", "commands.jsonl"))
	if os.IsNotExist(err) {
		return []CommandEntry{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 16<<20+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 16<<20 {
		return nil, errors.New("command log exceeds limit")
	}
	rows := []CommandEntry{}
	for _, line := range strings.Split(string(data), "\n") {
		var entry CommandEntry
		if json.Unmarshal([]byte(line), &entry) == nil {
			if _, err := confined(ws, entry.Cwd, false); err == nil {
				rows = append(rows, entry)
			}
		}
	}
	if len(rows) > 100 {
		rows = rows[len(rows)-100:]
	}
	return rows, nil
}
