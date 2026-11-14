package main

import (
	"path/filepath"

	"studio/internal/profile"
	"studio/internal/programs"
)

func setAsideProgramNames(p profile.Profile) []string {
	names := []string{p.CommandProgram, p.WindowProgram}
	for _, server := range p.ToolServers {
		names = append(names, server.Program)
	}
	for _, command := range p.ProgramCommands {
		names = append(names, command.Program)
	}
	return names
}

func (a *App) ListSetAsidePrograms() (programs.SetAsideList, error) {
	return programs.NewSetAsideService(filepath.Join(configDir(), "bin"), setAsideProgramNames(profile.Current())).List()
}

func (a *App) RemoveSetAsidePrograms(in programs.RemoveSetAsideInput) (programs.RemoveSetAsideResult, error) {
	return programs.NewSetAsideService(filepath.Join(configDir(), "bin"), setAsideProgramNames(profile.Current())).Remove(in)
}
