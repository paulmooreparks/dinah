// The compatibility gate's classification table, one case per row.
//
// Collapsing any two rows into one outcome turns that row's case red. The
// tool "0.1.0" row is the one that proves the gate does not read the release
// string: every build from source reports that literal forever, so a gate
// comparing it would refuse every contributor's own binary.

import assert from "node:assert/strict";
import { test } from "node:test";

import type { CliOutcome } from "../../src/cli";
import { classifyVersion, demotes } from "../../src/version";

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
