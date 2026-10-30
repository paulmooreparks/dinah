package main

import (
	"strconv"
	"strings"
)

// ClipboardContent is what the clipboard holds, as the Editor's Paste
// command takes it: the HTML a program put there, such as Word, a browser
// or the Editor's own Copy, when there is any, and the plain text.
type ClipboardContent struct {
	HTML string `json:"html"`
	Text string `json:"text"`
}

// ClipboardRead reads the clipboard for the Editor's Paste command. The page
// cannot read the clipboard itself without the web view asking the person
// for permission, and Wails reads only plain text, so Dinah reads it:
// on Windows both the HTML and the text, elsewhere the text through Wails.
// Keyboard paste (Ctrl+V) does not come here; the web view gives the page
// the clipboard with the key.
func (a *App) ClipboardRead() (ClipboardContent, error) {
	return a.readClipboard()
}

// htmlFragment is the HTML of a clipboard entry in Windows' HTML clipboard
// format, which starts with a header of byte offsets ("HTML Clipboard
// Format", Microsoft's Win32 documentation): the fragment between
// StartFragment and EndFragment, or, when the header gives none, the
// document between StartHTML and EndHTML. It returns "" when the header is
// not one.
func htmlFragment(data []byte) string {
	header := map[string]int{}
	for _, line := range strings.Split(string(data[:min(len(data), 512)]), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			header[key] = n
		}
	}
	span := func(from, to string) (string, bool) {
		start, okStart := header[from]
		end, okEnd := header[to]
		if !okStart || !okEnd || start < 0 || end > len(data) || start >= end {
			return "", false
		}
		return string(data[start:end]), true
	}
	if s, ok := span("StartFragment", "EndFragment"); ok {
		return s
	}
	if s, ok := span("StartHTML", "EndHTML"); ok {
		return s
	}
	return ""
}
