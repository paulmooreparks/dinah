//go:build js && wasm

// Command shwasm is the POSIX shell of Dinah's Terminal: the interpreter of
// mvdan.cc/sh, built for WebAssembly and run in a worker of its own
// (terminal-sh-worker.js).
//
// The interpreter runs the shell language itself: variables, quoting,
// expansion, if, for, while, case, functions, pipelines, redirection, $( )
// and arithmetic, and its own builtins such as cd, export and test. Every
// command that is not its own is a request to the Terminal, which runs it as
// it runs a command typed at the prompt; every file it opens, lists or
// looks at is in Dinah's file system, through the Terminal too.
//
// It sets globalThis.dinahShRun(request, host, write), which runs a script and
// returns a promise of its status, its exported variables and its folder.
// request has source, name, args, env (an object of the variables), cwd and
// stdin (text, or null when the script reads from no pipe). host(op, args)
// returns a promise of the Terminal's answer; write(stream, text) writes to
// "out" or "err".
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"syscall/js"
	"time"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

func main() {
	js.Global().Set("dinahShRun", js.FuncOf(run))
	select {}
}

// run starts a script in a goroutine of its own, since waiting on the
// Terminal's answers may not happen inside a callback from JavaScript.
func run(_ js.Value, a []js.Value) any {
	request, host, write := a[0], a[1], a[2]
	var resolve js.Value
	promise := js.Global().Get("Promise").New(js.FuncOf(func(_ js.Value, p []js.Value) any {
		resolve = p[0]
		return nil
	}))
	go func() {
		status, env, dir := execute(request, &bridge{host: host, write: write})
		vars := js.Global().Get("Object").New()
		for k, v := range env {
			vars.Set(k, v)
		}
		result := js.Global().Get("Object").New()
		result.Set("status", status)
		result.Set("env", vars)
		result.Set("cwd", dir)
		resolve.Invoke(result)
	}()
	return promise
}

// bridge is the way to the Terminal.
type bridge struct {
	host, write js.Value
	stdout      *stream
	stdin       io.Reader
}

// call asks the Terminal and waits for its answer, which is text.
func (b *bridge) call(op string, args ...any) (string, error) {
	list := js.Global().Get("Array").New()
	for _, a := range args {
		list.Call("push", js.ValueOf(a))
	}
	return await(b.host.Invoke(op, list))
}

func await(p js.Value) (string, error) {
	type result struct {
		value js.Value
		err   error
	}
	done := make(chan result, 1)
	ok := js.FuncOf(func(_ js.Value, a []js.Value) any {
		v := js.Undefined()
		if len(a) > 0 {
			v = a[0]
		}
		done <- result{value: v}
		return nil
	})
	fail := js.FuncOf(func(_ js.Value, a []js.Value) any {
		msg := "the Terminal refused the request"
		if len(a) > 0 {
			if m := a[0].Get("message"); m.Type() == js.TypeString {
				msg = m.String()
			} else {
				msg = a[0].String()
			}
		}
		done <- result{err: errors.New(msg)}
		return nil
	})
	p.Call("then", ok, fail)
	r := <-done
	ok.Release()
	fail.Release()
	if r.err != nil {
		return "", r.err
	}
	switch r.value.Type() {
	case js.TypeString:
		return r.value.String(), nil
	case js.TypeNumber:
		return fmt.Sprint(r.value.Int()), nil
	case js.TypeBoolean:
		return fmt.Sprint(r.value.Bool()), nil
	}
	return "", nil
}

// stream is the script's output or error output, written to the Terminal as
// it comes.
type stream struct {
	b    *bridge
	name string
}

func (s *stream) Write(p []byte) (int, error) {
	s.b.write.Invoke(s.name, string(p))
	return len(p), nil
}

