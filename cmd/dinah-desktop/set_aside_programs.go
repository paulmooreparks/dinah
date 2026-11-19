package main

import (
	"fmt"
	"os"
	"path/filepath"

	"studio/internal/layout"
	"studio/internal/profile"
	"studio/internal/programs"
)

// setAsideProgramNames are the programs whose set-aside copies the Updates
// page lists: the product's programs, and the names its command line and
// tool programs had in earlier releases, whose copies an earlier update set
// aside under those names. The service lists each name once.
func setAsideProgramNames(p profile.Profile) []string {
	names := []string{p.CommandProgram, p.WindowProgram}
	names = append(names, p.EarlierCommandPrograms...)
	for _, server := range p.ToolServers {
		names = append(names, server.Program)
		names = append(names, server.EarlierPrograms...)
	}
	for _, command := range p.ProgramCommands {
		names = append(names, command.Program)
	}
	return names
}

// The set-aside copies are beside the programs, in the program folder. They
// are those of every program an installation puts there, the stand-alone
// document applications among them: an installation sets their copies aside
// too, and until 20 November 2026 those were neither listed nor removable.
func setAsideService() (*programs.SetAsideService, error) {
	dir, err := programs.CurrentFolder()
	if err != nil {
		return nil, err
	}
	return programs.NewSetAsideService(dir, installedProgramNames(profile.Current())), nil
}

// sweepSetAside removes the copies installations set aside that may be
// removed (programs.SetAsideService.Sweep), and returns one line that says
// what it did, or "" when there was nothing to do.
func sweepSetAside() (string, error) {
	s, err := setAsideService()
	if err != nil {
		return "", err
	}
	r, err := s.Sweep()
	if err != nil {
		return "", err
	}
	if r.Removed == 0 && r.Refused == 0 {
		return "", nil
	}
	note := fmt.Sprintf("Removed %d older copies of the programs that earlier installations set aside (%s).", r.Removed, sizeWords(r.RemovedBytes))
	if left := r.InUse + r.Refused; left > 0 {
		note += fmt.Sprintf(" %d are still in use by running programs and are removed later.", left)
	}
	return note, nil
}

// sizeWords writes a number of bytes as megabytes or gigabytes.
func sizeWords(n int64) string {
	if n >= 1<<30 {
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	}
	return fmt.Sprintf("%d MB", n>>20)
}

// sweepSetAsideAtStart sweeps when the window starts, which is when the
// program an installation replaced has stopped and its copy may go. Only the
// installed window does it: a build run from somewhere else, as a check or a
// developer runs one, is not the program folder's keeper.
func sweepSetAsideAtStart() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	dir, err := programs.CurrentFolder()
	if err != nil || !layout.SamePath(filepath.Dir(exe), dir) {
		return
	}
	note, err := sweepSetAside()
	switch {
	case err != nil:
		logf("core", "warning", "the copies earlier installations set aside could not be removed: %v", err)
	case note != "":
		logf("core", "info", "%s", note)
	}
}

func (a *App) ListSetAsidePrograms() (programs.SetAsideList, error) {
	s, err := setAsideService()
	if err != nil {
		return programs.SetAsideList{}, err
	}
	return s.List()
}

func (a *App) RemoveSetAsidePrograms(in programs.RemoveSetAsideInput) (programs.RemoveSetAsideResult, error) {
	s, err := setAsideService()
	if err != nil {
		return programs.RemoveSetAsideResult{}, err
	}
	return s.Remove(in)
}
