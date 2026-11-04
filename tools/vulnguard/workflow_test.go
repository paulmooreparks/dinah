package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// needs is a job's needs list. GitHub accepts either one job name or a list.
type needs []string

const fullGoTestCommand = `$PSNativeCommandUseErrorActionPreference = $false
& pwsh -NoProfile -File ./scripts/go-gate.ps1 ./... -count=1 -json 2>&1 | Tee-Object -FilePath (Join-Path $env:DINAH_TEST_DIAGNOSTICS_DIR 'full-go-output.log')
$testExitCode = $LASTEXITCODE
exit $testExitCode
`

const prepareGoDiagnostics = `$diagnostics = Join-Path $env:RUNNER_TEMP 'full-go-diagnostics'
[IO.Directory]::CreateDirectory($diagnostics) | Out-Null
$nativeScratch = Join-Path $env:RUNNER_TEMP 'full-go-native-scratch'
[IO.Directory]::CreateDirectory($nativeScratch) | Out-Null
"DINAH_TEST_DIAGNOSTICS_DIR=$diagnostics" | Out-File -FilePath $env:GITHUB_ENV -Encoding utf8 -Append -ErrorAction Stop
"DINAH_TEST_NATIVE_SCRATCH_DIR=$nativeScratch" | Out-File -FilePath $env:GITHUB_ENV -Encoding utf8 -Append -ErrorAction Stop
`

func TestWorkflowGoSuiteHasNoException(t *testing.T) {
	var workflow struct {
		On   map[string]yaml.Node
		Jobs map[string]struct {
			If    string
			Steps []struct {
				Name, Run, If string
				Env           map[string]string
			}
		}
	}
	b, err := os.ReadFile("../../.github/workflows/build.yml")
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(b, &workflow); err != nil {
		t.Fatal(err)
	}
	var dispatch struct{ Inputs map[string]yaml.Node }
	if node, ok := workflow.On["workflow_dispatch"]; ok {
		if err := node.Decode(&dispatch); err != nil {
			t.Fatal(err)
		}
	}
	if _, ok := dispatch.Inputs["skip_test"]; ok {
		t.Fatal("the one-off test exception must remain withdrawn")
	}
	fullSuite := false
	for name, job := range workflow.Jobs {
		for _, step := range job.Steps {
			if strings.Contains(step.Run, "-skip") || step.Env["DINAH_ONE_OFF_SKIP"] != "" {
				t.Fatal("workflow must not carry a test exception")
			}
			if name == "go-tests" && step.Name == "Test every package" {
				fullSuite = job.If == "" && step.If == "" && step.Run == fullGoTestCommand
			}
		}
	}
	if !fullSuite {
		t.Fatal("the required Go job must run every test without a condition or exception")
	}
}

