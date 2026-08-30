// What each command sends, and what it says when dinah refuses.
//
// The argv assertions are the point. A verb that takes a card reference and
// gets a title, or takes a column identifier and gets the reference a person
// types, fails at run time against somebody's real board and nowhere else,
// and dinah-287 renamed exactly the field the move verb reads.

import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

import type { SpawnOutcome, Spawner } from "../../src/cli";
import type { CommandContext, CommandHost, PickItem } from "../../src/cardCommands";
import {
	blockCard,
	claimCard,
	contextFor,
	moveCard,
	movePick,
	openAttachment,
	openCard,
	orderLegalMoves,
	refusalMessage,
	releaseCard,
	unblockCard,
} from "../../src/cardCommands";
import type { RootRow, TreeElement } from "../../src/tree";
import type { LegalMove } from "../../src/wire";

interface Recorder {
	readonly context: CommandContext;
	readonly calls: string[][];
	readonly errors: string[];
	readonly checkpoints: string[];
	readonly opened: string[];
	/** The files handed to the host's file opener, in call order. */
	readonly files: string[];
	readonly logged: string[];
	/** The host the recorder watches, which the attachment handler reaches directly. */
	readonly host: CommandHost;
	/** What the next spawn answers, by which verb the argv names. */
	answers: Record<string, SpawnOutcome>;
	/** What the quick-pick returns, if it is opened. */
	picked?: PickItem;
	/** The items the quick-pick was offered. */
	offered: PickItem[];
	/** What the input box returns, if it is opened. */
	typed?: string;
}

function ok(payload: unknown): SpawnOutcome {
	return { code: 0, stdout: JSON.stringify(payload), stderr: "" };
}

function refused(refusal: string, detail?: string): SpawnOutcome {
	return { code: 2, stdout: JSON.stringify({ refusal, detail }), stderr: "" };
}

function recorder(answers: Record<string, SpawnOutcome> = {}): Recorder {
	const calls: string[][] = [];
	const errors: string[] = [];
	const checkpoints: string[] = [];
	const opened: string[] = [];
	const files: string[] = [];
	const logged: string[] = [];
	const offered: PickItem[] = [];
	const state = {
		calls,
		errors,
		checkpoints,
		opened,
		files,
		logged,
		offered,
		answers,
	} as Recorder;

	const spawner: Spawner = async (_exe, argv) => {
		calls.push([...argv]);
		for (const [verb, outcome] of Object.entries(state.answers)) {
			if (argv.includes(verb)) {
				return outcome;
			}
		}
		return ok({});
	};

	const host: CommandHost = {
		showError: (message) => errors.push(message),
		pick: async (items) => {
			offered.push(...items);
			return state.picked;
		},
		input: async () => state.typed,
		openDocument: async (path) => {
			opened.push(path);
		},
		openFile: async (path) => {
			files.push(path);
		},
		checkpoint: async (folder) => {
			checkpoints.push(folder);
		},
		log: (line) => logged.push(line),
	};

	(state as { context: CommandContext }).context = {
		spawner,
		exe: "dinah",
		host,
		folder: "C:\\work\\bench",
		root: "C:\\work\\bench",
		ref: "tr-4",
	};
	// The host is built after the state it closes over, so it is attached here
	// rather than in the literal: the attachment handler takes the host alone,
	// with no CommandContext around it.
	(state as { host: CommandHost }).host = host;
	return state;
}

// ---------------------------------------------------------------------------
// AC-11: a refusal is surfaced, and a checkpoint follows it
// ---------------------------------------------------------------------------

test("a claim refused for awaiting-outside names the refusal and its detail", async () => {
	const r = recorder({
		claim: refused(
			"dinah.awaiting-outside",
			"this column waits on somebody outside the workbench",
		),
	});
	await claimCard(r.context);
	assert.equal(r.errors.length, 1);
	assert.ok(r.errors[0].includes("dinah.awaiting-outside"));
	assert.ok(r.errors[0].includes("waits on somebody outside"));
	// Exactly one off-cycle check follows, which is what shows the reader the
	// board as it now stands rather than as they believed it stood.
	assert.deepEqual(r.checkpoints, ["C:\\work\\bench"]);
});

