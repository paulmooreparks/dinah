package release

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// assetCheckStep is the name release.yml gives the step this file exercises.
// The step asserts that dist/ holds exactly the binaries a release publishes,
// and it is the gate the extension archive slipped past before dinah-396.
const assetCheckStep = "Confirm dist/ carries exactly what a release should carry"

// TestTheReleaseAssetCheckRefusesAnythingButTheSixBinaries runs the committed
// text of release.yml's asset-check step against assembled dist/ directories.
//
// The step's own bytes are lifted out of the workflow and executed, rather
// than a copy of them being retyped here. A test that ran a retyped copy would
// go on passing after the workflow's copy drifted away from it, which is the
// class of defect this card was filed over.
//
// release.yml only runs on a push to the trunk, so nothing on a pull request
// executes this step. Extracting it is what gives it a standing test at all.
// What the extraction does not prove is that the surrounding workflow still
// reaches the step, or that the file parses as YAML at all; nothing in this
// repository parses a workflow file, which is dinah-401's subject.
func TestTheReleaseAssetCheckRefusesAnythingButTheSixBinaries(t *testing.T) {
	shell := gnuShellOrSkip(t)
	script := workflowRunBlock(t, assetCheckStep)

	stray := "dinah-universal.vsix"
	missing := publishedBinaries[len(publishedBinaries)-1]

	for _, testCase := range []struct {
		name        string
		files       []string
		directories []string
		wantFailure bool
		wantOutput  string
	}{
		{
			// The false-failure guard. A check that fires on a correct
			// build would be turned off within a week of landing.
			name:       "exactly the six binaries",
			files:      publishedBinaries,
			wantOutput: "dist/ holds exactly the 6 expected binaries.",
		},
		{
			name:        "a stray file beside the six",
			files:       append(append([]string{}, publishedBinaries...), stray),
			wantFailure: true,
			wantOutput:  "+" + stray,
		},
		{
			// An artifact carrying more than one file arrives as a
			// directory, because actions/upload-artifact roots the
			// upload at the least common ancestor of what it matched.
			name:        "a stray directory beside the six",
			files:       publishedBinaries,
			directories: []string{"vsix-ubuntu-latest"},
			wantFailure: true,
			wantOutput:  "+vsix-ubuntu-latest",
		},
		{
			name:        "one of the six never arrived",
			files:       publishedBinaries[:len(publishedBinaries)-1],
			wantFailure: true,
			wantOutput:  "-" + missing,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			workspace := scratchModule(t)
			assembleDist(t, workspace, testCase.files, testCase.directories)

			output, err := runShell(t, shell, workspace, script)
			if testCase.wantFailure && err == nil {
				t.Errorf("the asset check accepted a dist/ it should have refused, and said:\n%s", output)
			}
			if !testCase.wantFailure && err != nil {
				t.Errorf("the asset check refused a dist/ holding exactly what a release publishes: %v\n%s", err, output)
			}
			if !strings.Contains(output, testCase.wantOutput) {
				t.Errorf("the asset check did not report %q, and said:\n%s", testCase.wantOutput, output)
			}
		})
	}
}

// resolveStep is the name release.yml gives the step that hands the platform
// list to the build matrix.
const resolveStep = "Resolve the platform target list"

