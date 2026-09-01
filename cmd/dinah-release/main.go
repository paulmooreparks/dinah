// Command dinah-release answers the questions the release and promotion
// workflows ask git and the tag list.
//
// This is build tooling and not part of the product. Nothing ships it, nothing
// installs it, and release.yml still builds only ./cmd/dinah. It exists so the
// rules that decide a tag number and a cut's contents can be tested by go test
// instead of living as untestable shell inside a workflow file.
//
// The standard flag package parses the arguments here, rather than the
// hand-written parser cmd/dinah uses. That parser exists to give the product a
// help surface and a message catalogue in three languages, and neither of
// those is worth anything to a workflow step.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"dinah/internal/release"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "next-tag":
		err = nextTag(os.Args[2:])
	case "cut":
		err = cut(os.Args[2:])
	case "patch":
		err = patch(os.Args[2:])
	case "help", "-h", "--help":
		usage()
		return
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "::error::%s\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `dinah-release is the release workflows' helper.

  next-tag --channel dev|beta|stable --base 0.1 [--tags-from -]
      Reads tag names, one per line, from standard input and prints the tag the
      next release on that channel and line takes.

  patch --channel dev|beta|stable --base 0.1 --tag v0.1.2-beta
      Prints the number the tag carries on that channel and line, and fails
      when the tag belongs to another channel or another line.

  cut --base 0.1 --cards dinah-1,dinah-2 --links dinah-1>none,dinah-2>dinah-1 [--ref main] [--repo .]
      Resolves the named cards against the tagged commits of the current minor,
      refuses a cut that names a card --links says nothing about, refuses a cut
      that holds back a predecessor, and cherry-picks what is left
      onto the minor's beta base. Writes tag, base and head to GITHUB_OUTPUT
      when that variable is set, and to standard output otherwise.
`)
}

func nextTag(args []string) error {
	fs := flag.NewFlagSet("next-tag", flag.ExitOnError)
	channel := fs.String("channel", "", "dev, beta or stable")
	base := fs.String("base", "", "the major.minor line, as the VERSION file spells it")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, err := release.ParseChannel(*channel)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*base) == "" {
		return fmt.Errorf("--base is required and names the major.minor line")
	}
	var tags []string
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		tags = append(tags, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading the tag list: %w", err)
	}
	highest, found := release.HighestPatch(c, *base, tags)
	if found {
		fmt.Fprintf(os.Stderr, "the highest %s number on v%s is %d, so this one is %d\n", c, *base, highest, release.NextPatch(c, *base, tags))
	} else {
		fmt.Fprintf(os.Stderr, "v%s has no %s release yet, so this one opens the line\n", *base, c)
	}
	fmt.Println(release.NextTag(c, *base, tags))
	return nil
}

// patch reads the number a tag carries on a channel and a line, and refuses a
// tag that belongs to neither. The stable promotion calls it to establish that
// the tag it was handed really is a beta of the line it is about to publish,
// which nothing else in the workflow asks.
func patch(args []string) error {
	fs := flag.NewFlagSet("patch", flag.ExitOnError)
	channel := fs.String("channel", "", "dev, beta or stable")
	base := fs.String("base", "", "the major.minor line, as the VERSION file spells it")
	tag := fs.String("tag", "", "the tag to read")
	if err := fs.Parse(args); err != nil {
		return err
	}
	c, err := release.ParseChannel(*channel)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*base) == "" {
		return fmt.Errorf("--base is required and names the major.minor line")
	}
	if strings.TrimSpace(*tag) == "" {
		return fmt.Errorf("--tag is required and names the tag to read")
	}
	n, ok := release.Patch(c, *base, strings.TrimSpace(*tag))
	if !ok {
		return fmt.Errorf("%s is not a %s tag on the v%s line, so it cannot be read as one; a %s tag on that line is shaped %s", *tag, c, *base, c, release.Tag(c, *base, 0))
	}
	fmt.Println(n)
	return nil
}

func cut(args []string) error {
	fs := flag.NewFlagSet("cut", flag.ExitOnError)
	base := fs.String("base", "", "the major.minor line, as the VERSION file spells it")
	cards := fs.String("cards", "", "comma-separated card human-ids to promote")
	links := fs.String("links", "", "one dependent>predecessor or dependent>none declaration per card in the cut")
	ref := fs.String("ref", "main", "the trunk ref the candidates come from")
	repo := fs.String("repo", ".", "the repository to assemble the cut in")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*base) == "" {
		return fmt.Errorf("--base is required and names the major.minor line")
	}
	g := release.Git{Dir: *repo}

	parsedLinks, err := release.ParseLinks(*links)
	if err != nil {
		return err
	}

	tagList, err := g.Run("tag", "--list")
	if err != nil {
		return err
	}
	tags := strings.Split(tagList, "\n")

	start, err := release.MinorStart(g, *ref, *base)
	if err != nil {
		return err
	}

	// The base of the cut is the tip of this minor's latest beta, so a second
	// cut adds to the first rather than starting again from the minor's start
	// and losing what the first one carried.
	betaBase := start
	if highest, found := release.HighestPatch(release.Beta, *base, tags); found {
		betaBase = release.Tag(release.Beta, *base, highest)
	}

	candidates, err := release.Candidates(g, start, *ref, *base)
	if err != nil {
		return err
	}
	carried, err := release.CarriedSHAs(g, betaBase)
	if err != nil {
		return err
	}
	selection, err := release.Select(strings.Split(*cards, ","), candidates, parsedLinks, carried)
	if err != nil {
		return err
	}

	if len(selection.Unconstrained) > 0 {
		// Printed rather than assumed silently. The dependency check believes
		// what the dispatcher declared, so the cards it was told to treat as
		// having no predecessor belong in the run's own log where they can be
		// read back after a bad cut.
		fmt.Fprintf(os.Stderr, "declared to have no predecessor, and promoted on that declaration: %s\n", strings.Join(selection.Unconstrained, ", "))
	}
	for _, already := range selection.AlreadyCarried {
		fmt.Fprintf(os.Stderr, "%s is already in this minor's beta lineage, so it is not picked again\n", already)
	}
	for _, pick := range selection.Picks {
		fmt.Fprintf(os.Stderr, "picking %s (%s) from %s\n", pick.SHA, pick.Card, pick.Tag)
	}

	if err := release.CherryPick(g, betaBase, selection.Picks); err != nil {
		return err
	}
	head, err := g.Run("rev-parse", "HEAD")
	if err != nil {
		return err
	}

	return emit(map[string]string{
		"tag":  release.NextTag(release.Beta, *base, tags),
		"base": betaBase,
		"head": head,
	})
}

// emit writes the step's outputs where GitHub Actions reads them, or to
// standard output when nothing is listening, which is how the command behaves
// when somebody runs it by hand to see what a cut would produce.
func emit(outputs map[string]string) error {
	path := os.Getenv("GITHUB_OUTPUT")
	if path == "" {
		for _, key := range []string{"tag", "base", "head"} {
			if value, ok := outputs[key]; ok {
				fmt.Printf("%s=%s\n", key, value)
			}
		}
		return nil
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	for _, key := range []string{"tag", "base", "head"} {
		if value, ok := outputs[key]; ok {
			if _, err := fmt.Fprintf(file, "%s=%s\n", key, value); err != nil {
				return err
			}
		}
	}
	return nil
}
