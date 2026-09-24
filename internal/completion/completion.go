// Package completion holds what Dinah's shell completion shares between the
// verb that prints a script and the hidden callback the scripts run on every
// Tab: the four scripts themselves, the protocol they speak, the line splitter
// the bash script needs, the test a candidate has to pass before any shell sees
// it, and the writer that puts an answer on the wire.
//
// The package reads nothing from a workbench. What a candidate is comes from
// the head, which knows the commands and opens the workbench; this package
// only knows how the shells carry an answer.
package completion

import (
	"embed"
	"io"
	"strconv"
	"strings"
	"unicode"

	"dinah/internal/textwidth"
)

// Shells are the shells Dinah prints a completion script for, in the order
// every listing of them uses.
var Shells = []string{"powershell", "bash", "zsh", "fish"}

// Protocol is the version of the conversation between a script and the
// callback. A script sends it as the callback's first argument and accepts
// only an answer whose header carries it, so a script loaded from one build
// and a binary from another fail silently rather than badly.
const Protocol = 1

// protocolToken is the one placeholder an embedded script carries, replaced by
// Protocol when the script is printed.
const protocolToken = "__DINAH_PROTOCOL__"

// scripts are the four embedded scripts, one file per shell.
//
//go:embed scripts/dinah.bash scripts/dinah.zsh scripts/dinah.fish scripts/dinah.ps1
var scripts embed.FS

// scriptFiles names the embedded file each shell's script lives in.
var scriptFiles = map[string]string{
	"powershell": "scripts/dinah.ps1",
	"bash":       "scripts/dinah.bash",
	"zsh":        "scripts/dinah.zsh",
	"fish":       "scripts/dinah.fish",
}

// Script is the script for one shell with the protocol substituted, and false
// for a shell outside Shells. Nothing else is substituted, so the text carries
// no path, no workbench and no candidate, and it is the same wherever it is
// generated.
func Script(shell string) (string, bool) {
	file, ok := scriptFiles[shell]
	if !ok {
		return "", false
	}
	raw, err := scripts.ReadFile(file)
	if err != nil {
		return "", false
	}
	version := strconv.Itoa(Protocol)
	return strings.ReplaceAll(string(raw), protocolToken, version), true
}

// The four modes a header may carry, which tell a script what to do with the
// lines that follow it.
const (
	// ModeWords offers the candidates, and the shell appends its usual space.
	ModeWords = "words"
	// ModeNospace offers the candidates with no space appended, for a word
	// the person goes on typing, such as a query field ending in a colon.
	ModeNospace = "nospace"
	// ModeFiles hands the word to the shell's own file name completion.
	ModeFiles = "files"
	// ModeDirs hands the word to the shell's own directory name completion.
	ModeDirs = "dirs"
)

// Candidate is one completion: the whole word the person means, and what a
// shell able to show one displays beside it.
type Candidate struct {
	// Word is the whole word, starting with the whole current word, before
	// any part the shell keeps on the line is trimmed from it.
	Word string
	// Description is shown beside the word by the shells that show one. It
	// is sanitized on the way out, so a completer passes it as it stands.
	Description string
}

// descriptionColumns is how many display columns a description may take
// before it is cut.
const descriptionColumns = 60

// ellipsis ends a description that was cut.
const ellipsis = "…"

