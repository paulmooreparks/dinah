// The wizard, driven end to end without a window.
//
// What is asserted is the call the wizard composes, because that is the whole
// of what reaches the board. An optional argument the reader skipped must be
// absent from the arguments object rather than present and empty, a marker
// answered "No" must be absent rather than false, and a column choice must
// travel as the value dinah resolves rather than as the title a reader read
// (dinah-420 AC5 through AC8).

import assert from "node:assert/strict";
import { test } from "node:test";

import type { CommandHost, PickItem } from "../../src/cardCommands";
import type { SpawnOptions, Spawner } from "../../src/cli";
import { ENGLISH } from "../../src/l10n";
import { REQUEST_ID } from "../../src/mcpClient";
import type { RunVerbContext } from "../../src/runVerbCommand";
import { classifyCallResult, runVerbFromPalette } from "../../src/runVerbCommand";
import { VerbCatalog } from "../../src/verbCatalog";

/** One tool_call the wizard made, as the server would have read it. */
interface Call {
	readonly method: string;
	readonly name?: string;
	readonly arguments?: Record<string, unknown>;
}

/** A column as the columns tool answers with one. */
interface ColumnRow {
	readonly id: string;
	readonly slug?: string;
	readonly title: string;
}

/** Everything one wizard run said, was asked, and sent. */
interface Driver {
	readonly context: RunVerbContext;
	readonly calls: Call[];
	readonly errors: string[];
	readonly logged: string[];
	readonly prompts: string[];
	readonly offered: PickItem[][];
	readonly checkpoints: string[];
	/** The answers the quick pick gives, in order; a missing one cancels. */
	picks: (string | undefined)[];
	/** The answers the input box gives, in order; a missing one cancels. */
	typed: (string | undefined)[];
}

/** The tool the wizard is driven against, carrying one of each prompt kind. */
const WIZARD_TOOL = {
	name: "move",
	description: "moves a card",
	inputSchema: {
		type: "object",
		properties: {
			card: { type: "string", description: "the card" },
			column: {
				type: "string",
				description: "the destination",
				"x-dinah-vocabulary-source": "columns",
			},
			note: { type: "string", description: "an optional note" },
			override: { type: "boolean", description: "force the move" },
		},
		required: ["card"],
	},
};

function rpc(id: number, result: unknown): string {
	return `${JSON.stringify({ jsonrpc: "2.0", id, result })}\n`;
}

/**
 * A driver over a fake server.
 *
 * The spawner reads the request off the stdin the wizard wrote, so what the
 * test asserts on is the bytes the process would have received rather than an
 * argument list a helper remembered.
 */
function driver(columns: ColumnRow[], callResult: unknown = { outcome: "ok" }): Driver {
	const calls: Call[] = [];
	const errors: string[] = [];
	const logged: string[] = [];
	const prompts: string[] = [];
	const offered: PickItem[][] = [];
	const checkpoints: string[] = [];
	const state = {
		calls,
		errors,
		logged,
		prompts,
		offered,
		checkpoints,
		picks: [],
		typed: [],
	} as unknown as Driver;

	const spawner: Spawner = async (_exe, _argv, options: SpawnOptions) => {
		const lines = (options.stdin ?? "").split("\n").filter((line) => line !== "");
		const request = lines
			.map((line) => JSON.parse(line) as { id: number; method: string; params?: Record<string, unknown> })
			.find((line) => line.id === REQUEST_ID);
		assert.ok(request !== undefined, "the wizard wrote no request line");
		const params = request.params ?? {};
		calls.push({
			method: request.method,
			name: typeof params["name"] === "string" ? params["name"] : undefined,
			arguments: params["arguments"] as Record<string, unknown> | undefined,
		});
		if (request.method === "tools/list") {
			return {
				code: 0,
				stdout: rpc(1, {}) + rpc(REQUEST_ID, { tools: [WIZARD_TOOL] }),
				stderr: "",
			};
		}
		if (params["name"] === "columns") {
			return {
				code: 0,
				stdout: rpc(1, {}) + rpc(REQUEST_ID, { columns }),
				stderr: "",
			};
		}
		return { code: 0, stdout: rpc(1, {}) + rpc(REQUEST_ID, callResult), stderr: "" };
	};

	const host: CommandHost = {
		t: ENGLISH,
		showError: (message) => errors.push(message),
		showInfo: () => undefined,
		copyToClipboard: async () => undefined,
		pick: async (items) => {
			offered.push([...items]);
			const wanted = state.picks.shift();
			return wanted === undefined
				? undefined
				: items.find((item) => item.value === wanted || item.label === wanted);
		},
		input: async (prompt) => {
			prompts.push(prompt);
			return state.typed.shift();
		},
		openDocument: async () => undefined,
		openFile: async () => undefined,
		pickFile: async () => undefined,
		openServedText: async () => undefined,
		checkpoint: async (folder) => {
			checkpoints.push(folder);
		},
		log: (line) => logged.push(line),
	};

	(state as { context: RunVerbContext }).context = {
		spawner,
		exe: "dinah",
		host,
		folder: "C:\\work\\bench",
		root: "C:\\work\\bench",
		catalog: new VerbCatalog({
			spawner,
			exe: "dinah",
			log: (line) => logged.push(line),
		}),
	};
	return state;
}

