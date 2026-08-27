// Command renamesweep reads the diff of a word-for-word rename and reports
// where the new word landed in a sentence that meant the old word's other
// sense. It is a maintenance tool for this repository rather than part of
// Dinah, which is why it lives under internal and ships in no release.
//
// Run it against the range that carried the rename, before the rename is
// handed off:
//
//	go run ./internal/rename/renamesweep --old state --new column --range main..HEAD
//
// A rename that has already landed is swept the same way, because a squash
// commit's own diff is the whole branch:
//
//	go run ./internal/rename/renamesweep --old state --new column --range e9c0abd^..e9c0abd
//
// The report is one line per phrase, rarest first, where a phrase is the word
// before the replacement, the replacement, and the word after it. The reader
// judges each line by asking whether the phrase it names means the thing the
// rename was about, and opens the line rather than judging it whenever the
// phrase could mean either. Pass --all to see every site under each line.
// Paths after the flags become a git pathspec, which narrows the sweep to part
// of the tree.
//
// A word this tool cannot read as a single token is refused rather than swept,
// because a term the tokenizer splits matches nothing and the run would report
// a zero indistinguishable from a clean tree. A zero the diff contradicts is
// refused for the same reason after the sweep, which is what a script written
// without spaces between its words produces.
//
// See docs/design/renaming-a-word.md for when this is run and what the reader
// is looking for.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"dinah/internal/rename"
)

func main() {
	retired := flag.String("old", "", "the retired word, in the singular")
	adopted := flag.String("new", "", "the word that replaced it, in the singular")
	revisions := flag.String("range", "", "the git revision range that carried the rename, such as main..HEAD")
	repo := flag.String("repo", ".", "the repository to read the range out of")
	all := flag.Bool("all", false, "list every site under each phrase rather than one example")
	flag.Parse()
	if *retired == "" || *adopted == "" || *revisions == "" {
		fmt.Fprintln(os.Stderr, "renamesweep: --old, --new and --range are all required")
		flag.Usage()
		os.Exit(2)
	}
	diff, err := readDiff(*repo, *revisions, flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "renamesweep: %v\n", err)
		os.Exit(1)
	}
	result, err := rename.Sweep(diff, *retired, *adopted)
	if err != nil {
		fmt.Fprintf(os.Stderr, "renamesweep: %v\n", err)
		os.Exit(1)
	}
	if err := rename.Report(os.Stdout, result, *retired, *adopted, *all); err != nil {
		fmt.Fprintf(os.Stderr, "renamesweep: %v\n", err)
		os.Exit(1)
	}
}

// readDiff asks git for the range's diff. Context lines are kept, because the
// sweep reads the word in front of each replacement and prose is hard-wrapped:
// when a rename fits inside one line, the word in front of it sits on the line
// above, which is unchanged and reaches the sweep only as context. Rename
// detection stays on so that a file moved during the rename is read as the one
// file it is.
//
// Three settings are pinned rather than inherited, because each one changes
// the text this tool parses. Colour and an external diff driver would replace
// the format entirely. diff.suppressBlankEmpty is subtler and was found by a
// reviewer whose configuration set it: an unchanged blank line then arrives as
// an empty string rather than as a single space, misses the context branch of
// the parser, advances no line counter, and every line number after it in the
// file is reported one low.
func readDiff(repo, revisions string, pathspec []string) (string, error) {
	args := []string{
		"-C", repo,
		"-c", "diff.suppressBlankEmpty=false",
		"diff",
		"--unified=3",
		"--find-renames",
		"--no-color",
		"--no-ext-diff",
		revisions,
	}
	if len(pathspec) > 0 {
		args = append(args, "--")
		args = append(args, pathspec...)
	}
	cmd := exec.Command("git", args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}
