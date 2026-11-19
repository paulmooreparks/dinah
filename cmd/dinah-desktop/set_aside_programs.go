package main

import (
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

// The set-aside copies are beside the programs, in the program folder.
func setAsideService() (*programs.SetAsideService, error) {
	dir, err := programs.CurrentFolder()
	if err != nil {
		return nil, err
	}
	return programs.NewSetAsideService(dir, setAsideProgramNames(profile.Current())), nil
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