func execute(request js.Value, b *bridge) (int, map[string]string, string) {
	errOut := &stream{b: b, name: "err"}
	b.stdout = &stream{b: b, name: "out"}
	name := request.Get("name").String()
	fail := func(format string, a ...any) (int, map[string]string, string) {
		fmt.Fprintf(errOut, name+": "+format+"\n", a...)
		return 2, nil, ""
	}
	file, err := syntax.NewParser().Parse(strings.NewReader(request.Get("source").String()), name)
	if err != nil {
		return fail("%v", err)
	}
	env := map[string]string{}
	vars := request.Get("env")
	keys := js.Global().Get("Object").Call("keys", vars)
	pairs := []string{}
	for i := 0; i < keys.Length(); i++ {
		k := keys.Index(i).String()
		env[k] = vars.Get(k).String()
		pairs = append(pairs, k+"="+env[k])
	}
	var stdin io.Reader
	if s := request.Get("stdin"); s.Type() == js.TypeString {
		stdin = strings.NewReader(s.String())
		b.stdin = stdin
	}
	args := []string{"--"}
	list := request.Get("args")
	for i := 0; i < list.Length(); i++ {
		args = append(args, list.Index(i).String())
	}
	dir := request.Get("cwd").String()
	r, err := interp.New(
		interp.Env(expand.ListEnviron(pairs...)),
		// The folder is in Dinah's file system, which the process
		// cannot stat, so it is set as it is.
		func(r *interp.Runner) error { r.Dir = dir; return nil },
		interp.Params(args...),
		interp.StdIO(stdin, b.stdout, errOut),
		interp.ExecHandlers(func(interp.ExecHandlerFunc) interp.ExecHandlerFunc { return b.exec }),
		interp.OpenHandler(b.open),
		interp.StatHandler(b.stat),
		interp.ReadDirHandler2(b.readDir),
	)
	if err != nil {
		return fail("%v", err)
	}
	err = r.Run(context.Background(), file)
	status := 0
	if err != nil {
		var code interp.ExitStatus
		if errors.As(err, &code) {
			status = int(code)
		} else {
			fmt.Fprintf(errOut, "%s: %v\n", name, err)
			status = 1
		}
	}
	// What the script exported, for a script that is sourced, such as the
	// Terminal's startup script.
	for k, v := range r.Vars {
		if !v.IsSet() {
			delete(env, k)
		} else if v.Exported {
			env[k] = v.String()
		}
	}
	return status, env, r.Dir
}

// exec runs a command that is not the interpreter's own through the
// Terminal. Its output goes straight to the screen when it is the script's
// own output, so a command that asks or shows a page works there, and is
// collected otherwise, for a pipe, a redirection or $( ).
func (b *bridge) exec(ctx context.Context, args []string) error {
	hc := interp.HandlerCtx(ctx)
	direct := hc.Stdout == io.Writer(b.stdout)
	// A command reads what is piped into it, or what was piped into the
	// script; with neither, it reads nothing.
	var stdin any
	if hc.Stdin != nil {
		data, err := io.ReadAll(hc.Stdin)
		if err != nil {
			return err
		}
		stdin = string(data)
	}
	exported := map[string]string{}
	hc.Env.Each(func(name string, vr expand.Variable) bool {
		if vr.Exported && vr.IsSet() {
			exported[name] = vr.String()
		}
		return true
	})
	words, _ := json.Marshal(args)
	vars, _ := json.Marshal(exported)
	answer, err := b.call("execv", string(words), stdin, direct, string(vars), hc.Dir)
	if err != nil {
		fmt.Fprintf(hc.Stderr, "%s: %v\n", args[0], err)
		return interp.ExitStatus(1)
	}
	var result struct {
		Status int    `json:"status"`
		Out    string `json:"out"`
	}
	if err := json.Unmarshal([]byte(answer), &result); err != nil {
		return err
	}
	if !direct && result.Out != "" {
		io.WriteString(hc.Stdout, result.Out)
	}
	if result.Status != 0 {
		return interp.ExitStatus(uint8(result.Status))
	}
	return nil
}

// entry is a file or folder as the Terminal describes it.
type entry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Directory bool   `json:"directory"`
	Size      int64  `json:"size"`
	Modified  string `json:"modified"`
	Writable  bool   `json:"writable"`
}

