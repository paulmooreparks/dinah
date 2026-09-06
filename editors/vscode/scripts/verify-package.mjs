// Reads the packaged archive and asserts it carries the licence text and the
// marketplace icon, no binary at all, and no claim to be a pre-release.
//
// This script catches four failures, and none of them has another witness. An
// archive can ship with no statement of the terms it is distributed under,
// which is what dinah-371 found. An archive can ship without the icon its
// manifest promises, which dinah-372 added. A binary left in the extension
// tree by a bad local build is packaged like any other file, which would ship
// a copy of dinah inside an extension that says it carries none. And an
// archive can carry vsce's pre-release property, which VS Code reads to decide
// that the archive has no release version and refuses to install it as one,
// which is what dinah-405 found in the 1.0.0 release. None of the four shows
// up in a build log, and the person who finds out is a user with no licence,
// every visitor to the marketplace listing, a user driving a build of dinah
// they never installed, or a user who downloaded a release and could not
// install it.

import { existsSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

import { UNIVERSAL } from "./targets.mjs";
import { listZipEntries, readZipEntry } from "./zip.mjs";

const here = dirname(fileURLToPath(import.meta.url));
const outDir = join(here, "..", "vsix");

const problems = [];

// Where the licence text lands inside the archive.
//
// The source file is editors/vscode/LICENSE, with no extension, and this entry
// carries one because vsce's LicenseProcessor appends .txt to an extensionless
// licence file as it packages it. Checking for "extension/LICENSE" would
// therefore fail on every correct archive.
const LICENSE_ENTRY = "extension/LICENSE.txt";

// Where the marketplace icon lands inside the archive.
//
// package.json's top-level icon field is what the marketplace listing and the
// Extensions list both render, and it names a path inside the extension. An
// icon present in the source tree and absent from the archive publishes a grey
// placeholder, which is the same class of miss dinah-371 found for the licence.
// Unlike the licence, this path already carries an extension, so vsce has no
// reason to rename it; the assertion below reads the archive rather than the
// tree so that assumption is checked rather than trusted.
const ICON_ENTRY = "extension/media/icon.png";

// Where vsce records that an archive was packaged as a pre-release.
//
// vsce writes this property into the archive at packaging time rather than at
// publish time, and VS Code reads it to decide whether an archive has a release
// version at all. An archive carrying it can only be installed through Install
// Pre-Release, which is what a user downloading a GitHub release runs into.
// The property lives inside the content of the manifest entry rather than in
// its name, so this check decompresses the entry; every archive carries the
// entry itself, pre-release or not.
const MANIFEST_ENTRY = "extension.vsixmanifest";
const PRE_RELEASE_PROPERTY = 'Id="Microsoft.VisualStudio.Code.PreRelease"';

/** Every entry under `extension/bin/`, which must now be none. */
function carriedIn(vsix) {
	return listZipEntries(vsix)
		.filter((name) => name.startsWith("extension/bin/"))
		.map((name) => name.slice("extension/bin/".length))
		.filter((name) => name !== "");
}

/**
 * Every binary-looking entry that is NOT under extension/bin/.
 *
 * A staging directory left inside the extension root is packaged like any
 * other file, so an archive can carry a binary without it being under bin/.
 * Checking bin/ alone would call that archive correct while it shipped a copy
 * of dinah the extension promises not to carry.
 */
function strayIn(vsix) {
	return listZipEntries(vsix).filter(
		(name) => /(^|\/)dinah-[a-z0-9]+-[a-z0-9]+(\.exe)?$/.test(name) && !name.startsWith("extension/bin/"),
	);
}

function check(label, vsix) {
	if (!existsSync(vsix)) {
		problems.push(`${label}: ${vsix} was not produced`);
		return;
	}
	if (!listZipEntries(vsix).includes(LICENSE_ENTRY)) {
		problems.push(
			`${label}: does not carry ${LICENSE_ENTRY}, so whoever downloads it gets no statement of the terms`,
		);
	}
	if (!listZipEntries(vsix).includes(ICON_ENTRY)) {
		problems.push(
			`${label}: does not carry ${ICON_ENTRY}, so the marketplace listing and the Extensions list both show a placeholder`,
		);
	}
	if (readZipEntry(vsix, MANIFEST_ENTRY).toString("utf8").includes(PRE_RELEASE_PROPERTY)) {
		problems.push(
			`${label}: ${vsix} carries ${PRE_RELEASE_PROPERTY} in ${MANIFEST_ENTRY}, so VS Code refuses to install it as a release`,
		);
	}
	const stray = strayIn(vsix);
	if (stray.length > 0) {
		problems.push(
			`${label}: carries ${String(stray.length)} binary/binaries outside extension/bin/ (${stray.join(", ")})`,
		);
	}
	const carried = carriedIn(vsix);
	if (carried.length !== 0) {
		problems.push(
			`${label}: carries ${String(carried.length)} file(s) under extension/bin/ (${carried.join(", ")}), and the universal artifact must carry none`,
		);
	}
}

check(UNIVERSAL, join(outDir, "dinah-universal.vsix"));

if (problems.length > 0) {
	for (const problem of problems) {
		process.stderr.write(`${problem}\n`);
	}
	process.exit(1);
}

process.stdout.write(`Checked the ${UNIVERSAL} archive.\n`);
