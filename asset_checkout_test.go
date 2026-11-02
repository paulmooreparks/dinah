package buildpolicy

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"studio/internal/skins"
)

func TestBundledSkinBytesSurviveWindowsCheckout(t *testing.T) {
	attributes, err := os.ReadFile(".gitattributes")
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"cmd/dinah-desktop/frontend/pudl", "internal/web/static/pudl"} {
		t.Run(host, func(t *testing.T) {
			dir := t.TempDir()
			git := func(args ...string) []byte {
				t.Helper()
				cmd := exec.Command("git", args...)
				cmd.Dir = dir
				cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+filepath.Join(dir, "absent-config"))
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("git %v: %v: %s", args, err, out)
				}
				return out
			}
			git("init", "--quiet")
			git("config", "core.autocrlf", "true")
			git("config", "core.safecrlf", "false")
			if err := os.WriteFile(filepath.Join(dir, ".gitattributes"), attributes, 0600); err != nil {
				t.Fatal(err)
			}
			catalogueBytes, err := os.ReadFile(filepath.Join(host, "pudl-skins.json"))
			if err != nil {
				t.Fatal(err)
			}
			catalogue, err := skins.Parse(catalogueBytes)
			if err != nil {
				t.Fatal(err)
			}
			// Check the build's actual inputs as well as the isolated checkout.
			if err := catalogue.ValidateAssets(os.DirFS(host)); err != nil {
				t.Fatal(err)
			}
			inputs := map[string][]byte{"pudl-skins.json": catalogueBytes, "release-crlf-fixture.txt": []byte("release bytes\r\nsecond line\r\n")}
			sourceName := "cmd/dinah-desktop/shell_server.go"
			source, err := os.ReadFile(sourceName)
			if err != nil {
				t.Fatal(err)
			}
			sourceTarget := filepath.Join(dir, sourceName)
			if err := os.MkdirAll(filepath.Dir(sourceTarget), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(sourceTarget, source, 0600); err != nil {
				t.Fatal(err)
			}
			for _, skin := range catalogue.Admitted() {
				if skin.Stylesheet == nil {
					continue
				}
				data, err := os.ReadFile(filepath.Join(host, *skin.Stylesheet))
				if err != nil {
					t.Fatal(err)
				}
				inputs[*skin.Stylesheet] = data
			}
			target := filepath.Join(dir, host)
			if err := os.MkdirAll(target, 0700); err != nil {
				t.Fatal(err)
			}
			for name, data := range inputs {
				if err := os.WriteFile(filepath.Join(target, name), data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			git("add", ".")
			for name := range inputs {
				if err := os.Remove(filepath.Join(target, name)); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Remove(sourceTarget); err != nil {
				t.Fatal(err)
			}
			git("checkout-index", "--all", "--force")
			// This is the source extraction used by the html-safety suite.
			checkedSource, err := os.ReadFile(sourceTarget)
			if err != nil {
				t.Fatal(err)
			}
			if !regexp.MustCompile(`const shellPolicy = ([\s\S]*?)\n\n`).Match(checkedSource) {
				t.Error("checked-out source cannot supply the html-safety policy")
			}
			if bytes.Contains(checkedSource, []byte("\r\n")) {
				t.Error("source checkout contains CRLF")
			}
			for name, want := range inputs {
				got, err := os.ReadFile(filepath.Join(target, name))
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(got, want) {
					t.Errorf("checkout changed %s/%s", host, name)
				}
			}
			if err := catalogue.ValidateAssets(os.DirFS(target)); err != nil {
				t.Fatal(err)
			}
		})
	}
}
