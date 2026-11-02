package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestManualExceptionIsExactAndDispatchOnly(t *testing.T) {
	const selected = "TestCtrlCInterruptsNodeServerInPowerShell"
	const expression = "${{ github.event_name == 'workflow_dispatch' && inputs.skip_test || 'none' }}"
	var workflow struct {
		On struct {
			Dispatch struct {
				Inputs map[string]struct {
					Type, Default string
					Options       []string
				} `yaml:"inputs"`
			} `yaml:"workflow_dispatch"`
		} `yaml:"on"`
		Jobs map[string]struct {
			Steps []struct {
				Name, Run string
				Env       map[string]string
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
	input, ok := workflow.On.Dispatch.Inputs["skip_test"]
	if !ok || input.Type != "choice" || input.Default != "none" || strings.Join(input.Options, ",") != "none,"+selected {
		t.Fatal("manual dispatch must default to no exception and offer only the approved test")
	}
	goStep, notesStep := false, false
	for jobName, job := range workflow.Jobs {
		for _, step := range job.Steps {
			if strings.Contains(step.Run, "-skip") {
				t.Fatal("workflow must not bypass the dispatch-only test planner")
			}
			if jobName == "go-tests" && step.Name == "Test every package" {
				goStep = step.Run == "./scripts/ci-go-tests.ps1" && step.Env["DINAH_ONE_OFF_SKIP"] == expression
			}
			if jobName == "publish" && step.Name == "Replace the latest prerelease" {
				notesStep = step.Env["DINAH_ONE_OFF_SKIP"] == expression && strings.Contains(step.Run, `if [ "$DINAH_ONE_OFF_SKIP" != "none" ]`) && strings.Contains(step.Run, `"$DINAH_ONE_OFF_SKIP" >> notes.md`)
			}
			if _, exists := step.Env["DINAH_ONE_OFF_SKIP"]; exists && jobName != "go-tests" && jobName != "publish" {
				t.Fatalf("test exception leaked into required job %s", jobName)
			}
		}
	}
	if !goStep || !notesStep {
		t.Fatal("manual exception must be dispatch-only and recorded automatically in release notes")
	}
	for _, tc := range []struct {
		event, skip       string
		allowed, excepted bool
	}{
		{"push", "none", true, false},
		{"push", selected, false, false},
		{"workflow_dispatch", "none", true, false},
		{"workflow_dispatch", selected, true, true},
		{"workflow_dispatch", "TestOther", false, false},
		{"workflow_dispatch", selected + "|TestOther", false, false},
		{"workflow_dispatch", selected + ",TestOther", false, false},
	} {
		t.Run(tc.event+"/"+tc.skip, func(t *testing.T) {
			out, err := exec.Command("pwsh", "-NoProfile", "-File", "../../scripts/ci-go-tests.ps1", "-EventName", tc.event, "-SkipTest", tc.skip, "-Describe").CombinedOutput()
			if !tc.allowed {
				if err == nil {
					t.Fatalf("unapproved exception was accepted: %s", out)
				}
				return
			}
			if err != nil {
				t.Fatalf("test plan failed: %v: %s", err, out)
			}
			var args []string
			if err := json.Unmarshal(out, &args); err != nil {
				t.Fatalf("invalid plan: %v: %s", err, out)
			}
			want := []string{"./...", "-count=1"}
			if tc.excepted {
				want = append(want, "-skip", "^"+selected+"$")
			}
			if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
				t.Fatalf("got arguments %q, want %q", args, want)
			}
		})
	}
}

// needs is a job's needs list. GitHub accepts either one job name or a list.
type needs []string

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
