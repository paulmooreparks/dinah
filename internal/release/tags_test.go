package release

import "testing"

// The tag list this repository actually carries after the cutover: the old
// shape for everything published before it, the new shape for everything
// after, and both for the same line. The operator ruled that no published tag
// is rewritten, so this mixture is the permanent input rather than a
// transitional one.
var mixedShapes = []string{
	"v0.1.0-dev.71",
	"v0.1.0-dev.72",
	"v0.1.0-dev.73",
	"v0.1.74-dev",
	"v0.1.75-dev",
	"v0.1.0-beta",
	"v0.1.1-beta",
	"v0.1.0",
}

func TestPatchReadsBothDevShapes(t *testing.T) {
	cases := []struct {
		channel Channel
		tag     string
		want    int
		ok      bool
	}{
		{Dev, "v0.1.0-dev.73", 73, true},
		{Dev, "v0.1.74-dev", 74, true},
		{Dev, "v0.1.0-dev", 0, true},
		{Dev, "v0.1.0", 0, false},
		{Dev, "v0.1.2-beta", 0, false},
		{Dev, "v0.2.5-dev", 0, false},
		{Dev, "v0.1.x-dev", 0, false},
		{Beta, "v0.1.3-beta", 3, true},
		{Beta, "v0.1.0-dev.3", 0, false},
		{Stable, "v0.1.4", 4, true},
		{Stable, "v0.1.0-dev.7", 0, false},
		{Stable, "v0.1.2-beta", 0, false},
		{Stable, "v0.10.2", 0, false},
	}
	for _, c := range cases {
		got, ok := Patch(c.channel, "0.1", c.tag)
		if ok != c.ok || got != c.want {
			t.Errorf("Patch(%s, 0.1, %q) = %d, %t; want %d, %t", c.channel, c.tag, got, ok, c.want, c.ok)
		}
	}
}

// The failure this guards is the one the operator's ruling on the old tags
// creates: a next-number computation that reads only the new shape sees no
// tags at all on a line that already reached 73, and issues v0.1.1-dev over
// the top of a counter in the seventies.
func TestNextDevTagContinuesAcrossTheCutover(t *testing.T) {
	oldOnly := []string{"v0.1.0-dev.71", "v0.1.0-dev.72", "v0.1.0-dev.73"}
	if got := NextTag(Dev, "0.1", oldOnly); got != "v0.1.74-dev" {
		t.Errorf("with only old-shape tags present, next dev tag = %q; want v0.1.74-dev", got)
	}
	if got := NextTag(Dev, "0.1", mixedShapes); got != "v0.1.76-dev" {
		t.Errorf("with both shapes present, next dev tag = %q; want v0.1.76-dev", got)
	}
	newOnly := []string{"v0.1.74-dev", "v0.1.75-dev"}
	if got := NextTag(Dev, "0.1", newOnly); got != "v0.1.76-dev" {
		t.Errorf("with only new-shape tags present, next dev tag = %q; want v0.1.76-dev", got)
	}
}

// A beta or stable number counts releases within the minor from zero, and a
// dev number counts builds from one, which is what release.yml has always
// computed. An empty line is where the two rules differ, so it is where this
// asserts.
func TestNextPatchOnAnEmptyLine(t *testing.T) {
	if got := NextTag(Dev, "0.2", mixedShapes); got != "v0.2.1-dev" {
		t.Errorf("first dev tag of a new minor = %q; want v0.2.1-dev", got)
	}
	if got := NextTag(Beta, "0.2", mixedShapes); got != "v0.2.0-beta" {
		t.Errorf("first beta tag of a new minor = %q; want v0.2.0-beta", got)
	}
	if got := NextTag(Stable, "0.2", mixedShapes); got != "v0.2.0" {
		t.Errorf("first stable tag of a new minor = %q; want v0.2.0", got)
	}
}

// The counter resets when the minor increments and when the major increments,
// which falls out of every count being taken within one major.minor line. The
// assertion is that a line's own tags are the only ones it ever sees.
func TestEachLineCountsOnlyItsOwnTags(t *testing.T) {
	across := []string{"v0.1.0-dev.73", "v0.1.74-dev", "v0.2.3-dev", "v1.0.9-dev"}
	if got := NextTag(Dev, "0.2", across); got != "v0.2.4-dev" {
		t.Errorf("next dev tag on 0.2 = %q; want v0.2.4-dev", got)
	}
	if got := NextTag(Dev, "1.0", across); got != "v1.0.10-dev" {
		t.Errorf("next dev tag on 1.0 = %q; want v1.0.10-dev", got)
	}
	if got := NextTag(Beta, "1.0", across); got != "v1.0.0-beta" {
		t.Errorf("next beta tag on 1.0 = %q; want v1.0.0-beta", got)
	}
}

func TestNextBetaAndStableFollowTheirOwnLines(t *testing.T) {
	if got := NextTag(Beta, "0.1", mixedShapes); got != "v0.1.2-beta" {
		t.Errorf("next beta tag = %q; want v0.1.2-beta", got)
	}
	if got := NextTag(Stable, "0.1", mixedShapes); got != "v0.1.1" {
		t.Errorf("next stable tag = %q; want v0.1.1", got)
	}
}

func TestParseChannelRejectsAnythingElse(t *testing.T) {
	for _, name := range []string{"dev", "beta", "stable"} {
		if _, err := ParseChannel(name); err != nil {
			t.Errorf("ParseChannel(%q) rejected a real channel: %v", name, err)
		}
	}
	if _, err := ParseChannel("nightly"); err == nil {
		t.Error("ParseChannel accepted a channel that does not exist")
	}
}
