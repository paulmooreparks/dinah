package setup

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"sort"
	"strings"

	"dinah/internal/contract"
)

// mergeMember plans one member, reporting false when the member is a conflict
// or the file cannot be read.
func (p *planner) mergeMember(f *fileState, tokens []string, name, key string, desired []byte) (Change, bool) {
	change := Change{Kind: KindJSONMerge, File: f.rel, Key: key, After: indentJSON(desired, "")}
	if !f.exists {
		f.data = []byte("{}\n")
		f.exists = true
		f.created = true
	}
	root, err := parseJSONFile(f.data)
	if err != nil {
		p.markUnreadable(f, err.Error())
		return change, false
	}
	parent, depth, err := walk(root, tokens)
	if err != nil {
		p.markUnreadable(f, err.Error())
		return change, false
	}
	if depth < len(tokens) {
		f.data = p.createParents(f, tokens)
		root, _ = parseJSONFile(f.data)
		parent, _, _ = walk(root, tokens)
		f.data = addMember(f.data, parent, name, desired)
		change.Change = fileChange(f)
		return change, true
	}
	member, _, err := parent.member(name)
	if err != nil {
		p.markUnreadable(f, err.Error()+" at "+key)
		return change, false
	}
	if member == nil {
		f.data = addMember(f.data, parent, name, desired)
		change.Change = fileChange(f)
		return change, true
	}
	current := compactSpan(f.data[member.value.start:member.value.end])
	change.Before = indentJSON(current, "")
	if bytes.Equal(current, desired) {
		change.Change = ChangeUnchanged
		return change, true
	}
	entry := p.entryFor(f, key)
	if entry == nil || entry.Digest != digestOf(current) {
		p.conflict(f, key)
		return change, false
	}
	f.data = replaceMemberValue(f.data, member, desired)
	change.Change = ChangeUpdate
	return change, true
}

// createParents adds the missing objects on a pointer's path, each empty for
// the caller to fill, and records which it created.
func (p *planner) createParents(f *fileState, tokens []string) []byte {
	data := f.data
	for {
		root, err := parseJSONFile(data)
		if err != nil {
			return data
		}
		parent, depth, err := walk(root, tokens)
		if err != nil || depth == len(tokens) {
			return data
		}
		data = addMember(data, parent, tokens[depth], []byte("{}"))
		p.createdParent(slashPath(f.abs), joinPointer(tokens[:depth+1]))
	}
}

// createdParent records a parent object this run created in a file.
func (p *planner) createdParent(file, pointer string) {
	if p.parents == nil {
		p.parents = map[string][]string{}
	}
	if !contains(p.parents[file], pointer) {
		p.parents[file] = append(p.parents[file], pointer)
	}
}

// planSection plans a marked-section step.
func (p *planner) planSection(planned *plannedStep, f *fileState) {
	step := planned.step
	key := p.recipe.Name + "/" + step.ID
	p.produced[[2]string{slashPath(f.abs), key}] = true
	rendered, _ := renderText(p.recipe.templates[step.Template], p.facts, true)
	body := sectionBody(rendered)
	change := Change{Step: step.ID, Kind: KindMarkedSection, File: f.rel, Key: key, After: body}
	switch {
	case !f.exists:
		f.data = []byte(sectionText(step.Comment, key, body, "\n"))
		f.exists = true
		f.created = true
		change.Change = ChangeCreate
	default:
		lines := splitLines(f.data)
		s, err := locateSection(lines, step.Comment, key)
		if err != nil {
			p.markUnreadable(f, err.Error())
			return
		}
		if !s.found {
			f.data = appendSection(f.data, step.Comment, key, body)
			change.Change = ChangeAdd
			break
		}
		change.Before = s.body
		if s.body == body {
			change.Change = ChangeUnchanged
			break
		}
		entry := p.entryFor(f, key)
		if entry == nil || entry.Digest != digestOf([]byte(s.body)) {
			p.conflict(f, key)
			return
		}
		f.data = replaceSectionBody(f.data, lines, s, body)
		change.Change = ChangeUpdate
	}
	planned.changes = append(planned.changes, change)
	planned.writes = append(planned.writes, snapshot{file: f, data: f.data, exists: f.exists})
	planned.entries = append(planned.entries, p.newEntry(f, entrySection, key, digestOf([]byte(body))))
}

