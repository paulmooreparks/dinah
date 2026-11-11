// Package docfiles reads from a document's text the files it needs beside
// it: the pictures it shows and the files it links to. The draft service
// copies them with a document saved somewhere else, and a source that
// publishes a document, as a page with attachments, decides by the same
// reading which link is to a file it was given.
package docfiles

import (
	"encoding/json"
	"net/url"
	pathpkg "path"
	"regexp"
	"slices"
	"strings"
)

// The pictures a document shows are written in its text in three ways: a
// Markdown image, an HTML img element, and the {{figure}} shortcode the
// content applets write for Insert, Picture (frontend/YAVC.md). Shown reads
// all three, so that a copy of the document saved somewhere else takes its
// pictures with it wherever they are kept, and not only those in the
// folders a source lists beside a document. A file the document links to is
// written as a Markdown link, which Linked reads.

var (
	linkRe        = regexp.MustCompile(`(!?)\[[^\]]*\]\(\s*(?:<([^>\n]+)>|([^\s)]+))`)
	shownImageRe  = regexp.MustCompile(`!\[[^\]]*\]\(\s*(?:<([^>\n]+)>|([^\s)]+))`)
	shownImgTagRe = regexp.MustCompile(`(?i)<img\b[^>]*?\bsrc\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	shownFigureRe = regexp.MustCompile(`\{\{(?:figure|diagram|code):(.*?)\}\}`)
	shownCodeRe   = regexp.MustCompile("`+[^`\n]*`+")
	shownSchemeRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)
)

// A document also shows a saved diagram with {{diagram}} and a file as a
// code listing with {{code}}, which the applets draw from the file itself,
// so those files are a document's as its pictures are, and Shown lists them
// with the pictures.
//
// Shown lists the files a document's text shows as pictures, each as the
// path the text gives from the document's folder, in the order of their lines
// and once each. A picture at a web address or inside the text itself
// (a data address) is no file, and neither is one given by an absolute path,
// which is not a place beside the document. Text inside a code block or a
// code span is not read.
func Shown(text string) []string {
	var out []string
	add := func(dest string) {
		if name := localName(dest); name != "" && !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	prose(text, func(line string) {
		for _, m := range shownImageRe.FindAllStringSubmatch(line, -1) {
			add(m[1] + m[2])
		}
		for _, m := range shownImgTagRe.FindAllStringSubmatch(line, -1) {
			add(m[1] + m[2])
		}
		for _, m := range shownFigureRe.FindAllStringSubmatch(line, -1) {
			add(figureSource(m[1]))
		}
	})
	return out
}

// Linked lists the files a document's text links to, each as the path the
// text gives from the document's folder, in the order of their lines and
// once each (LinkedName says which links are to files).
func Linked(text string) []string {
	var out []string
	prose(text, func(line string) {
		for _, m := range linkRe.FindAllStringSubmatch(line, -1) {
			if m[1] != "" {
				continue
			}
			if name := LinkedName(m[2] + m[3]); name != "" && !slices.Contains(out, name) {
				out = append(out, name)
			}
		}
	})
	return out
}

// documentExtensions are the kinds of file a link to which is a link to
// another document, which is opened and not kept beside this one.
var documentExtensions = []string{".md", ".markdown"}

// LinkedName is the file a link's address points to, as a path from the
// document's folder, or "" when the link is not to a file kept beside the
// document: an address on the web or of another kind, a place in the
// document itself, an absolute path, another document, or a name without
// an extension, which is a folder or a page.
func LinkedName(dest string) string {
	name := localName(dest)
	ext := strings.ToLower(pathpkg.Ext(name))
	if name == "" || ext == "" || ext == "." || slices.Contains(documentExtensions, ext) {
		return ""
	}
	return name
}

// prose gives each line of the text that is not in a code block to read,
// with its code spans taken out.
func prose(text string, read func(line string)) {
	fence := ""
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimLeft(line, " ")
		if fence != "" {
			if strings.HasPrefix(trimmed, fence) && strings.Trim(trimmed, fence[:1]+" \t") == "" {
				fence = ""
			}
			continue
		}
		if len(line)-len(trimmed) < 4 {
			if mark := fenceMark(trimmed); mark != "" {
				fence = mark
				continue
			}
		}
		read(shownCodeRe.ReplaceAllString(line, ""))
	}
}

// figureSource is the picture a {{figure}} shows, from the shortcode's
// arguments: the one called src, or else the first that has no name. An
// argument is a word, or words in double quotes with a backslash before a
// quote inside them.
func figureSource(args string) string {
	first, i := "", 0
	found := false
	for i < len(args) {
		for i < len(args) && (args[i] == ' ' || args[i] == '\t') {
			i++
		}
		start := i
		for i < len(args) && (args[i] == '-' || args[i] == '_' || args[i] >= '0' && args[i] <= '9' || args[i] >= 'A' && args[i] <= 'Z' || args[i] >= 'a' && args[i] <= 'z') {
			i++
		}
		key := ""
		if i < len(args) && args[i] == '=' && i > start {
			key = args[start:i]
			i++
		} else {
			i = start
		}
		var value strings.Builder
		if i < len(args) && args[i] == '"' {
			for i++; i < len(args) && args[i] != '"'; i++ {
				if args[i] == '\\' && i+1 < len(args) {
					i++
				}
				value.WriteByte(args[i])
			}
			i++
		} else {
			for ; i < len(args) && args[i] != ' ' && args[i] != '\t'; i++ {
				value.WriteByte(args[i])
			}
		}
		switch {
		case key == "src":
			return value.String()
		case key == "" && !found:
			first, found = value.String(), true
		}
	}
	return first
}

// fenceMark is the run of backticks or tildes that opens a code block on a
// line, or "" when the line opens none.
func fenceMark(line string) string {
	for _, c := range []string{"`", "~"} {
		n := len(line) - len(strings.TrimLeft(line, c))
		if n >= 3 {
			return strings.Repeat(c, n)
		}
	}
	return ""
}

// localName is an address as a path from the document's folder, or "" when
// the address is not a file beside the document.
func localName(dest string) string {
	dest = strings.TrimSpace(dest)
	if i := strings.IndexAny(dest, "?#"); i >= 0 {
		dest = dest[:i]
	}
	if plain, err := url.PathUnescape(dest); err == nil {
		dest = plain
	}
	dest = strings.ReplaceAll(dest, `\`, "/")
	for strings.HasPrefix(dest, "./") {
		dest = dest[2:]
	}
	if dest == "" || strings.HasPrefix(dest, "/") || strings.HasPrefix(dest, "~") || shownSchemeRe.MatchString(dest) || strings.ContainsAny(dest, "\x00\r\n") {
		return ""
	}
	return dest
}

// CanvasFiles lists the files a canvas diagram shows as pictures, each as a
// path from the folder of the document that shows the canvas. name is the
// canvas file's own path from that folder, and data its content, which is
// JSON Canvas: a node of the type "file" gives the picture's path from the
// folder the canvas file is in. A canvas that cannot be read shows none. A
// path that leaves the document's folder is left out, as an address that is
// no file beside the document is.
func CanvasFiles(name string, data []byte) []string {
	var canvas struct {
		Nodes []struct {
			Type string `json:"type"`
			File string `json:"file"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(data, &canvas); err != nil {
		return nil
	}
	var out []string
	for _, n := range canvas.Nodes {
		file := localName(n.File)
		if n.Type != "file" || file == "" {
			continue
		}
		path := pathpkg.Join(pathpkg.Dir(name), file)
		if path == "." || strings.HasPrefix(path, "../") || path == ".." || slices.Contains(out, path) {
			continue
		}
		out = append(out, path)
	}
	return out
}