/** The tools/call the wizard made for the verb itself. */
function verbCall(d: Driver): Call | undefined {
	return d.calls.find((call) => call.method === "tools/call" && call.name === "move");
}

// ---------------------------------------------------------------------------
// AC5: what an answered wizard sends, and what it leaves out
// ---------------------------------------------------------------------------

test("the composed call carries what was answered and omits what was not", async () => {
	const d = driver([
		{ id: "c1", slug: "triage", title: "Triage" },
		{ id: "c2", slug: "build", title: "Build" },
		{ id: "c3", slug: "done", title: "Done" },
	]);
	// Verb, then card, then the column, then the note left blank, then No.
	d.picks = ["move", "build", "No"];
	d.typed = ["dn-12", ""];
	await runVerbFromPalette(d.context);

	const call = verbCall(d);
	assert.ok(call !== undefined, "the wizard made no call for the verb");
	assert.deepEqual(call.arguments, { card: "dn-12", column: "build" });
	// Named rather than left to the deepEqual above, because the defect this
	// guards is a key present with an empty or false value, and a reader of
	// the failure should be told which shape arrived.
	assert.ok(!("note" in (call.arguments ?? {})), "the blank optional was sent");
	assert.ok(!("override" in (call.arguments ?? {})), "the declined marker was sent");
	assert.deepEqual(d.errors, []);
	assert.deepEqual(d.checkpoints, ["C:\\work\\bench"]);
});

test("the column choice is offered by title and sent by slug", async () => {
	const d = driver([
		{ id: "c1", slug: "triage", title: "Triage" },
		{ id: "c2", slug: "build", title: "Build" },
	]);
	d.picks = ["move", "Build", "No"];
	d.typed = ["dn-12", ""];
	await runVerbFromPalette(d.context);

	const columnPick = d.offered[1];
	assert.deepEqual(
		columnPick.map((item) => [item.label, item.value]),
		[
			["Triage", "triage"],
			["Build", "build"],
		],
	);
	assert.equal(verbCall(d)?.arguments?.["column"], "build");
});

// ---------------------------------------------------------------------------
// AC6: a column with no slug travels as its id
// ---------------------------------------------------------------------------

test("a column written before the slug migration is sent as its id", async () => {
	const d = driver([{ id: "c9", slug: "", title: "Older" }]);
	d.picks = ["move", "Older", "No"];
	d.typed = ["dn-12", ""];
	await runVerbFromPalette(d.context);
	assert.equal(verbCall(d)?.arguments?.["column"], "c9");
});

// ---------------------------------------------------------------------------
// AC7: a required value cannot be answered blank
// ---------------------------------------------------------------------------

test("a required argument answered with spaces reopens the same prompt", async () => {
	const d = driver([{ id: "c1", slug: "build", title: "Build" }]);
	d.picks = ["move", "build", "No"];
	d.typed = ["   ", "dn-12", ""];
	await runVerbFromPalette(d.context);

	const required = d.prompts.filter((prompt) => prompt.includes("card"));
	assert.equal(required.length, 2, `the prompt was not reopened: ${d.prompts.join(" | ")}`);
	assert.equal(required[0], required[1]);
	assert.equal(verbCall(d)?.arguments?.["card"], "dn-12");
});

test("a required argument answered once is asked once", async () => {
	const d = driver([{ id: "c1", slug: "build", title: "Build" }]);
	d.picks = ["move", "build", "No"];
	d.typed = ["  dn-12  ", ""];
	await runVerbFromPalette(d.context);
	assert.equal(d.prompts.filter((prompt) => prompt.includes("card")).length, 1);
	// Trimmed on the way out, so a reader's stray space is not part of the
	// reference the verb is given.
	assert.equal(verbCall(d)?.arguments?.["card"], "dn-12");
});

test("a cancelled prompt stops the wizard without calling the verb", async () => {
	const d = driver([{ id: "c1", slug: "build", title: "Build" }]);
	d.picks = ["move"];
	d.typed = [undefined];
	await runVerbFromPalette(d.context);
	assert.equal(verbCall(d), undefined);
	assert.deepEqual(d.checkpoints, []);
});