func (e entry) info() fs.FileInfo { return fileInfo{e} }

type fileInfo struct{ e entry }

func (f fileInfo) Name() string { return f.e.Name }
func (f fileInfo) Size() int64  { return f.e.Size }

// Mode gives every file and folder the execute bit, since any script can be
// run by its path, and the write bits only where the Terminal can write.
func (f fileInfo) Mode() fs.FileMode {
	m := fs.FileMode(0o555)
	if f.e.Writable {
		m |= 0o200
	}
	if f.e.Directory {
		m |= fs.ModeDir
	}
	return m
}
func (f fileInfo) ModTime() time.Time {
	t, _ := time.Parse(time.RFC3339, f.e.Modified)
	return t
}
func (f fileInfo) IsDir() bool { return f.e.Directory }
func (f fileInfo) Sys() any    { return nil }

func notExist(op, name string) error {
	return &fs.PathError{Op: op, Path: name, Err: fs.ErrNotExist}
}

// pathError turns the Terminal's refusal into an error the interpreter says
// the usual way.
func pathError(op, name string, err error) error {
	if strings.Contains(err.Error(), "no such file") {
		return notExist(op, name)
	}
	return &fs.PathError{Op: op, Path: name, Err: err}
}

func (b *bridge) stat(ctx context.Context, name string, _ bool) (fs.FileInfo, error) {
	answer, err := b.call("stat", name)
	if err != nil {
		return nil, pathError("stat", name, err)
	}
	if answer == "" {
		return nil, notExist("stat", name)
	}
	var e entry
	if err := json.Unmarshal([]byte(answer), &e); err != nil {
		return nil, err
	}
	return e.info(), nil
}

func (b *bridge) readDir(ctx context.Context, name string) ([]fs.DirEntry, error) {
	answer, err := b.call("list", name)
	if err != nil {
		return nil, pathError("readdir", name, err)
	}
	var list []entry
	if err := json.Unmarshal([]byte(answer), &list); err != nil {
		return nil, err
	}
	out := make([]fs.DirEntry, 0, len(list))
	for _, e := range list {
		out = append(out, fs.FileInfoToDirEntry(e.info()))
	}
	return out, nil
}

// open reads a file whole, or collects what is written to it and saves it
// when it is closed.
func (b *bridge) open(ctx context.Context, name string, flag int, _ os.FileMode) (io.ReadWriteCloser, error) {
	if name == "/dev/null" {
		return devNull{}, nil
	}
	if !path.IsAbs(name) {
		name = path.Join(interp.HandlerCtx(ctx).Dir, name)
	}
	if flag&(os.O_WRONLY|os.O_RDWR) == 0 {
		text, err := b.call("read", name)
		if err != nil {
			return nil, pathError("open", name, err)
		}
		return &readFile{Reader: strings.NewReader(text)}, nil
	}
	mode := "replace"
	if flag&os.O_APPEND != 0 {
		mode = "append"
	}
	return &writeFile{b: b, name: name, mode: mode}, nil
}

type readFile struct{ *strings.Reader }

func (readFile) Write([]byte) (int, error) { return 0, errors.New("the file is open for reading") }
func (readFile) Close() error              { return nil }

type writeFile struct {
	b      *bridge
	name   string
	mode   string
	buf    bytes.Buffer
	closed bool
}

func (w *writeFile) Read([]byte) (int, error)    { return 0, errors.New("the file is open for writing") }
func (w *writeFile) Write(p []byte) (int, error) { return w.buf.Write(p) }
func (w *writeFile) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	if _, err := w.b.call("save", w.name, w.buf.String(), w.mode); err != nil {
		return pathError("write", w.name, err)
	}
	return nil
}

type devNull struct{}

func (devNull) Read([]byte) (int, error)    { return 0, io.EOF }
func (devNull) Write(p []byte) (int, error) { return len(p), nil }
func (devNull) Close() error                { return nil }
