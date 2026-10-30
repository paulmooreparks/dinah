// Package preview draws a Markdown document as HTML for the window's
// previews and for the Editor's Page view. It parses with goldmark (GitHub
// Flavored Markdown) and returns the document's top-level blocks one by one,
// each with the source lines it came from, so a preview can scroll with the
// text and Page view can edit each block by its lines.
//
// Nothing here reads the filesystem or the network. A local image is left as
// declarative markup (data-attachment, the file's name) for the page to load,
// and a diagram's source is handed to the page to draw.
package preview

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Block is one top-level block of a document: its HTML and the 1-based
// source line it starts on, front matter counted.
//
// The HTML is wrapped twice. The outer element is the block, which the page
// reads for scroll sync:
//
//	<div class="dinah-block" data-line="L">...</div>
//
// The inner element marks the block's source lines and its kind, for Page
// view (pageview.Units):
//
//	<div class="dinah-src" data-start="S" data-end="E" data-kind="K">...</div>
//
// The kind is one of paragraph, heading, list, tasklist, table, code, quote,
// rule, html or other.
type Block struct {
	Line     int       `json:"line"`
	HTML     string    `json:"html"`
	Diagrams []Diagram `json:"diagrams,omitempty"`
}

// Diagram is a diagram the page draws from its source. Line is the line of
// the block that holds it. ID is unique within one document's preview and
// is the value of the data-preview-diagram attribute on its placeholder.
type Diagram struct {
	ID       string `json:"id"`
	Language string `json:"language"`
	Source   string `json:"source"`
	Line     int    `json:"line"`
}

// Problem is something in the document the preview cannot show as written.
type Problem struct {
	Line    int
	Message string
}

// diagramLanguages are the fenced-code languages the page draws as diagrams.
var diagramLanguages = map[string]bool{"mermaid": true, "d2": true}

// FrontMatterEnd returns the number of leading lines taken by a YAML front
// matter block (its opening fence, its body and its closing fence), or 0
// when there is none. The opener is a first line that is exactly "---", and
// the closer is the next line that starts with "---".
func FrontMatterEnd(lines []string) int {
	if len(lines) == 0 || lines[0] != "---" {
		return 0
	}
	for i := 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "---") {
			return i + 1
		}
	}
	return 0
}

// Render draws a Markdown document as its top-level blocks. Front matter is
// not drawn, but its lines are counted, so every block's lines are lines of
// the document as the person sees it.
func Render(markdown string) ([]Block, []Problem) {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	// Front matter becomes blank lines, which Markdown ignores, so the
	// lines of everything after it keep their numbers.
	front := FrontMatterEnd(lines)
	for i := 0; i < front; i++ {
		lines[i] = ""
	}
	src := []byte(strings.Join(lines, "\n"))

	d := &drawer{}
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
		goldmark.WithRendererOptions(
			// The document is the person's own, and inline HTML (<u>, a
			// sized <img>) is part of how it is written.
			html.WithUnsafe(),
			renderer.WithNodeRenderers(util.Prioritized(d, 100)),
		),
	)
	doc := md.Parser().Parse(text.NewReader(src))

	var problems []Problem
	var blocks []Block
	for _, u := range units(doc, src) {
		d.line = u.start
		first := len(d.diagrams)
		var b bytes.Buffer
		for _, n := range u.nodes {
			if t, ok := n.(*extast.Table); ok {
				problems = append(problems, droppedCells(t, src)...)
			}
			if err := md.Renderer().Render(&b, src, n); err != nil {
				b.WriteString(`<p>` + escape(err.Error()) + `</p>`)
			}
		}
		inner := `<div class="dinah-src" data-start="` + strconv.Itoa(u.start) + `" data-end="` + strconv.Itoa(u.end) +
			`" data-kind="` + u.kind + `">` + b.String() + `</div>`
		blocks = append(blocks, Block{
			Line:     u.start,
			HTML:     `<div class="dinah-block" data-line="` + strconv.Itoa(u.start) + `">` + inner + `</div>`,
			Diagrams: append([]Diagram(nil), d.diagrams[first:]...),
		})
	}
	return blocks, problems
}

// unit is one top-level block with its first and last source line (1-based)
// and its kind. A block whose place in the source cannot be found (it has no
// text of its own) joins the block before it.
type unit struct {
	nodes      []ast.Node
	start, end int
	kind       string
}

// units finds the document's top-level blocks and their lines: from the
// line a block's first character is on to the line before the next block's,
// less blank lines.
func units(doc ast.Node, src []byte) []unit {
	var out []unit
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		off, ok := firstOffset(n, src)
		if !ok && len(out) > 0 {
			out[len(out)-1].nodes = append(out[len(out)-1].nodes, n)
			continue
		}
		out = append(out, unit{nodes: []ast.Node{n}, start: lineOf(src, off), kind: kindOf(n)})
	}
	lines := bytes.Split(src, []byte("\n"))
	for i := range out {
		end := len(lines)
		if i+1 < len(out) {
			end = out[i+1].start - 1
		}
		for end > out[i].start && len(bytes.TrimSpace(lines[end-1])) == 0 {
			end--
		}
		out[i].end = end
	}
	return out
}

