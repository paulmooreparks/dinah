package main

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// This check couples the scan coverage to the actual publication matrix, so
// adding an artifact cannot silently leave its platform outside the gate.
func TestEveryPublishedPlatformHasABlockingSourceScan(t *testing.T) {
	var workflow struct {
		Jobs map[string]struct {
			Needs    string `yaml:"needs"`
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
	if build.Needs != "vulnerabilities" || workflow.Jobs["publish"].Needs != "build" {
		t.Fatal("publication does not depend on all vulnerability scans")
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
			if step.Uses != "" && !sha.MatchString(step.Uses) {
				t.Fatalf("job %s uses an unpinned action %s", name, step.Uses)
			}
			if step.With["go-version"] != "" || (strings.HasPrefix(step.Uses, "actions/setup-go@") && step.With["go-version-file"] != "dinah/go.mod") {
				t.Fatalf("job %s does not read its Go version from go.mod", name)
			}
		}
	}
}
