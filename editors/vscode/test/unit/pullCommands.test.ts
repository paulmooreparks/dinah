// What the queue column's Pull sends, and what it says about each answer.
//
// The argv assertion carries the same weight it does in cardCommands.test.ts
// and creationCommands.test.ts, and one thing here is peculiar to this verb:
// the positional word is the DESTINATION column rather than the queue that was
// clicked, because `dinah pull` names where the card lands and picks the card
// itself. A command that sent the queue's own reference would pull out of the
// wrong column against somebody's real workbench and nowhere else.
//
// The three answers are the rest. A refusal is the interesting case rather
// than the exception, an ok answer carrying no card is a click that changed
// nothing and has to say so, and an ok answer carrying a card says nothing at
// all beyond the repaint.

import assert from "node:assert/strict";
import { test } from "node:test";

import type { CommandHost } from "../../src/cardCommands";
import { refusalMessage } from "../../src/cardCommands";
import type { SpawnOutcome, Spawner } from "../../src/cli";
import { contextForPull, emptyPullMessage, pullFromColumn } from "../../src/pullCommands";
import type { RootRow, TreeElement } from "../../src/tree";
import type { CardView, ColumnView, TreeNode } from "../../src/wire";

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

/**
 * The workbench root and the workspace folder the row was produced for, kept
 * apart for the reason creationCommands.test.ts keeps them apart: the argv is
 * pinned to ROOT and the checkpoint is asked for FOLDER, so a swap reddens.
 */
const ROOT = "C:\\work\\bench";
const FOLDER = "C:\\work\\bench\\editors";
const EXE = "C:\\tools\\dinah.exe";

/** The queue's own reference, which must never reach the argv. */
const QUEUE_SLUG = "design-queue";

/** What the queue publishes as its pull destination, which must. */
const DESTINATION = "spec";

function ok(payload: unknown): SpawnOutcome {
	return { code: 0, stdout: JSON.stringify(payload), stderr: "" };
}

interface Invocation {
	readonly exe: string;
	readonly argv: string[];
	readonly cwd?: string;
}

interface Recorder {
	readonly spawner: Spawner;
	readonly host: CommandHost;
	readonly calls: Invocation[];
	readonly errors: string[];
	readonly infos: string[];
	readonly checkpoints: string[];
	/**
	 * Every host method this command is supposed to leave alone, named as it
	 * was called.
	 *
	 * A double answering a method with undefined and recording nothing cannot
	 * notice the command starting to call it, so D-4 (no confirmation dialog,
	 * no toast on an ordinary pull) would have nothing holding it. The prompts
	 * are here for that: a Pull that grew a confirmation would reach `pick` or
	 * `input` and this is what says so.
	 */
	readonly unused: string[];
	/** What the next spawn answers, whatever the argv says. */
	answer: SpawnOutcome;
}

function recorder(): Recorder {
	const calls: Invocation[] = [];
	const errors: string[] = [];
	const infos: string[] = [];
	const checkpoints: string[] = [];
	const unused: string[] = [];
	const state: Recorder = {
		calls,
		errors,
		infos,
		checkpoints,
		unused,
		answer: ok({ outcome: "ok", card: { id: "aaa" } }),
		spawner: async (exe, argv, options) => {
			calls.push({ exe, argv: [...argv], cwd: options.cwd });
			return state.answer;
		},
		host: {
			showError: (message) => {
				errors.push(message);
			},
			showInfo: (message) => {
				infos.push(message);
			},
			copyToClipboard: async () => {
				unused.push("copyToClipboard");
			},
			pick: async () => {
				unused.push("pick");
				return undefined;
			},
			input: async () => {
				unused.push("input");
				return undefined;
			},
			openDocument: async () => {
				unused.push("openDocument");
			},
			openFile: async () => {
				unused.push("openFile");
			},
			pickFile: async () => {
				unused.push("pickFile");
				return undefined;
			},
			checkpoint: async (folder) => {
				checkpoints.push(folder);
			},
			log: () => undefined,
		},
	};
	return state;
}

function rowFixture(over: Partial<RootRow> = {}): RootRow {
	return {
		rowKind: "workbenchRoot",
		folder: FOLDER,
		folderName: "bench",
		description: "",
		sole: false,
		data: {
			path: ROOT,
			title: "Trees",
			columns: new Map(),
			cards: new Map(),
		},
		...over,
	};
}

function columnView(over: Partial<ColumnView> = {}): ColumnView {
	return {
		id: "0a1b2c3d4e5f",
		slug: QUEUE_SLUG,
		title: "Design Queue",
		kind: "queue",
		operator_owned: false,
		awaiting_outside: false,
		takes_work_up: false,
		pull_destination: DESTINATION,
		count: 0,
		...over,
	};
}

