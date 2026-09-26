#!/bin/sh
# Install Dinah on Linux or macOS, into ~/.local/bin, with no administrator
# privilege and no change to any shell startup file. It installs two programs
# from one release: dinah, and dinah-tui, the terminal UI's program, which
# dinah tui starts.
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
#
# The channel manifest says which release is current, and SHA256SUMS.txt
# inside that release says what the bytes should be. Reading the checksum from
# the checksum file rather than out of the manifest's JSON keeps this script
# working whatever layout the manifest is published in.

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
	echo "no Dinah build is published for $(uname -s) on $(uname -m); build both programs from source with: go build -o dinah ./cmd/dinah && go build -tags tui -o dinah-tui ./cmd/dinah" >&2
	exit 1
fi
binary="dinah-${goos}-${goarch}"
tui_binary="dinah-tui-${goos}-${goarch}"

# Step 2: confirm the install directory can be written before anything is
# fetched, so a permission problem is never mistaken for a failed download.
if ! mkdir -p "$install_dir" 2>/dev/null || [ ! -w "$install_dir" ]; then
	echo "cannot write to $install_dir; check that you have permission to write there" >&2
	exit 1
fi

# Step 3: read the channel manifest, which says which release is current. Only
# one value is taken from it, the download location, and whitespace carries no
# meaning between JSON tokens, so the document is flattened onto one line
# before it is read. No layout the publisher happens to emit, compact or
# pretty-printed, changes the answer.
if ! manifest="$(curl -fsSL "$manifest_url")"; then
	echo "could not fetch the release manifest from GitHub; check your network connection and try again" >&2
	exit 1
fi
flat_manifest="$(printf '%s' "$manifest" | tr '\n\r\t' '   ')"
download_base="$(printf '%s\n' "$flat_manifest" | sed -n 's/.*"downloadBase"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
if [ -z "$download_base" ]; then
	echo "the release manifest from GitHub named no download location, so there is nothing to fetch; the release may still be publishing, so try again in a few minutes" >&2
	exit 1
fi

# Step 4: read the checksums the downloads have to match, from the
# SHA256SUMS.txt published beside the binaries in that same release. sha256sum
# writes that file one line per binary, so a shell reads it a line and two
# fields at a time, the way the format was meant to be read. Nothing here digs
# a hash out of JSON, so reformatting the manifest cannot break the install,
# and this is also the file README tells you to check by hand.
if ! sums="$(curl -fsSL "${download_base}SHA256SUMS.txt")"; then
	echo "could not fetch SHA256SUMS.txt from the release; check your network connection and try again" >&2
	exit 1
fi

# published_sha prints the checksum SHA256SUMS.txt gives for the file it is
# named, or nothing when the release publishes no such file.
published_sha() {
	while read -r sum name; do
		# sha256sum marks a binary-mode line with a * before the name.
		name="${name#\*}"
		if [ "$name" = "$1" ]; then
			printf '%s\n' "$sum"
			return
		fi
	done <<SUMS
$(printf '%s' "$sums" | tr -d '\r')
SUMS
}

tmpfile=""
trap 'rm -f "$tmpfile"' EXIT

# install_program downloads the published file named $1, verifies it against
# its line of SHA256SUMS.txt, and installs it in the install directory as $2.
# $3 is what a failure leaves in place, which each failure message ends with.
install_program() {
	published="$1"
	installed="$2"
	left="$3"
	want_sha="$(published_sha "$published")"
	if [ -z "$want_sha" ]; then
		echo "the $channel channel publishes no $published, so there is no build for your machine to install; $left; build both programs from source with: go build -o dinah ./cmd/dinah && go build -tags tui -o dinah-tui ./cmd/dinah" >&2
		exit 1
	fi

	# Stage the download inside the install directory itself. mktemp's
	# template resolves to a name nothing in that directory already holds, so
	# the staging file cannot collide with a program from an earlier run.
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

	# Download. curl's -f turns a non-2xx response into a non-zero exit rather
	# than an error page saved as if it were a binary, and a dropped
	# connection exits non-zero as well.
	if ! curl -fsSL -o "$tmpfile" "${download_base}${published}"; then
		echo "download of $published did not complete (network error); $left, and it is safe to run this script again" >&2
		exit 1
	fi

	# Verify the bytes. Reaching here means the transfer finished, so a
	# mismatch is corruption or a manifest that no longer describes what is
	# being served, which is a different failure from the one above.
	if command -v sha256sum >/dev/null 2>&1; then
		got_sha="$(sha256sum "$tmpfile" | cut -d' ' -f1)"
	elif command -v shasum >/dev/null 2>&1; then
		got_sha="$(shasum -a 256 "$tmpfile" | cut -d' ' -f1)"
	else
		echo "this machine has neither sha256sum nor shasum, so the download cannot be verified; $left" >&2
		exit 1
	fi
	if [ "$got_sha" != "$want_sha" ]; then
		echo "downloaded file's checksum does not match the published checksum for $published; the download will not be installed, and $left" >&2
		exit 1
	fi

	# The one step that changes what sits at the install path, and it runs
	# only on a complete, verified download.
	chmod +x "$tmpfile"
	mv "$tmpfile" "$install_dir/$installed"
	echo "Installed $published as $install_dir/$installed"
}

# Steps 5 to 8: install dinah, then dinah-tui, the terminal UI's program,
# which dinah tui starts and looks for beside dinah first. The two come from
# the same release, which is what dinah tui checks. A dinah-tui that cannot be
# installed leaves the new dinah in place, and says that dinah tui waits on it.
install_program "$binary" dinah "nothing was installed"
install_program "$tui_binary" dinah-tui "dinah is installed and works, but dinah tui needs dinah-tui, so run this script again"

# Step 9: say whether dinah can be run right now, without editing a startup
# file this script did not write. The answer depends on this shell's PATH at
# this moment, not on the platform alone: ~/.local/bin is the documented
# location for a user's own executables on Linux, but a first install is not
# enough to reach it there either. Debian and Ubuntu add ~/.local/bin to PATH
# from the login profile only once the directory already exists, so the
# session that just created it is not the session that picks it up. macOS
# never adds it; its default PATH comes from /etc/paths and carries nothing
# under the home directory.
case ":${PATH}:" in
*":${install_dir}:"*)
	echo "You can run dinah now."
	echo "    dinah version"
	;;
*)
	echo ""
	echo "You installed dinah to $install_dir, but this shell does not have that directory on PATH yet."
	if [ "$goos" = "linux" ]; then
		echo "Debian and Ubuntu add ~/.local/bin to PATH automatically the next time a login shell starts, but only once the directory exists, and it did not exist before this install created it. Log out and back in to pick it up, or run this now to use dinah in this shell:"
	else
		echo "macOS does not add ~/.local/bin to PATH by default. Run this now to use dinah in this shell:"
	fi
	echo "    export PATH=\"\$HOME/.local/bin:\$PATH\""
	echo "Add that line to your shell's startup file (~/.profile, ~/.bashrc, ~/.zshrc, or wherever you keep it) to make it permanent."
	;;
esac
