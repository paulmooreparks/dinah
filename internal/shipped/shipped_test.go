package shipped

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestConfigsMatchThePromoteWorkflow holds Configs to the platforms
// .github/workflows/promote.yml builds, each with no tags and with tui.
func TestConfigsMatchThePromoteWorkflow(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "promote.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var workflow []string
	for _, line := range strings.Split(string(data), "\n") {
		rest, ok := strings.CutPrefix(strings.TrimSpace(line), "for target in ")
		if !ok {
			continue
		}
		rest, _, _ = strings.Cut(rest, ";")
		for _, target := range strings.Fields(rest) {
			parts := strings.Split(target, "/")
			if len(parts) < 2 {
				t.Fatalf("promote.yml names a target %q that is not os/arch/extension", target)
			}
			workflow = append(workflow, parts[0]+"/"+parts[1], parts[0]+"/"+parts[1]+" tui")
		}
	}
	if !strings.Contains(string(data), "-tags tui") {
		t.Errorf("promote.yml no longer builds with -tags tui, so the tui configurations name a build nobody makes")
	}
	var shipped []string
	for _, config := range Configs {
		shipped = append(shipped, config.String())
	}
	sort.Strings(workflow)
	sort.Strings(shipped)
	if len(workflow) == 0 || strings.Join(workflow, ",") != strings.Join(shipped, ",") {
		t.Errorf("promote.yml builds %v and Configs names %v", workflow, shipped)
	}
}
