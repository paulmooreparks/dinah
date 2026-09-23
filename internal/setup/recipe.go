package setup

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"dinah/internal/bench"
)

//go:embed recipes
var shippedRecipes embed.FS

// shipped lists the recipes Dinah ships, in the order a listing names them. A
// recipe embedded and not named here is served by nothing, and one named here
// and not embedded is offered by nothing; the test in this package holds the
// two sets equal in both directions.
var shipped = []string{"claude-code", "codex", "devin"}

// Shipped lists the recipes embedded in the binary.
func Shipped() []string {
	return append([]string(nil), shipped...)
}

// The places a recipe can come from, which a heading and a listing name.
const (
	// SourceProject is a recipe found in a project's own .dinah container.
	SourceProject = "project"
	// SourceUser is a recipe found in the user base.
	SourceUser = "user"
	// SourceShipped is a recipe embedded in the binary.
	SourceShipped = "shipped"
	// SourcePath is a recipe named by --recipe.
	SourcePath = "path"
)

// The scopes a recipe may declare.
const (
	ScopeProject = "project"
	ScopeUser    = "user"
)

// The step kinds a recipe may declare.
const (
	KindJSONMerge     = "json-merge"
	KindMarkedSection = "marked-section"
	KindWriteFile     = "write-file"
	KindRun           = "run"
)

// The tool profiles a recipe and --tools may name, which are the three dinah
// mcp serves.
var toolProfiles = []string{"station", "operator", "all"}

// recipeFormat is the recipe format revision this build reads.
const recipeFormat = 1

// The files a recipe directory holds.
const (
	manifestFile = "recipe.json"
	stepsFile    = "steps.json"
	promptFile   = "prompt.md"
	removeFile   = "remove.md"
	filesDir     = "files"
)

// Recipe is one harness's recipe, read and validated.
type Recipe struct {
	// Name is the recipe's name, which is also its directory's.
	Name string
	// Title is the human title a listing shows.
	Title string
	// Harness is the value written as DINAH_HARNESS.
	Harness string
	// Provider, Agent and Tools are the defaults of their flags.
	Provider, Agent, Tools string
	// Scopes are the scopes the recipe supports.
	Scopes []string
	// Documentation records the published pages the recipe's locations rest
	// on. Setup never fetches them.
	Documentation []string
	// Steps are the script part, in order.
	Steps []Step
	// Prompt is prompt.md, unrendered.
	Prompt string
	// Remove is remove.md, unrendered, and empty when the recipe has none.
	Remove string
	// templates are the files under files/ a step names, by name.
	templates map[string]string
	// Source says where the recipe was found.
	Source string
	// Dir is the recipe's directory, absolute, and empty for a shipped one.
	Dir string
}

// Step is one step of a recipe's script part.
type Step struct {
	// ID names the step in output, in the ledger and in a section's markers.
	ID string
	// Kind is json-merge, marked-section, write-file or run.
	Kind string
	// Scope is project or user.
	Scope string
	// Path is the file, relative to the scope's base, unrendered. A run step
	// has none.
	Path string
	// Pointer and value belong to a json-merge step.
	Pointer string
	value   *jvalue
	// Template belongs to a marked-section or write-file step.
	Template string
	// Comment chooses a marked section's marker syntax, html or hash.
	Comment string
	// Program and Args belong to a run step.
	Program string
	Args    []string
}

// manifest is recipe.json's own shape. Every member is a pointer or a slice
// so that a missing one is told apart from an empty one.
type manifest struct {
	Format        *int     `json:"format"`
	Name          *string  `json:"name"`
	Title         *string  `json:"title"`
	Harness       *string  `json:"harness"`
	Provider      *string  `json:"provider"`
	Agent         *string  `json:"agent"`
	Tools         *string  `json:"tools"`
	Scopes        []string `json:"scopes"`
	Documentation []string `json:"documentation"`
}

// stepKeys are the keys each step kind may carry beyond the four every step
// carries, with whether each is required.
var stepKeys = map[string]map[string]bool{
	KindJSONMerge:     {"pointer": true, "value": true},
	KindMarkedSection: {"template": true, "comment": true},
	KindWriteFile:     {"template": true},
	KindRun:           {"program": true, "args": false},
}

