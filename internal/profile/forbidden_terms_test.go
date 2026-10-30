package profile

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The terms that must not appear in a tracked file are kept outside the repository, because a list of them would
// itself be a mention. DINAH_FORBIDDEN_TERMS names a file with one regular expression per line (blank lines and
// lines that begin with # are ignored, and matching ignores case), and the test fails on any match in a tracked
// file or in a tracked file's path. Without the variable it is skipped. A failure says where a match is and which
// line of the list it came from, never what matched, so a gate's output does not repeat a term.

const forbiddenTermsVar = "DINAH_FORBIDDEN_TERMS"

func TestNoForbiddenTermsInTrackedFiles(t *testing.T) {
	path := os.Getenv(forbiddenTermsVar)
	if path == "" {
		t.Skip(forbiddenTermsVar + " is not set")
	}
	patterns, err := loadForbiddenTerms(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(patterns) == 0 {
		t.Fatalf("%s lists no terms", path)
	}
	rootOut, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatal(err)
	}
	root := strings.TrimSpace(string(rootOut))
	out, err := exec.Command("git", "-C", root, "ls-files", "-z").Output()
	if err != nil {
		t.Fatalf("listing tracked files: %v", err)
	}
	var files []string
	for _, f := range strings.Split(string(out), "\x00") {
		if f != "" {
			files = append(files, f)
		}
	}
	for _, m := range scanForbidden(root, files, patterns) {
		t.Error(m)
	}
}

func loadForbiddenTerms(path string) ([]*regexp.Regexp, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", forbiddenTermsVar, err)
	}
	defer f.Close()
	var out []*regexp.Regexp
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		re, err := regexp.Compile("(?i)" + line)
		if err != nil {
			return nil, fmt.Errorf("line %d of the forbidden terms is not a regular expression: %w", n, err)
		}
		out = append(out, re)
	}
	return out, sc.Err()
}

// scanForbidden reports each tracked file, relative to root, whose path or text matches a pattern, as
// "file:line: term N of the list" with N the pattern's place in the list. Files under vendor/, minified scripts
// and binary files are not read.
func scanForbidden(root string, files []string, patterns []*regexp.Regexp) []string {
	var found []string
	for _, f := range files {
		slash := filepath.ToSlash(f)
		if strings.HasPrefix(slash, "vendor/") || strings.Contains(slash, "/vendor/") || strings.HasSuffix(slash, ".min.js") {
			continue
		}
		for i, re := range patterns {
			if re.MatchString(slash) {
				found = append(found, fmt.Sprintf("%s: the path matches term %d of the list", slash, i+1))
			}
		}
		data, err := os.ReadFile(filepath.Join(root, f))
		if err != nil || bytes.IndexByte(data, 0) >= 0 {
			continue
		}
		for n, line := range strings.Split(string(data), "\n") {
			for i, re := range patterns {
				if re.MatchString(line) {
					found = append(found, fmt.Sprintf("%s:%d: term %d of the list", slash, n+1, i+1))
				}
			}
		}
	}
	return found
}

func TestScanForbiddenFindsTermsWithoutRepeatingThem(t *testing.T) {
	root := t.TempDir()
	write := func(name, text string) {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("clean.go", "package x\n")
	write("dirty.go", "package x\n// Mention of Quillfeather here.\n")
	write("vendor/lib.js", "quillfeather\n")
	write("app.min.js", "quillfeather\n")
	write("image.bin", "quillfeather\x00\x01")
	write("notes-quillfeather.md", "nothing\n")
	list := filepath.Join(root, "terms.txt")
	if err := os.WriteFile(list, []byte("# a comment\n\nquill[f]eather\nnever-present\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patterns, err := loadForbiddenTerms(list)
	if err != nil || len(patterns) != 2 {
		t.Fatalf("loading the list: %v %d", err, len(patterns))
	}
	got := scanForbidden(root, []string{"clean.go", "dirty.go", "vendor/lib.js", "app.min.js", "image.bin", "notes-quillfeather.md"}, patterns)
	want := []string{"dirty.go:2: term 1 of the list", "notes-quillfeather.md: the path matches term 1 of the list"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("found %q, want %q", got, want)
	}
	for _, m := range got {
		if strings.Contains(strings.ToLower(m), "quillfeather") && !strings.Contains(m, "notes-quillfeather.md") {
			t.Fatalf("a finding repeats the term: %s", m)
		}
	}
}

func TestForbiddenTermsFileWithABadExpressionIsRefused(t *testing.T) {
	list := filepath.Join(t.TempDir(), "terms.txt")
	if err := os.WriteFile(list, []byte("fine\n(unclosed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadForbiddenTerms(list); err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("expected a refusal naming line 2, got %v", err)
	}
}
