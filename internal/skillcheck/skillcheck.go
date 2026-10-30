// Package skillcheck checks a skill package without running it: its front
// matter against the Agent Skills specification (https://agentskills.io/specification)
// and Devin's skill format (https://docs.devin.ai/cli/extensibility/skills/creating-skills),
// its argument guidance, and the files its instructions refer to. The Skill
// window shows the problems in the Editor's problems pane, and a bundle
// build runs the same checks.
package skillcheck

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Problem is one thing found, at a line of SKILL.md.
type Problem struct {
	Line     int    `json:"line"`
	Severity string `json:"severity"` // error, warning or info
	Message  string `json:"message"`
}

// Input is what Check reads.
type Input struct {
	Text   string   // SKILL.md
	Folder string   // the package folder's name
	Files  []string // the package's files, relative to the folder, with forward slashes
	// WorkspaceHas, when set, says whether a workspace path such as
	// .devin/rules/x.md exists in the skill's workspace.
	WorkspaceHas func(path string) bool
}

// Known fields of the two formats. A field of neither is kept and reported
// as information, because a provider may read what this package does not
// know.
var (
	agentSkillsFields = map[string]bool{"name": true, "description": true, "license": true, "compatibility": true, "metadata": true, "allowed-tools": true}
	devinFields       = map[string]bool{"name": true, "description": true, "argument-hint": true, "model": true, "subagent": true, "agent": true, "allowed-tools": true, "permissions": true, "triggers": true}
	// agentSkillsName is the name the Agent Skills specification allows.
	agentSkillsName = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
)

// Check runs every check and returns the problems in line order.
func Check(in Input) []Problem {
	var out []Problem
	add := func(line int, severity, format string, args ...any) {
		out = append(out, Problem{Line: line, Severity: severity, Message: fmt.Sprintf(format, args...)})
	}
	lines := strings.Split(strings.ReplaceAll(in.Text, "\r\n", "\n"), "\n")
	meta, end, err := frontMatter(lines)
	switch {
	case end == 0:
		add(1, "error", "the skill has no front matter; it needs a block between --- lines with its name and description")
	case err != nil:
		add(2, "error", "the front matter is not valid YAML, so Devin would not offer the skill: %v", err)
	default:
		checkFields(meta, lines, end, in.Folder, add)
	}
	body := lines
	if end > 0 {
		body = lines[end:]
	}
	checkPaths(body, end, in, add)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Line < out[j].Line })
	if out == nil {
		out = []Problem{}
	}
	return out
}

// frontMatter parses the block, and gives the line count it occupies (0 when
// there is none) and each key's line.
func frontMatter(lines []string) (meta map[string]any, end int, err error) {
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, 0, nil
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i + 1
			break
		}
	}
	if end == 0 {
		return nil, 0, fmt.Errorf("the front matter has no closing --- line")
	}
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end-1], "\n")), &node); err != nil {
		return nil, end, err
	}
	meta = map[string]any{}
	if err := node.Decode(&meta); err != nil {
		return nil, end, err
	}
	return meta, end, nil
}

// keyLine is the line of a top-level key in the block, 1-based in the file.
func keyLine(lines []string, end int, key string) int {
	for i := 1; i < end-1; i++ {
		if strings.HasPrefix(lines[i], key+":") {
			return i + 1
		}
	}
	return 1
}

