package verb

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"dinah/internal/bench"
)

// guardArm matches one routing arm of a field write, which is a case line
// naming one or more guard constants inside internal/verb/fields.go, and
// guardArmName reads the constants out of the line guardArm found.
//
// The two are separate because a case may fold several constants onto one
// line, of the shape `case bench.GuardLevel, bench.GuardTier:`, and a pattern
// that captures a name straight off the case keyword sees only the first of
// them. A folded constant read that way is invisible here and passes only
// because it happens to have a second arm elsewhere, which is a coincidence
// this check should not depend on.
var guardArm = regexp.MustCompile(`(?m)^\s*case\s+bench\.Guard[A-Za-z]+(?:,\s*bench\.Guard[A-Za-z]+)*:`) // retired spelling, named deliberately

// guardArmName reads one guard constant out of a case line guardArm matched.
// It names no package, so the whole of a folded line answers it.
var guardArmName = regexp.MustCompile(`Guard[A-Za-z]+`)

// guardConstant matches one member of the closed guard set as internal/bench
// declares it, so the expectation is read off the declaration rather than
// written out here a second time.
var guardConstant = regexp.MustCompile(`(?m)^\t(Guard[A-Za-z]+)\s+= "`)

// TestEveryDeclaredGuardIsRouted holds the closed guard set to the routing in
// this package, in both directions: a guard a field declares and nothing
// routes would be a rule nobody runs, and an arm naming a guard the set does
// not declare would be a rule nothing can reach.
//
// The two sides are independent declarations rather than one computed from the
// other. The set comes from internal/bench's constant block, and the arms come
// from the switch statements this package writes, so neither can be made to
// agree with the other by editing one file.
func TestEveryDeclaredGuardIsRouted(t *testing.T) {
	declaration, err := os.ReadFile("../bench/fields.go")
	if err != nil {
		t.Fatalf("read the guard declaration: %v", err)
	}
	declared := map[string]bool{}
	for _, match := range guardConstant.FindAllStringSubmatch(string(declaration), -1) {
		declared[match[1]] = true
	}
	if len(declared) == 0 {
		t.Fatal("internal/bench declares no guard constant this check can read, so its pattern has gone stale")
	}
	if len(bench.Guards) != len(declared) {
		t.Errorf("the constant block declares %d guards and the list beside it carries %d, so the list has fallen behind the block", len(declared), len(bench.Guards))
	}

	router, err := os.ReadFile("fields.go")
	if err != nil {
		t.Fatalf("read the router: %v", err)
	}
	routed := map[string]bool{}
	for _, line := range guardArm.FindAllString(string(router), -1) {
		for _, name := range guardArmName.FindAllString(line, -1) {
			routed[name] = true
		}
	}
	if len(routed) == 0 {
		t.Fatal("the router carries no guard arm this check can read, so its pattern has gone stale")
	}

	for name := range declared {
		if !routed[name] {
			t.Errorf("the guard constant %s is declared and the write router has no arm for it, so a field declaring it runs no rule", name)
		}
	}
	for name := range routed {
		if !declared[name] {
			t.Errorf("the write router has an arm for the guard constant %s, which the closed set does not declare", name)
		}
	}
}

// TestEveryGuardIsDeclaredByAField asserts the other direction of the same
// closed set: a guard nothing declares on any field is a rule the tool cannot
// reach, whatever the router does with it.
//
// It is separate from the routing check because the two answer different
// questions. That one asks whether a declared guard is implemented; this one
// asks whether it is used at all, and a guard used by no field is a finding
// rather than a defect, so this reports the set it found in the failure text.
func TestEveryGuardIsDeclaredByAField(t *testing.T) {
	kinds := bench.EntityKinds()
	if len(kinds) == 0 {
		t.Fatal("the grammar names no kind, so this check read nothing")
	}
	used := map[string]bool{}
	for _, kind := range kinds {
		for _, name := range bench.FieldsOf(kind) {
			field, known := bench.FieldOf(kind, name)
			if !known || field.Guard == "" {
				continue
			}
			used[field.Guard] = true
		}
	}
	if len(used) == 0 {
		t.Fatal("no field declares a guard, so this check read nothing")
	}
	var unused []string
	for _, guard := range bench.Guards {
		if !used[guard] {
			unused = append(unused, guard)
		}
	}
	sort.Strings(unused)
	if len(unused) != 0 {
		t.Errorf("the closed guard set declares %s, and no field of any kind declares them", strings.Join(unused, ", "))
	}
}
