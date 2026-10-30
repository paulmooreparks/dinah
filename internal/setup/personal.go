package setup

// A personal install puts a package where every workspace has it, instead of
// into one workspace's main folder (docs/design/packages.md, "For You, in
// Every Workspace").
//
// The package is unpacked unchanged into its home, a folder of its own under
// Packages, with Apply, so versions, the hand-changed check and the backups
// work as they do for a workspace. The items of each kind of package that has
// a personal folder (workspaces.PackageKind.Personal) are then copied to that
// folder: skills to the person's own skills folder, and whatever a build
// registers to its own folders. The package's files refer to one another by
// paths relative to the folder an agent starts in, so in each copy a reference
// to a package file, a path that begins with one of the package's roots, is
// made absolute: into the home, or, for an item of a kind that is copied, to
// its copy. A kind's own Rewrite then changes what else it needs to.
//
// Files of a kind this build does not copy stay in the home, and the plan
// lists their folders (HomeOnly). They are installed and backed up like every
// other file, and references to them lead into the home, so nothing is lost:
// a build that registers the kind copies them at the next update.
//
// A ledger in the home records each copy and its hash as written. An update
// replaces only copies the ledger records, backs up every copy it replaces or
// removes, and removes copies the new version no longer makes. An item that
// exists and is not in the ledger belongs to someone else, and the plan
// refuses to overwrite it.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"studio/internal/profile"
	"studio/internal/workspaces"
)

// PersonalTargets are the folders a personal install writes to.
type PersonalTargets struct {
	// Packages is the folder package homes are kept in; a package's home is
	// Packages/<package name>.
	Packages string
	// Folders maps the key of each kind of package that is copied for the
	// person to the folder its items are copied to.
	Folders map[string]string
}

// Targets are the folders a personal install writes to under the state
// folder state: the packages folder in it, and each registered kind's
// personal folder. isolated says the state folder was given for a test or a
// trial run, so that every folder is inside it.
func Targets(state string, isolated bool) (PersonalTargets, error) {
	t := PersonalTargets{Packages: filepath.Join(state, "packages"), Folders: map[string]string{}}
	for _, kind := range workspaces.PackageKinds() {
		if kind.Personal == nil {
			continue
		}
		dir, err := kind.Personal(state, isolated)
		if err != nil {
			return PersonalTargets{}, fmt.Errorf("the folder for your own %s could not be found: %w", strings.ToLower(kind.Label), err)
		}
		t.Folders[kind.Key] = dir
	}
	return t, nil
}

// CopiesPath is the ledger of a personal install's copies, in the home.
const CopiesPath = "personal-copies.json"

type copiesLedger struct {
	Schema  int    `json:"schema"`
	Name    string `json:"name"`
	Version string `json:"version"`
	// Copies is each copy's path, with forward slashes, and its SHA-256 as
	// written.
	Copies map[string]string `json:"copies"`
}

// CopyAction is what a personal install does with one copy.
type CopyAction struct {
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	Modified bool   `json:"modified,omitempty"`
}

// PersonalPlan is what a personal install would do: the plan for the
// package's home, and the copies.
type PersonalPlan struct {
	*Plan
	Home   string       `json:"home"`
	Copies []CopyAction `json:"copies"`
	// Conflicts are folders and files in the way that the install did not
	// write; while there are any, it is refused.
	Conflicts []string `json:"conflicts,omitempty"`
	// Skills are the names of the skills the install copies.
	Skills []string `json:"skills"`
	// HomeOnly are the package's folders in PackageFolder whose files stay in
	// the home, because this build copies no such kind for the person.
	HomeOnly []string `json:"homeOnly,omitempty"`
	targets  PersonalTargets
	copies   map[string][]byte
}

