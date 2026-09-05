// Reads the two package.json blobs a workflow extracted and reports whether
// the version changed.
//
// The comparison lives in version-diff.mjs, where a unit test drives every
// branch of it. This file is the git-facing wrapper that version-diff.mjs is
// deliberately not: it reads two paths and writes two workflow outputs, which
// is the same division of labour version.mjs already draws between the tested
// unpublishedVersion and the two-line callers that supply it a repository
// root.
//
// Usage: node scripts/check-version-change.mjs <after-path> [before-path]
//
// The before path is optional and an empty string means the same as omitting
// it, because a workflow passing a shell variable finds an empty string easier
// to produce than an absent argument. Outputs go to $GITHUB_OUTPUT when the
// workflow set it, and to stdout otherwise, so a person can run this by hand
// and read the answer.

import { appendFileSync, readFileSync } from "node:fs";

import { versionDiff } from "./version-diff.mjs";

const [afterPath, beforePath] = process.argv.slice(2);

if (afterPath === undefined || afterPath === "") {
	process.stderr.write(
		"usage: check-version-change.mjs <after-path> [before-path]\n",
	);
	process.exit(1);
}

const after = readFileSync(afterPath, "utf8");
const before =
	beforePath === undefined || beforePath === ""
		? undefined
		: readFileSync(beforePath, "utf8");

const { changed, version, previous } = versionDiff({ before, after });

process.stdout.write(
	`version was ${previous ?? "(nothing to compare against)"}, is now ${version}, changed=${String(changed)}\n`,
);

const outputs = `changed=${String(changed)}\nversion=${version}\n`;
const outputFile = process.env.GITHUB_OUTPUT;
if (outputFile !== undefined && outputFile !== "") {
	appendFileSync(outputFile, outputs);
} else {
	process.stdout.write(outputs);
}
