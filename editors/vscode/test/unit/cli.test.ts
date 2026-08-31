import assert from "node:assert/strict";
import { test } from "node:test";

import {
	ArgvError,
	EXIT_READ_FINDINGS,
	composeArgv,
	runCheck,
	runDinah,
} from "../../src/cli";
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

// runCheck, and the two exit codes a structural read answers on
// ---------------------------------------------------------------------------

/** What a clean check answers, as verb.CheckReport marshals it. */
const CLEAN_REPORT = { outcome: "ok", findings: null };

/** What a check that found one defect answers. */
const DIRTY_REPORT = {
	outcome: "findings",
	findings: [
		{
			Path: "C:/bench/cards/f00000000009/card.md",
			Key: "check.dangling-workstream",
			Detail: "f00000000009",
		},
	],
};

test("runCheck aims the run at one workbench and asks for nothing else", async () => {
	// AC-4. A --migrate-* flag or a --finish here would turn a read the reader
	// asked for into a write they did not, against a workbench they may only
	// have wanted to look at.
	const { spawner, calls } = stub({
		code: 0,
		stdout: JSON.stringify(CLEAN_REPORT),
		stderr: "",
	});
	await runCheck(spawner, "dinah", "C:/bench");
	assert.deepEqual(calls[0].argv, ["--json", "--workbench", "C:/bench", "check"]);
});

test("runCheck reads exit 0 as ok", async () => {
	const { spawner } = stub({
		code: 0,
		stdout: JSON.stringify(CLEAN_REPORT),
		stderr: "",
	});
	const outcome = await runCheck(spawner, "dinah", "C:/bench");
	assert.equal(outcome.kind, "ok");
	assert.deepEqual(outcome.kind === "ok" ? outcome.json : undefined, CLEAN_REPORT);
});

test("runCheck reads exit 5 as ok, because the read happened", async () => {
	// The half dinah-346 landed. A workbench carrying defects is a read that
	// completed, so it is `ok` here and the report's own outcome member is what
	// the caller reads to tell it from a clean one. Before that card this
	// answer arrived on exit 2 and was indistinguishable from a bad
	// --workbench.
	const { spawner } = stub({
		code: EXIT_READ_FINDINGS,
		stdout: JSON.stringify(DIRTY_REPORT),
		stderr: "",
	});
	const outcome = await runCheck(spawner, "dinah", "C:/bench");
	assert.equal(outcome.kind, "ok");
	assert.deepEqual(outcome.kind === "ok" ? outcome.json : undefined, DIRTY_REPORT);
});

test("the exit code a read reports findings on is not the refusal code", async () => {
	// The overload dinah-346 removed, asserted from this side of the wire. A
	// change that put the two back on one code would make the test above and
	// the one below pass on the same fixture, so the codes being different is
	// what those two assertions rest on.
	assert.notEqual(EXIT_READ_FINDINGS, 2);
	assert.equal(EXIT_READ_FINDINGS, 5);
});

test("runCheck reads exit 2 as the refusal it now exclusively means", async () => {
	// AC-3. No body-sniffing: a refusal is a refusal because the CLI said so
	// with its exit code, and this fixture carries no findings key at all
	// because the refusal envelope never has one.
	const { spawner } = stub({
		code: 2,
		stdout: JSON.stringify({
			outcome: "refused",
			refusal: "dinah.no-workbench",
			detail: "C:/nowhere carries no workbench.md",
		}),
		stderr: "",
	});
	const outcome = await runCheck(spawner, "dinah", "C:/nowhere");
	assert.equal(outcome.kind, "refused");
	assert.equal(outcome.kind === "refused" ? outcome.refusal : "", "dinah.no-workbench");
});

test("runCheck reads exit 3 as stale and exit 4 as unreachable", async () => {
	const stale = stub({ code: 3, stdout: "", stderr: "the cursor is old" });
	assert.equal((await runCheck(stale.spawner, "dinah", "C:/b")).kind, "stale");
	const gone = stub({ code: 4, stdout: "", stderr: "no such directory" });
	assert.equal((await runCheck(gone.spawner, "dinah", "C:/b")).kind, "unreachable");
});

test("runCheck reads an undeclared exit code as not-json", async () => {
	const { spawner } = stub({ code: 9, stdout: "", stderr: "" });
	const outcome = await runCheck(spawner, "dinah", "C:/b");
	assert.equal(outcome.kind, "not-json");
});

test("runCheck reads a report-shaped exit with no JSON as not-json", async () => {
	// Both success codes have to answer this, because a binary too old to know
	// the check verb at all can exit 0 and print prose.
	for (const code of [0, EXIT_READ_FINDINGS]) {
		const { spawner } = stub({ code, stdout: "all good\n", stderr: "" });
		const outcome = await runCheck(spawner, "dinah", "C:/b");
		assert.equal(outcome.kind, "not-json", `exit ${String(code)} with prose`);
	}
});

test("runCheck reads a spawn failure as spawn-failed and keeps the errno", async () => {
	const { spawner } = stub({
		code: null,
		stdout: "",
		stderr: "",
		spawnError: { code: "ENOENT", message: "spawn dinah ENOENT" },
	});
	const outcome = await runCheck(spawner, "dinah", "C:/b");
	assert.equal(outcome.kind, "spawn-failed");
	assert.equal(outcome.kind === "spawn-failed" ? outcome.errno : "", "ENOENT");
});