test("a refusal is never swallowed, whichever verb raised it", async () => {
	for (const [verb, run] of [
		["release", releaseCard],
		["unblock", unblockCard],
	] as const) {
		const r = recorder({ [verb]: refused("dinah.not-held") });
		await run(r.context);
		assert.deepEqual(r.errors, ["dinah.not-held"]);
		assert.equal(r.checkpoints.length, 1);
	}
});

test("a successful call still checkpoints, and says nothing", async () => {
	const r = recorder({ claim: ok({ affordances: [] }) });
	await claimCard(r.context);
	assert.deepEqual(r.errors, []);
	assert.deepEqual(r.checkpoints, ["C:\\work\\bench"]);
});

test("a refusal with no detail is named by itself", () => {
	assert.equal(
		refusalMessage({ kind: "refused", refusal: "dinah.not-held" }),
		"dinah.not-held",
	);
	assert.equal(
		refusalMessage({ kind: "spawn-failed", detail: "ENOENT" }),
		"spawn-failed: ENOENT",
	);
});

test("every verb is pinned to the workbench the card stands in", async () => {
	const r = recorder();
	await claimCard(r.context);
	assert.deepEqual(r.calls[0], [
		"--json",
		"--workbench",
		"C:\\work\\bench",
		"claim",
		"tr-4",
	]);
});

// ---------------------------------------------------------------------------
// AC-12: the card's own file, read off the answer rather than composed
// ---------------------------------------------------------------------------

test("opening a card uses the path show reported and not one built from the ref", async () => {
	const r = recorder({ show: ok({ path: "C:\\bench\\cards\\abc\\card.md" }) });
	await openCard(r.context);
	assert.deepEqual(r.opened, ["C:\\bench\\cards\\abc\\card.md"]);
	// The reference is what identifies the card to dinah; it is not a path,
	// and a path built from it would be a second spelling of a layout the
	// binary already owns.
	assert.ok(!r.opened[0].includes("tr-4"));
});

test("a show answer carrying no path opens nothing and says so to the channel", async () => {
	const r = recorder({ show: ok({ body: "" }) });
	await openCard(r.context);
	assert.deepEqual(r.opened, []);
	assert.equal(r.logged.length, 1);
});

test("a refused show reports the refusal rather than opening anything", async () => {
	const r = recorder({ show: refused("dinah.unknown-card") });
	await openCard(r.context);
	assert.deepEqual(r.opened, []);
	assert.deepEqual(r.errors, ["dinah.unknown-card"]);
});

// ---------------------------------------------------------------------------
// AC-13: the move quick-pick, its order, and the field it passes
// ---------------------------------------------------------------------------

const MIXED_MOVES: LegalMove[] = [
	{ column: "col-back", ref: "intake", title: "Intake", direction: "backward" },
	{ column: "col-doing", ref: "doing", title: "Doing", direction: "forward" },
	{ column: "col-done", ref: "done", title: "Done", direction: "forward" },
];

test("forward destinations lead in their own array order, then the backward one", () => {
	assert.deepEqual(
		orderLegalMoves(MIXED_MOVES).map((move) => move.title),
		["Doing", "Done", "Intake"],
	);
});

test("invoking the second forward entry moves to that entry's own column", async () => {
	const r = recorder({ instructions: ok({ legal_moves: MIXED_MOVES }) });
	// The second forward entry is Done, whose column identifier is col-done.
	r.picked = movePick(MIXED_MOVES[2]);
	await moveCard(r.context);
	const move = r.calls.find((argv) => argv.includes("move"));
	assert.deepEqual(move, [
		"--json",
		"--workbench",
		"C:\\work\\bench",
		"move",
		"tr-4",
		"col-done",
	]);
});

