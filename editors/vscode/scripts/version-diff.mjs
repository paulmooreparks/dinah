// Whether a push changed the version the extension commits to.
//
// package.json carries the manifest, the command list, the configuration
// schema and the npm scripts, so it changes for many reasons that are not a
// version bump. A release workflow triggered by the file's path alone would
// cut a release for a renamed command or an edited description, and the
// marketplace refuses a version it already carries, so the run would fail on
// its last step having done everything up to it. The comparison below reads
// the version field at two points and answers the narrower question the
// trigger needs.
//
// The two readings arrive as file contents rather than as paths, so a unit
// test drives every branch without a checkout, a temporary file or a process.
// scripts/check-version-change.mjs reads the two blobs out of git and calls
// this function with their contents.

/**
 * The version a reading of package.json carries, or undefined when it carries
 * none that can be read.
 *
 * @param {string|undefined} text
 * @returns {string|undefined}
 */
function readVersion(text) {
	if (text === undefined) {
		return undefined;
	}
	let manifest;
	try {
		manifest = JSON.parse(text);
	} catch {
		return undefined;
	}
	const version = manifest?.version;
	return typeof version === "string" && version !== "" ? version : undefined;
}

/**
 * Whether the extension's committed version changed between two readings of
 * package.json.
 *
 * `after` has to parse as JSON and carry a non-empty string version. A reading
 * that fails either test throws rather than reporting no change, because
 * package.mjs depends on the same field and a manifest it cannot read should
 * stop the run loudly. A silent "no change" would look exactly like the
 * ordinary case where a push edited something else in the file.
 *
 * `before` is the earlier reading and it is allowed to be absent. An absent,
 * unparseable or version-less earlier reading means there is no prior version
 * to match, so the answer is that the version changed. No push to this
 * repository can produce that case today, because main has carried
 * package.json for hundreds of commits, but the function has an answer for it
 * rather than a crash.
 *
 * @param {{before?: string, after: string}} options - raw package.json text.
 * @returns {{changed: boolean, version: string, previous: string|undefined}}
 */
export function versionDiff({ before, after }) {
	let manifest;
	try {
		manifest = JSON.parse(after);
	} catch (err) {
		throw new Error(`package.json does not parse as JSON: ${String(err)}`);
	}
	const version = manifest?.version;
	if (typeof version !== "string" || version === "") {
		throw new Error(
			"package.json carries no version field, so there is nothing to compare against",
		);
	}
	const previous = readVersion(before);
	return { changed: previous !== version, version, previous };
}
