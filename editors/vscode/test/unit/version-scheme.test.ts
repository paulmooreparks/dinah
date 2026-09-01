// The extension's version scheme, which stopped being a projection of the
// CLI's.
//
// Two properties are what the scheme exists for and are what this asserts. An
// unpublished archive sorts below every published version, so that a build
// somebody installs by hand is never mistaken for a release. And two
// unpublished archives never carry the same version, because two archives
// sharing one is what made an install hang for minutes and then fail without
// saying why.
//
// The third assertion is that the derivation is gone. A tag-to-version mapping
// that came back would be silent: it would produce a plausible number, and
// only a marketplace that refused an update would ever say so.

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { pathToFileURL } from "node:url";
import { test } from "node:test";

const extensionRoot = join(__dirname, "..", "..", "..");
const repoRoot = join(extensionRoot, "..", "..");

/** The version module, imported as ESM from a CommonJS test. */
async function versionModule(): Promise<{
	unpublishedVersion: (options: {
		env?: NodeJS.ProcessEnv;
		repoRoot: string;
		count?: (repoRoot: string) => string;
	}) => string;
	isUnpublishedVersion: (version: string) => boolean;
}> {
	return (await import(
		pathToFileURL(join(extensionRoot, "scripts", "version.mjs")).href
	)) as never;
}

/** Compares two versions the way semver orders them. */
function orderedBelow(lower: string, higher: string): boolean {
	const parse = (v: string) => v.split(".").map((part) => Number(part));
	const [a, b] = [parse(lower), parse(higher)];
	for (let i = 0; i < 3; i += 1) {
		if (a[i] !== b[i]) {
			return a[i] < b[i];
		}
	}
	return false;
}

test("a CI archive is numbered by the run number, and two runs differ", async () => {
	const { unpublishedVersion } = await versionModule();
	const first = unpublishedVersion({ env: { GITHUB_RUN_NUMBER: "17" }, repoRoot });
	const second = unpublishedVersion({ env: { GITHUB_RUN_NUMBER: "18" }, repoRoot });
	assert.equal(first, "0.0.17");
	assert.equal(second, "0.0.18");
	assert.notEqual(first, second);
});

test("a local archive is numbered by the checkout's commit count", async () => {
	const { unpublishedVersion } = await versionModule();
	// The count arrives injected rather than measured, because the unit layer
	// starts no processes. That the injected function is the one that asks git
	// for the count is the default in version.mjs, and the packaging run is
	// what exercises it.
	assert.equal(unpublishedVersion({ env: {}, repoRoot, count: () => "412" }), "0.0.412");
	assert.equal(unpublishedVersion({ env: {}, repoRoot, count: () => "413" }), "0.0.413");
	assert.throws(
		() => unpublishedVersion({ env: {}, repoRoot, count: () => "" }),
		/no ordinal can be derived/,
		"an unanswerable commit count produced a version anyway",
	);
});

test("every unpublished archive sorts below the published version", async () => {
	const { unpublishedVersion, isUnpublishedVersion } = await versionModule();
	const manifest = JSON.parse(
		readFileSync(join(extensionRoot, "package.json"), "utf8"),
	) as { version: string };

	assert.ok(
		!isUnpublishedVersion(manifest.version),
		`package.json carries ${manifest.version}, which is on the line reserved for unpublished archives`,
	);
	// A run number climbs without limit, so the ordering cannot rest on the
	// patch. It rests on the major, which is why the published line starts at
	// 1.0.0 and the unpublished line never leaves 0.0.
	for (const run of ["1", "999999"]) {
		const unpublished = unpublishedVersion({ env: { GITHUB_RUN_NUMBER: run }, repoRoot });
		assert.ok(isUnpublishedVersion(unpublished), `${unpublished} is not on the 0.0.x line`);
		assert.ok(
			orderedBelow(unpublished, manifest.version),
			`${unpublished} does not sort below the published ${manifest.version}`,
		);
	}
});

test("the README says the two numbers are unrelated and names what to read instead", () => {
	// Decoupling the two numbers took away the only by-inspection way to tell
	// whether an installed extension and an installed binary belong together.
	// A reader who meets the mismatch has to be told what replaces it, or the
	// README states a problem and offers no answer.
	const readme = readFileSync(join(extensionRoot, "README.md"), "utf8");
	assert.ok(
		/unrelated by design/.test(readme),
		"the README no longer says the extension's version and the CLI's are unrelated by design",
	);
	assert.ok(
		/profile revision/.test(readme) && /--json version/.test(readme),
		"the README no longer names the profile revision as what a reader checks instead",
	);
});

test("nothing derives the extension's version from a dinah release tag", () => {
	const esbuild = readFileSync(join(extensionRoot, "esbuild.mjs"), "utf8");
	assert.ok(
		!esbuild.includes("extensionVersion"),
		"esbuild.mjs still exports the tag-derived extension version",
	);
	assert.ok(
		!esbuild.includes("-dev\\."),
		"esbuild.mjs still matches the dev counter out of a release tag",
	);

	const publish = readFileSync(
		join(extensionRoot, "scripts", "publish-extension.ps1"),
		"utf8",
	);
	assert.ok(
		!publish.includes("-dev\\.(\\d+)$"),
		"publish-extension.ps1 still derives a version from the tag's dev counter",
	);
	assert.ok(
		publish.includes("--published"),
		"publish-extension.ps1 no longer tells the packaging step that these archives are the published ones",
	);
});