// TestTheResolveStepWritesOneLineAndFailsOnADeadCommand runs the committed
// text of that step, once against a working Go toolchain and once against one
// that exits non-zero.
//
// Both halves matter and neither is obvious from reading the step. The output
// has to be a single line, because a step writing a multi-line value to
// GITHUB_OUTPUT without the delimiter form loses everything after the first
// line. And the step has to die when the command it runs dies: an earlier
// spelling put the command substitution inside echo, where its exit status was
// discarded, so a dead toolchain wrote an empty value and left the step green.
func TestTheResolveStepWritesOneLineAndFailsOnADeadCommand(t *testing.T) {
	shell := gnuShellOrSkip(t)
	script := workflowRunBlock(t, resolveStep)

	t.Run("the list reaches GITHUB_OUTPUT on one line", func(t *testing.T) {
		workspace := scratchModule(t)
		output := filepath.Join(workspace, "github-output.txt")
		result, err := runShell(t, shell, workspace, script, "GITHUB_OUTPUT="+strings.ReplaceAll(output, `\`, "/"))
		if err != nil {
			t.Fatalf("the step failed against a working toolchain: %v\n%s", err, result)
		}
		written, err := os.ReadFile(output)
		if err != nil {
			t.Fatalf("the step wrote no GITHUB_OUTPUT: %v", err)
		}
		lines := strings.Split(strings.TrimSuffix(strings.ReplaceAll(string(written), "\r\n", "\n"), "\n"), "\n")
		if len(lines) != 1 {
			t.Errorf("the step wrote %d lines to GITHUB_OUTPUT, and a value spanning more than one line loses everything after the first:\n%s", len(lines), written)
		}
		if want := `targets=[{"goos":`; !strings.HasPrefix(lines[0], want) {
			t.Errorf("the step wrote %q, which does not start with %q, so the build matrix would not parse it", lines[0], want)
		}
	})

	t.Run("a dead toolchain fails the step", func(t *testing.T) {
		workspace := scratchModule(t)
		output := filepath.Join(workspace, "github-output.txt")
		result, err := runShell(t, shell, workspace, script,
			"GITHUB_OUTPUT="+strings.ReplaceAll(output, `\`, "/"),
			"PATH="+deadToolchain(t, workspace)+string(os.PathListSeparator)+os.Getenv("PATH"))
		if err == nil {
			t.Errorf("the step succeeded although the command it runs died, so an empty target list would reach the build matrix:\n%s", result)
		}
	})
}

// deadToolchain returns a directory holding a go that exits non-zero, for
// putting at the head of the step's PATH.
func deadToolchain(t *testing.T, workspace string) string {
	t.Helper()
	dir := filepath.Join(workspace, "dead-toolchain")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("making the stand-in toolchain directory: %v", err)
	}
	stub := "#!/bin/sh\necho 'stand-in go, exiting 1' >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(dir, "go"), []byte(stub), 0o755); err != nil {
		t.Fatalf("writing the stand-in go: %v", err)
	}
	return strings.ReplaceAll(dir, `\`, "/")
}

// gnuShellOrSkip returns a bash the extracted step can run under, and skips
// when the machine has no GNU userland to run it with.
//
// The step is written for the runner it executes on, which is ubuntu-latest,
// and it uses find's -printf, a GNU findutils extension that the find shipped
// with macOS does not carry. Running the step where its own tools are missing
// would test the machine rather than the step, so this skips there and leaves
// the coverage on Linux and on Windows under Git Bash, the two places the
// tools are present.
func gnuShellOrSkip(t *testing.T) string {
	t.Helper()
	shell, err := exec.LookPath("bash")
	if err != nil {
		t.Skipf("release.yml's asset check is shell, and this machine has no bash to run it under: %v", err)
	}
	version, err := exec.Command(shell, "-c", "find --version").CombinedOutput()
	if err != nil || !strings.Contains(string(version), "GNU findutils") {
		t.Skipf("release.yml's asset check uses find -printf, which only GNU findutils carries, and this bash resolves find to something else: %s", strings.TrimSpace(string(version)))
	}
	return shell
}

// workflowRunBlock returns the shell text of the named step of release.yml,
// dedented to column zero the way the runner hands a block scalar to bash.
func workflowRunBlock(t *testing.T, step string) string {
	t.Helper()
	path := filepath.Join(repositoryRoot(t), ".github", "workflows", "release.yml")
	workflow, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the workflow the step lives in: %v", err)
	}
	lines := strings.Split(strings.ReplaceAll(string(workflow), "\r\n", "\n"), "\n")

	named := -1
	for at, line := range lines {
		if strings.TrimSpace(line) == "- name: "+step {
			named = at
			break
		}
	}
	if named < 0 {
		t.Fatalf("%s carries no step named %q, so the step this test exercises has been renamed or removed", path, step)
	}

	key := -1
	for at := named + 1; at < len(lines); at++ {
		trimmed := strings.TrimSpace(lines[at])
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "- name:") {
			break
		}
		if trimmed == "run: |" {
			key = at
			break
		}
	}
	if key < 0 {
		t.Fatalf("the step %q no longer carries a `run: |` block, so there is no shell here to run", step)
	}

	var block []string
	keyIndent := indentOf(lines[key])
	for at := key + 1; at < len(lines); at++ {
		if strings.TrimSpace(lines[at]) == "" {
			block = append(block, "")
			continue
		}
		if indentOf(lines[at]) <= keyIndent {
			break
		}
		block = append(block, lines[at])
	}
	for len(block) > 0 && block[len(block)-1] == "" {
		block = block[:len(block)-1]
	}
	if len(block) == 0 {
		t.Fatalf("the step %q carries an empty run block", step)
	}

	strip := -1
	for _, line := range block {
		if line == "" {
			continue
		}
		if width := indentOf(line); strip < 0 || width < strip {
			strip = width
		}
	}
	for at, line := range block {
		if line != "" {
			block[at] = line[strip:]
		}
	}
	return strings.Join(block, "\n") + "\n"
}

// indentOf counts the leading spaces of a line. YAML forbids tabs for
// indentation, so spaces are the only case to handle.
func indentOf(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

// repositoryRoot is the checkout this package is compiled from, which is two
// directories above internal/release.
func repositoryRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("locating the checkout this test runs from: %v", err)
	}
	return root
}

// scratchModule copies enough of the checkout into a temporary directory for
// `go run ./cmd/dinah-release` to work there.
//
// The step is written for a working directory that is both the module root and
// the directory holding dist/, and the checkout cannot be the second of those:
// assembling fixtures in the real dist/ would write into whatever tree the
// suite is running against. Copying the one command and the one package it
// imports gives a module root that is disposable. Only the fixtures are
// scaffolding here; the Go the step shells out to is the committed source.
func scratchModule(t *testing.T) string {
	t.Helper()
	root := repositoryRoot(t)
	workspace := t.TempDir()
	for _, name := range []string{"go.mod", "go.sum"} {
		copyInto(t, filepath.Join(root, name), filepath.Join(workspace, name))
	}
	for _, pkg := range [][]string{{"cmd", "dinah-release"}, {"internal", "release"}} {
		from := filepath.Join(append([]string{root}, pkg...)...)
		to := filepath.Join(append([]string{workspace}, pkg...)...)
		if err := os.MkdirAll(to, 0o755); err != nil {
			t.Fatalf("making the scratch module's %s: %v", filepath.Join(pkg...), err)
		}
		sources, err := filepath.Glob(filepath.Join(from, "*.go"))
		if err != nil {
			t.Fatalf("listing %s: %v", from, err)
		}
		for _, source := range sources {
			copyInto(t, source, filepath.Join(to, filepath.Base(source)))
		}
	}
	return workspace
}

// copyInto copies one file, failing the test rather than the run.
func copyInto(t *testing.T, from, to string) {
	t.Helper()
	content, err := os.ReadFile(from)
	if err != nil {
		t.Fatalf("reading %s: %v", from, err)
	}
	if err := os.WriteFile(to, content, 0o644); err != nil {
		t.Fatalf("writing %s: %v", to, err)
	}
}

// assembleDist builds the dist/ the step will inspect. The named directories
// each get a file inside them, because an artifact that arrives as a directory
// arrives with its contents.
func assembleDist(t *testing.T, workspace string, files, directories []string) {
	t.Helper()
	dist := filepath.Join(workspace, "dist")
	if err := os.MkdirAll(dist, 0o755); err != nil {
		t.Fatalf("making the stand-in dist/: %v", err)
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dist, name), []byte("stand-in for "+name+"\n"), 0o644); err != nil {
			t.Fatalf("writing the stand-in %s: %v", name, err)
		}
	}
	for _, name := range directories {
		nested := filepath.Join(dist, name)
		if err := os.MkdirAll(nested, 0o755); err != nil {
			t.Fatalf("making the stand-in directory %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(nested, "dinah-universal.vsix"), []byte("stand-in\n"), 0o644); err != nil {
			t.Fatalf("writing inside the stand-in directory %s: %v", name, err)
		}
	}
}

// runShell writes the extracted step to a file and runs it with the workspace
// as its working directory, which is what the runner does with a step's block
// scalar.
func runShell(t *testing.T, shell, workspace, script string, environment ...string) (string, error) {
	t.Helper()
	path := filepath.Join(workspace, "extracted-step.sh")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("writing the extracted step out to run it: %v", err)
	}
	command := exec.Command(shell, strings.ReplaceAll(path, `\`, "/"))
	command.Dir = workspace
	command.Env = append(os.Environ(), environment...)
	output, err := command.CombinedOutput()
	return string(output), err
}
