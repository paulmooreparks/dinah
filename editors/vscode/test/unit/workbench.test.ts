import assert from "node:assert/strict";
import { test } from "node:test";

import type { SpawnOutcome, Spawner } from "../../src/cli";
import {
	AMBIGUOUS_WORKBENCH,
	UNNAMED_SOURCE,
	isInside,
	parseRefusal,
	parseStatus,
	resolveWorkbench,
} from "../../src/workbench";

function stub(outcome: SpawnOutcome): {
	spawner: Spawner;
	calls: { argv: string[]; cwd?: string }[];
} {
	const calls: { argv: string[]; cwd?: string }[] = [];
	const spawner: Spawner = async (_exe, argv, options) => {
		calls.push({ argv: [...argv], cwd: options.cwd });
		return outcome;
	};
	return { spawner, calls };
}

test("parseStatus reads root, title, profile and the rung that answered", () => {
	const parsed = parseStatus(
		{
			workbench: "Dinah",
			root: "C:\\work\\bench\\.dinah\\abc",
			profile: "dinah-core/0.4",
			workbench_source: "search",
		},
		"C:\\work\\bench",
		true,
	);
	assert.equal(parsed?.state, "ok");
	if (parsed?.state !== "ok") {
		return;
	}
	assert.equal(parsed.root, "C:\\work\\bench\\.dinah\\abc");
	assert.equal(parsed.source, "search");
	assert.equal(parsed.insideWorkspace, true);
});

test("parseStatus defaults the rung when workbench_source is absent", () => {
	// verb.Status declares WorkbenchSource `json:"workbench_source,omitempty"`,
	// so an unnamed rung leaves the key out entirely rather than sending an
	// empty string. Without a default this renders "undefined" into a tooltip.
	const parsed = parseStatus(
		{ workbench: "Dinah", root: "/w/.dinah/abc", profile: "dinah-core/0.4" },
		"/w",
		false,
	);
	assert.equal(parsed?.state, "ok");
	if (parsed?.state !== "ok") {
		return;
	}
	assert.equal(parsed.source, UNNAMED_SOURCE);
});

test("parseStatus reports a root outside the folder as outside it", () => {
	// The dinah-241 case: the walk climbed past the opened folder and landed
	// on the parent's workbench.
	const parsed = parseStatus(
		{ workbench: "Parent", root: "/w/.dinah/abc", profile: "p", workbench_source: "search" },
		"/w/sub",
		false,
	);
	assert.equal(parsed?.state, "ok");
	if (parsed?.state !== "ok") {
		return;
	}
	assert.equal(parsed.insideWorkspace, false);
});

test("parseStatus rejects an answer carrying no root", () => {
	assert.equal(parseStatus({ workbench: "Dinah" }, "/w", false), undefined);
});

test("isInside compares segment by segment rather than as a text prefix", () => {
	assert.ok(isInside("/scratch/card/wt", "/scratch/card", false));
	assert.ok(!isInside("/scratch/card-old/wt", "/scratch/card", false));
	assert.ok(isInside("/scratch/card", "/scratch/card", false));
	assert.ok(!isInside("/scratch", "/scratch/card", false));
});

test("isInside folds case only where the platform does", () => {
	assert.ok(isInside("C:\\Work\\Bench\\.dinah", "c:/work/bench", true));
	assert.ok(!isInside("/Work/bench/.dinah", "/work/bench", false));
});

test("parseRefusal keeps the candidate list an ambiguous refusal carries", () => {
	const resolution = parseRefusal({
		kind: "refused",
		refusal: AMBIGUOUS_WORKBENCH,
		workbenches: [
			{ title: "one", slug: "alpha", path: "/base/.dinah/aaa" },
			{ title: "two", slug: "beta", path: "/base/.dinah/bbb" },
		],
	});
	assert.equal(resolution.state, "refused");
	if (resolution.state !== "refused") {
		return;
	}
	assert.equal(resolution.candidates?.length, 2);
});

test("resolveWorkbench runs status in the folder and adds no --workbench when unpinned", async () => {
	const { spawner, calls } = stub({
		code: 0,
		stdout: JSON.stringify({
			workbench: "Dinah",
			root: "/w/.dinah/abc",
			profile: "dinah-core/0.4",
			workbench_source: "search",
		}),
		stderr: "",
	});
	const resolution = await resolveWorkbench(spawner, "dinah", "/w", "", false);
	assert.equal(resolution.state, "ok");
	assert.deepEqual(calls[0].argv, ["--json", "status"]);
	assert.equal(calls[0].cwd, "/w");
});

test("resolveWorkbench pins the answer when dinah.workbench is set", async () => {
	const { spawner, calls } = stub({
		code: 0,
		stdout: JSON.stringify({
			workbench: "Dinah",
			root: "/pinned",
			profile: "dinah-core/0.4",
			workbench_source: "flag",
		}),
		stderr: "",
	});
	await resolveWorkbench(spawner, "dinah", "/w", "  /pinned  ", false);
	assert.deepEqual(calls[0].argv, ["--json", "--workbench", "/pinned", "status"]);
});

test("resolveWorkbench chooses nothing on an ambiguous refusal", async () => {
	const { spawner } = stub({
		code: 2,
		stdout: JSON.stringify({
			outcome: "refused",
			refusal: AMBIGUOUS_WORKBENCH,
			workbenches: [
				{ title: "one", path: "/base/.dinah/aaa" },
				{ title: "two", path: "/base/.dinah/bbb" },
			],
		}),
		stderr: "prose",
	});
	const resolution = await resolveWorkbench(spawner, "dinah", "/base", "", false);
	assert.equal(resolution.state, "refused");
	if (resolution.state !== "refused") {
		return;
	}
	assert.equal(resolution.refusal, AMBIGUOUS_WORKBENCH);
	assert.equal(resolution.candidates?.length, 2);
	assert.ok(!("root" in resolution));
});
