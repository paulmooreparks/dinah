package extensions

import (
	"context"
	"errors"
	extension "github.com/paulmooreparks/dinah-extension-go"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// PrepareSource takes a private snapshot before showing a trust decision.
// Git sources must contain a signed, already-built package. No build runs.
func PrepareSource(ctx context.Context, source string) (string, func(), error) {
	stage, err := os.MkdirTemp("", "dinah-extension-source-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { os.RemoveAll(stage) }
	fail := func(err error) (string, func(), error) { cleanup(); return "", nil, err }
	target := filepath.Join(stage, "package")
	if u, e := url.Parse(source); e == nil && (u.Scheme == "https" || u.Scheme == "ssh") && u.Host != "" {
		if u.Scheme == "https" && u.User != nil {
			return fail(errors.New("do not put credentials in an installation URL"))
		}
		ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		checkout := filepath.Join(stage, "checkout")
		cmd := exec.CommandContext(ctx, "git", "-c", "protocol.ext.allow=never", "-c", "protocol.file.allow=never", "clone", "--depth=1", "--", source, checkout)
		hideProcess(cmd)
		output := &limitedBuffer{limit: 64 << 10}
		cmd.Stdout, cmd.Stderr = output, output
		if err = cmd.Run(); err != nil {
			return fail(errors.New("could not clone the extension package repository: " + output.String()))
		}
		root := checkout
		if _, err = os.Stat(filepath.Join(root, extension.ManifestFile)); err != nil {
			root = filepath.Join(checkout, "dist", "extension")
		}
		manifest, err := extension.ReadManifest(root)
		if err != nil {
			return fail(err)
		}
		if len(manifest.Files) > extension.MaxFiles {
			return fail(errors.New("repository package contains too many files"))
		}
		if err = os.MkdirAll(target, 0700); err != nil {
			return fail(err)
		}
		names := []string{extension.ManifestFile}
		for p := range manifest.Files {
			names = append(names, p)
		}
		var total int
		for _, p := range names {
			if !extension.SafePath(p) {
				return fail(errors.New("unsafe package inventory path"))
			}
			src := filepath.Join(root, filepath.FromSlash(p))
			actual, e := filepath.EvalSymlinks(src)
			if e != nil || actual != src {
				return fail(errors.New("repository payload must not contain symlinks"))
			}
			f, e := os.Open(src)
			if e != nil {
				return fail(e)
			}
			data, e := io.ReadAll(io.LimitReader(f, 128<<20+1))
			f.Close()
			if e != nil || len(data) > 128<<20 {
				return fail(errors.New("repository file exceeds limit"))
			}
			total += len(data)
			if total > extension.MaxPackageBytes {
				return fail(errors.New("repository package exceeds size limit"))
			}
			dest := filepath.Join(target, filepath.FromSlash(p))
			if err = os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
				return fail(err)
			}
			if err = os.WriteFile(dest, data, 0644); err != nil {
				return fail(err)
			}
		}
	} else {
		info, e := os.Stat(source)
		if e != nil {
			return fail(e)
		}
		if info.IsDir() {
			if err = extension.CopyPackage(source, target); err != nil {
				return fail(err)
			}
		} else if strings.EqualFold(filepath.Ext(source), ".zip") {
			if err = extension.Unzip(source, target); err != nil {
				return fail(err)
			}
		} else {
			return fail(errors.New("choose a signed extension folder, ZIP or HTTPS/SSH Git URL"))
		}
	}
	if _, err = extension.Verify(target); err != nil {
		return fail(err)
	}
	return target, cleanup, nil
}
