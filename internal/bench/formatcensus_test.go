package bench

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// censusDeliberate are the tests that open a store declaring the card-unit
// format while the layout is switched off on purpose, because the refusal
// that open meets is what they assert, each keyed by the test's full name as
// the census records it, with its reason.
var censusDeliberate = map[string]string{
	"dinah/internal/bench.TestAFormatTwelveStoreIsRefusedWhileTheSwitchIsOff":      "asserts that every opener refuses a format-12 store unsupported-version while the layout is switched off",
	"dinah/internal/bench.TestAStoreMidStorageMigrationIsRefusedWithEitherSetting": "asserts that a store the storage migration stamped is refused with the layout switched off, as every older build refuses it",
	"dinah/cmd/dinah.TestAMigratedStoreIsRefusedByTheSwitchOffBuild":               "asserts that dinah check and dinah show refuse the migrated fixture unsupported-version once the layout is switched off again",
	"dinah/cmd/dinah.TestAStoreOfEitherVintageIsRefusedByTheOther":                 "asserts that a store declaring the format after the one this build opens is refused unsupported-version, which with the layout switched off is format 12",
}

// TestTheFormatCensus is the census half of dinah-637/criteria/39. The
// format-census job of .github/workflows/ci.yml runs the whole suite with
// testdata/formatcensus/census.go added to this package through go test
// -overlay and DINAH_FORMAT_CENSUS naming a directory, so every test process
// records that it ran and every format-12 store an opener refused while the
// layout was switched off, with the test that opened it. This test then reads
// that directory, named by DINAH_FORMAT_CENSUS_READ, and holds every such open
// to censusDeliberate: an open from any other test is a test that opened a
// format-12 store without calling EnableCardUnitForTest first, which counts as
// a failure here even where that test passed. Each deliberate test must
// appear too, so the census cannot pass by reading nothing, and so must the
// test processes of the packages that open stores. Without
// DINAH_FORMAT_CENSUS_READ the test skips, which is every run but that job's.
//
// Arming: a census line naming a test censusDeliberate does not declare is
// what a forgetful test leaves, and taking
// TestAStoreOfEitherVintageIsRefusedByTheOther out of the table makes its own
// line one; this test then names it with the store it opened. A census
// directory holding no file fails the process assertions.
func TestTheFormatCensus(t *testing.T) {
	dir := os.Getenv("DINAH_FORMAT_CENSUS_READ")
	if dir == "" {
		t.Skip("set DINAH_FORMAT_CENSUS_READ to the directory a census run wrote; the format-census CI job does")
	}
	files, err := filepath.Glob(filepath.Join(dir, "census-*.txt"))
	if err != nil {
		t.Fatal(err)
	}
	processes := map[string]int{}
	opened := map[string]int{}
	undeclared := map[string][]string{}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
			fields := strings.Split(line, "\t")
			switch {
			case len(fields) == 2 && fields[0] == "process":
				processes[fields[1]]++
			case len(fields) == 3 && fields[0] == "refused":
				test := fields[1]
				opened[test]++
				if _, deliberate := censusDeliberate[test]; !deliberate {
					undeclared[test] = append(undeclared[test], fields[2])
				}
			default:
				t.Errorf("%s carries a line the census does not write: %q", file, line)
			}
		}
	}
	var binaries []string
	for binary := range processes {
		binaries = append(binaries, binary)
	}
	sort.Strings(binaries)
	t.Logf("the census read %d processes of %d test binaries (%s) and %d format-12 opens refused with the layout switched off", len(files), len(binaries), strings.Join(binaries, " "), sum(opened))
	// This package's own process is proven by its deliberate entries below,
	// and these are the other packages whose tests open stores the most.
	for _, binary := range []string{"verb.test", "dinah.test", "mcp.test", "httphead.test", "resident.test"} {
		if processes[binary] == 0 {
			t.Errorf("no %s process wrote to the census, so the run did not reach that package", binary)
		}
	}
	for test := range censusDeliberate {
		if opened[test] == 0 {
			t.Errorf("%s is declared to open a format-12 store with the layout switched off and opened none, so its entry has outlived what it excused, or the census did not run it", test)
		}
	}
	for test, roots := range undeclared {
		t.Errorf("%s opened a format-12 store with the layout switched off %d times, at %s; it calls EnableCardUnitForTest after the open, or never", test, len(roots), roots[0])
	}
}

// sum adds a count map's values.
func sum(counts map[string]int) int {
	total := 0
	for _, n := range counts {
		total += n
	}
	return total
}
