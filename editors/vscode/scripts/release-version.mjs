// Which version the next extension release carries, and which version the
// newest one already carries.
//
// The number is no longer typed by hand. package.json's version field is a
// floor rather than a release: it names a major.minor line and its patch
// component is always 0, and nobody ever ships an archive carrying the
// committed string as read. A human moves the major or the minor when a change
// earns it and resets the patch to 0 in the same edit. Every released patch
// lives in the tag and release history instead, which is the same division the
// CLI's own VERSION file draws when it carries only a major and a minor.
//
// The release history is therefore the only place that knows which numbers are
// taken, so the two questions below are both answered from a list of tag names.
// The next release is one past the highest patch already on the committed line,
// and the newest release is the highest version any vscode-v tag carries. Both
// comparisons are numeric field by field, because a lexical comparison puts
// vscode-v1.10.0 below vscode-v1.9.0 and a list order rests on an ordering the
// releases endpoint does not promise.
//
// Every function here is pure and takes the tag list as an argument, so a unit
// test drives each branch without a network, a process or a checkout. The two
// wrappers beside this file, print-next-version.mjs and print-newest-version.mjs,
// are what read a file and talk to a caller.

/** The tag prefix the extension's releases live under. */
const TAG_PREFIX = "vscode-v";

/** A released extension version, as the tag namespace spells it. */
const RELEASE_TAG = /^vscode-v(\d+)\.(\d+)\.(\d+)$/;

/** A committed floor: a major, a minor, and a patch that has to be zero. */
const FLOOR = /^(\d+)\.(\d+)\.(\d+)$/;

/**
 * The major.minor line a committed version names.
 *
 * The committed patch has to be 0. A nonzero one means somebody hand-typed a
 * release number into the manifest, which the scheme retired, so this throws
 * and names the version rather than quietly releasing on a line the manifest
 * did not mean.
 *
 * @param {string} committedVersion
 * @returns {string}
 */
export function releaseBase(committedVersion) {
	const match = FLOOR.exec(String(committedVersion));
	if (match === null) {
		throw new Error(
			`the committed version ${JSON.stringify(committedVersion)} is not a major.minor.patch string, so no release line can be read from it`,
		);
	}
	if (match[3] !== "0") {
		throw new Error(
			`the committed version ${JSON.stringify(committedVersion)} carries a patch of ${match[3]}, and package.json's version is a floor whose patch is always 0; the released patch is computed per merge and never written back to the manifest`,
		);
	}
	return `${match[1]}.${match[2]}`;
}

/**
 * The patch the next release on `base` takes.
 *
 * A line nothing has shipped on starts at 0, which is what lets a brand new
 * major.minor line cut its first release without anybody deciding a number.
 * Otherwise the answer is one past the highest patch already released on that
 * line, read only from tags whose own line is `base`, so a busy neighbouring
 * line cannot lend this one its count.
 *
 * @param {string[]} tags
 * @param {string} base - a "major.minor" line, as releaseBase returns.
 * @returns {number}
 */
export function nextPatch(tags, base) {
	const line = new RegExp(`^${TAG_PREFIX}${base.replace(/\./gu, "\\.")}\\.(\\d+)$`, "u");
	let highest;
	for (const tag of tags) {
		const match = line.exec(tag);
		if (match === null) {
			continue;
		}
		const patch = Number(match[1]);
		if (highest === undefined || patch > highest) {
			highest = patch;
		}
	}
	return highest === undefined ? 0 : highest + 1;
}

/**
 * The full version the next release carries.
 *
 * @param {string[]} tags
 * @param {string} committedVersion
 * @returns {string}
 */
export function nextReleaseVersion(tags, committedVersion) {
	const base = releaseBase(committedVersion);
	return `${base}.${String(nextPatch(tags, base))}`;
}

/**
 * The version of the newest release in `tags`, or undefined when the list
 * carries no extension release at all.
 *
 * Undefined is a real answer here rather than a failure. A list carrying only
 * CLI tags, and an empty list, both mean the extension has never been released,
 * which is the state this repository is actually in, and a caller that has to
 * tell that apart from a lookup which could not answer at all needs the two
 * spelled differently.
 *
 * @param {string[]} tags
 * @returns {string|undefined}
 */
export function newestReleaseVersion(tags) {
	let newest;
	for (const tag of tags) {
		const match = RELEASE_TAG.exec(tag);
		if (match === null) {
			continue;
		}
		const candidate = [Number(match[1]), Number(match[2]), Number(match[3])];
		if (newest === undefined || greater(candidate, newest)) {
			newest = candidate;
		}
	}
	return newest === undefined ? undefined : newest.join(".");
}

/**
 * Whether one major/minor/patch triple outranks another.
 *
 * @param {number[]} a
 * @param {number[]} b
 * @returns {boolean}
 */
function greater(a, b) {
	for (let i = 0; i < 3; i += 1) {
		if (a[i] !== b[i]) {
			return a[i] > b[i];
		}
	}
	return false;
}

/**
 * A tag list read out of a file or a stream, with blank lines dropped.
 *
 * The list arrives as whatever `gh api ... --jq '.[] | .tag_name'` wrote, which
 * is one name per line and a trailing newline. Carriage returns are stripped so
 * that a list captured on Windows reads the same as one captured on a runner.
 *
 * @param {string} text
 * @returns {string[]}
 */
export function parseTagList(text) {
	return text
		.split("\n")
		.map((line) => line.trim())
		.filter((line) => line !== "");
}
