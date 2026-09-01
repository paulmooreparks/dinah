// The compatibility gate's classification table, one case per row.
//
// Collapsing any two rows into one outcome turns that row's case red. The
// tool "0.1.0" row is the one that proves the gate does not read the release
// string: every build from source reports that literal forever, so a gate
// comparing it would refuse every contributor's own binary.

import assert from "node:assert/strict";
import { test } from "node:test";

import type { CliOutcome } from "../../src/cli";
import { classifyVersion, demotes, parseProfile } from "../../src/version";

/** An exit-0 answer carrying the three fields `--json version` reports. */
function reported(tool: string, profile: string, format: number): CliOutcome {
	return { kind: "ok", json: { tool, profile, format } };
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
		outcome: reported("v0.1.0-dev.42", "dinah-core/0.4", 99),
		expected: "format-skew",
		detailHas: ["99", "1"],
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
		outcome: reported("v0.1.0-dev.42", "dinah-core/0.3", 1),
		expected: "binary-too-old",
		detailHas: ["0.3", "0.4"],
	},
	{
		row: "profile minor above the minimum",
		outcome: reported("v0.1.0-dev.42", "dinah-core/0.9", 1),
		expected: "ok",
	},
	{
		row: 'a source build reporting tool "0.1.0"',
		outcome: reported("0.1.0", "dinah-core/0.4", 1),
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
		json: { tool: "0.1.0", profile: "dinah-core/0.4" },
	});
	assert.equal(classification.kind, "unusable");
});

test("only the three skew rows demote to a carried binary", () => {
	assert.ok(demotes(classifyVersion(reported("x", "dinah-core/0.4", 99))));
	assert.ok(demotes(classifyVersion(reported("x", "dinah-core/1.4", 1))));
	assert.ok(demotes(classifyVersion(reported("x", "dinah-core/0.3", 1))));
	assert.ok(!demotes(classifyVersion(reported("x", "dinah-core/0.4", 1))));
	assert.ok(!demotes(classifyVersion({ kind: "unreachable", detail: "gone" })));
	assert.ok(
		!demotes(
			classifyVersion({ kind: "spawn-failed", errno: "ENOENT", detail: "no" }),
		),
	);
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
	const older = reported("0.1.0", PROFILE_BEFORE_THE_READ_EXIT_CONVENTION, 1);
	const newer = reported("0.1.0", PROFILE_PUBLISHING_THE_READ_EXIT_RULE, 1);
	for (const [outcome, expected] of [
		[older, false],
		[newer, true],
	] as const) {
		const classification = classifyVersion(outcome);
		assert.equal(classification.kind, "ok");
		assert.equal(
			speaksTheReadExitConvention(
				(classification as { version: { profile: string } }).version.profile,
			),
			expected,
		);
	}
});
