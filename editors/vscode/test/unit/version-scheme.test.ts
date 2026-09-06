// The extension's version scheme, which is computed per release rather than
// typed into the manifest.
//
// package.json's version field is a floor. It names a major.minor line, its
// patch is always zero, and nothing ever ships the committed string as read.
// Every released patch is minted from the release history instead, so the
// assertions below are about that arithmetic and about the guard that keeps the
// floor a floor. A hand-typed patch in the manifest would otherwise release on
// a line the manifest did not mean, and it would do so silently.
//
// The scheme this replaced numbered local and CI archives on a reserved 0.0.x
// line so that a hand-installed archive could never outrank a release. That
// property is inherited rather than dropped: every release takes the next patch
// on its own line and reserves it as a tag before anything is packaged, so no
// second numbering space is left for a release to collide with.
//
// Two older assertions stay because nothing here reintroduces what they guard.
// A tag-to-version derivation that came back would be silent, producing a
// plausible number that only a marketplace refusing an update would report. And
// package-lock.json still has to carry the number package.json commits to,
// because npm rewrites its copy from the manifest on any install and a release
// build is a bad place to discover a file modified underneath you.

import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { pathToFileURL } from "node:url";
import { test } from "node:test";

const extensionRoot = join(__dirname, "..", "..", "..");

/** The version module, imported as ESM from a CommonJS test. */
async function versionModule(): Promise<Record<string, unknown>> {
	return (await import(
		pathToFileURL(join(extensionRoot, "scripts", "version.mjs")).href
	)) as never;
}

/** The release-version module, imported the same way. */
async function releaseVersionModule(): Promise<{
	releaseBase: (committedVersion: string) => string;
	nextPatch: (tags: string[], base: string) => number;
	nextReleaseVersion: (tags: string[], committedVersion: string) => string;
	newestReleaseVersion: (tags: string[]) => string | undefined;
}> {
	return (await import(
		pathToFileURL(join(extensionRoot, "scripts", "release-version.mjs")).href
	)) as never;
}

/** package.mjs, whose packaging body does not run on an import. */
async function packageModule(): Promise<{
	resolvePackageVersion: (options: { argv: string[]; manifestVersion: string }) => string;
}> {
	return (await import(
		pathToFileURL(join(extensionRoot, "scripts", "package.mjs")).href
	)) as never;
}

/** The committed manifest, read off disk. */
function committedManifest(): { version: string } {
	return JSON.parse(readFileSync(join(extensionRoot, "package.json"), "utf8")) as {
		version: string;
	};
}

test("the committed version is a floor whose patch is always zero", () => {
	// The whole scheme rests on this. releaseBase reads the major.minor line off
	// the committed string and refuses a nonzero patch, so a hand-typed release
	// number fails the release workflow rather than shipping. This is the same
	// guard read from the other end: the file itself.
	const { version } = committedManifest();
	assert.match(
		version,
		/^\d+\.\d+\.0$/u,
		`package.json carries ${version}, and the committed version is a floor whose patch component is always 0; the released patch is computed per merge and never written back`,
	);
});

test("the release line is read off the floor and a nonzero committed patch is refused", async () => {
	const { releaseBase } = await releaseVersionModule();
	assert.equal(releaseBase("1.2.0"), "1.2");
	assert.equal(releaseBase("0.7.0"), "0.7");
	assert.equal(releaseBase("12.30.0"), "12.30");
	assert.throws(
		() => releaseBase("1.2.3"),
		(err: Error) => /1\.2\.3/u.test(err.message) && /patch/u.test(err.message),
		"a committed version carrying a nonzero patch was accepted as a release line",
	);
	assert.throws(
		() => releaseBase("1.2"),
		/1\.2/u,
		"a committed version that is not major.minor.patch was accepted as a release line",
	);
});

test("the next patch is counted on its own line and nobody else's", async () => {
	const { nextPatch, nextReleaseVersion } = await releaseVersionModule();
	// The fixture mixes lines on purpose. An implementation matching "1.0" as a
	// prefix without anchoring the patch to the line reads 1.1.0 as a 1.0 tag,
	// and one that never filters the namespace reads the CLI's tags as its own.
	const tags = [
		"vscode-v1.0.0",
		"vscode-v1.0.1",
		"vscode-v1.1.0",
		"dinah-v1.0.5",
		"v1.0.9",
		"v0.1.126-dev",
	];
	assert.equal(nextPatch(tags, "1.0"), 2);
	assert.equal(nextPatch(tags, "1.1"), 1);
	assert.equal(nextPatch(tags, "2.0"), 0, "a line nothing has shipped on does not start at 0");
	assert.equal(nextPatch([], "1.0"), 0);
	// The highest wins rather than the last, and ten outranks nine numerically
	// where it loses lexically.
	assert.equal(nextPatch(["vscode-v1.0.10", "vscode-v1.0.9"], "1.0"), 11);
	assert.equal(nextReleaseVersion(tags, "1.0.0"), "1.0.2");
	assert.equal(nextReleaseVersion(tags, "2.0.0"), "2.0.0");
});