// recipeError is a defect in one file of a recipe, which the malformed-recipe
// refusal reports as <file>: <defect>.
type recipeError struct {
	file   string
	defect string
}

// Error renders the defect the way the refusal's detail carries it.
func (e *recipeError) Error() string {
	return e.file + ": " + e.defect
}

// defect builds a recipeError.
func defect(file, format string, args ...any) error {
	return &recipeError{file: file, defect: fmt.Sprintf(format, args...)}
}

// readRecipe reads and validates the recipe in a directory of a filesystem.
// name is the directory's own name, which the manifest's name must equal, and
// source says where it was found, since a project's recipe is held to
// narrower rules.
func readRecipe(fsys fs.FS, name, source, dir string) (*Recipe, error) {
	r := &Recipe{Name: name, Source: source, Dir: dir, templates: map[string]string{}}
	if err := r.readManifest(fsys); err != nil {
		return nil, err
	}
	if err := r.readSteps(fsys); err != nil {
		return nil, err
	}
	for _, step := range r.Steps {
		if step.Template == "" {
			continue
		}
		if _, read := r.templates[step.Template]; read {
			continue
		}
		file := filesDir + "/" + step.Template
		text, err := readText(fsys, file, true)
		if err != nil {
			return nil, err
		}
		r.templates[step.Template] = text
	}
	prompt, err := readText(fsys, promptFile, true)
	if err != nil {
		return nil, err
	}
	r.Prompt = prompt
	remove, err := readText(fsys, removeFile, false)
	if err != nil {
		return nil, err
	}
	r.Remove = remove
	return r, nil
}

// readText reads one of a recipe's text files and checks its placeholders. A
// file that is not required and is absent reads as empty.
func readText(fsys fs.FS, file string, required bool) (string, error) {
	data, err := fs.ReadFile(fsys, file)
	if errors.Is(err, fs.ErrNotExist) && !required {
		return "", nil
	}
	if err != nil {
		return "", defect(file, "cannot be read (%v)", unwrapPathError(err))
	}
	if err := checkEncoding(data); err != nil {
		return "", defect(file, "%s", err.Error())
	}
	text := string(data)
	if _, err := scanPlaceholders(text, true); err != nil {
		return "", defect(file, "%s", err.Error())
	}
	return text, nil
}

// unwrapPathError drops the path a filesystem error carries, since the
// refusal already names the file.
func unwrapPathError(err error) error {
	var pathError *fs.PathError
	if errors.As(err, &pathError) {
		return pathError.Err
	}
	return err
}

// checkEncoding refuses bytes that are not UTF-8 or that open with a
// byte-order mark.
func checkEncoding(data []byte) error {
	if bytes.HasPrefix(data, []byte("\xef\xbb\xbf")) {
		return errors.New("opens with a byte-order mark")
	}
	if !utf8.Valid(data) {
		return errors.New("is not UTF-8")
	}
	return nil
}

// readJSON reads one of a recipe's JSON files, refusing an unknown member.
func readJSON(fsys fs.FS, file string, into any) error {
	data, err := fs.ReadFile(fsys, file)
	if err != nil {
		return defect(file, "cannot be read (%v)", unwrapPathError(err))
	}
	if err := checkEncoding(data); err != nil {
		return defect(file, "%s", err.Error())
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		return defect(file, "does not parse (%v)", err)
	}
	if dec.More() {
		return defect(file, "holds more than one JSON value")
	}
	return nil
}

