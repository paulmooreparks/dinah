package release

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// downloadStep is the name release.yml gives the step that pulls the run's
// artifacts into dist/ for the asset check to inspect.
const downloadStep = "Download every binary"

// extensionArchive is the filename the extension job's artifact carries into
// dist/ when nothing filters the download. It is the file that stopped three
// consecutive release runs, and asset_check_test.go names it as the stray a
// correct guard refuses.
const extensionArchive = "dinah-universal.vsix"

// runArtifact is one artifact a release run uploads, together with the files
// it contributes to dist/ when it is downloaded with merge-multiple set.
type runArtifact struct {
	name  string
	files []string
}

// TestTheReleaseDownloadTakesThePlatformBinariesAndNothingElse holds the
// download step's committed pattern against every artifact a release run
// produces, and then runs the committed text of the asset check over the dist/
// that pattern assembles.
//
// Both directions are needed and one of them alone proves nothing. A pattern
// matching no artifact at all would also keep the extension archive out of
// dist/, so the first subtest names the six artifacts the filter has to select
// rather than only the ones it has to refuse. The last subtest assembles the
// dist/ an unfiltered download would produce and watches the guard refuse it,
// which is what says the guard is still doing its own work rather than having
// been narrowed to fit the filter.
//
// The guard's shell is lifted out of release.yml rather than retyped, exactly
// as TestTheReleaseAssetCheckRefusesAnythingButTheSixBinaries lifts it, so a
// change to the committed step reaches this test.
func TestTheReleaseDownloadTakesThePlatformBinariesAndNothingElse(t *testing.T) {
	shell := gnuShellOrSkip(t)
	guard := workflowRunBlock(t, assetCheckStep)
	pattern := workflowStepInput(t, downloadStep, "pattern")
	produced := releaseRunArtifacts(t)

	var selected, refused []runArtifact
	for _, artifact := range produced {
		if artifactPatternSelects(t, pattern, artifact.name) {
			selected = append(selected, artifact)
		} else {
			refused = append(refused, artifact)
		}
	}

	t.Run("the pattern selects the six platform-binary artifacts", func(t *testing.T) {
		var want []string
		for _, target := range Targets {
			want = append(want, "binaries-"+target.GOOS+"-"+target.GOARCH)
		}
		got := artifactNames(selected)
		sort.Strings(want)
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("the download pattern %q selects %v, and the release needs it to select %v", pattern, got, want)
		}
	})

	t.Run("the pattern refuses every other artifact of the run", func(t *testing.T) {
		if len(refused) == 0 {
			t.Fatalf("the run produced nothing outside the platform binaries, so this test is checking a filter against an artifact set that cannot exercise it")
		}
		for _, artifact := range refused {
			if !strings.HasPrefix(artifact.name, "vsix-") {
				t.Errorf("the run uploads %q, which this test did not expect to be refused; the artifact set has changed and the case needs weighing", artifact.name)
			}
		}
	})

	t.Run("what the pattern selects satisfies the asset check", func(t *testing.T) {
		workspace := scratchModule(t)
		assembleDist(t, workspace, filesOf(selected), nil)

		output, err := runShell(t, shell, workspace, guard)
		if err != nil {
			t.Fatalf("the asset check refused the dist/ the filtered download assembles: %v\n%s", err, output)
		}
		if want := "dist/ holds exactly the 6 expected binaries."; !strings.Contains(output, want) {
			t.Errorf("the asset check did not report %q, and said:\n%s", want, output)
		}
	})

	t.Run("an unfiltered download does not", func(t *testing.T) {
		workspace := scratchModule(t)
		assembleDist(t, workspace, filesOf(produced), nil)

		output, err := runShell(t, shell, workspace, guard)
		if err == nil {
			t.Errorf("the asset check accepted the dist/ an unfiltered download assembles, so it would no longer catch a file the filter did not anticipate:\n%s", output)
		}
		if want := "+" + extensionArchive; !strings.Contains(output, want) {
			t.Errorf("the asset check did not report %q, and said:\n%s", want, output)
		}
	})
}

// artifactNames returns the artifacts' names in sorted order.
func artifactNames(artifacts []runArtifact) []string {
	var names []string
	for _, artifact := range artifacts {
		names = append(names, artifact.name)
	}
	sort.Strings(names)
	return names
}

// filesOf returns every file the artifacts contribute to dist/, deduplicated
// the way merge-multiple deduplicates them: two artifacts carrying the same
// filename land as one file.
func filesOf(artifacts []runArtifact) []string {
	seen := map[string]bool{}
	var files []string
	for _, artifact := range artifacts {
		for _, name := range artifact.files {
			if seen[name] {
				continue
			}
			seen[name] = true
			files = append(files, name)
		}
	}
	return files
}

// artifactPatternSelects reports whether the download step's pattern selects
// the named artifact.
//
// actions/download-artifact documents pattern as a glob and matches it with
// @actions/glob, and this models one shape only: literal text followed by a
// single trailing star. A pattern using anything else fails the test rather
// than being guessed at, because a model quietly disagreeing with the runner
// would report on a filter the release does not have.
func artifactPatternSelects(t *testing.T, pattern, name string) bool {
	t.Helper()
	prefix, trailingStar := strings.CutSuffix(pattern, "*")
	if !trailingStar || strings.ContainsAny(prefix, "*?[]{}!") {
		t.Fatalf("release.yml's download pattern is %q, and this test models only a literal prefix followed by one trailing star; extend the model before changing the pattern's shape", pattern)
	}
	return strings.HasPrefix(name, prefix)
}

