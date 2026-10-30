#!/bin/sh
# Builds Dinah on a Mac from this repository and installs it for the person
# running it, with nothing installed beforehand but macOS itself. It needs no
# administrator rights; everything it installs is in the person's own folders:
#
#   ~/.dinah/toolchain  Go and the Wails command line, used only here
#   ~/Applications      Dinah.app, the window
#   ~/.dinah/bin        dinah, the command line, added to PATH in ~/.zprofile
#
# ~/.dinah/bin is the folder dinah install uses on Windows and Linux, so the
# command line is in the same place on every system.
#
# Run it again to update after the repository has changed; it reuses what it
# installed before.
#
# Usage, from anywhere:  sh tools/macos-build/install-macos.sh [--no-open]
set -eu

say() { printf '\n== %s\n' "$*"; }
fail() { printf '\nDinah was not built: %s\n' "$*" >&2; exit 1; }

[ "$(uname -s)" = Darwin ] || fail "this script builds the Mac app and runs only on a Mac."
repo="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$repo"
open_after=yes
[ "${1:-}" = "--no-open" ] && open_after=no

case "$(uname -m)" in
  arm64) arch=arm64 ;;
  x86_64) arch=amd64 ;;
  *) fail "this Mac's processor, $(uname -m), is not one Go is built for." ;;
esac

# Xcode's command line tools give the compiler, the macOS SDK and git. Their
# installer is a window macOS opens; the build goes on once it has finished.
say "Checking for Xcode's command line tools"
if ! xcode-select -p >/dev/null 2>&1; then
  xcode-select --install >/dev/null 2>&1 || true
  fail "macOS is now offering to install Xcode's command line tools. Click Install in the window it opened, wait until it says the software was installed, and run this again."
fi

# Go, the version go.mod asks for or a later release of the same line, from
# go.dev, checked against the checksum go.dev publishes for it.
toolchain="$HOME/.dinah/toolchain"
mkdir -p "$toolchain/bin"
want="$(sed -n 's/^go \([0-9]*\.[0-9]*\).*/\1/p' go.mod)"
[ -n "$want" ] || fail "go.mod does not say which Go it needs."
have_go() {
  [ -x "$toolchain/go/bin/go" ] || return 1
  v="$("$toolchain/go/bin/go" env GOVERSION 2>/dev/null)"
  case "$v" in go"$want"|go"$want".*) return 0 ;; esac
  return 1
}
if ! have_go; then
  say "Downloading Go $want for this Mac"
  index="$(mktemp)"
  curl -fsSL "https://go.dev/dl/?mode=json" -o "$index" || fail "go.dev could not be reached to download Go; check the connection."
  pick="$(/usr/bin/python3 - "$index" "$want" "$arch" <<'PY'
import json, sys
releases = json.load(open(sys.argv[1]))
want, arch = sys.argv[2], sys.argv[3]
for r in releases:
    if r["stable"] and (r["version"] == "go" + want or r["version"].startswith("go" + want + ".")):
        for f in r["files"]:
            if f["os"] == "darwin" and f["arch"] == arch and f["kind"] == "archive":
                print(f["filename"], f["sha256"])
                sys.exit(0)
sys.exit(1)
PY
)" || fail "go.dev lists no Go $want for this Mac."
  rm -f "$index"
  file="${pick% *}"; sum="${pick#* }"
  archive="$toolchain/$file"
  curl -fsSL "https://go.dev/dl/$file" -o "$archive" || fail "Go could not be downloaded from go.dev."
  echo "$sum  $archive" | shasum -a 256 -c - >/dev/null || { rm -f "$archive"; fail "the Go download did not match the checksum go.dev publishes, so it was not used."; }
  rm -rf "$toolchain/go"
  tar -xzf "$archive" -C "$toolchain"
  rm -f "$archive"
fi
export PATH="$toolchain/go/bin:$toolchain/bin:$PATH"
export GOPATH="$toolchain/gopath" GOBIN="$toolchain/bin" GOTOOLCHAIN=local
go version

# The Wails command line, at the version go.mod uses.
wails_version="$(sed -n 's/.*github.com\/wailsapp\/wails\/v2 \(v[0-9.]*\).*/\1/p' go.mod | head -1)"
[ -n "$wails_version" ] || fail "go.mod does not say which Wails it uses."
if ! "$toolchain/bin/wails" version 2>/dev/null | grep -q "${wails_version#v}"; then
  say "Installing the Wails command line $wails_version"
  go install "github.com/wailsapp/wails/v2/cmd/wails@$wails_version" || fail "the Wails command line could not be installed."
fi

# The version is the release tag the repository is at, such as 1.2.0, or
# that tag with how far past it the repository is.
version="$(git describe --tags --always 2>/dev/null | sed 's/^v//')"
[ -n "$version" ] || version=0.0.0-dev

say "Building Dinah $version"
sh tools/macos-build/build-macos.sh "$version" || fail "the app's build failed; the messages above say why."
say "Building dinah $version"
bin="$HOME/.dinah/bin"
mkdir -p "$bin"
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X studio/internal/mcpserver.Version=$version" -o "$bin/dinah.new" ./cmd/dinah || fail "the command line's build failed; the messages above say why."
mv -f "$bin/dinah.new" "$bin/dinah"

say "Installing"
app="$HOME/Applications/Dinah.app"
mkdir -p "$HOME/Applications"
if pgrep -f "Dinah.app/Contents/MacOS/" >/dev/null 2>&1; then
  fail "Dinah is running. Quit it (Dinah > Quit, or Cmd-Q) and run this again; the new build is ready and only needs installing."
fi
rm -rf "$app"
ditto "cmd/dinah-desktop/build/bin/Dinah.app" "$app"
echo "Dinah.app is in $HOME/Applications"
echo "dinah is in $bin"

# ~/.dinah/bin on PATH for new Terminal windows, once, in zsh's login file.
profile="$HOME/.zprofile"
if ! grep -q 'dinah: ~/.dinah/bin' "$profile" 2>/dev/null; then
  printf '\n# dinah: ~/.dinah/bin holds the dinah command line\nexport PATH="$HOME/.dinah/bin:$PATH"\n' >> "$profile"
  echo "Added ~/.dinah/bin to PATH in $profile; it applies to new Terminal windows."
fi

say "Done"
if [ "$open_after" = yes ]; then open "$app"; fi