test("the destination passed is the Column field and not the Ref or the Title", () => {
	// dinah-287 renamed this field from State, and LegalMove carries three
	// strings any of which looks plausible in a quick-pick handler.
	const pick = movePick(MIXED_MOVES[2]);
	assert.equal(pick.value, "col-done");
	assert.notEqual(pick.value, "done");
	assert.notEqual(pick.value, "Done");
	assert.equal(pick.label, "Done");
});

test("the quick-pick is offered the destinations in the order the ordering gives", async () => {
	const r = recorder({ instructions: ok({ legal_moves: MIXED_MOVES }) });
	await moveCard(r.context);
	assert.deepEqual(
		r.offered.map((item) => item.label),
		["Doing", "Done", "Intake"],
	);
	// A backward destination says so, because moving a card back is a
	// different act from moving it on and the list does not otherwise show it.
	assert.equal(r.offered[2].detail, "backward");
	assert.equal(r.offered[0].detail, undefined);
});

test("dismissing the quick-pick moves nothing", async () => {
	const r = recorder({ instructions: ok({ legal_moves: MIXED_MOVES }) });
	r.picked = undefined;
	await moveCard(r.context);
	assert.equal(
		r.calls.find((argv) => argv.includes("move")),
		undefined,
	);
	assert.deepEqual(r.checkpoints, []);
});

test("a card with no legal moves says so rather than opening an empty picker", async () => {
	const r = recorder({ instructions: ok({ legal_moves: [] }) });
	await moveCard(r.context);
	assert.deepEqual(r.offered, []);
	assert.equal(r.errors.length, 1);
	assert.ok(r.errors[0].includes("tr-4"));
});

// ---------------------------------------------------------------------------
// Block, whose reason the verb requires
// ---------------------------------------------------------------------------

test("blocking sends the reason that was typed", async () => {
	const r = recorder();
	r.typed = "  waiting on the printer  ";
	await blockCard(r.context);
	assert.deepEqual(r.calls[0].slice(3), ["block", "tr-4", "waiting on the printer"]);
});

test("an empty reason blocks nothing", async () => {
	// dinah block takes the reason as an argument, and a blank one would
	// record a block nobody can act on.
	for (const typed of [undefined, "", "   "]) {
		const r = recorder();
		r.typed = typed;
		await blockCard(r.context);
		assert.deepEqual(r.calls, []);
		assert.deepEqual(r.checkpoints, []);
	}
});

// ---------------------------------------------------------------------------
// dinah-335 AC-8: an attachment opens its own file, through openFile and
// nothing else on the host
// ---------------------------------------------------------------------------

/** The smallest row an element can carry: resolved, and carrying no data. */
function rowFixture(): RootRow {
	return {
		rowKind: "workbenchRoot",
		folder: "C:\\work\\bench",
		folderName: "bench",
		description: "",
		sole: false,
	};
}

/**
 * An attachment element over that row, with the payload path given
 * explicitly so both spellings of a missing one reach the handler.
 */
function attachmentRow(path: string | undefined): TreeElement {
	return {
		kind: "attachment",
		row: rowFixture(),
		view: {
			id: "9a1b2c3d4e5f",
			ordinal: 1,
			ref: "tr-4/attachments/1",
			filename: "screenshot.png",
			provenance: "copy",
			path,
		},
	};
}

test("an attachment with a file opens it through openFile and touches nothing else on the host", async () => {
	const r = recorder();
	const lines: string[] = [];
	await openAttachment(
		attachmentRow("C:\\bench\\cards\\tr-4\\attachments\\screenshot.png"),
		r.host,
		(line) => lines.push(line),
	);
	assert.deepEqual(r.files, ["C:\\bench\\cards\\tr-4\\attachments\\screenshot.png"]);
	// No other call the host offers was made: no document forced open, no
	// checkpoint spent, no error surface, no picker, and no channel line
	// through the host, which is why the handler's own sayings go through a
	// callback the host does not hold.
	assert.deepEqual(r.opened, []);
	assert.deepEqual(r.checkpoints, []);
	assert.deepEqual(r.errors, []);
	assert.deepEqual(r.offered, []);
	assert.deepEqual(r.logged, []);
	assert.deepEqual(lines, []);
});

