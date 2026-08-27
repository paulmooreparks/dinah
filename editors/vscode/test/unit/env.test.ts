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

test("the temp root itself is not under itself, and a sibling prefix is outside", () => {
	assert.ok(!underRoot(TEMP_ROOT, TEMP_ROOT));
	const sibling = process.platform === "win32" ? "C:\\t\\run-old\\home" : "/t/run-old/home";
	assert.ok(!underRoot(sibling, TEMP_ROOT));
	assert.ok(underRoot(inside, TEMP_ROOT));
});
