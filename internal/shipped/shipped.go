// Package shipped names the build configurations the project ships a binary
// for, so every guard over the module's source judges the files those
// binaries are built from rather than the files of whatever platform the test
// runs on. internal/durable's file-primitive guard and internal/seamguard's
// read-seam guards both load the module under these configurations. It
// imports nothing of Dinah's, so any package's tests can use it without a
// cycle.
package shipped

import (
	"go/build"
	"strings"
)

// Config is one build configuration: a platform and the build tags given.
type Config struct {
	GOOS, GOARCH string
	Tags         []string
}

// String names a configuration as "windows/amd64" or "windows/amd64 tui".
func (c Config) String() string {
	name := c.GOOS + "/" + c.GOARCH
	if len(c.Tags) > 0 {
		name += " " + strings.Join(c.Tags, ",")
	}
	return name
}

// Context answers go/build's default context set to this configuration, with
// cgo off, as every shipped binary is built.
func (c Config) Context() build.Context {
	ctxt := build.Default
	ctxt.GOOS = c.GOOS
	ctxt.GOARCH = c.GOARCH
	ctxt.BuildTags = append([]string(nil), c.Tags...)
	ctxt.CgoEnabled = false
	return ctxt
}

// Env answers the environment entries that make the go command build for
// this configuration, for a go list run beside os.Environ. The tags go on
// the command line, as -tags, and Flags answers them.
func (c Config) Env() []string {
	return []string{"GOOS=" + c.GOOS, "GOARCH=" + c.GOARCH, "CGO_ENABLED=0"}
}

// Flags answers the go command's flags for this configuration's tags, empty
// when it has none.
func (c Config) Flags() []string {
	if len(c.Tags) == 0 {
		return nil
	}
	return []string{"-tags", strings.Join(c.Tags, ",")}
}

// Configs are the configurations the project builds a binary for: the six
// platforms .github/workflows/promote.yml loops over, each built once with no
// tags (dinah) and once with the tui tag (dinah-tui), as that workflow and
// release.yml build them. TestConfigsMatchThePromoteWorkflow holds the
// platform list to the workflow's.
var Configs = func() []Config {
	var configs []Config
	for _, platform := range [][2]string{
		{"windows", "amd64"}, {"windows", "arm64"},
		{"linux", "amd64"}, {"linux", "arm64"},
		{"darwin", "amd64"}, {"darwin", "arm64"},
	} {
		configs = append(configs, Config{GOOS: platform[0], GOARCH: platform[1]})
		configs = append(configs, Config{GOOS: platform[0], GOARCH: platform[1], Tags: []string{"tui"}})
	}
	return configs
}()

// Host is the configuration this process was built for, with no tags.
func Host() Config {
	return Config{GOOS: build.Default.GOOS, GOARCH: build.Default.GOARCH}
}
