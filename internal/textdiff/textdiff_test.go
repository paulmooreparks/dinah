package textdiff

import (
	"fmt"
	"strings"
	"testing"
)

func TestUnifiedDiffMatchesDifflib(t *testing.T) {
	// Reference output from Python's difflib.unified_diff for the same inputs.
	cases := [][3]string{
		{"a\nb\nc\nd\ne\nf\ng\nh\ni\nj\n", "a\nb\nc\nD\ne\nf\ng\nh\ni\nj\nk\n",
			"--- before\n+++ after\n@@ -1,10 +1,11 @@\n a\n b\n c\n-d\n+D\n e\n f\n g\n h\n i\n j\n+k\n"},
		{"one\ntwo\nthree\n", "one\nthree\nfour\n", "--- before\n+++ after\n@@ -1,3 +1,3 @@\n one\n-two\n three\n+four\n"},
		{"x\n", "", "--- before\n+++ after\n@@ -1 +0,0 @@\n-x\n"},
		{"", "y\n", "--- before\n+++ after\n@@ -0,0 +1 @@\n+y\n"},
		{"same\n", "same\n", ""},
	}
	var a, b strings.Builder
	for i := range 30 {
		fmt.Fprintf(&a, "line %d\n", i)
		if i == 3 || i == 20 {
			fmt.Fprintf(&b, "changed %d\n", i)
		} else {
			fmt.Fprintf(&b, "line %d\n", i)
		}
	}
	cases = append(cases, [3]string{a.String(), b.String(),
		"--- before\n+++ after\n@@ -1,7 +1,7 @@\n line 0\n line 1\n line 2\n-line 3\n+changed 3\n line 4\n line 5\n line 6\n" +
			"@@ -18,7 +18,7 @@\n line 17\n line 18\n line 19\n-line 20\n+changed 20\n line 21\n line 22\n line 23\n"})
	for i, c := range cases {
		if got := UnifiedDiff(c[0], c[1], "before", "after"); got != c[2] {
			t.Errorf("case %d:\ngot  %q\nwant %q", i, got, c[2])
		}
	}
}

func TestOpcodes(t *testing.T) {
	a, b := Tokens([]string{"x", "y", "z"}, []string{"x", "Y", "z", "w"})
	got := Opcodes(a, b)
	want := []Opcode{{'e', 0, 1, 0, 1}, {'r', 1, 2, 1, 2}, {'e', 2, 3, 2, 3}, {'i', 3, 3, 3, 4}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("%v", got)
	}
}