// readManifest reads recipe.json. It decodes the file twice, once into the
// manifest's own shape and once into its members, because provider is
// optional and a nil pointer alone cannot tell an omitted member from one
// written as null. readStep reads a step the same way, for the same reason.
func (r *Recipe) readManifest(fsys fs.FS) error {
	var m manifest
	if err := readJSON(fsys, manifestFile, &m); err != nil {
		return err
	}
	var members map[string]json.RawMessage
	if err := readJSON(fsys, manifestFile, &members); err != nil {
		return err
	}
	if m.Format == nil {
		return defect(manifestFile, "declares no format")
	}
	if *m.Format != recipeFormat {
		return defect(manifestFile, "declares format %d, and this build reads format %d", *m.Format, recipeFormat)
	}
	if m.Name == nil || *m.Name != r.Name {
		return defect(manifestFile, "names the recipe %s, and its directory is %s", deref(m.Name), r.Name)
	}
	if !bench.HarnessName(*m.Name) {
		return defect(manifestFile, "names the recipe %s, which is not a harness name", *m.Name)
	}
	if m.Title == nil || strings.TrimSpace(*m.Title) == "" || strings.ContainsAny(*m.Title, "\r\n") {
		return defect(manifestFile, "carries no one-line title")
	}
	r.Title = *m.Title
	r.Harness = r.Name
	if m.Harness != nil {
		if !bench.HarnessName(*m.Harness) {
			return defect(manifestFile, "declares the harness %s, which is not a harness name", *m.Harness)
		}
		r.Harness = *m.Harness
	}
	// provider is the one default a recipe may leave out, for a harness that
	// brokers models from several vendors and so has no true default of its
	// own. Absence is the member being gone from the file and nothing else,
	// so a member written as null, as an empty string or as anything else
	// that is not one word is the defect it has always been.
	if _, declared := members["provider"]; declared {
		if m.Provider == nil || !oneWord(*m.Provider) {
			return defect(manifestFile, "carries no one-word provider")
		}
		r.Provider = *m.Provider
	}
	if m.Agent == nil || !oneWord(*m.Agent) {
		return defect(manifestFile, "carries no one-word agent")
	}
	r.Agent = *m.Agent
	if m.Tools == nil || !contains(toolProfiles, *m.Tools) {
		return defect(manifestFile, "declares tools %s, and the profiles are station, operator and all", deref(m.Tools))
	}
	r.Tools = *m.Tools
	if len(m.Scopes) == 0 {
		return defect(manifestFile, "declares no scope")
	}
	for i, scope := range m.Scopes {
		if scope != ScopeProject && scope != ScopeUser {
			return defect(manifestFile, "declares the scope %s, and the scopes are project and user", scope)
		}
		if contains(m.Scopes[:i], scope) {
			return defect(manifestFile, "declares the scope %s twice", scope)
		}
	}
	r.Scopes = m.Scopes
	if len(m.Documentation) == 0 {
		return defect(manifestFile, "records no documentation")
	}
	for _, page := range m.Documentation {
		if !strings.HasPrefix(page, "https://") || len(page) == len("https://") || strings.ContainsAny(page, " \t\r\n") {
			return defect(manifestFile, "records %s, which is not an absolute https:// URL", page)
		}
	}
	r.Documentation = m.Documentation
	return nil
}

// deref reads a string pointer, answering empty for nil.
func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// contains reports whether a list holds a value.
func contains(list []string, value string) bool {
	for _, member := range list {
		if member == value {
			return true
		}
	}
	return false
}

// oneWord reports whether a value is one that setup writes safely into a
// command line, a JSON string and a TOML string alike: non-empty, with no
// whitespace, no control character, and none of the three quoting
// characters.
func oneWord(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r <= ' ' || r == 0x7f || r == '"' || r == '\'' || r == '\\' {
			return false
		}
		if r == 0x85 || r == 0xa0 || (r >= 0x2000 && r <= 0x200a) || r == 0x2028 || r == 0x2029 || r == 0x3000 {
			return false
		}
	}
	return true
}

// readSteps reads steps.json.
func (r *Recipe) readSteps(fsys fs.FS) error {
	var document struct {
		Steps []json.RawMessage `json:"steps"`
	}
	if err := readJSON(fsys, stepsFile, &document); err != nil {
		return err
	}
	if document.Steps == nil {
		return defect(stepsFile, "declares no steps member")
	}
	for i, raw := range document.Steps {
		step, err := r.readStep(fsys, raw, i+1)
		if err != nil {
			return err
		}
		r.Steps = append(r.Steps, step)
	}
	return r.checkOwnership()
}

