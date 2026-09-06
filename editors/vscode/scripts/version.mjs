// What version an archive of this extension carries, and whether the lockfile
// still agrees with the manifest about it.
//
// The extension's number is its own. It used to be derived from whichever
// dinah release tag a build was cut alongside, which published a dev build as
// 0.1.42 and then published the stable release v0.1.0 as 0.1.0, a number the
// marketplace reads as older and never offers as an update. Nothing computes
// one number from the other any more.
//
// This file used to mint a version too. Local and CI archives were numbered on
// a reserved 0.0.x line so that an archive somebody installed by hand could
// never outrank a release, and that machinery is gone: every release now takes
// the next patch on its own line, permanently reserved as a tag before anything
// is packaged, so there is no second numbering space left for a release to
// collide with. scripts/release-version.mjs is where the computation lives now.
// A CI sanity archive carries the manifest's committed floor, goes nowhere, and
// is installed as nobody's update path.
//
// What is left here is the one check that was never about minting a number.

/** Where package-lock.json records the extension's own version. */
const LOCKFILE_VERSION_SITES = [
	{ where: "version", read: (lock) => lock.version },
	{ where: 'packages[""].version', read: (lock) => lock.packages?.[""]?.version },
];

/**
 * Reports every way package-lock.json's copy of the version fails to mirror
 * package.json's.
 *
 * package.json is the authoritative number, for the reasons the header above
 * gives. npm keeps its own copy in the lockfile and rewrites that copy from
 * the manifest on any install, so a lockfile that disagrees hands whoever
 * builds the extension a modified file they did not edit.
 *
 * A site the lockfile does not carry at all is reported rather than skipped,
 * because a check that passes for having found nothing to compare would go on
 * passing after npm moved where it keeps the number.
 *
 * @param {{version?: string}} manifest
 * @param {{version?: string, packages?: Record<string, {version?: string}>}} lock
 * @returns {string[]}
 */
export function lockfileVersionDrift(manifest, lock) {
	const authoritative = manifest.version;
	if (typeof authoritative !== "string" || authoritative === "") {
		return ["package.json carries no version, so the lockfile has nothing to mirror"];
	}
	const problems = [];
	for (const site of LOCKFILE_VERSION_SITES) {
		const found = site.read(lock);
		if (found === undefined) {
			problems.push(
				`package-lock.json has no ${site.where}, so nothing there mirrors package.json's ${authoritative}`,
			);
		} else if (found !== authoritative) {
			problems.push(
				`package-lock.json ${site.where} is ${found} and package.json is ${authoritative}, so the next npm install rewrites one of them`,
			);
		}
	}
	return problems;
}
