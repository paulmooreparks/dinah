#!/bin/sh
# Install Dinah on Linux or macOS, into ~/.local/bin, with no administrator
# privilege and no change to any shell startup file.
#
# Set DINAH_CHANNEL to choose a channel. It defaults to dev.
#
#   curl -fsSL https://raw.githubusercontent.com/paulmooreparks/dinah/main/scripts/install.sh | sh
#
# The download is staged inside ~/.local/bin rather than in the system
# temporary directory, so the final move is a rename within one filesystem.
# /tmp is a separate filesystem on many machines, and a move across a
# filesystem boundary is a copy followed by a delete, which can leave a
# truncated binary at the install path if it is interrupted.

set -eu

channel="${DINAH_CHANNEL:-dev}"
install_dir="$HOME/.local/bin"
manifest_url="https://github.com/paulmooreparks/dinah/releases/download/channels/${channel}.json"

# Step 1: work out which binary this machine needs.
case "$(uname -s)" in
	Linux) goos=linux ;;
	Darwin) goos=darwin ;;
	*) goos="" ;;
esac
case "$(uname -m)" in
	x86_64 | amd64) goarch=amd64 ;;
	aarch64 | arm64) goarch=arm64 ;;
	*) goarch="" ;;
esac
if [ -z "$goos" ] || [ -z "$goarch" ]; then
	echo "no Dinah build is published for $(uname -s) on $(uname -m); build one from source with: go build -o dinah ./cmd/dinah" >&2
	exit 1
fi
binary="dinah-${goos}-${goarch}"

# Step 2: confirm the install directory can be written before anything is
# fetched, so a permission problem is never mistaken for a failed download.
if ! mkdir -p "$install_dir" 2>/dev/null || [ ! -w "$install_dir" ]; then
	echo "cannot write to $install_dir; check that you have permission to write there" >&2
	exit 1
fi

# Step 3: read the channel manifest.
if ! manifest="$(curl -fsSL "$manifest_url")"; then
	echo "could not fetch the release manifest from GitHub; check your network connection and try again" >&2
	exit 1
fi
download_base="$(printf '%s\n' "$manifest" | sed -n 's/.*"downloadBase"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
want_sha="$(printf '%s\n' "$manifest" | sed -n 's/.*"'"$binary"'"[[:space:]]*:[[:space:]]*{[[:space:]]*"sha256"[[:space:]]*:[[:space:]]*"\([0-9a-fA-F]*\)".*/\1/p')"
if [ -z "$download_base" ] || [ -z "$want_sha" ]; then
	echo "could not fetch the release manifest from GitHub; check your network connection and try again" >&2
	exit 1
fi

# Step 4: stage the download inside the install directory itself. mktemp's
# template resolves to a name nothing in that directory already holds, so the
# staging file cannot collide with a dinah from an earlier run.
tmpfile=""
if command -v mktemp >/dev/null 2>&1; then
	tmpfile="$(mktemp "$install_dir/.dinah.XXXXXX" 2>/dev/null || echo '')"
fi
if [ -z "$tmpfile" ]; then
	tmpfile="$install_dir/.dinah.$$.$(date +%s)"
	if ! : >"$tmpfile" 2>/dev/null; then
		echo "cannot write to $install_dir; check that you have permission to write there" >&2
		exit 1
	fi
fi
trap 'rm -f "$tmpfile"' EXIT

# Step 5: download. curl's -f turns a non-2xx response into a non-zero exit
# rather than an error page saved as if it were a binary, and a dropped
# connection exits non-zero as well.
if ! curl -fsSL -o "$tmpfile" "${download_base}${binary}"; then
	echo "download of $binary did not complete (network error); nothing was installed, and it is safe to run this script again" >&2
	exit 1
fi

# Step 6: verify the bytes. Reaching here means the transfer finished, so a
# mismatch is corruption or a manifest that no longer describes what is being
# served, which is a different failure from the one above.
if command -v sha256sum >/dev/null 2>&1; then
	got_sha="$(sha256sum "$tmpfile" | cut -d' ' -f1)"
elif command -v shasum >/dev/null 2>&1; then
	got_sha="$(shasum -a 256 "$tmpfile" | cut -d' ' -f1)"
else
	echo "this machine has neither sha256sum nor shasum, so the download cannot be verified; nothing was installed" >&2
	exit 1
fi
if [ "$got_sha" != "$want_sha" ]; then
	echo "downloaded file's checksum does not match the manifest for $binary; the download will not be installed" >&2
	exit 1
fi

# Step 7: the one step that changes what sits at the install path, and it runs
# only on a complete, verified download.
chmod +x "$tmpfile"
mv "$tmpfile" "$install_dir/dinah"
echo "Installed $binary as $install_dir/dinah"

# Step 8: say how to reach it, without editing a startup file this script did
# not write.
case ":${PATH}:" in
*":${install_dir}:"*) ;;
*)
	echo ""
	echo "$install_dir is not on your PATH. Add this line to your shell's startup file (~/.bashrc, ~/.zshrc, or the equivalent):"
	echo "    export PATH=\"\$HOME/.local/bin:\$PATH\""
	;;
esac
