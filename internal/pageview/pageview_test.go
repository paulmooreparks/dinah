package pageview

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// Parts written the way the documents write them are offered for rich editing,
// and each comes back byte for byte.
func TestPartsComeBackAsTheyWere(t *testing.T) {
	parts := []string{
		"# Keyboard Navigation",
		"### 3.2 Open Issues",
		"A plain paragraph.",
		"A paragraph\nover two lines.",
		"Some **bold**, some *italic*, and `code`.",
		"A [link](https://example.com/page) and ~~struck~~ text.",
		"**Bold with *italic* inside** and after.",
		"Braces {{like these}} are plain text.",
		"Escaped \\*star\\* and a snake_case_name.",
		"- one\n- two\n- three",
		"- one\n  - nested\n- two",
		"1. first\n2. second\n3. third",
		"- loose one\n\n- loose two",
		"* star bullets\n* here",
		"+ plus bullets",
		"- [ ] a task\n- [x] done\n- plain",
		"* [ ] Make a checklist\n* [ ] Check an item off the checklist",
		"1. [x] numbered task\n2. [ ] another",
		"| Field | Value |\n| --- | --- |\n| Version | 0042 |\n| Build | 7 |",
		"| A | B |\n| --- | --- |\n| `x` | **y** |",
	}
	for _, part := range parts {
		doc, err := Parse(part)
		if err != nil {
			t.Errorf("%q: %v", part, err)
			continue
		}
		back, err := Serialize(doc)
		if err != nil || back != part {
			t.Errorf("%q came back as %q (%v)", part, back, err)
		}
	}
}

// Parts written in a form the serializer does not write are edited as
// source, so editing never rewrites text the person did not touch.
func TestOtherFormsAreEditedAsSource(t *testing.T) {
	parts := []string{
		"Setext heading\n==============",
		"1) paren numbers",
		"Some __bold__ underscores.",
		"Some _italic_ underscores.",
		"A [link](https://x.example \"with a title\").",
		"Line with a hard break  \nafter it.",
		"- [X] an upper-case tick",
		"| A | B |\n| :-- | --: |\n| 1 | 2 |",
		"> a quote",
		"```\ncode\n```",
		"Raw <span>html</span> inline.",
		"",
	}
	for _, part := range parts {
		if _, err := Parse(part); !errors.Is(err, ErrNotEditable) {
			t.Errorf("%q: want ErrNotEditable, got %v", part, err)
		}
	}
}

// Spaces Markdown does not show are not a reason to edit as source: a table
// written without padding is editable, and comes back padded.
func TestUnshownSpacesAreIgnored(t *testing.T) {
	cases := map[string]string{
		"| A | B |\n|---|---|\n|1|  |": "| A | B |\n| --- | --- |\n| 1 | |",
		"Trailing space \nnext line.":  "Trailing space\nnext line.",
	}
	for part, want := range cases {
		doc, err := Parse(part)
		if err != nil {
			t.Errorf("%q: %v", part, err)
			continue
		}
		if got, _ := Serialize(doc); got != want {
			t.Errorf("%q came back as %q, want %q", part, got, want)
		}
	}
}

// A bare URL is a link written as itself.
func TestBareURLIsALink(t *testing.T) {
	doc, err := Parse("See https://example.com/page?id=1_2 for more.")
	if err != nil {
		t.Fatal(err)
	}
	p := doc.Content[0]
	if len(p.Content) != 3 || len(p.Content[1].Marks) != 1 || p.Content[1].Marks[0].Type != "link" || p.Content[1].Marks[0].Attrs["auto"] != true {
		t.Fatalf("%+v", p.Content)
	}
}

// Text typed in the editor that Markdown would read as markup is escaped, and
// reads back as the same text.
func TestTypedMarkupIsEscaped(t *testing.T) {
	typed := []string{
		"2 * 3 * 4",
		"a `tick` and [brackets]",
		"_leading underscore",
		"# not a heading",
		"- not a bullet",
		"1. not a number",
		"a ~~b~~ c",
		"less <than> sign",
		`a back\slash and \*`,
	}
	for _, s := range typed {
		doc := &Node{Type: "doc", Content: []*Node{{Type: "paragraph", Content: []*Node{{Type: "text", Text: s}}}}}
		md, err := Serialize(doc)
		if err != nil {
			t.Fatal(err)
		}
		again, err := Parse(md)
		if err != nil {
			t.Errorf("%q wrote %q, which does not read back: %v", s, md, err)
			continue
		}
		if got := plain(again); got != s {
			t.Errorf("%q wrote %q, which reads back as %q", s, md, got)
		}
	}
}

// A pipe typed into a table cell is escaped so the row keeps its columns.
func TestPipeInACellIsEscaped(t *testing.T) {
	cell := func(s string, header bool) *Node {
		typ := "table_cell"
		if header {
			typ = "table_header"
		}
		return &Node{Type: typ, Content: []*Node{{Type: "text", Text: s}}}
	}
	doc := &Node{Type: "doc", Content: []*Node{{Type: "table", Content: []*Node{
		{Type: "table_row", Content: []*Node{cell("A", true), cell("B", true)}},
		{Type: "table_row", Content: []*Node{cell("x | y", false), cell("z", false)}},
	}}}}
	md, err := Serialize(doc)
	if err != nil {
		t.Fatal(err)
	}
	want := "| A | B |\n| --- | --- |\n| x \\| y | z |"
	if md != want {
		t.Fatalf("got %q want %q", md, want)
	}
}

// Marks shared by neighbouring text stay open across them, and an edit that
// bolds part of a run writes one pair of delimiters.
func TestMarksSpanNeighbours(t *testing.T) {
	strong := []Mark{{Type: "strong"}}
	doc := &Node{Type: "doc", Content: []*Node{{Type: "paragraph", Content: []*Node{
		{Type: "text", Text: "plain "},
		{Type: "text", Text: "bold ", Marks: strong},
		{Type: "text", Text: "and both", Marks: []Mark{{Type: "strong"}, {Type: "em"}}},
		{Type: "text", Text: " end"},
	}}}}
	md, err := Serialize(doc)
	if err != nil {
		t.Fatal(err)
	}
	if want := "plain **bold *and both*** end"; md != want {
		t.Fatalf("got %q want %q", md, want)
	}
}

// The document survives the trip through JSON that the front end makes.
func TestDocumentSurvivesJSON(t *testing.T) {
	part := "1. first **one**\n2. second *two*"
	doc, err := Parse(part)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(doc)
	var back Node
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	md, err := Serialize(&back)
	if err != nil || md != part {
		t.Fatalf("got %q (%v)", md, err)
	}
}

func plain(n *Node) string {
	var b strings.Builder
	var walk func(*Node)
	walk = func(n *Node) {
		b.WriteString(n.Text)
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}
