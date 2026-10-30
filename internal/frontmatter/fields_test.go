package frontmatter

import "testing"

func TestFieldsTellScalarsFromBlocks(t *testing.T) {
	text := "---\nname: review\ndescription: >-\n  Read the evidence\n  and review.\nargument-hint: '<TITLE> [Area]'\nallowed-tools:\n  - read\npermissions:\n  allow: [Read(**)]\nmodel: sonnet\n---\n# Body\n"
	fields, present := Fields(text)
	if !present || len(fields) != 6 {
		t.Fatalf("%v %+v", present, fields)
	}
	want := map[string][2]any{"name": {"review", true}, "description": {"", false}, "argument-hint": {"<TITLE> [Area]", true}, "allowed-tools": {"", false}, "permissions": {"", false}, "model": {"sonnet", true}}
	for _, f := range fields {
		w := want[f.Key]
		if f.Value != w[0] || f.Scalar != w[1] {
			t.Errorf("%s: %+v", f.Key, f)
		}
	}
	if fields[0].Line != 2 || fields[5].Line != 11 {
		t.Errorf("lines %d %d", fields[0].Line, fields[5].Line)
	}
	if _, present := Fields("# No front matter\n"); present {
		t.Error("front matter found where there is none")
	}
}

func TestSetAnyKeepsTheRestAndRefusesBlocks(t *testing.T) {
	text := "---\nname: review\ndescription: >-\n  Folded.\nargument-hint: '<TITLE>'\n---\n# Body\n"
	out, err := SetAny(text, "argument-hint", "<TITLE> [Area]")
	if err != nil || out != "---\nname: review\ndescription: >-\n  Folded.\nargument-hint: '<TITLE> [Area]'\n---\n# Body\n" {
		t.Fatalf("%q %v", out, err)
	}
	out, err = SetAny(out, "model", "sonnet")
	if err != nil || out != "---\nname: review\ndescription: >-\n  Folded.\nargument-hint: '<TITLE> [Area]'\nmodel: sonnet\n---\n# Body\n" {
		t.Fatalf("%q %v", out, err)
	}
	if _, err := SetAny(out, "description", "One line"); err == nil {
		t.Error("a folded block was replaced")
	}
	for _, bad := range [][2]string{{"a b", "x"}, {"", "x"}, {"name", "two\nlines"}} {
		if _, err := SetAny(out, bad[0], bad[1]); err == nil {
			t.Errorf("SetAny(%q) was accepted", bad[0])
		}
	}
	// A hint that YAML would read as a flow sequence is quoted.
	out, _ = SetAny("# No front matter\n", "argument-hint", "[file] [options]")
	if out != "---\nargument-hint: \"[file] [options]\"\n---\n\n# No front matter\n" {
		t.Errorf("%q", out)
	}
}
