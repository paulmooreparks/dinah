// The compatibility gate's classification table, one case per row.
//
// Collapsing any two rows into one outcome turns that row's case red. The
// tool "0.1.0" row is the one that proves the gate does not read the release
// string: every build from source reports that literal forever, so a gate
// comparing it would refuse every contributor's own binary.

import assert from "node:assert/strict";
import { test } from "node:test";

import type { CliOutcome } from "../../src/cli";
import { classifyVersion, parseProfile } from "../../src/version";

/**
 * An exit-0 answer carrying the fields `--json version` reports.
 *
 * `executable` is typed as `unknown` and left out by default, so every row
 * below is unchanged and the type-rejection case can be expressed.
 */
function reported(
	tool: string,
	profile: string,
	format: number,
	executable?: unknown,
): CliOutcome {
	const json: Record<string, unknown> = { tool, profile, format };
	if (executable !== undefined) {
		json.executable = executable;
	}
	return { kind: "ok", json };
}

const rows: {
	row: string;
	outcome: CliOutcome;
	expected: string;
	detailHas?: string[];
}[] = [
	{
		row: "spawn fails with ENOENT",
		outcome: {
			kind: "spawn-failed",
			errno: "ENOENT",
			detail: "spawn dinah ENOENT",
		},
		expected: "enoent",
		detailHas: ["PATH", "dinah.path"],
	},
	{
		row: "spawn fails for another reason",
		outcome: {
			kind: "spawn-failed",
			errno: "EACCES",
			detail: "spawn dinah EACCES",
		},
		expected: "unusable",
		detailHas: ["EACCES"],
	},
	{
		row: "exit 2 with a refusal envelope",
		outcome: { kind: "refused", refusal: "dinah.no-workbench-found", detail: "here" },
		expected: "refused",
	},
	{
		row: "exit 4",
		outcome: { kind: "unreachable", detail: "gone" },
		expected: "unreachable",
	},
	{
		row: "exit 0 with non-JSON stdout",
		outcome: { kind: "not-json", detail: "this binary is not dinah, or is too old to answer `--json version`" },
		expected: "unusable",
		detailHas: ["not dinah"],
	},
	{
		row: "format outside the supported set",
		outcome: reported("v0.1.0-dev.42", "dinah-core/0.18", 99),
		expected: "format-skew",
		detailHas: ["99", "1, 2, 3"],
	},
	{
		row: "profile name differs",
		outcome: reported("v0.1.0-dev.42", "andoneer-core/0.4", 1),
		expected: "profile-skew",
	},
	{
		row: "profile major differs",
		outcome: reported("v0.1.0-dev.42", "dinah-core/1.4", 1),
		expected: "profile-skew",
	},
	{
		row: "profile minor below the minimum",
		// dinah-597 moved the minimum to 0.18, the revision that gave unblock
		// its reason, so the binary refused here is one claiming 0.17, which
		// is what a build from before that card claims.
		outcome: reported("v0.1.0-dev.42", "dinah-core/0.17", 1),
		expected: "binary-too-old",
		detailHas: ["0.17", "0.18"],
	},
	{
		row: "profile minor above the minimum",
		outcome: reported("v0.1.0-dev.42", "dinah-core/0.19", 1),
		expected: "ok",
	},
	{
		row: 'a source build reporting tool "0.1.0"',
		outcome: reported("0.1.0", "dinah-core/0.18", 1),
		expected: "ok",
	},
];

for (const { row, outcome, expected, detailHas } of rows) {
	test(`classifyVersion: ${row} classifies ${expected}`, () => {
		const classification = classifyVersion(outcome);
		assert.equal(classification.kind, expected);
		for (const fragment of detailHas ?? []) {
			const detail = (classification as { detail?: string }).detail ?? "";
			assert.ok(
				detail.includes(fragment),
				`expected the diagnostic to name ${fragment}, got: ${detail}`,
			);
		}
	});
}

test("classifyVersion: an answer missing a field is unusable rather than ok", () => {
	const classification = classifyVersion({
		kind: "ok",
		json: { tool: "0.1.0", profile: "dinah-core/0.18" },
	});
	assert.equal(classification.kind, "unusable");
});

// The two revisions dinah-358 names. The first is what a build claimed
// immediately before dinah-346 changed what `dinah check` returns to whoever
// invoked it, and the second is the revision that published CORE-OUT-7, the
// statement holding a tool to giving `refused` a number no other outcome uses.
const PROFILE_BEFORE_THE_READ_EXIT_CONVENTION = "dinah-core/0.7";
const PROFILE_PUBLISHING_THE_READ_EXIT_RULE = "dinah-core/0.9";