test("the newest release is the highest version rather than the last or the largest string", async () => {
	const { newestReleaseVersion } = await releaseVersionModule();
	assert.equal(newestReleaseVersion([]), undefined);
	assert.equal(
		newestReleaseVersion(["v1.2.3", "dinah-v1.0.5", "v0.1.0-beta"]),
		undefined,
		"a list carrying only CLI tags reported an extension release",
	);
	assert.equal(
		newestReleaseVersion(["vscode-v1.0.0", "vscode-v2.0.0", "vscode-v1.9.9"]),
		"2.0.0",
		"the newest release was read off the list's order rather than off the numbers",
	);
	assert.equal(
		newestReleaseVersion(["vscode-v1.9.0", "vscode-v1.10.0"]),
		"1.10.0",
		"the versions were compared as strings, where 1.9.0 outranks 1.10.0",
	);
	assert.equal(
		newestReleaseVersion(["vscode-v1.2.5", "v9.9.9", "vscode-v1.3.0", "vscode-v1.3.0-rc"]),
		"1.3.0",
		"a tag outside the vscode-v<major>.<minor>.<patch> shape was compared as if it were a release",
	);
});

test("the packaged version is given rather than chosen, and a bad one is refused", async () => {
	const { resolvePackageVersion } = await packageModule();
	assert.equal(
		resolvePackageVersion({ argv: [], manifestVersion: "1.0.0" }),
		"1.0.0",
		"packaging with no --version no longer carries the committed version",
	);
	// --published used to select the published path. It selects nothing now, so
	// an old caller still passing it gets the committed version exactly as a
	// caller passing no flags does.
	assert.equal(
		resolvePackageVersion({ argv: ["--published"], manifestVersion: "1.0.0" }),
		"1.0.0",
		"--published still steers the version choice",
	);
	assert.equal(resolvePackageVersion({ argv: ["--version", "1.4.7"], manifestVersion: "1.0.0" }), "1.4.7");
	assert.equal(resolvePackageVersion({ argv: ["--version=1.4.7"], manifestVersion: "1.0.0" }), "1.4.7");
	for (const bad of ["1.4", "v1.4.7", "1.4.7-dev", "", "not-a-version"]) {
		assert.throws(
			() => resolvePackageVersion({ argv: ["--version", bad], manifestVersion: "1.0.0" }),
			(err: Error) => err.message.includes(JSON.stringify(bad)),
			`--version ${JSON.stringify(bad)} was packaged instead of refused`,
		);
	}
	assert.throws(
		() => resolvePackageVersion({ argv: ["--version"], manifestVersion: "1.0.0" }),
		/--version/u,
		"--version with nothing after it packaged something",
	);
});

test("print-newest-version tells an absent release apart from a version", () => {
	// Both exit paths are driven for real, because the caller is a PowerShell
	// script that decides on the exit status. A wrapper that always exits zero,
	// or that reports the empty case as an ordinary version string, would leave
	// publish-extension.ps1 packaging a number nobody released.
	const script = join(extensionRoot, "scripts", "print-newest-version.mjs");
	const dir = mkdtempSync(join(tmpdir(), "dinah-newest-"));
	try {
		const none = join(dir, "none.txt");
		writeFileSync(none, "v1.2.3\ndinah-v1.0.5\n", "utf8");
		const absent = spawnSync(process.execPath, [script, none], { encoding: "utf8" });
		assert.notEqual(absent.status, 0, "an absent release exited zero");
		assert.match(
			absent.stderr,
			/no extension release exists yet/iu,
			"the absent-release path did not say that no extension release exists yet",
		);
		assert.equal(absent.stdout.trim(), "", "the absent-release path printed a version anyway");

		const found = spawnSync(process.execPath, [script, "-"], {
			encoding: "utf8",
			input: "vscode-v1.2.5\nvscode-v1.3.0\n",
		});
		assert.equal(found.status, 0, `reading a release list failed: ${found.stderr}`);
		assert.equal(found.stdout.trim(), "1.3.0");
	} finally {
		rmSync(dir, { recursive: true, force: true });
	}
});

test("the unpublished-ordinal machinery is gone and the lockfile check is not", async () => {
	const module = await versionModule();
	for (const name of [
		"UNPUBLISHED_PREFIX",
		"commitCount",
		"unpublishedVersion",
		"isUnpublishedVersion",
	]) {
		assert.equal(
			module[name],
			undefined,
			`version.mjs still exports ${name}, so the retired 0.0.x numbering is still reachable`,
		);
	}
	assert.equal(
		typeof module.lockfileVersionDrift,
		"function",
		"version.mjs no longer exports lockfileVersionDrift, which had nothing to do with the retired numbering",
	);
});

