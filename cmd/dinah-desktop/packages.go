package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"studio/internal/setup"
	"studio/internal/workspaces"
)

// Settings › Packages installs a content package from its ZIP, for the
// person or into the workspace in front, and removes one again, as the
// command line's package install and package remove do
// (docs/design/packages.md). The same package does the work in both
// programs.

// packagesMu is held while a package is installed, updated or removed, so
// that the update check, which runs on its own, and an install the person
// starts never write one package's files at once.
var packagesMu sync.Mutex

// PackagePlan is what installing or removing a package does, as the window
// shows it before and after.
type PackagePlan struct {
	Name      string `json:"name"`
	From      string `json:"from"`
	To        string `json:"to"`
	Added     int    `json:"added"`
	Updated   int    `json:"updated"`
	Removed   int    `json:"removed"`
	Unchanged int    `json:"unchanged"`
	// Changed are the files the person changed since the last install,
	// which are replaced or removed after a copy is kept in the backup.
	Changed  []string `json:"changed"`
	Warnings []string `json:"warnings"`
	Backup   string   `json:"backup,omitempty"`
	// Token is the package's content and the installed version the plan was
	// made from, so an install applies only the plan the person saw.
	Token string `json:"token"`
}

// packageToken is the SHA-256 of a package's bytes with the version
// installed now, which a plan is made from.
func packageToken(bundle, from string) (string, error) {
	data, err := os.ReadFile(bundle)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(append(data, []byte("\x00"+from)...))
	return hex.EncodeToString(sum[:]), nil
}

func packagePlanOf(p *setup.Plan) PackagePlan {
	c := p.Counts()
	out := PackagePlan{Name: p.Name, From: p.From, To: p.To, Added: c[setup.Add], Updated: c[setup.Update], Removed: c[setup.Remove], Unchanged: c[setup.Unchanged], Changed: []string{}, Warnings: append([]string{}, p.Warnings...), Backup: p.Backup}
	for _, a := range p.Actions {
		if a.Modified || a.Unknown && a.Kind != setup.Unchanged && a.Kind != setup.Add {
			out.Changed = append(out.Changed, a.Path)
		}
	}
	return out
}

// packageWorkspace is a registered workspace's main folder.
func (a *App) packageWorkspace(workspace string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if workspace == "" || workspaces.IndexAt(a.settings.WorkspaceRecords, workspace) < 0 {
		return "", fmt.Errorf("open the workspace in %s first", product.Name)
	}
	return filepath.Clean(workspace), nil
}

// personalPackageTargets are the folders an install for the person writes
// to: the packages folder in the state folder, and the person's folder of
// each kind of package. Under a test's state folder every one is inside it.
func personalPackageTargets() (setup.PersonalTargets, error) {
	return setup.Targets(configDir(), configOverride != "")
}

// ChoosePackageBundle asks for a package's ZIP.
func (a *App) ChoosePackageBundle() (string, error) {
	return wruntime.OpenFileDialog(a.ctx, wruntime.OpenDialogOptions{Title: "Install a Package",
		Filters: []wruntime.FileFilter{{DisplayName: "Packages (*.zip)", Pattern: "*.zip"}}})
}

// InstalledPackages are the packages installed for the person and in the
// workspace in front, for Settings › Packages.
func (a *App) InstalledPackages() ([]setup.InstalledPackage, error) {
	t, err := personalPackageTargets()
	if err != nil {
		return nil, err
	}
	return setup.InstalledPackages(t, a.currentWorkspace()), nil
}

// PlanPackages says what installing a package in a workspace would do,
// writing nothing.
func (a *App) PlanPackages(workspace, bundle string) (PackagePlan, error) {
	ws, err := a.packageWorkspace(workspace)
	if err != nil {
		return PackagePlan{}, err
	}
	p, err := setup.PlanApply(bundle, ws, appVersion)
	if err != nil {
		return PackagePlan{}, err
	}
	out := packagePlanOf(p)
	if out.Token, err = packageToken(bundle, p.From); err != nil {
		return PackagePlan{}, err
	}
	return out, nil
}

