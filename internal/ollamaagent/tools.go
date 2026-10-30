package ollamaagent

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"studio/internal/acp"
	"studio/internal/agentpolicy"
	"studio/internal/profile"
)

// The tools Dinah runs for an Ollama model, which has no runtime of
// its own. They match what Devin and Codex do in theirs: reading anything,
// writing in the workspace without asking and elsewhere only with the
// person's permission, and running a command only with it.

const (
	maxRead     = 256 << 10
	maxOutput   = 64 << 10
	maxFound    = 500
	maxMatches  = 200
	cmdDefault  = 120
	cmdLongest  = 1800
	toolTimeout = 30 * time.Minute

	// SearchMaxFiles and SearchMaxBytes bound the work of one find_files or
	// search_text call: the files it visits and, for a search, the bytes it
	// reads, whichever is reached first. They are a limit on work and not on
	// time, so the same tree gives the model the same answer on a busy
	// machine as on an idle one. A call that reaches either returns what it
	// found with a plain note that it stopped early and how far it got.
	SearchMaxFiles = 20000
	SearchMaxBytes = 256 << 20
)

// searchLimits start at SearchMaxFiles and SearchMaxBytes. They are variables
// only so that a test can lower them.
var searchLimits = struct{ files, bytes int }{SearchMaxFiles, SearchMaxBytes}

// errSearchBudget ends a walk that has done as much work as one call may.
var errSearchBudget = errors.New("the search reached its limit")

// searchWork is what a walk has done so far, for the budget and its note.
type searchWork struct {
	files, bytes int
	last         string
}

func (w *searchWork) note(root string) string {
	at := w.last
	if rel, err := filepath.Rel(root, w.last); err == nil && w.last != "" {
		at = filepath.ToSlash(rel)
	}
	return fmt.Sprintf("The search stopped early. It visited %d files and read %.1f MB, up to %q, which is as much as one search covers, so files after that point were not looked at. Search a narrower folder or use a narrower pattern.", w.files, float64(w.bytes)/(1<<20), at)
}

func fn(name, description string, properties map[string]any, required ...string) map[string]any {
	if required == nil {
		required = []string{}
	}
	return map[string]any{"type": "function", "function": map[string]any{"name": name, "description": description,
		"parameters": map[string]any{"type": "object", "properties": properties, "required": required}}}
}

func str(description string) map[string]any {
	return map[string]any{"type": "string", "description": description}
}

func num(description string) map[string]any {
	return map[string]any{"type": "integer", "description": description}
}

// ownTools are the tool definitions Dinah runs itself.
func ownTools(approvals string) []any {
	writeWarning, commandWarning := "", ""
	if approvals != ApprovalsAll && approvals != ApprovalsNone {
		writeWarning = " Writing outside the workspace folder asks the person first."
		commandWarning = " The person is asked first."
	}
	return []any{
		fn("read_file", "Read a text file. A relative path is relative to the workspace folder. Give offset (the first line, from 1) and limit (how many lines) to read part of a long file.",
			map[string]any{"path": str("The file's path."), "offset": num("The first line to read, from 1."), "limit": num("How many lines to read.")}, "path"),
		fn("write_file", "Write a text file whole, making its folders."+writeWarning,
			map[string]any{"path": str("The file's path."), "content": str("The file's whole new text.")}, "path", "content"),
		fn("list_directory", "List a folder's entries, folders ending in /.",
			map[string]any{"path": str("The folder's path; the workspace folder when empty.")}),
		fn("find_files", "Find files whose path matches a glob pattern such as **/*.md or docs/*.txt, under a folder.",
			map[string]any{"pattern": str("The glob pattern, matched against the path relative to the folder; ** matches any number of folders."), "path": str("The folder to search; the workspace folder when empty.")}, "pattern"),
		fn("search_text", "Search the text of files under a folder for a regular expression, giving each matching line with its file and line number.",
			map[string]any{"pattern": str("A regular expression (Go syntax)."), "path": str("The folder or file to search; the workspace folder when empty."), "glob": str("Only files whose name matches this glob, such as *.md.")}, "pattern"),
		fn("run_command", "Run a command line in a shell ("+shellName()+") and give its output and exit code."+commandWarning,
			map[string]any{"command": str("The command line."), "cwd": str("The folder to run it in; the workspace folder when empty."), "timeout_seconds": num(fmt.Sprintf("How long it may run, %d by default and at most %d.", cmdDefault, cmdLongest))}, "command"),
	}
}

func shellName() string {
	if runtime.GOOS == "windows" {
		return "PowerShell"
	}
	return "sh"
}

