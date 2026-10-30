// Package pageview turns a Markdown document into the document the
// Editor's Page view edits, and back. The editor is ProseMirror, and its document is JSON: a tree of
// nodes with marks. ParseDocument and SerializeDocument (document.go) work on
// a whole document, block by block; this file reads and writes one block, a
// paragraph, a heading, a list or a table, as the preview marks it (see
// preview.Block).
//
// Markdown stays the stored form. Parse reads a block with the same parser
// the preview uses, and Serialize writes the editor's block back as
// Markdown. A block is rich text only when writing back what Parse read gives
// the block's own text, byte for byte apart from spaces Markdown does not
// show (see layout), so editing never changes how text the person did not
// touch is drawn; any other block is edited as its source.
package pageview

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// Node is a ProseMirror node in its JSON form.
type Node struct {
	Type    string         `json:"type"`
	Attrs   map[string]any `json:"attrs,omitempty"`
	Content []*Node        `json:"content,omitempty"`
	Text    string         `json:"text,omitempty"`
	Marks   []Mark         `json:"marks,omitempty"`
}

// Mark is a ProseMirror mark in its JSON form.
type Mark struct {
	Type  string         `json:"type"`
	Attrs map[string]any `json:"attrs,omitempty"`
}

// ErrNotEditable means a part cannot be edited as rich text without risking
// its other text, so it is edited as source.
var ErrNotEditable = errors.New("this part is edited as source")

var md = goldmark.New(goldmark.WithExtensions(extension.GFM))

// Parse reads one part of a document as an editor's document. A part that
// would not be written back exactly as it is returns ErrNotEditable.
func Parse(part string) (*Node, error) {
	part = strings.TrimRight(part, "\n")
	if strings.TrimSpace(part) == "" || strings.Contains(part, "\r") {
		return nil, ErrNotEditable
	}
	src := []byte(part)
	root := md.Parser().Parse(text.NewReader(src))
	p := parser{src: src}
	doc := &Node{Type: "doc"}
	for n := root.FirstChild(); n != nil; n = n.NextSibling() {
		block, err := p.block(n)
		if err != nil {
			return nil, err
		}
		doc.Content = append(doc.Content, block)
	}
	if len(doc.Content) == 0 {
		return nil, ErrNotEditable
	}
	back, err := Serialize(doc)
	if err != nil || layout(back) != layout(part) {
		return nil, ErrNotEditable
	}
	return doc, nil
}

