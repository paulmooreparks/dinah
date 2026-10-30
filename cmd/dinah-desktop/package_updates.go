package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"time"

	"studio/internal/setup"
	"studio/internal/statefile"
)

// Packages are updated from the package sources the build registers
// (setup.RegisterSource, docs/design/packages.md, "Package Updates"). The
// window looks at them when it starts and every packageCheckEvery after,
// and offers each newer version in Notifications, saying what of the
// person's is in its way, if anything; it is installed when the person
// chooses to. A build with no source, as Dinah's own is, starts no check,
// and the window shows no command for updates.

// packageCheckEvery is how often the sources are looked at after the start.
const packageCheckEvery = 6 * time.Hour

// packageUpdatesDir is where the packages the sources offer are kept once
// they are fetched.
func packageUpdatesDir() string { return filepath.Join(configDir(), "package-updates") }

// watchPackageUpdates looks for updates now and every packageCheckEvery
// until stop is closed. It is started only when a source is registered.
func (a *App) watchPackageUpdates(stop <-chan struct{}) {
	tick := time.NewTicker(packageCheckEvery)
	defer tick.Stop()
	for {
		if err := a.checkPackageUpdates(); err != nil {
			logf("core", "warn", "checking for package updates: %v", err)
		}
		select {
		case <-stop:
			return
		case <-tick.C:
		}
	}
}

// installedPackages are the packages installed for the person and in the
// workspace in front.
func (a *App) installedPackages() []setup.InstalledPackage {
	t, err := personalPackageTargets()
	if err != nil {
		return []setup.InstalledPackage{}
	}
	return setup.InstalledPackages(t, a.currentWorkspace())
}

// checkPackageUpdates asks the sources for versions newer than those
// installed and offers each in Notifications once.
func (a *App) checkPackageUpdates() error {
	if len(setup.SourceIDs()) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	status, err := setup.FindUpdates(ctx, a.installedPackages(), packageUpdatesDir(), false)
	for _, u := range status.Updates {
		u.Reason = a.updateObstacle(u)
		if !a.offeredBefore(u) {
			a.emit("packages:update-available", u)
		}
	}
	return err
}

// FindPackageUpdates asks the sources now and says what is installed, what
// is newer and what could be installed, fetching the packages they offer,
// without offering anything in Notifications. With no source registered it
// finds nothing.
func (a *App) FindPackageUpdates() (setup.UpdateStatus, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	status, err := setup.FindUpdates(ctx, a.installedPackages(), packageUpdatesDir(), true)
	for i, u := range status.Updates {
		status.Updates[i].Reason = a.updateObstacle(u)
	}
	return status, err
}

// updateObstacle says what of the person's is in the way of an update, or
// "" when nothing is.
func (a *App) updateObstacle(u setup.PackageUpdate) string {
	packagesMu.Lock()
	defer packagesMu.Unlock()
	if u.Target == setup.ForMe {
		t, err := personalPackageTargets()
		if err != nil {
			return err.Error()
		}
		p, err := setup.PlanPersonal(u.Bundle, t, appVersion)
		if err != nil {
			return err.Error()
		}
		return setup.PersonalObstacle(p)
	}
	p, err := setup.PlanApply(u.Bundle, u.Workspace, appVersion)
	if err != nil {
		return err.Error()
	}
	return setup.WorkspaceObstacle(p)
}

// offeredBefore reports whether this update was offered already, and
// records that it now has been, so the same offer is not made at every
// check.
func (a *App) offeredBefore(u setup.PackageUpdate) bool {
	path := filepath.Join(packageUpdatesDir(), "offered.json")
	key := u.Target + "|" + u.Workspace + "|" + u.Name + "|" + u.To
	seen := false
	_ = statefile.Update(path, func(previous []byte) ([]byte, error) {
		offered := map[string]bool{}
		_ = json.Unmarshal(previous, &offered)
		if seen = offered[key]; seen {
			return nil, nil
		}
		offered[key] = true
		return json.Marshal(offered)
	})
	return seen
}
