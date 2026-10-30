package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"studio/internal/drafts"
	"studio/internal/frontmatter"
	"studio/internal/skillcheck"
	"studio/internal/workspaces"
)

// The Skill window edits a skill through the documents service, as the
// Editor edits any file: SKILL.md is opened as a draft, the form over its
// front matter changes one line of that draft at a time, and the checks of
// package skillcheck reach the Editor's problems pane through the draft's
// preview.

// SkillFields is a skill draft's front matter as the form shows it.
type SkillFields struct {
	Revision string               `json:"revision"`
	Folder   string               `json:"folder"`
	Present  bool                 `json:"present"`
	Fields   []frontmatter.Field  `json:"fields"`
	Problems []skillcheck.Problem `json:"problems"`
}

// skillOf says whether a draft is a skill's SKILL.md, and gives the package
// folder's name: a file called SKILL.md in a folder of a skills collection,
// or of the person's own ~/skills.
func skillOf(origin drafts.Origin) (folder string, ok bool) {
	p := origin.Path
	if path.Base(p) != "SKILL.md" || !strings.Contains(p, "/") {
		return "", false
	}
	if origin.WorkspaceID == "" && origin.SourceID == workspaces.HomeSourceID {
		if !strings.HasPrefix(p, "skills/") {
			return "", false
		}
	} else if kind, isCollection := workspaces.CollectionKind(origin.SourceID); !isCollection || kind.Key != "skills" {
		return "", false
	}
	return path.Base(path.Dir(p)), true
}

// OpenSkillDraft opens a skill's SKILL.md as a draft, the one the Skill
// window's Edit tab embeds the Editor on.
func (a *App) OpenSkillDraft(id string) (drafts.View, error) {
	pack, err := a.ReadSkill(id)
	if err != nil {
		return drafts.View{}, err
	}
	if pack.Skill.Ref == nil {
		return drafts.View{}, errors.New("this command has no instruction package to edit")
	}
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return drafts.View{}, err
	}
	return o.Open(*pack.Skill.Ref)
}

// SkillFields reads a skill draft's front matter for the form.
func (a *App) SkillFields(draftID string) (SkillFields, error) {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return SkillFields{}, err
	}
	d, err := o.Draft(draftID)
	if err != nil {
		return SkillFields{}, err
	}
	folder, ok := skillOf(d.Origin)
	if !ok {
		return SkillFields{}, errors.New("this draft is not a skill's SKILL.md")
	}
	fields, present := frontmatter.Fields(d.Text)
	if fields == nil {
		fields = []frontmatter.Field{}
	}
	return SkillFields{Revision: d.Revision, Folder: folder, Present: present, Fields: fields, Problems: a.skillProblems(d, folder)}, nil
}

// SetSkillField sets one front-matter key of a skill draft, changing that
// line and no other, provided the draft is still at expected. The name is
// the package folder's and is changed with RenameSkill.
func (a *App) SetSkillField(draftID, expected, key, value string) (drafts.View, error) {
	if key == "name" {
		return drafts.View{}, errors.New("the name is the skill's folder; use Rename")
	}
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return drafts.View{}, err
	}
	d, err := o.Draft(draftID)
	if err != nil {
		return drafts.View{}, err
	}
	if _, ok := skillOf(d.Origin); !ok {
		return drafts.View{}, errors.New("this draft is not a skill's SKILL.md")
	}
	text, err := frontmatter.SetAny(d.Text, key, value)
	if err != nil {
		return drafts.View{}, err
	}
	return o.Update(draftID, expected, text, nil)
}

// CheckSkillDraft runs the skill checks on a draft as it is.
func (a *App) CheckSkillDraft(draftID string) ([]Problem, error) {
	o, err := a.documentOps(a.bindingContext(), "")
	if err != nil {
		return nil, err
	}
	d, err := o.Draft(draftID)
	if err != nil {
		return nil, err
	}
	folder, ok := skillOf(d.Origin)
	if !ok {
		return nil, errors.New("this draft is not a skill's SKILL.md")
	}
	return skillProblemsOf(a.skillProblems(d, folder)), nil
}

// skillProblems are the checks of a skill draft, against the package's
// files on disk and, for a workspace skill, the workspace's files.
func (a *App) skillProblems(d drafts.Draft, folder string) []skillcheck.Problem {
	in := skillcheck.Input{Text: d.Text, Folder: folder}
	ref := ResourceRef{WorkspaceID: d.Origin.WorkspaceID, SourceID: d.Origin.SourceID, Path: path.Dir(d.Origin.Path)}
	if dir, isDir, err := (workspaceVFS{service: a.resourceService()}).localPath(ref); err == nil && isDir {
		in.Files = packageFiles(dir)
	}
	if d.Origin.WorkspaceID != "" {
		for _, record := range (workspaceVFS{service: a.resourceService()}).records() {
			if record.ID == d.Origin.WorkspaceID {
				if root := workspaces.ExecutionPath(record); root != "" {
					in.WorkspaceHas = func(p string) bool {
						if strings.Contains(p, "..") {
							return false
						}
						_, err := os.Stat(filepath.Join(root, filepath.FromSlash(p)))
						return err == nil
					}
				}
			}
		}
	}
	return skillcheck.Check(in)
}

func skillProblemsOf(problems []skillcheck.Problem) []Problem {
	out := make([]Problem, 0, len(problems))
	for _, p := range problems {
		out = append(out, Problem{Line: p.Line, Severity: p.Severity, Message: p.Message, Rule: "skill"})
	}
	return out
}