func TestFullGoDiagnosticsRetainOutputAndExitStatus(t *testing.T) {
	var workflow struct {
		Jobs map[string]struct {
			Env   map[string]string
			Steps []struct {
				Name, Run, If, Uses, ID string
				With                    map[string]string
			}
		}
	}
	b, err := os.ReadFile("../../.github/workflows/build.yml")
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(b, &workflow); err != nil {
		t.Fatal(err)
	}
	job := workflow.Jobs["go-tests"]
	if job.Env["DINAH_TEST_DIAGNOSTICS_DIR"] != "" {
		t.Fatal("diagnostics must be handed to later steps through GITHUB_ENV")
	}
	prepare, preflight, run, extraction, boundary, upload := -1, -1, -1, -1, -1, -1
	for i, step := range job.Steps {
		switch step.Name {
		case "Prepare full Go diagnostics":
			prepare = i
			if step.If != "" || step.Run != prepareGoDiagnostics {
				t.Fatal("diagnostics directory setup must fail visibly")
			}
		case "Test every package":
			run = i
		case "Record ordinary-handle Node startup":
			preflight = i
			if step.If != "" || step.Run != `go run ./tools/nodepreflight -out "$env:DINAH_TEST_DIAGNOSTICS_DIR/node-preflight.json"` {
				t.Fatal("Node preflight must retain evidence without replacing the full test command")
			}
		case "Extract restricted native stacks":
			extraction = i
			if step.If != "always()" || step.Run != `go run ./tools/diagnosticartifacts -mode extract -scratch "$env:DINAH_TEST_NATIVE_SCRATCH_DIR" -diagnostics "$env:DINAH_TEST_DIAGNOSTICS_DIR"` {
				t.Fatal("private dumps must be processed and cleaned after failed tests too")
			}
		case "Stage diagnostic upload":
			boundary = i
			if step.ID != "diagnostics_boundary" || step.If != "always()" || step.Run != `go run ./tools/diagnosticartifacts -mode stage -diagnostics "$env:DINAH_TEST_DIAGNOSTICS_DIR" -upload-parent "$env:RUNNER_TEMP" -github-output "$env:GITHUB_OUTPUT"` {
				t.Fatal("upload must use the guarded, isolated snapshot")
			}
		case "Retain full Go diagnostics":
			upload = i
			if step.If != "always() && steps.diagnostics_boundary.outcome == 'success'" || !strings.HasPrefix(step.Uses, "actions/upload-artifact@") || step.With["name"] != "full-go-diagnostics" || step.With["path"] != "${{ steps.diagnostics_boundary.outputs.upload_path }}/\n!${{ steps.diagnostics_boundary.outputs.upload_path }}/**/*.dmp\n" || step.With["if-no-files-found"] != "error" {
				t.Fatal("available full Go diagnostics must upload even after test failure")
			}
		}
	}
	if prepare < 0 || preflight <= prepare || run <= preflight || extraction <= run || boundary <= extraction || upload <= boundary {
		t.Fatal("prepare, preflight, tests, extraction, boundary and upload must run in order")
	}
	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		t.Fatal("PowerShell is required to verify the CI capture contract:", err)
	}
	var environment []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(strings.ToUpper(entry), "DINAH_TEST_DIAGNOSTICS_DIR=") {
			environment = append(environment, entry)
		}
	}
	t.Run("setup-failure", func(t *testing.T) {
		root := t.TempDir()
		blocked := filepath.Join(root, "full-go-diagnostics")
		if err := os.WriteFile(blocked, []byte("occupied"), 0600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(pwsh, "-NoProfile", "-Command", "$ErrorActionPreference = 'Stop'\n"+job.Steps[prepare].Run)
		cmd.Env = append(environment, "RUNNER_TEMP="+root, "GITHUB_ENV="+filepath.Join(root, "environment"))
		if output, err := cmd.CombinedOutput(); err == nil {
			t.Fatalf("diagnostics setup succeeded over a file: %s", output)
		}
	})
	for _, exitCode := range []int{0, 17} {
		t.Run(fmt.Sprint(exitCode), func(t *testing.T) {
			root := t.TempDir()
			envFile := filepath.Join(root, "environment")
			setup := exec.Command(pwsh, "-NoProfile", "-Command", "$ErrorActionPreference = 'Stop'\n"+job.Steps[prepare].Run)
			setup.Env = append(environment, "RUNNER_TEMP="+root, "GITHUB_ENV="+envFile)
			if output, err := setup.CombinedOutput(); err != nil {
				t.Fatalf("prepare step failed: %v: %s", err, output)
			}
			handoff, err := os.ReadFile(envFile)
			if err != nil {
				t.Fatal(err)
			}
			values := map[string]string{}
			var exported []string
			for _, line := range strings.Split(strings.TrimSpace(string(handoff)), "\n") {
				line = strings.TrimSpace(line)
				name, value, ok := strings.Cut(line, "=")
				if !ok {
					t.Fatalf("malformed environment handoff: %q", handoff)
				}
				values[name] = value
				exported = append(exported, line)
			}
			diagnostics := values["DINAH_TEST_DIAGNOSTICS_DIR"]
			if len(values) != 2 || !filepath.IsAbs(diagnostics) || filepath.Clean(diagnostics) != filepath.Join(root, "full-go-diagnostics") || filepath.Clean(values["DINAH_TEST_NATIVE_SCRATCH_DIR"]) != filepath.Join(root, "full-go-native-scratch") {
				t.Fatalf("invalid prepare-step handoff: %q", handoff)
			}
			if info, err := os.Stat(diagnostics); err != nil || !info.IsDir() {
				t.Fatalf("prepare step did not create its diagnostics directory: %v", err)
			}
			if err := os.MkdirAll(filepath.Join(root, "scripts"), 0700); err != nil {
				t.Fatal(err)
			}
			result := "[Console]::Out.WriteLine('stdout-marker')\n[Console]::Error.WriteLine('stderr-marker')\nexit " + fmt.Sprint(exitCode) + "\n"
			if err := os.WriteFile(filepath.Join(root, "result.ps1"), []byte(result), 0600); err != nil {
				t.Fatal(err)
			}
			fixture := "& pwsh -NoProfile -File ./result.ps1\nexit $LASTEXITCODE\n"
			if err := os.WriteFile(filepath.Join(root, "scripts", "go-gate.ps1"), []byte(fixture), 0600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(pwsh, "-NoProfile", "-Command", "$ErrorActionPreference = 'Stop'\n"+job.Steps[run].Run)
			cmd.Dir = root
			cmd.Env = append(environment, exported...)
			output, runErr := cmd.CombinedOutput()
			got := 0
			if runErr != nil {
				if status, ok := runErr.(*exec.ExitError); ok {
					got = status.ExitCode()
				} else {
					t.Fatal(runErr)
				}
			}
			if got != exitCode {
				t.Fatalf("capture returned %d, want %d: %s", got, exitCode, output)
			}
			captured, err := os.ReadFile(filepath.Join(diagnostics, "full-go-output.log"))
			if err != nil {
				t.Fatal(err)
			}
			for _, marker := range []string{"stdout-marker", "stderr-marker"} {
				if !strings.Contains(string(captured), marker) {
					t.Fatalf("capture lost %s: %s", marker, captured)
				}
			}
		})
	}
}

// Pin actionlint so workflow context validation is the same locally and in CI.
// Its v1.7.12 runner table includes macos-15-intel. External shell and Python
// linters are disabled; workflow syntax and expression checks remain enabled.
func TestWorkflowSyntaxWithActionlint(t *testing.T) {
	const tool = "github.com/rhysd/actionlint/cmd/actionlint@v1.7.12"
	lint := func(file string) ([]byte, error) {
		cmd := exec.Command("go", "run", tool, "-shellcheck=", "-pyflakes=", file)
		cmd.Dir = "../.."
		return cmd.CombinedOutput()
	}
	if output, err := lint(".github/workflows/build.yml"); err != nil {
		t.Fatalf("workflow lint failed: %v: %s", err, output)
	}
	invalid := filepath.Join(t.TempDir(), "invalid.yml")
	fixture := "on: push\njobs:\n  check:\n    runs-on: ubuntu-latest\n    env:\n      DIAGNOSTICS: ${{ runner.temp }}/diagnostics\n    steps:\n      - run: echo check\n"
	if err := os.WriteFile(invalid, []byte(fixture), 0600); err != nil {
		t.Fatal(err)
	}
	output, err := lint(invalid)
	if err == nil || !strings.Contains(string(output), `context "runner" is not allowed here`) {
		t.Fatalf("invalid job-env context was not rejected: %v: %s", err, output)
	}
}

func (n *needs) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		*n = needs{node.Value}
		return nil
	}
	return node.Decode((*[]string)(n))
}