func checkFields(meta map[string]any, lines []string, end int, folder string, add func(int, string, string, ...any)) {
	name, _ := meta["name"].(string)
	switch {
	case meta["name"] == nil:
		add(1, "warning", "the front matter has no name; add name: %s, which both skill formats expect", folder)
	case name != folder:
		add(keyLine(lines, end, "name"), "error", "the name %q is not the folder's name %q; Devin invokes the skill by its folder, and the Agent Skills format requires the two to match", name, folder)
	case !agentSkillsName.MatchString(name) || len(name) > 64:
		add(keyLine(lines, end, "name"), "warning", "the name %q is not one the Agent Skills format allows: up to 64 lowercase letters, digits and single hyphens", name)
	}
	description, _ := meta["description"].(string)
	switch {
	case meta["description"] == nil:
		add(1, "error", "the front matter has no description, which Devin shows beside the command and agents choose the skill by")
	case strings.TrimSpace(description) == "":
		add(keyLine(lines, end, "description"), "error", "the description is empty")
	case len(description) > 1024:
		add(keyLine(lines, end, "description"), "warning", "the description is longer than the 1024 characters the Agent Skills format allows")
	}
	if hint, ok := meta["argument-hint"].(string); ok {
		if msg := checkHint(hint); msg != "" {
			add(keyLine(lines, end, "argument-hint"), "error", "the argument-hint cannot be read as a parameter form: %s", msg)
		}
	} else if meta["argument-hint"] != nil {
		add(keyLine(lines, end, "argument-hint"), "error", "the argument-hint must be one string")
	}
	if tools, ok := meta["allowed-tools"]; ok {
		switch tools.(type) {
		case string, []any:
		default:
			add(keyLine(lines, end, "allowed-tools"), "warning", "allowed-tools is a list (Devin) or a space-separated string (Agent Skills), not %T", tools)
		}
	}
	keys := make([]string, 0, len(meta))
	for k := range meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !agentSkillsFields[k] && !devinFields[k] {
			add(keyLine(lines, end, k), "info", "%s is not a field of the Agent Skills or the Devin skill format; it is kept as it is", k)
		}
	}
}

// checkHint reads the argument guidance the way Chat's parameter form does
// (frontend/skill-inputs.js) and reports what the form could not read: an
// unclosed bracket or quote.
func checkHint(hint string) string {
	for i := 0; i < len(hint); {
		c := hint[i]
		switch {
		case c == ' ' || c == '\t':
			i++
		case c == '[':
			end := strings.IndexByte(hint[i:], ']')
			if end < 0 {
				return fmt.Sprintf("the [ at column %d has no ]", i+1)
			}
			i += end + 1
		case c == '"' && i+1 < len(hint) && hint[i+1] == '<':
			end := strings.Index(hint[i:], `>"`)
			if end < 0 {
				return fmt.Sprintf(`the "< at column %d has no >"`, i+1)
			}
			i += end + 2
		case c == '<':
			end := strings.IndexByte(hint[i:], '>')
			if end < 0 {
				return fmt.Sprintf("the < at column %d has no >", i+1)
			}
			i += end + 1
		default:
			for i < len(hint) && hint[i] != ' ' && hint[i] != '\t' {
				i++
			}
		}
	}
	return ""
}

// pathToken is a relative path in the instructions: at least one folder,
// a file name with an extension, no spaces, as `references/checklist.md` or
// `.devin/rules/writing-style.md`.
var pathToken = regexp.MustCompile(`(?:^|[\s` + "`" + `'"(\[])((?:\.?[A-Za-z0-9_.-]+/)+[A-Za-z0-9_.-]+\.[A-Za-z0-9]{1,8})(?:[\s` + "`" + `'")\],.;:]|$)`)

func checkPaths(body []string, end int, in Input, add func(int, string, string, ...any)) {
	files := map[string]bool{}
	folders := map[string]bool{}
	for _, f := range in.Files {
		files[f] = true
		for p := f; strings.Contains(p, "/"); {
			p = p[:strings.LastIndex(p, "/")]
			folders[p] = true
		}
	}
	seen := map[string]bool{}
	for i, line := range body {
		for _, m := range pathToken.FindAllStringSubmatch(line, -1) {
			p := strings.TrimPrefix(m[1], "./")
			if seen[p] || strings.Contains(p, "://") {
				continue
			}
			seen[p] = true
			first := p[:strings.Index(p, "/")]
			at := end + i + 1
			switch {
			case files[p]:
			case folders[first]:
				add(at, "warning", "%s is not in the package, though its folder %s is", p, first)
			case strings.HasPrefix(p, ".devin/"):
				if in.WorkspaceHas != nil && !in.WorkspaceHas(p) {
					add(at, "warning", "%s is not in the workspace", p)
				}
			}
		}
	}
}