// packageFiles lists a package's regular files, relative to it with forward
// slashes. Links are not followed, and a package with more files than any
// skill has is cut off, so a wrong folder cannot be walked whole.
func packageFiles(dir string) []string {
	const most = 2000
	var out []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || len(out) >= most {
			return fs.SkipAll
		}
		if d.Type().IsRegular() {
			if rel, err := filepath.Rel(dir, p); err == nil {
				out = append(out, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	return out
}

// CopySkillToMine copies a skill's package into the person's own skills, as
// an editable package of its own, and returns it. A skill of that name among
// the person's own is not replaced.
func (a *App) CopySkillToMine(id string) (CatalogSkill, error) {
	pack, err := a.ReadSkill(id)
	if err != nil {
		return CatalogSkill{}, err
	}
	if pack.Skill.Ref == nil {
		return CatalogSkill{}, errors.New("this command has no instruction package to copy")
	}
	from := *pack.Skill.Ref
	from.Path = path.Dir(from.Path)
	src, isDir, err := (workspaceVFS{service: a.resourceService()}).localPath(from)
	if err != nil {
		return CatalogSkill{}, err
	}
	if !isDir {
		return CatalogSkill{}, errors.New("the skill's package folder could not be found")
	}
	name := path.Base(from.Path)
	mine := filepath.Join(configDir(), "home", "skills")
	dst := filepath.Join(mine, name)
	if _, err := os.Stat(dst); err == nil {
		return CatalogSkill{}, fmt.Errorf("you already have a skill called %s; rename one of them first", name)
	}
	if err := os.MkdirAll(mine, 0o755); err != nil {
		return CatalogSkill{}, err
	}
	if err := copyPackage(src, dst); err != nil {
		os.RemoveAll(dst)
		return CatalogSkill{}, err
	}
	a.emitWorkspaceMountChanged("content-changed", "", workspaces.HomeSourceID)
	copied, err := a.ReadSkill(skillResourceID(ResourceRef{SourceID: workspaces.HomeSourceID, Path: "skills/" + name + "/SKILL.md"}))
	if err != nil {
		return CatalogSkill{}, err
	}
	return copied.Skill, nil
}

// copyPackage copies a package's regular files; links and anything else are
// left out, so a copy brings only files into the person's skills.
func copyPackage(src, dst string) error {
	for _, rel := range packageFiles(src) {
		data, err := os.ReadFile(filepath.Join(src, filepath.FromSlash(rel)))
		if err != nil {
			return err
		}
		target := filepath.Join(dst, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// RenameSkill renames a skill's package folder and sets the name in its
// front matter to match, and returns the skill under its new identity. A
// draft of the skill with unsaved changes stops the rename, because the
// draft would be left on a file that no longer exists.
func (a *App) RenameSkill(id, name string) (CatalogSkill, error) {
	name = strings.TrimSpace(name)
	if name == "" || name != filepath.Base(name) || strings.HasPrefix(name, ".") || strings.ContainsAny(name, `/\:*?"<>|`) {
		return CatalogSkill{}, errors.New("a skill's name is its folder's name: no slashes, no leading dot, none of the characters a file name cannot have")
	}
	pack, err := a.ReadSkill(id)
	if err != nil {
		return CatalogSkill{}, err
	}
	if pack.Skill.Ref == nil {
		return CatalogSkill{}, errors.New("this command has no package to rename")
	}
	svc, err := a.draftService()
	if err != nil {
		return CatalogSkill{}, err
	}
	origin := drafts.Origin{Kind: "local-folder", WorkspaceID: pack.Skill.Ref.WorkspaceID, SourceID: pack.Skill.Ref.SourceID, Path: pack.Skill.Ref.Path}
	open, _ := svc.Store.FindByOrigin(origin)
	for _, d := range open {
		if d.Changed() {
			return CatalogSkill{}, errors.New("the skill has unsaved changes; save or discard them before renaming it")
		}
	}
	from := *pack.Skill.Ref
	from.Path = path.Dir(from.Path)
	src, isDir, err := (workspaceVFS{service: a.resourceService()}).localPath(from)
	if err != nil {
		return CatalogSkill{}, err
	}
	if !isDir {
		return CatalogSkill{}, errors.New("the skill's package folder could not be found")
	}
	dst := filepath.Join(filepath.Dir(src), name)
	if _, err := os.Stat(dst); err == nil {
		return CatalogSkill{}, fmt.Errorf("there is already a skill called %s there", name)
	}
	if err := os.Rename(src, dst); err != nil {
		return CatalogSkill{}, err
	}
	file := filepath.Join(dst, "SKILL.md")
	if raw, err := os.ReadFile(file); err == nil {
		if text, err := frontmatter.SetAny(string(raw), "name", name); err == nil {
			_ = os.WriteFile(file, []byte(text), 0o644)
		}
	}
	// The draft of the old file, if any, is forgotten: it held no changes.
	for _, d := range open {
		_ = svc.Store.Discard(d.ID, d.Revision)
	}
	a.emitWorkspaceMountChanged("content-changed", pack.Skill.Ref.WorkspaceID, pack.Skill.Ref.SourceID)
	renamed := *pack.Skill.Ref
	renamed.Path = path.Dir(path.Dir(renamed.Path)) + "/" + name + "/SKILL.md"
	renamed.Path = strings.TrimPrefix(renamed.Path, "./")
	result, err := a.ReadSkill(skillResourceID(renamed))
	if err != nil {
		return CatalogSkill{}, err
	}
	return result.Skill, nil
}
