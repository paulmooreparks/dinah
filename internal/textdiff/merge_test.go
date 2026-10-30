package textdiff

import "testing"

func TestMerge3(t *testing.T) {
	base := "# Plan\n\nFirst.\n\nSecond.\n\nThird.\n"
	for _, tc := range []struct {
		name, mine, theirs, want string
		conflicts                int
	}{
		{"only mine", "# Plan\n\nFirst, mine.\n\nSecond.\n\nThird.\n", base, "# Plan\n\nFirst, mine.\n\nSecond.\n\nThird.\n", 0},
		{"only theirs", base, "# Plan\n\nFirst.\n\nSecond.\n\nThird, theirs.\n", "# Plan\n\nFirst.\n\nSecond.\n\nThird, theirs.\n", 0},
		{"apart", "# Plan\n\nFirst, mine.\n\nSecond.\n\nThird.\n", "# Plan\n\nFirst.\n\nSecond.\n\nThird, theirs.\n", "# Plan\n\nFirst, mine.\n\nSecond.\n\nThird, theirs.\n", 0},
		{"same change", "# Plan\n\nBoth.\n\nSecond.\n\nThird.\n", "# Plan\n\nBoth.\n\nSecond.\n\nThird.\n", "# Plan\n\nBoth.\n\nSecond.\n\nThird.\n", 0},
		{"same line", "# Plan\n\nMine.\n\nSecond.\n\nThird.\n", "# Plan\n\nTheirs.\n\nSecond.\n\nThird.\n", "# Plan\n\nMine.\n\nSecond.\n\nThird.\n", 1},
		{"neighbours", "# Plan\nMine.\n\nFirst.\n\nSecond.\n\nThird.\n", "# Plan, theirs\n\nFirst.\n\nSecond.\n\nThird.\n", "", 1},
		{"insert and append", "Intro.\n" + base, base + "\nFourth.\n", "Intro.\n" + base + "\nFourth.\n", 0},
		{"mine deletes, theirs edits elsewhere", "# Plan\n\nSecond.\n\nThird.\n", "# Plan\n\nFirst.\n\nSecond.\n\nThird, theirs.\n", "# Plan\n\nSecond.\n\nThird, theirs.\n", 0},
	} {
		got, conflicts := Merge3(base, tc.mine, tc.theirs)
		if len(conflicts) != tc.conflicts || tc.conflicts == 0 && got != tc.want {
			t.Errorf("%s: %q with %d conflicts %+v", tc.name, got, len(conflicts), conflicts)
		}
	}
	_, conflicts := Merge3(base, "# Plan\n\nMine.\n\nSecond.\n\nThird.\n", "# Plan\n\nTheirs.\n\nSecond.\n\nThird.\n")
	if c := conflicts[0]; c.Line != 3 || c.Base != "First.\n" || c.Mine != "Mine.\n" || c.Theirs != "Theirs.\n" {
		t.Errorf("conflict %+v", c)
	}
}