test("an attachment with no path opens nothing, calls nothing on the host, and says so once", async () => {
	for (const path of [undefined, ""]) {
		const r = recorder();
		const lines: string[] = [];
		await openAttachment(attachmentRow(path), r.host, (line) => lines.push(line));
		assert.deepEqual(r.files, []);
		assert.deepEqual(r.opened, []);
		assert.deepEqual(r.checkpoints, []);
		assert.deepEqual(r.errors, []);
		assert.deepEqual(r.offered, []);
		assert.deepEqual(r.logged, []);
		assert.equal(lines.length, 1, `the handler said: ${lines.join(" | ")}`);
		assert.ok(
			lines[0].includes("no path"),
			`the row did not say why it opened nothing: ${lines.join(" | ")}`,
		);
	}
});

// ---------------------------------------------------------------------------
// contextFor, and the argument the Command Palette does not pass
// ---------------------------------------------------------------------------

/** A host whose calls are recorded nowhere, since contextFor makes none. */
const silentHost: CommandHost = {
	showError: () => undefined,
	pick: async () => undefined,
	input: async () => undefined,
	openDocument: async () => undefined,
	openFile: async () => undefined,
	checkpoint: async () => undefined,
	log: () => undefined,
};

function rowFor(path: string | undefined): RootRow {
	return {
		rowKind: "workbenchRoot",
		folder: "C:/work",
		folderName: "work",
		description: "",
		sole: true,
		data:
			path === undefined
				? undefined
				: {
						path,
						title: "Work",
						columns: new Map(),
						cards: new Map(),
					},
	};
}

/** A card row of the shape the tree hands a context-menu invocation. */
function cardElement(): TreeElement {
	return {
		kind: "card",
		row: rowFor("C:/work/board"),
		node: { kind: "card", ref: "dinah-1", title: "A card", count: 1 },
	};
}

test("contextFor answers undefined for the argument the Command Palette passes", () => {
	// The palette invokes a command with no argument at all, and every flow
	// command shares one handler that called this function first. Reading a
	// field off the absent element threw a TypeError before the handler's own
	// missing-row branch could run, so the six commands each reported "Running
	// the contributed command failed" and said nothing a reader could act on.
	assert.equal(contextFor(undefined, "dinah", silentHost), undefined);
});

test("contextFor answers undefined for a row that is not a card", () => {
	// A workbench root, a column and a state group are all rows a keybinding or
	// another extension can aim a card command at, and none of them names a
	// card. The handler logs that and returns, which needs this answer rather
	// than a throw.
	const rows: TreeElement[] = [
		{ kind: "root", row: rowFor("C:/work/board") },
		{
			kind: "column",
			row: rowFor("C:/work/board"),
			node: { kind: "column", id: "spec", title: "Spec", count: 2 },
		},
		{
			kind: "group",
			row: rowFor("C:/work/board"),
			node: { kind: "group", axis: "state", value: "ready", count: 2 },
		},
	];
	for (const row of rows) {
		assert.equal(
			contextFor(row, "dinah", silentHost),
			undefined,
			`a ${row.kind} row composed a context`,
		);
	}
});

test("a row that names no attachment at all opens nothing and says which command was misaimed", async () => {
	const r = recorder();
	const lines: string[] = [];
	// A note row is a row of the wrong kind, and the same guard has to fire
	// for every kind the tree composes.
	const wrong: TreeElement = {
		kind: "note",
		owner: rowFixture(),
		text: "nothing to open here",
		tooltip: "nothing to open here",
	};
	await openAttachment(wrong, r.host, (line) => lines.push(line));
	assert.deepEqual(r.files, []);
	assert.equal(lines.length, 1);
	assert.ok(
		lines[0].includes("dinah.tree.openAttachment"),
		`the row did not name the command that was misaimed: ${lines.join(" | ")}`,
	);
});

