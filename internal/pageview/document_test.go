package pageview

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"studio/internal/preview"
)

func previewOf(md string) []preview.Block {
	blocks, _ := preview.Render(md)
	return blocks
}

func parseDoc(text string) *Node {
	return ParseDocument(text, Units(previewOf(text)))
}

func TestChecklistInDocumentIsRichEditable(t *testing.T) {
	text := "### This Is a Checklist\n\n* [ ] Make a checklist\n* [ ] Check an item off the checklist\n* [ ] Take a nap\n\n- Cut and deletion of code-backed items in the editor.\n"
	doc := parseDoc(text)
	if back, err := SerializeDocument(viaJSON(t, doc)); err != nil || back != text {
		t.Fatalf("checklist round trip changed the document: %q (%v)", back, err)
	}
	for _, node := range doc.Content {
		if strings.Contains(stringAttr(node, "src"), "Make a checklist") {
			if node.Type != "bullet_list" || len(node.Content) != 3 {
				t.Fatalf("checklist became %s: %+v", node.Type, node.Attrs)
			}
			node.Content[0].Attrs["checked"] = true
			back, err := SerializeDocument(viaJSON(t, doc))
			if err != nil || back != strings.Replace(text, "* [ ] Make", "* [x] Make", 1) {
				t.Fatalf("ticking the checklist changed other content: %q (%v)", back, err)
			}
			return
		}
	}
	t.Fatal("checklist was not found")
}

// viaJSON sends a document through JSON, as the page does.
func viaJSON(t *testing.T, doc *Node) *Node {
	t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var back Node
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	return &back
}

const sample = "---\ntype: design-note\n---\n\n# Tokens\n\nThe lexer reads the **token**   stream.  \n\n" +
	"- one\n- two\n\n| A |B|\n|---|---|\n| 1 | {{braces}} |\n\n```d2\na -> b\n```\n\n" +
	"> A *quoted*\n> passage.\n\n![image](sources/a.png)\n\nLast.\n"

// A document read and written back unchanged is the same text, byte for
// byte, its odd spacing included.
func TestDocumentComesBackAsItWas(t *testing.T) {
	doc := viaJSON(t, parseDoc(sample))
	got, err := SerializeDocument(doc)
	if err != nil || got != sample {
		t.Fatalf("got %q (%v)", got, err)
	}
	kinds := []string{}
	for _, n := range doc.Content {
		kinds = append(kinds, n.Type)
	}
	want := "heading paragraph bullet_list table source_block source_block source_block paragraph"
	if strings.Join(kinds, " ") != want {
		t.Fatalf("blocks %v", kinds)
	}
	diagram := doc.Content[4]
	if diagrams, _ := diagram.Attrs["diagrams"].([]any); len(diagrams) != 1 || !strings.Contains(stringAttr(diagram, "html"), "data-preview-diagram") {
		t.Fatalf("diagram %+v", diagram)
	}
}

// An edit changes the block it was made in, and no other.
func TestAnEditChangesOnlyItsBlock(t *testing.T) {
	doc := viaJSON(t, parseDoc(sample))
	// Edit the list's first item and add a paragraph at the end.
	doc.Content[2].Content[0].Content[0].Content[0].Text = "first"
	doc.Content = append(doc.Content, &Node{Type: "paragraph", Content: []*Node{{Type: "text", Text: "New."}}})
	got, err := SerializeDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSuffix(strings.Replace(sample, "- one\n", "- first\n", 1), "\n") + "\n\nNew.\n"
	if got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

// A paragraph split in two, as Enter does, keeps neither copy of the
// original's place, and an emptied block writes nothing.
func TestSplitAndEmptiedBlocks(t *testing.T) {
	doc := viaJSON(t, parseDoc("# T\n\nOne two.\n\nGone.\n"))
	p := doc.Content[1]
	second := &Node{Type: "paragraph", Attrs: map[string]any{}, Content: []*Node{{Type: "text", Text: "two."}}}
	p.Content = []*Node{{Type: "text", Text: "One"}}
	doc.Content = []*Node{doc.Content[0], p, second, {Type: "paragraph", Attrs: doc.Content[2].Attrs}}
	got, err := SerializeDocument(doc)
	if err != nil || got != "# T\n\nOne\n\ntwo.\n" {
		t.Fatalf("got %q (%v)", got, err)
	}
}

// Blocks read from a snippet and put into a document are written as the
// editor has them, after a blank line, and a source block keeps its Markdown.
func TestInsertedBlocks(t *testing.T) {
	doc := viaJSON(t, parseDoc("# T\n\nText.\n"))
	fresh := Fresh(parseDoc("New *words*.\n\n```\ncode\n```\n").Content)
	if len(fresh) != 2 || fresh[0].Type != "paragraph" || fresh[0].Attrs["uid"] != nil || fresh[0].Attrs["src"] != nil || fresh[1].Type != "source_block" {
		t.Fatalf("%+v", fresh)
	}
	doc.Content = append(doc.Content, viaJSON(t, &Node{Type: "doc", Content: fresh}).Content...)
	got, err := SerializeDocument(doc)
	if err != nil || got != "# T\n\nText.\n\nNew *words*.\n\n```\ncode\n```\n" {
		t.Fatalf("got %q (%v)", got, err)
	}
}

// An empty table, as Insert › Table makes it, is written as a table that
// the preview draws and the editor reads back as the same table.
func TestInsertedEmptyTable(t *testing.T) {
	cell := func(typ string) *Node {
		return &Node{Type: typ, Attrs: map[string]any{"colspan": 1, "rowspan": 1, "colwidth": nil}}
	}
	row := func(typ string) *Node {
		return &Node{Type: "table_row", Content: []*Node{cell(typ), cell(typ), cell(typ)}}
	}
	doc := viaJSON(t, parseDoc("# T\n"))
	doc.Content = append(doc.Content, &Node{Type: "table", Content: []*Node{row("table_header"), row("table_cell"), row("table_cell")}})
	got, err := SerializeDocument(doc)
	want := "# T\n\n| | | |\n| --- | --- | --- |\n| | | |\n| | | |\n"
	if err != nil || got != want {
		t.Fatalf("got %q (%v)", got, err)
	}
	again := parseDoc(got)
	if len(again.Content) != 2 || again.Content[1].Type != "table" || len(again.Content[1].Content) != 3 {
		t.Fatalf("read back as %+v", again.Content)
	}
}

// Windows line ends are kept.
func TestCRLFIsKept(t *testing.T) {
	text := "# T\r\n\r\nText.\r\n"
	got, err := SerializeDocument(viaJSON(t, parseDoc(text)))
	if err != nil || got != text {
		t.Fatalf("got %q (%v)", got, err)
	}
}

// Every local document listed in DINAH_CORPUS comes back byte for byte.
func TestCorpusComesBack(t *testing.T) {
	list := os.Getenv("DINAH_CORPUS")
	if list == "" {
		t.Skip("set DINAH_CORPUS to a file listing documents")
	}
	raw, _ := os.ReadFile(list)
	n := 0
	for _, path := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		data, err := os.ReadFile(strings.TrimSpace(path))
		if err != nil {
			continue
		}
		text := string(data)
		got, err := SerializeDocument(viaJSON(t, parseDoc(text)))
		if err != nil || got != text {
			at := 0
			for at < len(got) && at < len(text) && got[at] == text[at] {
				at++
			}
			t.Errorf("%s did not come back at byte %d (%v): had %q, wrote %q", path, at, err,
				text[max(0, at-40):min(len(text), at+40)], got[max(0, at-40):min(len(got), at+40)])
		}
		n++
	}
	t.Logf("%d documents", n)
}
