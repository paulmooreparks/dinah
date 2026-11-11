package drafts

import (
	"net/url"
	"regexp"
	"slices"
	"strings"
)

// The pictures a document shows are written in its text in three ways: a
// Markdown image, an HTML img element, and the {{figure}} shortcode the
// content applets write for Insert, Picture (frontend/YAVC.md). ShownFiles
// reads all three, so that a copy of the document saved somewhere else takes
// its pictures with it wherever they are kept, and not only those in the
// folders a source lists beside a document.

var (
	shownImageRe  = regexp.MustCompile(`!\[[^\]]*\]\(\s*(?:<([^>\n]+)>|([^\s)]+))`)
	shownImgTagRe = regexp.MustCompile(`(?i)<img\b[^>]*?\bsrc\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	shownFigureRe = regexp.MustCompile(`\{\{figure:(.*?)\}\}`)
	shownCodeRe   = regexp.MustCompile("`+[^`\n]*`+")
	shownSchemeRe = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)
)

// ShownFiles lists the files a document's text shows as pictures, each as the
// path the text gives from the document's folder, in the order of their lines
// and once each. A picture at a web address or inside the text itself
// (a data address) is no file, and neither is one given by an absolute path,
// which is not a place beside the document. Text inside a code block or a
// code span is not read.
func ShownFiles(text string) []string {
	var out []string
	add := func(dest string) {
		if name := shownName(dest); name != "" && !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
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
		line = shownCodeRe.ReplaceAllString(line, "")
		for _, m := range shownImageRe.FindAllStringSubmatch(line, -1) {
			add(m[1] + m[2])
		}
		for _, m := range shownImgTagRe.FindAllStringSubmatch(line, -1) {
			add(m[1] + m[2])
		}
		for _, m := range shownFigureRe.FindAllStringSubmatch(line, -1) {
			add(figureSource(m[1]))
		}
	}
	return out
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

// shownName is a picture's address as a path from the document's folder, or
// "" when the address is not a file beside the document.
func shownName(dest string) string {
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