// readStep reads and validates one step.
func (r *Recipe) readStep(fsys fs.FS, raw json.RawMessage, number int) (Step, error) {
	var members map[string]json.RawMessage
	if err := json.Unmarshal(raw, &members); err != nil {
		return Step{}, defect(stepsFile, "step %d is not an object", number)
	}
	var step Step
	if err := stringMember(members, "id", &step.ID); err != nil {
		return Step{}, defect(stepsFile, "step %d %v", number, err)
	}
	label := fmt.Sprintf("step %s", step.ID)
	if !bench.HarnessName(step.ID) {
		return Step{}, defect(stepsFile, "step %d names the id %q, which is not a harness name", number, step.ID)
	}
	for _, earlier := range r.Steps {
		if earlier.ID == step.ID {
			return Step{}, defect(stepsFile, "two steps carry the id %s", step.ID)
		}
	}
	if err := stringMember(members, "kind", &step.Kind); err != nil {
		return Step{}, defect(stepsFile, "%s %v", label, err)
	}
	own, known := stepKeys[step.Kind]
	if !known {
		return Step{}, defect(stepsFile, "%s declares the kind %s, and the kinds are json-merge, marked-section, write-file and run", label, step.Kind)
	}
	for _, key := range sortedKeys(members) {
		if key == "id" || key == "kind" || key == "scope" {
			continue
		}
		if key == "path" && step.Kind != KindRun {
			continue
		}
		if _, allowed := own[key]; !allowed {
			return Step{}, defect(stepsFile, "%s carries the key %s, which a %s step does not take", label, key, step.Kind)
		}
	}
	if err := stringMember(members, "scope", &step.Scope); err != nil {
		return Step{}, defect(stepsFile, "%s %v", label, err)
	}
	if !contains(r.Scopes, step.Scope) {
		return Step{}, defect(stepsFile, "%s declares the scope %s, which recipe.json does not list", label, step.Scope)
	}
	if r.Source == SourceProject && step.Scope == ScopeUser {
		return Step{}, defect(stepsFile, "%s declares the user scope, and a recipe found in a project's container may declare no user step", label)
	}
	for _, key := range sortedKeys(own) {
		if _, present := members[key]; own[key] && !present {
			return Step{}, defect(stepsFile, "%s carries no %s", label, key)
		}
	}
	if step.Kind == KindRun {
		return r.readRunStep(members, step, label)
	}
	if err := stringMember(members, "path", &step.Path); err != nil {
		return Step{}, defect(stepsFile, "%s %v", label, err)
	}
	if err := checkStepPath(step.Path); err != nil {
		return Step{}, defect(stepsFile, "%s %v", label, err)
	}
	switch step.Kind {
	case KindJSONMerge:
		return r.readJSONMergeStep(members, step, label)
	case KindMarkedSection:
		if err := stringMember(members, "comment", &step.Comment); err != nil {
			return Step{}, defect(stepsFile, "%s %v", label, err)
		}
		if step.Comment != "html" && step.Comment != "hash" {
			return Step{}, defect(stepsFile, "%s declares the comment %s, and the comments are html and hash", label, step.Comment)
		}
	}
	if err := stringMember(members, "template", &step.Template); err != nil {
		return Step{}, defect(stepsFile, "%s %v", label, err)
	}
	if !oneSegment(step.Template) {
		return Step{}, defect(stepsFile, "%s names the template %q, which is not one file name", label, step.Template)
	}
	info, err := fs.Stat(fsys, filesDir+"/"+step.Template)
	if err != nil || info.IsDir() {
		return Step{}, defect(stepsFile, "%s names the template %s, and files/ holds no such file", label, step.Template)
	}
	return step, nil
}

