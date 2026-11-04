package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type frame struct {
	Thread   int    `json:"thread"`
	Number   string `json:"number"`
	Module   string `json:"module"`
	Function string `json:"function,omitempty"`
	Offset   string `json:"offset,omitempty"`
}

type stackResult struct {
	Outcome string  `json:"outcome"`
	Frames  []frame `json:"frames"`
}

type extractionReport struct {
	DebuggerFound bool          `json:"debuggerFound"`
	DebuggerProbe string        `json:"debuggerProbe"`
	SymbolLimit   string        `json:"symbolLimit"`
	Captures      []stackResult `json:"captures"`
	CleanupFailed bool          `json:"cleanupFailed"`
}

var threadLine = regexp.MustCompile(`^[ .#]*([0-9]+)\s+Id:\s+[0-9a-fA-F]+\.[0-9a-fA-F]+\s+Suspend:`)
var frameLine = regexp.MustCompile(`^\s*([0-9a-fA-F]{1,4})\s+([A-Za-z0-9_]+)(?:!([A-Za-z0-9_?$@:.~<>\[\]-]{1,256}))?(?:\+0x([0-9a-fA-F]{1,16}))?\s*$`)

// Only system and Node module names are published. Unknown modules, arguments,
// source paths, addresses, banners and other debugger text are discarded.
var modules = map[string]bool{
	"node": true, "ntdll": true, "kernel32": true, "kernelbase": true,
	"ucrtbase": true, "vcruntime140": true, "msvcp140": true,
	"advapi32": true, "sechost": true, "rpcrt4": true, "ws2_32": true,
	"win32u": true, "user32": true,
}

type frameCollector struct {
	sync.Mutex
	pending  []byte
	dropping bool
	thread   int
	frames   []frame
}

func (c *frameCollector) line() {
	line := strings.TrimSpace(string(c.pending))
	if match := threadLine.FindStringSubmatch(line); match != nil {
		c.thread, _ = strconv.Atoi(match[1])
		return
	}
	match := frameLine.FindStringSubmatch(line)
	if match == nil || c.thread < 0 || len(c.frames) >= 4096 || !modules[strings.ToLower(match[2])] {
		return
	}
	c.frames = append(c.frames, frame{c.thread, match[1], strings.ToLower(match[2]), match[3], match[4]})
}

func (c *frameCollector) Write(p []byte) (int, error) {
	c.Lock()
	defer c.Unlock()
	for _, b := range p {
		if b == '\n' {
			if !c.dropping {
				c.line()
			}
			c.pending, c.dropping = c.pending[:0], false
		} else if !c.dropping {
			if len(c.pending) == 4096 {
				c.pending, c.dropping = c.pending[:0], true
			} else {
				c.pending = append(c.pending, b)
			}
		}
	}
	return len(p), nil
}

func findCDB() string {
	if path, err := exec.LookPath("cdb.exe"); err == nil {
		return path
	}
	if root := os.Getenv("ProgramFiles(x86)"); root != "" {
		path := filepath.Join(root, "Windows Kits", "10", "Debuggers", "x64", "cdb.exe")
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}

func runCDB(ctx context.Context, debugger, dump, work string) stackResult {
	limit := 10 * time.Second
	if dump == "" {
		limit = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	// k c prints only module/function, n adds frame numbers, L suppresses
	// source lines, and 40 is a hexadecimal 64-frame limit per thread.
	symbols := "srv*" + filepath.Join(work, "symbols") + "*https://msdl.microsoft.com/download/symbols"
	arguments := []string{"-sins", "-y", symbols, "-z", dump, "-c", "~*kcnL 40;q"}
	if dump == "" {
		arguments = []string{"-version"}
	}
	command := exec.CommandContext(ctx, debugger, arguments...)
	command.Dir = work
	command.WaitDelay = time.Second
	collector := &frameCollector{thread: -1}
	command.Stdout, command.Stderr = collector, collector
	err := command.Run()
	collector.Lock()
	defer collector.Unlock()
	r := stackResult{Outcome: "frames-extracted", Frames: collector.frames}
	if ctx.Err() != nil {
		r.Outcome = "debugger-deadline"
	} else if err != nil {
		r.Outcome = "debugger-failed"
	} else if dump == "" {
		r.Outcome, r.Frames = "debugger-available", nil
	} else if len(r.Frames) == 0 {
		r.Outcome = "symbols-or-stack-format-unavailable"
	}
	return r
}

type debuggerRunner func(context.Context, string, string, string) stackResult

func extract(scratch, diagnostics, debugger string, run debuggerRunner) (resultErr error) {
	if !filepath.IsAbs(scratch) || !filepath.IsAbs(diagnostics) || !disjointDirectories(scratch, diagnostics) {
		return errors.New("invalid private scratch configuration")
	}
	r := extractionReport{
		DebuggerFound: debugger != "", DebuggerProbe: "debugger-unavailable", Captures: []stackResult{},
		SymbolLimit: "Microsoft public symbols configured; Node PDB resolution is not established. WCT remains separate evidence.",
	}
	root, err := os.OpenRoot(scratch)
	if err != nil {
		return errors.New("private scratch directory unavailable")
	}
	defer root.Close()
	var dumps []string
	var enumerationFailed bool
	err = fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			enumerationFailed = true
			return nil
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(name), ".dmp") {
			dumps = append(dumps, name)
		}
		return nil
	})
	if enumerationFailed {
		r.CleanupFailed = true
		err = errors.New("private scratch enumeration failed")
	}
	// Cleanup and the safe report run on every subsequent return, including
	// debugger failure, missing symbols and exhaustion of the shared deadline.
	defer func() {
		for _, name := range dumps {
			if err := root.Remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
				r.CleanupFailed = true
			}
		}
		if r.CleanupFailed {
			resultErr = errors.Join(resultErr, errors.New("raw dump cleanup failed"))
		}
		data, marshalErr := json.MarshalIndent(r, "", "  ")
		if marshalErr == nil {
			marshalErr = os.WriteFile(filepath.Join(diagnostics, "native-stack-frames.json"), append(data, '\n'), 0600)
		}
		resultErr = errors.Join(resultErr, marshalErr)
	}()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	work := ""
	if debugger != "" {
		work, err = os.MkdirTemp(scratch, "cdb-work-")
		if err != nil {
			return errors.New("debugger scratch creation failed")
		}
		defer func() {
			if filepath.Dir(work) != filepath.Clean(scratch) || !strings.HasPrefix(filepath.Base(work), "cdb-work-") {
				r.CleanupFailed = true
				resultErr = errors.Join(resultErr, errors.New("debugger cleanup boundary failed"))
				return
			}
			if err := os.RemoveAll(work); err != nil {
				r.CleanupFailed = true
				resultErr = errors.Join(resultErr, errors.New("debugger scratch cleanup failed"))
			}
		}()
		r.DebuggerProbe = run(ctx, debugger, "", work).Outcome
	}
	for _, name := range dumps {
		if r.DebuggerProbe != "debugger-available" {
			r.Captures = append(r.Captures, stackResult{Outcome: r.DebuggerProbe, Frames: []frame{}})
			continue
		}
		if ctx.Err() != nil {
			r.Captures = append(r.Captures, stackResult{Outcome: "extraction-budget-exhausted", Frames: []frame{}})
			continue
		}
		r.Captures = append(r.Captures, run(ctx, debugger, filepath.Join(scratch, filepath.FromSlash(name)), work))
	}
	return nil
}