// InstallPackages installs a package in a workspace, keeping every file it
// replaces or removes in a backup folder outside the workspace. token is the
// plan's, and the install is refused when the package or the installed
// version has changed since that plan was shown.
func (a *App) InstallPackages(workspace, bundle, token string) (PackagePlan, error) {
	packagesMu.Lock()
	defer packagesMu.Unlock()
	ws, err := a.packageWorkspace(workspace)
	if err != nil {
		return PackagePlan{}, err
	}
	p, err := setup.PlanApply(bundle, ws, appVersion)
	if err != nil {
		return PackagePlan{}, err
	}
	if now, err := packageToken(bundle, p.From); err != nil || now != token {
		return PackagePlan{}, errors.New("the package or what is installed changed since the plan was shown; install it again to see the new plan")
	}
	backup, err := setup.DefaultBackup(ws, p.From, time.Now())
	if err != nil {
		return PackagePlan{}, err
	}
	if err := setup.Apply(p, backup); err != nil {
		return PackagePlan{}, err
	}
	a.emit("packages:changed", nil)
	return packagePlanOf(p), nil
}

// PersonalPackagePlan is what installing or removing a package for the
// person does, as the window shows it before and after.
type PersonalPackagePlan struct {
	PackagePlan
	Home   string `json:"home"`
	Copied int    `json:"copied"`
	// Conflicts are items in the way that this program did not install;
	// while there are any, the install is refused.
	Conflicts []string `json:"conflicts"`
	// Duplicates are the package's skills the workspace in front has copies
	// of too.
	Duplicates []string `json:"duplicates"`
	// HomeOnly are the package's folders whose files stay in its home,
	// because this build copies no such kind for the person.
	HomeOnly []string `json:"homeOnly"`
}

func (a *App) personalPlan(workspace, bundle string) (*setup.PersonalPlan, PersonalPackagePlan, error) {
	t, err := personalPackageTargets()
	if err != nil {
		return nil, PersonalPackagePlan{}, err
	}
	p, err := setup.PlanPersonal(bundle, t, appVersion)
	if err != nil {
		return nil, PersonalPackagePlan{}, err
	}
	out := PersonalPackagePlan{PackagePlan: packagePlanOf(p.Plan), Home: p.Home, Conflicts: append([]string{}, p.Conflicts...), Duplicates: []string{}, HomeOnly: append([]string{}, p.HomeOnly...)}
	c := p.CopyCounts()
	out.Copied = c[setup.Add] + c[setup.Update]
	out.Added += c[setup.Add]
	out.Updated += c[setup.Update]
	out.Removed += c[setup.Remove]
	out.Unchanged += c[setup.Unchanged]
	for _, ca := range p.Copies {
		if ca.Modified {
			out.Changed = append(out.Changed, ca.Path)
		}
	}
	// Devin does not document which skill it uses when a workspace and the
	// person both have one of the same name, so duplicates are pointed out
	// rather than relied on either way.
	if workspace != "" {
		for _, name := range p.Skills {
			if dirExists(filepath.Join(workspace, workspaces.PackageFolder, workspaces.Skills, name)) {
				out.Duplicates = append(out.Duplicates, name)
			}
		}
	}
	if out.Token, err = packageToken(bundle, p.From); err != nil {
		return nil, PersonalPackagePlan{}, err
	}
	return p, out, nil
}

// PlanPersonalPackages says what installing a package for the person would
// do, writing nothing. workspace, the one in front, is only checked for
// skills of the same names.
func (a *App) PlanPersonalPackages(workspace, bundle string) (PersonalPackagePlan, error) {
	_, out, err := a.personalPlan(workspace, bundle)
	return out, err
}

