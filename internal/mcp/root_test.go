package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dinah/internal/bench"
	"dinah/internal/contract"
	"dinah/internal/verb"
)

// mcpForest builds a directory holding a workbench at each relative place
// named, and returns that directory together with a library over the first of
// them. The library is what the server carries as its default, which is the
// shape a real server has: one workbench resolved at startup, and a root the
// per-call arguments may reach into.
func mcpForest(t *testing.T, places ...string) (string, *verb.Library) {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "forest")
	var first *verb.Library
	for _, place := range places {
		dir := containedPath(filepath.Join(append([]string{root}, strings.Split(place, "/")...)...))
		read, err := bench.ReadDefinition([]byte(definition))
		if err != nil {
			t.Fatalf("definition: %v", err)
		}
		slug := bench.Slugify(strings.ReplaceAll(place, "/", "-"))
		if err := bench.Instantiate(dir, slug, "alka", read); err != nil {
			t.Fatalf("instantiate %s: %v", place, err)
		}
		opened, err := bench.Open(dir)
		if err != nil {
			t.Fatalf("open %s: %v", place, err)
		}
		library := verb.New(opened, filepath.Join(base, "home"))
		added := library.Add(&verb.Request{Verb: "add", Actor: "alka", Title: "A card in " + place})
		if added.Outcome != contract.OutcomeOK {
			t.Fatalf("add in %s: %s", place, added.Refusal)
		}
		if first == nil {
			first = library
		}
	}
	return root, first
}

// callWithArguments drives one tool call against a server bounded by root.
func callWithArguments(t *testing.T, root string, library *verb.Library, tool string, arguments map[string]any) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": tool, "arguments": arguments},
	})
	if err != nil {
		t.Fatalf("marshal the call: %v", err)
	}
	return payload(t, askUnderRoot(t, root, library, string(encoded)))
}

// rootScopedTools pairs each tool that grows a root argument with the member
// its answer is published under. It is read off the surface's own tables rather
// than typed out, so a verb added to one and not the other fails here.
func rootScopedTools(t *testing.T) map[string]string {
	t.Helper()
	paired := map[string]string{}
	for name := range rootScoped {
		member, named := forestMember[name]
		if !named {
			t.Errorf("%s is root-scoped and no member name is declared for its answer", name)
			continue
		}
		paired[name] = member
	}
	for name := range forestMember {
		if _, dispatched := rootScoped[name]; !dispatched {
			t.Errorf("a member name is declared for %s and nothing dispatches a root-scoped read to it", name)
		}
	}
	if len(paired) != 5 {
		t.Fatalf("the surface declares %d root-scoped tools, wanted the five a refresh needs", len(paired))
	}
	return paired
}

// TestTheRootScopedRosterIsTheParamsTableRootCarryingSet asserts that
// rootScoped is not a second, hand-maintained declaration of which reads take
// a root. internal/verb's params table already declares that set, by which
// commands carry a root parameter, and the two have to name the same tools.
//
// The gap this closes is silent in both directions and neither build nor vet
// can see it. A sixth command growing a root parameter there would have its
// schema advertise the argument and assignValue would fill req.Root, so
// dinah-282's TestEveryDeclaredParameterReachesItsDeclaredField would pass
// while this handler quietly ignored the argument and answered for one
// workbench. A tool dispatched here whose command declares no root parameter
// has the opposite defect: it reads an argument no schema advertises.
//
// The CLI head is already safe by another route, since
// TestEveryDeclaredParameterIsReadByItsCommand reads the run function, which
// is why the missing pairing showed up on this head alone.
func TestTheRootScopedRosterIsTheParamsTableRootCarryingSet(t *testing.T) {
	declared := map[string]bool{}
	for _, one := range tools {
		for _, param := range verb.Params(one.command) {
			if param.Name == "root" {
				declared[one.name] = true
			}
		}
	}
	if len(declared) == 0 {
		t.Fatal("no tool's command declares a root parameter, so this case compares two empty sets and asserts nothing")
	}
	for name := range declared {
		if _, dispatched := rootScoped[name]; !dispatched {
			t.Errorf("%s declares a root parameter, so its schema advertises the argument, and no root-scoped read is dispatched for it", name)
		}
	}
	for name := range rootScoped {
		if !declared[name] {
			t.Errorf("%s dispatches a root-scoped read and its command declares no root parameter, so it reads an argument no schema advertises", name)
		}
	}
}

// TestEveryRootScopedToolAnswersForEveryWorkbenchBeneathTheRoot asserts
// dinah-281 AC-6's first half for all five tools: a root argument is read, the
// walk runs, and the answer carries one member per workbench beneath it.
func TestEveryRootScopedToolAnswersForEveryWorkbenchBeneathTheRoot(t *testing.T) {
	root, library := mcpForest(t, "alpha", "customer/beta", "customer/deep/gamma")
	for tool, member := range rootScopedTools(t) {
		t.Run(tool, func(t *testing.T) {
			answer := callWithArguments(t, root, library, tool, map[string]any{"actor": "alka", "root": root})
			body, ok := answer[member].(map[string]any)
			if !ok {
				t.Fatalf("the answer carries no %s member: %v", member, answer)
			}
			listed, ok := body["workbenches"].([]any)
			if !ok {
				t.Fatalf("the %s member carries no workbenches array: %v", member, body)
			}
			if len(listed) != 3 {
				t.Errorf("answered for %d workbenches, wanted the three beneath the root", len(listed))
			}
		})
	}
}