function columnNode(): TreeNode {
	return { kind: "group", axis: "column", value: QUEUE_SLUG, count: 0 };
}

function columnRow(
	over: Partial<ColumnView> | undefined,
	row = rowFixture(),
): TreeElement {
	return {
		kind: "column",
		row,
		node: columnNode(),
		view: over === undefined ? undefined : columnView(over),
	};
}

function cardRow(): TreeElement {
	return {
		kind: "card",
		row: rowFixture(),
		node: { kind: "card", id: "aaa", ref: "tr-4", title: "Draw the guides", count: 1 },
		view: { id: "aaa", ref: "tr-4" } as CardView,
	};
}

/** The context a pullable queue row resolves to, built once for the act tests. */
function pullable(r: Recorder) {
	const context = contextForPull(columnRow({}), EXE, r.host, r.spawner);
	assert.notEqual(context, undefined, "the fixture row is supposed to be pullable");
	return context as NonNullable<ReturnType<typeof contextForPull>>;
}

// ---------------------------------------------------------------------------
// AC-2: which rows Pull can be aimed at
// ---------------------------------------------------------------------------

test("Pull resolves a context from a pullable queue row and from nothing else", () => {
	// dinah-375 AC-2. The undefined element is the row dinah-342 was filed
	// over: a command reached from the Command Palette, a keybinding or
	// another extension arrives with no argument at all, and a builder that
	// read a field off it before checking would throw.
	//
	// The last two rows are this card's own scope decision rather than a
	// defensive check. A menu clause already hides the item on a work column,
	// but a clause is not a guard: nothing about a keybinding consults it.
	const cases: [string, TreeElement | undefined][] = [
		["no element at all, which is what the Command Palette passes", undefined],
		["a workbench root row", { kind: "root", row: rowFixture() }],
		["a card row", cardRow()],
		["a column the status/tree join missed", columnRow(undefined)],
		["a work column, whatever destination it publishes", columnRow({ takes_work_up: true })],
		[
			"a work column at the head of two claim-taking columns",
			columnRow({ takes_work_up: true, pull_destination: "review" }),
		],
		["a queue whose destination is absent", columnRow({ pull_destination: undefined })],
		["a queue whose destination is the empty string", columnRow({ pull_destination: "" })],
	];
	for (const [name, element] of cases) {
		const r = recorder();
		assert.equal(contextForPull(element, EXE, r.host, r.spawner), undefined, name);
	}
});

test("a queue row whose workbench path is missing names no context either", () => {
	// The path pins the call to a workbench. Without one the spawn would run
	// against whatever directory the editor happened to be standing in, which
	// on this product is a different operator's live board.
	const r = recorder();
	const row = rowFixture({ data: undefined });
	assert.equal(contextForPull(columnRow({}, row), EXE, r.host, r.spawner), undefined);
});

test("the context a queue row resolves to carries the destination, not the queue", () => {
	// dinah-375 AC-2. The destination is what the verb takes, and the queue's
	// own reference is deliberately not on the context at all: there is no
	// field a later edit could send by mistake.
	const r = recorder();
	const context = pullable(r);
	assert.equal(context.destination, DESTINATION);
	assert.equal(context.label, "Design Queue");
	assert.equal(context.root, ROOT);
	assert.equal(context.folder, FOLDER);
	assert.equal(context.exe, EXE);
});

// ---------------------------------------------------------------------------
// AC-3: what Pull sends
// ---------------------------------------------------------------------------

test("Pull sends the destination and --no-claim, pinned to the queue's own workbench", () => {
	// dinah-375 AC-3 and D-1. The whole argv is pinned rather than probed for
	// the flag, because a pull that dropped --no-claim would claim the card
	// for whoever right-clicked, and an assertion that only looked for the
	// destination would pass while it did.
	const r = recorder();
	return pullFromColumn(pullable(r)).then(() => {
		assert.equal(r.calls.length, 1);
		assert.deepEqual(r.calls[0].argv, [
			"--json",
			"--workbench",
			ROOT,
			"pull",
			DESTINATION,
			"--no-claim",
		]);
		assert.equal(r.calls[0].exe, EXE);
		assert.equal(r.calls[0].cwd, ROOT);
		assert.ok(
			!r.calls[0].argv.includes(QUEUE_SLUG),
			`the queue's own reference reached the argv: ${r.calls[0].argv.join(" ")}`,
		);
	});
});

// ---------------------------------------------------------------------------
// AC-4: a refusal is shown in the workbench's own words
// ---------------------------------------------------------------------------