// lineOf is the 1-based line of a byte offset in src.
func lineOf(src []byte, off int) int {
	if off > len(src) {
		off = len(src)
	}
	return bytes.Count(src[:off], []byte("\n")) + 1
}

// firstOffset is the offset of a block's first source character: where the
// parser opened it, or, for a block it made from another (a table from a
// paragraph), the opening fence of a fenced code block, or else the first
// character of its first line or of its first inline text, found depth
// first.
func firstOffset(n ast.Node, src []byte) (int, bool) {
	if p := n.Pos(); p >= 0 && p <= len(src) {
		return p, true
	}
	switch node := n.(type) {
	case *ast.FencedCodeBlock:
		start := -1
		if node.Info != nil {
			start = node.Info.Segment.Start
		} else if node.Lines().Len() > 0 {
			// The fence is on the line before the first line of code.
			start = node.Lines().At(0).Start
			if start > 0 {
				start--
			}
		} else {
			return 0, false
		}
		return bytes.LastIndexByte(src[:start], '\n') + 1, true
	case *ast.Text:
		return node.Segment.Start, true
	}
	if n.Type() == ast.TypeBlock && n.Lines().Len() > 0 {
		return n.Lines().At(0).Start, true
	}
	for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
		if off, ok := firstOffset(ch, src); ok {
			return off, true
		}
	}
	return 0, false
}

// kindOf is the kind of a top-level block.
func kindOf(n ast.Node) string {
	switch node := n.(type) {
	case *ast.Paragraph, *ast.TextBlock:
		return "paragraph"
	case *ast.Heading:
		return "heading"
	case *ast.List:
		if isTaskList(node) {
			return "tasklist"
		}
		return "list"
	case *extast.Table:
		return "table"
	case *ast.FencedCodeBlock, *ast.CodeBlock:
		return "code"
	case *ast.Blockquote:
		return "quote"
	case *ast.ThematicBreak:
		return "rule"
	case *ast.HTMLBlock:
		return "html"
	}
	return "other"
}

// isTaskList reports whether any item of a list starts with a checkbox.
func isTaskList(list *ast.List) bool {
	for item := list.FirstChild(); item != nil; item = item.NextSibling() {
		if block := item.FirstChild(); block != nil {
			if _, ok := block.FirstChild().(*extast.TaskCheckBox); ok {
				return true
			}
		}
	}
	return false
}

// droppedCells reports the body rows of a table that have more cells than
// its header row. GFM fixes a table's width at its delimiter row, and
// goldmark drops a longer row's extra cells without a word, so the source
// line is read to find them: whatever follows the last parsed cell, after
// its pipe, was dropped.
func droppedCells(t *extast.Table, src []byte) []Problem {
	var out []Problem
	for row := t.FirstChild(); row != nil; row = row.NextSibling() {
		if _, header := row.(*extast.TableHeader); header {
			continue
		}
		stop := -1
		for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
			// Cells goldmark adds to pad a short row have no source.
			if cell.Lines().Len() > 0 {
				stop = cell.Lines().At(0).Stop
			}
		}
		if stop < 0 || stop > len(src) {
			continue
		}
		eol := stop
		for eol < len(src) && src[eol] != '\n' {
			eol++
		}
		rest := strings.TrimSpace(string(src[stop:eol]))
		rest = strings.TrimSpace(strings.TrimPrefix(rest, "|"))
		rest = strings.TrimSpace(strings.TrimSuffix(rest, "|"))
		if rest != "" {
			out = append(out, Problem{Line: lineOf(src, stop), Message: "This table row has more cells than its header row, and the extra cells are not shown: " + strconv.Quote(abbreviate(rest, 60)) + ". Make the header and delimiter rows as wide as the widest row."})
		}
	}
	return out
}

// abbreviate shortens s to at most n runes for quoting in a message.
func abbreviate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "..."
}

// drawer draws the nodes the preview draws differently from goldmark's own
// HTML: diagrams, local images, and the <img> tags of inline HTML. It keeps
// the diagrams it has drawn, and the line of the block being drawn.
type drawer struct {
	line     int
	diagrams []Diagram
}

func (d *drawer) RegisterFuncs(r renderer.NodeRendererFuncRegisterer) {
	r.Register(ast.KindFencedCodeBlock, d.fencedCode)
	r.Register(ast.KindImage, d.image)
	r.Register(ast.KindRawHTML, d.rawHTML)
	r.Register(ast.KindHTMLBlock, d.htmlBlock)
}