test("package-lock.json carries the version package.json is authoritative for", async () => {
	const { lockfileVersionDrift } = (await versionModule()) as unknown as {
		lockfileVersionDrift: (
			manifest: { version?: string },
			lock: { version?: string; packages?: Record<string, { version?: string }> },
		) => string[];
	};
	const manifest = committedManifest();
	const lock = JSON.parse(
		readFileSync(join(extensionRoot, "package-lock.json"), "utf8"),
	) as { version?: string; packages?: Record<string, { version?: string }> };

	assert.deepEqual(
		lockfileVersionDrift(manifest, lock),
		[],
		"run `npm install --package-lock-only` in editors/vscode to put the lockfile back on the manifest's number",
	);
});

test("the drift check reads both places npm keeps the number", async () => {
	const { lockfileVersionDrift } = (await versionModule()) as unknown as {
		lockfileVersionDrift: (
			manifest: { version?: string },
			lock: { version?: string; packages?: Record<string, { version?: string }> },
		) => string[];
	};
	// The assertion above passes on a lockfile that agrees and on a check that
	// looks nowhere, so the check is driven wrong here and both sites have to
	// come back. A lockfile carrying neither site counts as drift for the same
	// reason: npm moving the number would otherwise retire this guard in
	// silence.
	//
	// Each case names the complaints it expects rather than counting them, and
	// the first two cases drift one site while leaving the other correct. Those
	// two are what make the check name distinct places: a site's label is a
	// constant, so a fixture drifting both sites to the same number produces
	// the expected messages even from a check that read one site twice, and
	// counting cannot tell those apart either. Under a check narrowed to the
	// top-level version the first case reports nothing at all.
	//
	// The narrowing matters because the drift it would then miss is the one
	// this card is about. Somebody hand-repairs the version at the top of the
	// lockfile, packages[""].version keeps the old number, and the next npm
	// install rewrites the file anyway.
	const cases = [
		{
			lock: { version: "1.0.0", packages: { "": { version: "0.1.0" } } },
			expected: [
				'package-lock.json packages[""].version is 0.1.0 and package.json is 1.0.0, so the next npm install rewrites one of them',
			],
		},
		{
			lock: { version: "0.1.0", packages: { "": { version: "1.0.0" } } },
			expected: [
				"package-lock.json version is 0.1.0 and package.json is 1.0.0, so the next npm install rewrites one of them",
			],
		},
		{
			lock: { version: "0.1.0", packages: { "": { version: "0.2.0" } } },
			expected: [
				"package-lock.json version is 0.1.0 and package.json is 1.0.0, so the next npm install rewrites one of them",
				'package-lock.json packages[""].version is 0.2.0 and package.json is 1.0.0, so the next npm install rewrites one of them',
			],
		},
		{
			lock: {},
			expected: [
				"package-lock.json has no version, so nothing there mirrors package.json's 1.0.0",
				'package-lock.json has no packages[""].version, so nothing there mirrors package.json\'s 1.0.0',
			],
		},
	];
	for (const { lock, expected } of cases) {
		assert.deepEqual(
			lockfileVersionDrift({ version: "1.0.0" }, lock),
			expected,
			`the drift reported for ${JSON.stringify(lock)} does not name both places npm keeps the version`,
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
		/unrelated by design/u.test(readme),
		"the README no longer says the extension's version and the CLI's are unrelated by design",
	);
	assert.ok(
		/profile revision/u.test(readme) && /--json version/u.test(readme),
		"the README no longer names the profile revision as what a reader checks instead",
	);
});

test("the user-facing prose no longer describes the retired 0.0 line", () => {
	// Both sentences told a reader that an archive numbered 0.0.x is not a
	// release, and one of them is on the marketplace listing. The line is gone,
	// so the sentences are false rather than merely stale.
	const readme = readFileSync(join(extensionRoot, "README.md"), "utf8");
	assert.ok(
		!readme.includes("begins `0.0.`") && !readme.includes("0.0.x"),
		"the extension README still describes the retired 0.0 line",
	);
	const releaseDoc = readFileSync(
		join(extensionRoot, "..", "..", "docs", "release.md"),
		"utf8",
	);
	const section = releaseDoc.slice(
		releaseDoc.indexOf("## The VS Code extension's numbers are its own"),
	);
	const ends = section.indexOf("\n## ", 1);
	assert.ok(
		!(ends === -1 ? section : section.slice(0, ends)).includes("0.0.x"),
		"docs/release.md's extension section still describes the retired 0.0.x line",
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
		!publish.includes("--published"),
		"publish-extension.ps1 still passes the retired --published flag, which selects nothing and would package the committed floor",
	);
	assert.ok(
		publish.includes("npm run package -- --version $version"),
		"publish-extension.ps1 no longer packages the version it looked up",
	);
});
