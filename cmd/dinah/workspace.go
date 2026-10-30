package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"studio/internal/bus"
	"studio/internal/statefile"
	"studio/internal/workspaces"
)

// The workspace command reads the window's workspace registry and never
// changes it: it lists the workspaces and writes a workspace's export,
// which Import Workspace… in the window reads on another computer
// (docs/design/workspace-export.md). Importing is done in the window, since
// it asks the person about folders and changes the registry.
func workspaceCommand(args []string, stdout io.Writer) error {
	usage := errors.New("usage: " + product.CommandProgram + " workspace list | export WORKSPACE [--out FILE]")
	if len(args) == 0 {
		return usage
	}
	switch args[0] {
	case "list":
		if len(args) != 1 {
			return usage
		}
		records, err := loadWorkspaces()
		if err != nil {
			return err
		}
		type row struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			MainFolder string `json:"mainFolder"`
			Sources    int    `json:"sources"`
		}
		out := []row{}
		for _, record := range records {
			out = append(out, row{ID: record.ID, Name: record.Name, MainFolder: workspaces.ExecutionPath(record), Sources: len(record.Sources)})
		}
		return writeIndented(stdout, out)
	case "export":
		flags := flag.NewFlagSet("workspace export", flag.ContinueOnError)
		flags.SetOutput(io.Discard)
		to := flags.String("out", "", "the file to write, ending in "+workspaces.ExportExtension)
		// The workspace may come before or after --out.
		rest := []string{}
		for list := args[1:]; ; {
			if err := flags.Parse(list); err != nil {
				return usage
			}
			if flags.NArg() == 0 {
				break
			}
			rest = append(rest, flags.Arg(0))
			list = flags.Args()[1:]
		}
		if len(rest) != 1 {
			return usage
		}
		records, err := loadWorkspaces()
		if err != nil {
			return err
		}
		record, err := findWorkspace(records, rest[0])
		if err != nil {
			return err
		}
		file := ""
		if *to != "" {
			if file, err = filepath.Abs(*to); err != nil {
				return err
			}
		}
		home, _ := os.UserHomeDir()
		data, err := json.MarshalIndent(workspaces.BuildExport(record, file, home), "", "  ")
		if err != nil {
			return err
		}
		data = append(data, '\n')
		// Without --out the export is printed, and its local folders can
		// then be found on the other computer only from the home folder or
		// in full.
		if file == "" {
			_, err = stdout.Write(data)
			return err
		}
		if err := statefile.Write(file, data); err != nil {
			return err
		}
		_, err = fmt.Fprintln(stdout, file)
		return err
	}
	return usage
}

// writeIndented prints a value as indented JSON.
func writeIndented(w io.Writer, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = w.Write(append(data, '\n'))
	return err
}

// loadWorkspaces reads the registry from the home folder the window uses.
// A registry that does not exist yet has not been moved out of the window's
// settings, which is different from one with no workspaces, so it is said
// to be so and not listed as empty.
func loadWorkspaces() ([]workspaces.Record, error) {
	home, err := bus.Home()
	if err != nil {
		return nil, err
	}
	records, err := workspaces.Load(home)
	if errors.Is(err, workspaces.ErrNoRegistry) {
		return nil, errors.New("the workspace registry has not been moved to its own file yet; start " + product.Name + " once, which moves it, and run this again")
	}
	return records, err
}

// findWorkspace finds a workspace by its ID, its main folder or its name,
// a name matching regardless of case and only when one workspace has it.
func findWorkspace(records []workspaces.Record, which string) (workspaces.Record, error) {
	for _, record := range records {
		if record.ID == which {
			return record, nil
		}
	}
	// A path is told from a name by its separator.
	if strings.ContainsAny(which, `/\`) {
		if abs, err := filepath.Abs(which); err == nil {
			if i := workspaces.IndexAt(records, abs); i >= 0 {
				return records[i], nil
			}
		}
	}
	var found []workspaces.Record
	for _, record := range records {
		if strings.EqualFold(record.Name, which) {
			found = append(found, record)
		}
	}
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return workspaces.Record{}, fmt.Errorf("no workspace has the ID, name or main folder %q; %s workspace list shows them", which, product.CommandProgram)
	}
	return workspaces.Record{}, errors.New("more than one workspace has that name; give its ID or main folder instead")
}
