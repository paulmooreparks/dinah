#!/bin/sh
# Builds Dinah's window, dinah-desktop, for macOS as Dinah.app. It has to run
# on a Mac: the app links Apple's WebKit through Apple's SDK, which Apple
# licenses for use on Apple hardware only, so it is not cross-compiled from
# Windows the way the command line is.
#
# Needs: the Go toolchain (the version in go.mod or later), Xcode's command line
# tools (xcode-select --install), and the Wails command line at the version in
# go.mod (go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0).
# tools/macos-build/install-macos.sh sets all three up and then runs this.
#
# It uses Wails' own documented build (wails build), which makes the .app
# bundle with its Info.plist and icon from cmd/dinah-desktop/wails.json and
# build/appicon.png. The page is plain files, so there is no front-end build
# step, and the bindings are not regenerated. The bundle is then signed ad
# hoc (codesign -s -), which a Mac with Apple silicon needs to run a program
# and which is enough for an app built on the Mac it runs on.
#
# PLATFORM chooses what to build for: the Mac's own processor by default
# (darwin/arm64 or darwin/amd64), or darwin/universal for both in one bundle.
#
# Usage, from the top of the repository:  sh tools/macos-build/build-macos.sh [version]
# The app is written to cmd/dinah-desktop/build/bin/Dinah.app.
set -eu
version="${1:-0.0.0-dev}"
case "$(uname -m)" in
  arm64) native=darwin/arm64 ;;
  *) native=darwin/amd64 ;;
esac
platform="${PLATFORM:-$native}"
cd "$(dirname "$0")/../../cmd/dinah-desktop"
command -v wails >/dev/null || { echo "the macOS build needs the Wails command line; see the top of this script" >&2; exit 1; }
wails build -platform "$platform" -tags "desktop,production" -skipbindings -s -trimpath \
  -ldflags "-s -w -X main.appVersion=$version"
codesign --force --deep --sign - "build/bin/Dinah.app"
echo "built $(pwd)/build/bin/Dinah.app"
