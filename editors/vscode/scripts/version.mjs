// What version an archive of this extension carries.
//
// The extension's number is its own. It used to be derived from whichever
// dinah release tag a build was cut alongside, which published a dev build as
// 0.1.42 and then published the stable release v0.1.0 as 0.1.0, a number the
// marketplace reads as older and never offers as an update. Nothing computes
// one number from the other any more: package.json carries the extension's
// published version, bumped by hand on the extension's own cadence, exactly
// the way VERSION carries the CLI's.
//
// An unpublished archive is the one case that still needs a computed number,
// and it needs two properties. It must sort below every published version, so
// that a local or CI build installed by hand is never mistaken for a release
// and never suppresses one. And it must differ from build to build, because
// two archives sharing a version is what once made an install hang for minutes
// and then fail without saying why.
//
// The 0.0.x line is reserved for unpublished archives and the published line
// starts at 1.0.0, which is what buys the first property. Semver compares the
// major first, and every unpublished archive keeps a major of 0 while every
// release carries 1 or more, so no ordinal an unpublished build ever reaches
// can lift it above a release. The second property comes from the ordinal,
// which counts up within whichever place produced the build: GITHUB_RUN_NUMBER
// for continuous integration, which GitHub documents as increasing across a
// workflow's whole history, and the checkout's own commit count for a local
// build, which needs neither a network nor any CI plumbing to answer.

import { execFileSync } from "node:child_process";

/** The line reserved for archives that are not published releases. */
export const UNPUBLISHED_PREFIX = "0.0.";

/**
 * The checkout's commit count, which is the ordinal a local build uses.
 *
 * @param {string} repoRoot
 * @returns {string}
 */
export function commitCount(repoRoot) {
	return execFileSync("git", ["-C", repoRoot, "rev-list", "--count", "HEAD"], {
		encoding: "utf8",
	}).trim();
}

/**
 * The version an unpublished archive carries.
 *
 * The commit count arrives as a function so that a unit test can drive both
 * branches without starting a process, which the unit layer forbids.
 *
 * @param {{env?: NodeJS.ProcessEnv, repoRoot: string, count?: (repoRoot: string) => string}} options
 * @returns {string}
 */
export function unpublishedVersion({ env = process.env, repoRoot, count = commitCount }) {
	const run = env.GITHUB_RUN_NUMBER;
	if (run !== undefined && /^\d+$/.test(run.trim())) {
		return `${UNPUBLISHED_PREFIX}${run.trim()}`;
	}
	const ordinal = count(repoRoot);
	if (!/^\d+$/.test(ordinal)) {
		throw new Error(
			`git reported the commit count as ${JSON.stringify(ordinal)}, so no ordinal can be derived for this archive`,
		);
	}
	return `${UNPUBLISHED_PREFIX}${ordinal}`;
}

/**
 * Reports whether a version belongs to the unpublished line.
 *
 * @param {string} version
 * @returns {boolean}
 */
export function isUnpublishedVersion(version) {
	return /^0\.0\.\d+$/.test(version);
}
