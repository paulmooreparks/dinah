// The binary resolution ladder: setting, then PATH, then the carried copy,
// then nothing. Each rung returns its own source label, so reordering the
// implementation turns a case here red.

import assert from "node:assert/strict";
import { test } from "node:test";

import { PATH_NAME, findCarried, resolveBinary } from "../../src/binary";
import type { ResolveDeps } from "../../src/binary";
import { classifyVersion } from "../../src/version";
import type { Classification } from "../../src/version";

const CARRIED_DIR = "/ext/bin";
const CARRIED = "/ext/bin/dinah-linux-amd64";

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
	entries?: string[] | { errno: string };
	answers?: Record<string, Classification>;
}

function deps(options: Options): {
	deps: ResolveDeps;
	chmodded: string[];
	probed: string[];
} {
	const chmodded: string[] = [];
	const probed: string[] = [];
	const entries = options.entries ?? ["dinah-linux-amd64"];
	return {
		chmodded,
		probed,
		deps: {
			setting: options.setting ?? "",
			carriedDir: CARRIED_DIR,
			listCarried: async () => {
				if (Array.isArray(entries)) {
					return entries;
				}
				const err = new Error(`ENOENT: no such directory`) as NodeJS.ErrnoException;
				err.code = entries.errno;
				throw err;
			},
			join: (dir, name) => `${dir}/${name}`,
			ensureExecutable: async (path) => {
				chmodded.push(path);
			},
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
		answers: { "/usr/local/bin/dinah": OK, [PATH_NAME]: OK, [CARRIED]: OK },
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

test("rung 2: with no setting, PATH wins over the carried copy", async () => {
	const { deps: d } = deps({ answers: { [PATH_NAME]: OK, [CARRIED]: OK } });
	const state = await resolveBinary(d);
	assert.equal(state.state, "ok");
	if (state.state !== "ok") {
		return;
	}
	assert.equal(state.source, "path");
	assert.equal(state.path, PATH_NAME);
});

test("rung 3: ENOENT on PATH falls through to the carried copy", async () => {
	const { deps: d, chmodded } = deps({
		answers: { [PATH_NAME]: ENOENT, [CARRIED]: OK },
	});
	const state = await resolveBinary(d);
	assert.equal(state.state, "ok");
	if (state.state !== "ok") {
		return;
	}
	assert.equal(state.source, "carried");
	assert.equal(state.path, CARRIED);
	assert.deepEqual(chmodded, [CARRIED]);
});

test("rung 4: nothing on PATH and nothing carried is no-binary", async () => {
	const { deps: d } = deps({ entries: [], answers: { [PATH_NAME]: ENOENT } });
	const state = await resolveBinary(d);
	assert.equal(state.state, "no-binary");
});

test("rung 4: a bin/ directory that does not exist at all is no-binary, not an error", async () => {
	// The universal vsix carries no bin/ directory. It is gitignored and git
	// tracks no empty directory, so ENOENT here is every universal install.
	const { deps: d } = deps({
		entries: { errno: "ENOENT" },
		answers: { [PATH_NAME]: ENOENT },
	});
	const state = await resolveBinary(d);
	assert.equal(state.state, "no-binary");
});

test("a listCarried failure that is not ENOENT is not swallowed", async () => {
	const { deps: d } = deps({
		entries: { errno: "EACCES" },
		answers: { [PATH_NAME]: ENOENT },
	});
	await assert.rejects(() => resolveBinary(d));
});

test("a spawn failure other than ENOENT is reported rather than stepped over", async () => {
	const { deps: d } = deps({ answers: { [PATH_NAME]: EACCES, [CARRIED]: OK } });
	const state = await resolveBinary(d);
	assert.equal(state.state, "unusable");
});

test("the announced demotion: a skewed PATH binary yields to a good carried one", async () => {
	const { deps: d } = deps({
		answers: { [PATH_NAME]: FORMAT_99, [CARRIED]: OK },
	});
	const state = await resolveBinary(d);
	assert.equal(state.state, "ok");
	if (state.state !== "ok") {
		return;
	}
	assert.equal(state.source, "carried");
	assert.equal(state.path, CARRIED);

	// A silent demotion would pass everything above. The diagnostic is what
	// makes it announced, so it has to name both paths and both versions.
	const said = state.demotedFrom ?? "";
	assert.ok(said.includes(PATH_NAME), `expected the PATH binary named: ${said}`);
	assert.ok(said.includes(CARRIED), `expected the carried path named: ${said}`);
	assert.ok(said.includes("format 99"), `expected the PATH version named: ${said}`);
	assert.ok(said.includes("format 1"), `expected the carried version named: ${said}`);
	assert.ok(
		said.includes("v0.1.0-dev.9") && said.includes("v0.1.0-dev.42"),
		`expected both release strings named: ${said}`,
	);
});

test("a skewed PATH binary with no carried copy reports the skew", async () => {
	const { deps: d } = deps({ entries: [], answers: { [PATH_NAME]: FORMAT_99 } });
	const state = await resolveBinary(d);
	assert.equal(state.state, "format-skew");
});

test("a skewed PATH binary and a skewed carried one reports the PATH skew", async () => {
	const { deps: d } = deps({
		answers: { [PATH_NAME]: FORMAT_99, [CARRIED]: FORMAT_99 },
	});
	const state = await resolveBinary(d);
	assert.equal(state.state, "format-skew");
	if (state.state !== "format-skew") {
		return;
	}
	assert.equal(state.path, PATH_NAME);
});

test("findCarried takes the sole entry rather than composing a name", async () => {
	const { deps: d } = deps({ entries: ["dinah-darwin-arm64"] });
	assert.equal(await findCarried(d), "/ext/bin/dinah-darwin-arm64");
});

test("findCarried refuses to guess when two binaries are staged", async () => {
	const { deps: d } = deps({ entries: ["dinah-linux-amd64", "dinah-linux-arm64"] });
	assert.equal(await findCarried(d), undefined);
});
