// Reads the packaged archive and asserts it carries the licence text and the
// marketplace icon, no binary at all, no claim to be a pre-release, and every
// composed operator-attention icon with the notice attributing them.
//
// This script catches five failures, and none of them has another witness. An
// archive can ship with no statement of the terms it is distributed under,
// which is what dinah-371 found. An archive can ship without the icon its
// manifest promises, which dinah-372 added. A binary left in the extension
// tree by a bad local build is packaged like any other file, which would ship
// a copy of dinah inside an extension that says it carries none. An archive
// can carry vsce's pre-release property, which VS Code reads to decide that
// the archive has no release version and refuses to install it as one, which
// is what dinah-405 found in the 1.0.0 release. And, since dinah-599, an
// archive can be missing an attention icon or the notice attributing the
// drawings it composes them from: the icon draws as a broken image at the
// moment the operator most needs it, and the notice's absence is a licence
// term unmet, and neither shows up in a build log. None of the five shows up
// there, and the person who finds out is a user with no licence, every
// visitor to the marketplace listing, a user driving a build of dinah they
// never installed, a user who downloaded a release and could not install it,
// or the operator looking at a broken icon on his own attention indicator.

import { existsSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

import { UNIVERSAL } from "./targets.mjs";
import { listZipEntries, readZipEntry } from "./zip.mjs";

const here = dirname(fileURLToPath(import.meta.url));
const outDir = join(here, "..", "vsix");

// The attention glyph ids this build composes, read from the same file the
// generator reads, so this assertion and the generator cannot drift apart.
export const ATTENTION_GLYPH_IDS = JSON.parse(
	readFileSync(join(here, "attention-glyphs.json"), "utf8"),
);

// Where the attention icons and their notice land inside the archive.
export const ATTENTION_NOTICE_ENTRY = "extension/media/attention/NOTICE.md";
export const attentionIconEntries = ATTENTION_GLYPH_IDS.flatMap((id) => [
	`extension/media/attention/${id}-light.svg`,
	`extension/media/attention/${id}-dark.svg`,
]);

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

/**
 * The problems one archive's already-listed entries raise about the
 * operator-attention icons: a missing notice, and any of the icon ids
 * `attentionIconEntries` names that the archive does not carry.
 *
 * Pure over the entry list rather than reading the archive itself, so a unit
 * test drives it over a plain array without building a zip (dinah-599
 * criteria/12), and `check` below calls it over one real read of the archive
 * it already made.
 */
export function attentionProblems(label, entries, glyphEntries = attentionIconEntries) {
	const found = [];
	if (!entries.includes(ATTENTION_NOTICE_ENTRY)) {
		found.push(
			`${label}: does not carry ${ATTENTION_NOTICE_ENTRY}, so the composed icons ship with no attribution for the drawings they come from`,
		);
	}
	const missingIcons = glyphEntries.filter((entry) => !entries.includes(entry));
	if (missingIcons.length > 0) {
		found.push(
			`${label}: is missing ${String(missingIcons.length)} attention icon(s) (${missingIcons.join(", ")}), which draws as a broken image the moment the operator most needs it`,
		);
	}
	return found;
}

function check(label, vsix) {
	if (!existsSync(vsix)) {
		problems.push(`${label}: ${vsix} was not produced`);
		return;
	}
	const entries = listZipEntries(vsix);
	if (!entries.includes(LICENSE_ENTRY)) {
		problems.push(
			`${label}: does not carry ${LICENSE_ENTRY}, so whoever downloads it gets no statement of the terms`,
		);
	}
	if (!entries.includes(ICON_ENTRY)) {
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
	problems.push(...attentionProblems(label, entries));
}

function main() {
	check(UNIVERSAL, join(outDir, "dinah-universal.vsix"));

	if (problems.length > 0) {
		for (const problem of problems) {
			process.stderr.write(`${problem}\n`);
		}
		process.exit(1);
	}

	process.stdout.write(`Checked the ${UNIVERSAL} archive.\n`);
}

// The check runs only when this file is the program, so a test importing
// attentionProblems does not check a real archive and does not exit the
// process, on the same terms scripts/package.mjs already guards its own main.
if (process.argv[1] !== undefined && pathToFileURL(process.argv[1]).href === import.meta.url) {
	main();
}
