// Prints the version the next extension release carries.
//
// Usage: node scripts/print-next-version.mjs <package.json path> <tags file, or - for stdin>
//
// The arithmetic lives in release-version.mjs, where a unit test drives every
// branch of it. This file is the file-facing wrapper release-version.mjs is
// not: it reads a manifest and a tag list off disk and writes one line to
// stdout. That is the same division of labour version.mjs already draws
// between a tested function and the caller that supplies it a path.

import { readFileSync } from "node:fs";

import { nextReleaseVersion, parseTagList } from "./release-version.mjs";

const [manifestPath, tagsPath] = process.argv.slice(2);

if (manifestPath === undefined || manifestPath === "" || tagsPath === undefined || tagsPath === "") {
	process.stderr.write(
		"usage: print-next-version.mjs <package.json path> <tags file, or - for stdin>\n",
	);
	process.exit(1);
}

const manifest = JSON.parse(readFileSync(manifestPath, "utf8"));
const tags = parseTagList(readFileSync(tagsPath === "-" ? 0 : tagsPath, "utf8"));

process.stdout.write(`${nextReleaseVersion(tags, manifest.version)}\n`);