var bundleName = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{0,63}$`)

// PersonalHome is the package's home for a package of the given name.
func PersonalHome(t PersonalTargets, name string) (string, error) {
	if !bundleName.MatchString(name) {
		return "", fmt.Errorf("the package's name %q cannot be a folder's name", name)
	}
	return filepath.Join(t.Packages, name), nil
}

// InstalledPersonal is the manifest of the package installed for the person
// under that name, or nil when there is none.
func InstalledPersonal(t PersonalTargets, name string) (*Manifest, error) {
	home, err := PersonalHome(t, name)
	if err != nil {
		return nil, err
	}
	if !isDir(home) {
		return nil, nil
	}
	return Installed(home)
}

// InstalledPersonalNames are the names of the packages installed for the
// person, in order.
func InstalledPersonalNames(t PersonalTargets) []string {
	entries, err := os.ReadDir(t.Packages)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if m, err := InstalledPersonal(t, e.Name()); err == nil && m != nil {
			names = append(names, e.Name())
		}
	}
	return names
}

// PlanPersonal works out what installing the package at zipPath for the
// person would do, writing nothing.
func PlanPersonal(zipPath string, t PersonalTargets, toolVersion string) (*PersonalPlan, error) {
	if t.Packages == "" || !filepath.IsAbs(t.Packages) {
		return nil, errors.New("a personal install needs absolute target folders")
	}
	for _, dir := range t.Folders {
		if dir == "" || !filepath.IsAbs(dir) {
			return nil, errors.New("a personal install needs absolute target folders")
		}
	}
	m, _, err := readBundle(zipPath)
	if err != nil {
		return nil, err
	}
	home, err := PersonalHome(t, m.Name)
	if err != nil {
		return nil, err
	}
	// A first install plans against an empty folder, so planning writes
	// nothing.
	at := home
	if !isDir(home) {
		empty, err := os.MkdirTemp("", profile.Current().ID+"-personal-plan-")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(empty)
		at = empty
	}
	plan, err := PlanApply(zipPath, at, toolVersion)
	if err != nil {
		return nil, err
	}
	plan.Workspace = home
	p := &PersonalPlan{Plan: plan, Home: home, Copies: []CopyAction{}, Skills: []string{}, targets: t}
	p.copies, p.HomeOnly = personalCopies(plan.bundle, plan.contents, home, t)
	ledger, err := readLedger(home)
	if err != nil {
		return nil, err
	}
	recorded := func(owner string) bool {
		prefix := filepath.ToSlash(owner)
		for k := range ledger.Copies {
			if k == prefix || strings.HasPrefix(k, prefix+"/") {
				return true
			}
		}
		return false
	}
	seenSkill, conflicted := map[string]bool{}, map[string]bool{}
	targets := make([]string, 0, len(p.copies))
	for k := range p.copies {
		targets = append(targets, k)
	}
	sort.Strings(targets)
	skills := t.Folders[workspaces.Skills]
	for _, target := range targets {
		owner := ownerOf(target, t)
		if skills != "" {
			if rel, err := filepath.Rel(skills, owner); err == nil && filepath.IsLocal(rel) && !seenSkill[rel] {
				seenSkill[rel] = true
				p.Skills = append(p.Skills, filepath.ToSlash(rel))
			}
		}
		key := filepath.ToSlash(target)
		cur, exists, err := absHash(target)
		if err != nil {
			return nil, err
		}
		was, inLedger := ledger.Copies[key]
		switch {
		case !exists:
			if _, err := os.Lstat(owner); err == nil && !recorded(owner) {
				if !conflicted[owner] {
					conflicted[owner] = true
					p.Conflicts = append(p.Conflicts, owner)
				}
				continue
			}
			p.Copies = append(p.Copies, CopyAction{Path: target, Kind: Add})
		case cur == digest(p.copies[target]):
			p.Copies = append(p.Copies, CopyAction{Path: target, Kind: Unchanged})
		case inLedger:
			p.Copies = append(p.Copies, CopyAction{Path: target, Kind: Update, Modified: cur != was})
		default:
			if !conflicted[owner] {
				conflicted[owner] = true
				p.Conflicts = append(p.Conflicts, owner)
			}
		}
	}
	var retired []string
	for k := range ledger.Copies {
		if _, still := p.copies[filepath.FromSlash(k)]; !still {
			retired = append(retired, k)
		}
	}
	sort.Strings(retired)
	for _, k := range retired {
		target := filepath.FromSlash(k)
		// A copy is only ever made in one of the person's kind folders, so a
		// recorded copy anywhere else is not removed.
		if rootOf(target, t) == "" {
			continue
		}
		cur, exists, err := absHash(target)
		if err != nil {
			return nil, err
		}
		if exists {
			p.Copies = append(p.Copies, CopyAction{Path: target, Kind: Remove, Modified: cur != ledger.Copies[k]})
		}
	}
	return p, nil
}

// CopyCounts counts the copies by kind.
func (p *PersonalPlan) CopyCounts() map[string]int {
	n := map[string]int{}
	for _, a := range p.Copies {
		n[a.Kind]++
	}
	return n
}

// ApplyPersonal carries out a personal plan: the home first, as Apply does,
// then the copies, each replaced or removed copy backed up under backup, and
// the ledger last.
func ApplyPersonal(p *PersonalPlan, backup string) error {
	if len(p.Conflicts) > 0 {
		return fmt.Errorf("these are in the way and were not installed by %s: %s", profile.Current().Name, strings.Join(p.Conflicts, "; "))
	}
	if err := os.MkdirAll(p.Home, 0o755); err != nil {
		return err
	}
	if err := Apply(p.Plan, backup); err != nil {
		return err
	}
	bk := p.Plan.Backup
	for _, a := range p.Copies {
		if a.Kind != Update && a.Kind != Remove {
			continue
		}
		b, err := os.ReadFile(a.Path)
		if err != nil {
			return fmt.Errorf("backing up %s: %w", a.Path, err)
		}
		if err := writeFile(filepath.Join(bk, "personal", backupName(a.Path)), b); err != nil {
			return fmt.Errorf("backing up %s: %w", a.Path, err)
		}
	}
	ledger := copiesLedger{Schema: 1, Name: p.Name, Version: p.To, Copies: map[string]string{}}
	for _, a := range p.Copies {
		switch a.Kind {
		case Add, Update:
			if err := writeFile(a.Path, p.copies[a.Path]); err != nil {
				return fmt.Errorf("writing %s: %w (the copies replaced so far are backed up in %s)", a.Path, err, bk)
			}
		case Remove:
			if err := os.Remove(a.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("removing %s: %w (backed up in %s)", a.Path, err, bk)
			}
			pruneEmpty(rootOf(a.Path, p.targets), filepath.Dir(a.Path))
		}
	}
	for target, b := range p.copies {
		ledger.Copies[filepath.ToSlash(target)] = digest(b)
	}
	b, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(p.Home, CopiesPath), append(b, '\n'))
}

// PlanRemovePersonal works out what removing the package installed for the
// person under that name would do: the home's files, as PlanRemove plans
// them, and every copy the ledger records that is still there.
func PlanRemovePersonal(t PersonalTargets, name string) (*PersonalPlan, error) {
	home, err := PersonalHome(t, name)
	if err != nil {
		return nil, err
	}
	plan, err := PlanRemove(home)
	if err != nil {
		return nil, err
	}
	ledger, err := readLedger(home)
	if err != nil {
		return nil, err
	}
	p := &PersonalPlan{Plan: plan, Home: home, Copies: []CopyAction{}, Skills: []string{}, targets: t}
	keys := make([]string, 0, len(ledger.Copies))
	for k := range ledger.Copies {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		target := filepath.FromSlash(k)
		// As in an update, a recorded copy outside the person's kind folders
		// is not removed.
		if rootOf(target, t) == "" {
			continue
		}
		cur, exists, err := absHash(target)
		if err != nil {
			return nil, err
		}
		if exists {
			p.Copies = append(p.Copies, CopyAction{Path: target, Kind: Remove, Modified: cur != ledger.Copies[k]})
		}
	}
	return p, nil
}

// RemovePersonal carries out a personal removal plan: the copies, each
// backed up first, then the home, and the home's folder with its ledger
// last.
func RemovePersonal(p *PersonalPlan, backup string) error {
	bk, err := filepath.Abs(backup)
	if err != nil {
		return err
	}
	for _, a := range p.Copies {
		b, err := os.ReadFile(a.Path)
		if err != nil {
			return fmt.Errorf("backing up %s: %w", a.Path, err)
		}
		if err := writeFile(filepath.Join(bk, "personal", backupName(a.Path)), b); err != nil {
			return fmt.Errorf("backing up %s: %w", a.Path, err)
		}
	}
	for _, a := range p.Copies {
		if err := os.Remove(a.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("removing %s: %w (backed up in %s)", a.Path, err, bk)
		}
		pruneEmpty(rootOf(a.Path, p.targets), filepath.Dir(a.Path))
	}
	if err := Uninstall(p.Plan, backup); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(p.Home, CopiesPath)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_ = os.Remove(p.Home)
	return nil
}

func readLedger(home string) (copiesLedger, error) {
	ledger := copiesLedger{Copies: map[string]string{}}
	b, err := os.ReadFile(filepath.Join(home, CopiesPath))
	if errors.Is(err, os.ErrNotExist) {
		return ledger, nil
	}
	if err != nil {
		return ledger, err
	}
	if err := json.Unmarshal(b, &ledger); err != nil {
		return ledger, fmt.Errorf("the record of the personal copies in %s cannot be read: %w", home, err)
	}
	if ledger.Copies == nil {
		ledger.Copies = map[string]string{}
	}
	return ledger, nil
}

func absHash(p string) (string, bool, error) {
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return digest(b), true, nil
}

// kindOf returns the kind of a copy and the folder of that kind it is in,
// or "" when it is in none of the person's kind folders.
func kindOf(target string, t PersonalTargets) (string, string) {
	keys := make([]string, 0, len(t.Folders))
	for key := range t.Folders {
		keys = append(keys, key)
	}
	// The longest folder is tried first, so a kind's folder inside another's
	// is told apart from it.
	sort.Slice(keys, func(i, j int) bool { return len(t.Folders[keys[i]]) > len(t.Folders[keys[j]]) })
	for _, key := range keys {
		root := t.Folders[key]
		if rel, err := filepath.Rel(root, target); err == nil && filepath.IsLocal(rel) {
			return key, root
		}
	}
	return "", ""
}

// ownerOf is the item a copy belongs to: its folder, as a skill's, or the
// file itself, for a kind whose items are files.
func ownerOf(target string, t PersonalTargets) string {
	key, root := kindOf(target, t)
	if key == "" {
		return target
	}
	if kind, ok := workspaces.LookupPackageKind(key); ok && kind.FileItems {
		return target
	}
	rel, _ := filepath.Rel(root, target)
	first := strings.SplitN(filepath.ToSlash(rel), "/", 2)[0]
	return filepath.Join(root, first)
}

// rootOf is the kind folder a copy is in, or "" when it is in none.
func rootOf(target string, t PersonalTargets) string {
	_, root := kindOf(target, t)
	return root
}

// backupName is a copy's path made into a relative path for the backup.
func backupName(p string) string {
	s := filepath.ToSlash(filepath.Clean(p))
	s = strings.ReplaceAll(s, ":", "")
	return filepath.FromSlash(strings.TrimLeft(s, "/"))
}

// kindFolder is the folder of a package that holds the items of the kind
// with the key, with its slash.
func kindFolder(key string) string { return workspaces.PackageFolder + "/" + key + "/" }

// personalCopies are the copies a package makes for the person: each file's
// target path and its content with package references made absolute. It
// also returns the folders in the package's PackageFolder whose files are not
// copied.
func personalCopies(m *Manifest, contents map[string][]byte, home string, t PersonalTargets) (map[string][]byte, []string) {
	type copied struct {
		key, folder string
		kind        workspaces.PackageKind
		items       map[string]bool
	}
	var kinds []copied
	for _, kind := range workspaces.PackageKinds() {
		if folder, ok := t.Folders[kind.Key]; ok {
			kinds = append(kinds, copied{key: kind.Key, folder: folder, kind: kind, items: map[string]bool{}})
		}
	}
	homeOnly := map[string]bool{}
	for p := range m.Files {
		found := false
		for _, k := range kinds {
			if rest, ok := strings.CutPrefix(p, kindFolder(k.key)); ok {
				k.items[strings.SplitN(rest, "/", 2)[0]] = true
				found = true
			}
		}
		if rest, ok := strings.CutPrefix(p, workspaces.PackageFolder+"/"); ok && !found {
			if folder, _, nested := strings.Cut(rest, "/"); nested {
				homeOnly[workspaces.PackageFolder+"/"+folder+"/"] = true
			}
		}
	}
	rules := []prefixRule{}
	slash := func(p string) string { return strings.TrimSuffix(filepath.ToSlash(p), "/") }
	for _, k := range kinds {
		rules = append(rules, prefixRule{kindFolder(k.key), slash(k.folder) + "/", true})
	}
	for _, r := range m.Roots {
		rules = append(rules, prefixRule{r, slash(home) + "/" + r, strings.HasSuffix(r, "/")})
	}
	// The longest prefix is tried first, so .devin/skills/ wins over .devin/.
	sort.SliceStable(rules, func(i, j int) bool { return len(rules[i].from) > len(rules[j].from) })
	rw := rewriter{rules: rules}

	out := map[string][]byte{}
	for p := range m.Files {
		for _, k := range kinds {
			rest, ok := strings.CutPrefix(p, kindFolder(k.key))
			if !ok {
				continue
			}
			data := contents[p]
			if isTextFile(p, data) {
				text := rw.rewrite(string(data))
				for _, other := range kinds {
					if other.kind.Rewrite != nil {
						text = other.kind.Rewrite(text, sortedKeys(other.items))
					}
				}
				data = []byte(text)
			}
			out[filepath.Join(k.folder, filepath.FromSlash(rest))] = data
		}
	}
	return out, sortedKeys(homeOnly)
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func isTextFile(p string, data []byte) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".md", ".json", ".toml", ".txt", ".yaml", ".yml":
		return utf8.Valid(data)
	}
	return false
}

type prefixRule struct {
	from, to string
	dir      bool // the prefix is a folder, and a reference continues past it
}

type rewriter struct {
	rules []prefixRule
}

func isPathChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' || c == '.' || c == '/' || c == '\\'
}

func isNameChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_'
}

// rewrite makes the package references in s absolute. A reference is a
// package root at the start of a path: not preceded by a character that could
// belong to a longer path, and, for a folder root, followed by more of the
// path, so a bare mention of ".devin/" is left alone.
func (rw rewriter) rewrite(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		matched := false
		if i == 0 || !isPathChar(s[i-1]) {
			for _, r := range rw.rules {
				if !strings.HasPrefix(s[i:], r.from) {
					continue
				}
				end := i + len(r.from)
				if r.dir && (end >= len(s) || !isNameChar(s[end])) {
					continue
				}
				if !r.dir && end < len(s) && isPathChar(s[end]) && s[end] != '.' {
					continue
				}
				b.WriteString(r.to)
				i = end
				matched = true
				break
			}
		}
		if !matched {
			b.WriteByte(s[i])
			i++
		}
	}
	return b.String()
}
