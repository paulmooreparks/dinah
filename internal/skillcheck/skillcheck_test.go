package skillcheck

import (
	"strings"
	"testing"
)

func messages(problems []Problem) string {
	var b strings.Builder
	for _, p := range problems {
		b.WriteString(p.Severity + " " + p.Message + "\n")
	}
	return b.String()
}

func TestACleanSkillHasNoProblems(t *testing.T) {
	in := Input{Folder: "design-review", Files: []string{"SKILL.md", "references/checklist.md"},
		Text: "---\nname: design-review\ndescription: Review a design doc.\nargument-hint: '<PROJECT> \"<Title>\" [Area]'\nmodel: sonnet\n---\n\nRead `references/checklist.md`, then run `git status`.\n"}
	if got := Check(in); len(got) != 0 {
		t.Fatalf("%s", messages(got))
	}
}

func TestEachCheck(t *testing.T) {
	cases := []struct {
		name  string
		in    Input
		want  string
		level string
	}{
		{"no front matter", Input{Folder: "x", Text: "# Just text\n"}, "has no front matter", "error"},
		{"bad yaml", Input{Folder: "x", Text: "---\ndescription: [broken\n---\n"}, "not valid YAML", "error"},
		{"name differs", Input{Folder: "x", Text: "---\nname: y\ndescription: d\n---\n"}, "is not the folder's name", "error"},
		{"name shape", Input{Folder: "My_Skill", Text: "---\nname: My_Skill\ndescription: d\n---\n"}, "not one the Agent Skills format allows", "warning"},
		{"no name", Input{Folder: "x", Text: "---\ndescription: d\n---\n"}, "has no name", "warning"},
		{"no description", Input{Folder: "x", Text: "---\nname: x\n---\n"}, "has no description", "error"},
		{"empty description", Input{Folder: "x", Text: "---\nname: x\ndescription: ''\n---\n"}, "description is empty", "error"},
		{"hint unclosed", Input{Folder: "x", Text: "---\nname: x\ndescription: d\nargument-hint: '<TITLE [Area]'\n---\n"}, "no >", "error"},
		{"unknown field", Input{Folder: "x", Text: "---\nname: x\ndescription: d\nowner: me\n---\n"}, "owner is not a field", "info"},
		{"missing package file", Input{Folder: "x", Files: []string{"SKILL.md", "references/a.md"}, Text: "---\nname: x\ndescription: d\n---\nRead references/b.md.\n"}, "references/b.md is not in the package", "warning"},
		{"missing workspace file", Input{Folder: "x", WorkspaceHas: func(string) bool { return false }, Text: "---\nname: x\ndescription: d\n---\nRead `.devin/rules/style.md`.\n"}, ".devin/rules/style.md is not in the workspace", "warning"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Check(c.in)
			found := false
			for _, p := range got {
				if strings.Contains(p.Message, c.want) {
					found = true
					if p.Severity != c.level {
						t.Errorf("severity %s, want %s", p.Severity, c.level)
					}
				}
			}
			if !found {
				t.Errorf("no problem says %q:\n%s", c.want, messages(got))
			}
		})
	}
}

func TestProblemLinesAndSilence(t *testing.T) {
	in := Input{Folder: "x", Files: []string{"SKILL.md"}, WorkspaceHas: func(string) bool { return true },
		Text: "---\nname: x\ndescription: d\nargument-hint: '<a'\n---\n\nSee https://example.com/a/b.md and .devin/rules/ok.md.\n\nRun `git status`.\n"}
	got := Check(in)
	if len(got) != 1 || got[0].Line != 4 {
		t.Fatalf("%+v", got)
	}
}
