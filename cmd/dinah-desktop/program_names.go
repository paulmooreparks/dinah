package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"studio/internal/acp"
	"studio/internal/layout"
	"studio/internal/profile"
	"studio/internal/programs"
)

/* Moving an installation to the names this release gives its programs.

   A product may rename its command line (profile.EarlierCommandPrograms) and
   its tool programs (profile.ToolServer.EarlierPrograms), and may give one
   program's earlier name to another. An update made by an earlier release
   installs the new programs under the earlier names, since that is all the
   earlier release knows. The window, on its first start from the product's
   bin folder, moves them to their current names:

     1. Each tool program found under an earlier name is copied to its
        current name.
     2. The command line found under an earlier name is placed under its
        current name. If a tool program has that name now, it is set aside
        as every replaced program is (programs.Place), so a running copy
        keeps its file.
     3. Once the command line is in place, the Devin hooks the window can
        rewrite are brought to the current name, and the file of the command
        line's earlier name is removed. A hooks file the window cannot
        rewrite, or an entry a person wrote by hand, does not keep the file;
        such an entry stops working (Paul, 18 November 2026).

   A file is moved only when it says, through programs.IdentityCommand, that
   it is the program wanted, of this product and of the window's own release.
   A program built before that command existed cannot say so, and is never
   moved: an earlier build is never placed under a current name, and a file
   name or size is never taken as an answer. Dinah renamed nothing, and none
   of this runs for it. */

// renamesPrograms reports whether a product gave any of its programs an
// earlier name.
func renamesPrograms(p profile.Profile) bool {
	if len(p.EarlierCommandPrograms) > 0 {
		return true
	}
	for _, s := range p.ToolServers {
		if len(s.EarlierPrograms) > 0 {
			return true
		}
	}
	return false
}

// programNameMover moves the programs in one bin folder.
type programNameMover struct {
	profile profile.Profile
	bin     string
	// version is the window's own release, the one every program moved
	// must report.
	version string
	// identify asks a program what it is, place puts a copy of a program
	// under another name, setting aside what was there, and remove removes
	// a file. A test replaces them.
	identify func(path string) (programs.Identity, error)
	place    func(src, dst string) error
	remove   func(path string) error
	// rewriteHooks brings the hooks the window knows of that run one of the
	// earlier command program's commands to the current name, and returns
	// the hooks files that still run one, which it could not rewrite.
	rewriteHooks func(commands []string) []string
	// notes are what was done and what was left, for the log.
	notes []string
}

func (m *programNameMover) path(name string) string {
	return filepath.Join(m.bin, programs.FileName(name))
}

func exists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && st.Mode().IsRegular()
}

// is reports whether the file at path is program, of this product, at the
// window's release.
func (m *programNameMover) is(path, program string) bool {
	if !exists(path) {
		return false
	}
	id, err := m.identify(path)
	return err == nil && id.Product == m.profile.ID && id.Program == program && id.Version == m.version
}

func (m *programNameMover) note(format string, args ...any) {
	m.notes = append(m.notes, fmt.Sprintf(format, args...))
}

// settled reports, from the folder alone, that there is nothing to move:
// no file has an earlier name of the command line, and every tool program
// with an earlier name is in its current place. An installation that is
// moved already starts no program to ask what it is, so the check costs a
// start nothing.
func (m *programNameMover) settled() bool {
	for _, earlier := range m.profile.EarlierCommandPrograms {
		if exists(m.path(earlier)) {
			return false
		}
	}
	for _, s := range m.profile.ToolServers {
		if len(s.EarlierPrograms) > 0 && !exists(m.path(s.Program)) {
			return false
		}
	}
	return true
}

// run moves the programs and returns what it did.
func (m *programNameMover) run() []string {
	if m.settled() {
		return nil
	}
	m.moveToolPrograms()
	m.moveCommandProgram()
	m.removeEarlierCommandPrograms()
	return m.notes
}

func (m *programNameMover) moveToolPrograms() {
	for _, s := range m.profile.ToolServers {
		dst := m.path(s.Program)
		if len(s.EarlierPrograms) == 0 || m.is(dst, s.Program) {
			continue
		}
		for _, earlier := range s.EarlierPrograms {
			src := m.path(earlier)
			if !m.is(src, s.Program) {
				continue
			}
			if err := m.place(src, dst); err != nil {
				m.note("%s was not copied to %s: %v", src, dst, err)
			} else {
				m.note("copied %s, which is %s, to %s", src, s.Program, dst)
			}
			break
		}
	}
}

// freeForCommand reports whether the file at the command program's current
// name may be replaced: there is none, it is the command program of another
// release, or it is a tool program that is in its own current place now.
// Anything else, such as a build that cannot say what it is, is kept.
func (m *programNameMover) freeForCommand(dst string) bool {
	if !exists(dst) {
		return true
	}
	id, err := m.identify(dst)
	if err != nil || id.Product != m.profile.ID {
		return false
	}
	if id.Program == m.profile.CommandProgram {
		return true
	}
	for _, s := range m.profile.ToolServers {
		if id.Program == s.Program && m.is(m.path(s.Program), s.Program) {
			return true
		}
	}
	return false
}