// layout is a part's text without the spaces Markdown does not show: those at
// the end of a line, and the padding around a table's pipes. Two parts with
// the same layout are drawn the same.
func layout(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		line = strings.TrimRight(line, " \t")
		if strings.HasPrefix(line, "|") {
			line = tableRow(line)
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

// tableRow writes a table row's cells with one space either side, and a
// rule's cells as "---".
func tableRow(line string) string {
	var cells []string
	var cell strings.Builder
	for i := 1; i < len(line); i++ {
		switch {
		case line[i] == '\\' && i+1 < len(line):
			cell.WriteByte(line[i])
			i++
			cell.WriteByte(line[i])
		case line[i] == '|':
			cells = append(cells, strings.TrimSpace(cell.String()))
			cell.Reset()
		default:
			cell.WriteByte(line[i])
		}
	}
	if rest := strings.TrimSpace(cell.String()); rest != "" {
		cells = append(cells, rest)
	}
	for i, c := range cells {
		if c != "" && strings.Trim(c, "-") == "" {
			cells[i] = "---"
		}
	}
	return "| " + strings.Join(cells, " | ") + " |"
}

type parser struct {
	src []byte
}

func (p parser) block(n ast.Node) (*Node, error) {
	switch node := n.(type) {
	case *ast.Paragraph, *ast.TextBlock:
		inline, err := p.inline(n)
		if err != nil {
			return nil, err
		}
		return &Node{Type: "paragraph", Content: inline}, nil
	case *ast.Heading:
		if !p.atx(node) {
			return nil, ErrNotEditable
		}
		inline, err := p.inline(n)
		if err != nil {
			return nil, err
		}
		return &Node{Type: "heading", Attrs: map[string]any{"level": node.Level}, Content: inline}, nil
	case *ast.List:
		return p.list(node)
	case *extast.Table:
		return p.table(node)
	}
	return nil, ErrNotEditable
}

// atx reports whether a heading is written with leading #s, the only form
// written back.
func (p parser) atx(h *ast.Heading) bool {
	if h.Lines().Len() == 0 {
		return false
	}
	start := h.Lines().At(0).Start
	line := start
	for line > 0 && p.src[line-1] != '\n' {
		line--
	}
	return strings.HasPrefix(strings.TrimLeft(string(p.src[line:start]), " "), "#")
}

func (p parser) list(l *ast.List) (*Node, error) {
	if l.IsOrdered() {
		if l.Marker != '.' {
			return nil, ErrNotEditable
		}
	}
	out := &Node{Type: "bullet_list", Attrs: map[string]any{"tight": l.IsTight}}
	if l.IsOrdered() {
		out.Type = "ordered_list"
		out.Attrs["order"] = l.Start
	} else if l.Marker != '-' {
		// A list written with * or + is written back with it.
		out.Attrs["marker"] = string(l.Marker)
	}
	for item := l.FirstChild(); item != nil; item = item.NextSibling() {
		li := &Node{Type: "list_item"}
		// A task item's box is the first inline of its first block (goldmark's
		// task list extension); the item keeps whether it is ticked, and the
		// box is left out of the paragraph's text (walk).
		if first := item.FirstChild(); first != nil {
			if box, ok := first.FirstChild().(*extast.TaskCheckBox); ok {
				li.Attrs = map[string]any{"checked": box.IsChecked}
			}
		}
		for ch := item.FirstChild(); ch != nil; ch = ch.NextSibling() {
			b, err := p.block(ch)
			if err != nil {
				return nil, err
			}
			li.Content = append(li.Content, b)
		}
		if len(li.Content) == 0 || li.Content[0].Type != "paragraph" {
			return nil, ErrNotEditable
		}
		out.Content = append(out.Content, li)
	}
	return out, nil
}

func (p parser) table(t *extast.Table) (*Node, error) {
	for _, a := range t.Alignments {
		if a != extast.AlignNone {
			return nil, ErrNotEditable
		}
	}
	out := &Node{Type: "table"}
	for row := t.FirstChild(); row != nil; row = row.NextSibling() {
		_, header := row.(*extast.TableHeader)
		r := &Node{Type: "table_row"}
		for cell := row.FirstChild(); cell != nil; cell = cell.NextSibling() {
			inline, err := p.inline(cell)
			if err != nil {
				return nil, err
			}
			typ := "table_cell"
			if header {
				typ = "table_header"
			}
			r.Content = append(r.Content, &Node{Type: typ, Attrs: map[string]any{"colspan": 1, "rowspan": 1, "colwidth": nil}, Content: inline})
		}
		out.Content = append(out.Content, r)
	}
	return out, nil
}

// inline reads a block's inline content: text with its marks, and breaks.
func (p parser) inline(n ast.Node) ([]*Node, error) {
	var out []*Node
	err := p.walk(n, nil, &out)
	return out, err
}

func (p parser) walk(n ast.Node, marks []Mark, out *[]*Node) error {
	// A mark opened inside this node by inline HTML (<u>) must close inside
	// it too, or it would not be written back where it was.
	depth := len(marks)
	var run strings.Builder
	flush := func() error {
		if run.Len() == 0 {
			return nil
		}
		nodes, err := p.textNodes(run.String(), &marks)
		if err != nil {
			return err
		}
		*out = append(*out, nodes...)
		run.Reset()
		return nil
	}
	for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
		switch node := ch.(type) {
		case *extast.TaskCheckBox:
			// Kept by its list item (list), not as text.
			if ch != n.FirstChild() {
				return ErrNotEditable
			}
		case *ast.Text:
			run.Write(node.Segment.Value(p.src))
			if node.HardLineBreak() {
				return ErrNotEditable
			}
			if node.SoftLineBreak() {
				if err := flush(); err != nil {
					return err
				}
				*out = append(*out, &Node{Type: "soft_break"})
			}
		case *ast.String:
			run.Write(node.Value)
		case *ast.Emphasis:
			if err := flush(); err != nil {
				return err
			}
			mark := Mark{Type: "em"}
			if node.Level == 2 {
				mark.Type = "strong"
			}
			if !p.starred(node) {
				return ErrNotEditable
			}
			if err := p.walk(node, withMark(marks, mark), out); err != nil {
				return err
			}
		case *extast.Strikethrough:
			if err := flush(); err != nil {
				return err
			}
			if err := p.walk(node, withMark(marks, Mark{Type: "strike"}), out); err != nil {
				return err
			}
		case *ast.CodeSpan:
			if err := flush(); err != nil {
				return err
			}
			var code strings.Builder
			for c := node.FirstChild(); c != nil; c = c.NextSibling() {
				if t, ok := c.(*ast.Text); ok {
					code.Write(t.Segment.Value(p.src))
				}
			}
			*out = append(*out, &Node{Type: "text", Text: code.String(), Marks: withMark(marks, Mark{Type: "code"})})
		case *ast.Link:
			if err := flush(); err != nil {
				return err
			}
			if len(node.Title) > 0 {
				return ErrNotEditable
			}
			link := Mark{Type: "link", Attrs: map[string]any{"href": string(node.Destination), "title": nil}}
			if err := p.walk(node, withMark(marks, link), out); err != nil {
				return err
			}
		case *ast.RawHTML:
			// Underlining is the one piece of inline HTML the documents use, as
			// <u> and </u> around a run of text.
			var tag strings.Builder
			for i := 0; i < node.Segments.Len(); i++ {
				seg := node.Segments.At(i)
				tag.Write(seg.Value(p.src))
			}
			if err := flush(); err != nil {
				return err
			}
			switch tag.String() {
			case "<u>":
				marks = withMark(marks, Mark{Type: "underline"})
			case "</u>":
				if len(marks) == 0 || marks[len(marks)-1].Type != "underline" {
					return ErrNotEditable
				}
				marks = marks[:len(marks)-1]
			default:
				return ErrNotEditable
			}
		case *ast.AutoLink:
			if err := flush(); err != nil {
				return err
			}
			url := string(node.URL(p.src))
			label := string(node.Label(p.src))
			if node.AutoLinkType != ast.AutoLinkURL || url != label {
				return ErrNotEditable
			}
			link := Mark{Type: "link", Attrs: map[string]any{"href": url, "title": nil, "auto": true}}
			*out = append(*out, &Node{Type: "text", Text: label, Marks: withMark(marks, link)})
		default:
			return ErrNotEditable
		}
	}
	if err := flush(); err != nil {
		return err
	}
	if len(marks) != depth {
		return ErrNotEditable
	}
	return nil
}

// starred reports whether emphasis is written with asterisks, the only form
// written back.
func (p parser) starred(e *ast.Emphasis) bool {
	first := e.FirstChild()
	if first == nil {
		return false
	}
	t, ok := first.(*ast.Text)
	if !ok {
		off, ok2 := firstText(e)
		if !ok2 {
			return false
		}
		return off >= e.Level && p.src[off-1] == '*'
	}
	off := t.Segment.Start
	return off >= e.Level && p.src[off-1] == '*'
}

func firstText(n ast.Node) (int, bool) {
	for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
		if t, ok := ch.(*ast.Text); ok {
			return t.Segment.Start, true
		}
		if off, ok := firstText(ch); ok {
			return off, true
		}
	}
	return 0, false
}

