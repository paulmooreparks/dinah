// Reads the packaged archives and asserts each one carries exactly the binary
// it is supposed to.
//
// Two failures this catches, both of which have no other witness: a
// .vscodeignore change that ships all six binaries in every artifact, and a
// staging step that leaves the previous target's binary behind. Neither shows
// up in a build log, and a user on the wrong platform is the one who finds
// out.

import { existsSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

import { PLATFORM_TARGETS, UNIVERSAL } from "./targets.mjs";
import { listZipEntries } from "./zip.mjs";

const here = dirname(fileURLToPath(import.meta.url));
const outDir = join(here, "..", "vsix");

const problems = [];

/** Every entry under `extension/bin/`, which is where a carried binary lands. */
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
 * other file, so an artifact can carry all six binaries without a single one
 * of them being under bin/. Checking bin/ alone would call that archive
 * correct while it was six times the size it should be.
 */
function strayIn(vsix) {
	return listZipEntries(vsix).filter(
		(name) => /(^|\/)dinah-[a-z0-9]+-[a-z0-9]+(\.exe)?$/.test(name) && !name.startsWith("extension/bin/"),
	);
}

function check(label, vsix, expected) {
	if (!existsSync(vsix)) {
		problems.push(`${label}: ${vsix} was not produced`);
		return;
	}
	const stray = strayIn(vsix);
	if (stray.length > 0) {
		problems.push(
			`${label}: carries ${String(stray.length)} binary/binaries outside extension/bin/ (${stray.join(", ")})`,
		);
	}
	const carried = carriedIn(vsix);
	if (expected === undefined) {
		if (carried.length !== 0) {
			problems.push(
				`${label}: carries ${String(carried.length)} file(s) under extension/bin/ (${carried.join(", ")}), and the universal artifact must carry none`,
			);
		}
		return;
	}
	if (carried.length !== 1) {
		problems.push(
			`${label}: carries ${String(carried.length)} file(s) under extension/bin/ (${carried.join(", ") || "none"}), and exactly one is required`,
		);
		return;
	}
	if (carried[0] !== expected) {
		problems.push(`${label}: carries ${carried[0]}, and this target needs ${expected}`);
	}
}

for (const { target, binary } of PLATFORM_TARGETS) {
	check(target, join(outDir, `dinah-${target}.vsix`), binary);
}
check(UNIVERSAL, join(outDir, "dinah-universal.vsix"), undefined);

if (problems.length > 0) {
	for (const problem of problems) {
		process.stderr.write(`${problem}\n`);
	}
	process.exit(1);
}

process.stdout.write(
	`Checked ${String(PLATFORM_TARGETS.length)} platform archives and the universal one.\n`,
);
