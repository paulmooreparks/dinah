// What the command palette offers, and what it does when it cannot ask.
//
// Two failures are guarded here rather than one. The first is a palette
// shorter than the tool table with nothing saying so, which is a quiet lie
// about what dinah can do. The second is an empty palette standing in for a
// failure to enumerate at all, which tells a reader "no verbs" when the truth
// is "nobody could ask" (dinah-420 AC3, AC4).

import assert from "node:assert/strict";
import { test } from "node:test";

import type { PickItem } from "../../src/cardCommands";
import type { SpawnOutcome, Spawner } from "../../src/cli";
import { ENGLISH } from "../../src/l10n";
import { REQUEST_ID } from "../../src/mcpClient";
import type { CatalogBuild } from "../../src/verbCatalog";
import { buildCatalog, classifyProperty, verbPickItems } from "../../src/verbCatalog";

/** A spawner answering with the two JSON-RPC lines a real server writes. */
function serving(tools: unknown[]): Spawner {
	return async () => ({
		code: 0,
		stdout:
			`${JSON.stringify({ jsonrpc: "2.0", id: 1, result: { protocolVersion: "2024-11-05" } })}\n` +
			`${JSON.stringify({ jsonrpc: "2.0", id: REQUEST_ID, result: { tools } })}\n`,
		stderr: "",
	});
}

/** A spawner answering with whatever the caller wants on stdout. */
function writing(stdout: string, stderr = ""): Spawner {
	return async () => ({ code: 0, stdout, stderr });
}

/** A spawner that never started a process at all. */
const failingToStart: Spawner = async () => ({
	code: null,
	stdout: "",
	stderr: "",
	spawnError: { code: "ENOENT", message: "spawn dinah ENOENT" },
});

/** One ordinary tool, shaped the way dinah publishes one today. */
function stringTool(name: string): unknown {
	return {
		name,
		description: "does one thing",
		inputSchema: {
			type: "object",
			properties: {
				card: { type: "string", description: "the card" },
				actor: { type: "string", description: "who is acting" },
			},
			required: ["card"],
		},
	};
}

async function build(spawner: Spawner, logged: string[] = []): Promise<CatalogBuild> {
	return buildCatalog({ spawner, exe: "dinah", log: (line) => logged.push(line) });
}

// ---------------------------------------------------------------------------
// AC3: four failures, none of which opens a picker, and one success
// ---------------------------------------------------------------------------

test("a spawn that never started is its own arm and carries the error", async () => {
	const outcome = await build(failingToStart);
	assert.equal(outcome.kind, "spawn-failed");
	assert.ok(
		outcome.kind === "spawn-failed" && outcome.detail.includes("ENOENT"),
		"the arm carries no spawn error",
	);
});

test("a line that is not JSON-RPC fails the read rather than being skipped", async () => {
	const outcome = await build(writing("dinah: unknown command \"mcp\"\n"));
	assert.equal(outcome.kind, "transport-error");
});

test("an empty tools array is reported as no renderable verb, never as a picker", async () => {
	// The tool table is a fixed, non-empty literal in the binary, so an empty
	// one means something is structurally wrong upstream rather than that this
	// build has no verbs. It reaches the reader as the same message a table
	// full of undrawable shapes does, because the next thing to do is the same
	// either way (dinah-420 D5).
	const outcome = await build(serving([]));
	assert.equal(outcome.kind, "no-renderable-verbs");
});

test("a table where every entry is undrawable is the same arm, not an empty success", async () => {
	const outcome = await build(
		serving([
			{
				name: "future",
				inputSchema: {
					type: "object",
					properties: { tags: { type: "array", description: "several" } },
				},
			},
		]),
	);
	assert.equal(outcome.kind, "no-renderable-verbs");
});

test("a current-shape table answers ok with at least one verb the palette can run", async () => {
	const outcome = await build(serving([stringTool("claim"), stringTool("release")]));
	assert.equal(outcome.kind, "ok");
	assert.ok(outcome.kind === "ok" && outcome.verbs.length >= 1);
});

test("no failing build ever produces a pick list at all", async () => {
	// The picker is reachable only through the ok arm, so the check is that
	// the other three arms carry no verb list for a caller to hand a picker.
	// A test asserting that host.pick was not called would pass just as well
	// against a wizard that called it with an empty array a moment later.
	const failures: Spawner[] = [
		failingToStart,
		writing("not json at all\n"),
		serving([]),
	];
	for (const spawner of failures) {
		const outcome = await build(spawner);
		assert.notEqual(outcome.kind, "ok");
		assert.ok(
			!("verbs" in outcome),
			`a ${outcome.kind} build carried a verb list: ${JSON.stringify(outcome)}`,
		);
	}
});