func (m *programNameMover) moveCommandProgram() {
	dst := m.path(m.profile.CommandProgram)
	if len(m.profile.EarlierCommandPrograms) == 0 || m.is(dst, m.profile.CommandProgram) {
		return
	}
	for _, earlier := range m.profile.EarlierCommandPrograms {
		src := m.path(earlier)
		if !exists(src) {
			continue
		}
		if !m.is(src, m.profile.CommandProgram) {
			m.note("%s is not the command line of release %s, so it was not placed as %s", src, m.version, dst)
			continue
		}
		if !m.freeForCommand(dst) {
			m.note("%s was not placed as %s, since the program there is not one this window can set aside safely", src, dst)
			return
		}
		if err := m.place(src, dst); err != nil {
			m.note("%s was not placed as %s: %v", src, dst, err)
		} else {
			m.note("placed %s, which is the command line, as %s", src, dst)
		}
		return
	}
}

func (m *programNameMover) removeEarlierCommandPrograms() {
	if len(m.profile.EarlierCommandPrograms) == 0 || !m.is(m.path(m.profile.CommandProgram), m.profile.CommandProgram) {
		return
	}
	var present, commands []string
	for _, earlier := range m.profile.EarlierCommandPrograms {
		if exists(m.path(earlier)) {
			present = append(present, m.path(earlier))
			commands = append(commands, earlier+" steering-hook")
		}
	}
	if len(present) == 0 {
		return
	}
	// The file is removed whatever is left: a hooks file that could not be
	// rewritten, or an entry a person wrote by hand, stops working.
	if left := m.rewriteHooks(commands); len(left) > 0 {
		m.note("the Devin hooks in %s still run %s, which is removed, so they stop working", strings.Join(left, " and "), strings.Join(present, " and "))
	}
	for _, path := range present {
		if err := m.remove(path); err != nil {
			m.note("%s was not removed, and is tried again at the next start: %v", path, err)
		} else {
			m.note("removed %s", path)
		}
	}
}

// identifyProgram runs a program with programs.IdentityCommand, hidden, and
// reads what it says.
func identifyProgram(path string) (programs.Identity, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, programs.IdentityCommand)
	acp.HideWindow(cmd)
	out, err := cmd.Output()
	if err != nil {
		return programs.Identity{}, err
	}
	return programs.ParseIdentity(string(out))
}

// moveProgramsToCurrentNames runs the move for the installed window, before
// any chat starts, since a chat's start needs the command line under its
// current name (devin_steering.go). A window started from anywhere else
// than the product's bin folder, such as a build in a source folder, moves
// nothing.
func (a *App) moveProgramsToCurrentNames() {
	p := profile.Current()
	if !renamesPrograms(p) {
		return
	}
	self, err := os.Executable()
	bin := filepath.Join(configDir(), "bin")
	if err != nil || !layout.SamePath(filepath.Dir(self), bin) {
		return
	}
	m := &programNameMover{profile: p, bin: bin, version: appVersion,
		identify: identifyProgram, place: programs.Place, remove: os.Remove,
		rewriteHooks: func(commands []string) []string { return rewriteEarlierSteering(a.knownChatFolders(), commands) }}
	notes := m.run()
	for _, note := range notes {
		logf("programs", "info", "%s", note)
	}
	if len(notes) == 0 {
		return
	}
	// Devin's own settings are brought to the tool programs' current names
	// too; a chat's start does the same (devinPrepareApplets).
	ensureToolServerRegistrations()
}

// knownChatFolders are the folders the window knows chats in: its
// workspaces' and those of the chats it keeps a saved copy of.
func (a *App) knownChatFolders() []string {
	seen := map[string]bool{}
	var out []string
	add := func(folder string) {
		if folder == "" {
			return
		}
		key := strings.ToLower(filepath.Clean(folder))
		if !seen[key] {
			seen[key] = true
			out = append(out, folder)
		}
	}
	a.mu.Lock()
	for _, w := range a.settings.WorkspaceRecords {
		add(w.ExecutionPath)
	}
	a.mu.Unlock()
	_, records := conversationLinks()
	for _, r := range records {
		add(r.Workspace)
	}
	return out
}

// rewriteEarlierSteering brings each folder's Devin hooks that run one of
// the earlier commands to the current command, and returns the hooks files
// that still run one afterwards or could not be read or written. A hook a
// person added with a matcher of their own is theirs and is not changed.
func rewriteEarlierSteering(folders, commands []string) []string {
	var left []string
	for _, folder := range folders {
		path := filepath.Join(folder, ".devin", "hooks.v1.json")
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			continue
		}
		uses, err := devinHooksRunAny(path, commands)
		if err == nil && uses {
			if err = registerDevinSteeringHooks(path); err == nil {
				logf("programs", "info", "the Devin hooks in %s now run %s", path, devinSteeringCommand)
				uses, err = devinHooksRunAny(path, commands)
			}
		}
		if err != nil {
			logf("programs", "warn", "the Devin hooks in %s could not be rewritten: %v", path, err)
			left = append(left, path)
			continue
		}
		if uses {
			left = append(left, path)
		}
	}
	return left
}