// readRunStep reads a run step's own keys.
func (r *Recipe) readRunStep(members map[string]json.RawMessage, step Step, label string) (Step, error) {
	if r.Source == SourceProject {
		return Step{}, defect(stepsFile, "%s runs a program, and a recipe found in a project's container may declare no run step", label)
	}
	if err := stringMember(members, "program", &step.Program); err != nil {
		return Step{}, defect(stepsFile, "%s %v", label, err)
	}
	if step.Program == "" {
		return Step{}, defect(stepsFile, "%s names no program", label)
	}
	if _, err := scanPlaceholders(step.Program, false); err != nil {
		return Step{}, defect(stepsFile, "%s %v", label, err)
	}
	if raw, present := members["args"]; present {
		if err := json.Unmarshal(raw, &step.Args); err != nil || step.Args == nil && string(raw) != "[]" {
			return Step{}, defect(stepsFile, "%s carries args that are not an array of strings", label)
		}
	}
	for _, arg := range step.Args {
		if _, err := scanPlaceholders(arg, false); err != nil {
			return Step{}, defect(stepsFile, "%s %v", label, err)
		}
	}
	return step, nil
}

// readJSONMergeStep reads a json-merge step's pointer and value.
func (r *Recipe) readJSONMergeStep(members map[string]json.RawMessage, step Step, label string) (Step, error) {
	if err := stringMember(members, "pointer", &step.Pointer); err != nil {
		return Step{}, defect(stepsFile, "%s %v", label, err)
	}
	if _, err := splitPointer(step.Pointer); err != nil {
		return Step{}, defect(stepsFile, "%s %v", label, err)
	}
	value, err := parseValue(members["value"])
	if err != nil {
		return Step{}, defect(stepsFile, "%s carries a value that does not parse (%v)", label, err)
	}
	if value.kind != 'o' || len(value.members) == 0 {
		return Step{}, defect(stepsFile, "%s carries a value that is not an object with at least one member", label)
	}
	if err := checkValuePlaceholders(value); err != nil {
		return Step{}, defect(stepsFile, "%s %v", label, err)
	}
	step.value = value
	return step, nil
}

// stringMember reads one string member of a step.
func stringMember(members map[string]json.RawMessage, key string, into *string) error {
	raw, present := members[key]
	if !present {
		return fmt.Errorf("carries no %s", key)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return fmt.Errorf("carries a %s that is not a string", key)
	}
	return nil
}

