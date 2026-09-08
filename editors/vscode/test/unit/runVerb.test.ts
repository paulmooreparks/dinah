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

/**
 * A tool carrying one argument its caller supplies and three the head does.
 *
 * `dinah mcp` fills in the acting name, the claim basis and the workbench on
 * every call, and marks each of those properties with `x-dinah-injected` so a
 * client can tell them from the arguments a reader has to answer. This is the
 * shape every served tool really has, reduced to one ordinary argument so the
 * count of prompts is a number a failure can name (dinah-420 AC15).
 */
const INJECTED_TOOL = {
	name: "move",
	description: "moves a card",
	inputSchema: {
		type: "object",
		properties: {
			card: { type: "string", description: "the card" },
			actor: {
				type: "string",
				description: "who is acting",
				"x-dinah-injected": true,
			},
			basis: {
				type: "string",
				description: "the claim basis",
				"x-dinah-injected": true,
			},
			workbench: {
				type: "string",
				description: "the workbench",
				"x-dinah-injected": true,
			},
		},
		required: ["card"],
	},
};

function rpc(id: number, result: unknown): string {
	return `${JSON.stringify({ jsonrpc: "2.0", id, result })}\n`;
}

/**
 * A tool whose only two arguments are optional and constrained, which is the
 * shape nine served tools have and the shape the wizard could not compose
 * before dinah-420's code review.
 *
 * `kind` is a fixed set, as `new_column`'s own is, and `fields` is a
 * comma-separated list drawn from a fixed set, as `show`'s is. Neither is
 * required, so the ordinary invocation of this tool sends neither.
 */
const OPTIONAL_TOOL = {
	name: "show",
	description: "reads a card",
	inputSchema: {
		type: "object",
		properties: {
			kind: {
				type: "string",
				description: "which kind",
				enum: ["flow", "queue"],
			},
			fields: {
				type: "string",
				description: "which fields",
				"x-dinah-value-list": true,
				"x-dinah-vocabulary-members": ["card", "body", "links"],
			},
		},
	},
};

/**
 * A driver over a fake server.
 *
 * The spawner reads the request off the stdin the wizard wrote, so what the
 * test asserts on is the bytes the process would have received rather than an
 * argument list a helper remembered.
 */