// TestARootArgumentOutsideTheServersRootIsRefused asserts dinah-281 AC-6's
// second half: a root that escapes the directory the server was started with
// is refused by the same name and shape a workbench argument escaping it gets,
// on every one of the five tools and on workbenches itself.
//
// The escape refusal is compared against the one the workbench argument
// already raises, rather than against the name typed out here, so the two
// cannot drift into two different answers to one question.
func TestARootArgumentOutsideTheServersRootIsRefused(t *testing.T) {
	root, library := mcpForest(t, "alpha")
	// The path outside the root is a real directory holding a real workbench,
	// not a name nothing sits at. A missing directory is refused by the stat
	// the containment check makes before it decides anything, so a fixture
	// pointing at one asserts that a refusal happened rather than that the
	// bound was applied, and it stays green with the bound removed.
	outside := filepath.Join(root, "..", "elsewhere")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	read, err := bench.ReadDefinition([]byte(definition))
	if err != nil {
		t.Fatalf("definition: %v", err)
	}
	if err := bench.Instantiate(containedPath(outside), "elsewhere", "alka", read); err != nil {
		t.Fatalf("instantiate: %v", err)
	}

	known := callWithArguments(t, root, library, "status", map[string]any{"actor": "alka", "workbench": outside})
	wanted, _ := known["refusal"].(string)
	if wanted != contract.OutsideRoot {
		t.Fatalf("the workbench argument's own escape answered %q, wanted %s", wanted, contract.OutsideRoot)
	}

	for tool := range rootScopedTools(t) {
		t.Run(tool, func(t *testing.T) {
			answer := callWithArguments(t, root, library, tool, map[string]any{"actor": "alka", "root": outside})
			if got, _ := answer["refusal"].(string); got != wanted {
				t.Errorf("the root argument's escape answered %q, wanted the same %s the workbench argument gives", got, wanted)
			}
			if got, _ := answer["outcome"].(string); got != contract.OutcomeRefused {
				t.Errorf("outcome %q, wanted %s", got, contract.OutcomeRefused)
			}
		})
	}
	t.Run("workbenches", func(t *testing.T) {
		answer := callWithArguments(t, root, library, "workbenches", map[string]any{"path": outside})
		if got, _ := answer["refusal"].(string); got != wanted {
			t.Errorf("the path argument's escape answered %q, wanted %s", got, wanted)
		}
	})
}

// TestTheWorkbenchesToolReadsItsPathAndItsDepth asserts dinah-281 AC-13: the
// two arguments the tool's generated schema advertises are the two its handler
// applies, through bench.EnumerateDeep, and a call naming neither answers
// exactly as it did before this card.
func TestTheWorkbenchesToolReadsItsPathAndItsDepth(t *testing.T) {
	root, library := mcpForest(t, "one", "two/three", "two/four/five")

	t.Run("the path is walked downward", func(t *testing.T) {
		answer := callWithArguments(t, root, library, "workbenches", map[string]any{"path": root})
		listed, _ := answer["workbenches"].([]any)
		if len(listed) != 3 {
			t.Errorf("reported %d workbenches, wanted the three beneath the path", len(listed))
		}
	})
	t.Run("the depth bounds the walk", func(t *testing.T) {
		for depth, want := range map[string]int{"1": 1, "2": 2, "3": 3, "0": 3} {
			answer := callWithArguments(t, root, library, "workbenches",
				map[string]any{"path": root, "max-depth": depth})
			listed, _ := answer["workbenches"].([]any)
			if len(listed) != want {
				t.Errorf("at depth %s reported %d workbenches, wanted %d", depth, len(listed), want)
			}
		}
	})
	t.Run("a depth with no path to bound is refused", func(t *testing.T) {
		answer := callWithArguments(t, root, library, "workbenches", map[string]any{"max-depth": "2"})
		if got, _ := answer["refusal"].(string); got != contract.DepthWithoutRoot {
			t.Errorf("refusal %q, wanted %s", got, contract.DepthWithoutRoot)
		}
	})
	t.Run("naming neither is the walk from the server's own root", func(t *testing.T) {
		answer := callWithArguments(t, root, library, "workbenches", map[string]any{})
		listed, ok := answer["workbenches"].([]any)
		if !ok {
			t.Fatalf("the bare call carries no listing: %v", answer)
		}
		if len(listed) != 3 {
			t.Errorf("the bare call reported %d workbenches, wanted what the walk from the server's own configured root finds", len(listed))
		}
	})
}

// TestADepthWithNoRootIsRefusedOnEveryRootScopedTool asserts that the terminal
// and this head answer one question alike: a depth bound with nothing to bound
// is refused rather than read and dropped, on every tool that carries the flag.
func TestADepthWithNoRootIsRefusedOnEveryRootScopedTool(t *testing.T) {
	root, library := mcpForest(t, "alpha")
	for tool := range rootScopedTools(t) {
		t.Run(tool, func(t *testing.T) {
			answer := callWithArguments(t, root, library, tool,
				map[string]any{"actor": "alka", "max-depth": "2"})
			if got, _ := answer["refusal"].(string); got != contract.DepthWithoutRoot {
				t.Errorf("refusal %q, wanted %s", got, contract.DepthWithoutRoot)
			}
		})
	}
}
