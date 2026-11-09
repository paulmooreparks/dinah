package threads

import "strings"

// contextLen is how much text on each side of a thread's words is kept to
// find them again, as a suggestion keeps (package drafts).
const contextLen = 48

// span takes a thread's words and the text around them from text.
func span(text string, start, end int) (quote, prefix, suffix string) {
	return text[start:end], clamp(text[:start], contextLen, true), clamp(text[end:], contextLen, false)
}

// clamp cuts s to at most n bytes without splitting a UTF-8 sequence,
// keeping its start, or its end when keepEnd is set.
func clamp(s string, n int, keepEnd bool) string {
	if len(s) <= n {
		return s
	}
	if keepEnd {
		i := len(s) - n
		for i < len(s) && s[i]&0xC0 == 0x80 {
			i++
		}
		return s[i:]
	}
	i := n
	for i > 0 && s[i]&0xC0 == 0x80 {
		i--
	}
	return s[:i]
}

// Placed is a thread as it stands in a document's current text: where its
// words are, or Lost when they are no longer there, in which case Start is
// where they were last seen, clamped to the text.
type Placed struct {
	Thread
	At   int  `json:"at"`
	End  int  `json:"end"`
	Lost bool `json:"lost,omitempty"`
}

// Place finds each thread's words in text: with the words around them,
// nearest where they were, then the words alone. A thread whose words are
// gone is Lost and shown where they were last seen, so it is never dropped
// unannounced; when the words come back, it is found again.
func Place(list []Thread, text string) []Placed {
	out := make([]Placed, 0, len(list))
	for _, t := range list {
		p := Placed{Thread: t}
		if at, ok := find(t, text); ok {
			p.At, p.End = at, at+len(t.Quote)
		} else {
			p.At, p.End, p.Lost = min(max(t.Start, 0), len(text)), min(max(t.Start, 0), len(text)), true
		}
		out = append(out, p)
	}
	return out
}

func find(t Thread, text string) (int, bool) {
	best, found := -1, false
	pick := func(needle string, offset int) {
		if needle == "" {
			return
		}
		for from := 0; from <= len(text); {
			i := strings.Index(text[from:], needle)
			if i < 0 {
				return
			}
			at := from + i + offset
			if !found || abs(at-t.Start) < abs(best-t.Start) {
				best, found = at, true
			}
			from += i + 1
		}
	}
	pick(t.Prefix+t.Quote+t.Suffix, len(t.Prefix))
	if !found && t.Quote != "" {
		pick(t.Quote, 0)
	}
	return best, found
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
