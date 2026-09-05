// The binary resolution ladder: the dinah.path setting, then PATH, then
// nothing. Each rung returns its own source label, so reordering the
// implementation turns a case here red.

import assert from "node:assert/strict";
import { test } from "node:test";

import { PATH_NAME, resolveBinary } from "../../src/binary";
import type { ResolveDeps } from "../../src/binary";
import { classifyVersion } from "../../src/version";
import type { Classification } from "../../src/version";

const OK: Classification = classifyVersion({
	kind: "ok",
	json: { tool: "v0.1.0-dev.42", profile: "dinah-core/0.4", format: 1 },
});
const FORMAT_99: Classification = classifyVersion({
	kind: "ok",
	json: { tool: "v0.1.0-dev.9", profile: "dinah-core/0.4", format: 99 },
});
const ENOENT: Classification = classifyVersion({
	kind: "spawn-failed",
	errno: "ENOENT",
	detail: "spawn dinah ENOENT",
});
const EACCES: Classification = classifyVersion({
	kind: "spawn-failed",
	errno: "EACCES",
	detail: "spawn dinah EACCES",
});

interface Options {
	setting?: string;
	answers?: Record<string, Classification>;
}

function deps(options: Options): { deps: ResolveDeps; probed: string[] } {
	const probed: string[] = [];
	return {
		probed,
		deps: {
			setting: options.setting ?? "",
			join: (dir, name) => `${dir}/${name}`,
			probe: async (exe) => {
				probed.push(exe);
				return options.answers?.[exe] ?? ENOENT;
			},
		},
	};
}

test("rung 1: the dinah.path setting wins and reports source setting", async () => {
	const { deps: d, probed } = deps({
		setting: "/usr/local/bin/dinah",
		answers: { "/usr/local/bin/dinah": OK, [PATH_NAME]: OK },
	});
	const state = await resolveBinary(d);
	assert.equal(state.state, "ok");
	if (state.state !== "ok") {
		return;
	}
	assert.equal(state.source, "setting");
	assert.equal(state.path, "/usr/local/bin/dinah");
	assert.deepEqual(probed, ["/usr/local/bin/dinah"]);
});

test("rung 2: with no setting, PATH answers and reports source path", async () => {
	const { deps: d } = deps({ answers: { [PATH_NAME]: OK } });
	const state = await resolveBinary(d);
	assert.equal(state.state, "ok");
	if (state.state !== "ok") {
		return;
	}
	assert.equal(state.source, "path");
	assert.equal(state.path, PATH_NAME);
});

test("neither rung answers, so nothing on PATH is no-binary", async () => {
	const { deps: d } = deps({ answers: { [PATH_NAME]: ENOENT } });
	const state = await resolveBinary(d);
	assert.equal(state.state, "no-binary");
});

test("a spawn failure other than ENOENT is reported rather than stepped over", async () => {
	// A permission denial or a binary built for another architecture is a real
	// answer about a real file, and there is no rung below this one to fall to.
	const { deps: d } = deps({ answers: { [PATH_NAME]: EACCES } });
	const state = await resolveBinary(d);
	assert.equal(state.state, "unusable");
});

test("a skewed PATH binary reports the skew rather than being stepped over", async () => {
	const { deps: d } = deps({ answers: { [PATH_NAME]: FORMAT_99 } });
	const state = await resolveBinary(d);
	assert.equal(state.state, "format-skew");
	if (state.state !== "format-skew") {
		return;
	}
	assert.equal(state.path, PATH_NAME);
});
