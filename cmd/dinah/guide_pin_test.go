package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"dinah/internal/guide/guidepin"
)

// goTestFunction matches a Go test function's declaration. The capture is the
// name, which is what a pin's Provenance has to resolve to.
var goTestFunction = regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]*)\(t \*testing\.T\)`)

// testFunctionsUnder collects every test function declared under root, and
// says how many _test.go files it read.
//
// The file count is returned rather than being folded into the caller,
// because a scan that read no file finds no function, and "no function
// declared with that name" and "no file read at all" are the same answer to a
// caller that cannot tell them apart. This board has shipped two sweeps that
// reported success on an empty subject.
func testFunctionsUnder(root string) (map[string]string, int, error) {
	declared := map[string]string{}
	files := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if name := entry.Name(); name == ".git" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files++
		for _, match := range goTestFunction.FindAllStringSubmatch(string(data), -1) {
			declared[match[1]] = path
		}
		return nil
	})
	return declared, files, err
}

// TestEveryPinnedStatementStandsInItsGuideAndNamesALiveTest sweeps the pin
// roster in both of the directions a pin can rot.
//
// A guide can be edited so that a pinned sentence is gone, which is the rot
// dinah-461 found: the references guide said Dinah has no restore, thirty
// lines above a table listing restore, and nothing read the sentence. And a
// pin can outlive the test it claims to be tied to, which is the quieter
// failure, because the pin then reads as evidence while proving nothing. The
// provenance half is what catches the second.
//
// The roster's size is asserted rather than merely being non-empty. Five is
// what the references guide's "Reading the archive" section claims, and a pin
// dropped in an edit would otherwise leave a smaller sweep reporting success.
func TestEveryPinnedStatementStandsInItsGuideAndNamesALiveTest(t *testing.T) {
	roster := guidepin.Pinned()
	if len(roster) == 0 {
		t.Fatal("the pin roster is empty, so this sweep read nothing")
	}
	if len(roster) != 5 {
		t.Fatalf("the pin roster holds %d statements and the references guide's archive section makes five claims", len(roster))
	}

	// The root is held in a variable rather than being spelled twice, so that
	// a failure names the directory the scan actually read. A message naming
	// repositoryRoot while the scan read elsewhere sends its reader to the
	// wrong tree, which is the same class of defect as the guide sentence
	// this card corrected.
	provenanceRoot := repositoryRoot
	declared, files, err := testFunctionsUnder(provenanceRoot)
	if err != nil {
		t.Fatalf("reading the test functions under %s: %v", provenanceRoot, err)
	}
	if files == 0 {
		t.Fatalf("no _test.go file stands under %s, so the provenance half of this sweep read nothing", provenanceRoot)
	}

	for _, statement := range roster {
		if err := guidepin.Carries(statement.Topic, statement.Text); err != nil {
			t.Errorf("%s: %v", statement.Provenance, err)
		}
		if where, ok := declared[statement.Provenance]; !ok {
			t.Errorf("the pin on the %s guide names %s as the test proving it and no test function of that name stands under %s", statement.Topic, statement.Provenance, provenanceRoot)
		} else {
			t.Logf("%s pinned by %s", statement.Topic, filepath.Base(where)+":"+statement.Provenance)
		}
	}
	t.Logf("%d pinned statements checked, %d _test.go files scanned for their provenance", len(roster), files)
}
