package seamguard

import (
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// constrainedPackage writes a package whose files four configurations tell
// apart, with a file no configuration builds, and answers its directory.
func constrainedPackage(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":               "module example.com/p\n\ngo 1.26\n",
		"plain.go":             "package p\n",
		"tagged.go":            "//go:build tui\n\npackage p\n\nimport \"os\"\n\nfunc tuiSideRead(p string) ([]byte, error) { return os.ReadFile(p) }\n",
		"arm_windows_arm64.go": "package p\n\nconst onWindowsArm = true\n",
		"only_linux.go":        "package p\n\nconst onLinux = true\n",
		"never.go":             "//go:build ignore\n\npackage p\n",
		"plain_test.go":        "package p\n",
	}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestDistinctSelectsEveryFileAShippedConfigurationBuilds holds the guards'
// file sets to the shipped configurations rather than to the platform the
// test runs on: a file behind the tui tag, a file named for windows/arm64 and
// a file named for Linux are each selected by some shipped configuration, and
// a file behind a tag nothing ships is reported as selected by none.
func TestDistinctSelectsEveryFileAShippedConfigurationBuilds(t *testing.T) {
	dir := constrainedPackage(t)
	configs, loaded, unselected, err := Distinct(Shipped, dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded != 4 {
		t.Errorf("the shipped configurations selected %d files, wanted the four that one of them builds", loaded)
	}
	if len(unselected) != 1 || filepath.Base(unselected[0]) != "never.go" {
		t.Errorf("the files no shipped configuration selects are %v, wanted never.go alone", unselected)
	}
	var names []string
	for _, config := range configs {
		names = append(names, config.String())
	}
	t.Logf("distinct configurations: %v", names)
	if len(configs) < 4 {
		t.Errorf("found %d distinct configurations, and plain, tagged, windows/arm64 and Linux file sets make at least four", len(configs))
	}
}

// TestLoadSelectsTheFilesItsConfigurationBuilds loads the package under the
// configuration dinah-tui is built with on windows/arm64 and finds the two
// files that configuration adds, one of them holding a read.
func TestLoadSelectsTheFilesItsConfigurationBuilds(t *testing.T) {
	dir := constrainedPackage(t)
	config := Config{GOOS: "windows", GOARCH: "arm64", Tags: []string{"tui"}}
	pkg, err := Load(dir, nil, LoadOptions{Importer: SourceImporterFor(token.NewFileSet(), config)})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, name := range pkg.Names {
		names = append(names, filepath.Base(name))
	}
	sort.Strings(names)
	if strings.Join(names, " ") != "arm_windows_arm64.go plain.go tagged.go" {
		t.Errorf("%s loaded %v, wanted arm_windows_arm64.go, plain.go and tagged.go", config, names)
	}
	g := BuildGraph(pkg, GraphOptions{})
	read := false
	for _, n := range g.Nodes {
		for _, r := range n.Reads {
			read = read || r.Member == "os.ReadFile"
		}
	}
	if !read {
		t.Errorf("%s: the graph holds no read of os.ReadFile, which only tagged.go makes, so a file behind a tag is still unjudged", config)
	}
}

// TestShippedMatchesThePromoteWorkflow holds Shipped to the platforms
// .github/workflows/promote.yml builds, each with no tags and with tui.
func TestShippedMatchesThePromoteWorkflow(t *testing.T) {
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
		t.Errorf("promote.yml no longer builds with -tags tui, so Shipped's tui configurations name a build nobody makes")
	}
	var shipped []string
	for _, config := range Shipped {
		shipped = append(shipped, config.String())
	}
	sort.Strings(workflow)
	sort.Strings(shipped)
	if len(workflow) == 0 || strings.Join(workflow, ",") != strings.Join(shipped, ",") {
		t.Errorf("promote.yml builds %v and Shipped names %v", workflow, shipped)
	}
}
