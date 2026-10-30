package setup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"studio/internal/profile"
)

// A package source is somewhere newer versions of packages can be found,
// such as a page that packages are attached to. A build registers its
// sources when it starts, in the way workspaces.RegisterSourceKind registers
// a kind of source, and the window looks at every registered source when it
// starts and every few hours after, offering each newer version it finds in
// Notifications (docs/design/packages.md, "Package Updates"). Dinah registers
// none, so its window looks nowhere and offers nothing.
//
// Nothing is installed without asking: a source is only as trustworthy as
// whoever can put a file there, and a package brings skills the agents
// follow. Paul decided on 22 October 2026 that only a
// signed package may be installed without asking, and packages are not
// signed.

// Source is one place packages can be found.
type Source struct {
	// Label is how people see the source, as in "the setup page".
	Label string
	// Enabled says whether the person has the source turned on. Nil means
	// it always is.
	Enabled func() bool
	// Find lists the packages the source offers, at least the newest version
	// of each. It may return an error that says what the person can do
	// about it, such as connecting to the site.
	Find func(ctx context.Context) ([]Candidate, error)
}

// Candidate is one version of a package that a source offers.
type Candidate struct {
	// Version is the package's version, as its manifest has it. Which
	// package it is, is read from the manifest once it is fetched.
	Version string
	// Fetch makes the package's ZIP a file in dir, the folder updates are
	// kept in, and returns the file's path. A file fetched at an earlier
	// check may be returned again. A file that is not a package is removed
	// after it is read, so a corrected one is fetched at the next check.
	Fetch func(ctx context.Context, dir string) (string, error)
}

var (
	sourcesMu sync.RWMutex
	sources   = map[string]Source{}
)

var sourceIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

// RegisterSource registers a package source under an ID of the same form as
// a kind's. Registering an ID twice, or a source without Find, panics.
func RegisterSource(id string, s Source) {
	sourcesMu.Lock()
	defer sourcesMu.Unlock()
	if !sourceIDPattern.MatchString(id) || s.Find == nil {
		panic("setup: cannot register the package source " + id)
	}
	if _, ok := sources[id]; ok {
		panic("setup: the package source " + id + " is registered twice")
	}
	sources[id] = s
}