// InstallPersonalPackages installs a package for the person, so every
// workspace has its skills. token is the plan's, and the install is refused
// when the package or the installed version has changed since that plan was
// shown.
func (a *App) InstallPersonalPackages(workspace, bundle, token string) (PersonalPackagePlan, error) {
	packagesMu.Lock()
	defer packagesMu.Unlock()
	p, out, err := a.personalPlan(workspace, bundle)
	if err != nil {
		return PersonalPackagePlan{}, err
	}
	if out.Token != token {
		return PersonalPackagePlan{}, errors.New("the package or what is installed changed since the plan was shown; install it again to see the new plan")
	}
	backup, err := setup.DefaultBackup(p.Home, p.From, time.Now())
	if err != nil {
		return PersonalPackagePlan{}, err
	}
	if err := setup.ApplyPersonal(p, backup); err != nil {
		return PersonalPackagePlan{}, err
	}
	out.Backup = p.Plan.Backup
	a.emit("packages:changed", nil)
	return out, nil
}

// removalPlan is what removing a package would do: the one called name
// installed for the person (setup.ForMe), or the one in the workspace in
// front (setup.ForWorkspace).
func (a *App) removalPlan(target, workspace, name string) (*setup.PersonalPlan, *setup.Plan, PersonalPackagePlan, error) {
	out := PersonalPackagePlan{Conflicts: []string{}, Duplicates: []string{}, HomeOnly: []string{}}
	var plan *setup.Plan
	var personal *setup.PersonalPlan
	switch target {
	case setup.ForMe:
		t, err := personalPackageTargets()
		if err != nil {
			return nil, nil, out, err
		}
		names := setup.InstalledPersonalNames(t)
		if name == "" && len(names) == 1 {
			name = names[0]
		}
		if name == "" {
			if len(names) == 0 {
				return nil, nil, out, errors.New("no package is installed for you")
			}
			return nil, nil, out, fmt.Errorf("several packages are installed for you (%s); choose one", strings.Join(names, ", "))
		}
		if personal, err = setup.PlanRemovePersonal(t, name); err != nil {
			return nil, nil, out, err
		}
		plan = personal.Plan
		out.Home = personal.Home
		for _, c := range personal.Copies {
			out.Removed++
			if c.Modified {
				out.Changed = append(out.Changed, c.Path)
			}
		}
	case setup.ForWorkspace:
		ws, err := a.packageWorkspace(workspace)
		if err != nil {
			return nil, nil, out, err
		}
		if plan, err = setup.PlanRemove(ws); err != nil {
			return nil, nil, out, err
		}
		if name != "" && plan.Name != name {
			return nil, nil, out, fmt.Errorf("the workspace has %s installed, not %s", plan.Name, name)
		}
	default:
		return nil, nil, out, errors.New("remove a package installed for you or in this workspace")
	}
	base := packagePlanOf(plan)
	base.Removed += out.Removed
	base.Changed = append(base.Changed, out.Changed...)
	out.PackagePlan = base
	sum, _ := json.Marshal(plan.Actions)
	if personal != nil {
		more, _ := json.Marshal(personal.Copies)
		sum = append(sum, more...)
	}
	h := sha256.Sum256(sum)
	out.Token = hex.EncodeToString(h[:])
	return personal, plan, out, nil
}

// PlanRemovePackages says what removing a package would do, for the person
// or from the workspace, writing nothing.
func (a *App) PlanRemovePackages(target, workspace, name string) (PersonalPackagePlan, error) {
	_, _, out, err := a.removalPlan(target, workspace, name)
	return out, err
}

// RemovePackages removes a package, every file it removes backed up first.
// token is the plan's, and the removal is refused when what is installed
// changed since that plan was shown.
func (a *App) RemovePackages(target, workspace, name, token string) (PersonalPackagePlan, error) {
	packagesMu.Lock()
	defer packagesMu.Unlock()
	personal, plan, out, err := a.removalPlan(target, workspace, name)
	if err != nil {
		return out, err
	}
	if out.Token != token {
		return out, errors.New("what is installed changed since the plan was shown; remove the package again to see the new plan")
	}
	backup, err := setup.DefaultBackup(plan.Workspace, plan.From, time.Now())
	if err != nil {
		return out, err
	}
	if personal != nil {
		err = setup.RemovePersonal(personal, backup)
	} else {
		err = setup.Uninstall(plan, backup)
	}
	if err != nil {
		return out, err
	}
	out.Backup = plan.Backup
	a.emit("packages:changed", nil)
	return out, nil
}

func dirExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}
