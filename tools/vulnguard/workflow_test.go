package main

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// needs is a job's needs list. GitHub accepts either one job name or a list.
type needs []string

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
				fullSuite = job.If == "" && step.If == "" && step.Run == "./scripts/go-gate.ps1 ./... -count=1"
			}
		}
	}
	if !fullSuite {
		t.Fatal("the required Go job must run every test without a condition or exception")
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
