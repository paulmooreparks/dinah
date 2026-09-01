package profile

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

// TestInstallScriptServesEveryChannel runs scripts/install.sh against a
// stand-in release on each of the three channels and asserts it installs the
// verified binary every time.
//
// The claim under test is that the install scripts need no change to serve
// beta and stable. They read DINAH_CHANNEL, fetch channels/$channel.json, and
// parse no tag shape of their own, so the only thing a new channel changes is
// the name in the URL. That is easy to say and cheap to break: a script that
// grew a tag-shape check, or that hard-coded the dev manifest's name anywhere,
// would still pass every dev-only test in this file.
//
// The server refuses any manifest path other than the channel asked for, so a
// script that quietly fell back to dev.json fails here rather than passing on
// the wrong file.
func TestInstallScriptServesEveryChannel(t *testing.T) {
	for _, tool := range []string{"sh", "curl"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("this machine has no %s, which scripts/install.sh needs", tool)
		}
	}
	if _, shaErr := exec.LookPath("sha256sum"); shaErr != nil {
		if _, sumErr := exec.LookPath("shasum"); sumErr != nil {
			t.Skip("this machine has neither sha256sum nor shasum, which scripts/install.sh needs")
		}
	}

	root := filepath.Join("..", "..")
	source, err := os.ReadFile(filepath.Join(root, "scripts", "install.sh"))
	if err != nil {
		t.Fatalf("reading scripts/install.sh: %v", err)
	}

	const wanted = "dinah-linux-amd64"
	for _, channel := range []struct{ name, tag string }{
		{"dev", "v0.1.7-dev"},
		{"beta", "v0.1.2-beta"},
		{"stable", "v0.1.0"},
	} {
		t.Run(channel.name, func(t *testing.T) {
			var release publishedRelease
			var served []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasPrefix(path.Base(r.URL.Path), "channels/") || strings.Contains(r.URL.Path, "/channels/"):
					served = append(served, r.URL.Path)
					if !strings.HasSuffix(r.URL.Path, "/channels/"+channel.name+".json") {
						http.NotFound(w, r)
						return
					}
					w.Write(release.manifest)
				case strings.HasSuffix(r.URL.Path, "/SHA256SUMS.txt"):
					w.Write(release.sums)
				default:
					content, ok := release.binaries[path.Base(r.URL.Path)]
					if !ok {
						http.NotFound(w, r)
						return
					}
					w.Write(content)
				}
			}))
			defer server.Close()

			built, err := buildPublishedChannelRelease(channel.name, channel.tag, server.URL+"/releases/download/"+channel.tag+"/")
			if err != nil {
				t.Fatalf("assembling the stand-in %s release: %v", channel.name, err)
			}
			release = built

			script := strings.ReplaceAll(string(source), "https://github.com/paulmooreparks/dinah", server.URL)
			if !strings.Contains(script, server.URL) {
				t.Fatal("the test server URL did not reach the script under test")
			}
			script = stubbedPlatform(t, script, "Linux", "x86_64", false)

			home := t.TempDir()
			scriptDir := t.TempDir()
			scriptPath := filepath.Join(scriptDir, "install.sh")
			if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
				t.Fatalf("writing the script under test: %v", err)
			}

			command := exec.Command("sh", scriptPath)
			command.Env = append(os.Environ(), "HOME="+home, "DINAH_CHANNEL="+channel.name)
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("scripts/install.sh failed on the %s channel: %v\n%s", channel.name, err, output)
			}

			installed, err := os.ReadFile(filepath.Join(home, ".local", "bin", "dinah"))
			if err != nil {
				t.Fatalf("no binary was installed from the %s channel: %v\n%s", channel.name, err, output)
			}
			if !bytes.Equal(installed, release.binaries[wanted]) {
				t.Errorf("the %s channel installed something other than the published %s", channel.name, wanted)
			}

			// The manifest really was fetched, and it was the channel's own.
			// Without this the test would pass against a script that never
			// read DINAH_CHANNEL at all, if some other path happened to serve
			// it a usable answer.
			if len(served) == 0 {
				t.Fatalf("the script fetched no channel manifest at all")
			}
			for _, requested := range served {
				if !strings.HasSuffix(requested, "/channels/"+channel.name+".json") {
					t.Errorf("with DINAH_CHANNEL=%s the script fetched %s", channel.name, requested)
				}
			}
		})
	}
}
