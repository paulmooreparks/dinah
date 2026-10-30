package agents

import (
	"context"
	"debug/pe"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"studio/internal/acp"
	"studio/internal/profile"
)

// A file named devin on the search path is not always the Devin command line.
// On a machine where Devin Desktop is installed for every user, its own
// program, Devin.exe, can be on the search path, and an older command line can
// be left there by an earlier install. So every devin found is checked, and
// the first that is the Devin command line is used:
//
//   - On Windows it must be an .exe, since Devin documents the command line as
//     the devin binary, and a console program by its Portable Executable header
//     (https://learn.microsoft.com/windows/win32/debug/pe-format), which rules
//     out Devin Desktop itself without starting it.
//   - Started as `devin acp`, it must answer the Agent Client Protocol's
//     initialize request (https://agentclientprotocol.com/protocol/initialization)
//     with the protocol version Dinah speaks.
//
// When none passes, DevinMissing says why each was passed over and what to
// do: Devin documents Install Devin CLI in Devin Desktop's Command Palette as
// the way to put its bundled command line on the search path
// (https://docs.devin.ai/cli).

// DevinCandidate is one devin found, and what checking it showed.
type DevinCandidate struct {
	Path string `json:"path"`
	// Version is the version the agent reported, when it answered.
	Version string `json:"version,omitempty"`
	// Release is what `devin --version` printed, shown to the person when
	// several copies work so that a stale one can be told from the current
	// one; nothing is decided by it.
	Release string `json:"release,omitempty"`
	// Problem says why it is not used; it is empty for the one that works.
	Problem string `json:"problem,omitempty"`
}

// ProtocolVersion is the Agent Client Protocol version Dinah speaks.
const ProtocolVersion = 1

// probeTimeout bounds how long a candidate has to answer initialize.
var probeTimeout = 20 * time.Second

// probeACP starts a candidate as an agent and asks it to initialize; tests
// replace it.
var probeACP = func(path string) (acp.InitializeResult, error) {
	dir, err := os.MkdirTemp("", "dinah-probe-")
	if err != nil {
		return acp.InitializeResult{}, err
	}
	defer os.RemoveAll(dir)
	conn, err := acp.Start(path, []string{"acp"}, dir, probeHandler{}, acp.Options{})
	if err != nil {
		return acp.InitializeResult{}, err
	}
	defer func() {
		// The candidate is ended before its folder is removed.
		conn.Close()
		select {
		case <-conn.Done():
		case <-time.After(5 * time.Second):
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	var init acp.InitializeResult
	err = conn.Call(ctx, "initialize", acp.InitializeParams{
		ProtocolVersion:    ProtocolVersion,
		ClientCapabilities: acp.ClientCaps(),
		ClientInfo:         &acp.Implementation{Name: "dinah", Title: "Dinah", Version: "probe"},
	}, &init)
	if ctx.Err() != nil {
		return init, errors.New("it did not answer")
	}
	return init, err
}

// probeHandler answers nothing: initialize asks nothing of the client.
type probeHandler struct{}

func (probeHandler) Notify(string, json.RawMessage) {}
func (probeHandler) Request(context.Context, string, json.RawMessage) (any, *acp.Error) {
	return nil, &acp.Error{Code: acp.CodeMethodNotFound, Message: "not available while checking the command line"}
}

// checked keeps each candidate's result by its path, size and time, so that
// a file is started once, and again only when it changes.
var checked sync.Map

// checkingNow is the checks under way, by the key checked keeps them under.
var (
	checking    sync.Mutex
	checkingNow = map[string]chan struct{}{}
)

// FindDevin finds the Devin command line among the devin executables on the
// search path, checking every one. It returns the path of the one that
// passes, and every candidate it checked. When several pass, it returns "":
// one of them may be a stale copy that still answers, and which is current is
// the person's to say (DevinChoices), not something to guess from the order
// of the search path.
func FindDevin(workspaces []string) (string, []DevinCandidate) {
	var out []DevinCandidate
	for _, path := range executablesIn("devin", workspaces, searchPath()) {
		out = append(out, checkDevin(path))
	}
	if working := DevinChoices(out); len(working) == 1 {
		return working[0].Path, out
	}
	return "", out
}

// DevinChoices are the candidates that passed.
func DevinChoices(candidates []DevinCandidate) []DevinCandidate {
	var out []DevinCandidate
	for _, c := range candidates {
		if c.Problem == "" {
			out = append(out, c)
		}
	}
	return out
}

var releaseNumber = regexp.MustCompile(`\d+(?:\.\d+)+`)

// LikelyCurrent is the index of the choice whose release has the highest
// number, as a suggestion to the person, or -1 when the releases cannot be
// told apart. Devin does not document its numbering, so this only marks one
// choice; the person chooses.
func LikelyCurrent(choices []DevinCandidate) int {
	best, bestParts, tie := -1, []int(nil), false
	for i, c := range choices {
		m := releaseNumber.FindString(c.Release)
		if m == "" {
			return -1
		}
		var parts []int
		for _, s := range strings.Split(m, ".") {
			n, _ := strconv.Atoi(s)
			parts = append(parts, n)
		}
		switch cmp := compareParts(parts, bestParts); {
		case best < 0 || cmp > 0:
			best, bestParts, tie = i, parts, false
		case cmp == 0:
			tie = true
		}
	}
	if tie {
		return -1
	}
	return best
}

func compareParts(a, b []int) int {
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y int
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			if x > y {
				return 1
			}
			return -1
		}
	}
	return 0
}

