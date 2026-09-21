import assert from "node:assert/strict";
import { test } from "node:test";

import { TempRootViolation, checkedEnv, underRoot } from "../support/env";

const TEMP_ROOT = process.platform === "win32" ? "C:\\t\\run" : "/t/run";
const inside = process.platform === "win32" ? "C:\\t\\run\\home" : "/t/run/home";
const outside = process.platform === "win32" ? "C:\\Users\\paul" : "/home/paul";

test("an environment with DINAH_HOME under the temp root is built", () => {
	const env = checkedEnv(TEMP_ROOT, { DINAH_HOME: inside, DINAH_ACTOR: "tester" });
	assert.equal(env.DINAH_HOME, inside);
	assert.equal(env.DINAH_ACTOR, "tester");
});

test("an environment with no DINAH_HOME is refused", () => {
	// Without it, `dinah init` in a fixture writes into the operator's own
	// user base and the test passes anyway.
	assert.throws(() => checkedEnv(TEMP_ROOT, { DINAH_ACTOR: "tester" }), TempRootViolation);
	assert.throws(() => checkedEnv(TEMP_ROOT, { DINAH_HOME: "  " }), TempRootViolation);
});

test("an environment with DINAH_HOME outside the temp root is refused", () => {
	assert.throws(() => checkedEnv(TEMP_ROOT, { DINAH_HOME: outside }), TempRootViolation);
});

test("an inherited DINAH_WORKBENCH is blanked rather than passed to the child", () => {
	// DINAH_WORKBENCH outranks the directory walk, so a child inheriting one
	// from the shell acts on that workbench wherever it runs. The value set
	// here stands for a shell that exported it to reach a real workbench.
	const saved = process.env.DINAH_WORKBENCH;
	process.env.DINAH_WORKBENCH = outside;
	try {
		const env = checkedEnv(TEMP_ROOT, { DINAH_HOME: inside });
		assert.equal(env.DINAH_WORKBENCH, "");
	} finally {
		if (saved === undefined) {
			delete process.env.DINAH_WORKBENCH;
		} else {
			process.env.DINAH_WORKBENCH = saved;
		}
	}
});

test("a DINAH_WORKBENCH the caller names under the temp root is kept", () => {
	const bench = process.platform === "win32" ? "C:\\t\\run\\wb" : "/t/run/wb";
	const env = checkedEnv(TEMP_ROOT, { DINAH_HOME: inside, DINAH_WORKBENCH: bench });
	assert.equal(env.DINAH_WORKBENCH, bench);
});

test("a DINAH_WORKBENCH the caller names outside the temp root is refused", () => {
	assert.throws(
		() => checkedEnv(TEMP_ROOT, { DINAH_HOME: inside, DINAH_WORKBENCH: outside }),
		TempRootViolation,
	);
});

test("the temp root itself is not under itself, and a sibling prefix is outside", () => {
	assert.ok(!underRoot(TEMP_ROOT, TEMP_ROOT));
	const sibling = process.platform === "win32" ? "C:\\t\\run-old\\home" : "/t/run-old/home";
	assert.ok(!underRoot(sibling, TEMP_ROOT));
	assert.ok(underRoot(inside, TEMP_ROOT));
});