function driver(
	columns: ColumnRow[],
	callResult: unknown = { outcome: "ok" },
	served: readonly unknown[] = [WIZARD_TOOL],
): Driver {
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
				stdout: rpc(1, {}) + rpc(REQUEST_ID, { tools: served }),
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
	// The leave-out row comes first because the argument is optional, and the
	// resolved rows follow in the order the columns tool answered with.
	assert.deepEqual(
		columnPick.slice(1).map((item) => [item.label, item.value]),
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

// ---------------------------------------------------------------------------
// An optional argument the schema constrains can still be left out
// ---------------------------------------------------------------------------

/** The tools/call the wizard made for the optional-argument tool. */
function showCall(d: Driver): Call | undefined {
	return d.calls.find((call) => call.method === "tools/call" && call.name === "show");
}

test("an optional argument with a fixed set is offered a way to be left out", async () => {
	const d = driver([], { outcome: "ok" }, [OPTIONAL_TOOL]);
	// The verb, then the leave-out row on the fixed set, then a blank answer
	// to the list, which is that prompt's own way of declining.
	d.picks = ["show", ENGLISH("dialog.runVerb.omitOptional", { argument: "kind" })];
	d.typed = [""];
	await runVerbFromPalette(d.context);

	const call = showCall(d);
	assert.ok(call !== undefined, "the wizard ran no call, so the verb is unreachable");
	assert.deepEqual(
		call.arguments,
		{},
		"an argument the reader declined reached the call",
	);
	assert.deepEqual(d.errors, []);
	assert.deepEqual(d.checkpoints, [d.context.folder]);
});

test("the leave-out row is offered first and only where the schema allows it", async () => {
	const d = driver([], { outcome: "ok" }, [OPTIONAL_TOOL]);
	d.picks = ["show", "flow"];
	d.typed = ["card,body"];
	await runVerbFromPalette(d.context);

	// offered[0] is the verb list; offered[1] is the fixed set, which is the
	// only pick this tool opens, because the list argument is typed.
	const kindPick = d.offered[1];
	assert.equal(
		kindPick[0].label,
		ENGLISH("dialog.runVerb.omitOptional", { argument: "kind" }),
	);
	assert.deepEqual(
		kindPick.slice(1).map((item) => item.value),
		["flow", "queue"],
	);
	assert.deepEqual(showCall(d)?.arguments, { kind: "flow", fields: "card,body" });
});

test("a required argument with a fixed set is offered no way to be left out", async () => {
	const required = {
		...OPTIONAL_TOOL,
		inputSchema: { ...OPTIONAL_TOOL.inputSchema, required: ["kind"] },
	};
	const d = driver([], { outcome: "ok" }, [required]);
	d.picks = ["show", "flow"];
	d.typed = [""];
	await runVerbFromPalette(d.context);

	assert.deepEqual(
		d.offered[1].map((item) => item.value),
		["flow", "queue"],
	);
	assert.deepEqual(showCall(d)?.arguments, { kind: "flow" });
});

test("an optional runtime-vocabulary argument can be left out too", async () => {
	const d = driver([
		{ id: "c1", slug: "triage", title: "Triage" },
		{ id: "c2", slug: "build", title: "Build" },
	]);
	// The verb, the leave-out row on the column, then No to the marker.
	d.picks = [
		"move",
		ENGLISH("dialog.runVerb.omitOptional", { argument: "column" }),
		"No",
	];
	d.typed = ["dn-12", ""];
	await runVerbFromPalette(d.context);

	const call = verbCall(d);
	assert.ok(call !== undefined, "the wizard ran no call for the verb");
	assert.deepEqual(call.arguments, { card: "dn-12" });
});

test("the list prompt names the members a reader may combine", async () => {
	const d = driver([], { outcome: "ok" }, [OPTIONAL_TOOL]);
	d.picks = ["show", ENGLISH("dialog.runVerb.omitOptional", { argument: "kind" })];
	d.typed = ["card,body"];
	await runVerbFromPalette(d.context);

	const listPrompt = d.prompts.find((prompt) => prompt.includes("fields"));
	assert.ok(listPrompt !== undefined, `no prompt named fields: ${d.prompts.join(" | ")}`);
	for (const member of ["card", "body", "links"]) {
		assert.ok(
			listPrompt.includes(member),
			`the prompt hides the member ${member}: ${listPrompt}`,
		);
	}
	// Two members together is the value the published enum had made illegal,
	// and it travels as the reader typed it.
	assert.deepEqual(showCall(d)?.arguments, { fields: "card,body" });
});

// ---------------------------------------------------------------------------
// AC15: the head's own plumbing is never a question the reader is asked
// ---------------------------------------------------------------------------

test("a property the head fills in is neither prompted for nor sent", async () => {
	const d = driver([], { outcome: "ok" }, [INJECTED_TOOL]);
	d.picks = ["move"];
	d.typed = ["dn-12"];
	await runVerbFromPalette(d.context);

	// One prompt, not four. The count is the assertion because the defect is a
	// reader being asked to type a claim basis, and a wizard that asked for all
	// three and dropped the answers would still compose the right call.
	assert.deepEqual(
		d.prompts.length,
		1,
		`the wizard raised ${String(d.prompts.length)} prompts: ${d.prompts.join(" | ")}`,
	);
	assert.ok(
		d.prompts[0].includes("card"),
		`the one prompt was not the card: ${d.prompts[0]}`,
	);

	const call = verbCall(d);
	assert.ok(call !== undefined, "the wizard made no call for the verb");
	assert.deepEqual(call.arguments, { card: "dn-12" });
	assert.deepEqual(d.errors, []);
});

test("holding the head's plumbing back never costs the reader the verb", async () => {
	// The other direction. A skipped property that had also been counted as an
	// argument this build cannot draw would take the verb out of the palette
	// altogether, and the reader would be told a count with no verb behind it.
	const d = driver([], { outcome: "ok" }, [INJECTED_TOOL]);
	d.picks = ["move"];
	d.typed = ["dn-12"];
	await runVerbFromPalette(d.context);

	assert.deepEqual(
		d.offered[0].map((item) => item.value),
		["move"],
		"the verb list is not the one tool the fixture served",
	);
	assert.deepEqual(d.logged, [], "an exclusion was logged for injected plumbing");
});