// normalised is a whole file's text with CRLF written as LF, which is the form
// a whole file's digest and comparison read.
func normalised(data []byte) string {
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

// planWriteFile plans a write-file step.
func (p *planner) planWriteFile(planned *plannedStep, f *fileState) {
	step := planned.step
	p.produced[[2]string{slashPath(f.abs), ""}] = true
	rendered, _ := renderText(p.recipe.templates[step.Template], p.facts, true)
	desired := normalised([]byte(rendered))
	change := Change{Step: step.ID, Kind: KindWriteFile, File: f.rel, After: desired}
	switch {
	case !f.exists:
		f.data = []byte(rendered)
		f.exists = true
		f.created = true
		change.Change = ChangeCreate
	default:
		current := normalised(f.data)
		change.Before = current
		if current == desired {
			change.Change = ChangeUnchanged
			break
		}
		entry := p.entryFor(f, "")
		if entry == nil || entry.Digest != digestOf([]byte(current)) {
			p.conflict(f, "")
			return
		}
		f.data = []byte(rendered)
		change.Change = ChangeUpdate
	}
	planned.changes = append(planned.changes, change)
	planned.writes = append(planned.writes, snapshot{file: f, data: f.data, exists: f.exists})
	planned.entries = append(planned.entries, p.newEntry(f, entryFile, "", digestOf([]byte(desired))))
}

// planStale plans the removal of every location the ledger records for this
// run that the current steps no longer produce, which is how an apply
// converges the files on what the run says.
func (p *planner) planStale(stale *plannedStep) {
	for _, e := range p.ledger.ofRun(p.recipe.Name, p.scope, p.baseKey) {
		if p.produced[[2]string{e.File, e.Key}] {
			continue
		}
		p.planTakeBack(stale, e)
	}
}

// planTakeBack plans taking back one location the ledger records: gone when
// it is absent, removed when it is still owned, and a conflict otherwise.
func (p *planner) planTakeBack(planned *plannedStep, e ledgerEntry) {
	f := p.fileByKey(e.File)
	if f.unreadable != "" {
		return
	}
	var change Change
	var ok bool
	switch e.Kind {
	case entryJSONMember:
		change, ok = p.takeBackMember(f, e)
	case entrySection:
		change, ok = p.takeBackSection(f, e)
	default:
		change, ok = p.takeBackFile(f, e)
	}
	if !ok {
		return
	}
	change.Step = p.stepFor(f, e)
	planned.changes = append(planned.changes, change)
	planned.writes = append(planned.writes, snapshot{file: f, data: f.data, exists: f.exists})
}

// takeBackMember plans taking back one JSON member.
func (p *planner) takeBackMember(f *fileState, e ledgerEntry) (Change, bool) {
	change := Change{Kind: KindJSONMerge, File: f.rel, Key: e.Key, Change: ChangeGone}
	if !f.exists {
		return change, true
	}
	root, err := parseJSONFile(f.data)
	if err != nil {
		p.markUnreadable(f, err.Error())
		return change, false
	}
	tokens, err := splitPointer(e.Key)
	if err != nil || len(tokens) == 0 {
		p.markUnreadable(f, "is named in the ledger by the pointer "+e.Key)
		return change, false
	}
	parent, depth, err := walk(root, tokens[:len(tokens)-1])
	if err != nil {
		p.markUnreadable(f, err.Error())
		return change, false
	}
	if depth < len(tokens)-1 {
		return change, true
	}
	member, index, err := parent.member(tokens[len(tokens)-1])
	if err != nil {
		p.markUnreadable(f, err.Error()+" at "+e.Key)
		return change, false
	}
	if member == nil {
		return change, true
	}
	current := compactSpan(f.data[member.value.start:member.value.end])
	change.Before = indentJSON(current, "")
	if digestOf(current) != e.Digest {
		p.conflict(f, e.Key)
		return change, false
	}
	f.data = removeMember(f.data, parent, index)
	change.Change = ChangeRemove
	return change, true
}

// takeBackSection plans taking back one marked section. The ledger does not
// record the marker syntax, so the recipe's own step answers it, and both
// syntaxes are tried when the recipe no longer carries the step.
func (p *planner) takeBackSection(f *fileState, e ledgerEntry) (Change, bool) {
	change := Change{Kind: KindMarkedSection, File: f.rel, Key: e.Key, Change: ChangeGone}
	if !f.exists {
		return change, true
	}
	lines := splitLines(f.data)
	var s section
	for _, comment := range p.commentsFor(e.Key) {
		found, err := locateSection(lines, comment, e.Key)
		if err != nil {
			p.markUnreadable(f, err.Error())
			return change, false
		}
		if found.found {
			s = found
			break
		}
	}
	if !s.found {
		return change, true
	}
	change.Before = s.body
	if digestOf([]byte(s.body)) != e.Digest {
		p.conflict(f, e.Key)
		return change, false
	}
	f.data = removeSection(f.data, lines, s)
	change.Change = ChangeRemove
	return change, true
}

// commentsFor lists the marker syntaxes a section key may be written in: the
// recipe step's own when the recipe carries it, and both otherwise.
func (p *planner) commentsFor(key string) []string {
	for _, step := range p.recipe.Steps {
		if step.Kind == KindMarkedSection && p.recipe.Name+"/"+step.ID == key {
			return []string{step.Comment}
		}
	}
	return []string{"html", "hash"}
}

// takeBackFile plans taking back a whole file.
func (p *planner) takeBackFile(f *fileState, e ledgerEntry) (Change, bool) {
	change := Change{Kind: KindWriteFile, File: f.rel, Change: ChangeGone}
	if !f.exists {
		return change, true
	}
	current := normalised(f.data)
	change.Before = current
	if digestOf([]byte(current)) != e.Digest {
		p.conflict(f, "")
		return change, false
	}
	f.data = nil
	f.exists = false
	change.Change = ChangeRemove
	return change, true
}

// stepFor names the recipe step that owns a ledger entry's location, empty
// when the recipe no longer carries one.
func (p *planner) stepFor(f *fileState, e ledgerEntry) string {
	for _, step := range p.recipe.Steps {
		if step.Scope != p.scope || step.Kind == KindRun {
			continue
		}
		if step.Kind == KindMarkedSection {
			if p.recipe.Name+"/"+step.ID == e.Key {
				return step.ID
			}
			continue
		}
		rel, err := p.renderPath(step)
		if err != nil || rel != f.rel {
			continue
		}
		if step.Kind == KindWriteFile && e.Kind == entryFile {
			return step.ID
		}
		if step.Kind != KindJSONMerge || e.Kind != entryJSONMember {
			continue
		}
		tokens, _ := splitPointer(step.Pointer)
		for _, member := range step.value.members {
			if joinPointer(append(append([]string{}, tokens...), member.name)) == e.Key {
				return step.ID
			}
		}
	}
	return ""
}

// recordCreation completes each new ledger entry with what this run and the
// runs before it created: the file, when either created it, and every parent
// object on the member's path that either created.
func (p *planner) recordCreation() {
	createdFile := map[string]bool{}
	parents := map[string][]string{}
	for file, created := range p.parents {
		parents[file] = append(parents[file], created...)
	}
	for _, e := range p.ledger.ofRun(p.recipe.Name, p.scope, p.baseKey) {
		if e.CreatedFile {
			createdFile[e.File] = true
		}
		for _, parent := range e.CreatedParents {
			if !contains(parents[e.File], parent) {
				parents[e.File] = append(parents[e.File], parent)
			}
		}
	}
	for _, f := range p.files {
		if f.created {
			createdFile[slashPath(f.abs)] = true
		}
	}
	for _, planned := range p.steps {
		for i := range planned.entries {
			e := &planned.entries[i]
			e.CreatedFile = createdFile[e.File]
			if e.Kind == entryJSONMember {
				e.CreatedParents = ancestorsOf(e.Key, parents[e.File])
			}
		}
	}
}

// ancestorsOf returns the pointers among a set that are ancestors of a key,
// outermost first.
func ancestorsOf(key string, pointers []string) []string {
	var found []string
	for _, pointer := range pointers {
		if strings.HasPrefix(key, pointer+"/") {
			found = append(found, pointer)
		}
	}
	sort.SliceStable(found, func(i, j int) bool {
		return strings.Count(found[i], "/") < strings.Count(found[j], "/")
	})
	return found
}

// report starts the report of this run, with a text rendered as its prompt.
func (p *planner) report(prompt string) *Report {
	rendered, _ := renderText(prompt, p.facts, true)
	workbench := ""
	if p.scope == ScopeProject {
		workbench = slashPath(p.opts.Workbench)
	}
	path := ""
	if p.recipe.Dir != "" {
		path = slashPath(p.recipe.Dir)
	}
	return &Report{
		Recipe: RecipeInfo{
			Name:   p.recipe.Name,
			Title:  p.recipe.Title,
			Source: p.recipe.Source,
			Path:   path,
		},
		Scope:     p.scope,
		Base:      p.baseKey,
		Workbench: workbench,
		DryRun:    p.opts.DryRun,
		Remove:    p.opts.Remove,
		Changes:   []Change{},
		Prompt:    rendered,
		BaseDir:   p.base,
	}
}

// execute carries the plan out step by step, recording in the ledger what
// each data step wrote. A run step that fails stops the run with the data
// steps before it applied and recorded.
func (p *planner) execute(report *Report) (*Report, error) {
	var committed []ledgerEntry
	for _, planned := range p.steps {
		if planned.step.Kind == KindRun {
			if refusal := p.runProgram(planned); refusal != nil {
				kept := p.ledger.replaceRun(p.recipe.Name, p.scope, p.baseKey, committed, supersededBy(committed))
				if err := p.ledger.write(kept); err != nil {
					return nil, err
				}
				return nil, &StepFailed{Refusal: refusal, Step: planned.step.ID, Report: report}
			}
			report.Changes = append(report.Changes, planned.changes...)
			continue
		}
		if err := writeSnapshots(planned.writes); err != nil {
			return nil, err
		}
		report.Changes = append(report.Changes, planned.changes...)
		committed = append(committed, planned.entries...)
	}
	recorded := p.ledger.replaceRun(p.recipe.Name, p.scope, p.baseKey, committed, nil)
	if err := p.ledger.write(recorded); err != nil {
		return nil, err
	}
	return report, nil
}

// supersededBy keeps the old entries of a run that no committed entry
// replaces, which is what a run that stopped partway leaves recorded.
func supersededBy(committed []ledgerEntry) func(ledgerEntry) bool {
	replaced := map[[2]string]bool{}
	for _, e := range committed {
		replaced[[2]string{e.File, e.Key}] = true
	}
	return func(e ledgerEntry) bool {
		return !replaced[[2]string{e.File, e.Key}]
	}
}

// writeSnapshots writes each file a step changed, once, as the step left it,
// and deletes one the step took away. A file whose bytes would not change is
// not written.
func writeSnapshots(writes []snapshot) error {
	last := map[*fileState]snapshot{}
	var order []*fileState
	for _, w := range writes {
		if _, seen := last[w.file]; !seen {
			order = append(order, w.file)
		}
		last[w.file] = w
	}
	for _, f := range order {
		w := last[f]
		if w.exists == f.onDiskExists && bytes.Equal(w.data, f.onDisk) {
			continue
		}
		if !w.exists {
			if err := os.Remove(f.abs); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			f.onDisk, f.onDiskExists = nil, false
			continue
		}
		if err := writeFileAtomic(f.abs, w.data); err != nil {
			return err
		}
		f.onDisk, f.onDiskExists = w.data, true
	}
	return nil
}

// remove takes back every location the ledger records for this recipe, scope
// and base.
func (p *planner) remove() (*Report, error) {
	entries := p.ledger.ofRun(p.recipe.Name, p.scope, p.baseKey)
	if len(entries) == 0 {
		report := p.report("")
		report.Nothing = true
		return report, nil
	}
	planned := &plannedStep{}
	for _, e := range entries {
		p.fileByKey(e.File)
		p.planTakeBack(planned, e)
	}
	if p.unreadable != "" {
		return nil, contract.Refuse(contract.SetupUnreadableTarget, p.unreadable)
	}
	if len(p.conflicts) > 0 {
		return nil, refuseListing(contract.SetupConflict, "locations", strings.Join(p.conflicts, "\n"))
	}
	p.planTidy(planned, entries)
	report := p.report(p.recipe.Remove)
	report.Changes = append(report.Changes, planned.changes...)
	if p.opts.DryRun {
		return report, nil
	}
	if err := writeSnapshots(planned.writes); err != nil {
		return nil, err
	}
	if err := p.ledger.write(p.ledger.replaceRun(p.recipe.Name, p.scope, p.baseKey, nil, nil)); err != nil {
		return nil, err
	}
	return report, nil
}

// planTidy plans what the take-backs leave behind in each file the run
// reached: the parent objects setup created that are now empty, and the file
// itself where the rule on an emptied file holds. It is the one caller of
// tidy, and it runs at plan time on the apply path and on the removal path
// alike, so a dry run reports exactly what the apply after it carries out.
func (p *planner) planTidy(planned *plannedStep, entries []ledgerEntry) {
	for _, f := range p.ordered {
		leftover, removed := p.tidy(planned, f, entries)
		if removed {
			planned.changes = append(planned.changes, Change{
				Kind:   KindEmptiedFile,
				File:   f.rel,
				Change: ChangeRemove,
				Before: leftover,
			})
		}
		planned.writes = append(planned.writes, snapshot{file: f, data: f.data, exists: f.exists})
	}
}

// removedFrom reports whether the plan takes something out of a file, which
// is the condition that setup removes only a file this run emptied itself. A
// file whose every take-back found its location already gone is left as
// whoever emptied it left it.
func (p *planner) removedFrom(planned *plannedStep, f *fileState) bool {
	for _, c := range planned.changes {
		if c.File != f.rel || c.Change != ChangeRemove {
			continue
		}
		if c.Kind == KindJSONMerge || c.Kind == KindMarkedSection || c.Kind == KindWriteFile {
			return true
		}
	}
	return false
}

// producesIn reports whether any step of the current run produces a location
// in a file, which stops the emptied-file rule from removing it. On the
// removal path the map is empty, because a removal produces nothing, so every
// file passes this guard there.
func (p *planner) producesIn(f *fileState) bool {
	file := slashPath(f.abs)
	for location := range p.produced {
		if location[0] == file {
			return true
		}
	}
	return false
}

// tidy removes what a take-back leaves behind in one file: each parent object
// setup created that is now empty, innermost first, and then the file itself
// when all four conditions on an emptied file hold. Those are that the plan
// still leaves a file there, that setup created it, that this run took
// something out of it, and that no current step produces a location in it. It
// reports what the file held when it was removed, and whether it was removed
// at all.
func (p *planner) tidy(planned *plannedStep, f *fileState, entries []ledgerEntry) (string, bool) {
	if !f.exists {
		return "", false
	}
	file := slashPath(f.abs)
	created := false
	isJSON := false
	var parents []string
	for _, e := range entries {
		if e.File != file {
			continue
		}
		created = created || e.CreatedFile
		isJSON = isJSON || e.Kind == entryJSONMember
		for _, parent := range e.CreatedParents {
			if !contains(parents, parent) {
				parents = append(parents, parent)
			}
		}
	}
	sort.SliceStable(parents, func(i, j int) bool {
		return strings.Count(parents[i], "/") > strings.Count(parents[j], "/")
	})
	for _, parent := range parents {
		f.data = removeIfEmpty(f.data, parent)
	}
	if !created || !p.removedFrom(planned, f) || p.producesIn(f) {
		return "", false
	}
	if isJSON {
		root, err := parseJSONFile(f.data)
		if err != nil || len(root.members) > 0 {
			return "", false
		}
	} else if strings.TrimSpace(string(f.data)) != "" {
		return "", false
	}
	leftover := string(f.data)
	f.data, f.exists = nil, false
	return leftover, true
}

// removeIfEmpty removes the object a pointer names when it holds no member.
func removeIfEmpty(data []byte, pointer string) []byte {
	root, err := parseJSONFile(data)
	if err != nil {
		return data
	}
	tokens, err := splitPointer(pointer)
	if err != nil || len(tokens) == 0 {
		return data
	}
	parent, depth, err := walk(root, tokens[:len(tokens)-1])
	if err != nil || depth < len(tokens)-1 {
		return data
	}
	member, index, err := parent.member(tokens[len(tokens)-1])
	if err != nil || member == nil || member.value.kind != '{' || len(member.value.members) > 0 {
		return data
	}
	return removeMember(data, parent, index)
}
