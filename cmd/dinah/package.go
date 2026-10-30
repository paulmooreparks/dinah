package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"studio/internal/bus"
	"studio/internal/mcpserver"
	"studio/internal/setup"
)

// The package command installs, lists and removes content packages, as
// Settings › Packages does in the window (docs/design/packages.md). The
// installer is internal/setup, which the window uses too.
func packageCommand(args []string, stdout io.Writer) error {
	usage := errors.New("usage: " + product.CommandProgram + " package install ZIP [--personal] [--workspace DIR] [--backup-dir DIR] [--dry-run] [--json] [--state-dir DIR]\n" +
		"       " + product.CommandProgram + " package status [--workspace DIR] [--state-dir DIR]\n" +
		"       " + product.CommandProgram + " package remove [--personal [--name NAME]] [--workspace DIR] [--backup-dir DIR] [--dry-run] [--state-dir DIR]")
	if len(args) == 0 {
		return usage
	}
	flags := flag.NewFlagSet("package "+args[0], flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	workspace := flags.String("workspace", ".", "the workspace's main folder")
	backup := flags.String("backup-dir", "", "where to back up replaced and removed files (default: setup-backups in your home folder)")
	stateDir := flags.String("state-dir", "", "a state folder to use instead of the one the window uses, for tests and trial runs; your own skills folder is then inside it too")
	dryRun := flags.Bool("dry-run", false, "print the plan and write nothing")
	asJSON := flags.Bool("json", false, "print the plan as JSON")
	personal := flags.Bool("personal", false, "for you, in every workspace, instead of in one workspace")
	name := flags.String("name", "", "with --personal, the package to remove")
	// Flags may come before or after the ZIP.
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
	state, isolated := *stateDir, *stateDir != ""
	if !isolated {
		home, err := bus.Home()
		if err != nil {
			return err
		}
		state = home
	}
	targets := func() (setup.PersonalTargets, error) { return setup.Targets(state, isolated) }
	ws, err := filepath.Abs(*workspace)
	if err != nil {
		return err
	}
	switch args[0] {
	case "install":
		if len(rest) != 1 {
			return usage
		}
		if *personal {
			t, err := targets()
			if err != nil {
				return err
			}
			return installPersonal(stdout, rest[0], t, *backup, mcpserver.Version, *dryRun, *asJSON, time.Now())
		}
		return installInWorkspace(stdout, rest[0], ws, *backup, mcpserver.Version, *dryRun, *asJSON, time.Now())
	case "status":
		if len(rest) != 0 {
			return usage
		}
		t, err := targets()
		if err != nil {
			return err
		}
		return packageStatus(stdout, t, ws)
	case "remove":
		if len(rest) != 0 {
			return usage
		}
		return removePackage(stdout, *personal, *name, ws, *backup, *dryRun, targets)
	}
	return usage
}

const changedNote = "  (changed since it was installed; your copy is in the backup)"

// installInWorkspace installs or updates a package in a workspace's main
// folder.
func installInWorkspace(out io.Writer, zipPath, workspace, backup, version string, dryRun, asJSON bool, now time.Time) error {
	plan, err := setup.PlanApply(zipPath, workspace, version)
	if err != nil {
		return err
	}
	if !dryRun {
		if backup == "" {
			if backup, err = setup.DefaultBackup(workspace, plan.From, now); err != nil {
				return err
			}
		}
		if err := setup.Apply(plan, backup); err != nil {
			return err
		}
	}
	if asJSON {
		return writeIndented(out, struct {
			DryRun bool `json:"dry_run"`
			*setup.Plan
		}{dryRun, plan})
	}
	fmt.Fprintf(out, "%s: %s, installed version %s, package version %s\n", verb(dryRun, "INSTALLED"), plan.Name, orNone(plan.From), plan.To)
	c := plan.Counts()
	fmt.Fprintf(out, "  %d added, %d updated, %d removed, %d unchanged\n", c[setup.Add], c[setup.Update], c[setup.Remove], c[setup.Unchanged])
	for _, a := range plan.Actions {
		if a.Kind == setup.Unchanged {
			continue
		}
		note := ""
		switch {
		case a.Modified:
			note = changedNote
		case a.Unknown:
			note = "  (no installed record to compare with; your copy is in the backup)"
		}
		fmt.Fprintf(out, "  %-9s %s%s\n", a.Kind, a.Path, note)
	}
	for _, w := range plan.Warnings {
		fmt.Fprintf(out, "WARNING: %s\n", w)
	}
	if !dryRun && plan.Backup != "" {
		fmt.Fprintf(out, "backup: %s\n", plan.Backup)
	}
	return nil
}

// installPersonal installs or updates a package for the person: it is
// unpacked into the packages folder, and the items of each kind with a
// personal folder are copied there.
func installPersonal(out io.Writer, zipPath string, t setup.PersonalTargets, backup, version string, dryRun, asJSON bool, now time.Time) error {
	plan, err := setup.PlanPersonal(zipPath, t, version)
	if err != nil {
		return err
	}
	if len(plan.Conflicts) > 0 && !dryRun {
		return fmt.Errorf("%s did not install these, and will not overwrite them: %s", product.Name, strings.Join(plan.Conflicts, "; "))
	}
	if !dryRun {
		if backup == "" {
			if backup, err = setup.DefaultBackup(plan.Home, plan.From, now); err != nil {
				return err
			}
		}
		if err := setup.ApplyPersonal(plan, backup); err != nil {
			return err
		}
	}
	if asJSON {
		return writeIndented(out, struct {
			DryRun bool `json:"dry_run"`
			*setup.PersonalPlan
		}{dryRun, plan})
	}
	fmt.Fprintf(out, "%s: %s for you, installed version %s, package version %s\n", verb(dryRun, "INSTALLED"), plan.Name, orNone(plan.From), plan.To)
	c, cc := plan.Counts(), plan.CopyCounts()
	fmt.Fprintf(out, "  package folder %s: %d added, %d updated, %d removed, %d unchanged\n", plan.Home, c[setup.Add], c[setup.Update], c[setup.Remove], c[setup.Unchanged])
	fmt.Fprintf(out, "  copies: %d added, %d updated, %d removed, %d unchanged\n", cc[setup.Add], cc[setup.Update], cc[setup.Remove], cc[setup.Unchanged])
	for _, a := range plan.Copies {
		if a.Kind == setup.Unchanged {
			continue
		}
		note := ""
		if a.Modified {
			note = changedNote
		}
		fmt.Fprintf(out, "  %-9s %s%s\n", a.Kind, a.Path, note)
	}
	for _, folder := range plan.HomeOnly {
		fmt.Fprintf(out, "KEPT IN THE PACKAGE'S FOLDER: %s (%s does not copy these to your own folders)\n", folder, product.Name)
	}
	for _, c := range plan.Conflicts {
		fmt.Fprintf(out, "IN THE WAY: %s (not installed by %s)\n", c, product.Name)
	}
	for _, w := range plan.Warnings {
		fmt.Fprintf(out, "WARNING: %s\n", w)
	}
	if !dryRun && plan.Backup != "" {
		fmt.Fprintf(out, "backup: %s\n", plan.Backup)
	}
	return nil
}

// packageStatus prints the packages installed for the person and in the
// workspace, as JSON.
func packageStatus(out io.Writer, t setup.PersonalTargets, workspace string) error {
	type row struct {
		setup.InstalledPackage
		Built   string `json:"built"`
		MinTool string `json:"min_tool"`
		Files   int    `json:"files"`
	}
	rows := []row{}
	for _, p := range setup.InstalledPackages(t, workspace) {
		var m *setup.Manifest
		if p.Target == setup.ForMe {
			m, _ = setup.InstalledPersonal(t, p.Name)
		} else {
			m, _ = setup.Installed(p.Workspace)
		}
		r := row{InstalledPackage: p}
		if m != nil {
			r.Built, r.MinTool, r.Files = m.Built, m.MinTool, len(m.Files)
		}
		rows = append(rows, r)
	}
	return writeIndented(out, rows)
}

// removePackage removes the package installed in a workspace, or the one
// installed for the person, each file backed up first.
func removePackage(out io.Writer, personal bool, name, workspace, backup string, dryRun bool, targets func() (setup.PersonalTargets, error)) error {
	var plan *setup.Plan
	var personalPlan *setup.PersonalPlan
	if personal {
		t, err := targets()
		if err != nil {
			return err
		}
		if name == "" {
			names := setup.InstalledPersonalNames(t)
			switch len(names) {
			case 0:
				return errors.New("no package is installed for you")
			case 1:
				name = names[0]
			default:
				return fmt.Errorf("several packages are installed for you (%s); choose one with --name", strings.Join(names, ", "))
			}
		}
		if personalPlan, err = setup.PlanRemovePersonal(t, name); err != nil {
			return err
		}
		plan = personalPlan.Plan
	} else {
		var err error
		if plan, err = setup.PlanRemove(workspace); err != nil {
			return err
		}
	}
	fmt.Fprintf(out, "%s: %s version %s from %s\n", verb(dryRun, "REMOVED"), plan.Name, plan.From, plan.Workspace)
	line := func(path string, modified bool) {
		note := ""
		if modified {
			note = changedNote
		}
		fmt.Fprintf(out, "  remove    %s%s\n", path, note)
	}
	for _, a := range plan.Actions {
		line(a.Path, a.Modified)
	}
	if personalPlan != nil {
		for _, c := range personalPlan.Copies {
			line(c.Path, c.Modified)
		}
	}
	if dryRun {
		return nil
	}
	if backup == "" {
		var err error
		if backup, err = setup.DefaultBackup(plan.Workspace, plan.From, time.Now()); err != nil {
			return err
		}
	}
	var err error
	if personalPlan != nil {
		err = setup.RemovePersonal(personalPlan, backup)
	} else {
		err = setup.Uninstall(plan, backup)
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "backup: %s\n", plan.Backup)
	return nil
}

func verb(dryRun bool, done string) string {
	if dryRun {
		return "DRY RUN"
	}
	return done
}

func orNone(v string) string {
	if v == "" {
		return "none recorded"
	}
	return v
}