test("a refused pull shows the refusal the workbench gave and checkpoints anyway", async () => {
	// dinah-375 AC-4. A work limit, a tier ceiling, a gate, an unresolved item
	// on a claiming pull and a card already held all arrive as this envelope,
	// so one case drives the branch and the message is compared against
	// refusalMessage's own composition rather than against a sentence spelled
	// twice, which would pass whatever the command showed.
	const r = recorder();
	r.answer = {
		code: 2,
		stdout: JSON.stringify({
			refusal: "work_limit",
			detail: "Spec holds 3 cards against a limit of 3",
		}),
		stderr: "",
	};
	const outcome = await pullFromColumn(pullable(r));
	assert.equal(outcome.kind, "refused");
	assert.deepEqual(r.errors, [refusalMessage(outcome)]);
	assert.ok(
		r.errors[0].includes("work_limit") &&
			r.errors[0].includes("Spec holds 3 cards against a limit of 3"),
		`the refusal reached the reader stripped of its own words: ${r.errors[0]}`,
	);
	assert.deepEqual(r.infos, []);
	// The board moved under the reader often enough that the read which shows
	// them why is exactly the one a refusal needs, so the checkpoint runs here
	// as it does on every other mutating command.
	assert.deepEqual(r.checkpoints, [FOLDER]);
});

// ---------------------------------------------------------------------------
// AC-5: the two ok answers, which do not look alike
// ---------------------------------------------------------------------------

test("an ok pull that took nothing says which column was looked in", async () => {
	// dinah-375 AC-5. okEmpty answers at exit 0 with no card, so a command
	// reading only the exit code would report a click that changed nothing as
	// a click that worked. The two titles come off message_values rather than
	// off the row, because the upstream is a column the reader did not click.
	const r = recorder();
	r.answer = ok({
		outcome: "ok",
		message: "answer.pull.empty.named",
		message_values: { upstream: "Design Queue", destination: "Spec" },
	});
	await pullFromColumn(pullable(r));
	assert.deepEqual(r.infos, ["Nothing in Design Queue is ready to pull into Spec."]);
	assert.deepEqual(r.errors, []);
	assert.deepEqual(r.checkpoints, [FOLDER]);
});

test("an ok pull that took nothing names the destination the argv was built from, never the queue twice", async () => {
	// The values are omitempty on the Go side, so this pins which column the
	// caller hands each half of the fallback rather than only that the
	// fallback exists. Passing the queue's own title for both would compose
	// "Nothing in Design Queue is ready to pull into Design Queue.", which
	// tells the reader the card goes back where it already is.
	const r = recorder();
	r.answer = ok({ outcome: "ok", message: "answer.pull.empty.named" });
	await pullFromColumn(pullable(r));
	assert.deepEqual(r.infos, [
		`Nothing in Design Queue is ready to pull into ${DESTINATION}.`,
	]);
	assert.notEqual(
		r.infos[0],
		"Nothing in Design Queue is ready to pull into Design Queue.",
	);
	assert.deepEqual(r.errors, []);
	assert.deepEqual(r.checkpoints, [FOLDER]);
});

test("an ok pull that took a card says nothing at all", async () => {
	// dinah-375 D-4. Claim, Release, Move and Unblock all report nothing on
	// success, and the card's arrival in its new column is the trace a pull
	// leaves. A toast here would be the odd one out.
	const r = recorder();
	r.answer = ok({ outcome: "ok", card: { id: "aaa", ref: "tr-4", column: "spec" } });
	await pullFromColumn(pullable(r));
	assert.deepEqual(r.infos, []);
	assert.deepEqual(r.errors, []);
	assert.deepEqual(r.checkpoints, [FOLDER]);
});

test("Pull opens no prompt of any kind", async () => {
	// dinah-375 D-4, the other half. Pull needs no further argument once the
	// destination is read off the row, so it fires immediately as Claim,
	// Release and Unblock do, and only Block and Move prompt because each
	// prompt supplies an argument the verb requires.
	const r = recorder();
	await pullFromColumn(pullable(r));
	assert.deepEqual(r.unused, []);
});

test("each half of the empty-pull sentence falls back to its own column, never to a gap and never to the other", () => {
	// The values are omitempty on the Go side, so a future answer could carry
	// neither. The queue that was clicked is the destination's upstream, so it
	// is the honest fallback for the first name; the second falls back to the
	// destination the argv was built from, unresolved to a title but naming
	// the column the card would land in. Falling back to the queue for both
	// halves would say the card goes back where it already is, which is the
	// one sentence this test exists to refuse.
	assert.equal(
		emptyPullMessage({}, "Design Queue", "spec"),
		"Nothing in Design Queue is ready to pull into spec.",
	);
	assert.equal(
		emptyPullMessage(
			{ message_values: { upstream: "Intake" } },
			"Design Queue",
			"spec",
		),
		"Nothing in Intake is ready to pull into spec.",
	);
	assert.equal(
		emptyPullMessage(
			{ message_values: { destination: "Spec" } },
			"Design Queue",
			"spec",
		),
		"Nothing in Design Queue is ready to pull into Spec.",
	);
});