// turnTools is what a turn needs to run the tools: the session, the host and
// the applet servers.
type turnTools struct {
	c        *Conn
	session  string
	cwd      string
	commands string          // when the person is asked (ApprovalsEvery, ApprovalsChat, ApprovalsNone)
	allowed  map[string]bool // what the person allowed, by key
	mu       *sync.Mutex
	refusal  *string   // a failed permission request stops further requests in this turn
	scope    FileScope // where the file tools may go; nil for anywhere (files.go)
}

func (t turnTools) path(p string) string {
	if p == "" {
		return t.cwd
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(t.cwd, p)
	}
	return filepath.Clean(p)
}

func (t turnTools) inside(p string) bool {
	rel, err := filepath.Rel(t.cwd, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

// title is how a tool call is shown in the chat.
func title(name string, args map[string]any) (string, string) {
	s := func(k string) string { v, _ := args[k].(string); return v }
	switch name {
	case "read_file":
		return "Read " + s("path"), "read"
	case "write_file":
		return "Write " + s("path"), "edit"
	case "list_directory":
		return "List " + s("path"), "read"
	case "find_files":
		return "Find " + s("pattern"), "search"
	case "search_text":
		return "Search " + s("pattern"), "search"
	case "run_command":
		return s("command"), "execute"
	}
	return strings.ReplaceAll(name, "__", ": "), "other"
}

// run carries out one tool call and returns its result for the model. An
// error the model can act on is a result, not a failure of the turn.
func (t turnTools) run(ctx context.Context, id, name string, args map[string]any) (string, bool) {
	s := func(k string) string { v, _ := args[k].(string); return v }
	n := func(k string) int { v, _ := args[k].(float64); return int(v) }
	switch name {
	case "read_file":
		return t.read(ctx, t.path(s("path")), n("offset"), n("limit"))
	case "write_file":
		return t.write(ctx, id, t.path(s("path")), s("content"))
	case "list_directory":
		p := t.path(s("path"))
		fsys, done, refused := t.files(p)
		if refused != "" {
			return refused, false
		}
		defer done()
		return listIn(fsys, p)
	case "find_files":
		p := t.path(s("path"))
		fsys, done, refused := t.files(p)
		if refused != "" {
			return refused, false
		}
		defer done()
		return findIn(ctx, fsys, p, s("pattern"))
	case "search_text":
		p := t.path(s("path"))
		fsys, done, refused := t.files(p)
		if refused != "" {
			return refused, false
		}
		defer done()
		return searchIn(ctx, fsys, p, s("pattern"), s("glob"))
	case "run_command":
		dir := t.path(s("cwd"))
		if refused := t.checkDir(dir); refused != "" {
			return refused, false
		}
		return t.command(ctx, id, s("command"), dir, n("timeout_seconds"))
	}
	if server, tool, ok := strings.Cut(name, "__"); ok {
		return t.c.callApplet(ctx, t, id, server, tool, args)
	}
	return "There is no tool named " + name + ".", false
}

// allow asks the person whether a tool call may go ahead, unless the answer
// they already gave covers it, which key records. ApprovalsNone asks nothing.
// The second result is the reason to give the model when it may not.
func (t turnTools) allow(ctx context.Context, id, key, heading string, input any) (bool, string) {
	if t.commands == ApprovalsNone || t.commands == ApprovalsAll {
		return true, ""
	}
	t.mu.Lock()
	refusal := ""
	if t.refusal != nil {
		refusal = *t.refusal
	}
	known := t.allowed[key]
	t.mu.Unlock()
	if known {
		return true, ""
	}
	if refusal != "" {
		return false, refusal
	}
	ok, why := t.c.ask(ctx, t.session, id, heading, input, true)
	if !ok {
		t.mu.Lock()
		if t.refusal != nil {
			*t.refusal = why
		}
		t.mu.Unlock()
		return false, why
	}
	if why == "always" {
		t.mu.Lock()
		t.allowed[key] = true
		t.mu.Unlock()
	}
	return true, ""
}

func (t turnTools) read(ctx context.Context, p string, offset, limit int) (string, bool) {
	fsys, done, refused := t.files(p)
	if refused != "" {
		return refused, false
	}
	defer done()
	if _, limited := fsys.(rootFiles); limited {
		return t.readLimited(fsys, p, offset, limit)
	}
	params := acp.ReadTextFileParams{SessionID: t.session, Path: p}
	if offset > 0 {
		params.Line = &offset
	}
	if limit > 0 {
		params.Limit = &limit
	}
	out, err := t.c.handler.Request(ctx, "fs/read_text_file", acp.Raw(params))
	if err != nil {
		return err.Message, false
	}
	var r acp.ReadTextFileResult
	if acp.Decode(out, &r) != nil {
		return "The file could not be read.", false
	}
	if len(r.Content) > maxRead {
		return r.Content[:maxRead] + fmt.Sprintf("\n\n[The file goes on; it was cut at %d KB. Read the rest with offset and limit.]", maxRead>>10), true
	}
	return r.Content, true
}

func (t turnTools) write(ctx context.Context, id, p, content string) (string, bool) {
	fsys, done, refused := t.files(p)
	if refused != "" {
		return refused, false
	}
	defer done()
	if _, limited := fsys.(rootFiles); limited {
		// The person has said where this chat may work, so a write inside is
		// not asked about and one outside was refused above.
		if err := fsys.writeFile(p, content); err != nil {
			return t.failed(err), false
		}
		return fmt.Sprintf("Wrote %s (%d bytes).", p, len(content)), true
	}
	if !t.inside(p) {
		if ok, why := t.allow(ctx, id, "write", "Write "+p, map[string]any{"path": p}); !ok {
			return why, false
		}
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err.Error(), false
	}
	if _, err := t.c.handler.Request(ctx, "fs/write_text_file", acp.Raw(acp.WriteTextFileParams{SessionID: t.session, Path: p, Content: content})); err != nil {
		return err.Message, false
	}
	return fmt.Sprintf("Wrote %s (%d bytes).", p, len(content)), true
}

func list(p string) (string, bool) { return listIn(osFiles{}, p) }

func listIn(fsys fileSystem, p string) (string, bool) {
	entries, err := fsys.readDir(p)
	if err != nil {
		return err.Error(), false
	}
	var b strings.Builder
	for _, e := range entries {
		b.WriteString(e.Name())
		if e.IsDir() {
			b.WriteString("/")
		}
		b.WriteString("\n")
	}
	if b.Len() == 0 {
		return "The folder is empty.", true
	}
	return b.String(), true
}

// skipped are folders a search passes over.
var skipped = map[string]bool{".git": true, "node_modules": true}

// stoppedSearch is what a search or a find that was stopped, or ran out of
// time, gives back: what it had found, if anything, and that it did not
// finish, so that the model does not take the lines for all there are, or
// the lack of them for no match.
func stoppedSearch(found []string) string {
	note := "The search was stopped before it finished, because the turn was stopped or the time allowed ran out."
	if len(found) == 0 {
		return note + " Nothing had been found yet. Search a narrower folder."
	}
	return strings.Join(found, "\n") + "\n[" + note + " These are only the ones found so far.]"
}

// globRe turns a glob with ** into a regular expression over slash paths.
func globRe(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	p := filepath.ToSlash(pattern)
	for i := 0; i < len(p); i++ {
		switch c := p[i]; c {
		case '*':
			if i+1 < len(p) && p[i+1] == '*' {
				i++
				if i+1 < len(p) && p[i+1] == '/' {
					i++
					b.WriteString("(?:.*/)?")
				} else {
					b.WriteString(".*")
				}
			} else {
				b.WriteString("[^/]*")
			}
		case '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	return regexp.Compile("(?i)" + b.String())
}

func find(ctx context.Context, root, pattern string) (string, bool) {
	return findIn(ctx, osFiles{}, root, pattern)
}

func findIn(ctx context.Context, fsys fileSystem, root, pattern string) (string, bool) {
	re, err := globRe(pattern)
	if err != nil {
		return "The pattern could not be read: " + err.Error(), false
	}
	var found []string
	var work searchWork
	err = fsys.walk(root, func(p string, d fs.DirEntry, err error) error {
		// A stopped or timed-out call ends the walk. Returning nil here, as
		// this once did, left the walk going through the whole tree.
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return nil
		}
		if d.IsDir() && skipped[d.Name()] {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			if work.files >= searchLimits.files {
				return errSearchBudget
			}
			work.files++
			work.last = p
		}
		rel, _ := filepath.Rel(root, p)
		if !d.IsDir() && re.MatchString(filepath.ToSlash(rel)) {
			found = append(found, filepath.ToSlash(rel))
			if len(found) >= maxFound {
				return fs.SkipAll
			}
		}
		return nil
	})
	if errors.Is(err, errSearchBudget) {
		sort.Strings(found)
		return strings.Join(append(found, "["+work.note(root)+"]"), "\n"), true
	}
	if err != nil {
		if ctx.Err() != nil {
			return stoppedSearch(found), false
		}
		return err.Error(), false
	}
	if len(found) == 0 {
		return "No file matches.", true
	}
	sort.Strings(found)
	out := strings.Join(found, "\n")
	if len(found) >= maxFound {
		out += fmt.Sprintf("\n[Only the first %d are listed; narrow the pattern.]", maxFound)
	}
	return out, true
}

func search(ctx context.Context, root, pattern, glob string) (string, bool) {
	return searchIn(ctx, osFiles{}, root, pattern, glob)
}

func searchIn(ctx context.Context, fsys fileSystem, root, pattern, glob string) (string, bool) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return "The pattern could not be read: " + err.Error(), false
	}
	var only *regexp.Regexp
	if glob != "" {
		if only, err = globRe(glob); err != nil {
			return "The glob could not be read: " + err.Error(), false
		}
	}
	var lines []string
	var work searchWork
	stop := errors.New("enough")
	walk := func(p string, d fs.DirEntry, err error) error {
		// A stopped or timed-out call ends the walk (see find).
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipped[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if work.files >= searchLimits.files {
			return errSearchBudget
		}
		work.files++
		work.last = p
		if only != nil && !only.MatchString(d.Name()) {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > 4<<20 {
			return nil
		}
		if work.bytes+int(info.Size()) > searchLimits.bytes {
			return errSearchBudget
		}
		b, err := fsys.readFile(p)
		if err != nil || !utf8ish(b) {
			return nil
		}
		work.bytes += len(b)
		for i, line := range strings.Split(string(b), "\n") {
			if re.MatchString(line) {
				rel, _ := filepath.Rel(root, p)
				if rel == "." {
					rel = filepath.Base(p)
				}
				if len(line) > 300 {
					line = line[:300] + "…"
				}
				lines = append(lines, fmt.Sprintf("%s:%d: %s", filepath.ToSlash(rel), i+1, strings.TrimRight(line, "\r")))
				if len(lines) >= maxMatches {
					return stop
				}
			}
		}
		return nil
	}
	if err := fsys.walk(root, walk); errors.Is(err, errSearchBudget) {
		return strings.Join(append(lines, "["+work.note(root)+"]"), "\n"), true
	} else if err != nil && !errors.Is(err, stop) {
		if ctx.Err() != nil {
			return stoppedSearch(lines), false
		}
		return err.Error(), false
	}
	if len(lines) == 0 {
		return "No line matches.", true
	}
	out := strings.Join(lines, "\n")
	if len(lines) >= maxMatches {
		out += fmt.Sprintf("\n[Only the first %d matches are listed; narrow the pattern.]", maxMatches)
	}
	return out, true
}

// utf8ish says whether a file looks like text: no NUL in its first 8 KB.
func utf8ish(b []byte) bool {
	n := len(b)
	if n > 8<<10 {
		n = 8 << 10
	}
	for _, c := range b[:n] {
		if c == 0 {
			return false
		}
	}
	return true
}

func (t turnTools) command(ctx context.Context, id, line, dir string, seconds int) (string, bool) {
	if strings.TrimSpace(line) == "" {
		return "Give the command to run.", false
	}
	if agentpolicy.ReservedCommand(line) {
		return profile.Current().Name + " does not let an agent read or set credentials or answer a run's approvals.", false
	}
	// Under ApprovalsEvery one answer covers the exact line, so any change to
	// it asks again; under ApprovalsChat it covers the chat's commands.
	key := "cmd\x00" + line
	if t.commands == ApprovalsChat {
		key = "cmd"
	}
	if ok, why := t.allow(ctx, id, key, line, map[string]any{"command": line, "cwd": dir}); !ok {
		return why, false
	}
	if seconds <= 0 {
		seconds = cmdDefault
	}
	if seconds > cmdLongest {
		seconds = cmdLongest
	}
	cctx, cancel := context.WithTimeout(ctx, time.Duration(seconds)*time.Second)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(cctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", line)
	} else {
		cmd = exec.CommandContext(cctx, "/bin/sh", "-c", line)
	}
	cmd.Dir = dir
	acp.HideWindow(cmd)
	acp.EndWhole(cmd)
	out := &tail{each: func(s string) {
		t.c.update(t.session, map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": id, "content": []any{map[string]any{"type": "content", "content": map[string]string{"type": "text", "text": s}}}})
	}}
	cmd.Stdout, cmd.Stderr = out, out
	err := cmd.Run()
	text := out.String()
	switch {
	case cctx.Err() == context.DeadlineExceeded:
		return text + fmt.Sprintf("\n[The command was stopped after %d seconds.]", seconds), false
	case ctx.Err() != nil:
		return text + "\n[The command was stopped.]", false
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return text + fmt.Sprintf("\n[Exit code %d.]", exit.ExitCode()), false
	}
	if err != nil {
		return "The command could not be run: " + err.Error(), false
	}
	return text + "\n[Exit code 0.]", true
}

// tail keeps the last maxOutput bytes a command wrote and passes each write
// on to the chat.
type tail struct {
	mu   sync.Mutex
	b    []byte
	each func(string)
}

func (t *tail) Write(p []byte) (int, error) {
	t.mu.Lock()
	t.b = append(t.b, p...)
	if len(t.b) > maxOutput {
		t.b = t.b[len(t.b)-maxOutput:]
	}
	t.mu.Unlock()
	if t.each != nil {
		t.each(string(p))
	}
	return len(p), nil
}

func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return strings.TrimRight(string(t.b), "\r\n")
}