// ---------------------------------------------------------------------------
// AC8: which payloads are a failure, and which are a success
// ---------------------------------------------------------------------------

test("a refused outcome is a failure carrying its refusal and detail", () => {
	assert.deepEqual(
		classifyCallResult({
			outcome: "refused",
			refusal: "dinah.wip-limit-reached",
			detail: "build",
		}),
		{ kind: "failed", message: "dinah.wip-limit-reached: build" },
	);
});

test("an ok outcome and a payload with no outcome at all are both success", () => {
	assert.deepEqual(classifyCallResult({ outcome: "ok" }), { kind: "ok" });
	// A read tool wraps its answer in an object of its own and carries no
	// outcome member on success, so the absence has to read as success rather
	// than as a payload nobody could classify.
	assert.deepEqual(classifyCallResult({ columns: [], affordances: [] }), {
		kind: "ok",
	});
});

test("a refusal reaches the reader through the same message a CLI refusal does", async () => {
	const d = driver([{ id: "c1", slug: "build", title: "Build" }], {
		outcome: "refused",
		refusal: "dinah.not-claimed",
		detail: "dn-12",
	});
	d.picks = ["move", "build", "No"];
	d.typed = ["dn-12", ""];
	await runVerbFromPalette(d.context);
	assert.deepEqual(d.errors, ["dinah.not-claimed: dn-12"]);
	// The checkpoint runs on a refusal too, because a refusal often means the
	// board moved under the reader.
	assert.deepEqual(d.checkpoints, ["C:\\work\\bench"]);
});

test("a successful call says nothing and repaints the tree", async () => {
	const d = driver([{ id: "c1", slug: "build", title: "Build" }]);
	d.picks = ["move", "build", "No"];
	d.typed = ["dn-12", ""];
	await runVerbFromPalette(d.context);
	assert.deepEqual(d.errors, []);
	assert.deepEqual(d.checkpoints, ["C:\\work\\bench"]);
});

// ---------------------------------------------------------------------------
// The resolution that failed, and the enumeration that did
// ---------------------------------------------------------------------------

test("a failed vocabulary lookup names the argument and runs no verb", async () => {
	const d = driver([], { outcome: "ok" });
	// The columns call answers a refusal rather than a list, which is a
	// failure of the lookup rather than a workbench with no columns.
	const failing = driver([]);
	const context: RunVerbContext = {
		...failing.context,
		spawner: async (_exe, _argv, options) => {
			const request = (options.stdin ?? "")
				.split("\n")
				.filter((line) => line !== "")
				.map((line) => JSON.parse(line) as { id: number; method: string; params?: Record<string, unknown> })
				.find((line) => line.id === REQUEST_ID);
			if (request?.method === "tools/list") {
				return {
					code: 0,
					stdout: rpc(1, {}) + rpc(REQUEST_ID, { tools: [WIZARD_TOOL] }),
					stderr: "",
				};
			}
			return {
				code: 0,
				stdout:
					rpc(1, {}) +
					rpc(REQUEST_ID, {
						outcome: "unreachable",
						refusal: "dinah.no-workbench",
						detail: "nothing here",
					}),
				stderr: "",
			};
		},
	};
	const rebuilt = new VerbCatalog({
		spawner: context.spawner,
		exe: "dinah",
		log: (line) => failing.logged.push(line),
	});
	failing.picks = ["move"];
	failing.typed = ["dn-12"];
	await runVerbFromPalette({ ...context, catalog: rebuilt });

	assert.equal(failing.errors.length, 1);
	assert.ok(
		failing.errors[0].includes("column"),
		`the message names no argument: ${failing.errors[0]}`,
	);
	assert.ok(
		failing.logged.some((line) => line.includes("move") && line.includes("column")),
		`no channel line named the verb and the argument: ${failing.logged.join(" | ")}`,
	);
	assert.deepEqual(d.calls, []);
});

test("an enumeration that failed shows a message and opens no picker", async () => {
	const d = driver([]);
	const context: RunVerbContext = {
		...d.context,
		catalog: new VerbCatalog({
			spawner: async () => ({
				code: null,
				stdout: "",
				stderr: "",
				spawnError: { code: "ENOENT", message: "spawn dinah ENOENT" },
			}),
			exe: "dinah",
			log: (line) => d.logged.push(line),
		}),
	};
	await runVerbFromPalette(context);
	assert.equal(d.errors.length, 1);
	assert.deepEqual(d.offered, []);
	assert.ok(
		d.logged.some((line) => line.includes("ENOENT")),
		`the channel carries no detail: ${d.logged.join(" | ")}`,
	);
});