// SourceIDs are the IDs of the registered package sources, in order.
func SourceIDs() []string {
	sourcesMu.RLock()
	defer sourcesMu.RUnlock()
	ids := make([]string, 0, len(sources))
	for id := range sources {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func lookupSource(id string) (Source, bool) {
	sourcesMu.RLock()
	defer sourcesMu.RUnlock()
	s, ok := sources[id]
	return s, ok
}

// Targets of an installed package.
const (
	ForMe        = "me"        // installed for the person, in every workspace
	ForWorkspace = "workspace" // installed in one workspace's main folder
)

// InstalledPackage is a package installed for the person or in a
// workspace.
type InstalledPackage struct {
	Target    string `json:"target"`
	Workspace string `json:"workspace,omitempty"`
	Name      string `json:"name"`
	Version   string `json:"version"`
}

// InstalledPackages are the packages installed for the person and, when
// workspace is not "", in that workspace.
func InstalledPackages(t PersonalTargets, workspace string) []InstalledPackage {
	out := []InstalledPackage{}
	for _, name := range InstalledPersonalNames(t) {
		if m, err := InstalledPersonal(t, name); err == nil && m != nil {
			out = append(out, InstalledPackage{Target: ForMe, Name: name, Version: m.Version})
		}
	}
	if workspace != "" {
		if m, err := Installed(workspace); err == nil && m != nil {
			out = append(out, InstalledPackage{Target: ForWorkspace, Workspace: workspace, Name: m.Name, Version: m.Version})
		}
	}
	return out
}

// PackageUpdate is a newer version of an installed package.
type PackageUpdate struct {
	Target    string `json:"target"`
	Workspace string `json:"workspace,omitempty"`
	Name      string `json:"name"`
	From      string `json:"from"`
	To        string `json:"to"`
	// Bundle is the fetched package, which the update installs.
	Bundle string `json:"bundle"`
	// Reason says what of the person's is in the way of the update, for
	// them to know before they install it, or "" when nothing is.
	Reason string `json:"reason,omitempty"`
}

// AvailablePackage is a package a source offers that is not installed.
type AvailablePackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Bundle  string `json:"bundle"`
}

// UpdateStatus is what the sources offer: the packages installed, the
// updates for them and the packages that could be installed.
type UpdateStatus struct {
	Installed []InstalledPackage `json:"installed"`
	Updates   []PackageUpdate    `json:"updates"`
	Available []AvailablePackage `json:"available"`
	// Off says that every registered source is turned off.
	Off bool `json:"off,omitempty"`
}

// FindUpdates asks every registered source that is turned on for packages
// newer than those installed, fetching each into dir. With uninstalled, the
// newest version of a package that is not installed at all is fetched too,
// to offer to install. An error from one source does not stop the others;
// what they found is returned with it.
func FindUpdates(ctx context.Context, installed []InstalledPackage, dir string, uninstalled bool) (UpdateStatus, error) {
	status := UpdateStatus{Installed: installed, Updates: []PackageUpdate{}, Available: []AvailablePackage{}}
	ids := SourceIDs()
	var errs []error
	on := 0
	for _, id := range ids {
		s, _ := lookupSource(id)
		if s.Enabled != nil && !s.Enabled() {
			continue
		}
		on++
		candidates, err := s.Find(ctx)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, c := range candidates {
			newer := uninstalled
			for _, h := range installed {
				newer = newer || versionNewer(c.Version, h.Version)
			}
			if !newer || c.Fetch == nil {
				continue
			}
			bundle, err := c.Fetch(ctx, dir)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			m, err := BundleManifest(bundle)
			if err != nil {
				// Not a package, or one that cannot be read; it is not kept, so
				// a corrected one of the same name is fetched at the next check.
				_ = os.Remove(bundle)
				continue
			}
			known := false
			for _, h := range installed {
				if h.Name != m.Name {
					continue
				}
				known = true
				if !versionNewer(m.Version, h.Version) {
					continue
				}
				status.Updates = append(status.Updates, PackageUpdate{Target: h.Target, Workspace: h.Workspace, Name: h.Name, From: h.Version, To: m.Version, Bundle: bundle})
			}
			if !known && uninstalled && !hasAvailable(status.Available, m.Name, m.Version) {
				status.Available = append(status.Available, AvailablePackage{Name: m.Name, Version: m.Version, Bundle: bundle})
			}
		}
	}
	status.Off = len(ids) > 0 && on == 0
	return status, errors.Join(errs...)
}

func hasAvailable(list []AvailablePackage, name, version string) bool {
	for _, a := range list {
		if a.Name == name && a.Version == version {
			return true
		}
	}
	return false
}

// PersonalObstacle says what of the person's is in the way of installing a
// personal plan, or "" when nothing is.
func PersonalObstacle(p *PersonalPlan) string {
	if len(p.Conflicts) > 0 {
		names := make([]string, len(p.Conflicts))
		for i, c := range p.Conflicts {
			names[i] = filepath.Base(c)
		}
		return fmt.Sprintf("%s did not install %s, which %s in the way", profile.Current().Name, strings.Join(names, ", "), map[bool]string{true: "is", false: "are"}[len(names) == 1])
	}
	for _, c := range p.Copies {
		if c.Modified {
			return "files of the package were changed by hand"
		}
	}
	return WorkspaceObstacle(p.Plan)
}

// WorkspaceObstacle says what of the person's is in the way of installing
// a plan in a workspace, or "" when nothing is.
func WorkspaceObstacle(p *Plan) string {
	for _, a := range p.Actions {
		if a.Modified || a.Unknown {
			return "files of the package were changed by hand"
		}
	}
	return ""
}

// versionNewer says whether version a is newer than b. Versions are numbers
// separated by dots, with an optional leading "v", as 17 or 1.20.0; a missing
// part counts as 0. A version not of that form is never newer.
func versionNewer(a, b string) bool {
	x, okA := versionParts(a)
	y, okB := versionParts(b)
	if !okA || !okB {
		return false
	}
	for i := 0; i < max(len(x), len(y)); i++ {
		var p, q int
		if i < len(x) {
			p = x[i]
		}
		if i < len(y) {
			q = y[i]
		}
		if p != q {
			return p > q
		}
	}
	return false
}

func versionParts(v string) ([]int, bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if v == "" {
		return nil, false
	}
	var out []int
	for _, part := range strings.Split(v, ".") {
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return nil, false
		}
		out = append(out, n)
	}
	return out, true
}