// oneSegment reports whether a name is one file name and nothing more.
func oneSegment(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, `/\`)
}

// checkStepPath validates a step's path, literal or rendered: forward
// slashes, relative, and no empty, `.` or `..` segment.
func checkStepPath(stepPath string) error {
	if stepPath == "" {
		return errors.New("carries an empty path")
	}
	if _, err := scanPlaceholders(stepPath, false); err != nil {
		return err
	}
	if strings.Contains(stepPath, `\`) {
		return fmt.Errorf("writes the path %q with a backslash, and a path is written with forward slashes", stepPath)
	}
	if strings.HasPrefix(stepPath, "/") || filepath.IsAbs(stepPath) || filepath.VolumeName(stepPath) != "" || strings.Contains(stepPath, ":") {
		return fmt.Errorf("writes the path %q, which is not relative", stepPath)
	}
	for _, segment := range strings.Split(stepPath, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("writes the path %q, which carries an empty, . or .. segment", stepPath)
		}
	}
	return nil
}

// ownedKey is one location a step owns, before its path is rendered.
type ownedKey struct {
	scope, path, key string
}

// checkOwnership refuses two steps of one scope that own one location, and a
// file one step reads as JSON and another as marked text.
func (r *Recipe) checkOwnership() error {
	owners := map[ownedKey]string{}
	kinds := map[ownedKey]string{}
	for _, step := range r.Steps {
		if step.Kind == KindRun {
			continue
		}
		file := ownedKey{scope: step.Scope, path: step.Path}
		if earlier, taken := kinds[file]; taken {
			if earlier == KindWriteFile || step.Kind == KindWriteFile || earlier != step.Kind {
				return defect(stepsFile, "step %s writes %s, which another step of the %s scope already writes", step.ID, step.Path, step.Scope)
			}
		}
		kinds[file] = step.Kind
		if step.Kind != KindJSONMerge {
			continue
		}
		tokens, _ := splitPointer(step.Pointer)
		for _, member := range step.value.members {
			location := ownedKey{scope: step.Scope, path: step.Path, key: joinPointer(append(append([]string{}, tokens...), member.name))}
			if earlier, taken := owners[location]; taken {
				return defect(stepsFile, "steps %s and %s both own %s in %s", earlier, step.ID, location.key, step.Path)
			}
			owners[location] = step.ID
		}
	}
	return nil
}

// hasRun reports whether any step of a scope runs a program.
func (r *Recipe) hasRun(scope string) bool {
	for _, step := range r.Steps {
		if step.Scope == scope && step.Kind == KindRun {
			return true
		}
	}
	return false
}

// place is one directory setup searches for recipes.
type place struct {
	// source is the place's name in a heading and a listing.
	source string
	// root is the recipes directory, absolute, and empty for the shipped
	// place.
	root string
	// fsys reads the recipes directory.
	fsys fs.FS
}

// places lists where setup searches, in order: the project container at
// project scope, the user base, then the shipped recipes. A project container
// that is the user base is searched once, as the user base.
func places(container, userBase string, projectScope bool) []place {
	var found []place
	if projectScope && container != "" && !sameDirectory(container, userBase) {
		root := filepath.Join(container, "recipes")
		found = append(found, place{source: SourceProject, root: root, fsys: os.DirFS(root)})
	}
	if userBase != "" {
		root := filepath.Join(userBase, "recipes")
		found = append(found, place{source: SourceUser, root: root, fsys: os.DirFS(root)})
	}
	embedded, _ := fs.Sub(shippedRecipes, "recipes")
	found = append(found, place{source: SourceShipped, fsys: embedded})
	return found
}

// holds reports whether a place holds a directory of a name.
func (p place) holds(name string) bool {
	info, err := fs.Stat(p.fsys, name)
	return err == nil && info.IsDir()
}

// open reads the recipe of a name from a place.
func (p place) open(name string) (*Recipe, error) {
	sub, err := fs.Sub(p.fsys, name)
	if err != nil {
		return nil, defect(manifestFile, "cannot be read (%v)", err)
	}
	dir := ""
	if p.root != "" {
		dir = filepath.Join(p.root, name)
	}
	return readRecipe(sub, name, p.source, dir)
}

// names lists the recipe directories a place holds, in name order.
func (p place) names() []string {
	entries, err := fs.ReadDir(p.fsys, ".")
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names
}

// sameDirectory reports whether two paths name one directory, asking the
// filesystem rather than comparing spellings.
func sameDirectory(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	first, err := os.Stat(a)
	if err != nil {
		return false
	}
	second, err := os.Stat(b)
	if err != nil {
		return false
	}
	return os.SameFile(first, second)
}

// Listing is one row of dinah setup --list.
type Listing struct {
	// Name is the recipe's name.
	Name string `json:"name"`
	// Title is the manifest's title, empty for a recipe that does not read.
	Title string `json:"title"`
	// Source is project, user or shipped.
	Source string `json:"source"`
	// Used says the name resolves to this row at project scope.
	Used bool `json:"used"`
	// Path is the recipe's directory, empty for a shipped recipe.
	Path string `json:"path"`
	// Malformed is why the recipe cannot be used, empty when it can.
	Malformed string `json:"malformed"`
}

// List reports every recipe setup can find and which one each name resolves
// to at project scope. container is the .dinah directory holding the resolved
// workbench, empty when none resolved, in which case the project place is not
// searched and nothing is refused.
func List(container, userBase string) []Listing {
	searched := places(container, userBase, true)
	byName := map[string][]Listing{}
	for _, p := range searched {
		for _, name := range p.names() {
			row := Listing{Name: name, Source: p.source}
			if p.root != "" {
				row.Path = filepath.Join(p.root, name)
			}
			recipe, err := p.open(name)
			if err != nil {
				row.Malformed = err.Error()
			} else {
				row.Title = recipe.Title
			}
			byName[name] = append(byName[name], row)
		}
	}
	var names []string
	for name := range byName {
		names = append(names, name)
	}
	sort.Strings(names)
	rows := []Listing{}
	for _, name := range names {
		group := byName[name]
		group[0].Used = true
		rows = append(rows, group...)
	}
	return rows
}

// sortedKeys lists a map's keys in order, so a defect naming one of several
// is the same defect on every run.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