// fencedCode draws a diagram as a placeholder for the page to draw into, and
// any other fenced block as code.
func (d *drawer) fencedCode(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.FencedCodeBlock)
	language := string(n.Language(src))
	var code strings.Builder
	for i := 0; i < n.Lines().Len(); i++ {
		seg := n.Lines().At(i)
		code.Write(seg.Value(src))
	}
	if diagramLanguages[language] {
		id := "preview-diagram-" + strconv.Itoa(len(d.diagrams)+1)
		d.diagrams = append(d.diagrams, Diagram{ID: id, Language: language, Source: code.String(), Line: d.line})
		_, _ = w.WriteString(`<div class="dinah-diagram-placeholder" data-preview-diagram="` + id + `"><pre><code class="language-` + language + `">` + escape(code.String()) + "</code></pre></div>\n")
		return ast.WalkSkipChildren, nil
	}
	_, _ = w.WriteString("<pre><code")
	if language != "" {
		_, _ = w.WriteString(` class="language-` + escape(language) + `"`)
	}
	_, _ = w.WriteString(">" + escape(code.String()) + "</code></pre>\n")
	return ast.WalkSkipChildren, nil
}

// image draws an image on the web as it is, and a local one as the file's
// name for the page to load (data-attachment).
func (d *drawer) image(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.Image)
	_, _ = w.WriteString(imageTag(string(n.Destination), plainText(n, src), "", ""))
	return ast.WalkSkipChildren, nil
}

func (d *drawer) rawHTML(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	n := node.(*ast.RawHTML)
	var b strings.Builder
	for i := 0; i < n.Segments.Len(); i++ {
		seg := n.Segments.At(i)
		b.Write(seg.Value(src))
	}
	_, _ = w.WriteString(localImages(b.String()))
	return ast.WalkSkipChildren, nil
}

func (d *drawer) htmlBlock(w util.BufWriter, src []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.HTMLBlock)
	var b strings.Builder
	for i := 0; i < n.Lines().Len(); i++ {
		seg := n.Lines().At(i)
		b.Write(seg.Value(src))
	}
	if n.HasClosure() {
		b.Write(n.ClosureLine.Value(src))
	}
	_, _ = w.WriteString(localImages(b.String()))
	return ast.WalkContinue, nil
}

// imageTag is the <img> for an image at dest.
func imageTag(dest, alt, width, height string) string {
	size := ""
	if width != "" {
		size += ` width="` + escape(width) + `"`
	}
	if height != "" {
		size += ` height="` + escape(height) + `"`
	}
	if isWeb(dest) {
		return `<img src="` + escape(dest) + `" alt="` + escape(alt) + `"` + size + `>`
	}
	// A path from the document's folder is kept, so the file is read from
	// there (localfs.FilePlaces); an absolute one is looked for by its name.
	name := strings.ReplaceAll(dest, `\`, "/")
	for strings.HasPrefix(name, "./") {
		name = name[2:]
	}
	if strings.HasPrefix(name, "/") || strings.Contains(name, ":") {
		name = name[strings.LastIndex(name, "/")+1:]
	}
	if alt == "" {
		alt = name[strings.LastIndex(name, "/")+1:]
	}
	return `<img class="dinah-attachment-image" data-attachment="` + escape(name) + `" alt="` + escape(alt) + `"` + size + `>`
}

func isWeb(dest string) bool {
	return strings.HasPrefix(dest, "http://") || strings.HasPrefix(dest, "https://") || strings.HasPrefix(dest, "data:image/")
}

var (
	imgTagRe  = regexp.MustCompile(`(?is)<img\b[^>]*?/?>`)
	imgAttrRe = regexp.MustCompile(`(?is)([a-zA-Z_:][-a-zA-Z0-9_:.]*)\s*=\s*(?:"([^"]*)"|'([^']*)')`)
)

// localImages draws each <img> in inline HTML whose source is a local file
// as a local image, keeping its size; other HTML is left as it is.
func localImages(s string) string {
	return imgTagRe.ReplaceAllStringFunc(s, func(tag string) string {
		var src, alt, width, height string
		for _, m := range imgAttrRe.FindAllStringSubmatch(tag, -1) {
			val := m[2] + m[3]
			switch strings.ToLower(m[1]) {
			case "src":
				src = val
			case "alt":
				alt = val
			case "width":
				width = val
			case "height":
				height = val
			}
		}
		if src == "" || isWeb(src) {
			return tag
		}
		return imageTag(src, alt, width, height)
	})
}

// plainText is the text inside a node, without its markup.
func plainText(n ast.Node, src []byte) string {
	var b strings.Builder
	for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
		switch t := ch.(type) {
		case *ast.Text:
			b.Write(t.Segment.Value(src))
		case *ast.String:
			b.Write(t.Value)
		default:
			b.WriteString(plainText(ch, src))
		}
	}
	return b.String()
}

var escaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")

func escape(s string) string { return escaper.Replace(s) }
