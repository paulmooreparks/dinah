// Package release holds the decisions the release and promotion workflows
// make, separated from the shell that carries them out.
//
// Two workflows need the same answers. release.yml asks what the next dev tag
// is, and promote.yml asks what the next beta or stable tag is, which cards a
// cut may carry, and which commits a beta lineage already holds. Those answers
// used to be a few lines of shell inside one workflow file, where nothing
// could test them. They live here instead, so a fixture tag list and a fixture
// repository can drive them under go test, and the workflows call the small
// command in cmd/dinah-release rather than reimplementing the rules.
package release

import (
	"fmt"
	"strconv"
	"strings"
)

// Channel names a release line. The three lines share one tag namespace and
// one number position, and the number counts a different thing in each of
// them: builds in dev, beta releases within the minor in beta, stable releases
// within the minor in stable. Nothing compares a number across two channels,
// because two such numbers count different things.
type Channel string

const (
	Dev    Channel = "dev"
	Beta   Channel = "beta"
	Stable Channel = "stable"
)

// Channels lists the three lines in the order the release document introduces
// them.
var Channels = []Channel{Dev, Beta, Stable}

// ParseChannel resolves a channel name, rejecting anything outside the three.
func ParseChannel(name string) (Channel, error) {
	for _, c := range Channels {
		if string(c) == name {
			return c, nil
		}
	}
	return "", fmt.Errorf("%q is not a release channel; the channels are dev, beta and stable", name)
}

// Patch reports the number a tag carries for the given base ("major.minor",
// the content of the VERSION file) on the given channel, and whether the tag
// belongs to that channel and that line at all.
//
// The dev channel reads two shapes, and it will go on reading two shapes for
// as long as this repository keeps its history. Tags up to the cutover carry
// the counter in the pre-release label with the patch pinned at zero
// (v0.1.0-dev.73), and tags from the cutover forward carry the counter in the
// patch position (v0.1.74-dev). The operator ruled that no published tag is
// ever rewritten, so both shapes are permanent, and a reader that knows only
// the new one would compute a next number of 1 and restart a counter that had
// reached the seventies.
func Patch(c Channel, base, tag string) (int, bool) {
	switch c {
	case Dev:
		if n, ok := numberBetween(tag, "v"+base+".", "-dev"); ok {
			return n, true
		}
		return numberBetween(tag, "v"+base+".0-dev.", "")
	case Beta:
		return numberBetween(tag, "v"+base+".", "-beta")
	case Stable:
		return numberBetween(tag, "v"+base+".", "")
	}
	return 0, false
}

// numberBetween reads the decimal number a tag carries between a prefix and a
// suffix, and reports whether the tag has that shape and nothing else in it.
// The digits-only test is what keeps v0.1.0-dev.7 out of the stable channel's
// results and v0.1.2-beta out of the dev channel's: both carry the right
// prefix, and neither leaves digits alone in the middle.
func numberBetween(tag, prefix, suffix string) (int, bool) {
	if !strings.HasPrefix(tag, prefix) || !strings.HasSuffix(tag, suffix) {
		return 0, false
	}
	middle := tag[len(prefix) : len(tag)-len(suffix)]
	if middle == "" {
		return 0, false
	}
	for _, r := range middle {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(middle)
	if err != nil {
		return 0, false
	}
	return n, true
}

// HighestPatch reports the largest number any tag in the list carries for the
// channel and the line, and whether the line has any tag at all.
func HighestPatch(c Channel, base string, tags []string) (int, bool) {
	highest := 0
	found := false
	for _, tag := range tags {
		n, ok := Patch(c, base, strings.TrimSpace(tag))
		if !ok {
			continue
		}
		if !found || n > highest {
			highest = n
			found = true
		}
	}
	return highest, found
}

// NextPatch reports the number the next release on this channel and line
// takes.
//
// The two answers for an empty line differ on purpose. A dev line with no tags
// yet starts at 1, which is what release.yml has always computed by seeding
// its maximum at zero and adding one, and changing it would renumber nothing
// while breaking the one guarantee the counter offers, which is that it only
// goes up. A beta or stable line with no tags yet starts at 0, because
// vX.Y.0-beta and vX.Y.0 are the first release of a line rather than a gap
// where a zeroth release should have been.
func NextPatch(c Channel, base string, tags []string) int {
	highest, found := HighestPatch(c, base, tags)
	if found {
		return highest + 1
	}
	if c == Dev {
		return 1
	}
	return 0
}

// Tag builds the tag string for a channel, a line and a number.
func Tag(c Channel, base string, patch int) string {
	switch c {
	case Stable:
		return fmt.Sprintf("v%s.%d", base, patch)
	default:
		return fmt.Sprintf("v%s.%d-%s", base, patch, c)
	}
}

// NextTag builds the tag the next release on this channel and line takes.
func NextTag(c Channel, base string, tags []string) string {
	return Tag(c, base, NextPatch(c, base, tags))
}
