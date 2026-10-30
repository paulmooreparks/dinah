package drafts

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"studio/internal/textdiff"
)

// Suggestion is a change proposed to a draft's text, by an agent or a flow,
// that the person accepts or rejects; it is never applied without them. It
// is kept beside the text, in the draft's record, so it is never published
// and never mistaken for the document's own content. The text it replaces,
// Before, sits at [Start, End) of the draft's text (byte offsets), and the
// context around it lets the suggestion be found again after the person
// types elsewhere. A suggestion whose text was changed under it is Stale: it
// is shown, so it is not lost unannounced, but it can only be rejected.
type Suggestion struct {
	ID     string `json:"id"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
	Before string `json:"before"`
	After  string `json:"after"`
	Prefix string `json:"prefix"`
	Suffix string `json:"suffix"`
	Stale  bool   `json:"stale,omitempty"`
	// Group is the proposal the suggestion came from, so that its changes
	// can be accepted or rejected together.
	Group   string   `json:"group"`
	Author  string   `json:"author"`
	Reason  string   `json:"reason,omitempty"`
	Sources []string `json:"sources,omitempty"`
	// Flags are the sentences the proposal's editor flagged as possibly
	// wrong or contradictory, for the person to look at before accepting.
	Flags   []Flag    `json:"flags,omitempty"`
	Created time.Time `json:"created"`
	// Edited says the person changed After before deciding (Revise), so
	// that the suggestion no longer reads as its author's alone.
	Edited bool `json:"edited,omitempty"`
}

// Proposal describes where a set of suggestions came from.
type Proposal struct {
	// Author is who proposed the changes, such as a flow run and its step.
	Author  string
	Reason  string
	Sources []string
	Flags   []Flag
}

// Flag is a sentence an editor flagged as possibly wrong or contradictory,
// as Read the Editor's Flags gives it: the sentence's mark and why.
type Flag struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

// contextLen is how much text on each side of a suggestion is kept to find it
// again.
const contextLen = 48

// ErrSuggestionGone means the suggestion no longer exists.
var ErrSuggestionGone = errors.New("the suggestion is no longer there")

// ErrSuggestionStale means the text a suggestion would replace has changed,
// so it can only be rejected.
var ErrSuggestionStale = errors.New("the text the suggestion would change has been changed since")

func suggestionID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// clamp cuts s to at most n bytes without splitting a UTF-8 sequence,
// keeping its start (keepEnd false) or its end (keepEnd true).
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

// anchor finds where a suggestion's text is in text: the occurrence of its
// context and text nearest to near, or of its text alone when the context
// was changed. It reports false when the text is not there, or when only the
// context is left of a suggestion that removes nothing.
func anchor(s Suggestion, text string, near int) (int, bool) {
	best, found := -1, false
	pick := func(needle string, offset int) {
		for from := 0; from <= len(text); {
			i := strings.Index(text[from:], needle)
			if i < 0 {
				return
			}
			at := from + i + offset
			if !found || abs(at-near) < abs(best-near) {
				best, found = at, true
			}
			from += i + 1
		}
	}
	pick(s.Prefix+s.Before+s.Suffix, len(s.Prefix))
	if !found && s.Before != "" {
		pick(s.Before, 0)
	}
	// An insertion has no text of its own to look for, so it falls back on
	// the nearest lines around its place: the line before it and the line
	// after, then either alone.
	if !found && s.Before == "" {
		before, after := lastLine(s.Prefix), firstLine(s.Suffix)
		if before != "" && after != "" {
			pick(before+after, len(before))
		}
		if !found && before != "" {
			pick(before, len(before))
		}
		if !found && after != "" {
			pick(after, 0)
		}
	}
	return best, found
}

// lastLine is the last line of s with its line ending, or the end of s when
// it has no complete line.
func lastLine(s string) string {
	i := strings.LastIndex(strings.TrimSuffix(s, "\n"), "\n")
	return s[i+1:]
}

// firstLine is the first line of s with its line ending.
func firstLine(s string) string {
	if i := strings.Index(s, "\n"); i >= 0 {
		return s[:i+1]
	}
	return s
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// rebase finds every suggestion again in text, the draft's text after a
// change. One whose text is gone becomes stale; a stale one stays stale.
func rebase(list []Suggestion, text string) []Suggestion {
	out := list[:0]
	for _, s := range list {
		if !s.Stale {
			if at, ok := anchor(s, text, s.Start); ok {
				s.Start, s.End = at, at+len(s.Before)
			} else {
				s.Stale = true
			}
		}
		out = append(out, s)
	}
	return out
}

// propose turns a proposed text into suggestions against base, the text the
// proposer read, paragraph by paragraph (Markdown keeps a paragraph on one
// line), and places each in the draft's current text, which may have moved
// on from base. A change whose place cannot be found is kept stale.
func propose(base, proposed, current string, p Proposal, now time.Time) []Suggestion {
	a, b := textdiff.Lines(base), textdiff.Lines(proposed)
	ta, tb := textdiff.Tokens(a, b)
	group := suggestionID()
	offsets := make([]int, len(a)+1)
	for i, line := range a {
		offsets[i+1] = offsets[i] + len(line)
	}
	var out []Suggestion
	for _, op := range textdiff.Opcodes(ta, tb) {
		if op.Tag == 'e' {
			continue
		}
		start, end := offsets[op.I1], offsets[op.I2]
		s := Suggestion{
			ID: suggestionID(), Group: group, Author: p.Author, Reason: p.Reason, Sources: p.Sources, Flags: p.Flags, Created: now,
			Before: base[start:end], After: strings.Join(b[op.J1:op.J2], ""),
			Prefix: clamp(base[:start], contextLen, true), Suffix: clamp(base[end:], contextLen, false),
		}
		if at, ok := anchor(s, current, start); ok {
			s.Start, s.End = at, at+len(s.Before)
		} else {
			s.Stale = true
		}
		out = append(out, s)
	}
	return out
}

// overlaps reports whether two placed suggestions touch the same text, so
// that accepting one makes the other stale.
func overlaps(a, b Suggestion) bool {
	if a.Stale || b.Stale {
		return false
	}
	if a.Start == a.End || b.Start == b.End {
		return a.Start >= b.Start && a.Start <= b.End || b.Start >= a.Start && b.Start <= a.End
	}
	return a.Start < b.End && b.Start < a.End
}

// Propose adds suggestions to a draft: the changes from base, the text the
// proposer read, to proposed, placed in the draft as it is now. It returns
// the draft and the suggestions it added; the draft's text is not changed.
func (s *Store) Propose(id, base, proposed string, p Proposal) (Draft, []Suggestion, error) {
	if err := checkText(proposed); err != nil {
		return Draft{}, nil, err
	}
	// The line breaks a text ends with separate it from what follows, and a
	// proposer that trims them changes nothing, so a proposal ends as its
	// base does.
	proposed = strings.TrimRight(proposed, "\r\n") + base[len(strings.TrimRight(base, "\r\n")):]
	var added []Suggestion
	d, err := s.changeSuggestions(id, func(d *Draft) error {
		// A proposer may write its texts with other line endings than the
		// draft's, as a Windows shell does; the draft's are used, or no
		// change would be found in it.
		b, pr := sameLineEndings(base, d.Text), sameLineEndings(proposed, d.Text)
		for _, n := range propose(b, pr, d.Text, p, s.now().UTC()) {
			d.Suggestions = withoutRepeats(d.Suggestions, n)
			if !suggested(d.Suggestions, n) {
				added = append(added, n)
			}
		}
		d.Suggestions = append(d.Suggestions, added...)
		return nil
	})
	return d, added, err
}

// A proposal made again, as an agent does when it is unsure whether its first
// one arrived, adds nothing twice: a change already suggested in the same
// place is not added again, and a placed change takes over from the same
// change kept earlier because it could not be placed.

// suggested reports whether list has n's change, placed where n is.
func suggested(list []Suggestion, n Suggestion) bool {
	if n.Stale {
		return false
	}
	for _, o := range list {
		if !o.Stale && o.Start == n.Start && o.Before == n.Before && o.After == n.After {
			return true
		}
	}
	return false
}

// withoutRepeats is list without the stale suggestions of n's change, when
// n is placed, including those kept with other line endings.
func withoutRepeats(list []Suggestion, n Suggestion) []Suggestion {
	if n.Stale {
		return list
	}
	out := make([]Suggestion, 0, len(list))
	for _, o := range list {
		if o.Stale && sameLineEndings(o.Before, n.Before) == n.Before && sameLineEndings(o.After, n.After) == n.After {
			continue
		}
		out = append(out, o)
	}
	return out
}

// sameLineEndings writes s with the line endings like uses: CRLF when like
// has one, LF otherwise.
func sameLineEndings(s, like string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if strings.Contains(like, "\r\n") {
		s = strings.ReplaceAll(s, "\n", "\r\n")
	}
	return s
}

// Accept applies a suggestion to the draft, provided the draft is still at
// expected and the suggestion's text is still in place; the suggestion goes,
// and others it overlapped become stale.
func (s *Store) Accept(id, expected, suggestion string) (Draft, error) {
	return s.change(id, expected, func(d *Draft) error {
		i := d.suggestion(suggestion)
		if i < 0 {
			return ErrSuggestionGone
		}
		sg := d.Suggestions[i]
		if sg.Stale || sg.End > len(d.Text) || d.Text[sg.Start:sg.End] != sg.Before {
			return ErrSuggestionStale
		}
		d.Suggestions = append(d.Suggestions[:i:i], d.Suggestions[i+1:]...)
		for j := range d.Suggestions {
			if overlaps(d.Suggestions[j], sg) {
				d.Suggestions[j].Stale = true
			}
		}
		d.Text = d.Text[:sg.Start] + sg.After + d.Text[sg.End:]
		return checkText(d.Text)
	})
}

// AcceptGroup applies every suggestion of a proposal that is still in place,
// in one change of the draft, provided the draft is still at expected. Stale
// ones are left for the person to see and reject.
func (s *Store) AcceptGroup(id, expected, group string) (Draft, error) {
	return s.change(id, expected, func(d *Draft) error {
		var take []Suggestion
		keep := d.Suggestions[:0:0]
		for _, sg := range d.Suggestions {
			if sg.Group == group && !sg.Stale && sg.End <= len(d.Text) && d.Text[sg.Start:sg.End] == sg.Before {
				take = append(take, sg)
			} else {
				keep = append(keep, sg)
			}
		}
		if len(take) == 0 {
			return ErrSuggestionGone
		}
		// From the end backwards, so each change leaves the places of those
		// before it as they were; two that overlap cannot both be taken.
		for i := 1; i < len(take); i++ {
			for j := i; j > 0 && take[j].Start > take[j-1].Start; j-- {
				take[j], take[j-1] = take[j-1], take[j]
			}
		}
		text, limit := d.Text, len(d.Text)+1
		for _, sg := range take {
			if sg.End > limit || sg.Start == sg.End && sg.End == limit {
				sg.Stale = true
				keep = append(keep, sg)
				continue
			}
			text = text[:sg.Start] + sg.After + text[sg.End:]
			limit = sg.Start
		}
		d.Text, d.Suggestions = text, keep
		return checkText(d.Text)
	})
}

// RejectGroup removes every suggestion of a proposal.
func (s *Store) RejectGroup(id, group string) (Draft, error) {
	return s.changeSuggestions(id, func(d *Draft) error {
		keep := d.Suggestions[:0:0]
		for _, sg := range d.Suggestions {
			if sg.Group != group {
				keep = append(keep, sg)
			}
		}
		if len(keep) == len(d.Suggestions) {
			return ErrSuggestionGone
		}
		d.Suggestions = keep
		return nil
	})
}

// Reject removes a suggestion without changing the text.
func (s *Store) Reject(id, suggestion string) (Draft, error) {
	return s.changeSuggestions(id, func(d *Draft) error {
		i := d.suggestion(suggestion)
		if i < 0 {
			return ErrSuggestionGone
		}
		d.Suggestions = append(d.Suggestions[:i:i], d.Suggestions[i+1:]...)
		return nil
	})
}

// Revise changes the text a suggestion proposes, as the person edits it before
// deciding; the text it replaces, its place and its proposal stay. The text
// of the draft is not changed. A stale suggestion can only be rejected, so it
// cannot be revised either.
func (s *Store) Revise(id, suggestion, after string) (Draft, error) {
	return s.changeSuggestions(id, func(d *Draft) error {
		i := d.suggestion(suggestion)
		if i < 0 {
			return ErrSuggestionGone
		}
		if d.Suggestions[i].Stale {
			return ErrSuggestionStale
		}
		if d.Suggestions[i].After != after {
			d.Suggestions[i].After = after
			d.Suggestions[i].Edited = true
		}
		return nil
	})
}

// Spans gives each placed suggestion's place in the draft's text in UTF-16
// code units, as a JavaScript editor counts them, keyed by the suggestion's
// ID, as [from, to].
func (d Draft) Spans() map[string][2]int {
	out := map[string][2]int{}
	for _, s := range d.Suggestions {
		if !s.Stale && s.End <= len(d.Text) {
			out[s.ID] = [2]int{utf16Len(d.Text[:s.Start]), utf16Len(d.Text[:s.End])}
		}
	}
	return out
}

func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

func (d *Draft) suggestion(id string) int {
	for i, s := range d.Suggestions {
		if s.ID == id {
			return i
		}
	}
	return -1
}