// releaseRunArtifacts returns every artifact a release run uploads.
//
// A release run is release.yml's own jobs plus ci.yml's, because release.yml
// calls ci.yml as a reusable workflow and a called workflow's uploads belong to
// the calling run. promote.yml and vscode-release.yml are their own runs and
// cannot put anything into this one's artifact set, so they are left out.
func releaseRunArtifacts(t *testing.T) []runArtifact {
	t.Helper()
	var artifacts []runArtifact
	for _, workflow := range []string{"release.yml", "ci.yml"} {
		artifacts = append(artifacts, workflowUploads(t, workflow)...)
	}
	if len(artifacts) == 0 {
		t.Fatalf("no upload-artifact step was found in the workflows a release run executes, so there is no artifact set to filter")
	}
	return artifacts
}

// workflowUploads returns the artifacts the named workflow's upload steps
// produce, with each matrix reference in an artifact name expanded over the
// values the run supplies.
//
// The expansion is deliberately narrow. Every reference it does not recognise
// fails the test by name, so a new artifact shape has to be weighed here rather
// than passing through as a literal that no pattern will ever match.
func workflowUploads(t *testing.T, workflow string) []runArtifact {
	t.Helper()
	path := filepath.Join(repositoryRoot(t), ".github", "workflows", workflow)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s to find what it uploads: %v", workflow, err)
	}
	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")

	var artifacts []runArtifact
	for at, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "uses: actions/upload-artifact") {
			continue
		}
		name := inputBelow(t, lines, at, "name", workflow)
		switch {
		case strings.Contains(name, "${{ matrix.goos }}"):
			// The build job's matrix comes from internal/release.Targets by
			// way of the resolve step, so the same declaration that decides
			// which binaries exist decides which artifact names carry them.
			for _, target := range Targets {
				expanded := strings.NewReplacer(
					"${{ matrix.goos }}", target.GOOS,
					"${{ matrix.goarch }}", target.GOARCH,
				).Replace(name)
				artifacts = append(artifacts, runArtifact{name: expanded, files: []string{target.BinaryName()}})
			}
		case strings.Contains(name, "${{ matrix.os }}"):
			// The extension job uploads what `npm run package` wrote, which
			// the workflow names by glob, so the archive's filename is spelled
			// here rather than derived. asset_check_test.go spells the same
			// name for the same reason.
			for _, runner := range operatingSystemsAbove(t, lines, at, workflow) {
				expanded := strings.ReplaceAll(name, "${{ matrix.os }}", runner)
				artifacts = append(artifacts, runArtifact{name: expanded, files: []string{extensionArchive}})
			}
		case strings.Contains(name, "${{"):
			t.Fatalf("%s uploads an artifact named %q, and this test does not know how to expand that reference; teach it before the release's download filter can be held against the name", workflow, name)
		default:
			t.Fatalf("%s uploads an artifact named %q, which carries no matrix reference, so this test cannot tell what lands in dist/ under it", workflow, name)
		}
	}
	return artifacts
}

// operatingSystemsAbove returns the runner list of the job the step at the
// given line belongs to, read from the nearest `os: [...]` line above it.
func operatingSystemsAbove(t *testing.T, lines []string, step int, workflow string) []string {
	t.Helper()
	for at := step; at >= 0; at-- {
		trimmed := strings.TrimSpace(lines[at])
		if !strings.HasPrefix(trimmed, "os: [") || !strings.HasSuffix(trimmed, "]") {
			continue
		}
		var runners []string
		for _, entry := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(trimmed, "os: ["), "]"), ",") {
			if trimmedEntry := strings.TrimSpace(entry); trimmedEntry != "" {
				runners = append(runners, trimmedEntry)
			}
		}
		if len(runners) == 0 {
			t.Fatalf("%s carries an empty runner list at line %d", workflow, at+1)
		}
		return runners
	}
	t.Fatalf("%s uploads an artifact named for ${{ matrix.os }} at line %d and declares no runner list above it", workflow, step+1)
	return nil
}

// inputBelow returns the value of the named key in the `with:` mapping that
// follows the step's uses: line.
func inputBelow(t *testing.T, lines []string, uses int, key, workflow string) string {
	t.Helper()
	for at := uses + 1; at < len(lines); at++ {
		trimmed := strings.TrimSpace(lines[at])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			break
		}
		if value, found := strings.CutPrefix(trimmed, key+": "); found {
			return strings.TrimSpace(value)
		}
	}
	t.Fatalf("%s carries an upload step at line %d with no %s: input", workflow, uses+1, key)
	return ""
}

// workflowStepInput returns the value of one key in the `with:` mapping of the
// named step of release.yml.
func workflowStepInput(t *testing.T, step, key string) string {
	t.Helper()
	path := filepath.Join(repositoryRoot(t), ".github", "workflows", "release.yml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the workflow the step lives in: %v", err)
	}
	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")

	for at, line := range lines {
		if strings.TrimSpace(line) != "- name: "+step {
			continue
		}
		for below := at + 1; below < len(lines); below++ {
			trimmed := strings.TrimSpace(lines[below])
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			if strings.HasPrefix(trimmed, "- name:") {
				break
			}
			if value, found := strings.CutPrefix(trimmed, key+": "); found {
				return strings.TrimSpace(value)
			}
		}
		t.Fatalf("the step %q carries no %s: input, so the release downloads without one", step, key)
	}
	t.Fatalf("release.yml carries no step named %q, so the step this test exercises has been renamed or removed", step)
	return ""
}