// ---------------------------------------------------------------------------
// AC4: the verb that cannot be drawn is left out, said so, and counted
// ---------------------------------------------------------------------------

test("an undrawable verb is left out, named in the log, and counted in a separator", async () => {
	const logged: string[] = [];
	const outcome = await build(
		serving([
			stringTool("claim"),
			{
				name: "future",
				inputSchema: {
					type: "object",
					properties: {
						card: { type: "string", description: "the card" },
						tags: { type: "array", description: "several tags" },
					},
				},
			},
		]),
		logged,
	);
	assert.equal(outcome.kind, "ok");
	if (outcome.kind !== "ok") {
		return;
	}
	assert.deepEqual(
		outcome.verbs.map((verb) => verb.name),
		["claim"],
	);
	assert.equal(logged.length, 1);
	assert.ok(logged[0].includes("future"), `the line names no tool: ${logged[0]}`);
	assert.ok(logged[0].includes("tags"), `the line names no property: ${logged[0]}`);
	assert.ok(logged[0].includes("array"), `the line names no reason: ${logged[0]}`);

	const items: PickItem[] = verbPickItems(outcome.verbs, outcome.excluded, ENGLISH);
	const last = items[items.length - 1];
	assert.equal(last.kind, "separator");
	assert.ok(
		last.label.includes("1"),
		`the separator names no count: ${last.label}`,
	);
	assert.equal(items.filter((item) => item.kind !== "separator").length, 1);
});

test("a table this build can draw entirely ends in no separator row", async () => {
	const outcome = await build(serving([stringTool("claim")]));
	assert.equal(outcome.kind, "ok");
	if (outcome.kind !== "ok") {
		return;
	}
	const items = verbPickItems(outcome.verbs, outcome.excluded, ENGLISH);
	assert.deepEqual(
		items.map((item) => item.kind),
		[undefined],
	);
});

// ---------------------------------------------------------------------------
// The classifier's own five answers, and what it refuses
// ---------------------------------------------------------------------------

test("each published property shape classifies as the prompt it earns", () => {
	assert.deepEqual(classifyProperty({ type: "string" }), {
		kind: "prompt",
		prompt: { kind: "text" },
	});
	assert.deepEqual(classifyProperty({ type: "boolean" }), {
		kind: "prompt",
		prompt: { kind: "boolean" },
	});
	assert.deepEqual(classifyProperty({ type: "string", enum: ["queue", "flow"] }), {
		kind: "prompt",
		prompt: { kind: "choice", values: ["queue", "flow"] },
	});
	assert.deepEqual(
		classifyProperty({ type: "string", "x-dinah-vocabulary-source": "columns" }),
		{ kind: "prompt", prompt: { kind: "vocabulary", source: "columns" } },
	);
	assert.deepEqual(classifyProperty({ type: "string", format: "duration" }), {
		kind: "prompt",
		prompt: { kind: "duration" },
	});
});

test("a shape this build has no rule for is unrenderable and says which", () => {
	// The singular spelling is in here on purpose. dinah publishes the source
	// `columns`, and a resolver table keyed on `column` would match nothing
	// and take four verbs out of the palette without an error anywhere, so the
	// singular has to reach the reader as an exclusion rather than as silence.
	for (const property of [
		{ type: "number" },
		{ description: "no type at all" },
		{ type: "string", format: "uri" },
		{ type: "string", "x-dinah-vocabulary-source": "column" },
		{ type: "string", "x-dinah-vocabulary-source": "guides" },
		{ type: "string", enum: [] },
		{ type: "boolean", enum: ["yes"] },
		{ type: "string", enum: ["a"], format: "duration" },
	]) {
		const verdict = classifyProperty(property);
		assert.equal(
			verdict.kind,
			"unrenderable",
			`${JSON.stringify(property)} classified as renderable`,
		);
		assert.ok(
			verdict.kind === "unrenderable" && verdict.detail.trim() !== "",
			`${JSON.stringify(property)} was excluded with no reason`,
		);
	}
});

test("a required argument is prompted before an optional one", async () => {
	const outcome = await build(serving([stringTool("claim")]));
	assert.equal(outcome.kind, "ok");
	if (outcome.kind !== "ok") {
		return;
	}
	assert.deepEqual(
		outcome.verbs[0].args.map((argument) => [argument.name, argument.required]),
		[
			["card", true],
			["actor", false],
		],
	);
});

test("a spawn outcome carrying no answer to the request is a transport error", async () => {
	const outcome: SpawnOutcome = {
		code: 0,
		stdout: `${JSON.stringify({ jsonrpc: "2.0", id: 1, result: {} })}\n`,
		stderr: "",
	};
	const built = await build(async () => outcome);
	assert.equal(built.kind, "transport-error");
});