test("openAttachment survives the argument the Command Palette does not pass", async () => {
	// The palette invokes a command with no argument at all, and this handler
	// read element.kind before it had established that an element arrived, so
	// the invocation threw a TypeError and the reader saw "Running the
	// contributed command failed" instead of a sentence naming the cause. Two
	// reviewers flagged the shape on this card while dinah-342 was in flight,
	// and the repair belongs here because that card's branch never touched
	// this file. The row command is also hidden from the palette now, which
	// closes the route rather than the hole; both are wanted, because a
	// keybinding and another extension reach the handler past the manifest.
	const r = recorder();
	const lines: string[] = [];
	await openAttachment(undefined, r.host, (line) => lines.push(line));
	assert.deepEqual(r.files, []);
	assert.deepEqual(r.opened, []);
	assert.deepEqual(r.checkpoints, []);
	assert.deepEqual(r.errors, []);
	assert.deepEqual(r.offered, []);
	assert.deepEqual(r.logged, []);
	assert.equal(lines.length, 1, `the handler said: ${lines.join(" | ")}`);
	assert.ok(
		lines[0].includes("dinah.tree.openAttachment"),
		`the absent row did not name the command that was misaimed: ${lines.join(" | ")}`,
	);
});

test("contextFor still composes the context a card row names", () => {
	// The two refusals above are satisfied by a function that refuses
	// everything, so this is what keeps them honest: the ordinary invocation
	// from a card's context menu goes on producing the same pinned call.
	const target = contextFor(cardElement(), "dinah", silentHost);
	assert.ok(target, "a card row composed no context");
	assert.equal(target.ref, "dinah-1");
	assert.equal(target.root, "C:/work/board");
	assert.equal(target.folder, "C:/work");
	assert.equal(target.exe, "dinah");
	assert.equal(target.host, silentHost);
});

test("contextFor answers undefined for a card row whose workbench never resolved", () => {
	// A candidate row that has not been expanded carries no data, so there is
	// no path to pin the call to. This is the branch that predates dinah-342
	// and it is asserted here so the reordering above did not remove it.
	const element: TreeElement = {
		kind: "card",
		row: rowFor(undefined),
		node: { kind: "card", ref: "dinah-1", count: 1 },
	};
	assert.equal(contextFor(element, "dinah", silentHost), undefined);
});

// This file is compiled to out/test/unit/, so the extension root is three up.
const extensionSource = readFileSync(
	join(__dirname, "..", "..", "..", "src", "extension.ts"),
	"utf8",
);

test("the command handler hands its argument straight to contextFor", () => {
	// Testing the decision and not the wiring is how the defect survived. The
	// handler reads nothing off the element itself: it passes the binding to
	// contextFor, which is the function the three tests above hold to the
	// missing-element contract. A regression that read a field first, which is
	// exactly what shipped, would leave those three green, so this reads the
	// one module the unit layer cannot import the way layers.ts and
	// spawn-sites.ts already read src for a single-site invariant.
	assert.ok(
		extensionSource.includes("contextFor("),
		"extension.ts no longer calls contextFor, so this check proved nothing",
	);
	const dereferences = extensionSource
		.split(/\r?\n/)
		.filter((line) => /\belement[.?]/.test(line));
	assert.deepEqual(
		dereferences,
		[],
		"extension.ts reads a field off an element binding, which throws when the Command Palette passes none",
	);
});

test("the handler's parameter admits the argument the palette does not pass", () => {
	// The type is half the guard. A handler declared to take a TreeElement
	// tells every later reader that an element always arrives, and the compiler
	// then agrees that reading a field off it is safe.
	assert.ok(
		extensionSource.includes("async (element: TreeElement | undefined) =>"),
		"the flow-command handler no longer declares its element as possibly absent",
	);
});