func (n needs) has(job string) bool {
	for _, j := range n {
		if j == job {
			return true
		}
	}
	return false
}

// This check couples the scan coverage to the actual publication matrix, so
// adding an artifact cannot silently leave its platform outside the gate.
// Building, and so publication, must also wait for the review's guards on
// Windows and Linux and the full Go and browser suites
// (docs/design/invariants.md).
func TestEveryPublishedPlatformHasABlockingSourceScan(t *testing.T) {
	var workflow struct {
		Jobs map[string]struct {
			Needs    needs `yaml:"needs"`
			Strategy struct {
				Matrix struct {
					Include []struct {
						GOOS, GOARCH string
					} `yaml:"include"`
				} `yaml:"matrix"`
			} `yaml:"strategy"`
			Steps []struct {
				Uses string            `yaml:"uses"`
				With map[string]string `yaml:"with"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	b, err := os.ReadFile("../../.github/workflows/build.yml")
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(b, &workflow); err != nil {
		t.Fatal(err)
	}
	build, scans := workflow.Jobs["build"], workflow.Jobs["vulnerabilities"]
	for _, gate := range []string{"vulnerabilities", "guards", "guards-linux", "go-tests", "browser"} {
		if _, ok := workflow.Jobs[gate]; !ok || !build.Needs.has(gate) {
			t.Fatalf("building does not depend on the %s job", gate)
		}
	}
	if !workflow.Jobs["publish"].Needs.has("build") {
		t.Fatal("publication does not depend on the build")
	}
	want, got := map[string]bool{}, map[string]bool{}
	for _, target := range build.Strategy.Matrix.Include {
		arches := []string{target.GOARCH}
		if target.GOARCH == "universal" {
			if target.GOOS != "darwin" {
				t.Fatal("unknown universal artifact")
			}
			arches = []string{"amd64", "arm64"}
		}
		for _, arch := range arches {
			want[target.GOOS+"/"+arch] = true
		}
	}
	for _, target := range scans.Strategy.Matrix.Include {
		key := target.GOOS + "/" + target.GOARCH
		if got[key] || !want[key] {
			t.Fatalf("duplicate or unexpected source scan %s", key)
		}
		got[key] = true
	}
	if len(want) == 0 || len(want) != len(got) {
		t.Fatalf("published platforms %v differ from source scans %v", want, got)
	}
	sha := regexp.MustCompile(`^actions/[a-z-]+@[0-9a-f]{40}$`)
	for name, job := range workflow.Jobs {
		for _, step := range job.Steps {
			if name == "publish" && strings.HasPrefix(step.Uses, "actions/download-artifact@") && (step.With["pattern"] != "dinah-*" || step.With["name"] != "") {
				t.Fatal("publication must download only build archives, excluding scan reports")
			}
			if step.Uses != "" && !sha.MatchString(step.Uses) {
				t.Fatalf("job %s uses an unpinned action %s", name, step.Uses)
			}
			if step.With["go-version"] != "" || (strings.HasPrefix(step.Uses, "actions/setup-go@") && step.With["go-version-file"] != "dinah/go.mod") {
				t.Fatalf("job %s does not read its Go version from go.mod", name)
			}
		}
	}
}
