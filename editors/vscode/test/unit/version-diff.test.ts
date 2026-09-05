// The question the extension's release trigger asks, which is narrower than
// the one its paths filter answers.
//
// editors/vscode/package.json changes for many reasons that are not a version
// bump, and a release cut for one of them is a release the marketplace refuses
// because it already carries that version. So the workflow reads the version
// field at both ends of the push and compares them, and versionDiff is that
// comparison. The cases below are the four the workflow can actually meet: an
// ordinary push that left the version alone, a bump, a push that created the
// ref and has nothing earlier to compare against, and an earlier reading that
// cannot be read.
//
// The last of those is the one worth stating plainly. An unreadable earlier
// reading reports a change rather than no change, because the alternative is a
// version bump that silently ships nothing. An unreadable later reading throws
// instead, because that manifest is the one package.mjs is about to package.

import assert from "node:assert/strict";
import { join } from "node:path";
import { pathToFileURL } from "node:url";
import { test } from "node:test";

const extensionRoot = join(__dirname, "..", "..", "..");

/** The version-diff module, imported as ESM from a CommonJS test. */
async function versionDiffModule(): Promise<{
	versionDiff: (options: { before?: string; after: string }) => {
		changed: boolean;
		version: string;
		previous: string | undefined;
	};
}> {
	return (await import(
		pathToFileURL(join(extensionRoot, "scripts", "version-diff.mjs")).href
	)) as never;
}

/** A package.json body carrying the given version and the fields around it. */
function manifest(version: string): string {
	return JSON.stringify({
		name: "dinah",
		version,
		contributes: { commands: [{ command: "dinah.refresh" }] },
	});
}

test("a push that left the version alone reports no change", async () => {
	const { versionDiff } = await versionDiffModule();
	// The two readings differ in a field that is not the version, which is
	// exactly the push this trigger has to ignore: a renamed command, an
	// edited configuration description, a lockfile-driven bump of the npm
	// scripts. A comparison that answered on the file rather than the field
	// would call this one a release.
	const before = JSON.stringify({ name: "dinah", version: "1.0.0", scripts: { lint: "eslint src" } });
	const after = JSON.stringify({ name: "dinah", version: "1.0.0", scripts: { lint: "eslint src test" } });
	assert.deepEqual(versionDiff({ before, after }), {
		changed: false,
		version: "1.0.0",
		previous: "1.0.0",
	});
});

test("a bumped version reports a change and names both numbers", async () => {
	const { versionDiff } = await versionDiffModule();
	assert.deepEqual(versionDiff({ before: manifest("1.0.0"), after: manifest("1.1.0") }), {
		changed: true,
		version: "1.1.0",
		previous: "1.0.0",
	});
});

test("no earlier reading at all reports a change", async () => {
	const { versionDiff } = await versionDiffModule();
	// The workflow passes no earlier reading when github.event.created says
	// the push created the ref, which is a push with no earlier state on that
	// ref to compare against.
	assert.deepEqual(versionDiff({ after: manifest("1.0.0") }), {
		changed: true,
		version: "1.0.0",
		previous: undefined,
	});
});

test("an earlier reading nothing can read reports a change", async () => {
	const { versionDiff } = await versionDiffModule();
	for (const before of [
		"this is not JSON at all",
		JSON.stringify({ name: "dinah" }),
		JSON.stringify({ name: "dinah", version: "" }),
		JSON.stringify({ name: "dinah", version: 100 }),
	]) {
		assert.deepEqual(
			versionDiff({ before, after: manifest("1.0.0") }),
			{ changed: true, version: "1.0.0", previous: undefined },
			`an earlier reading of ${before} was read as a version to match`,
		);
	}
});

test("a later reading nothing can read stops the run", async () => {
	const { versionDiff } = await versionDiffModule();
	// This manifest is the one package.mjs is about to package, so a run that
	// cannot read it has to fail rather than report that nothing changed. The
	// two failures look identical from the workflow's outputs otherwise.
	assert.throws(
		() => versionDiff({ before: manifest("1.0.0"), after: "{ not json" }),
		/does not parse as JSON/,
		"an unparseable manifest produced an answer instead of an error",
	);
	for (const after of [
		JSON.stringify({ name: "dinah" }),
		JSON.stringify({ name: "dinah", version: "" }),
		JSON.stringify({ name: "dinah", version: 100 }),
	]) {
		assert.throws(
			() => versionDiff({ before: manifest("1.0.0"), after }),
			/carries no version field/,
			`a manifest of ${after} produced an answer instead of an error`,
		);
	}
});
