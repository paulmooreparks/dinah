package preview

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
)

var unitRe = regexp.MustCompile(`<div class="dinah-src" data-start="(\d+)" data-end="(\d+)" data-kind="([^"]+)">`)

func unitsOf(t *testing.T, md string) []string {
	t.Helper()
	blocks, problems := Render(md)
	if len(problems) > 0 {
		t.Fatalf("%+v", problems)
	}
	var out []string
	for _, b := range blocks {
		ms := unitRe.FindAllStringSubmatch(b.HTML, -1)
		if len(ms) != 1 {
			t.Fatalf("a block holds %d units: %s", len(ms), b.HTML)
		}
		if !strings.HasPrefix(b.HTML, `<div class="dinah-block" data-line="`+ms[0][1]+`">`) {
			t.Fatalf("a block's line is not its first line: %s", b.HTML)
		}
		out = append(out, ms[0][3]+" "+ms[0][1]+"-"+ms[0][2])
	}
	return out
}

// Each top-level block is a block of its own, with its source lines and
// kind, front matter counted.
func TestEachBlockHasItsLines(t *testing.T) {
	md := strings.Join([]string{
		"---",               // 1
		"type: roadmap",     // 2
		"---",               // 3
		"",                  // 4
		"# Title",           // 5
		"",                  // 6
		"First paragraph,",  // 7
		"on two lines.",     // 8
		"",                  // 9
		"- one",             // 10
		"- two",             // 11
		"  - nested",        // 12
		"",                  // 13
		"* [ ] a task",      // 14
		"* [x] done",        // 15
		"",                  // 16
		"| a | b |",         // 17
		"| --- | --- |",     // 18
		"| 1 | 2 |",         // 19
		"",                  // 20
		"```d2",             // 21
		"a -> b",            // 22
		"```",               // 23
		"",                  // 24
		"> quoted",          // 25
		"",                  // 26
		"---",               // 27
		"",                  // 28
		"    indented code", // 29
		"",                  // 30
		"Last one.",         // 31
	}, "\n")
	want := []string{"heading 5-5", "paragraph 7-8", "list 10-12", "tasklist 14-15", "table 17-19", "code 21-23",
		"quote 25-25", "rule 27-27", "code 29-29", "paragraph 31-31"}
	if got := unitsOf(t, md); !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
}

func TestFrontMatterIsNotDrawn(t *testing.T) {
	blocks, _ := Render("---\r\ntitle: Secret\r\n---\r\nBody.\r\n")
	if len(blocks) != 1 || strings.Contains(blocks[0].HTML, "Secret") || blocks[0].Line != 4 {
		t.Fatalf("%+v", blocks)
	}
	if FrontMatterEnd([]string{"---", "a: b", "---", "x"}) != 3 || FrontMatterEnd([]string{"---", "a: b"}) != 0 || FrontMatterEnd([]string{"x"}) != 0 {
		t.Fatal("front matter boundary")
	}
}

func TestGenericMarkdown(t *testing.T) {
	blocks, _ := Render("# Scope\n\n## Scope\n\nSome *em* and **strong**, `code`, ~~gone~~, <u>under</u>, [a link](https://example.com) and https://example.org.\n\n- [x] done\n\n| H |\n| --- |\n| c |\n")
	var html string
	for _, b := range blocks {
		html += b.HTML
	}
	for _, want := range []string{`<h1 id="scope">`, `<h2 id="scope-1">`, "<em>em</em>", "<strong>strong</strong>", "<code>code</code>", "<del>gone</del>",
		"<u>under</u>", `<a href="https://example.com">a link</a>`, `<a href="https://example.org">`, `type="checkbox"`, "<th>H</th>", "<td>c</td>"} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s in %s", want, html)
		}
	}
}

func TestImages(t *testing.T) {
	// A local image keeps its path from the document's folder; an absolute
	// one is looked for by its name.
	blocks, _ := Render("![Chart](images/chart.png)\n\n![](https://example.com/a.png)\n\n<img src=\"./pics/b.png\" width=\"40\">\n\n![](C:\\shots\\c.png)\n")
	var html string
	for _, b := range blocks {
		html += b.HTML
	}
	for _, want := range []string{`<img class="dinah-attachment-image" data-attachment="images/chart.png" alt="Chart">`, `<img src="https://example.com/a.png" alt="">`,
		`<img class="dinah-attachment-image" data-attachment="pics/b.png" alt="b.png" width="40">`, `data-attachment="c.png" alt="c.png"`} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s in %s", want, html)
		}
	}
}

// A diagram is a placeholder in its block, and the block lists it with the
// block's line; its source is escaped, and the ids are stable.
func TestDiagrams(t *testing.T) {
	source := "---\ntitle: Example\n---\n\n```d2\na -> b\n```\n\n- item\n\n  ```mermaid\n  <script>alert(1)</script>\n  ```\n"
	blocks, _ := Render(source)
	var all []Diagram
	for _, b := range blocks {
		for _, d := range b.Diagrams {
			all = append(all, d)
			if d.Line != b.Line || !strings.Contains(b.HTML, `data-preview-diagram="`+d.ID+`"`) {
				t.Fatalf("diagram %+v in %+v", d, b)
			}
		}
		if strings.Contains(b.HTML, "<script>") {
			t.Fatalf("unescaped source %s", b.HTML)
		}
	}
	if len(all) != 2 || all[0].Language != "d2" || all[0].Source != "a -> b\n" || all[1].Language != "mermaid" || all[0].ID == all[1].ID || all[0].Line != 5 {
		t.Fatalf("%+v", all)
	}
	again, _ := Render(source)
	if !reflect.DeepEqual(blocks, again) {
		t.Fatal("ids are not stable")
	}
	plain, _ := Render("```go\nx := 1 < 2\n```\n")
	if len(plain[0].Diagrams) != 0 || !strings.Contains(plain[0].HTML, `<pre><code class="language-go">x := 1 &lt; 2`) {
		t.Fatalf("%+v", plain)
	}
}

func TestDroppedCellsAreAProblem(t *testing.T) {
	_, problems := Render("Intro.\n\n| a | b |\n| --- | --- |\n| 1 | 2 | 3 |\n")
	if len(problems) != 1 || problems[0].Line != 5 || !strings.Contains(problems[0].Message, `"3"`) {
		t.Fatalf("%+v", problems)
	}
}