// releaseOf is the first line `devin --version` prints, or "".
var releaseOf = func(path string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "--version")
	acp.HideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	first, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	return strings.TrimSpace(first)
}

func checkDevin(path string) DevinCandidate {
	key := path
	if st, err := os.Stat(path); err == nil {
		key = fmt.Sprintf("%s|%d|%d", path, st.Size(), st.ModTime().UnixNano())
	}
	if c, ok := checked.Load(key); ok {
		return c.(DevinCandidate)
	}
	// A check starts Devin, so callers that ask at once, as the page's
	// first calls do, wait for the one check under way.
	checking.Lock()
	wait, busy := checkingNow[key]
	if !busy {
		wait = make(chan struct{})
		checkingNow[key] = wait
	}
	checking.Unlock()
	if busy {
		<-wait
		if c, ok := checked.Load(key); ok {
			return c.(DevinCandidate)
		}
	} else {
		defer func() {
			checking.Lock()
			delete(checkingNow, key)
			checking.Unlock()
			close(wait)
		}()
	}
	c := DevinCandidate{Path: path}
	if runtime.GOOS == "windows" {
		if !strings.EqualFold(filepath.Ext(path), ".exe") {
			c.Problem = "it is not an .exe; the Devin command line is devin.exe"
		} else if console, err := consoleProgram(path); err != nil {
			c.Problem = "it could not be read as a program: " + err.Error()
		} else if !console {
			c.Problem = "it is a desktop program, not a command line; this is often Devin Desktop itself"
		}
	}
	if c.Problem == "" {
		init, err := probeACP(path)
		switch {
		case err != nil:
			c.Problem = "it did not answer as the Devin command line (" + err.Error() + "); it may be an older version without `devin acp`"
		case init.ProtocolVersion != ProtocolVersion:
			c.Problem = fmt.Sprintf("it speaks version %d of the agent protocol, and %s needs version %d", init.ProtocolVersion, profile.Current().Name, ProtocolVersion)
		}
		if init.AgentInfo != nil {
			c.Version = init.AgentInfo.Version
		}
		if c.Problem == "" {
			c.Release = releaseOf(path)
		}
	}
	checked.Store(key, c)
	return c
}

// consoleProgram reports whether a Windows executable is a console program
// rather than a desktop one, from the subsystem in its optional header.
func consoleProgram(path string) (bool, error) {
	f, err := pe.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	var subsystem uint16
	switch h := f.OptionalHeader.(type) {
	case *pe.OptionalHeader64:
		subsystem = h.Subsystem
	case *pe.OptionalHeader32:
		subsystem = h.Subsystem
	default:
		return false, errors.New("it has no optional header")
	}
	return subsystem == pe.IMAGE_SUBSYSTEM_WINDOWS_CUI, nil
}

// DevinMissing says, for a person, that no Devin command line was found or
// none worked, why each devin found was passed over, and what to do. desktop
// is the installed Devin Desktop's name and version, or "" when none is
// listed among the installed programs.
func DevinMissing(candidates []DevinCandidate, desktop string) string {
	var b strings.Builder
	name := profile.Current().Name
	if choices := DevinChoices(candidates); len(choices) > 1 {
		b.WriteString("More than one Devin command line works, and one may be a stale copy left by an earlier install. Choose the one " + name + " should use in Settings › Agents:")
		likely := LikelyCurrent(choices)
		for i, c := range choices {
			fmt.Fprintf(&b, " %s", c.Path)
			if c.Release != "" {
				fmt.Fprintf(&b, " (%s)", c.Release)
			}
			if i == likely {
				b.WriteString(", likely the current one")
			}
			b.WriteString(";")
		}
		return strings.TrimSuffix(b.String(), ";") + "."
	}
	if desktop != "" {
		fmt.Fprintf(&b, "%s is installed, but the Devin command line that %s needs is not on the search path. ", desktop, name)
		b.WriteString("In Devin Desktop, open the Command Palette (Ctrl+Shift+P), run Install Devin CLI, and restart " + name + ". ")
		b.WriteString("If Install Devin CLI is not listed, ask your Devin administrator to turn on \"Install Devin CLI in Devin Desktop\" in the team's Devin CLI settings. ")
	} else {
		b.WriteString("The Devin command line was not found. Install Devin Desktop and run Install Devin CLI from its Command Palette (Ctrl+Shift+P), or install the command line as https://docs.devin.ai/cli describes, and restart " + name + ". ")
	}
	b.WriteString("You can also choose the command line in Settings › Agents.")
	for _, c := range candidates {
		fmt.Fprintf(&b, " Passed over %s: %s.", c.Path, c.Problem)
	}
	return b.String()
}
