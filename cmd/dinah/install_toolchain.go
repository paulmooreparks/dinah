package main

import (
	"debug/buildinfo"
	"fmt"
)

type installProgram struct {
	source, destination string
}

// installPrograms checks the compiler identity in every artifact before
// replacing any installed program. ReadFile reads metadata without running it.
func installPrograms(programs []installProgram, required string) error {
	for _, program := range programs {
		info, err := buildinfo.ReadFile(program.source)
		if err != nil {
			return fmt.Errorf("reading compiler identity of %s: %w", program.source, err)
		}
		if info.GoVersion != required {
			return fmt.Errorf("refusing to install %s: built with %s; go.mod requires %s", program.source, info.GoVersion, required)
		}
	}
	for _, program := range programs {
		if err := place(program.source, program.destination); err != nil {
			return err
		}
		fmt.Printf("Installed %s.\n", program.destination)
	}
	return nil
}
