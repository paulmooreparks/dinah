import assert from "node:assert/strict";
import { test } from "node:test";

import { ArgvError, composeArgv, runDinah } from "../../src/cli";
import type { SpawnOutcome, Spawner } from "../../src/cli";

/** A spawner that records what it was asked to run and replays one outcome. */
function stub(outcome: SpawnOutcome): {
	spawner: Spawner;
	calls: { exe: string; argv: string[] }[];
} {
	const calls: { exe: string; argv: string[] }[] = [];
	const spawner: Spawner = async (exe, argv) => {
		calls.push({ exe, argv: [...argv] });
		return outcome;
	};
	return { spawner, calls };
}

test("composeArgv puts --json in front of everything", () => {
	assert.deepEqual(composeArgv(["status"]), ["--json", "status"]);
	assert.deepEqual(composeArgv(["--workbench", "C:/x", "status"]), [
		"--json",
		"--workbench",
		"C:/x",
		"status",
	]);
});

test("composeArgv refuses an argv that already carries --json", () => {
	assert.throws(() => composeArgv(["--json", "status"]), ArgvError);
	assert.throws(() => composeArgv(["--json=1", "status"]), ArgvError);
});

test("composeArgv refuses a rendering flag", () => {
	assert.throws(() => composeArgv(["--lang", "de", "status"]), ArgvError);
	assert.throws(() => composeArgv(["--lang=de", "status"]), ArgvError);
	assert.throws(() => composeArgv(["--quiet", "status"]), ArgvError);
});

test("runDinah composes --json onto every invocation", async () => {
	const { spawner, calls } = stub({ code: 0, stdout: "{}", stderr: "" });
	await runDinah(spawner, "dinah", ["version"]);
	assert.deepEqual(calls[0].argv, ["--json", "version"]);
});

test("runDinah reads exit 0 with JSON as ok", async () => {
	const { spawner } = stub({ code: 0, stdout: '{"tool":"0.1.0"}', stderr: "" });
	const outcome = await runDinah(spawner, "dinah", ["version"]);
	assert.equal(outcome.kind, "ok");
});

test("runDinah reads exit 0 with prose as not-json", async () => {
	const { spawner } = stub({ code: 0, stdout: "dinah 0.1.0\n", stderr: "" });
	const outcome = await runDinah(spawner, "dinah", ["version"]);
	assert.equal(outcome.kind, "not-json");
});

test("runDinah reads exit 2 as the refusal envelope on stdout", async () => {
	const envelope = JSON.stringify({
		outcome: "refused",
		refusal: "dinah.ambiguous-workbench",
		context: { base: "C:\\base\\.dinah" },
		workbenches: [
			{ title: "one", slug: "alpha", path: "C:\\base\\.dinah\\aaa" },
			{ title: "two", slug: "beta", path: "C:\\base\\.dinah\\bbb" },
		],
	});
	const { spawner } = stub({ code: 2, stdout: envelope, stderr: "prose" });
	const outcome = await runDinah(spawner, "dinah", ["status"]);
	assert.equal(outcome.kind, "refused");
	if (outcome.kind !== "refused") {
		return;
	}
	assert.equal(outcome.refusal, "dinah.ambiguous-workbench");
	assert.equal(outcome.workbenches?.length, 2);
	assert.equal(outcome.context?.base, "C:\\base\\.dinah");
});

test("runDinah reads exit 4 as unreachable", async () => {
	const { spawner } = stub({ code: 4, stdout: "", stderr: "gone" });
	const outcome = await runDinah(spawner, "dinah", ["status"]);
	assert.equal(outcome.kind, "unreachable");
});

test("runDinah reads a spawn failure as spawn-failed and keeps the errno", async () => {
	const { spawner } = stub({
		code: null,
		stdout: "",
		stderr: "",
		spawnError: { code: "ENOENT", message: "spawn dinah ENOENT" },
	});
	const outcome = await runDinah(spawner, "dinah", ["version"]);
	assert.equal(outcome.kind, "spawn-failed");
	if (outcome.kind !== "spawn-failed") {
		return;
	}
	assert.equal(outcome.errno, "ENOENT");
});