// Write puts one answer on the wire: the header naming the mode, then one line
// per candidate carrying the text the shell inserts, a TAB, and the sanitized
// description. The insert is the candidate with its first replaceFrom bytes
// removed, which are the ones the shell leaves on the line. describe false
// writes every description empty, which is what the bash callback does.
//
// The offset is counted in bytes rather than in runes. A caller keeps only a
// candidate that starts with the current word under ASCII case folding, which
// compares bytes, so the candidate and the current word share a prefix of the
// same byte length and a byte offset cuts both at the same character. It is
// never written to the wire, so no script sees the unit.
//
// Write filters nothing. A caller has already kept what matches and passes
// SafeWord, and a candidate whose word is shorter than replaceFrom is a defect
// of that caller, written as an empty insert that every script ignores.
func Write(w io.Writer, mode string, candidates []Candidate, replaceFrom int, describe bool) error {
	var b strings.Builder
	b.WriteString("dinah-complete ")
	b.WriteString(strconv.Itoa(Protocol))
	b.WriteString(" ")
	b.WriteString(mode)
	b.WriteString("\n")
	for _, candidate := range candidates {
		b.WriteString(trimFront(candidate.Word, replaceFrom))
		b.WriteString("\t")
		if describe {
			b.WriteString(Sanitize(candidate.Description))
		}
		b.WriteString("\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// trimFront drops the first n bytes of a word, answering the empty string
// where the word has fewer.
func trimFront(word string, n int) string {
	if n >= len(word) {
		return ""
	}
	if n <= 0 {
		return word
	}
	return word[n:]
}

// Sanitize is what a description becomes before a shell sees it. Every
// control character, TAB, CR and LF among them, becomes one space, a run of
// whitespace collapses to one space, the result is trimmed, and it is cut to
// sixty display columns ending in an ellipsis when it was longer. Quotation
// marks, backslashes and every other printable character pass unchanged.
func Sanitize(description string) string {
	spaced := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, description)
	collapsed := strings.Join(strings.Fields(spaced), " ")
	if textwidth.Columns(collapsed) <= descriptionColumns {
		return collapsed
	}
	room := descriptionColumns - textwidth.Columns(ellipsis)
	var kept strings.Builder
	for _, r := range collapsed {
		next := kept.String() + string(r)
		if textwidth.Columns(next) > room {
			break
		}
		kept.WriteRune(r)
	}
	return kept.String() + ellipsis
}

// safePunctuation are the punctuation characters a candidate may carry. Each
// is a bare character in all four shells wherever a candidate can stand, so no
// script has to quote or escape an insertion.
const safePunctuation = "-_./:=+,"

// SafeWord reports whether a candidate may be offered: every rune a Unicode
// letter, a Unicode digit or one of - _ . / : = + , and the word not empty. A
// candidate failing it is dropped rather than quoted, because the four shells
// quote four different ways and PowerShell reads a comma and an at sign as
// operators. Whether a word may begin with a dash is the caller's question,
// since only the flag list may offer one.
func SafeWord(word string) bool {
	if word == "" {
		return false
	}
	for _, r := range word {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		if strings.ContainsRune(safePunctuation, r) {
			continue
		}
		return false
	}
	return true
}

// commandSeparators end one command on a bash line and begin the next, when
// they stand unquoted.
const commandSeparators = ";&|(\n"

// SplitBash reads a bash command line the way readline hands it to a
// completion function, and answers the words after the program, the word being
// completed, and replaceFrom, the byte offset in that word at which readline's
// own replacement begins. programFound is false when the line holds no program
// word before the word being completed, so there is nothing to complete an
// argument of.
//
// It applies these rules and no others. The line is cut at the last unquoted
// command separator. Words are separated by unquoted spaces, tabs and
// newlines. Single quotes make everything literal; inside double quotes a
// backslash escapes only $, `, ", \ and a newline; outside quotes a backslash
// escapes the next character and a backslash-newline pair is removed. Nothing
// is expanded. Leading NAME=value words are dropped, and so is the program
// word after them. A line ending in unquoted whitespace completes an empty
// word, and a line ending inside an open quote completes the unquoted text so
// far. replaceFrom is one past the last character of the current word that is
// unquoted, not escaped and a member of wordbreaks, and 0 when there is none.
func SplitBash(line, wordbreaks string) (words []string, current string, replaceFrom int, programFound bool) {
	line = afterLastSeparator(line)
	all, last := splitWords(line, wordbreaks)
	all = dropAssignmentsAndProgram(all, &programFound)
	if !programFound {
		return nil, "", 0, false
	}
	return all, last.text, last.breakAfter, true
}

// bashWord is one word of a bash line with its quoting removed, and, for the
// word being completed, where readline's replacement of it begins.
type bashWord struct {
	// text is the word with one level of quoting removed.
	text string
	// breakAfter is the byte offset just past the last unquoted, unescaped
	// word-break character in text, and 0 when there is none.
	breakAfter int
}

// afterLastSeparator is the part of a line after its last unquoted command
// separator, which is all of it when there is none.
func afterLastSeparator(line string) string {
	cut := 0
	quote := rune(0)
	escaped := false
	for i, r := range line {
		if escaped {
			escaped = false
			continue
		}
		switch {
		case quote == '\'':
			if r == '\'' {
				quote = 0
			}
		case quote == '"':
			if r == '\\' {
				escaped = true
			} else if r == '"' {
				quote = 0
			}
		case r == '\\':
			escaped = true
		case r == '\'' || r == '"':
			quote = r
		case strings.ContainsRune(commandSeparators, r):
			cut = i + len(string(r))
		}
	}
	return line[cut:]
}

// splitWords splits the part of a line after its last separator into words,
// every word but the last as the prior words and the last as the current one,
// which is empty when the line ends in unquoted whitespace.
func splitWords(line, wordbreaks string) (prior []string, current bashWord) {
	var text []rune
	breakAfter := 0
	// breakAfter is kept in bytes, so it is the length of the text so far
	// as it would be written out, measured when a break character lands.
	started := false
	quote := rune(0)
	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case quote == '\'':
			if r == '\'' {
				quote = 0
				continue
			}
			text = append(text, r)
		case quote == '"':
			if r == '"' {
				quote = 0
				continue
			}
			if r == '\\' && i+1 < len(runes) && strings.ContainsRune("$`\"\\\n", runes[i+1]) {
				i++
				if runes[i] != '\n' {
					text = append(text, runes[i])
				}
				continue
			}
			text = append(text, r)
		case r == '\\':
			if i+1 >= len(runes) {
				started = true
				continue
			}
			i++
			started = true
			if runes[i] != '\n' {
				text = append(text, runes[i])
			}
		case r == ' ' || r == '\t' || r == '\n':
			if started {
				prior = append(prior, string(text))
			}
			text = nil
			breakAfter = 0
			started = false
		case r == '\'' || r == '"':
			quote = r
			started = true
		default:
			started = true
			text = append(text, r)
			if strings.ContainsRune(wordbreaks, r) {
				breakAfter = len(string(text))
			}
		}
	}
	return prior, bashWord{text: string(text), breakAfter: breakAfter}
}

// dropAssignmentsAndProgram removes the leading NAME=value words and the
// program word after them, reporting through found whether a program word was
// there at all.
func dropAssignmentsAndProgram(words []string, found *bool) []string {
	for i, word := range words {
		if isAssignment(word) {
			continue
		}
		*found = true
		return words[i+1:]
	}
	*found = false
	return nil
}

// isAssignment reports whether a word has the shape NAME=value, where NAME
// starts with a letter or an underscore and continues with letters, digits and
// underscores.
func isAssignment(word string) bool {
	name, _, found := strings.Cut(word, "=")
	if !found || name == "" {
		return false
	}
	for i, r := range name {
		letter := r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
		digit := r >= '0' && r <= '9'
		if !letter && !(digit && i > 0) {
			return false
		}
	}
	return true
}
