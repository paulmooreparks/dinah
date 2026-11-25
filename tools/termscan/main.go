// Command termscan checks unpublished commits against the private
// forbidden-terms list, which the tracked-files guard in internal/profile
// cannot do: that guard reads only the files in the tree, so a term in a
// commit message, or in a commit whose file was changed again before the
// push, would reach the public history unseen.
//
// It reads records from standard input, separated by NUL bytes, each one
// "<commit hash><TAB><text>", and matches every record's text against each
// pattern in the list (Go regular expressions, matched without regard to
// case; blank lines and lines starting with # are skipped). It prints only
// the short hash and the list's line number for a match, never the matched
// text, since the list itself is private. It exits 1 when anything matched
// and 2 on a usage or reading error.
//
// Run from the repository root before pushing master:
//
//	{ git log -z --format='%H%x09%B' origin/master..HEAD; printf 'patch\t'; git diff origin/master..HEAD; printf '\0'; } \
//	  | go run ./tools/termscan ~/.dinah/forbidden-terms.txt
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

type term struct {
	line int
	re   *regexp.Regexp
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: termscan <terms file> < records")
		os.Exit(2)
	}
	terms, err := readTerms(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "termscan:", err)
		os.Exit(2)
	}
	if len(terms) == 0 {
		fmt.Fprintln(os.Stderr, "termscan: the terms file has no patterns")
		os.Exit(2)
	}
	in, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "termscan:", err)
		os.Exit(2)
	}
	hits, records := scan(string(in), terms, os.Stdout)
	fmt.Println("records scanned:", records, "hits:", hits)
	if hits > 0 {
		os.Exit(1)
	}
}

func readTerms(path string) ([]term, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var terms []term
	s := bufio.NewScanner(f)
	for n := 1; s.Scan(); n++ {
		l := strings.TrimSpace(s.Text())
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		re, err := regexp.Compile("(?i)" + l)
		if err != nil {
			return nil, fmt.Errorf("line %d of the terms file is not a valid pattern", n)
		}
		terms = append(terms, term{n, re})
	}
	return terms, s.Err()
}

func scan(in string, terms []term, out io.Writer) (hits, records int) {
	for _, rec := range strings.Split(in, "\x00") {
		hash, body, ok := strings.Cut(strings.TrimSpace(rec), "\t")
		if !ok {
			continue
		}
		records++
		if len(hash) > 8 {
			hash = hash[:8]
		}
		for _, t := range terms {
			if t.re.MatchString(body) {
				fmt.Fprintln(out, hash, "list line", t.line)
				hits++
			}
		}
	}
	return hits, records
}