func withMark(marks []Mark, m Mark) []Mark {
	out := append([]Mark(nil), marks...)
	return append(out, m)
}

// textNodes reads a run of source text as a text node. The text is shown
// without its Markdown escapes; Serialize puts back the ones it needs.
func (p parser) textNodes(raw string, marks *[]Mark) ([]*Node, error) {
	return []*Node{{Type: "text", Text: unescape(raw), Marks: *marks}}, nil
}

// unescape removes the backslash from a Markdown escape.
func unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && isEscapable(s[i+1]) {
			i++
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isEscapable(c byte) bool {
	return strings.IndexByte("\\`*_{}[]()#+-.!|<>~", c) >= 0
}

// Serialize writes an editor's document as Markdown: ATX headings,
// "-" bullets, "1." numbering, ** and * emphasis, ~~ strikethrough, and
// tables with "| --- |" rules.
func Serialize(doc *Node) (string, error) {
	if doc == nil || doc.Type != "doc" {
		return "", errors.New("not an editor's document")
	}
	var parts []string
	for _, b := range doc.Content {
		s, err := serializeBlock(b, "")
		if err != nil {
			return "", err
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, "\n\n"), nil
}

func serializeBlock(n *Node, indent string) (string, error) {
	switch n.Type {
	case "paragraph":
		s, err := serializeInline(n.Content, false)
		if err != nil {
			return "", err
		}
		s = escapeLineStart(s)
		return strings.ReplaceAll(s, "\n", "\n"+indent), nil
	case "heading":
		level := intAttr(n, "level", 1)
		if level < 1 || level > 6 {
			return "", fmt.Errorf("a heading's level is 1 to 6, not %d", level)
		}
		s, err := serializeInline(n.Content, false)
		if err != nil {
			return "", err
		}
		return strings.Repeat("#", level) + " " + strings.ReplaceAll(s, "\n", " "), nil
	case "bullet_list", "ordered_list":
		return serializeList(n, indent)
	case "table":
		return serializeTable(n)
	}
	return "", fmt.Errorf("a %s cannot be written as Markdown here", n.Type)
}

func serializeList(n *Node, indent string) (string, error) {
	tight, _ := n.Attrs["tight"].(bool)
	order := intAttr(n, "order", 1)
	var items []string
	bullet := "-"
	if m, _ := n.Attrs["marker"].(string); m == "*" || m == "+" {
		bullet = m
	}
	for i, item := range n.Content {
		marker := bullet + " "
		if n.Type == "ordered_list" {
			marker = strconv.Itoa(order+i) + ". "
		}
		inner := indent + strings.Repeat(" ", len(marker))
		// A task item's box goes before its first block's text.
		box := ""
		if checked, ok := item.Attrs["checked"].(bool); ok {
			box = "[ ] "
			if checked {
				box = "[x] "
			}
		}
		var blocks []string
		for j, b := range item.Content {
			s, err := serializeBlock(b, inner)
			if err != nil {
				return "", err
			}
			if j == 0 {
				blocks = append(blocks, indent+marker+box+s)
			} else {
				blocks = append(blocks, inner+strings.TrimLeft(s, " "))
			}
		}
		sep := "\n"
		if !tight {
			sep = "\n\n"
		}
		items = append(items, strings.Join(blocks, sep))
	}
	if tight {
		return strings.Join(items, "\n"), nil
	}
	return strings.Join(items, "\n\n"), nil
}

func serializeTable(n *Node) (string, error) {
	var lines []string
	for i, row := range n.Content {
		var line strings.Builder
		for _, cell := range row.Content {
			s, err := serializeInline(cell.Content, true)
			if err != nil {
				return "", err
			}
			// An empty cell is written "| |".
			line.WriteString("| ")
			if s != "" {
				line.WriteString(s)
				line.WriteString(" ")
			}
		}
		line.WriteString("|")
		lines = append(lines, line.String())
		if i == 0 {
			lines = append(lines, strings.Repeat("| --- ", len(row.Content))+"|")
		}
	}
	return strings.Join(lines, "\n"), nil
}

func intAttr(n *Node, key string, def int) int {
	switch v := n.Attrs[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	}
	return def
}

// markOrder is the order marks are opened in, outermost first.
var markOrder = []string{"link", "strong", "em", "underline", "strike", "code"}

func delimiters(m Mark) (string, string) {
	switch m.Type {
	case "strong":
		return "**", "**"
	case "em":
		return "*", "*"
	case "underline":
		return "<u>", "</u>"
	case "strike":
		return "~~", "~~"
	case "link":
		if isAuto(m) {
			return "", ""
		}
		href, _ := m.Attrs["href"].(string)
		return "[", "](" + href + ")"
	}
	return "", ""
}

// isAuto reports whether a link is a bare URL, written as the URL itself.
func isAuto(m Mark) bool {
	auto, _ := m.Attrs["auto"].(bool)
	return m.Type == "link" && auto
}

// serializeInline writes inline content, opening and closing marks around
// runs of nodes that share them.
func serializeInline(nodes []*Node, cell bool) (string, error) {
	var b strings.Builder
	var open []Mark
	has := func(marks []Mark, m Mark) bool {
		for _, x := range marks {
			if sameMark(x, m) {
				return true
			}
		}
		return false
	}
	closeTo := func(keep int) {
		for len(open) > keep {
			_, end := delimiters(open[len(open)-1])
			b.WriteString(end)
			open = open[:len(open)-1]
		}
	}
	for _, n := range nodes {
		marks := sortedMarks(n.Marks)
		var code, auto bool
		var outer []Mark
		for _, m := range marks {
			if m.Type == "code" {
				code = true
			} else {
				auto = auto || isAuto(m)
				outer = append(outer, m)
			}
		}
		// Keep the open marks this node shares, from the outside in.
		keep := 0
		for keep < len(open) && keep < len(outer) && sameMark(open[keep], outer[keep]) {
			keep++
		}
		closeTo(keep)
		for _, m := range outer[keep:] {
			if has(open, m) {
				continue
			}
			start, _ := delimiters(m)
			b.WriteString(start)
			open = append(open, m)
		}
		switch n.Type {
		case "text":
			if code {
				b.WriteString(codeSpan(n.Text))
			} else if auto {
				b.WriteString(n.Text)
			} else {
				b.WriteString(escapeText(n.Text, cell))
			}
		case "soft_break":
			if cell {
				b.WriteString(" ")
			} else {
				b.WriteString("\n")
			}
		case "hard_break":
			if cell {
				b.WriteString(" ")
			} else {
				b.WriteString("\\\n")
			}
		default:
			return "", fmt.Errorf("a %s cannot be written inline", n.Type)
		}
	}
	closeTo(0)
	return b.String(), nil
}

func sameMark(a, b Mark) bool {
	if a.Type != b.Type {
		return false
	}
	switch a.Type {
	case "link":
		return a.Attrs["href"] == b.Attrs["href"] && isAuto(a) == isAuto(b)
	}
	return true
}

func sortedMarks(marks []Mark) []Mark {
	var out []Mark
	for _, t := range markOrder {
		for _, m := range marks {
			if m.Type == t {
				out = append(out, m)
			}
		}
	}
	return out
}

func codeSpan(s string) string {
	fence := "`"
	for strings.Contains(s, fence) {
		fence += "`"
	}
	if strings.HasPrefix(s, "`") || strings.HasSuffix(s, "`") {
		return fence + " " + s + " " + fence
	}
	return fence + s + fence
}

// escapeText escapes the characters in text that Markdown would otherwise
// read as markup: backslashes before punctuation, asterisks, backticks,
// underscores at a word's edge, a bracket that would close a link, and in a
// table cell the pipe.
func escapeText(s string, cell bool) string {
	var b strings.Builder
	rs := []rune(s)
	for i, r := range rs {
		switch r {
		case '\\':
			if i+1 < len(rs) && rs[i+1] < 128 && isEscapable(byte(rs[i+1])) {
				b.WriteString(`\\`)
				continue
			}
		case '*', '`':
			b.WriteRune('\\')
		case ']':
			// A bracket is plain text unless "](" or "][" would close a link.
			if i+1 < len(rs) && (rs[i+1] == '(' || rs[i+1] == '[') {
				b.WriteRune('\\')
			}
		case '_':
			before := i > 0 && isWord(rs[i-1])
			after := i+1 < len(rs) && isWord(rs[i+1])
			if !(before && after) {
				b.WriteRune('\\')
			}
		case '~':
			if i+1 < len(rs) && rs[i+1] == '~' {
				b.WriteRune('\\')
			}
		case '<':
			if i+1 < len(rs) && (unicode.IsLetter(rs[i+1]) || rs[i+1] == '/' || rs[i+1] == '!') {
				b.WriteRune('\\')
			}
		case '|':
			if cell {
				b.WriteRune('\\')
			}
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isWord(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// escapeLineStart escapes a paragraph's first character when it would make
// the paragraph a heading, a list or a quote.
func escapeLineStart(s string) string {
	switch {
	case strings.HasPrefix(s, "#"), strings.HasPrefix(s, ">"),
		strings.HasPrefix(s, "- "), strings.HasPrefix(s, "+ "):
		return `\` + s
	}
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i > 0 && i < len(s)-1 && (s[i] == '.' || s[i] == ')') && s[i+1] == ' ' {
		return s[:i] + `\` + s[i:]
	}
	return s
}