/**
 * A client-side gate for the read exit convention, written with the reader
 * the extension already publishes. It reads the conformance claim and nothing
 * else: not `tool`, which says nothing about conformance, and not the shape of
 * an answer the binary has given, which is the guessing dinah-346 and
 * dinah-353 both refuse.
 */
function speaksTheReadExitConvention(profile: string): boolean {
	const floor = parseProfile(PROFILE_PUBLISHING_THE_READ_EXIT_RULE);
	const claimed = parseProfile(profile);
	if (floor === undefined || claimed === undefined) {
		return false;
	}
	if (claimed.name !== floor.name) {
		return false;
	}
	if (claimed.major !== floor.major) {
		return claimed.major > floor.major;
	}
	return claimed.minor >= floor.minor;
}

test("a client tells the read exit convention apart from the version alone", () => {
	// dinah-358 AC-5 from the client's side. The two claims differ by nothing
	// a caller can see except the number, and the number is the whole answer:
	// a binary claiming the later revision exits 5 for a workbench carrying
	// findings, and one claiming the earlier revision published nothing about
	// what its exit status means.
	assert.ok(
		!speaksTheReadExitConvention(PROFILE_BEFORE_THE_READ_EXIT_CONVENTION),
		`${PROFILE_BEFORE_THE_READ_EXIT_CONVENTION} published nothing about the exit status a reading returns`,
	);
	assert.ok(
		speaksTheReadExitConvention(PROFILE_PUBLISHING_THE_READ_EXIT_RULE),
		`${PROFILE_PUBLISHING_THE_READ_EXIT_RULE} is the revision CORE-OUT-7 was published at`,
	);

	// The decision is taken from the version report and from nothing else, so
	// two binaries whose release numbers are identical are still told apart.
	// Every build from source reports the same release string forever, which
	// is why a gate reading it answers the same for both of these.
	//
	// Both revisions sit below the gate since dinah-597 moved it to 0.18, so
	// each classifies binary-too-old rather than ok. The classification still
	// carries the report it read, and the gate here reads the claim off that
	// report, which is the point: the answer comes from the version report
	// whatever the activation gate made of it.
	const older = reported("0.1.0", PROFILE_BEFORE_THE_READ_EXIT_CONVENTION, 1);
	const newer = reported("0.1.0", PROFILE_PUBLISHING_THE_READ_EXIT_RULE, 1);
	for (const [outcome, expected] of [
		[older, false],
		[newer, true],
	] as const) {
		const classification = classifyVersion(outcome);
		assert.equal(classification.kind, "binary-too-old");
		assert.equal(
			speaksTheReadExitConvention(
				(classification as { version: { profile: string } }).version.profile,
			),
			expected,
		);
	}
});

test("the decode carries the binary's own location through, and only as a string", () => {
	// dinah-424 AC-16. readReport builds its result field by field, so a field
	// named nowhere there is undefined on every binary however faithfully the
	// CLI sends it. The payload is parsed rather than hand-built, because a
	// hand-built VersionReport would pass over a decoder that drops the field.
	//
	// This proves the decode carries what it is given and proves nothing about
	// what the CLI sends. versionExecutable-live.test.ts is the join.
	const carried = classifyVersion(
		reported("0.1.0", "dinah-core/0.18", 1, "C:/tools/dinah.exe"),
	);
	assert.equal(carried.kind, "ok");
	assert.equal(
		(carried as { version: { executable?: string } }).version.executable,
		"C:/tools/dinah.exe",
	);

	// A value of another type is read as absent rather than refusing the whole
	// report, because the three fields the gate reads are what decide whether
	// the binary is usable at all.
	const mistyped = classifyVersion(reported("0.1.0", "dinah-core/0.18", 1, 17));
	assert.equal(mistyped.kind, "ok");
	assert.equal(
		(mistyped as { version: { executable?: string } }).version.executable,
		undefined,
	);

	// A binary older than the field sends none, and so does one whose own
	// os.Executable failed.
	const absent = classifyVersion(reported("0.1.0", "dinah-core/0.18", 1));
	assert.equal(absent.kind, "ok");
	assert.equal(
		(absent as { version: { executable?: string } }).version.executable,
		undefined,
	);
});
